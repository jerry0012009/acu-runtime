package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capabilityFixture struct {
	prefs     map[string]any
	servers   []map[string]any
	writes    []string
	fail      bool
	searchKey string
}

func capabilityTestHost(t *testing.T) (*server, *capabilityFixture) {
	t.Helper()
	f := &capabilityFixture{prefs: map[string]any{"gateway": "http://127.0.0.1:3425", "vision": "", "imageGen": "", "theme": "dark", "searchAPIs": []map[string]any{{"vendor": "searxng", "ready": true}}, "visionModels": []map[string]string{{"id": "acu-runtime/gpt-6.1-sol", "name": "Sol"}}, "imageGenModels": []map[string]string{{"id": "acu-runtime/gpt-image-2.5-sunburst-2026-09-08", "name": "Sunburst"}}}, servers: []map[string]any{{"name": "external", "transport": "stdio", "command": "existing", "agents": []string{"codex"}}}}
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			f.writes = append(f.writes, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/providers":
			io.WriteString(w, strings.Replace(setupProviders, `"models":[`, `"models":[{"id":"gpt-6.1-sol","on":true},`, 1))
		case "/api/state":
			io.WriteString(w, `{"agents":[{"id":"codex"},{"id":"claude"},{"id":"hermes"},{"id":"workbuddy"}]}`)
		case "/api/settings":
			if r.Method == "POST" {
				json.NewDecoder(r.Body).Decode(&f.prefs)
			}
			json.NewEncoder(w).Encode(f.prefs)
		case "/api/library":
			json.NewEncoder(w).Encode(map[string]any{"servers": f.servers})
		case "/api/provider/key":
			io.WriteString(w, `{"key":"private-acu-test-key"}`)
		case "/api/settings/search-api":
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if key, ok := in["key"].(string); ok {
				f.searchKey = key
				f.prefs["searchAPIs"] = []map[string]any{{"vendor": "searxng", "url": in["url"], "ready": true}}
			}
			io.WriteString(w, `{}`)
		case "/api/library/servers/save":
			var in struct {
				Old    string
				Server map[string]any
			}
			json.NewDecoder(r.Body).Decode(&in)
			found := false
			for i, s := range f.servers {
				if s["name"] == in.Old {
					f.servers[i] = in.Server
					found = true
				}
			}
			if !found {
				f.servers = append(f.servers, in.Server)
			}
			if f.fail {
				io.WriteString(w, `{"result":{"problems":[{"agent":"codex","error":"fixture"}]}}`)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"servers": f.servers})
			}
		default:
			t.Errorf("unexpected kernel API: %s", r.URL.Path)
			http.Error(w, "unexpected", 404)
		}
	}))
	host.prepareSearch = func(context.Context, string) error { return nil }
	return host, f
}
func TestCapabilityKernelDelegationAndIdempotence(t *testing.T) {
	host, f := capabilityTestHost(t)
	no := false
	in := capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}, FreeSearch: &no}
	view, err := host.capabilityApply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Vision != "acu-runtime/gpt-6.1-sol" || !strings.Contains(view.ImageGen, "sunburst") || len(f.servers) != 4 {
		t.Fatalf("recommendations missing: %+v", view)
	}
	f.prefs["vision"] = "off"
	f.prefs["imageGen"] = "native/user-choice"
	f.servers[2]["disabledTools"] = []string{"generate_image"}
	f.servers[2]["hub"] = "direct"
	in.Agents = []string{"claude"}
	if _, err = host.capabilityApply(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(f.servers) != 4 || f.prefs["vision"] != "off" || f.prefs["imageGen"] != "native/user-choice" || f.prefs["theme"] != "dark" || f.servers[2]["hub"] != "direct" {
		t.Fatal("overwrote existing choices")
	}
	b, _ := json.Marshal(f.servers[2])
	if !strings.Contains(string(b), `"agents":["codex","claude"]`) || !strings.Contains(string(b), `"disabledTools":["generate_image"]`) {
		t.Fatalf("lost options: %s", b)
	}
	if f.servers[0]["command"] != "existing" {
		t.Fatal("overwrote external MCP")
	}
	for _, path := range f.writes {
		if path == "/api/library/servers/hub" {
			t.Fatal("changed global hub")
		}
	}
}
func TestCapabilityValidationBeforeMutation(t *testing.T) {
	for _, kind := range []string{"conflict", "model", "agent", "provider", "url", "gateway"} {
		t.Run(kind, func(t *testing.T) {
			host, f := capabilityTestHost(t)
			in := capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}}
			switch kind {
			case "conflict":
				f.servers = append(f.servers, map[string]any{"name": "acu-vision", "note": "user-owned"})
			case "model":
				value := "other/unknown"
				in.Vision = &value
			case "agent":
				in.Agents = []string{"unknown"}
			case "provider":
				in.Provider = "other"
			case "url":
				value := "https://acu.best.evil.test"
				in.SearchURL = &value
			case "gateway":
				f.prefs["gateway"] = "http://evil.test"
			}
			if _, err := host.capabilityApply(context.Background(), in); err == nil {
				t.Fatal("accepted invalid config")
			}
			if len(f.writes) > 0 {
				t.Fatalf("mutated before validation: %v", f.writes)
			}
		})
	}
}
func TestCapabilitySearchKeyStaysInKernel(t *testing.T) {
	host, f := capabilityTestHost(t)
	url := "https://search.acu.best"
	response := httptest.NewRecorder()
	host.capabilities(response, httptest.NewRequest("POST", "/runtime/capabilities", strings.NewReader(`{"provider":"acu-runtime","agents":["codex"],"searchURL":"`+url+`"}`)))
	if response.Code != 200 || f.searchKey != "private-acu-test-key" || strings.Contains(response.Body.String(), "private-acu-test-key") {
		t.Fatalf("search delegation failed: %d", response.Code)
	}
}
func TestCapabilityPartialFailure(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.fail = true
	if _, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}}); err == nil || !strings.Contains(err.Error(), "写入失败") {
		t.Fatalf("false success: %v", err)
	}
}
func TestCapabilitySearchURLBoundary(t *testing.T) {
	for _, url := range []string{"http://search.acu.best", "https://acu.best.evil.test", "https://acu.best@evil.test", "https://search.acu.best:8443", "https://search.acu.best?key=x", "https://127.0.0.1"} {
		if validateSearchURL(url) == nil {
			t.Errorf("accepted %s", url)
		}
	}
	for _, url := range []string{"https://search.acu.best", "https://search.acucompute.com/api", ""} {
		if err := validateSearchURL(url); err != nil {
			t.Error(err)
		}
	}
}
func TestCapabilityFreeSearchUsesNoSecrets(t *testing.T) {
	if freeSearchCommand() == "" {
		t.Skip("uvx not installed")
	}
	host, f := capabilityTestHost(t)
	f.prefs["searchAPIs"] = []any{}
	view, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if !view.FreeSearchConfigured {
		t.Fatal("free search missing")
	}
	for _, s := range f.servers {
		if s["name"] == "acu-free-search" {
			b, _ := json.Marshal(s)
			if s["transport"] != "stdio" || strings.Contains(string(b), "private-acu") || !strings.Contains(string(b), "==0.7.0") {
				t.Fatalf("incorrect free search: %s", b)
			}
		}
	}
}
