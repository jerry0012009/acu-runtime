package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNetworkCandidatesMatchProxyAndBypass(t *testing.T) {
	current := networkRoute{bestNetworkBase, ""}
	status := networkProxyStatus{"http://127.0.0.1:7897", "system"}
	system := func() (string, []string, bool) { return "127.0.0.1:7897", []string{"<local>"}, true }
	got := networkCandidates(current, status, system)
	if len(got) != 4 || got[0].route != (networkRoute{bestNetworkBase, "direct"}) || got[0].proxy != nil || got[1].route != current || got[1].proxy.String() != status.Address || got[2].route != (networkRoute{gaNetworkBase, "direct"}) || got[3].route != (networkRoute{gaNetworkBase, ""}) || got[3].proxy.String() != status.Address {
		t.Fatalf("unexpected candidate matrix: %#v", got)
	}
	got = networkCandidates(current, status, func() (string, []string, bool) {
		return "127.0.0.1:7897", []string{"*.acucompute.com"}, true
	})
	if len(got) != 3 || got[2].route != (networkRoute{gaNetworkBase, ""}) || got[2].proxy != nil {
		t.Fatalf("system bypass was ignored: %#v", got)
	}
	got = networkCandidates(current, networkProxyStatus{Source: "down"}, system)
	if len(got) != 2 || got[0].proxy != nil || got[1].proxy != nil || got[1].route.Proxy != "" {
		t.Fatalf("inactive proxy should produce only two distinct candidates: %#v", got)
	}
	got = networkCandidates(current, status, func() (string, []string, bool) { return "127.0.0.1:8080", nil, true })
	if len(got) != 0 {
		t.Fatal("changed proxy configuration should not be guessed")
	}
}

func TestNetworkChoiceKeepingHealthyIncumbent(t *testing.T) {
	current := networkRoute{bestNetworkBase, ""}
	other := networkRoute{gaNetworkBase, "direct"}
	for _, tc := range []struct {
		name string
		a, b time.Duration
		want networkRoute
		ok   bool
	}{
		{"all failed", 0, 0, current, false},
		{"incumbent failed", 0, time.Second, other, true},
		{"close absolute", 320 * time.Millisecond, 100 * time.Millisecond, current, true},
		{"close relative", 2 * time.Second, 1750 * time.Millisecond, current, true},
		{"clear improvement", 4 * time.Second, time.Second, other, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chosen, ok := chooseNetwork([]networkSample{{current, tc.a, tc.a != 0}, {other, tc.b, tc.b != 0}}, current)
			if chosen != tc.want || ok != tc.ok {
				t.Fatalf("chosen=%v available=%v", chosen, ok)
			}
		})
	}
}

const networkConfigFixture = `{"future":{"note":"keep"},"providers":[{"id":"other","key":"other-fixture","x":[3,1]},{"id":"acu-runtime","name":"ACU","key":"fixture-secret","chat":"https://acu.best/v1","responses":"https://acu.best/v1","anthropic":"https://acu.best","modelsURL":"https://acu.best/v1/models","socket":"off","models":["gpt-6.2-sol","claude-opus-5-5"],"delisted":["hidden"],"future":{"mode":true}}],"groups":[{"id":"example"}]}`

func networkFileFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(data, "providers.json")
	if err := os.WriteFile(path, []byte(networkConfigFixture), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	return path, filepath.Join(root, "fixture-gateway.exe")
}

func networkFixtureAPI(data []byte) []byte {
	var view struct {
		Providers []map[string]json.RawMessage `json:"providers"`
	}
	_ = json.Unmarshal(data, &view)
	for _, provider := range view.Providers {
		provider["key"] = json.RawMessage(`{"set":true}`)
		delete(provider, "models")
	}
	encoded, _ := json.Marshal(view)
	return encoded
}

func TestNetworkConfigPreservesProviderAndRollsBack(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "commit"
		if reject {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			path, executable := networkFileFixture(t)
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				switch request.URL.Path {
				case "/api/providers":
					data, err := os.ReadFile(path)
					if err != nil {
						t.Error(err)
					}
					if reject {
						data = []byte(networkConfigFixture)
					}
					_, _ = response.Write(networkFixtureAPI(data))
				case "/api/provider/key":
					_, _ = io.WriteString(response, `{"key":"fixture-secret"}`)
				default:
					t.Errorf("unexpected kernel call %s", request.URL.Path)
					response.WriteHeader(404)
				}
			}))
			host.kernel.exe, host.kernel.enforceVersion = executable, true
			host.networkFile = filepath.Join(filepath.Dir(executable), "prefs.json")
			ctx := context.Background()
			p, err := host.configuredProvider(ctx)
			if err != nil {
				t.Fatal(err)
			}
			entry, err := host.managedNetwork(p)
			if err != nil {
				t.Fatal(err)
			}
			next := networkRoute{gaNetworkBase, "direct"}
			err = host.applyNetworkConfig(ctx, entry, next, networkPrefs{"auto", next})
			written, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if reject {
				if err == nil || string(written) != networkConfigFixture || host.networkPrefs().Mode != "" {
					t.Fatal("failed verification must restore both route and preference")
				}
				return
			}
			if err != nil || host.networkPrefs() != (networkPrefs{"auto", next}) {
				t.Fatalf("write failed: %v", err)
			}
			var old, updated struct {
				Providers []map[string]any `json:"providers"`
				Future    map[string]any   `json:"future"`
				Groups    []any            `json:"groups"`
			}
			if json.Unmarshal([]byte(networkConfigFixture), &old) != nil || json.Unmarshal(written, &updated) != nil {
				t.Fatal("invalid configuration")
			}
			for field, expected := range map[string]string{"chat": gaNetworkBase + "/v1", "responses": gaNetworkBase + "/v1", "anthropic": gaNetworkBase, "modelsURL": gaNetworkBase + "/v1/models", "proxy": "direct"} {
				if updated.Providers[1][field] != expected {
					t.Errorf("wrong %s", field)
				}
				delete(updated.Providers[1], field)
				delete(old.Providers[1], field)
			}
			if !reflect.DeepEqual(old, updated) {
				t.Fatal("wrote other connection configuration")
			}
		})
	}
}

