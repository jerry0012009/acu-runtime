//go:build darwin

package main

import (
	"context"
	"os/exec"
)

func openTapnowDirectory(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", path).Run()
}
