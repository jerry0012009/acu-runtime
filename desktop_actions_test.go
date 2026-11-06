package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopActionsValidateAndQueue(t *testing.T) {
	host := &server{host: "127.0.0.1:4317", token: "test-session", desktop: true, desktopActions: make(chan desktopAction, 16)}
	for _, item := range []struct {
		body, token string
		status      int
	}{
		{`{"action":"main","query":"view=usage&tab=requests&provider=example&unexpected=secret"}`, "test-session", 200},
		{`{"action":"fit","height":640}`, "test-session", 200},
		{`{"action":"hide"}`, "test-session", 200},
		{`{"action":"main"}`, "", 403},
		{`{"action":"quit"}`, "test-session", 400},
		{`{"action":"main","query":"view=https%3A%2F%2Fevil.invalid"}`, "test-session", 400},
		{`{"action":"fit","height":-1}`, "test-session", 400},
		{`{"action":"fit","height":100001}`, "test-session", 400},
		{`{"action":"main","url":"https://evil.invalid"}`, "test-session", 400},
		{`{"action":"main"}{}`, "test-session", 400},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://"+host.host+"/runtime/desktop/action", strings.NewReader(item.body))
		request.Header.Set("X-ACU-Token", item.token)
		response := httptest.NewRecorder()
		host.ServeHTTP(response, request)
		if response.Code != item.status {
			t.Fatalf("%s: got %d, want %d", item.body, response.Code, item.status)
		}
	}
	if len(host.desktopActions) != 3 {
		t.Fatalf("unexpected action count: %d", len(host.desktopActions))
	}
	first := <-host.desktopActions
	if strings.Contains(first.Query, "unexpected") || !strings.Contains(first.Query, "tab=requests") {
		t.Fatalf("invalid navigation query: %s", first.Query)
	}
}

func TestPanelDocumentIsLocalAndUncached(t *testing.T) {
	host := &server{host: "127.0.0.1:4317", token: "test-session"}
	response := httptest.NewRecorder()
	host.ServeHTTP(response, httptest.NewRequest("GET", "http://"+host.host+"/panel", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `content="test-session"`) || !strings.Contains(response.Body.String(), `/ui/panel.js`) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("panel document not served safely")
	}
}
