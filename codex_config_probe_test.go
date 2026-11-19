package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("ACU_TEST_CODEX_RPC") == "1" && len(os.Args) == 2 && os.Args[1] == "app-server" {
		decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
		for {
			var in struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			if decoder.Decode(&in) != nil {
				os.Exit(0)
			}
			if in.ID == 0 {
				continue
			}
			var result any = map[string]any{}
			switch in.Method {
			case "account/read":
				// No identity probe is needed to load an API configuration.
				os.Exit(8)
			case "config/read":
				model := "acu-runtime/acu-auto"
				if os.Getenv("ACU_TEST_CODEX_SCENARIO") == "wrong-model" {
					model = "wrong"
				}
				result = map[string]any{"config": map[string]string{"model": model}}
			case "model/list":
				result = map[string]any{"data": []map[string]string{{"id": "acu-runtime/acu-auto"}}}
				if os.Getenv("ACU_TEST_CODEX_SCENARIO") == "auth-changed" {
					os.WriteFile(filepath.Join(codexConfigDirectory(), "auth.json"), []byte(`{"auth_mode":"apikey"}`), 0600)
				}
			}
			encoder.Encode(map[string]any{"id": in.ID, "result": result})
		}
	}
	os.Exit(m.Run())
}

func TestDesktopProbeUnavailableReadback(t *testing.T) {
	const model = "acu-runtime/acu-auto"
	for _, scenario := range []string{"live", "local", "missing-file", "wrong-slug", "wrong-config", "wrong-route", "hidden", "filtered", "missing-model", "drift", "hydrogen", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			config := `model = "` + model + `"`
			switch scenario {
			case "local", "missing-file", "wrong-slug":
				config += "\nmodel_catalog_json = \"magpie-models.json\""
				if scenario != "missing-file" {
					slug := model
					if scenario == "wrong-slug" {
						slug = "other"
					}
					if err := os.WriteFile(filepath.Join(dir, "magpie-models.json"), []byte(`{"models":[{"slug":"`+slug+`"}]}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "wrong-config":
				config = `model = "wrong"`
			}
			if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/state":
					state := `{"agents":[{"id":"codex","wired":true,"fields":[{"key":"model","value":"` + model + `"}]}]}`
					if scenario == "wrong-route" {
						state = strings.Replace(state, model, "other", 1)
					}
					if scenario == "drift" {
						state = strings.Replace(state, `"wired":true`, `"wired":true,"drift":{"kind":"unwired"}`, 1)
					}
					io.WriteString(w, state)
				case "/api/agent-models/codex":
					id := model
					if scenario == "missing-model" {
						id = "other"
					}
					json.NewEncoder(w).Encode(agentCatalog{Models: []agentCatalogModel{{ID: id, Hidden: scenario == "hidden", Kept: scenario == "filtered"}}})
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			selected := model
			if scenario == "hydrogen" {
				selected = codexHydrogenModel
			}
			err := host.kernel.verifyDesktopCodexWithoutProbe(ctx, selected)
			if (err == nil) != (scenario == "live" || scenario == "local") {
				t.Fatalf("readback: %v", err)
			}
			if scenario == "hydrogen" && !strings.Contains(err.Error(), "Code Mode") {
				t.Fatal(err)
			}
		})
	}
}

func TestCodexProbeStartIsNotConfigurationRejection(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	err := probeCodexConfiguration(context.Background(), filepath.Join(t.TempDir(), "missing.exe"), "acu-runtime/acu-auto")
	if !errors.Is(err, errCodexProbeStart) {
		t.Fatal(err)
	}
}

func TestCodexProbeLoadsModelsWithoutRequiringAccountResponse(t *testing.T) {
	for _, scenario := range []string{"signed-in", "no-auth", "wrong-model", "auth-changed"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Setenv("ACU_TEST_CODEX_RPC", "1")
			t.Setenv("ACU_TEST_CODEX_SCENARIO", scenario)
			if scenario != "no-auth" {
				os.WriteFile(filepath.Join(codexConfigDirectory(), "auth.json"), []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"fixture"}}`), 0600)
			}
			executable, _ := os.Executable()
			err := probeCodexConfiguration(context.Background(), executable, "acu-runtime/acu-auto")
			wantFailure := scenario == "wrong-model" || scenario == "auth-changed"
			if (err != nil) != wantFailure {
				t.Fatalf("unexpected probe result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "登录身份") {
				t.Fatal("historical account gate returned")
			}
		})
	}
}
