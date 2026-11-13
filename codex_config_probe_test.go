package main

import (
	"context"
	"encoding/json"
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
