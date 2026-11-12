package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

func startupPlist() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.acu.runtime.plist")
}
func installedRuntimeExecutable(exe string) bool {
	home, _ := os.UserHomeDir()
	clean := filepath.ToSlash(exe)
	return strings.HasPrefix(clean, "/Applications/ACU Runtime.app/Contents/MacOS/") || strings.HasPrefix(clean, filepath.ToSlash(filepath.Join(home, "Applications", "ACU Runtime.app", "Contents", "MacOS"))+"/")
}
func startupEntryExists() bool  { _, err := os.Stat(startupPlist()); return err == nil }
func readStartup() (bool, bool) { return startupEntryExists(), true }
func writeStartup(exe string, enabled bool) error {
	file := startupPlist()
	if !enabled {
		err := os.Remove(file)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	var path strings.Builder
	xml.EscapeText(&path, []byte(exe))
	return os.WriteFile(file, []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>com.acu.runtime</string><key>ProgramArguments</key><array><string>`+path.String()+`</string><string>--background</string></array><key>RunAtLoad</key><true/></dict></plist>`), 0600)
}
