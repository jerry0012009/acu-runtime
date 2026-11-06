package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const setupProviders = `{"providers":[{"id":"acu-runtime","name":"ACU","chat":"https://acu.best/v1","key":{"set":true,"masked":"sk-fixture"},"models":[{"id":"acu-auto","on":true}]}]}`

func setupRequest(host *server, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	host.configureAgent(response, httptest.NewRequest("POST", "/runtime/agents/configure", strings.NewReader(body)))
	return response
}

func modelProbe(t *testing.T, status int) accountTransport {
	t.Helper()
	return accountTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != "GET" || request.URL.String() != "https://acu.best/v1/models" || request.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Errorf("unexpected model probe: %s %s", request.Method, request.URL)
		}
		response := cloudFixture(`{"data":[{"id":"acu-auto"}]}`)
		response.StatusCode = status
		return response, nil
	})
}

func TestProviderProbeRejectsRemovedKeysWithoutRevealingOrWriting(t *testing.T) {
	for _, providers := range []string{`{"providers":[]}`, strings.Replace(setupProviders, `"set":true`, `"set":false`, 1), strings.Replace(setupProviders, `"name":"ACU"`, `"name":"ACU","off":true`, 1)} {
		host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Method != "GET" || request.URL.Path != "/api/providers" {
				t.Errorf("removed key triggered a reveal or write: %s", request.URL.Path)
			}
			io.WriteString(response, providers)
		}))
		host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) {
			t.Error("removed key reached the cloud")
			return nil, errors.New("unexpected")
		})
		if err := host.checkProvider(context.Background(), "acu-runtime"); err == nil {
			t.Fatal("removed key was accepted")
		}
	}
}

func TestProviderHealthDistinguishesRevocationFromNetworkAndCloudflare(t *testing.T) {
	for _, current := range []struct {
		name   string
		status int
		state  string
	}{
		{"valid", 200, "valid"}, {"revoked", 401, "invalid"}, {"cloudflare", 403, "unknown"}, {"rate-limit", 429, "unknown"}, {"network", 0, "unknown"},
	} {
		t.Run(current.name, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/api/providers":
					io.WriteString(response, setupProviders)
				case "/api/provider/key":
					io.WriteString(response, `{"key":"fixture-secret"}`)
				default:
					t.Errorf("unexpected kernel request: %s", request.URL.Path)
				}
			}))
			host.cloudTransport = modelProbe(t, current.status)
			if current.status == 0 {
				host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
			}
			view := host.connection(context.Background())
			host.connectionHealth(view)
			deadline := time.Now().Add(3 * time.Second)
			for {
				host.health.mu.Lock()
				state, checking := host.health.state, host.health.checking
				host.health.mu.Unlock()
				if !checking {
					if state != current.state {
						t.Fatalf("health = %s, want %s", state, current.state)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("health check did not finish")
				}
				time.Sleep(time.Millisecond)
			}
			if got := host.connectionHealth(view); got.Configured != (current.state != "invalid") {
				t.Fatalf("invalid saved key still appeared connected: %+v", got)
			}
			host.health.mu.Lock()
			host.health.state = "invalid"
			host.health.mu.Unlock()
			view.Masked = "sk-new-key"
			host.health.mu.Lock()
			host.health.checking = true
			host.health.mu.Unlock()
			if !host.connectionHealth(view).Configured {
				t.Fatal("old revocation was applied to a replacement key")
			}
		})
	}
}

func TestAgentSetupRejectsInvalidSelectionBeforeWrites(t *testing.T) {
	for _, body := range []string{
		`{"agent":"unknown","provider":"acu-runtime","model":"acu-auto"}`,
		`{"agent":"codex","provider":"deleted","model":"acu-auto"}`,
		`{"agent":"codex","provider":"acu-runtime","model":"not-enabled"}`,
	} {
		host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Method != "GET" || request.URL.Path != "/api/providers" {
				t.Errorf("invalid selection triggered a write: %s", request.URL.Path)
			}
			io.WriteString(response, setupProviders)
		}))
		if response := setupRequest(host, body); response.Code != 400 {
			t.Fatalf("invalid selection accepted: %s", response.Body)
		}
	}
}

