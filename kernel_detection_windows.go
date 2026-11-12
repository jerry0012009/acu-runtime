package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

// Do not invoke wsl.exe to find out whether WSL is installed: that probe is
// itself capable of showing Windows' installation window. Registry errors are
// inconclusive, so leave the user's kernel setting alone in that case.
func noRegisteredWSLDistros() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Lxss`, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	return err == nil && len(names) == 0
}

func prepareKernelDetection(ctx context.Context, executable string) error {
	if !noRegisteredWSLDistros() {
		return nil
	}
	if kernelSettingsMatch(executable, func(settings map[string]json.RawMessage) bool {
		var disabled bool
		return json.Unmarshal(settings["noWSLAgents"], &disabled) == nil && disabled
	}) {
		return nil
	}
	command := exec.CommandContext(ctx, executable, "settings", "set", "noWSLAgents", "on")
	hideProcess(command)
	if err := command.Run(); err != nil {
		return errors.New("无法关闭无发行版的 WSL 探测，请重试")
	}
	return nil
}
