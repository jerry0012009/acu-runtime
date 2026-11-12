package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const billingSite = "https://console.acucompute.com"

func TestPreferenceOnlyLoginDoesNotSelectOrWriteModelKey(t *testing.T) {
	host := &server{}
	ticket := loginTicket{Site: billingSite, State: "preference-login", Preferences: true, Expires: time.Now().Add(time.Minute)}
	host.account.ticket = ticket
	var paths []string
	host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/user/self":
			return cloudFixture(`{"success":true,"data":{"id":42,"username":"preference-reader"}}`), nil
		case "/api/token/":
			return cloudFixture(`{"success":true,"data":{"total":1,"items":[{"id":7,"status":1,"unlimited_quota":true}]}}`), nil
		default:
			t.Fatalf("preference login must not reveal or apply a model Key: %s", r.URL.Path)
			return nil, nil
		}
	})
	message, _ := json.Marshal(map[string]any{"type": "acu-account-login", "state": ticket.State, "accessToken": "fixture-access", "expiresAt": time.Now().Add(time.Hour).Unix(), "sessionId": "fixture-session"})
	if err := host.completeAccountLogin(context.Background(), "acu-login-"+ticket.State, billingSite, billingSite, string(message)); err != nil {
		t.Fatal(err)
	}
	view := host.account.snapshot()
	if !view.LoggedIn || view.Selected != 0 || len(paths) != 2 {
		t.Fatalf("preference-only login changed model connection: %+v %v", view, paths)
	}
}

func TestPreferencesSessionAccessAndContract(t *testing.T) {
	for _, test := range []struct {
		name     string
		logged   bool
		token    string
		upstream int
		body     string
		want     int
	}{
		{"local token required", true, "", 200, `{"success":true,"data":{"version":1,"records":[]}}`, 403},
		{"account login required", false, "local", 200, `{}`, 401},
		{"unsupported distinct from empty", true, "local", 404, `{}`, 404},
		{"empty library", true, "local", 200, `{"success":true,"data":{"version":1,"records":[]}}`, 200},
		{"reject future schema", true, "local", 200, `{"success":true,"data":{"version":2,"records":[]}}`, 502},
		{"transient failure", true, "local", 503, `secret upstream diagnostics`, 502},
		{"revocation", true, "local", 401, `{}`, 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := &server{token: "local"}
			host.account.view = accountView{LoggedIn: test.logged, Site: billingSite}
			host.account.bearer, host.account.userID, host.account.expires = "private-access", 42, time.Now().Add(time.Hour).Unix()
			calls := 0
			host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.String() != billingSite+"/api/user/self/acu-preference-library" || r.Header.Get("Authorization") != "Bearer private-access" {
					t.Fatalf("unexpected personal read %s %s", r.Method, r.URL)
				}
				res := cloudFixture(test.body)
				res.StatusCode = test.upstream
				return res, nil
			})
			req := httptest.NewRequest("GET", "http://localhost/runtime/preferences/library?user_id=99", nil)
			req.Header.Set("X-ACU-Token", test.token)
			res := httptest.NewRecorder()
			host.accountPreferences(res, req)
			if res.Code != test.want || res.Header().Get("Cache-Control") != "no-store" || strings.Contains(res.Body.String(), "private-access") || strings.Contains(res.Body.String(), "secret upstream") {
				t.Fatalf("unsafe response %d %s", res.Code, res.Body.String())
			}
			if (!test.logged || test.token == "") && calls != 0 {
				t.Fatal("unauthorized upstream request")
			}
			if test.upstream == 401 && host.account.snapshot().LoggedIn {
				t.Fatal("revoked account retained")
			}
		})
	}
}

func TestPreferencesDiscardAccountSwitchAndLogoutResponses(t *testing.T) {
	for _, logout := range []bool{false, true} {
		host := &server{token: "local"}
		host.account.view = accountView{LoggedIn: true, Site: billingSite, Username: "same-name"}
		host.account.bearer, host.account.userID, host.account.expires = "old-access", 42, time.Now().Add(time.Hour).Unix()
		host.account.session.State = "old-session"
		oldIdentity := host.account.snapshot().Identity
		host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) {
			if logout {
				host.account.mu.Lock()
				host.account.view = accountView{}
				host.account.bearer = ""
				host.account.mu.Unlock()
			} else {
				host.account.mu.Lock()
				host.account.userID = 43
				host.account.bearer = "new-access"
				host.account.session.State = "new-session"
				host.account.mu.Unlock()
			}
			return cloudFixture(`{"success":true,"data":{"version":1,"records":[{"id":"old","summary":"old private summary"}]}}`), nil
		})
		req := httptest.NewRequest("GET", "http://localhost/runtime/preferences/library", nil)
		req.Header.Set("X-ACU-Token", "local")
		res := httptest.NewRecorder()
		host.accountPreferences(res, req)
		if res.Code != 409 || strings.Contains(res.Body.String(), "old private") {
			t.Fatal("old account response leaked")
		}
		if host.account.snapshot().Identity == oldIdentity {
			t.Fatal("identity did not change")
		}
	}
}
