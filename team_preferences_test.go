package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTeamPreferencesUseAccountSessionAndDoNotRevokeLoginForMembership(t *testing.T) {
	for _, status := range []int{200, 403} {
		host := &server{token: "local"}
		host.account.view = accountView{LoggedIn: true, Site: billingSite}
		host.account.bearer, host.account.userID, host.account.expires = "session", 42, time.Now().Add(time.Hour).Unix()
		host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/user/self/acu-team-preferences" || r.Header.Get("Authorization") != "Bearer session" || r.URL.RawQuery != "" {
				t.Fatal("team data must use authenticated account")
			}
			response := cloudFixture(`{"success":true,"data":{"version":1,"spaces":[]}}`)
			response.StatusCode = status
			return response, nil
		})
		request := httptest.NewRequest(http.MethodGet, "/runtime/preferences/teams?userId=99", nil)
		request.Header.Set("X-ACU-Token", "local")
		result := httptest.NewRecorder()
		host.accountPreferences(result, request)
		if result.Code != 200 || !host.account.view.LoggedIn {
			t.Fatalf("membership must not log out account: %d", result.Code)
		}
	}
}
