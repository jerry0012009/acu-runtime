package main

import (
	"context"
	"encoding/json"
	"errors"
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

func TestCodexCommandOutcomeRequiresExactFinalState(t *testing.T) {
	for _, current := range []struct {
		name, state         string
		disconnect, success bool
	}{
		{"persisted", `{"agents":[{"id":"codex","wired":true,"fields":[{"key":"model","value":"acu/gpt-6.2-sol"}]}]}`, false, true},
		{"unchanged", `{"agents":[{"id":"codex","wired":true,"fields":[{"key":"model","value":"acu/gpt-6.1-sol"}]}]}`, false, false},
		{"drift", `{"agents":[{"id":"codex","wired":true,"drift":{"kind":"unwired"},"fields":[{"key":"model","value":"acu/gpt-6.2-sol"}]}]}`, false, false},
		{"disconnected", `{"agents":[{"id":"codex","wired":false}]}`, true, true},
		{"still-connected", `{"agents":[{"id":"codex","wired":true}]}`, true, false},
	} {
		t.Run(current.name, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, current.state)
			}))
			changes := []codexFieldChange{{"model", "acu/gpt-6.2-sol"}}
			if current.disconnect {
				changes = nil
			}
			err := verifyCodexCommandOutcome(context.Background(), host.kernel, changes, current.disconnect)
			if (err == nil) != current.success {
				t.Fatalf("unexpected final outcome: %v", err)
			}
			if err := verifyCodexCommandOutcome(context.Background(), host.kernel, nil, false); err == nil {
				t.Fatal("accepted unspecified mutation")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := verifyCodexCommandOutcome(ctx, host.kernel, changes, current.disconnect); err == nil {
				t.Fatal("ignored cancellation")
			}
		})
	}
	err := codexRollbackCompleted(errors.New("Codex 本次修改未完成"))
	if strings.Contains(err.Error(), "正在") || !strings.Contains(err.Error(), "本次修改未生效") || !strings.Contains(err.Error(), "已恢复此前配置") {
		t.Fatalf("ambiguous final recovery message: %v", err)
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
	probeExecutable := os.Getenv("ACU_QUIET_CODEX_EXE")
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
	if probeExecutable != "" {
		k.codexValidate = func(ctx context.Context, model string) error {
			return probeCodexConfiguration(ctx, probeExecutable, model)
		}
	}
	baseValidate := k.codexValidate
	k.codexValidate = func(ctx context.Context, model string) error {
		// The signed-in route has no local catalog: its model list is
		// fetched from the gateway, which must be running during the probe.
		serving, err := readServingKernel(ctx)
		if err != nil {
			return err
		}
		k.mu.Lock()
		command := k.process
		k.mu.Unlock()
		if command == nil || command.Process == nil || uint32(command.Process.Pid) != serving.PID {
			t.Error("Codex validation ran without the owned gateway")
			return context.Canceled
		}
		if baseValidate != nil {
			return baseValidate(ctx, model)
		}
		return validateInstalledCodex(ctx, model)
	}
	defaultValidate := k.codexValidate
	validationCalls := 0
	k.codexValidate = func(ctx context.Context, model string) error {
		validationCalls++
		return defaultValidate(ctx, model)
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
	configureStarted := time.Now()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/kernel/api/set", strings.NewReader(`{"agent":"codex","field":"model","value":"acu-runtime/acu-auto"}`)).WithContext(ctx)
	if probeExecutable != "" {
		state, e := k.changeCodexQuietly(ctx, []codexFieldChange{{"model", "acu-runtime/acu-auto"}}, false)
		if e != nil {
			t.Fatal(e)
		}
		host.json(w, 200, state)
	} else {
		host.proxy(w, req)
	}
	if w.Code != 200 {
		t.Fatalf("advanced model proxy: %d %s", w.Code, w.Body.String())
	}
	t.Logf("Verified initial Codex configuration in %s", time.Since(configureStarted).Round(time.Millisecond))
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
	if !strings.Contains(string(b), `name = "ACU"`) {
		t.Fatal("native provider display name was not branded after restart")
	}
	// An external tool can leave the model unchanged while replacing its route.
	// Reapply through the same paused-watcher transaction, then read drift back.
	broken := strings.Replace(string(b), "http://"+address+"/v1", "http://127.0.0.1:1/v1", 1)
	if broken == string(b) {
		t.Fatal("fixture did not find the managed provider URL")
	}
	if err := os.WriteFile(filepath.Join(codex, "config.toml"), []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	repairHost := &server{kernel: k, desktop: true}
	err = repairHost.prepareCodexSetupRoute(ctx, "acu-runtime", []string{"acu-auto"})
	if err != nil {
		t.Fatal("preserved Codex drift repair", err)
	}
	state, err = k.readState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCodexRouteState(state, []codexFieldChange{{"model", "acu-runtime/acu-auto"}}, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if !strings.Contains(string(b), "preserved-test-marker") {
		t.Fatal("repair lost existing MCP")
	}
	// A replaced default is repaired by the already-confirmed one-click model,
	// never by restoring whatever historical model was last recorded.
	replaced := strings.Replace(string(b), `model = "acu-runtime/acu-auto"`, `model = "unregistered-external-model"`, 1)
	if replaced == string(b) {
		t.Fatal("fixture did not replace model")
	}
	if err := os.WriteFile(filepath.Join(codex, "config.toml"), []byte(replaced), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = repairHost.repairPreservedCodex(ctx, codexFieldChange{"model", "acu-runtime/acu-auto"})
	if err != nil {
		t.Fatal("one-click replaced model recovery", err)
	}
	if err := verifyCodexRouteState(state, []codexFieldChange{{"model", "acu-runtime/acu-auto"}}, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	before := append([]byte(nil), b...)
	assertLogin := func(state map[string]json.RawMessage, want string) {
		t.Helper()
		var agents []setupAgent
		json.Unmarshal(state["agents"], &agents)
		for _, agent := range agents {
			if agent.ID != "codex" {
				continue
			}
			for _, field := range agent.Fields {
				if field.Key == "login" {
					if field.Value != want {
						t.Fatalf("model selection changed login mode: %q want %q", field.Value, want)
					}
					return
				}
			}
		}
		t.Fatal("missing native login field")
	}
	assertLogin(state, "")
	_, err = k.changeCodexQuietlyWith(ctx, nil, false, func(executable string) error {
		return nativeCatalogManagement(ctx, executable, func(control *kernelState) error {
			return control.exchange(ctx, "POST", "/api/agent-models/codex", map[string]any{"hidden": []string{"acu-runtime/second-model"}}, nil)
		})
	})
	if err != nil {
		t.Fatal("quiet native catalog hide", err)
	}
	_, err = k.changeCodexQuietlyWith(ctx, nil, false, func(executable string) error {
		return nativeCatalogManagement(ctx, executable, func(control *kernelState) error {
			return control.exchange(ctx, "POST", "/api/agent-models/codex", map[string]any{"hidden": []string{}}, nil)
		})
	})
	if err != nil {
		t.Fatal("quiet native catalog reveal", err)
	}
	var syncedCatalog agentCatalog
	if err = k.exchange(ctx, "GET", "/api/agent-models/codex", nil, &syncedCatalog); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range syncedCatalog.Models {
		if m.ID == "acu-runtime/second-model" && !m.Hidden {
			found = true
		}
	}
	if !found {
		t.Fatal("new model was not visible after quiet native sync")
	}
	batchStarted := time.Now()
	batchBody, _ := json.Marshal(map[string]any{"agent": "codex", "expected": syncedCatalog, "operations": []map[string]any{
		{"only": true}, {"shown": []string{"acu-runtime/acu-auto"}}, {"ownHidden": true}, {"order": []string{"acu-runtime/acu-auto"}},
	}})
	batchResponse := httptest.NewRecorder()
	host.syncAgentModels(batchResponse, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(string(batchBody))).WithContext(ctx))
	if batchResponse.Code != 200 {
		t.Fatalf("quiet native batch: %d %s", batchResponse.Code, batchResponse.Body)
	}
	var saved agentCatalog
	if err := json.Unmarshal(batchResponse.Body.Bytes(), &saved); err != nil || !saved.Only || !saved.OwnHidden {
		t.Fatalf("batch flags not saved: %s", batchResponse.Body)
	}
	for _, model := range saved.Models {
		if model.ID == "acu-runtime/second-model" && !model.Hidden {
			t.Fatal("batch did not hide unselected model")
		}
	}
	t.Logf("Verified four Codex catalog operations in one safe transaction in %s", time.Since(batchStarted).Round(time.Millisecond))
	restoreBody, _ := json.Marshal(map[string]any{"agent": "codex", "expected": saved, "operations": []map[string]any{
		{"only": false}, {"hidden": []string{}}, {"ownHidden": false}, {"order": []string{}},
	}})
	restoreResponse := httptest.NewRecorder()
	host.syncAgentModels(restoreResponse, httptest.NewRequest("POST", "/runtime/agents/models", strings.NewReader(string(restoreBody))).WithContext(ctx))
	if restoreResponse.Code != 200 {
		t.Fatalf("quiet native batch restore: %d %s", restoreResponse.Code, restoreResponse.Body)
	}
	for _, mode := range []string{"api", "chatgpt", ""} {
		callsBefore := validationCalls
		state, err = k.changeCodexQuietly(ctx, []codexFieldChange{{"login", mode}}, false)
		if err != nil {
			t.Fatal("native login change", mode, err)
		}
		if validationCalls != callsBefore+1 {
			t.Fatal("login change skipped native validation")
		}
		assertLogin(state, mode)
		state, err = k.changeCodexQuietly(ctx, []codexFieldChange{{"model", "acu-runtime/acu-auto"}}, false)
		if err != nil {
			t.Fatal("model change with native login", mode, err)
		}
		assertLogin(state, mode)
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	before = append([]byte(nil), b...)
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
	_, err = k.changeCodexQuietlyWith(ctx, nil, false, func(string) error { return errors.New("synthetic catalog failure") })
	if err == nil || !strings.Contains(err.Error(), "已恢复此前配置") || strings.Contains(err.Error(), "正在恢复") {
		t.Fatalf("failed mutation did not report terminal rollback: %v", err)
	}
	b, _ = os.ReadFile(filepath.Join(codex, "config.toml"))
	if string(b) != string(before) {
		t.Fatal("failed catalog mutation changed config")
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
	k.codexValidate = defaultValidate
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
