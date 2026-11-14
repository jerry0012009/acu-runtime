package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestVideoTogglePreservesOtherToolsAndAssignments(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.servers = append(f.servers, map[string]any{"name": "acu-image", "note": capabilityOwner, "agents": []string{"workbuddy"}, "command": "custom-preserved", "disabledTools": []string{"generate_video", "generate_image"}})
	for _, enabled := range []bool{true, true, false} {
		_, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", VideoEnabled: &enabled})
		if err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(f.servers[len(f.servers)-1])
	if !strings.Contains(string(data), `"disabledTools":["generate_image","generate_video"]`) || !strings.Contains(string(data), `"command":"custom-preserved"`) || !strings.Contains(string(data), `"agents":["workbuddy"]`) {
		t.Fatalf("lost user settings: %s", data)
	}
	if len(f.writes) != 2 {
		t.Fatalf("toggle not idempotent: %v", f.writes)
	}
	for _, path := range f.writes {
		if path != "/api/library/servers/tools" {
			t.Fatalf("unexpected write %s", path)
		}
	}
}

func TestVideoToggleRequiresAssignedToolAndSeparateSave(t *testing.T) {
	for _, mode := range []string{"missing", "paused", "unassigned", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			host, f := capabilityTestHost(t)
			if mode != "missing" {
				tool := map[string]any{"name": "acu-image", "note": capabilityOwner, "agents": []string{"workbuddy"}}
				if mode == "paused" {
					tool["paused"] = true
				}
				if mode == "unassigned" {
					tool["agents"] = []string{}
				}
				f.servers = append(f.servers, tool)
			}
			yes := true
			in := capabilityInput{Provider: "acu-runtime", VideoEnabled: &yes}
			if mode == "mixed" {
				in.ModelsOnly = true
			}
			if _, err := host.capabilityApply(context.Background(), in); err == nil {
				t.Fatal("invalid toggle accepted")
			}
			if len(f.writes) != 0 {
				t.Fatalf("invalid toggle mutated settings: %v", f.writes)
			}
		})
	}
}
