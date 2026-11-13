package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type updateDownloadTransport struct {
	t          *testing.T
	content    string
	calls      []string
	failMirror bool
}

func (transport *updateDownloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls = append(transport.calls, request.URL.Host)
	deadline, ok := request.Context().Deadline()
	if !ok {
		transport.t.Fatal("download must be bounded")
	}
	remaining := time.Until(deadline)
	if request.URL.Host == "github.com" {
		if remaining > time.Minute {
			transport.t.Fatal("blocked direct downloads must fall back promptly")
		}
	} else if remaining < 4*time.Minute {
		transport.t.Fatal("healthy mirror download must not be limited to 45 seconds")
	}
	body := transport.content
	if transport.failMirror && request.URL.Host == "gh-proxy.com" {
		body = "corrupted mirror installer"
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)),
		Header: make(http.Header), ContentLength: int64(len(body)), Request: request}, nil
}

func TestUpdatePrefersAcceleratedDownloadAndFallsBackOnDigestMismatch(t *testing.T) {
	for _, failMirror := range []bool{false, true} {
		t.Run(fmt.Sprint("corrupt=", failMirror), func(t *testing.T) {
			content := "official installer bytes"
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			transport := &updateDownloadTransport{t: t, content: content, failMirror: failMirror}
			client := &http.Client{Transport: transport}
			url := updateRepository + "/releases/download/v0.1.99/ACU-Runtime-0.1.99-windows-amd64-Setup.exe"
			target := filepath.Join(t.TempDir(), "setup.exe")
			if err := downloadUpdatePackage(context.Background(), client, updateDownloadSources(url), target, digest, nil); err != nil {
				t.Fatal(err)
			}
			want := "gh-proxy.com"
			if failMirror {
				want += ",github.com"
			}
			if strings.Join(transport.calls, ",") != want || !verifyKernel(target, digest) {
				t.Fatalf("download source order or verified output incorrect: %v", transport.calls)
			}
		})
	}
}

func TestUpdateFallbackChecksAndVerifiesDownloads(t *testing.T) {
	content := []byte("verified installer payload")
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/offline":
			http.Error(w, "unavailable", 503)
		case "/release":
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.18", "assets": []any{map[string]any{"name": "ACU-Runtime-0.1.18-windows-amd64-Setup.exe", "browser_download_url": updateRepository + "/releases/download/v0.1.18/ACU-Runtime-0.1.18-windows-amd64-Setup.exe", "digest": "sha256:" + digest, "size": len(content)}}})
		case "/corrupt":
			w.Write([]byte("not the installer"))
		case "/package":
			w.Write(content)
		}
	}))
	defer server.Close()
	release, err := checkUpdateWithFallback(context.Background(), server.Client(), []string{server.URL + "/offline", server.URL + "/release"}, "0.1.17", "windows", "amd64")
	if err != nil || release == nil || release.Version != "0.1.18" {
		t.Fatalf("fallback check: %#v %v", release, err)
	}
	target := filepath.Join(t.TempDir(), "installer.exe")
	err = downloadUpdatePackage(context.Background(), server.Client(), []string{server.URL + "/offline", server.URL + "/corrupt", server.URL + "/package"}, target, digest, nil)
	if err != nil || !verifyKernel(target, digest) {
		t.Fatalf("fallback download not verified: %v", err)
	}
	if strings.Join(calls, ",") != "/offline,/release,/offline,/corrupt,/package" {
		t.Fatalf("unexpected fallback order: %v", calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := downloadUpdatePackage(ctx, server.Client(), []string{server.URL + "/package"}, target, digest, nil); err == nil {
		t.Fatal("cancelled update continued")
	}
}

func TestNewerVersion(t *testing.T) {
	for _, test := range []struct {
		candidate, current string
		want               bool
	}{
		{"0.1.15", "0.1.14", true}, {"v0.2.0", "0.1.99", true}, {"0.1.14", "0.1.14", false},
		{"0.1.13", "0.1.14", false}, {"v0.1.15-rc1", "0.1.14", false}, {"latest", "0.1.14", false},
	} {
		if got := newerVersion(test.candidate, test.current); got != test.want {
			t.Errorf("newerVersion(%q, %q) = %v", test.candidate, test.current, got)
		}
	}
}

func TestCheckUpdateSelectsExactPlatformInstallerAndDigest(t *testing.T) {
	name := "ACU-Runtime-0.1.15-windows-amd64-Setup.exe"
	digest := strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.15", "assets": []any{map[string]any{
			"name": name, "browser_download_url": updateRepository + "/releases/download/v0.1.15/" + name, "digest": "sha256:" + digest, "size": 10,
		}}})
	}))
	defer server.Close()
	release, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.14", "windows", "amd64")
	if err != nil || release == nil || release.Version != "0.1.15" || release.SHA256 != digest {
		t.Fatalf("update was not accepted: %#v %v", release, err)
	}
}

func TestCheckUpdateRejectsUntrustedOrUnsupportedReleaseAssets(t *testing.T) {
	for _, asset := range []map[string]any{
		{"name": "ACU-Runtime-0.1.15-windows-amd64-Setup.exe", "browser_download_url": "https://attacker.invalid/update.exe", "digest": "sha256:" + strings.Repeat("a", 64), "size": 10},
		{"name": "ACU-Runtime-0.1.15-windows-amd64-Setup.exe", "browser_download_url": updateRepository + "/releases/download/v0.1.15/ACU-Runtime-0.1.15-windows-amd64-Setup.exe", "digest": "", "size": 10},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.15", "assets": []any{asset}})
		}))
		if _, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.14", "windows", "amd64"); err == nil {
			t.Fatal("untrusted release asset accepted")
		}
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.15", "prerelease": true})
	}))
	defer server.Close()
	if _, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.14", "windows", "amd64"); err == nil {
		t.Fatal("prerelease accepted")
	}
}

func TestCheckUpdateReportsCurrentAndUnsupportedPlatforms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.14"})
	}))
	defer server.Close()
	if release, err := checkUpdate(context.Background(), server.Client(), server.URL, appVersion, "windows", "amd64"); err != nil || release != nil {
		t.Fatalf("current release check: %#v %v", release, err)
	}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.15", "assets": []any{}})
	}))
	defer server.Close()
	if _, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.14", "darwin", "arm64"); err == nil {
		t.Fatal("missing platform installer reported as available")
	}
}

func TestUpdateSupportIsLimitedToWindowsAmd64Desktop(t *testing.T) {
	state := updateState{}
	if state.snapshot(true).Supported != (runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Fatal("application update support does not match the platform")
	}
	if state.snapshot(false).Supported {
		t.Fatal("web mode must not offer application updates")
	}
}

func TestParseWindowsSystemProxy(t *testing.T) {
	for _, test := range []struct{ value, scheme, want string }{
		{"127.0.0.1:7897", "https", "http://127.0.0.1:7897"},
		{"http=localhost:8000;https=localhost:9000", "https", "http://localhost:9000"},
		{"http=localhost:8000", "https", ""},
		{"file:///bad", "https", ""},
		{"", "https", ""},
	} {
		got := parseSystemProxy(test.value, test.scheme)
		text := ""
		if got != nil {
			text = got.String()
		}
		if text != test.want {
			t.Fatalf("parseSystemProxy(%q): %q, want %q", test.value, text, test.want)
		}
	}
}
