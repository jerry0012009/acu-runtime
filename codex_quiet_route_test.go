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

func TestRouteSnapshotRestoresFilesAndRemovesOnlyNewFiles(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "config.toml")
	added := filepath.Join(root, "magpie-models.json")
	os.WriteFile(original, []byte("original"), 0600)
	snapshot := routeSnapshot{{path: original, data: []byte("original"), mode: 0600, exists: true}, {path: added}}
	os.WriteFile(original, []byte("changed"), 0600)
	os.WriteFile(added, []byte("new"), 0600)
	if err := snapshot.restore(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(original)
	if string(b) != "original" {
		t.Fatal("original config not restored")
	}
	if _, err := os.Stat(added); !os.IsNotExist(err) {
		t.Fatal("new catalog not removed")
	}
}
func TestQuietCodexNativeIsolation(t *testing.T) {
	if os.Getenv("ACU_QUIET_CODEX_SMOKE") != "1" {
		t.Skip("opt-in official kernel + installed Codex isolation test")
	}
	executable := os.Getenv("ACU_QUIET_KERNEL")
	if executable == "" {
		t.Fatal("explicit test kernel required")
	}
	signedInAuth := os.Getenv("ACU_QUIET_CHATGPT_AUTH")
	root := t.TempDir()
	for _, name := range []string{"USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "HERMES_HOME", "WORKBUDDY_CONFIG_DIR"} {
		p := filepath.Join(root, name)
		os.MkdirAll(p, 0700)
		t.Setenv(name, p)
	}
	codex := codexConfigDirectory()
	original := []byte("[mcp_servers.mine]\ncommand = \"preserved-test-marker\"\n")
	os.WriteFile(filepath.Join(codex, "config.toml"), original, 0600)
	os.MkdirAll(filepath.Join(os.Getenv("USERPROFILE"), ".codex", "packages", "standalone"), 0700)
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listen.Addr().String()
	listen.Close()
	t.Setenv("MAGPIE_ADDR", address)
	t.Setenv("ACU_KERNEL_PATH", executable)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected non-read request during configuration validation: %s", r.Method)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[{"id":"acu-auto","object":"model"},{"id":"second-model","object":"model"}]}`))
	}))
	defer remote.Close()
	k, err := newKernel("")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	defer func() {
		k.close()
		deadline := time.Now().Add(10 * time.Second)
		for {
			k.mu.Lock()
			gone := k.process == nil
			k.mu.Unlock()
			if gone {
				return
			}
			if time.Now().After(deadline) {
				t.Error("owned test gateway did not exit")
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	if err = k.start(ctx); err != nil {
		t.Fatal(err)
	}
	var response map[string]json.RawMessage
	if err = k.exchange(ctx, "POST", "/api/provider/save", map[string]any{"id": "acu-runtime", "name": "Isolation fixture", "new": true, "key": "not-a-real-key", "chat": remote.URL + "/v1", "models": []string{"acu-auto", "second-model"}}, &response); err != nil {
		t.Fatal(err)
	}
	first, err := readServingKernel(ctx)
	if err != nil {
		t.Fatal(err)
	}
	host := &server{kernel: k, desktop: true}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/kernel/api/set", strings.NewReader(`{"agent":"codex","field":"model","value":"acu-runtime/acu-auto"}`)).WithContext(ctx)
	host.proxy(w, req)
	if w.Code != 200 {
		t.Fatalf("advanced model proxy: %d %s", w.Code, w.Body.String())
	}
	var state map[string]json.RawMessage
	err = json.Unmarshal(w.Body.Bytes(), &state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state["agents"]), "acu-runtime/acu-auto") {
		t.Fatal("new native state did not confirm model")
	}
	second, _ := readServingKernel(ctx)
	if first.PID == second.PID {
		t.Fatal("route changed through a live watcher")
	}
	b, _ := os.ReadFile(filepath.Join(codex, "config.toml"))
	if !strings.Contains(string(b), "preserved-test-marker") {
		t.Fatal("existing MCP lost")
	}
	before := append([]byte(nil), b...)
	_, err = k.changeCodexQuietly(ctx, []codexFieldChange{{"model", "unregistered-model-does-not-exist"}}, false)
	if err == nil {
		t.Fatal("unloadable selection passed validation")
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if string(b) != string(before) {
		t.Fatal("failed validation did not roll back exact configuration")
	}
	if _, err = k.readState(ctx); err != nil {
		t.Fatal("gateway not recovered after rollback", err)
	}
	short, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	_, err = k.changeCodexQuietly(short, []codexFieldChange{{"model", "acu-runtime/second-model"}}, false)
	if err == nil {
		t.Fatal("canceled idle wait succeeded")
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if string(b) != string(before) {
		t.Fatal("canceled operation changed configuration")
	}
	interrupted, interrupt := context.WithCancel(ctx)
	k.codexValidate = func(context.Context, string) error { interrupt(); return context.Canceled }
	_, err = k.changeCodexQuietly(interrupted, []codexFieldChange{{"model", "acu-runtime/second-model"}}, false)
	k.codexValidate = nil
	if err == nil {
		t.Fatal("canceled post-write validation succeeded")
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if string(b) != string(before) {
		t.Fatal("cancellation after write did not restore original config")
	}
	if _, err = k.readState(ctx); err != nil {
		t.Fatal("gateway did not recover after request cancellation", err)
	}
	if _, err = k.changeCodexQuietly(ctx, nil, true); err != nil {
		t.Fatal("disconnect failed", err)
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if strings.Contains(string(b), "model_provider = \"magpie\"") {
		t.Fatal("disconnect did not restore native provider")
	}
	if !strings.Contains(string(b), "preserved-test-marker") {
		t.Fatal("disconnect lost original configuration")
	}
	if signedInAuth != "" {
		authData, e := os.ReadFile(signedInAuth)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(codex, "auth.json"), authData, 0600); e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(codex, "config.toml"), original, 0600)
		if _, err = k.changeCodexQuietly(ctx, []codexFieldChange{{"model", "acu-runtime/acu-auto"}}, false); err != nil {
			t.Fatal("signed-in native API configuration", err)
		}
		if err = validateInstalledCodex(ctx, "acu-runtime/acu-auto"); err != nil {
			t.Fatal("signed-in identity/readback verification", err)
		}
		afterAuth, _ := os.ReadFile(filepath.Join(codex, "auth.json"))
		var auth struct {
			Mode string `json:"auth_mode"`
		}
		json.Unmarshal(afterAuth, &auth)
		if auth.Mode != "chatgpt" {
			t.Fatal("native sign-in was replaced")
		}
		t.Log("PASS: signed-in account retained and selected ACU model loaded from native API-mode catalog")
	}
	t.Log("PASS: official kernel + installed Codex parse/model validation; watcher replacement, failure rollback, cancellation, disconnect, original MCP preservation; no GUI launch or inference")
}
