package main

import (
	"context"
	"encoding/json"
	"io"
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
	models := []string{"acu-auto", "gpt-6.2-sol", "gpt-6.2-astra", "claude-fable-5-1", "claude-fable-5-2", "gemini-4-carbon", "gemini-4-hydrogen", "gemini-4-argon"}
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
		if err := runCatalogCLI(ctx, executable, []string{"provider", "models", "acu-runtime", "+gpt-6.2-sol", "+gpt-6.2-astra", "+claude-fable-5-1", "+claude-fable-5-2", "+gemini-4-carbon", "+gemini-4-hydrogen", "+gemini-4-argon"}); err != nil {
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
		if err := k.exchange(ctx, "POST", "/api/agent-models/workbuddy", map[string]any{"shown": shown}, nil); err != nil {
			return err
		}
		host := &server{kernel: k}
		var before agentCatalog
		if err := k.exchange(ctx, "GET", "/api/agent-models/workbuddy", nil, &before); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"agent": "workbuddy", "expected": before, "operations": []map[string]any{
			{"only": true}, {"shown": []string{"acu-runtime/acu-auto", "acu-runtime/gemini-4-carbon"}},
			{"ownHidden": true}, {"order": []string{"acu-runtime/gemini-4-carbon", "acu-runtime/acu-auto"}},
		}})
		w := httptest.NewRecorder()
		host.syncAgentModels(w, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(string(body))))
		if w.Code != 200 {
			t.Fatalf("native batch: %d %s", w.Code, w.Body)
		}
		var batch agentCatalog
		if err := k.exchange(ctx, "GET", "/api/agent-models/workbuddy", nil, &batch); err != nil {
			return err
		}
		if !batch.Only || !batch.OwnHidden || len(batch.Models) == 0 || batch.Models[0].ID != "acu-runtime/gemini-4-carbon" {
			t.Fatalf("native batch not applied: %+v", batch)
		}
		for _, m := range batch.Models {
			if m.ID == "acu-runtime/claude-fable-5-2" && !m.Hidden {
				t.Fatal("unselected model still shown")
			}
		}
		for _, operation := range []map[string]any{{"only": false}, {"hidden": []string{}}, {"ownHidden": false}, {"order": []string{}}} {
			if err := k.exchange(ctx, "POST", "/api/agent-models/workbuddy", operation, nil); err != nil {
				return err
			}
		}
		defs := []struct{ name, path string }{{"acu-vision", "vision"}, {"acu-image", "image"}, {"acu-search", "web-search"}}
		for round := 0; round < 2; round++ {
			start := time.Now()
			if err := host.assignNativeTools(ctx, defs, []string{"workbuddy"}); err != nil {
				return err
			}
			t.Logf("official kernel native tool assignment round=%d elapsed=%dms", round+1, time.Since(start).Milliseconds())
		}
		return nil
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

func TestModelSyncPreservesEveryNativeInventoryOperation(t *testing.T) {
	for _, payload := range []string{
		`{"hidden":[]}`, `{"hidden":["acu-runtime/acu-auto"]}`,
		`{"shown":[]}`, `{"shown":["acu-runtime/acu-auto"]}`,
		`{"only":true}`, `{"only":false}`,
		`{"ownHidden":true}`, `{"ownHidden":false}`,
		`{"order":[]}`, `{"order":["acu-runtime/acu-auto"]}`,
	} {
		t.Run(payload, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/agent-models/codex" {
					t.Fatalf("unexpected native mutation: %s %s", r.Method, r.URL.Path)
				}
				var got, want map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				_ = json.Unmarshal([]byte(payload), &want)
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(want)
				if string(gotJSON) != string(wantJSON) {
					t.Fatalf("inventory operation changed: got %s want %s", gotJSON, wantJSON)
				}
				json.NewEncoder(w).Encode(agentCatalog{Only: true, OwnHidden: true, Models: []agentCatalogModel{{ID: "acu-runtime/acu-auto"}}})
			}))
			w := httptest.NewRecorder()
			body := `{"agent":"codex",` + payload[1:]
			host.syncAgentModels(w, httptest.NewRequest(http.MethodPost, "/runtime/agents/models", strings.NewReader(body)))
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"only":true`) {
				t.Fatalf("native result lost: status=%d body=%s", w.Code, w.Body)
			}
		})
	}
}

func TestModelBatchUsesNativeOperationsInOrderAndReadsBack(t *testing.T) {
	var writes []map[string]any
	reads := 0
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent-models/codex" {
			t.Fatal(r.URL.Path)
		}
		if r.Method == "GET" {
			reads++
			json.NewEncoder(w).Encode(agentCatalog{Models: []agentCatalogModel{}})
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		writes = append(writes, body)
		io.WriteString(w, `{}`)
	}))
	w := httptest.NewRecorder()
	host.syncAgentModels(w, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(`{"agent":"codex","expected":{"models":[]},"operations":[{"only":true},{"shown":[]},{"ownHidden":true},{"order":[]}]}`)))
	if w.Code != 200 || reads != 2 || len(writes) != 4 {
		t.Fatalf("status=%d reads=%d writes=%v body=%s", w.Code, reads, writes, w.Body)
	}
	for i, field := range []string{"only", "shown", "ownHidden", "order"} {
		if _, ok := writes[i][field]; !ok {
			t.Fatalf("operation %d lost %s", i, field)
		}
	}
}

func TestModelBatchRejectsConflictAndAmbiguousOperationsBeforeWrites(t *testing.T) {
	for _, body := range []string{
		`{"agent":"codex","expected":{"models":[],"only":true},"operations":[{"only":false}]}`,
		`{"agent":"codex","operations":[{"only":true,"hidden":[]}]}`,
		`{"agent":"codex","operations":[{}]}`,
	} {
		host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Fatal("conflicting batch wrote native settings")
			}
			io.WriteString(w, `{"models":[]}`)
		}))
		w := httptest.NewRecorder()
		host.syncAgentModels(w, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(body)))
		if w.Code != 400 && w.Code != 409 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
	}
}
