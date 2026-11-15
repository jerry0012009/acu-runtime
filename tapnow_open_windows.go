//go:build windows

package main

import (
	"context"
	"os/exec"
)

func openTapnowDirectory(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "explorer.exe", path)
	hideProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
