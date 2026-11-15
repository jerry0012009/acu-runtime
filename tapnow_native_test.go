package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureTapnowIDs(t *testing.T) {
	t.Helper()
	a, b := tapnowCompanionID, tapnowBackupID
	tapnowCompanionID = strings.Repeat("a", 32)
	tapnowBackupID = strings.Repeat("b", 32)
	t.Cleanup(func() {
		tapnowCompanionID, tapnowBackupID = a, b
	})
}

func nativeFixtureHost(t *testing.T) *server {
	t.Helper()
	fixtureTapnowIDs(t)
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/providers":
			io.WriteString(w, `{"providers":[{"id":"acu","responses":"https://acu.best/v1","key":{"set":true,"masked":"***"},"models":[{"id":"acu-auto","on":true}]}]}`)
		case "/api/provider/key":
			io.WriteString(w, `{"key":"sk-native-isolated-fixture"}`)
		case "/api/provider/models":
			io.WriteString(w, `{}`)
		default:
			t.Error("unexpected kernel call", r.URL.Path)
			w.WriteHeader(400)
		}
	}))
	host.desktop = true
	host.videoToolPath = func(context.Context) (string, error) { return "/fixture/acu-video-mcp", nil }
	host.tapnow.root = t.TempDir()
	host.tapnow.register = func() error { return nil }
	host.tapnow.openStore = func(context.Context, string) error { return nil }
	return host
}

func nativeAPIRequest(host *server, msg tapnowNativeMessage, token, origin, site string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(msg)
	r := httptest.NewRequest("POST", "http://"+host.host+"/runtime/tapnow/native", bytes.NewReader(body))
	r.Header.Set("X-ACU-Tapnow-Token", token)
	r.Header.Set("Origin", origin)
	r.Header.Set("Sec-Fetch-Site", site)
	w := httptest.NewRecorder()
	host.ServeHTTP(w, r)
	return w
}

