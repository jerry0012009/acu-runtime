package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestKernelInstallVerifiedAndCached(t *testing.T) {
	content := "isolated kernel fixture, not an executable"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		fmt.Fprint(response, content)
	}))
	defer remote.Close()
	target := filepath.Join(t.TempDir(), "kernel", "acu-kernel")
	var downloaded int64
	for attempt := 0; attempt < 2; attempt++ {
		installed, err := installKernel(context.Background(), remote.Client(), remote.URL, target, digest, func(count, total int64) { downloaded = count })
		if err != nil || installed != target || !verifyKernel(target, digest) {
			t.Fatalf("install failed: %s %v", installed, err)
		}
	}
	if requests.Load() != 1 || downloaded != int64(len(content)) {
		t.Fatal("cached installation was downloaded again or progress was lost")
	}
}

func TestKernelInstallRejectsBadDownload(t *testing.T) {
	for _, status := range []int{200, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(status)
				fmt.Fprint(response, "corrupted")
			}))
			defer remote.Close()
			directory := t.TempDir()
			target := filepath.Join(directory, "acu-kernel")
			if _, err := installKernel(context.Background(), remote.Client(), remote.URL, target, strings.Repeat("0", 64), nil); err == nil {
				t.Fatal("untrusted package installed")
			}
			entries, _ := os.ReadDir(directory)
			if len(entries) != 0 {
				t.Fatal("failed download left executable or temporary files")
			}
		})
	}
}

func TestKernelInstallCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := filepath.Join(t.TempDir(), "acu-kernel")
	if _, err := installKernel(ctx, http.DefaultClient, "http://127.0.0.1:1", target, strings.Repeat("0", 64), nil); err == nil {
		t.Fatal("cancelled install succeeded")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("cancelled install left a binary")
	}
}

func TestKernelPlatformPackages(t *testing.T) {
	for _, platform := range []string{"windows", "darwin"} {
		for _, architecture := range []string{"amd64", "arm64"} {
			release, err := releaseFor(platform, architecture)
			if err != nil || len(release.sha256) != 64 || !strings.Contains(release.name, platform+"-"+architecture) {
				t.Fatal("missing pinned platform package")
			}
		}
	}
	if _, err := releaseFor("unknown", "amd64"); err == nil {
		t.Fatal("unknown platform accepted")
	}
}

func TestSetupRefreshesFallbackWhenExternalKernelExists(t *testing.T) {
	source, target := t.TempDir(), filepath.Join(t.TempDir(), "installed")
	name := filepath.Base(managedKernelPath())
	content := []byte("new verified kernel")
	release := kernelRelease{sha256: fmt.Sprintf("%x", sha256.Sum256(content))}
	if err := os.MkdirAll(filepath.Join(source, "kernel"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "kernel"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "kernel", name), content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "kernel", name), []byte("old fallback kernel"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "ACU Runtime.exe"), []byte("application fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "user-config.json"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareInstallation(source, target, release); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "ACU Runtime.exe")); err != nil {
		t.Fatal(err)
	}
	if !verifyKernel(filepath.Join(target, "kernel", name), release.sha256) {
		t.Fatal("updated fallback kernel missing despite external kernel")
	}
	if contents, err := os.ReadFile(filepath.Join(target, "user-config.json")); err != nil || string(contents) != "preserve" {
		t.Fatal("updating the kernel modified user configuration")
	}
	notice, err := os.ReadFile(filepath.Join(target, "THIRD-PARTY-NOTICES.txt"))
	if err != nil || !strings.Contains(string(notice), "Magpie") || !strings.Contains(string(notice), "MIT License") {
		t.Fatal("local third-party license missing")
	}
}

func TestSetupIncludesVerifiedKernelWhenMissing(t *testing.T) {
	source, target := t.TempDir(), filepath.Join(t.TempDir(), "installed")
	name := filepath.Base(managedKernelPath())
	if err := os.MkdirAll(filepath.Join(source, "kernel"), 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("kernel fixture")
	os.WriteFile(filepath.Join(source, "kernel", name), content, 0600)
	os.WriteFile(filepath.Join(source, "ACU Runtime.exe"), []byte("app fixture"), 0600)
	release := kernelRelease{sha256: fmt.Sprintf("%x", sha256.Sum256(content))}
	if err := prepareInstallation(source, target, release); err != nil {
		t.Fatal(err)
	}
	if !verifyKernel(filepath.Join(target, "kernel", name), release.sha256) {
		t.Fatal("verified kernel missing from installation")
	}
}

func TestSetupRejectsCorruptBundleBeforeChangingInstallation(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	name := filepath.Base(managedKernelPath())
	for _, directory := range []string{source, target} {
		if err := os.MkdirAll(filepath.Join(directory, "kernel"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(source, "kernel", name), []byte("corrupt"), 0600)
	os.WriteFile(filepath.Join(source, "ACU Runtime.exe"), []byte("new app"), 0600)
	os.WriteFile(filepath.Join(target, "kernel", name), []byte("old kernel"), 0600)
	os.WriteFile(filepath.Join(target, "ACU Runtime.exe"), []byte("old app"), 0600)
	if err := prepareInstallation(source, target, kernelRelease{sha256: strings.Repeat("0", 64)}); err == nil {
		t.Fatal("corrupt bundle accepted")
	}
	for file, want := range map[string]string{"ACU Runtime.exe": "old app", filepath.Join("kernel", name): "old kernel"} {
		got, err := os.ReadFile(filepath.Join(target, file))
		if err != nil || string(got) != want {
			t.Fatalf("failed update changed %s", file)
		}
	}
}

func TestSetupRollsBackKernelWhenApplicationReplacementFails(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	name := filepath.Base(managedKernelPath())
	for _, directory := range []string{source, target} {
		if err := os.MkdirAll(filepath.Join(directory, "kernel"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	kernel := []byte("new kernel")
	digest := fmt.Sprintf("%x", sha256.Sum256(kernel))
	if err := os.WriteFile(filepath.Join(source, "kernel", name), kernel, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "kernel", name), []byte("old kernel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "ACU Runtime.exe"), []byte("old app"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareInstallation(source, target, kernelRelease{sha256: digest}); err == nil {
		t.Fatal("missing application unexpectedly installed")
	}
	if got, err := os.ReadFile(filepath.Join(target, "kernel", name)); err != nil || string(got) != "old kernel" {
		t.Fatal("failed update did not restore the old kernel")
	}
	if got, err := os.ReadFile(filepath.Join(target, "ACU Runtime.exe")); err != nil || string(got) != "old app" {
		t.Fatal("failed update changed the current application")
	}
}
