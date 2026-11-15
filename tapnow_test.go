package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tapnowZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var body bytes.Buffer
	z := zip.NewWriter(&body)
	for name, text := range files {
		f, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, text); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func tapnowFixturePackage(edition, version string, body []byte) tapnowPackage {
	digest := sha256.Sum256(body)
	return tapnowPackage{Version: version, URL: tapnowRepository + "/releases/download/v" + version + "/tapnow-" + edition + "-" + version + "-chrome.zip", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body))}
}

func tapnowRequest(host *server, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+host.host+path, strings.NewReader(body))
	r.Header.Set("X-ACU-Token", token)
	w := httptest.NewRecorder()
	host.ServeHTTP(w, r)
	return w
}

func TestTapnowOptInAndSessionBoundary(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.tapnow.root = t.TempDir()
	calls := 0
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
		calls++
		t.Error("unexpected network")
		return nil, io.EOF
	})}
	w := tapnowRequest(host, "GET", "/runtime/tapnow", "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"checked":true`) || calls != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"check", "prepare", "open", "config"} {
		w = tapnowRequest(host, "POST", "/runtime/tapnow/"+path, `{}`, "")
		if w.Code != 403 {
			t.Fatal(path, w.Code)
		}
	}
	for _, body := range []string{`{"edition":"../escape"}`, `{"edition":"backup","url":"https://evil.test"}`, `{} {}`} {
		w = tapnowRequest(host, "POST", "/runtime/tapnow/prepare", body, host.token)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatal("opt-in triggered network")
	}
}

func TestTapnowCatalogRejectsUntrustedMetadata(t *testing.T) {
	good := tapnowFixturePackage("backup", "0.1.23", []byte("fixture"))
	if !validTapnowPackage("backup", good) {
		t.Fatal("valid rejected")
	}
	for _, modify := range []func(*tapnowPackage){
		func(p *tapnowPackage) { p.URL = "https://evil.test/package.zip" },
		func(p *tapnowPackage) { p.URL += "?redirect=x" },
		func(p *tapnowPackage) { p.Version = "../1" },
		func(p *tapnowPackage) { p.Size = 33 << 20 },
		func(p *tapnowPackage) { p.SHA256 = "not-a-digest" },
	} {
		bad := good
		modify(&bad)
		if validTapnowPackage("backup", bad) {
			t.Fatal(bad)
		}
	}
	host, _ := testServer(t, http.NotFoundHandler())
	host.tapnow.root = t.TempDir()
	var reply string
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(reply)), Header: make(http.Header)}, nil
	})}
	for _, reply = range []string{`{}`, `{"schema":2,"packages":{}}`, `{"schema":1,"packages":{"backup":{}}}`} {
		w := tapnowRequest(host, "POST", "/runtime/tapnow/check", `{}`, host.token)
		if w.Code != 400 || host.tapnow.snapshot().Checked {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestTapnowSafeExtraction(t *testing.T) {
	manifest := `{"manifest_version":3,"version":"0.1.23"}`
	for _, name := range []string{"../escape", "/absolute", "C:/evil", "a\\..\\evil", "../manifest.json", "MANIFEST.json"} {
		body := tapnowZIP(t, map[string]string{"manifest.json": manifest, name: "inert"})
		if unpackTapnow(body, t.TempDir(), "0.1.23") == nil {
			t.Fatal("accepted", name)
		}
	}
	body := tapnowZIP(t, map[string]string{"manifest.json": manifest, "chunks/popup.js": "inert fixture"})
	if err := unpackTapnow(body, t.TempDir(), "0.1.23"); err != nil {
		t.Fatal(err)
	}
	if unpackTapnow(body, t.TempDir(), "0.1.24") == nil {
		t.Fatal("version mismatch")
	}
	if unpackTapnow([]byte("invalid zip"), t.TempDir(), "0.1.23") == nil {
		t.Fatal("invalid zip accepted")
	}
}

func TestTapnowIndependentPrepareUpdateAndFailurePreservePrevious(t *testing.T) {
	root := t.TempDir()
	s := &tapnowState{root: root}
	var body []byte
	client := &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	for _, version := range []string{"0.1.23", "0.1.24"} {
		body = tapnowZIP(t, map[string]string{"manifest.json": `{"manifest_version":3,"version":"` + version + `"}`, "popup.js": version})
		if err := s.prepare(context.Background(), client, "companion", tapnowFixturePackage("companion", version, body)); err != nil {
			t.Fatal(err)
		}
		if readTapnowManifest(filepath.Join(root, "companion")) != version {
			t.Fatal("not prepared")
		}
	}
	if readTapnowManifest(filepath.Join(root, "companion.previous")) != "0.1.23" {
		t.Fatal("previous lost")
	}
	p := tapnowFixturePackage("companion", "0.1.25", body)
	p.SHA256 = strings.Repeat("0", 64)
	if s.prepare(context.Background(), client, "companion", p) == nil {
		t.Fatal("accepted tamper")
	}
	if readTapnowManifest(filepath.Join(root, "companion")) != "0.1.24" {
		t.Fatal("failure changed current")
	}
	if s.prepare(context.Background(), client, "companion", tapnowFixturePackage("companion", "0.1.22", body)) == nil {
		t.Fatal("downgrade")
	}
	if _, err := os.Stat(filepath.Join(root, "backup")); !os.IsNotExist(err) {
		t.Fatal("installed unselected edition")
	}
}

func TestTapnowRejectsSymlinkTarget(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "backup")); err != nil {
		t.Skip(err)
	}
	body := tapnowZIP(t, map[string]string{"manifest.json": `{"manifest_version":3,"version":"0.1.23"}`})
	client := &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if (&tapnowState{root: root}).prepare(context.Background(), client, "backup", tapnowFixturePackage("backup", "0.1.23", body)) == nil {
		t.Fatal("accepted symlink")
	}
	files, _ := os.ReadDir(outside)
	if len(files) != 0 {
		t.Fatal("wrote through symlink")
	}
}

