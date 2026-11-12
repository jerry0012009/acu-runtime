package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAccountRestoresIdentityWithoutReapplyingAnyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account-session.json")
	host := &server{accountFile: path}
	host.account.view = accountView{LoggedIn: true, Site: billingSite, Selected: 9}
	host.account.userID, host.account.sessionID, host.account.bearer = 42, "server-session", "never-persist-this-access-token"
	host.rememberAccount()
	encoded, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(encoded), "never-persist") {
		t.Fatalf("unsafe persistence: %s, %v", encoded, err)
	}
	restarted := &server{accountFile: path, loginRequests: make(chan loginTicket, 1)}
	restarted.restoreAccount()
	ticket := <-restarted.loginRequests
	if !ticket.Restore || ticket.SessionID != "server-session" || !restarted.account.snapshot().Restoring {
		t.Fatal("missing restoration metadata")
	}
	var paths []string
	restarted.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.Path)
		switch request.URL.Path {
		case "/api/user/self":
			return cloudFixture(`{"success":true,"data":{"id":42,"username":"restored"}}`), nil
		case "/api/token/":
			return cloudFixture(`{"success":true,"data":{"items":[{"id":7,"status":1,"unlimited_quota":true},{"id":9,"status":1,"unlimited_quota":true}]}}`), nil
		default:
			t.Fatalf("restoration must not reveal or save a Key: %s", request.URL.Path)
			return nil, nil
		}
	})
	message, _ := json.Marshal(map[string]any{"type": "acu-account-login", "state": ticket.State, "accessToken": "new-access", "expiresAt": time.Now().Add(time.Hour).Unix(), "sessionId": "server-session"})
	if err := restarted.completeAccountLogin(context.Background(), "acu-login-"+ticket.State, billingSite, billingSite, string(message)); err != nil {
		t.Fatal(err)
	}
	view := restarted.account.snapshot()
	if !view.LoggedIn || view.Selected != 9 || view.Restoring || len(paths) != 2 {
		t.Fatalf("restore result: %+v %v", view, paths)
	}
	restarted.forgetAccount()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("logout retained auto-login metadata")
	}
	if restarted.account.snapshot().LoggedIn {
		t.Fatal("logout retained authentication")
	}
}

func TestRestoreRejectsChangedAccountAndUntrustedFailure(t *testing.T) {
	host := &server{accountFile: filepath.Join(t.TempDir(), "account.json"), loginRequests: make(chan loginTicket, 1)}
	host.account.view = accountView{LoggedIn: true, Site: billingSite, Selected: 9}
	host.account.userID, host.account.sessionID = 42, "old-session"
	host.rememberAccount()
	host.restoreAccount()
	ticket := <-host.loginRequests
	if err := host.accountRestoreError("acu-login-"+ticket.State, "https://evil.test", billingSite, `{"state":"`+ticket.State+`","expired":true}`); err == nil {
		t.Fatal("untrusted window cleared login")
	}
	if !host.account.snapshot().Pending {
		t.Fatal("untrusted failure changed login")
	}
	host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) {
		return cloudFixture(`{"success":true,"data":{"id":43,"username":"different"}}`), nil
	})
	message, _ := json.Marshal(map[string]any{"type": "acu-account-login", "state": ticket.State, "accessToken": "new-access", "expiresAt": time.Now().Add(time.Hour).Unix(), "sessionId": "old-session"})
	if err := host.completeAccountLogin(context.Background(), "acu-login-"+ticket.State, billingSite, billingSite, string(message)); err == nil {
		t.Fatal("changed account restored")
	}
	if host.account.snapshot().LoggedIn {
		t.Fatal("changed identity authenticated")
	}
}

func TestBillingUsesCloudHistoricalAmountsAndSanitizesLogs(t *testing.T) {
	host := &server{}
	host.account.view = accountView{LoggedIn: true, Site: billingSite, Username: "fixture", Selected: 9}
	host.account.userID, host.account.bearer, host.account.expires = 42, "fixture-bearer", time.Now().Add(time.Hour).Unix()
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/status" && request.Header.Get("Authorization") != "Bearer fixture-bearer" {
			t.Fatal("missing authentication")
		}
		switch request.URL.Path {
		case "/api/status":
			return cloudFixture(`{"success":true,"data":{"quota_per_unit":500000,"usd_exchange_rate":6.74,"price":7.3}}`), nil
		case "/api/user/self":
			return cloudFixture(`{"success":true,"data":{"id":42,"quota":100000}}`), nil
		case "/api/token/9":
			return cloudFixture(`{"success":true,"data":{"id":9,"name":"fixture","remain_quota":200,"unlimited_quota":true,"key":"must-not-leak"}}`), nil
		case "/api/log/self/stat":
			if request.URL.Query().Get("type") != "2" || request.URL.Query().Get("token_name") != "" {
				t.Fatal("incorrect history scope")
			}
			return cloudFixture(`{"success":true,"data":{"user_charge_cny":"0.000000000123456789","exact_charge_count":1,"legacy_charge_count":0}}`), nil
		case "/api/log/self":
			if request.URL.Query().Get("p") != "2" || request.URL.Query().Get("page_size") != "20" {
				t.Fatal("pagination lost")
			}
			return cloudFixture(`{"success":true,"data":{"total":21,"items":[{"id":21,"model_name":"acu-auto","quota":9000000,"other":"{\"user_charge_cny\":\"0.123456789\",\"acu_cost_breakdown\":{\"retail_price_version\":\"old-price-version\"},\"private_info\":\"must-not-leak\"}","content":"must-not-leak"}]}}`), nil
		default:
			t.Fatal("unexpected endpoint: " + request.URL.Path)
			return nil, nil
		}
	})
	response := httptest.NewRecorder()
	host.accountBilling(response, httptest.NewRequest("GET", "/runtime/billing/account?days=30&page=2", nil))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{"0.000000000123456789", "0.123456789", "old-price-version", "unlimited_quota", "0.00001348"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, "must-not-leak") || strings.Contains(body, "Bearer") {
		t.Fatal("billing leaked private data")
	}
}

func TestBillingWithoutAccountMakesNoCloudRequest(t *testing.T) {
	host := &server{cloudTransport: accountTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("unauthenticated billing made cloud request")
		return nil, nil
	})}
	response := httptest.NewRecorder()
	host.accountBilling(response, httptest.NewRequest("GET", "/runtime/billing/account", nil))
	if !strings.Contains(response.Body.String(), `"loggedIn":false`) {
		t.Fatal(response.Body.String())
	}
}

func TestPriceRefreshFailureRetainsLastSuccessfulCatalog(t *testing.T) {
	host := &server{}
	calls := 0
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Path != "/api/pricing" {
			t.Fatal(request.URL.Path)
		}
		if calls == 1 {
			return cloudFixture(`{"success":true,"data":[{"model_name":"fixture","payable":{"input_cny_per_million":0}}]}`), nil
		}
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})
	for _, path := range []string{"/runtime/billing/prices", "/runtime/billing/prices", "/runtime/billing/prices?refresh=1"} {
		response := httptest.NewRecorder()
		host.modelPrices(response, httptest.NewRequest("GET", path, nil))
		if !strings.Contains(response.Body.String(), "fixture") {
			t.Fatal("lost cached price")
		}
		if strings.Contains(path, "refresh") && !strings.Contains(response.Body.String(), "503") {
			t.Fatal("failure not reported")
		}
	}
	if calls != 2 {
		t.Fatal("catalog cache not reused")
	}
}
