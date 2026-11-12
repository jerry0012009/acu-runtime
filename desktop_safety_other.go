//go:build !windows

package main

func codexDesktopOpen() (bool, error) { return false, nil }
