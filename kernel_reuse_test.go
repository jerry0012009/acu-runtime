package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPauseUpdatesUsesNativeCommandsOnly(t *testing.T) {
	var calls [][]string
	if err := pauseKernelUpdatesWith(func(args []string) error { calls = append(calls, args); return nil }); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"update", "auto", "off", "notify"}, {"settings", "set", "autoRestart", "off"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected native commands: %v", calls)
	}
	if err := pauseKernelUpdatesWith(func([]string) error { return errors.New("private diagnostic") }); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("policy failure was ignored or leaked command output")
	}
}

func TestExistingManagementIsAuthenticatedAndNeverOwned(t *testing.T) {
	var authenticated bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			authenticated = r.URL.Query().Get("k") == "local-secret"
		case "/api/state":
			if !authenticated {
				w.WriteHeader(401)
				return
			}
			fmt.Fprint(w, `{"agents":[]}`)
		}
	}))
	defer remote.Close()
	kernel, _ := newKernel("")
	if !kernel.attachExisting(context.Background(), remote.URL+"/?k=local-secret") || kernel.process != nil {
		t.Fatal("existing management was not reused without a child process")
	}
	kernel.close()
	if _, err := kernel.readState(context.Background()); err != nil {
		t.Fatal("closing Runtime stopped external management")
	}
}

func TestExistingManagementFailureRestoresPreviousConnection(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"unrelated":true}`) }))
	defer remote.Close()
	kernel, _ := newKernel("http://127.0.0.1:4317")
	kernel.key = "previous"
	if kernel.attachExisting(context.Background(), remote.URL+"/?k=test") || kernel.endpoint.String() != "http://127.0.0.1:4317" || kernel.key != "previous" {
		t.Fatal("unrelated service replaced the working connection")
	}
}

func TestBrowserRecordAndRedirectNeverLeaveLoopback(t *testing.T) {
	for _, link := range []string{"http://example.com/?k=x", "https://127.0.0.1:3430/?k=x", "http://127.0.0.1:3430/api?k=x", "http://u@127.0.0.1:3430/?k=x", "http://127.0.0.1:3430/?k=x&other=y", "http://127.0.0.1:3430/"} {
		if _, _, ok := kernelLink(link); ok {
			t.Fatalf("accepted unsafe link %s", link)
		}
	}
	dir := t.TempDir()
	for _, mode := range []string{"", "local", "lan", "invalid"} {
		data, _ := json.Marshal(map[string]any{"mode": mode, "key": "private key"})
		os.WriteFile(filepath.Join(dir, "web-page.json"), data, 0600)
		link := browserKernelLink(dir)
		if mode == "local" || mode == "lan" {
			u, key, ok := kernelLink(link)
			if !ok || u.Host != "127.0.0.1:3430" || key != "private key" {
				t.Fatal("local record was not parsed")
			}
		} else if link != "" {
			t.Fatal("disabled browser access was enabled")
		}
	}
	output := &kernelOutput{}
	output.Write([]byte("existing service \x1b[1mhttp://127.0.0.1:3430/?k=" + url.QueryEscape("a b") + "\x1b[0m\n"))
	if len(output.links()) != 1 {
		t.Fatal("native service redirect was lost")
	}
	output.Write([]byte(strings.Repeat("x", 32768)))
	if len(output.text) > 16384 {
		t.Fatal("unbounded private command output")
	}
}
