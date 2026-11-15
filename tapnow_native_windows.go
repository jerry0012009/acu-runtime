package main

import (
	"context"
	"errors"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
)

func registerTapnowHost() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir, err := (&tapnowState{}).directory()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, tapnowNativeName+".json")
	if err := writeTapnowPrivateJSON(path, tapnowHostManifest(exe)); err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Google\Chrome\NativeMessagingHosts\`+tapnowNativeName, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("", path)
}

func tapnowChromeExecutable() (string, error) {
	paths := []string{}
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(hive, `Software\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
		if err == nil {
			value, _, e := key.GetStringValue("")
			key.Close()
			if e == nil {
				paths = append(paths, value)
			}
		}
	}
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base != "" {
			paths = append(paths, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	for _, exe := range paths {
		if info, err := os.Stat(exe); err == nil && info.Mode().IsRegular() {
			return exe, nil
		}
	}
	return "", errors.New("Google Chrome unavailable")
}

func openTapnowChrome(_ context.Context, address string) error {
	exe, err := tapnowChromeExecutable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, tapnowChromeLaunchArgs(address)...)
	hideProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