func TestNetworkStartupRunsOnceAndKeepsManual(t *testing.T) {
	for _, manual := range []bool{false, true} {
		name := "auto"
		if manual {
			name = "manual"
		}
		t.Run(name, func(t *testing.T) {
			_, executable := networkFileFixture(t)
			var keyReads, checks atomic.Int32
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/api/providers":
					_, _ = response.Write(networkFixtureAPI([]byte(networkConfigFixture)))
				case "/api/provider/key":
					keyReads.Add(1)
					_, _ = io.WriteString(response, `{"key":"fixture-secret"}`)
				case "/api/settings":
					checks.Add(1)
					_, _ = io.WriteString(response, `{"proxySource":"settings"}`)
				}
			}))
			host.kernel.exe, host.kernel.enforceVersion = executable, true
			host.networkFile = filepath.Join(filepath.Dir(executable), "prefs.json")
			if manual {
				if err := host.saveNetworkPrefs(networkPrefs{"manual", networkRoute{bestNetworkBase, ""}}); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			host.startNetworkCheck(ctx)
			host.startNetworkCheck(ctx)
			for {
				host.networkMu.Lock()
				checked := host.networkChecked
				host.networkMu.Unlock()
				if checked {
					break
				}
				if ctx.Err() != nil {
					t.Fatal("startup check did not finish")
				}
				time.Sleep(time.Millisecond)
			}
			var wantReads, wantChecks int32
			if !manual {
				wantReads, wantChecks = 2, 1
			}
			if keyReads.Load() != wantReads || checks.Load() != wantChecks {
				t.Fatalf("startup made %d key reads and %d checks, want %d reads and %d checks", keyReads.Load(), checks.Load(), wantReads, wantChecks)
			}
		})
	}
}

func TestStartupProbeLeavesConnectionUsableAndDefersToManualEdit(t *testing.T) {
	path, executable := networkFileFixture(t)
	probeStarted := make(chan struct{})
	finishProbe := make(chan struct{})
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/providers":
			_, _ = w.Write(networkFixtureAPI([]byte(networkConfigFixture)))
		case "/api/provider/key":
			_, _ = io.WriteString(w, `{"key":"fixture-secret"}`)
		case "/api/settings":
			close(probeStarted)
			select {
			case <-finishProbe:
			case <-r.Context().Done():
			}
			_, _ = io.WriteString(w, `{"proxySource":"settings"}`)
		}
	}))
	host.kernel.exe, host.kernel.enforceVersion = executable, true
	host.networkFile = filepath.Join(filepath.Dir(executable), "prefs.json")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	host.startNetworkCheck(ctx)
	select {
	case <-probeStarted:
	case <-ctx.Done():
		t.Fatal("probe did not start")
	}
	view := httptest.NewRecorder()
	host.networkSettings(view, httptest.NewRequest(http.MethodGet, "/runtime/network", nil))
	var status networkView
	if view.Code != 200 || json.Unmarshal(view.Body.Bytes(), &status) != nil || !status.Checking || !status.Managed || status.Route != (networkRoute{bestNetworkBase, ""}) {
		t.Fatalf("current connection unavailable during probe: %s", view.Body.String())
	}
	manual := httptest.NewRecorder()
	host.networkSettings(manual, httptest.NewRequest(http.MethodPost, "/runtime/network", strings.NewReader(`{"mode":"manual","route":{"base":"https://acu.best","proxy":""}}`)))
	close(finishProbe)
	if manual.Code != 200 {
		t.Fatalf("manual edit waited on the probe: %s", manual.Body.String())
	}
	for {
		host.networkMu.Lock()
		checking, prefs := host.networkChecking, host.networkPrefs()
		host.networkMu.Unlock()
		if !checking {
			if prefs.Mode != "manual" {
				t.Fatal("the background probe overwrote the manual edit")
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatal("probe did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != networkConfigFixture {
		t.Fatal("the existing provider changed")
	}
}
