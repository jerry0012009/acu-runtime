package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPreferenceSettingsCloudRoundtrip(t *testing.T) {
	bearer := os.Getenv("ACU_SETTINGS_ACCEPTANCE_TOKEN")
	if bearer == "" {
		t.Skip("explicit cloud acceptance only")
	}
	userID, err := strconv.Atoi(os.Getenv("ACU_SETTINGS_ACCEPTANCE_USER"))
	if err != nil || userID <= 0 {
		t.Fatal("acceptance user required")
	}
	host := &server{token: "acceptance-local", cloudTransport: http.DefaultTransport}
	host.account.view = accountView{LoggedIn: true, Site: billingSite}
	host.account.bearer, host.account.userID, host.account.expires = bearer, userID, time.Now().Add(time.Hour).Unix()
	call := func(method string, body string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, "http://localhost/runtime/preferences/settings", strings.NewReader(body))
		req.Header.Set("X-ACU-Token", "acceptance-local")
		res := httptest.NewRecorder()
		host.accountPreferences(res, req)
		if res.Code != 200 {
			t.Fatalf("cloud settings HTTP %d", res.Code)
		}
		var data map[string]any
		if json.Unmarshal(res.Body.Bytes(), &data) != nil {
			t.Fatal("invalid cloud response")
		}
		return data
	}
	before := call(http.MethodGet, "")
	patch := map[string]any{}
	for _, key := range []string{"learningEnabled", "observerEnabled", "advisorEnabled", "injectionEnabled"} {
		if _, ok := before[key].(bool); !ok {
			t.Fatal("missing settings flags")
		}
		patch[key] = before[key]
	}
	patch["observerInterval"] = before["observerInterval"]
	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	saved := call(http.MethodPut, string(body))
	after := call(http.MethodGet, "")
	for key, value := range patch {
		if saved[key] != value || after[key] != value {
			t.Fatalf("settings did not persist: %s", key)
		}
	}
	t.Log("Runtime GET/PUT/GET matched website settings without changing flags or interval")
}

func TestPreferenceSettingsUseWebsiteContract(t *testing.T) {
	for _, interval := range []string{"null", "37"} {
		t.Run(interval, func(t *testing.T) {
			host := &server{token: "local"}
			host.account.view = accountView{LoggedIn: true, Site: billingSite}
			host.account.bearer, host.account.userID, host.account.expires = "account-session", 42, time.Now().Add(time.Hour).Unix()
			body := `{"learningEnabled":false,"observerEnabled":true,"advisorEnabled":false,"injectionEnabled":true,"observerInterval":` + interval + `}`
			host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPut || r.URL.String() != billingSite+"/api/user/self/acu-advisor/settings" || r.Header.Get("Authorization") != "Bearer account-session" || r.Header.Get("Origin") != billingSite || r.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("unexpected settings write: %s %s", r.Method, r.URL)
				}
				var settings map[string]any
				if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
					t.Fatal(err)
				}
				if len(settings) != 5 || settings["learningEnabled"] != false || settings["observerEnabled"] != true || settings["advisorEnabled"] != false || settings["injectionEnabled"] != true {
					t.Fatalf("lost explicit flags: %+v", settings)
				}
				if interval == "null" && settings["observerInterval"] != nil || interval == "37" && settings["observerInterval"] != float64(37) {
					t.Fatalf("interval changed: %+v", settings)
				}
				return cloudFixture(`{"success":true,"data":{"learningEnabled":false,"observerEnabled":true,"advisorEnabled":false,"injectionEnabled":true,"observerInterval":` + interval + `,"globalEnabled":true}}`), nil
			})
			req := httptest.NewRequest(http.MethodPut, "http://localhost/runtime/preferences/settings?user_id=99", strings.NewReader(body))
			req.Header.Set("X-ACU-Token", "local")
			res := httptest.NewRecorder()
			host.accountPreferences(res, req)
			if res.Code != 200 || strings.Contains(res.Body.String(), "account-session") {
				t.Fatalf("settings response: %d %s", res.Code, res.Body.String())
			}
		})
	}
}

func TestPreferenceSettingsRejectInvalidWritesAndAccountChanges(t *testing.T) {
	valid := `{"learningEnabled":true,"observerEnabled":false,"advisorEnabled":false,"injectionEnabled":false,"observerInterval":null}`
	for _, test := range []struct {
		name, body            string
		logged, local, change bool
		want                  int
	}{
		{"missing local token", valid, true, false, false, 403},
		{"missing login", valid, false, true, false, 401},
		{"partial flags", `{"learningEnabled":true}`, true, true, false, 400},
		{"foreign owner", strings.TrimSuffix(valid, "}") + `,"newapiUserId":"99"}`, true, true, false, 400},
		{"invalid interval", strings.Replace(valid, "null", "0", 1), true, true, false, 400},
		{"oversized interval", strings.Replace(valid, "null", "100001", 1), true, true, false, 400},
		{"invalid type", strings.Replace(valid, "true", `"true"`, 1), true, true, false, 400},
		{"account switched", valid, true, true, true, 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := &server{token: "local"}
			host.account.view = accountView{LoggedIn: test.logged, Site: billingSite}
			host.account.bearer, host.account.userID, host.account.expires = "old-session", 42, time.Now().Add(time.Hour).Unix()
			calls := 0
			host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				host.account.mu.Lock()
				host.account.userID = 43
				host.account.bearer = "new-session"
				host.account.mu.Unlock()
				return cloudFixture(`{"success":true,"data":{"learningEnabled":true}}`), nil
			})
			req := httptest.NewRequest(http.MethodPut, "http://localhost/runtime/preferences/settings", strings.NewReader(test.body))
			if test.local {
				req.Header.Set("X-ACU-Token", "local")
			}
			res := httptest.NewRecorder()
			host.accountPreferences(res, req)
			if res.Code != test.want || (!test.change && calls != 0) || strings.Contains(res.Body.String(), `"learningEnabled"`) {
				t.Fatalf("unsafe settings response: %d %s calls=%d", res.Code, res.Body.String(), calls)
			}
		})
	}
}

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
