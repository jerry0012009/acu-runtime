package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVideoReadingAssignmentUsesKernelAndPreservesCustomization(t *testing.T) {
	host, f := capabilityTestHost(t)
	for range 2 {
		if err := host.assignVideoReading(context.Background(), []string{"codex"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.servers) != 2 {
		t.Fatal("duplicate server", f.servers)
	}
	tool := f.servers[1]
	if tool["command"] != "/native/magpie" {
		t.Fatal("Runtime became MCP launcher", tool)
	}
	encoded, _ := json.Marshal(tool["args"])
	if string(encoded) != `["mcp","video"]` {
		t.Fatal(string(encoded))
	}
	tool["command"] = "custom"
	tool["paused"] = true
	tool["disabledTools"] = []string{"video_watch"}
	if err := host.assignVideoReading(context.Background(), []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if tool["command"] != "custom" || tool["paused"] != true {
		t.Fatal("overwrote customization")
	}
}

func TestUnsupportedKernelNeverInstallsOrLaunchesRuntimeFallback(t *testing.T) {
	host, f := capabilityTestHost(t)
	f.videoError = true
	if err := host.assignVideoReading(context.Background(), []string{"codex"}); err == nil {
		t.Fatal("false readiness")
	}
	if len(f.servers) != 1 || len(f.writes) != 1 || f.writes[0] != "/api/library/video/prepare" {
		t.Fatal(f.writes, f.servers)
	}
}

func TestTapnowFramesAreDelegatedToKernelNotFetchedByRuntime(t *testing.T) {
	fixtureTapnowIDs(t)
	calls := 0
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/library/video/frames" {
			t.Fatal(r.URL.Path)
		}
		var in struct{ URL string }
		json.NewDecoder(r.Body).Decode(&in)
		if in.URL != "https://files.tapnow.media/test.mov" {
			t.Fatal(in)
		}
		io.WriteString(w, `{"ok":true,"content":[{"type":"image","mimeType":"image/jpeg","data":"aGVsbG8="}],"audioAnalyzed":false}`)
	}))
	host.tapnow.nativeToken = strings.Repeat("a", 64)
	msg := tapnowNativeMessage{Action: "video-frames", Origin: "chrome-extension://" + tapnowCompanionID + "/", Version: "0.1.24", URL: "https://files.tapnow.media/test.mov"}
	w := nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) || calls != 1 {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
	msg.Origin = "chrome-extension://" + tapnowBackupID + "/"
	w = nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", "")
	if w.Code != 403 || calls != 1 {
		t.Fatal("backup reached kernel", w.Code, calls)
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/runtime/tapnow/native", strings.NewReader(`{}`))
	host.tapnowNativeAPI(w, r)
	if w.Code != 403 || calls != 1 {
		t.Fatal("browser reached kernel")
	}
}
