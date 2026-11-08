package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type agentPackageTransport func(*http.Request) (*http.Response, error)

func (f agentPackageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentPackageUsesDomesticPinnedVendorArtifacts(t *testing.T) {
	for _, p := range []struct{ os, arch, suffix string }{{"windows", "amd64", ".exe"}, {"darwin", "amd64", ".zip"}, {"darwin", "arm64", ".zip"}} {
		v, err := workBuddyPackage(p.os, p.arch)
		if err != nil || !trustedWorkBuddyURL(v.URL) || !digestPattern.MatchString(v.SHA256) || !strings.HasSuffix(v.Name, p.suffix) {
			t.Fatal(v, err)
		}
	}
	if _, err := workBuddyPackage("windows", "arm64"); err == nil {
		t.Fatal("invented unsupported vendor artifact")
	}
	for _, u := range []string{"http://download.codebuddy.cn/a.exe", "https://evil.test/a.exe", "https://download.codebuddy.cn.evil.test/a.exe", "https://x@download.codebuddy.cn/a.exe", "https://download.codebuddy.cn:8443/a.exe"} {
		if trustedWorkBuddyURL(u) {
			t.Fatal(u)
		}
	}
}

func TestAgentDownloadRejectsTamperingBeforeInstaller(t *testing.T) {
	data := "signed installer fixture"
	h := sha256.Sum256([]byte(data))
	digest := hex.EncodeToString(h[:])
	client := &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data)), ContentLength: int64(len(data)), Header: make(http.Header)}, nil
	})}
	for _, good := range []bool{true, false} {
		target := filepath.Join(t.TempDir(), "package.exe")
		expected := digest
		if !good {
			expected = strings.Repeat("0", 64)
		}
		err := downloadAgentPackage(context.Background(), client, "https://download.codebuddy.cn/fixture.exe", target, expected, nil)
		if good && err != nil {
			t.Fatal(err)
		}
		if !good {
			if err == nil {
				t.Fatal("accepted tampering")
			}
			if _, e := os.Stat(target); !os.IsNotExist(e) {
				t.Fatal("kept unverified installer")
			}
		}
	}
	if err := downloadAgentPackage(context.Background(), client, "https://evil.test/a.exe", filepath.Join(t.TempDir(), "a"), digest, nil); err == nil {
		t.Fatal("accepted foreign host")
	}
}

func TestAgentInstallEndpointRejectsUnsafeAndNonDesktopRequests(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	for _, body := range []string{`{"agent":"workbuddy","provider":"x","model":"y"}`, `{"agent":"shell","url":"https://evil.test"}`} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:4317/runtime/agents/install", strings.NewReader(body))
		r.Header.Set("X-ACU-Token", host.token)
		w := httptest.NewRecorder()
		host.ServeHTTP(w, r)
		if w.Code != 409 {
			t.Fatal("web mode allowed installation", w.Code)
		}
	}
	host.desktop = true
	for _, body := range []string{`{"agent":"shell"}`, `{"agent":"workbuddy","provider":"x","model":"y","url":"https://evil.test"}`, `{"agent":"codex"} {}`} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:4317/runtime/agents/install", strings.NewReader(body))
		r.Header.Set("X-ACU-Token", host.token)
		w := httptest.NewRecorder()
		host.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("accepted arbitrary install command", w.Code)
		}
	}
}

func TestInstallWaitingCanBeCancelledWithoutTouchingInstaller(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.desktop = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host.agentInstall.view.Phase = "waiting"
	host.agentInstall.cancel = cancel
	r := httptest.NewRequest("POST", "http://127.0.0.1:4317/runtime/agents/install/cancel", strings.NewReader(`{}`))
	r.Header.Set("X-ACU-Token", host.token)
	w := httptest.NewRecorder()
	host.ServeHTTP(w, r)
	if w.Code != 200 || ctx.Err() == nil {
		t.Fatal("waiting was not cancelled")
	}
}

func TestInstalledAgentDelegatesAndPreservesExistingModel(t *testing.T) {
	var calls []string
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			calls = append(calls, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/providers":
			io.WriteString(w, setupProviders)
		case "/api/provider/key":
			io.WriteString(w, `{"key":"fixture-secret"}`)
		case "/api/state":
			io.WriteString(w, `{"agents":[{"id":"workbuddy","wired":true,"fields":[]}]}`)
		case "/api/settings":
			io.WriteString(w, `{"gateway":"http://127.0.0.1:3425","vision":"existing/vision","imageGen":"off","searchAPIs":[{"vendor":"you","ready":true}]}`)
		case "/api/library":
			io.WriteString(w, `{"servers":[]}`)
		case "/api/library/servers/save":
			io.WriteString(w, `{}`)
		case "/api/library/skills/probe":
			io.WriteString(w, `{"candidates":[{"name":"acu-image-tools","path":"."}]}`)
		case "/api/library/skills/install":
			io.WriteString(w, `{}`)
		default:
			t.Errorf("unexpected kernel call %s", r.URL.Path)
			http.Error(w, "unexpected", 404)
		}
	}))
	host.cloudTransport = modelProbe(t, 200)
	host.imageSkillSource = func() (string, error) { return t.TempDir(), nil }
	if err := host.configureInstalledAgent(context.Background(), "workbuddy", "acu-runtime", "acu-auto"); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call == "/api/set" || call == "/api/agents/connect/workbuddy" {
			t.Fatal("replaced existing client selection")
		}
	}
	n := 0
	for _, call := range calls {
		if call == "/api/library/servers/save" {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("expected three kernel-managed MCP assignments, got %d", n)
	}
}
