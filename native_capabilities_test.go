package main

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestNativeToolMigrationPreservesOptionsAndSearchAssignments(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.servers = append(f.servers, map[string]any{"name": "acu-vision", "transport": "http", "url": "http://127.0.0.1:3425/mcp/magpie/vision", "note": capabilityOwner, "agents": []string{"codex"}, "disabledTools": []string{"user-disabled"}, "hub": "direct", "startupTimeout": 83})
	if err := host.assignNativeTool(context.Background(), "acu-vision", "vision", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.servers {
		if s["name"] == "acu-vision" {
			if s["command"] != "/native/magpie" || s["hub"] != "direct" || s["startupTimeout"] != float64(83) {
				t.Fatalf("lost native definition or user options %+v", s)
			}
		}
	}
	f.servers = append(f.servers, map[string]any{"name": "acu-free-search", "transport": "stdio", "command": "old-uvx", "args": []string{"--from", "duckduckgo-mcp-server[browser]==0.7.0", "duckduckgo-mcp-server"}, "note": capabilityOwner, "agents": []string{"codex", "hermes"}})
	if err := host.assignNativeTool(context.Background(), "acu-search", "web-search", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.servers {
		if s["name"] == "acu-free-search" && len(s["agents"].([]string)) != 0 {
			t.Fatal("duplicate search assignment retained")
		}
		if s["name"] == "acu-search" {
			ids := s["agents"].([]any)
			if len(ids) != 3 {
				t.Fatalf("old assignments lost %+v", ids)
			}
		}
	}
	before := len(f.writes)
	if err := host.assignNativeTool(context.Background(), "acu-search", "web-search", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != before {
		t.Fatalf("idempotent assignment rewrote tools: %v", f.writes[before:])
	}
}

func TestModelValidationCompletesBeforeAnySettingsWrite(t *testing.T) {
	host, f := capabilityTestHost(t)
	vision, image := "off", "missing-image"
	_, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", ModelsOnly: true, Vision: &vision, ImageGen: &image})
	if err == nil || len(f.writes) != 0 {
		t.Fatalf("invalid second choice partially saved: %v, %v", err, f.writes)
	}
}

func TestCustomOrDisabledFreeSearchIsNotMigrated(t *testing.T) {
	for _, option := range []string{"command", "paused", "disabledTools", "hub"} {
		t.Run(option, func(t *testing.T) {
			host, f := capabilityTestHost(t)
			legacy := map[string]any{"name": "acu-free-search", "transport": "stdio", "command": "custom-search", "note": capabilityOwner, "agents": []string{"hermes"}}
			if option != "command" {
				legacy["args"] = []string{"duckduckgo-mcp-server[browser]==0.7.0", "duckduckgo-mcp-server"}
				legacy[option] = map[string]any{"paused": true, "disabledTools": []string{"search"}, "hub": "direct"}[option]
			}
			f.servers = append(f.servers, legacy)
			if err := host.assignNativeTool(context.Background(), "acu-search", "web-search", []string{"codex"}); err != nil {
				t.Fatal(err)
			}
			if len(legacy["agents"].([]string)) != 1 {
				t.Fatal("custom search detached")
			}
		})
	}
}

func TestNativeDefinitionCanBeBrandedWithoutLosingExistingUsers(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.servers = append(f.servers, map[string]any{"name": "magpie-image", "transport": "stdio", "command": "/native/magpie", "args": []string{"mcp", "image"}, "agents": []string{"hermes"}, "disabledTools": []string{"generate_video"}})
	// Match the native market's Have field to an existing record.
	remote := host.kernel.client.Transport
	host.kernel.client.Transport = accountTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/library/market/servers" {
			return cloudFixture(`{"items":[{"id":"magpie-image","name":"magpie-image","have":"magpie-image"}]}`), nil
		}
		return remote.RoundTrip(r)
	})
	if err := host.assignNativeTool(context.Background(), "acu-image", "image", []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(f.servers, func(s map[string]any) bool { return s["name"] == "magpie-image" }) {
		t.Fatal("duplicate native name left")
	}
	for _, s := range f.servers {
		if s["name"] == "acu-image" {
			if len(s["agents"].([]any)) != 2 || s["command"] != "/native/magpie" {
				t.Fatalf("renaming lost native data %+v", s)
			}
		}
	}
}

func TestCapabilityFallbackUsesAnyNativeCandidateAndHonorsUserPick(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.prefs["visionAuto"] = ""
	f.prefs["visionModels"] = []map[string]string{{"id": "acu-runtime/future-visual-model", "name": "Native candidate"}}
	if _, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	if f.prefs["vision"] != "acu-runtime/future-visual-model" {
		t.Fatal("working vision fallback missing")
	}
	f.prefs["vision"] = "off"
	if _, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	if f.prefs["vision"] != "off" {
		t.Fatal("user opt-out replaced")
	}
}
