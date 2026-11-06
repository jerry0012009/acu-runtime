package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

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
