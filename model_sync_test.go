package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeCatalogManagementSync(t *testing.T) {
	executable := os.Getenv("ACU_MODEL_SYNC_KERNEL")
	if executable == "" {
		t.Skip("explicit official kernel isolation test")
	}
	root := t.TempDir()
	for _, name := range []string{"USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "HERMES_HOME", "WORKBUDDY_CONFIG_DIR"} {
		p := filepath.Join(root, name)
		_ = os.MkdirAll(p, 0700)
		t.Setenv(name, p)
	}
	t.Setenv("MAGPIE_NO_ACCOUNTS", "1")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	t.Setenv("MAGPIE_ADDR", address)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	models := []string{"acu-auto", "claude-fable-5-1", "claude-fable-5-2", "gemini-4-carbon", "gemini-4-hydrogen"}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("configuration sent a model request")
			w.WriteHeader(500)
			return
		}
		var rows []map[string]string
		for _, id := range models {
			rows = append(rows, map[string]string{"id": id, "object": "model"})
		}
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": rows})
	}))
	defer remote.Close()
	err = nativeCatalogManagement(ctx, executable, func(k *kernelState) error {
		if err := k.exchange(ctx, "POST", "/api/provider/save", map[string]any{"id": "acu-runtime", "name": "ACU", "new": true, "key": "fixture", "chat": remote.URL + "/v1", "models": []string{"acu-auto"}}, nil); err != nil {
			return err
		}
		if err := k.exchange(ctx, "POST", "/api/provider/models", map[string]string{"id": "acu-runtime"}, nil); err != nil {
			return err
		}
		var providers struct {
			Providers []syncProvider `json:"providers"`
		}
		if err := k.exchange(ctx, "GET", "/api/providers", nil, &providers); err != nil {
			return err
		}
		for _, p := range providers.Providers {
			for _, m := range p.Models {
				if m.ID == "claude-fable-5-2" && m.On {
					t.Error("refresh unexpectedly exposed unpicked model")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal("temporary gateway reservation leaked", err)
	}
	probe.Close()
	// The temporary controller uses native APIs and does not start a gateway.
	if _, err := readServingKernel(ctx); err == nil {
		t.Fatal("unexpected temporary gateway")
	}
	// Exercise native visibility, then verify its own persisted model catalog.
	err = nativeCatalogManagement(ctx, executable, func(k *kernelState) error {
		if err := runCatalogCLI(ctx, executable, []string{"provider", "models", "acu-runtime", "+claude-fable-5-1", "+claude-fable-5-2", "+gemini-4-carbon", "+gemini-4-hydrogen"}); err != nil {
			return err
		}
		var catalog agentCatalog
		if err := k.exchange(ctx, "GET", "/api/agent-models/workbuddy", nil, &catalog); err != nil {
			return err
		}
		if len(catalog.Models) < len(models) {
			t.Fatalf("native catalog missing recommendations: %+v", catalog)
		}
		shown := []string{}
		for _, m := range catalog.Models {
			shown = append(shown, m.ID)
		}
		if err := k.exchange(ctx, "POST", "/api/agents/connect/workbuddy", map[string]any{}, nil); err != nil {
			return err
		}
		return k.exchange(ctx, "POST", "/api/agent-models/workbuddy", map[string]any{"shown": shown}, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(os.Getenv("WORKBUDDY_CONFIG_DIR"), "models.json"))
	if err != nil {
		t.Fatal("native WorkBuddy file not written", err)
	}
	for _, id := range models {
		if !strings.Contains(string(b), id) {
			t.Errorf("WorkBuddy file missing %s", id)
		}
	}
}

func TestPrepareModelsRejectsUnknownOrForeignProviders(t *testing.T) {
	for _, address := range []string{"https://foreign.invalid/v1", "https://acu.best/v1"} {
		t.Run(address, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/providers" {
					t.Error("invalid selection reached mutation")
					w.WriteHeader(500)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"providers": []map[string]any{{"id": "actual", "chat": address}}})
			}))
			w := httptest.NewRecorder()
			host.prepareModels(w, httptest.NewRequest("POST", "/runtime/models/prepare", strings.NewReader(`{"provider":"missing","models":["claude-fable-5-2"]}`)))
			if w.Code != 409 {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestModelSyncUsesNativeVisibilityAndRejectsUnknownAgent(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent-models/workbuddy" {
			t.Fatal("unexpected kernel endpoint", r.URL.Path)
		}
		var body map[string][]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body["shown"]) != 1 || body["shown"][0] != "acu-runtime/claude-fable-5-2" {
			t.Error("visibility changed")
		}
		json.NewEncoder(w).Encode(agentCatalog{Models: []agentCatalogModel{{ID: "acu-runtime/claude-fable-5-2"}}})
	}))
	for _, agent := range []string{"workbuddy", "unsupported"} {
		w := httptest.NewRecorder()
		host.syncAgentModels(w, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(`{"agent":"`+agent+`","shown":["acu-runtime/claude-fable-5-2"]}`)))
		want := 200
		if agent == "unsupported" {
			want = 400
		}
		if w.Code != want {
			t.Fatalf("%s status=%d body=%s", agent, w.Code, w.Body)
		}
	}
}
