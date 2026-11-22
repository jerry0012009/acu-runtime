package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRuntimeRedeemUsesCurrentDashboardSessionOnce(t *testing.T) {
	for _, tc := range []struct {
		name         string
		localToken   string
		staleAccount bool
		cloudStatus  int
		cloudBody    string
		wantStatus   int
		wantText     string
	}{
		{"missing local session", "", false, 0, "", 403, "invalid local session"},
		{"account changed", "local", true, 0, "", 409, "登录账户已变化"},
		{"redeemed", "local", false, 200, `{"success":true,"data":500000}`, 200, `"addedQuota":500000`},
		{"rejected", "local", false, 200, `{"success":false,"message":"fixture-code"}`, 422, "兑换未成功"},
		{"redirect refused", "local", false, 307, "", 502, "无法确认兑换结果"},
		{"rate limited", "local", false, 429, "", 429, "提交过于频繁"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &server{host: "runtime.test", token: "local"}
			host.account.view = accountView{LoggedIn: true, Site: "https://acu.best"}
			host.account.userID, host.account.bearer, host.account.expires = 42, "fixture-session", time.Now().Add(time.Hour).Unix()
			identity := host.account.snapshot().Identity
			if tc.staleAccount {
				host.account.userID++
			}
			calls := 0
			host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.String() != "https://acu.best/api/user/topup" || r.Header.Get("Authorization") != "Bearer fixture-session" {
					t.Fatalf("unexpected cloud request: %s %s", r.Method, r.URL)
				}
				var body map[string]string
				if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 1 || body["key"] != "fixture-code" {
					t.Fatal("cloud request did not receive only the trimmed code")
				}
				res := cloudFixture(tc.cloudBody)
				res.StatusCode = tc.cloudStatus
				res.Header.Set("Location", "https://acu.best/other")
				return res, nil
			})
			request := httptest.NewRequest(http.MethodPost, "http://runtime.test/runtime/billing/redeem", strings.NewReader(`{"code":" fixture-code ","identity":"`+identity+`"}`))
			request.Header.Set("X-ACU-Token", tc.localToken)
			out := httptest.NewRecorder()
			host.ServeHTTP(out, request)
			if out.Code != tc.wantStatus || !strings.Contains(out.Body.String(), tc.wantText) || strings.Contains(out.Body.String(), "fixture-") {
				t.Fatalf("unexpected response %d %s", out.Code, out.Body)
			}
			wantCalls := 1
			if tc.cloudStatus == 0 {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("cloud calls: got %d, want %d", calls, wantCalls)
			}
		})
	}
}
