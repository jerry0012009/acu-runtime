package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatGPTCompletePackageAllowlist(t *testing.T) {
	for _, platform := range []struct{ os, arch, suffix string }{{"windows", "amd64", "x64.msix"}, {"windows", "arm64", "arm64.msix"}, {"darwin", "arm64", ".dmg"}, {"darwin", "amd64", ".dmg"}} {
		pkg, err := chatGPTPackage(platform.os, platform.arch)
		if err != nil || !trustedChatGPTURL(pkg.URL) || !strings.HasSuffix(pkg.Name, platform.suffix) {
			t.Fatal(pkg, err)
		}
	}
	for _, source := range []string{"http://persistent.oaistatic.com/codex-app-prod/ChatGPT-x64.msix", "https://persistent.oaistatic.com.evil.test/codex-app-prod/ChatGPT-x64.msix", "https://persistent.oaistatic.com/codex-app-prod/other.exe", "https://persistent.oaistatic.com/codex-app-prod/ChatGPT-x64.msix?url=evil", "https://user@persistent.oaistatic.com/codex-app-prod/ChatGPT-x64.msix"} {
		if trustedChatGPTURL(source) {
			t.Fatal(source)
		}
	}
	if _, err := chatGPTPackage("linux", "amd64"); err == nil {
		t.Fatal("unsupported desktop package")
	}
}

func TestChatGPTDownloadKeepsCompleteCacheOnFailure(t *testing.T) {
	pkg, _ := chatGPTPackage("windows", "amd64")
	for _, length := range []int64{5, 100, 3 << 30} {
		target := filepath.Join(t.TempDir(), pkg.Name)
		os.WriteFile(target, []byte("old verified candidate"), 0600)
		client := &http.Client{Transport: agentPackageTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, ContentLength: length, Body: io.NopCloser(strings.NewReader("hello")), Header: make(http.Header)}, nil
		})}
		err := downloadChatGPTPackage(context.Background(), client, pkg.URL, target, nil)
		data, _ := os.ReadFile(target)
		if length == 5 {
			if err != nil || string(data) != "hello" {
				t.Fatal(err, string(data))
			}
		} else if err == nil || string(data) != "old verified candidate" {
			t.Fatal("failed download replaced complete cache", err)
		}
		files, _ := os.ReadDir(filepath.Dir(target))
		if len(files) != 1 {
			t.Fatal("partial package retained", files)
		}
	}
}

func TestChatGPTResumesSameReleaseAndSkipsUnchangedCompletedDownload(t *testing.T) {
	pkg, _ := chatGPTPackage("windows", "amd64")
	target := filepath.Join(t.TempDir(), pkg.Name)
	gets := 0
	client := &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		header := http.Header{"Etag": {`"release-1"`}}
		body, status, size := "", 200, int64(10)
		if r.Method == "GET" {
			gets++
			if gets == 1 {
				body = "hello"
			} else {
				if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"release-1"` {
					t.Fatal("did not resume same artifact", r.Header)
				}
				body, status, size = "world", 206, 5
				header.Set("Content-Range", "bytes 5-9/10")
			}
		}
		return &http.Response{StatusCode: status, ContentLength: size, Body: io.NopCloser(strings.NewReader(body)), Header: header}, nil
	})}
	if downloadChatGPTPackage(context.Background(), client, pkg.URL, target, nil) == nil {
		t.Fatal("truncated download succeeded")
	}
	if info, err := os.Stat(target + ".partial"); err != nil || info.Size() != 5 {
		t.Fatal("missing resumable partial", err)
	}
	if err := downloadChatGPTPackage(context.Background(), client, pkg.URL, target, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "helloworld" {
		t.Fatal(string(data))
	}
	if err := downloadChatGPTPackage(context.Background(), client, pkg.URL, target, nil); err != nil {
		t.Fatal(err)
	}
	if gets != 2 {
		t.Fatal("downloaded unchanged completed package again", gets)
	}
}

func TestChatGPTLatestChangeRestartsInsteadOfMixingReleases(t *testing.T) {
	pkg, _ := chatGPTPackage("windows", "amd64")
	target := filepath.Join(t.TempDir(), pkg.Name)
	os.WriteFile(target+".partial", []byte("old"), 0600)
	os.WriteFile(target+".download.json", []byte(`{"Source":"`+pkg.URL+`","ETag":"\"old\"","Size":10}`), 0600)
	client := &http.Client{Transport: agentPackageTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Range") != "" {
			t.Fatal("resumed stale release")
		}
		body := "new bytes"
		return &http.Response{StatusCode: 200, ContentLength: int64(len(body)), Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Etag": {`"new"`}}}, nil
	})}
	if err := downloadChatGPTPackage(context.Background(), client, pkg.URL, target, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new bytes" {
		t.Fatal("mixed release bytes", string(data))
	}
}
