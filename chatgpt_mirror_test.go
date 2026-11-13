package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMirrorRejectsLagInvalidDigestAndExpiredMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, official, version, hash string
		age                           time.Duration
		want                          bool
	}{
		{"latest", "1.2.3.4", "1.2.3.4", base64.StdEncoding.EncodeToString(make([]byte, 32)), 0, true},
		{"lag", "1.2.3.5", "1.2.3.4", base64.StdEncoding.EncodeToString(make([]byte, 32)), 0, false},
		{"digest", "1.2.3.4", "1.2.3.4", "bad", 0, false},
		{"stableOfficialLatest", "1.2.3.4", "1.2.3.4", base64.StdEncoding.EncodeToString(make([]byte, 32)), 8 * 24 * time.Hour, true},
		{"expiredOffline", "", "1.2.3.4", base64.StdEncoding.EncodeToString(make([]byte, 32)), 8 * 24 * time.Hour, false},
		{"offlineOfficial", "", "1.2.3.4", base64.StdEncoding.EncodeToString(make([]byte, 32)), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{"packageIdentity":"OpenAI.Codex","buildVersion":%q}`, tc.official)
				status := 200
				if strings.Contains(r.URL.Path, "windows-store-update") {
					if tc.official == "" {
						status = 503
					}
				} else {
					body = fmt.Sprintf(`{"generatedAt":%q,"sources":{"windows":{"architectures":{"x64":{"version":%q,"contentLength":123,"downloadable":true,"catalog":{"hash":%q}}}}}}`, time.Now().Add(-tc.age).UTC().Format(time.RFC3339), tc.version, tc.hash)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			choice, err := selectChatGPTMirror(context.Background(), client, "amd64")
			if (err == nil) != tc.want {
				t.Fatal(choice, err)
			}
			if tc.want && choice.URL != "https://codexapp.agentsmirror.com/latest/win-x64" {
				t.Fatal(choice)
			}
		})
	}
}

func TestMirrorPackageRequiresMatchingManifestAndDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.msix")
	f, _ := os.Create(path)
	z := zip.NewWriter(f)
	w, _ := z.Create("AppxManifest.xml")
	io.WriteString(w, `<Package><Identity Version="1.2.3.4"/></Package>`)
	z.Close()
	f.Close()
	data, _ := os.ReadFile(path)
	h := sha256.Sum256(data)
	choice := chatGPTDownloadChoice{Version: "1.2.3.4", SHA256: hex.EncodeToString(h[:]), Size: int64(len(data))}
	if err := verifyChatGPTDownload(path, choice); err != nil {
		t.Fatal(err)
	}
	if err := requireChatGPTTarget(path, "1.2.3.5"); err == nil || !strings.Contains(err.Error(), "下载包仍为 1.2.3.4") {
		t.Fatalf("stale complete package accepted: %v", err)
	}
	if err := requireChatGPTTarget(path, "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if err := requireChatGPTTarget(path, "1.2.3.3"); err != nil {
		t.Fatal("newer package rejected", err)
	}
	choice.Version = "1.2.3.5"
	if verifyChatGPTDownload(path, choice) == nil {
		t.Fatal("accepted stale signed-package candidate")
	}
	choice.Version = "1.2.3.4"
	choice.SHA256 = strings.Repeat("0", 64)
	if verifyChatGPTDownload(path, choice) == nil {
		t.Fatal("accepted corrupt bytes")
	}
}

func TestOfficialFullPackageLagIsRejectedBeforeDownload(t *testing.T) {
	for _, tc := range []struct {
		version string
		status  int
		reject  bool
	}{
		{"26.1106.6628.0", 200, true}, {"26.1111.2228.0", 200, false}, {"26.1112.1.0", 200, false}, {"", 200, false}, {"invalid", 200, false}, {"26.1106.6628.0", 503, false},
	} {
		client := &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method != "HEAD" {
				t.Fatal("preflight downloaded full package")
			}
			return &http.Response{StatusCode: tc.status, Header: http.Header{"X-Ms-Meta-Package_version": []string{tc.version}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}
		err := checkChatGPTTargetSource(context.Background(), client, "https://persistent.oaistatic.com/codex-app-prod/ChatGPT-x64.msix", "26.1111.2228.0")
		if (err != nil) != tc.reject {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func TestLiveChatGPTUpdatePackageConsistency(t *testing.T) {
	if os.Getenv("ACU_CHATGPT_METADATA_ACCEPTANCE") != "1" {
		t.Skip("explicit read-only vendor metadata acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := chatGPTDownloadClient()
	var release struct {
		BuildVersion string `json:"buildVersion"`
	}
	if err := readChatGPTMetadata(ctx, client, "https://persistent.oaistatic.com/codex-app-prod/windows-store-update.json", &release); err != nil {
		t.Fatal(err)
	}
	if !validAppxVersion(release.BuildVersion) {
		t.Fatal("invalid vendor target")
	}
	// A lag is a valid diagnostic result; it must never be treated as an installed update.
	if err := checkChatGPTTargetSource(ctx, client, "https://persistent.oaistatic.com/codex-app-prod/ChatGPT-x64.msix", release.BuildVersion); err != nil {
		t.Log("blocked stale vendor package:", err)
		return
	}
	t.Log("no known package lag in metadata; signed-file and registered-version checks still required")
}

func TestMirrorRedirectAllowlist(t *testing.T) {
	for _, u := range []string{"https://codexapp.agentsmirror.com/latest/win-x64", "https://fgws3-ocloud.ihep.ac.cn/20830-codex/latest/win-x64?X-Amz-Signature=fixture"} {
		if !trustedChatGPTURL(u) {
			t.Fatal(u)
		}
	}
	for _, u := range []string{"http://codexapp.agentsmirror.com/latest/win-x64", "https://codexapp.agentsmirror.com.evil/latest/win-x64", "https://fgws3-ocloud.ihep.ac.cn/another-bucket/evil.exe", "https://user@codexapp.agentsmirror.com/latest/win-x64"} {
		if trustedChatGPTURL(u) {
			t.Fatal(u)
		}
	}
}
