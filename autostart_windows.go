package main

import (
	"errors"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strings"
)

const startupRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupApprovedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

func installedRuntimeExecutable(exe string) bool {
	return strings.EqualFold(filepath.Clean(exe), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime", "ACU Runtime.exe"))
}
func startupEntryExists() bool {
	k, e := registry.OpenKey(registry.CURRENT_USER, startupRunKey, registry.QUERY_VALUE)
	if e != nil {
		return false
	}
	defer k.Close()
	_, _, e = k.GetStringValue("ACU Runtime")
	return e == nil
}
func readStartup() (bool, bool) {
	if !startupEntryExists() {
		return false, true
	}
	k, e := registry.OpenKey(registry.CURRENT_USER, startupApprovedKey, registry.QUERY_VALUE)
	if e == nil {
		defer k.Close()
		b, _, e := k.GetBinaryValue("ACU Runtime")
		if e == nil && len(b) > 0 && b[0]&1 == 1 {
			return false, true
		}
	}
	return true, true
}
func writeStartup(exe string, enabled bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, startupRunKey, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if !enabled {
		e = k.DeleteValue("ACU Runtime")
		if errors.Is(e, registry.ErrNotExist) {
			return nil
		}
		return e
	}
	if e = k.SetStringValue("ACU Runtime", `"`+exe+`" --background`); e != nil {
		return e
	}
	approved, e := registry.OpenKey(registry.CURRENT_USER, startupApprovedKey, registry.SET_VALUE)
	if errors.Is(e, registry.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	defer approved.Close()
	e = approved.DeleteValue("ACU Runtime")
	if errors.Is(e, registry.ErrNotExist) {
		return nil
	}
	return e
}
