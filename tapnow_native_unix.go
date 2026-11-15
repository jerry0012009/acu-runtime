//go:build !windows

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

func registerTapnowHost() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var dirs []string
	if runtime.GOOS == "darwin" {
		dirs = []string{filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts")}
	} else {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		for _, browser := range []string{"google-chrome", "chromium", "google-chrome-for-testing"} {
			dirs = append(dirs, filepath.Join(base, browser, "NativeMessagingHosts"))
		}
	}
	for _, dir := range dirs {
		if err := writeTapnowPrivateJSON(filepath.Join(dir, tapnowNativeName+".json"), tapnowHostManifest(exe)); err != nil {
			return err
		}
	}
	return nil
}

func copyTapnowText(ctx context.Context, value string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("clipboard unavailable")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/pbcopy")
	cmd.Stdin = strings.NewReader(value)
	return cmd.Run()
}

func openTapnowChrome(ctx context.Context, address string) error {
	if runtime.GOOS == "darwin" {
		// Launch Services treats chrome:// as an external protocol and Chrome
		// filters it. Supply only a normal launch target via application args.
		args := append([]string{"-a", "Google Chrome", "--args"}, tapnowChromeLaunchArgs(address)...)
		return exec.CommandContext(ctx, "/usr/bin/open", args...).Run()
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if exe, err := exec.LookPath(name); err == nil {
			cmd := exec.Command(exe, tapnowChromeLaunchArgs(address)...)
			if err := cmd.Start(); err != nil {
				return err
			}
			go cmd.Wait()
			return nil
		}
	}
	return errors.New("Google Chrome unavailable")
}

func tapnowChromeExecutable() (string, error) {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		for _, p := range []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", filepath.Join(home, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome")} {
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				return p, nil
			}
		}
	} else {
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				return p, nil
			}
		}
	}
	return "", errors.New("Google Chrome unavailable")
}
