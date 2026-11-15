package main

import (
	"context"
	"net/http"
	"testing"
)

func TestTapnowGuideCanvasActionOnlyOpensCanvas(t *testing.T) {
	host := nativeFixtureHost(t)
	opened := ""
	host.tapnow.openStore = func(_ context.Context, address string) error { opened = address; return nil }
	w := tapnowRequest(host, "POST", "/runtime/tapnow/canvas", `{"edition":"companion"}`, "")
	if w.Code != http.StatusForbidden || opened != "" {
		t.Fatal(w.Code, opened)
	}
	w = tapnowRequest(host, "POST", "/runtime/tapnow/canvas", `{"edition":"companion"}`, host.token)
	if w.Code != http.StatusOK || opened != "https://app.tapnow.ai/" || host.tapnow.grants != nil {
		t.Fatal(w.Code, opened)
	}
}

func TestTapnowClipboardCannotAcceptArbitraryContent(t *testing.T) {
	host := nativeFixtureHost(t)
	for _, path := range []string{"copy-path", "copy-address"} {
		for _, body := range []string{`{"edition":"backup","text":"untrusted"}`, `{"edition":"../escape"}`} {
			w := tapnowRequest(host, "POST", "/runtime/tapnow/"+path, body, host.token)
			if w.Code == http.StatusOK {
				t.Fatal("accepted arbitrary clipboard input", path, body)
			}
		}
		w := tapnowRequest(host, "POST", "/runtime/tapnow/"+path, `{"edition":"backup"}`, "")
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code)
		}
	}
	w := tapnowRequest(host, "POST", "/runtime/tapnow/copy-path", `{"edition":"backup"}`, host.token)
	if w.Code != http.StatusBadRequest {
		t.Fatal("copied an unprepared directory", w.Code)
	}
}
