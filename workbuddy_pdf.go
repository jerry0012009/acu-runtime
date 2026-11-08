package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// WorkBuddy's official PDF launcher supports WORKBUDDY_PDF, but its external
// package installer puts the executable outside the launcher's bundled/PATH
// lookup. Point only that supported setting at an already installed component.
// Never replace a user's override, edit the built-in skill, or relax permissions.
func prepareWorkBuddyPDF() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	dir := os.Getenv("WORKBUDDY_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(home, ".workbuddy")
	}
	return prepareWorkBuddyPDFIn(dir)
}

func prepareWorkBuddyPDFIn(dir string) error {
	cli := filepath.Join(dir, "external-packages", "pdf-cli", "workbuddy-pdf.exe")
	info, err := os.Stat(cli)
	if os.IsNotExist(err) {
		return nil // WorkBuddy has not installed its own component yet.
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("WorkBuddy PDF 组件路径不是文件")
	}
	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte(`{}`)
	} else if err != nil {
		return err
	}
	var settings map[string]json.RawMessage
	if json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &settings) != nil || settings == nil {
		return errors.New("WorkBuddy 配置格式无法识别，未修改")
	}
	var env map[string]json.RawMessage
	if raw, ok := settings["env"]; ok {
		if json.Unmarshal(raw, &env) != nil || env == nil {
			return errors.New("WorkBuddy 环境配置格式无法识别，未修改")
		}
	} else {
		env = make(map[string]json.RawMessage)
	}
	if _, exists := env["WORKBUDDY_PDF"]; exists || os.Getenv("WORKBUDDY_PDF") != "" {
		return nil
	}
	env["WORKBUDDY_PDF"], _ = json.Marshal(cli)
	settings["env"], _ = json.Marshal(env)
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".acu-pdf-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(append(updated, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	// Do not overwrite a setting changed while the replacement was prepared.
	latest, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if readErr == nil && !bytes.Equal(latest, data) {
		return errors.New("WorkBuddy 配置正在更新，请重试")
	}
	return os.Rename(file.Name(), path)
}

func (host *server) prepareAgentComponents(agents []string) error {
	if !slices.Contains(agents, "workbuddy") {
		return nil
	}
	host.kernel.mu.Lock()
	managed := host.kernel.exe != ""
	host.kernel.mu.Unlock()
	if !managed {
		return nil // Development/mock management connections never edit real files.
	}
	return prepareWorkBuddyPDF()
}

// Repair existing connections on Runtime upgrades too; don't configure an
// unrelated WorkBuddy installation that has never been connected to Magpie.
func repairConnectedWorkBuddyPDF() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	dir := os.Getenv("WORKBUDDY_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(home, ".workbuddy")
	}
	data, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var models []struct{ Vendor string }
	if json.Unmarshal(data, &models) != nil {
		return nil
	}
	for _, m := range models {
		if m.Vendor == "magpie" {
			return prepareWorkBuddyPDFIn(dir)
		}
	}
	return nil
}