func TestTapnowNativeConnectRequiresOneTimeGrantAndReadbackReceipt(t *testing.T) {
	host := nativeFixtureHost(t)
	w := tapnowRequest(host, "POST", "/runtime/tapnow/connect", `{"edition":"companion","provider":"acu","model":"acu-auto"}`, host.token)
	if w.Code != 200 || strings.Contains(w.Body.String(), "sk-native") {
		t.Fatal(w.Code, w.Body.String())
	}
	msg := tapnowNativeMessage{Action: "connect", Origin: "chrome-extension://" + tapnowCompanionID + "/", Version: "0.1.23"}
	w = nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	var reply tapnowNativeReply
	json.Unmarshal(w.Body.Bytes(), &reply)
	if w.Code != 200 || reply.Config == nil || reply.Config.APIKey != "sk-native-isolated-fixture" || reply.Config.Settings.Base != "https://acu.best/v1" {
		t.Fatal(w.Code, w.Body.String())
	}
	if host.tapnow.snapshot().Editions[0].Connection != "saving" {
		t.Fatal("claimed connected before receipt")
	}
	if nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "").Code != 409 {
		t.Fatal("grant reused")
	}
	ack := tapnowNativeMessage{Action: "ack", Origin: msg.Origin, Grant: reply.Grant, Version: "0.1.23", Success: true}
	bad := ack
	bad.Grant = secret()
	if nativeAPIRequest(host, bad, host.tapnow.nativeToken, "", "").Code != 409 {
		t.Fatal("foreign receipt")
	}
	bad = ack
	bad.Version = "0.1.24"
	if nativeAPIRequest(host, bad, host.tapnow.nativeToken, "", "").Code != 409 {
		t.Fatal("wrong version")
	}
	for i := 0; i < 2; i++ {
		if nativeAPIRequest(host, ack, host.tapnow.nativeToken, "", "").Code != 200 {
			t.Fatal("ack not idempotent")
		}
	}
	v := host.tapnow.snapshot().Editions[0]
	if v.Connection != "configured" || v.ConfirmedAt == "" || v.Version != "0.1.23" {
		t.Fatal(v)
	}
	w = nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"confirmed":true`) || strings.Contains(w.Body.String(), "sk-native") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTapnowNativeBoundaryAndBackupHasNoKey(t *testing.T) {
	host := nativeFixtureHost(t)
	w := tapnowRequest(host, "POST", "/runtime/tapnow/connect", `{"edition":"backup"}`, host.token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	msg := tapnowNativeMessage{Action: "connect", Origin: "chrome-extension://" + tapnowBackupID + "/", Version: "0.1.23"}
	for _, test := range []struct{ token, origin, site string }{{"", "", ""}, {host.token, "", ""}, {host.tapnow.nativeToken, "http://" + host.host, ""}, {host.tapnow.nativeToken, "", "same-origin"}} {
		if nativeAPIRequest(host, msg, test.token, test.origin, test.site).Code != 403 {
			t.Fatal("browser request accepted")
		}
	}
	foreign := msg
	foreign.Origin = "chrome-extension://" + strings.Repeat("c", 32) + "/"
	if nativeAPIRequest(host, foreign, host.tapnow.nativeToken, "", "").Code != 409 {
		t.Fatal("foreign extension")
	}
	w = nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	var reply tapnowNativeReply
	json.Unmarshal(w.Body.Bytes(), &reply)
	if w.Code != 200 || reply.Config != nil || strings.Contains(w.Body.String(), "sk-native") {
		t.Fatal(w.Code, w.Body.String())
	}
	host.tapnow.grants["backup"].Expires = time.Now().Add(-time.Second)
	if nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "").Code != 409 {
		t.Fatal("expired grant")
	}
}

func TestTapnowReleaseGateAndOfficialBases(t *testing.T) {
	if checkTapnowRelease() != nil {
		t.Fatal("fixed ZIP identities rejected")
	}
	fixtureTapnowIDs(t)
	if checkTapnowRelease() != nil {
		t.Fatal("fixture IDs rejected")
	}
	for _, value := range []string{"https://acu.best", "https://acu.best/v1", "https://api.acucompute.com/v1", "https://console.acucompute.com/v1", "https://acucompute.com/v1/", "https://api.acu.best/acu/v1"} {
		if _, err := tapnowAPIBase(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"https://acu.best.evil/v1", "http://acu.best/v1", "https://x@acu.best/v1", "https://acu.best:8443/v1", "https://acu.best/v1?x=1", "https://acu.best/other"} {
		if _, err := tapnowAPIBase(value); err == nil {
			t.Fatal(value)
		}
	}
}

func TestTapnowNativeFraming(t *testing.T) {
	var framed bytes.Buffer
	if err := writeNativeFrame(&framed, tapnowNativeReply{OK: true}); err != nil {
		t.Fatal(err)
	}
	body, err := readNativeFrame(&framed)
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatal(string(body), err)
	}
	for _, size := range []uint32{0, 16<<10 + 1} {
		var bad bytes.Buffer
		binary.Write(&bad, binary.LittleEndian, size)
		if _, err := readNativeFrame(&bad); err == nil {
			t.Fatal(size)
		}
	}
	var result bytes.Buffer
	if err := runTapnowNative("chrome-extension://untrusted/", strings.NewReader(""), &result); err != nil {
		t.Fatal(err)
	}
	body, err = readNativeFrame(&result)
	if err != nil || !strings.Contains(string(body), "untrusted") {
		t.Fatal(string(body), err)
	}
}

func TestTapnowZIPInstallPreparesOnlyChosenEditionAndOpensChrome(t *testing.T) {
	host := nativeFixtureHost(t)
	key := []byte("public-key-fixture-for-unpacked-extension-identity")
	digest := sha256.Sum256(key)
	id := make([]byte, 32)
	for i, b := range digest[:16] {
		id[2*i] = 'a' + (b >> 4)
		id[2*i+1] = 'a' + (b & 15)
	}
	tapnowBackupID = string(id)
	manifest, _ := json.Marshal(map[string]any{"manifest_version": 3, "version": "0.1.23", "key": base64.StdEncoding.EncodeToString(key)})
	archive := tapnowZIP(t, map[string]string{"manifest.json": string(manifest)})
	catalog := tapnowCatalog{Schema: 1, Packages: map[string]tapnowPackage{"backup": tapnowFixturePackage("backup", "0.1.23", archive), "companion": tapnowFixturePackage("companion", "0.1.23", archive)}}
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		body := archive
		if r.URL.String() == tapnowCatalogURL {
			body, _ = json.Marshal(catalog)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	openedDir, openedChrome := "", ""
	host.tapnow.open = func(_ context.Context, path string) error { openedDir = path; return nil }
	host.tapnow.openStore = func(_ context.Context, address string) error { openedChrome = address; return nil }
	w := tapnowRequest(host, "POST", "/runtime/tapnow/install", `{"edition":"backup"}`, host.token)
	if w.Code != 200 || openedChrome != "about:blank" || openedDir != filepath.Join(host.tapnow.root, "backup") {
		t.Fatal(w.Code, w.Body.String(), openedDir, openedChrome)
	}
	if host.tapnow.grants["backup"] == nil || host.tapnow.grants["companion"] != nil {
		t.Fatal("wrong edition granted")
	}
	if !tapnowPreparedIdentity(openedDir, "backup") {
		t.Fatal("fixed manifest identity rejected")
	}
	tapnowBackupID = strings.Repeat("p", 32)
	w = tapnowRequest(host, "POST", "/runtime/tapnow/install", `{"edition":"backup"}`, host.token)
	if w.Code != 400 {
		t.Fatal("accepted mismatched ZIP identity", w.Code)
	}
}
