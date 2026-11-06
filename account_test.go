package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type accountTransport func(*http.Request) (*http.Response, error)

func (transport accountTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func cloudFixture(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestACUOfficialAddressBoundary(t *testing.T) {
	for _, address := range []string{"http://acu.best", "https://acu.best.evil.test", "https://evilacu.best", "https://user@acu.best", "https://acu.best:1234", "https://acu.best/v1?key=secret", "https://acu.best/#secret", "https://acu.best/other"} {
		if _, err := acuBase(address); err == nil {
			t.Errorf("accepted unsafe ACU address %q", address)
		}
	}
	for address, expected := range map[string]string{"https://acu.best/v1/": "https://acu.best", "https://console.acucompute.com": "https://console.acucompute.com", "https://api.acu.best/acu/v1": "https://api.acu.best/acu"} {
		actual, err := acuBase(address)
		if err != nil || actual != expected {
			t.Errorf("normalization: %q, %v", actual, err)
		}
	}
	actual, err := accountSite("acucompute.com")
	if err != nil || actual != "https://console.acucompute.com" {
		t.Fatal("marketing domain was not mapped to the actual account console")
	}
	if _, err := accountSite("https://other.acu.best"); err == nil {
		t.Fatal("unexpected login origin")
	}
}

func TestConnectionRequiresAnEnabledOfficialProviderAndKey(t *testing.T) {
	providers := `[{"id":"unrelated","chat":"https://example.com/v1","key":{"set":true}}, {"id":"disabled","chat":"https://acu.best/v1","off":true,"key":{"set":true}}, {"id":"no-key","chat":"https://acu.best/v1","key":{"set":false}}]`
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		io.WriteString(response, `{"providers":`+providers+`}`)
	}))
	if view := host.connection(context.Background()); !view.Known || view.Configured {
		t.Fatalf("unrelated model availability counted as ACU: %+v", view)
	}
	providers = `[{"id":"existing-acu","responses":"https://acu.best/v1","key":{"set":true,"masked":"sk-***1234"},"models":[{"on":true},{"on":false}]}]`
	view := host.connection(context.Background())
	if !view.Configured || view.Provider != "existing-acu" || view.Models != 1 || view.Masked != "sk-***1234" {
		t.Fatalf("existing ACU connection was not reused: %+v", view)
	}
}

func TestManualConnectionPreparesThreeProtocolsWithoutInference(t *testing.T) {
	var writes []map[string]any
	var paths []string
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			io.WriteString(response, `{"providers":[]}`)
			return
		}
		if request.URL.Path != "/api/provider/save" {
			t.Errorf("unexpected kernel write: %s", request.URL.Path)
		}
		var input map[string]any
		json.NewDecoder(request.Body).Decode(&input)
		writes = append(writes, input)
		io.WriteString(response, `{}`)
	}))
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		if request.Header.Get("Authorization") != "Bearer fixture-model-key" {
			t.Error("incorrect API credential")
		}
		return cloudFixture(`{"data":[{"id":"acu-auto"},{"id":"gpt-6.1-sol"},{"id":"acu-auto"}]}`), nil
	})
	if err := host.saveACUConnection(context.Background(), "https://acu.best/v1", "fixture-model-key"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "GET /v1/models" || len(writes) != 1 {
		t.Fatalf("unexpected inference or write: %v, %v", paths, writes)
	}
	for field, expected := range map[string]string{"id": "acu-runtime", "chat": "https://acu.best/v1", "responses": "https://acu.best/v1", "anthropic": "https://acu.best", "modelsURL": "https://acu.best/v1/models"} {
		if writes[0][field] != expected {
			t.Errorf("%s = %v", field, writes[0][field])
		}
	}
	if len(writes[0]["models"].([]any)) != 2 {
		t.Fatal("duplicate models were not removed")
	}
}

