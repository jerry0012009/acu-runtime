package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestVideoPreparationCachesOnlyUnchangedInstalledTool(t *testing.T) {
	file := os.Getenv("ACU_VIDEO_PREPARE_TEST_BINARY")
	if file == "" {
		t.Skip("explicit packaged video tool acceptance")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	target, err := videoMCPPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, data, 0700); err != nil {
		t.Fatal(err)
	}
	host := &server{}
	for i := 0; i < 2; i++ {
		start := time.Now()
		if err := host.prepareVideoTools(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Logf("packaged video prepare round=%d elapsed=%dms", i+1, time.Since(start).Milliseconds())
	}
	if err = os.WriteFile(target, []byte("invalid replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = host.prepareVideoTools(ctx); err == nil {
		t.Fatal("changed binary accepted from cache")
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

func TestVideoAudioUsesExistingProviderCredentialAndStandardLibraryEnv(t *testing.T) {
	host, f := capabilityTestHost(t)
	if err := host.assignVideoReading(context.Background(), []string{"workbuddy"}); err != nil {
		t.Fatal(err)
	}
	if err := host.configureVideoAudio(context.Background(), "acu-runtime"); err != nil {
		t.Fatal(err)
	}
	tool := f.servers[1]
	env, ok := tool["env"].(map[string]any)
	if !ok || env["ACU_AUDIO_API_KEY"] != "private-acu-test-key" || env["ACU_AUDIO_MODEL"] != "gemini-3.5-flash-lite" || env["ACU_AUDIO_BASE_URL"] != "https://acu.best/v1" {
		t.Fatal("audio connection not configured")
	}
	if tool["command"] != "/fixture/acu-video-mcp" || f.prefs["vision"] != "" {
		t.Fatal("replaced media owner or main settings")
	}
	tool["command"] = "/custom/video-tool"
	before := strings.Count(strings.Join(f.writes, "\n"), "/api/library/servers/save")
	if err := host.configureVideoAudio(context.Background(), "acu-runtime"); err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.Join(f.writes, "\n"), "/api/library/servers/save") != before {
		t.Fatal("overwrote custom MCP")
	}
}

func TestVideoReadingUpgradesOnlyOwnedVersionedDistribution(t *testing.T) {
	host, f := capabilityTestHost(t)
	target, err := videoMCPPath()
	if err != nil {
		t.Fatal(err)
	}
	host.videoToolPath = func(context.Context) (string, error) { return target, nil }
	old := filepath.Join(filepath.Dir(filepath.Dir(target)), "0.1.0", filepath.Base(target))
	f.servers = append(f.servers, map[string]any{"name": videoReadingName, "note": capabilityOwner,
		"transport": "stdio", "command": old, "args": []string{}, "agents": []string{"workbuddy"},
		"paused": true, "disabledTools": []string{"video_detail"}})
	if err := host.assignVideoReading(context.Background(), []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if f.servers[1]["command"] != target || f.servers[1]["paused"] != true {
		t.Fatal("owned upgrade lost preferences")
	}
}

func TestVideoReadingToggleIsIndependentAndExplicitOffSurvivesToolRepair(t *testing.T) {
	host, f := capabilityTestHost(t)
	ctx := context.Background()
	view, _, _, err := host.capabilityRead(ctx, "acu-runtime")
	if err != nil || !view.VideoReadingEnabled || view.VideoReadingReady {
		t.Fatal("fresh default must be enabled but not falsely ready")
	}
	if err := host.setVideoReadingEnabled(ctx, "acu-runtime", nil, false); err != nil {
		t.Fatal(err)
	}
	view, _, _, _ = host.capabilityRead(ctx, "acu-runtime")
	if view.VideoReadingEnabled {
		t.Fatal("disable not persisted")
	}
	if err := host.setVideoReadingEnabled(ctx, "acu-runtime", []string{"workbuddy"}, true); err != nil {
		t.Fatal(err)
	}
	view, _, _, _ = host.capabilityRead(ctx, "acu-runtime")
	if !view.VideoReadingEnabled || !view.VideoReadingReady {
		t.Fatal("reader not enabled")
	}
	for _, tool := range f.servers {
		if tool["name"] == "acu-image" {
			t.Fatal("standalone video reading created unrelated generation tool")
		}
	}
	if err := host.setVideoReadingEnabled(ctx, "acu-runtime", nil, false); err != nil {
		t.Fatal(err)
	}
	if err := host.assignVideoReading(ctx, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	view, _, _, _ = host.capabilityRead(ctx, "acu-runtime")
	if view.VideoReadingEnabled || view.VideoReadingReady {
		t.Fatal("ordinary assignment re-enabled explicit opt-out")
	}
}