func TestTapnowConfigurationExportChecksProviderAndNeverChangesAgents(t *testing.T) {
	enabled := true
	endpoint := "https://api.acucompute.com/v1"
	var writes []string
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			writes = append(writes, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/providers":
			json.NewEncoder(w).Encode(map[string]any{"providers": []any{map[string]any{"id": "acu", "responses": endpoint, "key": map[string]any{"set": true, "masked": "sk-***"}, "models": []any{map[string]any{"id": "acu-auto", "on": enabled}}}}})
		case "/api/provider/models":
			io.WriteString(w, `{}`)
		case "/api/provider/key":
			io.WriteString(w, `{"key":"sk-isolated-export-fixture"}`)
		default:
			t.Error("unexpected", r.URL.Path)
			w.WriteHeader(400)
		}
	}))
	w := tapnowRequest(host, "POST", "/runtime/tapnow/config", `{"provider":"acu","model":"acu-auto"}`, host.token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "sk-isolated-export-fixture") || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal(w.Code, w.Body.String())
	}
	var config map[string]any
	if json.Unmarshal(w.Body.Bytes(), &config) != nil || config["kind"] != "acu-tapnow-companion" {
		t.Fatal(config)
	}
	for _, change := range []func(){func() { enabled = false }, func() { enabled = true; endpoint = "https://api.acucompute.com.evil.invalid/v1" }} {
		change()
		writes = nil
		w = tapnowRequest(host, "POST", "/runtime/tapnow/config", `{"provider":"acu","model":"acu-auto"}`, host.token)
		if w.Code != 400 || strings.Contains(w.Body.String(), "sk-isolated") {
			t.Fatal(w.Code, w.Body.String())
		}
		for _, path := range writes {
			if path == "/api/provider/key" {
				t.Fatal("read key for unavailable config")
			}
		}
	}
	for _, path := range writes {
		if path != "/api/provider/models" {
			t.Fatal("changed agent", path)
		}
	}
}

func TestTapnowCheckDoesNotPrepareOrOpen(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.tapnow.root = t.TempDir()
	body := tapnowZIP(t, map[string]string{"manifest.json": `{"manifest_version":3,"version":"0.1.23"}`})
	catalog := tapnowCatalog{Schema: 1, Packages: map[string]tapnowPackage{"backup": tapnowFixturePackage("backup", "0.1.23", body), "companion": tapnowFixturePackage("companion", "0.1.23", body)}}
	count := 0
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		count++
		if r.URL.String() != tapnowCatalogURL {
			t.Fatal(r.URL)
		}
		payload, _ := json.Marshal(catalog)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}
	w := tapnowRequest(host, "POST", "/runtime/tapnow/check", `{}`, host.token)
	if w.Code != 200 || count != 1 || !host.tapnow.snapshot().Checked {
		t.Fatal(w.Code, w.Body.String())
	}
	files, _ := os.ReadDir(host.tapnow.root)
	if len(files) != 0 {
		t.Fatal("check prepared files")
	}
}