func TestConnectionFailureNeverOverwritesExistingProviders(t *testing.T) {
	for _, test := range []struct{ name, models, providers string }{
		{"no-models", `{"data":[]}`, `[]`},
		{"invalid-key", `{"error":{"message":"private upstream detail"}}`, `[]`},
		{"provider-collision", `{"data":[{"id":"acu-auto"}]}`, `[{"id":"acu-runtime","chat":"https://unrelated.test/v1"}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Method != "GET" {
					t.Error("configuration was overwritten after a failure")
				}
				io.WriteString(response, `{"providers":`+test.providers+`}`)
			}))
			host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) { return cloudFixture(test.models), nil })
			if err := host.saveACUConnection(context.Background(), "https://acu.best", "fixture-key"); err == nil || strings.Contains(err.Error(), "private upstream detail") {
				t.Fatalf("missing or unsanitized error: %v", err)
			}
		})
	}
}

func TestNativeAccountBridgeOriginNonceExpiryAndWindow(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.account.ticket = loginTicket{Site: "https://acu.best", State: "one-use-test-nonce", Expires: time.Now().Add(time.Minute)}
	for _, input := range []struct{ window, sender, top, nonce string }{
		{"acu-main", "https://acu.best", "https://acu.best", "one-use-test-nonce"},
		{"acu-login-one-use-test-nonce", "https://evil.test", "https://acu.best", "one-use-test-nonce"},
		{"acu-login-one-use-test-nonce", "https://acu.best", "https://evil.test", "one-use-test-nonce"},
		{"acu-login-one-use-test-nonce", "https://acu.best.evil.test", "https://acu.best", "one-use-test-nonce"},
		{"acu-login-one-use-test-nonce", "", "", "one-use-test-nonce"},
		{"acu-login-one-use-test-nonce", "https://acu.best", "https://acu.best", "wrong-nonce"},
	} {
		message := `{"type":"acu-account-login","state":"` + input.nonce + `","accessToken":"fixture","expiresAt":` + jsonNumber(time.Now().Add(time.Minute).Unix()) + `}`
		if err := host.completeAccountLogin(context.Background(), input.window, input.sender, input.top, message); err == nil {
			t.Errorf("accepted invalid native message: %+v", input)
		}
		ready := `{"type":"acu-account-ready","state":"` + input.nonce + `"}`
		if err := host.acknowledgeAccountWindow(input.window, input.sender, input.top, ready); err == nil {
			t.Errorf("accepted invalid window handshake: %+v", input)
		}
	}
	if err := host.acknowledgeAccountWindow("acu-login-one-use-test-nonce", "https://acu.best/sign-in", "https://acu.best", `{"type":"acu-account-ready","state":"one-use-test-nonce"}`); err != nil || !host.account.snapshot().WindowReady || host.account.snapshot().LoggedIn {
		t.Fatal("window ready was not distinguished from account login")
	}
	request := httptest.NewRequest("POST", "http://"+host.host+"/runtime/account/complete", nil)
	request.Header.Set("Origin", "https://acu.best")
	response := httptest.NewRecorder()
	host.ServeHTTP(response, request)
	if response.Code != 403 || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("HTTP must not expose a cross-origin login exception")
	}
	host.account.ticket.Expires = time.Now().Add(-time.Second)
	if err := host.completeAccountLogin(context.Background(), "acu-login-one-use-test-nonce", "https://acu.best", "https://acu.best", `{}`); err == nil {
		t.Fatal("expired native message accepted")
	}
}

func TestAccountLoginAndSelectedKeyStayInNativeBackend(t *testing.T) {
	var savedKey string
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/provider/save" {
			var body struct {
				Key string `json:"key"`
			}
			json.NewDecoder(request.Body).Decode(&body)
			savedKey = body.Key
		}
		io.WriteString(response, `{"providers":[]}`)
	}))
	host.account.ticket = loginTicket{Site: "https://acu.best", State: "one-use-test-nonce", Expires: time.Now().Add(time.Minute)}
	var remotePaths []string
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		remotePaths = append(remotePaths, request.URL.Path)
		switch request.URL.Path {
		case "/api/user/self":
			return cloudFixture(`{"success":true,"data":{"id":42,"username":"Fixture account"}}`), nil
		case "/api/token/":
			return cloudFixture(`{"success":true,"data":{"total":3,"items":[{"id":1,"status":2},{"id":7,"name":"First usable","status":1,"expired_time":-1,"unlimited_quota":true},{"id":9,"name":"Other key","status":1,"remain_quota":100}]}}`), nil
		case "/api/token/9/key":
			return cloudFixture(`{"success":true,"data":{"key":"fixture-private-model-key"}}`), nil
		case "/api/token/7/key":
			return cloudFixture(`{"success":true,"data":{"key":"fixture-private-first-key"}}`), nil
		case "/v1/models":
			return cloudFixture(`{"data":[{"id":"acu-auto"}]}`), nil
		default:
			t.Errorf("unexpected remote request: %s", request.URL.Path)
			return cloudFixture(`{}`), nil
		}
	})
	body := `{"type":"acu-account-login","state":"one-use-test-nonce","accessToken":"fixture-private-session","expiresAt":` + jsonNumber(time.Now().Add(time.Minute).Unix()) + `}`
	err := host.completeAccountLogin(context.Background(), "acu-login-one-use-test-nonce", "https://acu.best/sign-in", "https://acu.best/console", body)
	if err != nil || !host.account.snapshot().LoggedIn || host.account.snapshot().Selected != 7 {
		t.Fatalf("login/default key failed: %v", err)
	}
	if savedKey != "fixture-private-first-key" {
		t.Fatal("first usable key was not automatically connected")
	}
	snapshot, _ := json.Marshal(host.account.snapshot())
	if strings.Contains(string(snapshot), "fixture-private") {
		t.Fatal("account credentials leaked into browser response")
	}
	if err := host.completeAccountLogin(context.Background(), "acu-login-one-use-test-nonce", "https://acu.best", "https://acu.best", body); err == nil {
		t.Fatal("login message replay accepted")
	}
	for _, keyID := range []string{"1", "999", "9"} {
		request := httptest.NewRequest("POST", "http://"+host.host+"/runtime/connection", strings.NewReader(`{"id":`+keyID+`}`))
		request.Header.Set("X-ACU-Token", host.token)
		response := httptest.NewRecorder()
		host.ServeHTTP(response, request)
		if keyID == "9" {
			if response.Code != 200 || savedKey != "fixture-private-model-key" || host.account.snapshot().Selected != 9 {
				t.Fatalf("selected Key was not configured: %d %s", response.Code, response.Body)
			}
		} else if response.Code != 400 || savedKey != "fixture-private-first-key" {
			t.Fatal("invalid/unavailable account Key was used")
		}
		if strings.Contains(response.Body.String(), "fixture-private") {
			t.Fatal("full API credential leaked to browser")
		}
	}
	if strings.Join(remotePaths, ",") != "/api/user/self,/api/token/,/api/token/7/key,/v1/models,/api/token/9/key,/v1/models" {
		t.Fatalf("unexpected key reveal/inference: %v", remotePaths)
	}
	host.account.expires = time.Now().Add(-time.Second).Unix()
	if !host.account.snapshot().LoggedIn || !host.account.snapshot().Refreshing {
		t.Fatal("short-lived access expiry must renew, not sign the user out")
	}
	before := len(remotePaths)
	if err := host.useAccountKey(context.Background(), 9); err == nil || len(remotePaths) != before {
		t.Fatal("expired access credential must not be sent upstream")
	}
}

func jsonNumber(value int64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestLoginRequestRequiresDesktopSessionAndCancellationWorks(t *testing.T) {
	host, _ := testServer(t, http.NotFoundHandler())
	host.desktop, host.loginRequests = true, make(chan loginTicket, 1)
	request := httptest.NewRequest("POST", "http://"+host.host+"/runtime/account/login", strings.NewReader(`{"site":"acucompute.com","register":true}`))
	request.Header.Set("X-ACU-Token", host.token)
	response := httptest.NewRecorder()
	host.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	ticket := <-host.loginRequests
	if !ticket.Register || ticket.Site != "https://console.acucompute.com" || !host.account.pending(ticket.State) {
		t.Fatalf("wrong native login ticket: %+v", ticket)
	}
	request = httptest.NewRequest("POST", "http://"+host.host+"/runtime/account/cancel", nil)
	request.Header.Set("X-ACU-Token", host.token)
	response = httptest.NewRecorder()
	host.ServeHTTP(response, request)
	if response.Code != 200 || host.account.pending(ticket.State) {
		t.Fatal("native login was not cancelled")
	}
}
