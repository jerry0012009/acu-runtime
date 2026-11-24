package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCodexGeminiToolCatalogScopeAndUserSettings(t *testing.T) {
	raw := []byte(`{"other":{"keep":true},"models":[{"slug":"acu-runtime/gemini-4-hydrogen","context_window":1000000,"supports_search_tool":true},{"slug":"acu-runtime/gpt-6-astra","tool_mode":"code_mode_only"},{"slug":"other/gemini-4-hydrogen"},{"slug":"acu-runtime/gemini-4-carbon"}]}`)
	next, changed, err := codexGeminiToolCatalog(raw)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	var before, after struct {
		Other  json.RawMessage
		Models []map[string]json.RawMessage
	}
	json.Unmarshal(raw, &before)
	json.Unmarshal(next, &after)
	if string(after.Models[0]["tool_mode"]) != `"code_mode"` || string(after.Models[0]["use_responses_lite"]) != "false" {
		t.Fatal("mode missing")
	}
	if string(after.Models[0]["context_window"]) != "1000000" || string(after.Models[0]["supports_search_tool"]) != "true" {
		t.Fatal("capabilities changed")
	}
	for i := 1; i < len(before.Models); i++ {
		a, _ := json.Marshal(before.Models[i])
		b, _ := json.Marshal(after.Models[i])
		if string(a) != string(b) {
			t.Fatal("other model changed")
		}
	}
	if _, changed, err := codexGeminiToolCatalog(next); err != nil || changed {
		t.Fatal("not idempotent")
	}
	for _, mode := range []string{"direct", "code_mode_only"} {
		data := []byte(`{"models":[{"slug":"acu-runtime/gemini-4-hydrogen","tool_mode":"` + mode + `"}]}`)
		if _, changed, err := codexGeminiToolCatalog(data); err != nil || changed {
			t.Fatal("explicit mode overwritten")
		}
	}
}

func TestConfigureCodexGeminiToolsLeavesCustomCatalogAlone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	path := filepath.Join(dir, "custom.json")
	raw := []byte(`{"models":[{"slug":"acu-runtime/gemini-4-hydrogen"}]}`)
	os.WriteFile(path, raw, 0600)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("model_catalog_json = "+quoteTOMLForTest(path)+"\n"), 0600)
	if err := configureCodexGeminiTools(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("custom catalog changed")
	}
}

func quoteTOMLForTest(value string) string { b, _ := json.Marshal(value); return string(b) }

func TestConfigureCodexGeminiToolsUsesExistingRollbackSnapshot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	path := filepath.Join(dir, "magpie-models.json")
	raw := []byte(`{"models":[{"slug":"acu-runtime/gemini-4-hydrogen"}]}`)
	os.WriteFile(path, raw, 0600)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("model_catalog_json = "+quoteTOMLForTest(path)+"\n"), 0600)
	snapshot, err := snapshotCodexRoute("")
	if err != nil {
		t.Fatal(err)
	}
	if err := configureCodexGeminiTools(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) == string(raw) {
		t.Fatal("managed catalog not configured")
	}
	if err := snapshot.restore(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("catalog rollback lost original")
	}
}
