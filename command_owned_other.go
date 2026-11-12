//go:build !windows

package main

import (
	"os/exec"
)

func startOwnedCommand(c *exec.Cmd) (func(), error) {
	if err := c.Start(); err != nil {
		return nil, err
	}
	return func() { _ = c.Process.Kill() }, nil
}
