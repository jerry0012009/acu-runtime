//go:build !darwin

package main

import (
	"context"
	"errors"
)

func (host *server) downloadMacUpdate(context.Context, *updateRelease) error {
	return errors.New("macOS updater unavailable on this platform")
}
func applyMacUpdate(string) error       { return errors.New("macOS updater unavailable on this platform") }
func verifySignedMacKernel(string) bool { return false }
