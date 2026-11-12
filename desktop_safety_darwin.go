package main

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// macOS retains the close-before-configure path until its native quiet
// transaction has separate acceptance. Inspect without launching an app.
func codexDesktopOpen() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "comm=").Output()
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.ToLower(string(b)), "\n") {
		if strings.Contains(line, "/chatgpt.app/contents/") || strings.Contains(line, "/codex.app/contents/") {
			return true, nil
		}
	}
	return false, nil
}
