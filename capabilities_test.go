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
	prefs        map[string]any
	servers      []map[string]any
	writes       []string
	fail         bool
	searchKey    string
	refreshError bool
}

func capabilityTestHost(t *testing.T) (*server, *capabilityFixture) {
	t.Helper()
	f := &capabilityFixture{prefs: map[string]any{"gateway": "http://127.0.0.1:3425", "vision": "", "visionAuto": "acu-runtime/gpt-6.1-sol", "imageGen": "", "imageGenAuto": "acu-runtime/gpt-image-2.5-sunburst-2026-09-08", "theme": "dark", "searchAPIs": []map[string]any{{"vendor": "searxng", "ready": true}}, "visionModels": []map[string]string{{"id": "acu-runtime/gpt-6.1-sol", "name": "Sol"}}, "imageGenModels": []map[string]string{{"id": "acu-runtime/gpt-image-2.5-sunburst-2026-09-08", "name": "Sunburst"}}}, servers: []map[string]any{{"name": "external", "transport": "stdio", "command": "existing", "agents": []string{"codex"}}}}
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			f.writes = append(f.writes, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/provider/models":
			if f.refreshError {
				http.Error(w, "network unavailable", 502)
				return
			}
			f.prefs["imageGenModels"] = []map[string]string{{"id": "acu-runtime/gpt-image-2.5-sunburst-2026-09-08"}, {"id": "acu-runtime/gpt-image-2.6-sunburst"}, {"id": "acu-runtime/gpt-image-2.6-flare"}}
			io.WriteString(w, `{}`)
		case "/api/providers":
			io.WriteString(w, strings.Replace(setupProviders, `"models":[`, `"models":[{"id":"gpt-6.1-sol","on":true,"images":true},`, 1))
		case "/api/state":
			io.WriteString(w, `{"agents":[{"id":"codex"},{"id":"claude"},{"id":"hermes"},{"id":"workbuddy"}]}`)
		case "/api/settings":
			if r.Method == "POST" {
				json.NewDecoder(r.Body).Decode(&f.prefs)
			}
			json.NewEncoder(w).Encode(f.prefs)
		case "/api/library":
			json.NewEncoder(w).Encode(map[string]any{"servers": f.servers})
		case "/api/library/market/servers":
			io.WriteString(w, `{"items":[{"id":"magpie-vision","name":"magpie-vision"},{"id":"magpie-image","name":"magpie-image"},{"id":"magpie-web-search","name":"magpie-web-search"}]}`)
		case "/api/library/market/server":
			var in struct{ ID string }
			json.NewDecoder(r.Body).Decode(&in)
			kind := map[string]string{"magpie-vision": "vision", "magpie-image": "image", "magpie-web-search": "search"}[in.ID]
			f.servers = append(f.servers, map[string]any{"name": in.ID, "transport": "stdio", "command": "/native/magpie", "args": []string{"mcp", kind}, "agents": []string{}})
			io.WriteString(w, `{}`)
		case "/api/library/servers/agents":
			var in struct {
				Name   string
				Agents []string
			}
			json.NewDecoder(r.Body).Decode(&in)
			for _, s := range f.servers {
				if s["name"] == in.Name {
					s["agents"] = in.Agents
				}
			}
			io.WriteString(w, `{}`)
		case "/api/library/servers/remove":
			var in struct{ Name string }
			json.NewDecoder(r.Body).Decode(&in)
			for i, s := range f.servers {
				if s["name"] == in.Name {
					f.servers = append(f.servers[:i], f.servers[i+1:]...)
					break
				}
			}
			io.WriteString(w, `{}`)
		case "/api/library/skills/probe":
			io.WriteString(w, `{"candidates":[{"name":"acu-image-tools","path":"."}]}`)
		case "/api/library/skills/install":
			io.WriteString(w, `{}`)
		case "/api/provider/key":
			io.WriteString(w, `{"key":"private-acu-test-key"}`)
		case "/api/settings/search-api":
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if key, ok := in["key"].(string); ok {
				f.searchKey = key
				f.prefs["searchAPIs"] = []map[string]any{{"vendor": in["vendor"], "url": in["url"], "ready": true}}
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
	host.setKernelSetting = func(ctx context.Context, key, value string) error {
		f.writes = append(f.writes, "native-setting:"+key)
		f.prefs[key] = value
		return nil
	}
	host.imageSkillSource = func() (string, error) { return t.TempDir(), nil }
	return host, f
}

func TestCapabilityRefreshUpdatesExistingCatalogWithoutChangingPicks(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.prefs["imageGen"] = "acu-runtime/gpt-image-2.5-sunburst-2026-09-08"
	w := httptest.NewRecorder()
	host.refreshCapabilityModels(w, httptest.NewRequest("POST", "/runtime/capabilities/refresh", strings.NewReader(`{"provider":"acu-runtime"}`)))
	var view capabilityView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil || len(view.ImageModels) != 3 || view.ImageGen != f.prefs["imageGen"] {
		t.Fatalf("refresh did not preserve old choice and expose new models: %d %s", w.Code, w.Body.String())
	}
	if len(f.writes) != 1 || f.writes[0] != "/api/provider/models" {
		t.Fatalf("unexpected configuration writes %v", f.writes)
	}
}

func TestCapabilityRefreshFailureAndInvalidProviderPreserveExistingCatalog(t *testing.T) {
	for _, provider := range []string{"acu-runtime", "not-acu"} {
		t.Run(provider, func(t *testing.T) {
			host, f := capabilityTestHost(t)
			f.refreshError = true
			before, _ := json.Marshal(f.prefs)
			w := httptest.NewRecorder()
			host.refreshCapabilityModels(w, httptest.NewRequest("POST", "/runtime/capabilities/refresh", strings.NewReader(`{"provider":"`+provider+`"}`)))
			after, _ := json.Marshal(f.prefs)
			if w.Code != 400 || string(before) != string(after) {
				t.Fatalf("failed refresh changed existing settings: %d %s", w.Code, w.Body.String())
			}
			if provider == "not-acu" && len(f.writes) != 0 {
				t.Fatal("unknown provider refreshed")
			}
		})
	}
}
func TestCapabilityKernelDelegationAndIdempotence(t *testing.T) {
	host, f := capabilityTestHost(t)
	no := false
	in := capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}, FreeSearch: &no}
	view, err := host.capabilityApply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Vision != "" || view.ImageGen != "" || len(f.servers) != 4 {
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
	for _, kind := range []string{"conflict", "model", "agent", "provider", "url"} {
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
func TestCapabilityDoesNotExtractModelKeyForSearch(t *testing.T) {
	host, f := capabilityTestHost(t)
	url := "https://search.acu.best"
	response := httptest.NewRecorder()
	host.capabilities(response, httptest.NewRequest("POST", "/runtime/capabilities", strings.NewReader(`{"provider":"acu-runtime","agents":["codex"],"searchURL":"`+url+`"}`)))
	if response.Code != 400 || f.searchKey != "" || strings.Contains(response.Body.String(), "private-acu-test-key") {
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
func TestCapabilityDisabledSearchDoesNotInstallExtraRuntime(t *testing.T) {
	host, f := capabilityTestHost(t)
	// An explicit disabled source is retained; the extra free MCP remains available.
	f.prefs["searchAPIs"] = []map[string]any{{"vendor": "you", "off": true}}
	view, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if view.FreeSearchConfigured || view.SearchReady {
		t.Fatal("disabled native search overridden")
	}
	for _, s := range f.servers {
		if s["name"] == "acu-free-search" || s["name"] == "acu-search" {
			t.Fatal("extra search installed")
		}
	}
}

func TestCapabilityModelSaveDoesNotTouchTools(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.prefs["gateway"] = "unavailable"
	f.servers = append(f.servers, map[string]any{"name": "acu-vision", "note": "user-owned"})
	model := "acu-runtime/gpt-6.1-sol"
	_, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", ModelsOnly: true, Vision: &model})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 || f.writes[0] != "native-setting:vision" {
		t.Fatalf("model save rewrote tools: %v", f.writes)
	}
	if f.prefs["imageGen"] != "" {
		t.Fatal("filled an untouched model")
	}
}
func TestCapabilityModelSaveRejectsMixedOperations(t *testing.T) {
	host, f := capabilityTestHost(t)
	if _, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", ModelsOnly: true, Agents: []string{"codex"}}); err == nil {
		t.Fatal("accepted mixed save")
	}
	if len(f.writes) > 0 {
		t.Fatal("mutated on invalid request")
	}
}

func TestCapabilityPrefersNativeKeylessSearch(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.prefs["searchAPIs"] = []any{}
	view, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if !view.SearchReady || f.searchKey != "" {
		t.Fatal("keyless native search not ready")
	}
	for _, s := range f.servers {
		if s["name"] == "acu-free-search" {
			t.Fatal("installed redundant search MCP")
		}
	}
}
