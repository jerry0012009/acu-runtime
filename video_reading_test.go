package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVideoReadingAssignmentAndPreservation(t *testing.T) {
	host, f := capabilityTestHost(t)
	for range 2 {
		if err := host.assignVideoReading(context.Background(), []string{"codex"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.servers) != 2 {
		t.Fatal("duplicate tool", f.servers)
	}
	tool := f.servers[1]
	tool["disabledTools"] = []string{"video_watch"}
	tool["paused"] = true
	tool["command"] = "custom"
	if err := host.assignVideoReading(context.Background(), []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if tool["command"] != "custom" || tool["paused"] != true || len(tool["disabledTools"].([]string)) != 1 {
		t.Fatal("overwrote explicit settings")
	}
	tool["note"] = "user-owned"
	if err := host.assignVideoReading(context.Background(), []string{"codex"}); err == nil {
		t.Fatal("overwrote user-owned tool")
	}
}

func TestVideoReadingPreparationFailureIsRetryable(t *testing.T) {
	host, f := capabilityTestHost(t)
	host.prepareVideoReading = func(context.Context) (videoReadingInstall, error) {
		return videoReadingInstall{}, errors.New("offline")
	}
	if err := host.assignVideoReading(context.Background(), []string{"codex"}); err == nil {
		t.Fatal("false readiness")
	}
	if len(f.writes) != 0 {
		t.Fatal("assigned unavailable tool")
	}
}

func TestVideoNodeArchiveRejectsTraversal(t *testing.T) {
	for _, name := range []string{"node/../../escape", "node/C:\\escape", "node/bin/node"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "node.tar.gz")
			f, _ := os.Create(archive)
			gz := gzip.NewWriter(f)
			tr := tar.NewWriter(gz)
			if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0700, Size: 4}); err != nil {
				t.Fatal(err)
			}
			tr.Write([]byte("test"))
			tr.Close()
			gz.Close()
			f.Close()
			err := unpackVideoNode(archive, filepath.Join(dir, "out"))
			if name == "node/bin/node" && err != nil {
				t.Fatal(err)
			}
			if name != "node/bin/node" && err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestTapnowVideoRejectsForeignSourcesAndBackup(t *testing.T) {
	for _, raw := range []string{"https://files.tapnow.media.evil.test/a.mov", "http://files.tapnow.media/a.mov", "https://user:pass@files.tapnow.media/a.mov", "https://files.tapnow.media:443/a.mov", "file:///tmp/a.mov", "https://127.0.0.1/a.mov"} {
		if _, err := tapnowVideoURL(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	if _, err := tapnowVideoURL("https://files.tapnow.media/a.mov?token=fixture"); err != nil {
		t.Fatal(err)
	}
	fixtureTapnowIDs(t)
	if runTapnowVideo(context.Background(), "chrome-extension://"+tapnowBackupID+"/", tapnowNativeMessage{URL: "https://files.tapnow.media/a.mov"}).OK {
		t.Fatal("backup read video")
	}
}

// Explicit opt-in: downloads only pinned public dependencies, never inference.
func TestVideoReadingRealMCP(t *testing.T) {
	if os.Getenv("ACU_VIDEO_READING_SMOKE") != "1" {
		t.Skip("isolated download/MCP smoke only")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	installed, err := ensureVideoReading(ctx)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "fixture.mov")
	cmd := exec.CommandContext(ctx, filepath.Join(installed.Root, "bin", videoProgram("ffmpeg")), "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=12", "-t", "10", "-c:v", "libx264", "-pix_fmt", "yuv420p", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	content, err := readVideoFrames(ctx, installed, file)
	if err != nil {
		t.Fatal(err)
	}
	images := 0
	late := false
	for _, part := range content {
		if part.Type == "image" {
			images++
		}
		if strings.Contains(part.Text, "00:00:09") {
			late = true
		}
	}
	if images < 8 || !late {
		t.Fatalf("incomplete sampling: images=%d late=%v", images, late)
	}
	t.Logf("Pinned MCP read H264 MOV; %d sampled frames, final seconds covered; no cloud inference", images)
}