func TestAgentSetupUsesExactNativeModelAndPreservesOtherSettings(t *testing.T) {
	for _, current := range []struct {
		name, agent                                             string
		wired, auxiliary, unsupported, failed, ignored, revoked bool
	}{
		{name: "main", agent: "codex"},
		{name: "auxiliary", agent: "claude", auxiliary: true},
		{name: "existing", agent: "codex", wired: true},
		{name: "workbuddy", agent: "workbuddy"},
		{name: "existing-workbuddy", agent: "workbuddy", wired: true},
		{name: "unsupported-model", agent: "hermes", unsupported: true},
		{name: "partial-failure", agent: "codex", failed: true},
		{name: "ignored-model-write", agent: "codex", ignored: true},
		{name: "revoked-key", agent: "codex", revoked: true},
	} {
		t.Run(current.name, func(t *testing.T) {
			wired := current.wired
			selected := "other-provider/native-model"
			var writes []map[string]string
			var connects int
			fields := []setupField{{Key: "model", Options: []setupOption{{Value: "exact-native-model", Ref: "acu-runtime/acu-auto"}, {Value: "other-native", Ref: "other/acu-auto"}}}, {Key: "subagent_model", Options: []setupOption{{Value: "exact-native-aux", Ref: "acu-runtime/acu-auto"}}}}
			if current.agent == "workbuddy" || current.unsupported {
				fields = nil
			}
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/api/providers":
					io.WriteString(response, setupProviders)
					return
				case "/api/provider/key":
					io.WriteString(response, `{"key":"fixture-secret"}`)
					return
				case "/api/agents/connect/" + current.agent:
					connects++
					wired = true
				case "/api/set":
					var input map[string]string
					json.NewDecoder(request.Body).Decode(&input)
					writes = append(writes, input)
					if current.failed {
						response.WriteHeader(500)
						return
					}
					if input["field"] == "model" && !current.ignored {
						selected = input["value"]
					}
				case "/api/state":
				default:
					t.Errorf("unexpected Agent request: %s", request.URL.Path)
				}
				for index := range fields {
					if fields[index].Key == "model" {
						fields[index].Value = selected
					}
				}
				json.NewEncoder(response).Encode(map[string]any{"agents": []setupAgent{{ID: current.agent, Wired: wired, Fields: fields}}})
			}))
			host.cloudTransport = modelProbe(t, 200)
			if current.revoked {
				host.cloudTransport = modelProbe(t, 401)
			}
			body, _ := json.Marshal(map[string]any{"agent": current.agent, "provider": "acu-runtime", "model": "acu-auto", "auxiliary": current.auxiliary})
			response := setupRequest(host, string(body))
			if current.unsupported || current.revoked {
				if response.Code != 400 || len(writes) != 0 || connects != 0 {
					t.Fatalf("preflight failure changed Agent: %s, writes %v", response.Body, writes)
				}
				return
			}
			if current.failed || current.ignored {
				if response.Code != 400 || !strings.Contains(response.Body.String(), "部分配置") {
					t.Fatalf("partial write was claimed successful: %s", response.Body)
				}
				return
			}
			if response.Code != 200 || strings.Contains(response.Body.String(), "fixture-secret") {
				t.Fatalf("setup failed or leaked key: %s", response.Body)
			}
			if (connects == 0) != current.wired || connects > 1 {
				t.Fatalf("unexpected reconnect count: %d", connects)
			}
			expected := 1
			if current.auxiliary {
				expected++
			}
			if current.agent == "workbuddy" {
				expected = 0
			}
			if len(writes) != expected {
				t.Fatalf("unexpected field writes: %+v", writes)
			}
			for _, write := range writes {
				if write["agent"] != current.agent || write["field"] == "model" && write["value"] != "exact-native-model" || write["field"] == "subagent_model" && write["value"] != "exact-native-aux" {
					t.Fatalf("did not use the selected provider's native option: %+v", write)
				}
			}
		})
	}
}
