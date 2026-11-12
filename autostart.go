package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type startupPreference struct {
	Enabled   bool   `json:"enabled"`
	Supported bool   `json:"supported"`
	Error     string `json:"error,omitempty"`
}

func startupPreferencePath() string {
	cache, _ := os.UserCacheDir()
	return filepath.Join(cache, "ACURuntime", "startup.json")
}
func initializeStartup() error {
	exe, err := os.Executable()
	if err != nil || !installedRuntimeExecutable(exe) {
		return nil
	}
	file := startupPreferencePath()
	if _, err = os.Stat(file); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	enabled, supported := readStartup()
	if !supported {
		return nil
	}
	// First installation defaults on. An existing system entry is respected.
	if !startupEntryExists() {
		enabled = true
		if err = writeStartup(exe, true); err != nil {
			return err
		}
	}
	return saveStartupPreference(enabled)
}
func saveStartupPreference(enabled bool) error {
	return writeStartupPreference(startupPreferencePath(), enabled)
}
func writeStartupPreference(file string, enabled bool) error {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]bool{"enabled": enabled})
	return os.WriteFile(file, b, 0600)
}
func (host *server) startupSettings(w http.ResponseWriter, r *http.Request) {
	enabled, supported := readStartup()
	if r.Method == http.MethodGet {
		host.json(w, 200, startupPreference{Enabled: enabled, Supported: supported})
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil || in.Enabled == nil {
		host.json(w, 400, map[string]string{"error": "请选择开机自启动状态"})
		return
	}
	exe, err := os.Executable()
	if err != nil || !supported || !installedRuntimeExecutable(exe) {
		host.json(w, 409, map[string]string{"error": "请在已安装的 ACU Runtime 中设置自启动"})
		return
	}
	if err = writeStartup(exe, *in.Enabled); err == nil {
		err = saveStartupPreference(*in.Enabled)
	}
	if err != nil {
		host.json(w, 500, map[string]string{"error": "无法保存开机自启动设置，请重试"})
		return
	}
	host.json(w, 200, startupPreference{Enabled: *in.Enabled, Supported: true})
}
