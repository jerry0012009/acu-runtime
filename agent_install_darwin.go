package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func withChatGPTDMG(ctx context.Context, path string, use func(string) error) error {
	dir, err := os.MkdirTemp("", "acu-chatgpt-dmg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if exec.CommandContext(ctx, "/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", dir, path).Run() != nil {
		return errors.New("ChatGPT 官方安装包无法打开，请重新下载")
	}
	defer exec.Command("/usr/bin/hdiutil", "detach", dir).Run()
	app := filepath.Join(dir, "ChatGPT.app")
	if exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).Run() != nil || exec.CommandContext(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", app).Run() != nil {
		return errors.New("ChatGPT 官方签名或公证校验失败，未安装")
	}
	info, err := exec.CommandContext(ctx, "/usr/bin/codesign", "-dv", "--verbose=4", app).CombinedOutput()
	if err != nil || !strings.Contains(string(info), "Authority=Developer ID Application: OpenAI (") {
		return errors.New("ChatGPT 发布者验证失败，未安装")
	}
	binary := filepath.Join(app, "Contents", "MacOS", "ChatGPT")
	arch := "arm64"
	if runtime.GOARCH == "amd64" {
		arch = "x86_64"
	}
	if exec.CommandContext(ctx, "/usr/bin/lipo", "-verify_arch", arch, binary).Run() != nil {
		return errors.New("当前官方 ChatGPT 安装包不支持此 Mac 架构，请使用 WorkBuddy")
	}
	return use(app)
}

func verifyChatGPTPackage(ctx context.Context, path string) error {
	return withChatGPTDMG(ctx, path, func(string) error { return nil })
}

func chatGPTInstalled() bool {
	home, _ := os.UserHomeDir()
	for _, path := range []string{"/Applications/ChatGPT.app", filepath.Join(home, "Applications", "ChatGPT.app")} {
		if regularFile(filepath.Join(path, "Contents", "Info.plist")) {
			return true
		}
	}
	return false
}

func installChatGPT(ctx context.Context, path string) error {
	return withChatGPTDMG(ctx, path, func(app string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		for _, existing := range []string{"/Applications/ChatGPT.app", filepath.Join(home, "Applications", "ChatGPT.app")} {
			if _, err := os.Stat(existing); err == nil {
				return errors.New("ChatGPT 已存在，请使用客户端内更新；需要手动更新时，官方完整安装包已下载到本地缓存")
			}
		}
		target := filepath.Join(home, "Applications", "ChatGPT.app")
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		staging, err := os.MkdirTemp(filepath.Dir(target), ".acu-chatgpt-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		bundle := filepath.Join(staging, "ChatGPT.app")
		if err := exec.CommandContext(ctx, "/usr/bin/ditto", app, bundle).Run(); err != nil {
			return err
		}
		return os.Rename(bundle, target)
	})
}
func openAgentURL(ctx context.Context, address string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", address).Run()
}
func openInstalledAgent(ctx context.Context, id string) error {
	name := "WorkBuddy"
	if id == "codex" {
		name = "ChatGPT"
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, "Applications", "ChatGPT.app")
		if _, err := os.Stat(path); err == nil {
			return exec.CommandContext(ctx, "/usr/bin/open", path).Run()
		}
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
