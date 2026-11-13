package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Opt-in only on disposable release runners. Never install or replace apps
// as a side effect of the normal developer test suite.
func TestOfficialWorkBuddyInstallSmoke(t *testing.T) {
	if os.Getenv("ACU_AGENT_INSTALL_SMOKE") != "1" {
		t.Skip("requires disposable installation runner")
	}
	if workBuddyInstalled() {
		t.Fatal("smoke runner already has WorkBuddy; refusing to alter it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pkg, err := latestWorkBuddyPackage(ctx, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), pkg.Name)
	if err := downloadAgentPackage(ctx, agentDownloadClient(), pkg.URL, path, pkg.SHA256, nil); err != nil {
		t.Fatal(err)
	}
	if err := installWorkBuddy(ctx, path); err != nil {
		t.Fatal(err)
	}
	if !workBuddyInstalled() {
		t.Fatal("installer returned but desktop executable/bundle was not found")
	}
	t.Log("official package installed without modifying sign-in or client credentials")
}

func TestOfficialChatGPTInstallSmoke(t *testing.T) {
	if os.Getenv("ACU_CHATGPT_INSTALL_SMOKE") != "1" {
		t.Skip("requires disposable installation runner")
	}
	if chatGPTInstalled() {
		t.Fatal("runner already has ChatGPT; refusing to alter it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pkg, err := chatGPTPackage(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), pkg.Name)
	if err = prepareLatestChatGPT(ctx, runtime.GOOS, runtime.GOARCH, path, nil, func(s string) { t.Log(s) }); err != nil {
		t.Fatal(err)
	}
	if err = verifyChatGPTPackage(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err = installChatGPT(ctx, path); err != nil {
		t.Fatal(err)
	}
	if !chatGPTInstalled() {
		t.Fatal("app registration missing after installation")
	}
	if runtime.GOOS == "windows" {
		target, err := chatGPTPackageVersion(path)
		if err != nil {
			t.Fatal(err)
		}
		host := &server{}
		installed, err := host.verifyDesktopUpdate(ctx, "chatgpt", target)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("registered ChatGPT package verified:", installed.Version)
	}
	t.Log("official signed ChatGPT package installed on a clean profile")
}
