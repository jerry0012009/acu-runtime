//go:build !windows

package main

import "os"

func replaceInstalledPath(source, target string) error { return os.Rename(source, target) }
