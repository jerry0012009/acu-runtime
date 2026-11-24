package main

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClaudeCompactBoundaries(t *testing.T) {
	var p localProvider
	if err := json.Unmarshal([]byte(`{"id":"acu-runtime","key":{"set":true},"compacts":{"claude-opus-custom":381000,"gpt-sol":171000},"models":[{"id":"claude-opus-5-5","context":1000000,"on":true},{"id":"claude-haiku-4-5","context":200000,"on":true},{"id":"claude-opus-custom","context":1000000,"on":true},{"id":"alias","vendor":"claude-fable-5-2","context":1000000,"on":true},{"id":"claude-sonnet-5.5","context":1000000,"on":true},{"id":"gpt-sol","context":1000000,"on":true},{"id":"claude-haiku-new","context":1000000,"on":true},{"id":"claude-opus-hidden","context":1000000}]}`), &p); err != nil {
		t.Fatal(err)
	}
	out := recommendedCompacts(p)
	if !maps.Equal(out, map[string]int{"claude-opus-5-5": 750000, "claude-opus-custom": 381000, "alias": 750000, "claude-sonnet-5.5": 750000, "claude-haiku-new": 750000, "gpt-sol": 171000}) {
		t.Fatal(out)
	}
	p.Compacts = out
	if recommendedCompacts(p) != nil {
		t.Fatal("not idempotent")
	}
	p.Compacts = map[string]int{"*": 400000}
	if recommendedCompacts(p) != nil {
		t.Fatal("overrode wildcard")
	}
	p.Compacts = nil
	p.ID = "manual"
	if recommendedCompacts(p) != nil {
		t.Fatal("modified another provider")
	}
}

// Uses the release-pinned executable, an empty home and synthetic metadata.
// No inference is required. Put the keyless/disabled-network environment in
// place before starting the native management listener.
func TestClaudeCompactNativeReadback(t *testing.T) {
	exe := os.Getenv("ACU_MODEL_SYNC_KERNEL")
	if exe == "" {
		t.Skip("needs packaged kernel")
	}
	root := t.TempDir()
	for _, n := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "APPDATA", "LOCALAPPDATA", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "XDG_CACHE_HOME", "XDG_DATA_HOME"} {
		d := filepath.Join(root, n)
		if err := os.MkdirAll(d, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(n, d)
	}
	t.Setenv("MAGPIE_NO_ACCOUNTS", "1")
	for _, n := range []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY"} {
		t.Setenv(n, "http://127.0.0.1:1")
	}
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAGPIE_ADDR", ln.Addr().String())
	ln.Close()
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "magpie")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "providers.json")
	data := `{"providers":[{"id":"acu-runtime","name":"ACU","key":"unit-fixture","chat":"https://acu.best/v1","responses":"https://acu.best/v1","anthropic":"https://acu.best","modelsURL":"https://acu.best/v1/models","socket":"off","models":["claude-opus-5-5","gpt-6.1-sol"]}]}`
	if err = os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(codexConfigDirectory(), "config.toml"), []byte("# fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	err = nativeCatalogManagement(ctx, exe, func(k *kernelState) error {
		k.enforceVersion = true
		host := &server{kernel: k, desktop: true, networkFile: filepath.Join(root, "network.json")}
		if err := runCatalogCLI(ctx, exe, []string{"codex", "acu-runtime/claude-opus-5-5"}); err != nil {
			return err
		}
		p, err := host.configuredProvider(ctx)
		if err != nil {
			return err
		}
		if recommendedCompacts(p) == nil {
			t.Fatalf("no eligible model: %+v", p.Models)
		}
		// Simulate an existing user's custom global budget before the first
		// reconciliation. The same path runs after an upgrade and on setup.
		if err := k.exchange(ctx, "POST", "/api/settings/full-context", map[string]any{"at": 420000}, nil); err != nil {
			return err
		}
		if err := host.applyClaudeCompacts(ctx); err != nil {
			return err
		}
		p, err = host.configuredProvider(ctx)
		if err != nil || len(p.Compacts) != 0 {
			t.Fatalf("overrode global budget: %v, %v", p.Compacts, err)
		}
		if err := k.exchange(ctx, "POST", "/api/settings/full-context", map[string]any{"at": 0}, nil); err != nil {
			return err
		}
		if err = host.applyClaudeCompacts(ctx); err != nil {
			return err
		}
		p, err = host.configuredProvider(ctx)
		if err != nil {
			return err
		}
		if p.Compacts["claude-opus-5-5"] != 750000 || len(p.Compacts) != 1 {
			t.Fatalf("native readback: %v", p.Compacts)
		}
		for i := 0; i < 50; i++ {
			var c struct {
				Models []struct {
					Slug   string
					Window int `json:"context_window"`
					At     int `json:"auto_compact_token_limit"`
				}
			}
			raw, _ := os.ReadFile(filepath.Join(codexConfigDirectory(), "magpie-models.json"))
			json.Unmarshal(raw, &c)
			for _, m := range c.Models {
				if m.Slug == "acu-runtime/claude-opus-5-5" && m.Window == 1000000 && m.At == 750000 {
					return host.applyClaudeCompacts(ctx)
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("Codex catalog not refreshed")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
