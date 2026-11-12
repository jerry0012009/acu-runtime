//go:build !windows

package main

import "context"

func installedDesktopVersions(context.Context) map[string]string { return nil }
