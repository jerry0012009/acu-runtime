package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Opt-in on disposable desktop release runners only. Normal tests never modify
// the user's clipboard; preserve the runner's prior text when available.
func TestTapnowNativeClipboardSmoke(t *testing.T) {
	if os.Getenv("ACU_TAPNOW_CLIPBOARD_SMOKE") != "1" {
		t.Skip("clipboard smoke is opt-in")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("desktop clipboard platforms only")
	}
	read := func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", `[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); [Console]::Write((Get-Clipboard -Raw -ErrorAction Stop))`)
			hideProcess(cmd)
		} else {
			cmd = exec.CommandContext(ctx, "/usr/bin/pbpaste")
		}
		body, err := cmd.Output()
		return strings.TrimRight(string(body), "\r\n"), err
	}
	previous, err := read()
	if err == nil {
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = copyTapnowText(ctx, previous)
		})
	}
	for _, value := range []string{"chrome://extensions/", `C:\Users\测试 用户\TapNow 插件\companion`} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := copyTapnowText(ctx, value)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		actual, err := read()
		if err != nil || actual != value {
			t.Fatalf("clipboard mismatch: %q != %q (%v)", actual, value, err)
		}
	}
}
