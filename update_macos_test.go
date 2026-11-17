package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMacUpdateSelectsMatchingArchitectureZip(t *testing.T) {
	for _, arch := range []string{"arm64", "amd64"} {
		t.Run(arch, func(t *testing.T) {
			name := "ACU-Runtime-0.2.0-macos-" + arch + ".app.zip"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.2.0", "assets": []any{map[string]any{"name": name, "browser_download_url": updateRepository + "/releases/download/v0.2.0/" + name, "digest": "sha256:" + strings.Repeat("a", 64), "size": 100}}})
			}))
			defer server.Close()
			release, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.112", "darwin", arch)
			if err != nil || release == nil || !strings.HasSuffix(release.URL, "macos-"+arch+".app.zip") {
				t.Fatalf("Mac update not selected: %+v %v", release, err)
			}
		})
	}
}

func TestMacUpdateRejectsWrongOriginDigestAndArchitecture(t *testing.T) {
	name := "ACU-Runtime-0.2.0-macos-arm64.app.zip"
	for _, tc := range []struct{ name, url, digest string }{
		{name, "https://untrusted.example/" + name, "sha256:" + strings.Repeat("a", 64)},
		{name, updateRepository + "/releases/download/v0.2.0/" + name, ""},
		{"ACU-Runtime-0.2.0-macos-amd64.app.zip", updateRepository + "/releases/download/v0.2.0/ACU-Runtime-0.2.0-macos-amd64.app.zip", "sha256:" + strings.Repeat("a", 64)},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.2.0", "assets": []any{map[string]any{"name": tc.name, "browser_download_url": tc.url, "digest": tc.digest, "size": 100}}})
		}))
		_, err := checkUpdate(context.Background(), server.Client(), server.URL, "0.1.112", "darwin", "arm64")
		server.Close()
		if err == nil {
			t.Fatal("untrusted or incorrect Mac package accepted")
		}
	}
}
