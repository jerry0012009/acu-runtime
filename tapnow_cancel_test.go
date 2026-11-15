package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTapnowCancelStopsPreparationWithoutGrantOrBrowser(t *testing.T) {
	host := nativeFixtureHost(t)
	entered := make(chan struct{})
	host.tapnow.client = &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	opened := false
	host.tapnow.openStore = func(context.Context, string) error { opened = true; return nil }
	finished := make(chan int, 1)
	go func() {
		w := tapnowRequest(host, "POST", "/runtime/tapnow/install", `{"edition":"backup"}`, host.token)
		finished <- w.Code
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("prepare did not start")
	}
	if v := host.tapnow.snapshot(); v.Preparing != "backup" || !v.Busy || v.Phase != "checking" {
		t.Fatal(v)
	}
	w := tapnowRequest(host, "POST", "/runtime/tapnow/cancel", `{"edition":"backup"}`, host.token)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	select {
	case code := <-finished:
		if code != 400 {
			t.Fatal(code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not interrupt download")
	}
	if v := host.tapnow.snapshot(); v.Busy || v.Preparing != "" || v.Editions[1].Connection != "unknown" || opened {
		t.Fatal(v, opened)
	}
}

func TestTapnowReopenBrowserIsReadOnlyAndSessionProtected(t *testing.T) {
	host := nativeFixtureHost(t)
	opened := ""
	host.tapnow.openStore = func(_ context.Context, address string) error { opened = address; return nil }
	w := tapnowRequest(host, "POST", "/runtime/tapnow/browser", `{"edition":"backup"}`, "")
	if w.Code != 403 || opened != "" {
		t.Fatal(w.Code, opened)
	}
	w = tapnowRequest(host, "POST", "/runtime/tapnow/browser", `{"edition":"backup"}`, host.token)
	if w.Code != 200 || opened != "about:blank" || host.tapnow.grants != nil || !strings.Contains(w.Body.String(), `"manualNavigationRequired":true`) {
		t.Fatal(w.Code, opened)
	}
}

func TestTapnowCancelWaitingGrantRejectsLaterReceipt(t *testing.T) {
	host := nativeFixtureHost(t)
	w := tapnowRequest(host, "POST", "/runtime/tapnow/connect", `{"edition":"backup"}`, host.token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = tapnowRequest(host, "POST", "/runtime/tapnow/cancel", `{"edition":"backup"}`, host.token)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	msg := tapnowNativeMessage{Action: "connect", Origin: "chrome-extension://" + tapnowBackupID + "/", Version: "0.1.23"}
	if w := nativeAPIRequest(host, msg, host.tapnow.nativeToken, "", ""); w.Code != 409 {
		body, _ := io.ReadAll(w.Body)
		t.Fatal(w.Code, strings.TrimSpace(string(body)))
	}
}

func TestTapnowConnectNeverLaunchesExtensionPage(t *testing.T) {
	host := nativeFixtureHost(t)
	opened := ""
	host.tapnow.openStore = func(_ context.Context, address string) error { opened = address; return nil }
	w := tapnowRequest(host, "POST", "/runtime/tapnow/connect", `{"edition":"backup"}`, host.token)
	if w.Code != 200 || opened != "https://app.tapnow.ai/" || strings.Contains(w.Body.String(), "chrome-extension://") {
		t.Fatal(w.Code, opened, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "连接 ACU Runtime") {
		t.Fatal("missing popup handoff instruction")
	}
}
