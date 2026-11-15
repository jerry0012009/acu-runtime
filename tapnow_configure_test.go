package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func configureCatalogFixture(t *testing.T, host *server, edition string, version *string, calls *[]string) {
	t.Helper()
	key := []byte("public-key-fixture-for-configure-and-version-confirmation")
	digest := sha256.Sum256(key)
	id := make([]byte, 32)
	for i, b := range digest[:16] {
		id[2*i] = 'a' + (b >> 4)
		id[2*i+1] = 'a' + (b & 15)
	}
	if edition == "companion" {
		tapnowCompanionID = string(id)
	} else {
		tapnowBackupID = string(id)
	}
	archives := map[string][]byte{}
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		*calls = append(*calls, r.URL.String())
		manifest, _ := json.Marshal(map[string]any{"manifest_version": 3, "version": *version, "key": base64.StdEncoding.EncodeToString(key)})
		if archives[*version] == nil {
			archives[*version] = tapnowZIP(t, map[string]string{"manifest.json": string(manifest), "popup.js": "fixture"})
		}
		body := archives[*version]
		if r.URL.String() == tapnowCatalogURL {
			body, _ = json.Marshal(tapnowCatalog{Schema: 1, Packages: map[string]tapnowPackage{
				"companion": tapnowFixturePackage("companion", *version, body), "backup": tapnowFixturePackage("backup", *version, body),
			}})
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	host.tapnow.open = func(context.Context, string) error { return nil }
}

func TestTapnowConfigureAlwaysRefreshesAndRequiresPreparedVersion(t *testing.T) {
	host := nativeFixtureHost(t)
	version := "0.1.23"
	calls := []string{}
	configureCatalogFixture(t, host, "companion", &version, &calls)
	opened := ""
	host.tapnow.openStore = func(_ context.Context, address string) error { opened = address; return nil }
	configure := func() {
		t.Helper()
		w := tapnowRequest(host, "POST", "/runtime/tapnow/configure", `{"edition":"companion","provider":"acu","model":"acu-auto"}`, host.token)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	configure()
	if opened != "chrome://extensions/" {
		t.Fatal(opened)
	}
	if g := host.tapnow.grants["companion"]; g.RequiredVersion != "0.1.23" || !g.ReloadRequired {
		t.Fatal(g)
	}
	version = "0.1.24"
	calls = nil
	configure()
	if len(calls) != 2 || calls[0] != tapnowCatalogURL || !strings.Contains(calls[1], "v0.1.24") {
		t.Fatal(calls)
	}
	if readTapnowManifest(filepath.Join(host.tapnow.root, "companion")) != "0.1.24" {
		t.Fatal("latest files not prepared")
	}
	msg := tapnowNativeMessage{Action: "connect", Origin: "chrome-extension://" + tapnowCompanionID + "/", Version: "0.1.23"}
	w := nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	if w.Code != 409 || strings.Contains(w.Body.String(), "sk-native") || host.tapnow.grants["companion"].Phase != "waiting" {
		t.Fatal(w.Code, w.Body.String())
	}
	msg.Version = "0.1.24"
	w = nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	var reply tapnowNativeReply
	json.Unmarshal(w.Body.Bytes(), &reply)
	if w.Code != 200 || reply.Config == nil || reply.Config.APIKey != "sk-native-isolated-fixture" {
		t.Fatal(w.Code, w.Body.String())
	}
	ack := tapnowNativeMessage{Action: "ack", Origin: msg.Origin, Grant: reply.Grant, Version: msg.Version, Success: true}
	if nativeAPIRequest(host, ack, host.tapnow.nativeToken, "", "").Code != 200 {
		t.Fatal("ack rejected")
	}
	if host.tapnow.snapshot().Editions[0].ReloadRequired {
		t.Fatal("reload requirement retained after current-version receipt")
	}
	// Same version still refreshes catalog/package, but opens the connection page
	// rather than unnecessarily sending the user through directory loading again.
	calls = nil
	configure()
	if len(calls) != 2 || opened != "chrome-extension://"+tapnowCompanionID+"/connect.html" {
		t.Fatal(calls, opened)
	}
	// A failed new attempt cannot report a previous confirmed configuration.
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("offline"))}, nil
	})}
	w = tapnowRequest(host, "POST", "/runtime/tapnow/configure", `{"edition":"companion","provider":"acu","model":"acu-auto"}`, host.token)
	if w.Code != 400 || host.tapnow.snapshot().Editions[0].Connection == "configured" {
		t.Fatal(w.Code, w.Body.String())
	}
}
