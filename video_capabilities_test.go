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

func TestDefaultVideoModelPersistsNativeSettingWithoutChangingTools(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.prefs["videoGenModels"] = []map[string]string{{"id": "acu-runtime/grok-imagine-video"}, {"id": "other-provider/video-model"}}
	f.prefs["videoGenAuto"] = "acu-runtime/grok-imagine-video"
	for _, model := range []string{"other-provider/video-model", ""} {
		view, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", ModelsOnly: true, VideoGen: &model})
		if err != nil {
			t.Fatal(err)
		}
		if view.VideoGen != model || f.prefs["videoGen"] != model {
			t.Fatalf("native pick not saved: %+v", view)
		}
	}
	if len(f.writes) != 2 || f.writes[0] != "native-setting:videoGen" || f.writes[1] != "native-setting:videoGen" {
		t.Fatalf("unexpected writes %v", f.writes)
	}
}

func TestDefaultVideoValidationPrecedesAllModelWrites(t *testing.T) {
	for _, supported := range []bool{false, true} {
		host, f := capabilityTestHost(t)
		if supported {
			f.prefs["videoGenModels"] = []map[string]string{{"id": "acu-runtime/video"}}
		}
		video, vision := "unknown/video", "off"
		_, err := host.capabilityApply(context.Background(), capabilityInput{Provider: "acu-runtime", ModelsOnly: true, VideoGen: &video, Vision: &vision})
		if err == nil || len(f.writes) != 0 {
			t.Fatalf("invalid video partially saved: %v %v", err, f.writes)
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
