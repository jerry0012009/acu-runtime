//go:build !windows && !darwin

package main

func codexDesktopOpen() (bool, error) { return false, nil }
