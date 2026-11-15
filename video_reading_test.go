package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestVideoReadingUsesStandardExternalMCP(t *testing.T) {
	host, f := capabilityTestHost(t)
	for range 2 {
		if err := host.assignVideoReading(context.Background(), []string{"codex"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.servers) != 2 {
		t.Fatal("duplicate tool")
	}
	tool := f.servers[1]
	if tool["command"] != "/fixture/acu-video-mcp" {
		t.Fatal("not external MCP", tool)
	}
	data, _ := json.Marshal(tool["args"])
	if string(data) != "[]" {
		t.Fatal(string(data))
	}
	for _, write := range f.writes {
		if strings.Contains(write, "/video/") {
			t.Fatal("private kernel API", write)
		}
	}
	tool["command"] = "custom"
	tool["hidden"] = true
	tool["disabledTools"] = []string{"video_watch"}
	if err := host.assignVideoReading(context.Background(), []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if tool["command"] != "custom" || tool["hidden"] != true {
		t.Fatal("customization lost")
	}
}
func TestVideoReadingFailureDoesNotAssignUnavailableTool(t *testing.T) {
	host, f := capabilityTestHost(t)
	host.videoToolPath = func(context.Context) (string, error) { return "", errors.New("offline") }
	if host.assignVideoReading(context.Background(), []string{"codex"}) == nil {
		t.Fatal("false success")
	}
	if len(f.writes) != 0 {
		t.Fatal(f.writes)
	}
}
func TestVideoReadingMigratesOnlyOwnLegacyLauncher(t *testing.T) {
	host, f := capabilityTestHost(t)
	self, _ := os.Executable()
	f.servers = append(f.servers, map[string]any{"name": videoReadingName, "note": capabilityOwner, "command": self, "transport": "stdio", "args": []string{"--video-reader"}, "agents": []string{"codex"}, "disabledTools": []string{"video_detail"}})
	if err := host.assignVideoReading(context.Background(), []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(f.servers[1])
	if !strings.Contains(string(data), `"command":"/fixture/acu-video-mcp"`) || !strings.Contains(string(data), `"disabledTools":["video_detail"]`) {
		t.Fatal(string(data))
	}
}
