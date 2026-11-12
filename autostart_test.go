package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitStartupChoicePersistsInIsolatedRuntimeState(t *testing.T) {
	cache := t.TempDir()
	file := filepath.Join(cache, "startup.json")
	for _, enabled := range []bool{false, true, false} {
		if err := writeStartupPreference(file, enabled); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var saved struct {
			Enabled bool `json:"enabled"`
		}
		if json.Unmarshal(b, &saved) != nil || saved.Enabled != enabled {
			t.Fatal("startup choice was not preserved")
		}
	}
}
