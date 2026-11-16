package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRepeatCapabilitySetupDoesNotRewriteOrRefetch(t *testing.T) {
	host, f := capabilityTestHost(t)
	in := capabilityInput{Provider: "acu-runtime", Agents: []string{"codex"}}
	if _, err := host.capabilityApply(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	f.writes = nil
	start := time.Now()
	if _, err := host.capabilityApply(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	for _, path := range f.writes {
		if path != "/api/provider/key" {
			t.Fatalf("repeated configuration mutated: %v", f.writes)
		}
	}
	t.Logf("repeat setup %dms, zero writes and provider refreshes", time.Since(start).Milliseconds())
	in.Agents = []string{"codex", "claude"}
	if _, err := host.capabilityApply(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range f.writes {
		if path == "/api/library/servers/agents-some" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("native bulk assignment calls=%d writes=%v", count, f.writes)
	}
}

func TestToolSetupReturnsBeforeVideoAndCoalesces(t *testing.T) {
	host, _ := capabilityTestHost(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host.lifecycle = ctx
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	host.videoToolPath = func(ctx context.Context) (string, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return "/fixture/acu-video-mcp", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	post := func(body string) toolSetupView {
		t.Helper()
		w := httptest.NewRecorder()
		start := time.Now()
		host.prepareCapabilities(w, httptest.NewRequest("POST", "/runtime/capabilities/prepare", strings.NewReader(body)))
		if w.Code != 202 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if time.Since(start) > time.Second {
			t.Fatal("request blocked on video")
		}
		var view toolSetupView
		json.Unmarshal(w.Body.Bytes(), &view)
		return view
	}
	a := post(`{"provider":"acu-runtime","agents":["codex"]}`)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("video did not start")
	}
	b := post(`{"provider":"acu-runtime","agents":["claude"]}`)
	if a.ID != b.ID || len(b.Agents) != 2 {
		t.Fatalf("task not coalesced: %+v %+v", a, b)
	}
	// The downloader must not hold the mutation lock or block read/settings actions.
	locked := make(chan struct{})
	go func() { host.capabilityMu.Lock(); host.capabilityMu.Unlock(); close(locked) }()
	select {
	case <-locked:
	case <-time.After(time.Second):
		t.Fatal("download holds capability mutation lock")
	}
	close(release)
	for deadline := time.Now().Add(5 * time.Second); ; {
		host.toolSetup.Lock()
		view := host.toolSetup.view
		host.toolSetup.Unlock()
		if view.Phase == "complete" {
			break
		}
		if view.Phase == "error" || time.Now().After(deadline) {
			t.Fatalf("task did not complete: %+v", view)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestToolSetupFailureRetainsBasicReady(t *testing.T) {
	host, _ := capabilityTestHost(t)
	host.videoToolPath = func(context.Context) (string, error) { return "", errors.New("download unavailable") }
	w := httptest.NewRecorder()
	host.prepareCapabilities(w, httptest.NewRequest("POST", "/runtime/capabilities/prepare", strings.NewReader(`{"provider":"acu-runtime","agents":["codex"]}`)))
	for deadline := time.Now().Add(5 * time.Second); ; {
		host.toolSetup.Lock()
		view := host.toolSetup.view
		host.toolSetup.Unlock()
		if view.Phase == "error" {
			if !view.BasicReady {
				t.Fatal("lost image/search completion")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not fail")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestVideoDownloadRejectsCorruptionAndReportsProgress(t *testing.T) {
	data := []byte("verified executable fixture")
	sum := sha256.Sum256(data)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}))
	defer remote.Close()
	target := filepath.Join(t.TempDir(), "video")
	if err := os.WriteFile(target, []byte("keep original"), 0700); err != nil {
		t.Fatal(err)
	}
	progress := false
	ctx := context.WithValue(context.Background(), videoProgressKey{}, func(string) { progress = true })
	if _, err := downloadVideoMCP(ctx, remote.Client(), videoPackage{URL: remote.URL, SHA256: strings.Repeat("0", 64)}, target); err == nil {
		t.Fatal("corrupt digest accepted")
	}
	old, _ := os.ReadFile(target)
	if string(old) != "keep original" {
		t.Fatal("failed download replaced original")
	}
	if _, err := downloadVideoMCP(ctx, remote.Client(), videoPackage{URL: remote.URL, SHA256: hex.EncodeToString(sum[:])}, target); err != nil {
		t.Fatal(err)
	}
	if !progress {
		t.Fatal("no download progress")
	}
}
