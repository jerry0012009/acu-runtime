package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexBrandChangesOnlyNativeDisplayValue(t *testing.T) {
	for _, config := range []string{
		"model_provider = 'magpie'\n[model_providers.magpie]\nname = 'magpie' # keep\nbase_url = 'http://127.0.0.1:3425/v1'\nrequires_openai_auth = true\n",
		"# comment\n[model_providers.'magpie']\nname = \"magpie\"\n[model_providers.other]\nname='magpie'\n",
		"model_providers.magpie.name = 'magpie'\n",
	} {
		t.Run(config[:10], func(t *testing.T) {
			t.Setenv("CODEX_HOME", t.TempDir())
			path := filepath.Join(codexConfigDirectory(), "config.toml")
			os.WriteFile(path, []byte(config), 0600)
			if err := brandCodexProvider(); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			var parsed map[string]any
			if err := toml.Unmarshal(after, &parsed); err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(config, "'magpie' # keep", `"ACU" # keep`, 1)
			if want == config {
				want = strings.Replace(config, "name = \"magpie\"", `name = "ACU"`, 1)
			}
			if want == config {
				want = strings.Replace(config, "name = 'magpie'", `name = "ACU"`, 1)
			}
			if string(after) != want {
				t.Fatalf("unexpected rewrite:\n%s", after)
			}
			if err := brandCodexProvider(); err != nil {
				t.Fatal(err)
			}
			repeated, _ := os.ReadFile(path)
			if string(after) != string(repeated) {
				t.Fatal("not idempotent")
			}
		})
	}
}

func TestCodexBrandPreservesCustomAndUnrecognizedConfiguration(t *testing.T) {
	for _, config := range []string{"[model_providers.magpie]\nname='My gateway'\n", "[model_providers.other]\nname='magpie'\n", "broken [", "[model_providers]\nmagpie={name='magpie'}\n"} {
		t.Setenv("CODEX_HOME", t.TempDir())
		path := filepath.Join(codexConfigDirectory(), "config.toml")
		os.WriteFile(path, []byte(config), 0600)
		if err := brandCodexProvider(); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != config {
			t.Fatal("changed custom or unsupported config")
		}
	}
}
