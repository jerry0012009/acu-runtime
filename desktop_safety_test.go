package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunningDesktopBlocksKernelWritesBeforeForwarding(t *testing.T) {
	for _, path := range []string{"/api/set", "/kernel/api/agents/connect/codex", "/kernel/api/agents/disconnect/codex", "/kernel/api/agents/disconnect-offline/codex", "/kernel/api/agents/stage/codex", "/kernel/api/agents/restart/codex", "/kernel/api/agent-models/codex"} {
		t.Run(path, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unsafe request reached kernel") }))
			host.desktop = true
			host.desktopProbe = func() (bool, error) { return true, nil }
			w := httptest.NewRecorder()
			host.proxy(w, httptest.NewRequest("POST", path, strings.NewReader(`{"agent":"codex","field":"model","value":"acu/auto"}`)))
			if w.Code != http.StatusConflict {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestDesktopGuardPreservesReadsAndOtherAgents(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/kernel/api/agent-models/codex", ""},
		{"POST", "/api/set", `{"agent":"workbuddy","field":"model","value":"acu/auto"}`},
		{"POST", "/kernel/api/agents/connect/workbuddy", `{}`},
	} {
		host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if string(b) != tc.body {
				t.Fatalf("request body changed: %q", b)
			}
			w.WriteHeader(204)
		}))
		host.desktop = true
		host.desktopProbe = func() (bool, error) { return true, nil }
		w := httptest.NewRecorder()
		host.proxy(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != 204 {
			t.Fatalf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestDesktopGuardClosedAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		open    bool
		err     error
		allowed bool
	}{
		{false, nil, true}, {true, nil, false}, {false, errors.New("cannot inspect processes"), false},
	} {
		host := &server{desktop: true, desktopProbe: func() (bool, error) { return tc.open, tc.err }}
		if got := host.allowCodexDesktopChange(httptest.NewRecorder()); got != tc.allowed {
			t.Fatalf("allowed %v, want %v", got, tc.allowed)
		}
	}
}

func TestRecommendationBlocksBeforeCloudOrKernel(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("blocked recommendation reached kernel") }))
	host.desktop = true
	host.desktopProbe = func() (bool, error) { return true, nil }
	w := setupRequest(host, `{"agent":"codex","provider":"acu-runtime","model":"acu-auto"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
}
