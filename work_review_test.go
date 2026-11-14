package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWorkReviewAccountProxy(t *testing.T) {
	host := &server{token: "local"}
	host.account.view = accountView{LoggedIn: true, Site: billingSite}
	host.account.bearer = "session"
	host.account.userID = 122
	host.account.expires = time.Now().Add(time.Hour).Unix()
	host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/user/self/acu-work-review" || r.Header.Get("Authorization") != "Bearer session" || r.URL.RawQuery != "" {
			t.Fatalf("incorrect account request")
		}
		return cloudFixture(`{"success":true,"data":{"version":1,"items":[],"unavailable":[]}}`), nil
	})
	r := httptest.NewRequest("GET", "http://localhost/runtime/preferences/work-review?userId=999", nil)
	r.Header.Set("X-ACU-Token", "local")
	w := httptest.NewRecorder()
	host.accountPreferences(w, r)
	if w.Code != 200 {
		t.Fatalf("review response %d", w.Code)
	}
	r.Header.Del("X-ACU-Token")
	w = httptest.NewRecorder()
	host.accountPreferences(w, r)
	if w.Code != 403 {
		t.Fatal("missing local authentication accepted")
	}
}

func TestWorkReviewRejectsInvalidFeedback(t *testing.T) {
	host := &server{token: "local"}
	r := httptest.NewRequest("POST", "http://localhost/runtime/preferences/work-review", nil)
	r.Header.Set("X-ACU-Token", "local")
	w := httptest.NewRecorder()
	host.accountPreferences(w, r)
	if w.Code != 400 {
		t.Fatalf("invalid feedback endpoint: %d", w.Code)
	}
}
