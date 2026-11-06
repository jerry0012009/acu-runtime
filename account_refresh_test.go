package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func refreshFixture(t *testing.T) *server {
	t.Helper()
	host, _ := testServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("renewal must not modify kernel or Agent configuration")
	}))
	host.account = accountState{bearer: "old-access", expires: time.Now().Add(-time.Second).Unix(), session: loginTicket{Site: "https://acu.best", State: "session-nonce", Expires: time.Now().Add(-time.Hour)}, userID: 42, sessionID: "original-session", view: accountView{LoggedIn: true, Site: "https://acu.best", Selected: 9, Keys: []accountKey{{ID: 9, Available: true}}}}
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/user/self" || request.Header.Get("Authorization") != "Bearer renewed-access" {
			t.Error("renewal must only validate account identity with the new access credential")
		}
		return cloudFixture(`{"success":true,"data":{"id":42}}`), nil
	})
	return host
}

func refreshMessage(kind string, expired bool) string {
	encoded, _ := json.Marshal(map[string]any{"type": kind, "state": "session-nonce", "sessionId": "original-session", "accessToken": "renewed-access", "expiresAt": time.Now().Add(15 * time.Minute).Unix(), "expired": expired})
	return string(encoded)
}

func TestRenewalSurvivesAccessAndInitialLoginTicketExpiry(t *testing.T) {
	host := refreshFixture(t)
	if !host.account.refreshDue("session-nonce") || host.account.refreshDue("old-window") {
		t.Fatal("renewal scheduling is not tied to the active native window")
	}
	err := host.completeAccountRefresh(context.Background(), "acu-login-session-nonce", "https://acu.best/dashboard", "https://acu.best", refreshMessage("acu-account-refresh", false))
	view := host.account.snapshot()
	if err != nil || !view.LoggedIn || view.Refreshing || view.Selected != 9 || len(view.Keys) != 1 || host.account.bearer != "renewed-access" || host.account.refreshDue("session-nonce") {
		t.Fatalf("renewal failed or rewrote selected Key: %v %+v", err, view)
	}
	encoded, _ := json.Marshal(view)
	if strings.Contains(string(encoded), "access") || strings.Contains(string(encoded), "nonce") || strings.Contains(string(encoded), "original-session") {
		t.Fatal("renewal secrets leaked to main-window state")
	}
}

func TestTemporaryRenewalFailurePreservesAccountAndRecovers(t *testing.T) {
	host := refreshFixture(t)
	for _, kind := range []string{"acu-account-refresh-error", "acu-account-refresh"} {
		if err := host.completeAccountRefresh(context.Background(), "acu-login-session-nonce", "https://acu.best", "https://acu.best", refreshMessage(kind, false)); err != nil {
			t.Fatal(err)
		}
		view := host.account.snapshot()
		if !view.LoggedIn || view.Selected != 9 || (kind == "acu-account-refresh-error" && view.Error == "") || (kind == "acu-account-refresh" && view.Error != "") {
			t.Fatalf("temporary failure lost account or did not recover: %+v", view)
		}
	}
}

func TestRevokedRefreshSessionRequiresLoginWithoutTouchingKey(t *testing.T) {
	host := refreshFixture(t)
	if err := host.completeAccountRefresh(context.Background(), "acu-login-session-nonce", "https://acu.best", "https://acu.best", refreshMessage("acu-account-refresh-error", true)); err != nil {
		t.Fatal(err)
	}
	if host.account.snapshot().LoggedIn || host.account.bearer != "" || host.account.sessionActive("session-nonce") {
		t.Fatal("revoked refresh session was retained")
	}
}

func TestRefreshRejectsWrongWindowOriginNonceSessionAndAccount(t *testing.T) {
	for _, test := range []struct{ name, window, sender, top, replace string }{
		{"window", "acu-main", "https://acu.best", "https://acu.best", ""},
		{"origin", "acu-login-session-nonce", "https://evil.test", "https://acu.best", ""},
		{"subframe", "acu-login-session-nonce", "https://acu.best", "https://evil.test", ""},
		{"nonce", "acu-login-session-nonce", "https://acu.best", "https://acu.best", "session-nonce"},
		{"server-session", "acu-login-session-nonce", "https://acu.best", "https://acu.best", "original-session"},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := refreshFixture(t)
			message := refreshMessage("acu-account-refresh", false)
			if test.replace != "" {
				message = strings.ReplaceAll(message, test.replace, "wrong")
			}
			if err := host.completeAccountRefresh(context.Background(), test.window, test.sender, test.top, message); err == nil || host.account.bearer != "old-access" {
				t.Fatal("invalid renewal was accepted")
			}
		})
	}
	for _, body := range []string{`{"success":true,"data":{"id":99}}`, `{"success":false}`} {
		host := refreshFixture(t)
		host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if err := host.completeAccountRefresh(context.Background(), "acu-login-session-nonce", "https://acu.best", "https://acu.best", refreshMessage("acu-account-refresh", false)); err == nil || host.account.bearer != "old-access" {
			t.Fatal("unverified account renewal was accepted")
		}
	}
}
