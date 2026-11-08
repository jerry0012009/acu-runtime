package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

func installWorkBuddy(ctx context.Context, path string) error {
	dir, err := os.MkdirTemp("", "acu-workbuddy-app-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := exec.CommandContext(ctx, "/usr/bin/ditto", "-x", "-k", path, dir).Run(); err != nil {
		return errors.New("WorkBuddy 官方安装包解压失败")
	}
	app := filepath.Join(dir, "WorkBuddy.app")
	if err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).Run(); err != nil {
		return errors.New("WorkBuddy 应用签名校验失败，未安装")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	target := filepath.Join(home, "Applications", "WorkBuddy.app")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		return errors.New("WorkBuddy 已存在，未覆盖；请重新检测应用")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// Both paths are concrete bundles, and target was checked absent. Rename
	// doesn't merge/overwrite an existing user's application.
	staging, err := os.MkdirTemp(filepath.Dir(target), ".acu-workbuddy-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	bundle := filepath.Join(staging, "WorkBuddy.app")
	if err := exec.CommandContext(ctx, "/usr/bin/ditto", app, bundle).Run(); err != nil {
		return err
	}
	return os.Rename(bundle, target)
}

func openChatGPTInstaller(ctx context.Context) error {
	return openAgentURL(ctx, "https://chatgpt.com/download/")
}
func openAgentURL(ctx context.Context, address string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", address).Run()
}
func openInstalledAgent(ctx context.Context, id string) error {
	name := "WorkBuddy"
	if id == "codex" {
		name = "ChatGPT"
	} else {
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, "Applications", "WorkBuddy.app")
		if _, err := os.Stat(path); err == nil {
			return exec.CommandContext(ctx, "/usr/bin/open", path).Run()
		}
	}
	return exec.CommandContext(ctx, "/usr/bin/open", "-a", name).Run()
}

func workBuddyInstalled() bool {
	home, _ := os.UserHomeDir()
	for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
		if _, err := os.Stat(filepath.Join(dir, "WorkBuddy.app", "Contents", "Info.plist")); err == nil {
			return true
		}
	}
	return false
}
