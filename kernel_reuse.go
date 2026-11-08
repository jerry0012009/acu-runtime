package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Use Magpie's own commands for settings writes: these validate, lock and
// preserve the rest of the user's configuration, including newer settings.
func pauseKernelUpdates(ctx context.Context, executable string) error {
	return pauseKernelUpdatesWith(func(args []string) error {
		command := exec.CommandContext(ctx, executable, args...)
		hideProcess(command)
		return command.Run()
	})
}

func pauseKernelUpdatesWith(run func([]string) error) error {
	for _, args := range [][]string{{"update", "auto", "off", "notify"}, {"settings", "set", "autoRestart", "off"}} {
		if err := run(args); err != nil {
			return errors.New("无法关闭 Magpie 自动更新；请检查内核设置后重试，已有安装未被替换")
		}
	}
	return nil
}

// This is Magpie's documented local browser-access record. Never enable
// network access, generate a new key, or overwrite the user's record here.
func kernelConfigDirs(executable string) []string {
	if real, err := filepath.EvalSymlinks(executable); err == nil {
		executable = real
	}
	base := filepath.Dir(executable)
	if runtime.GOOS == "darwin" && filepath.Base(base) == "MacOS" && filepath.Base(filepath.Dir(base)) == "Contents" {
		base = filepath.Dir(filepath.Dir(filepath.Dir(base)))
	}
	dirs := []string{filepath.Join(base, "data")}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		dirs = append(dirs, filepath.Join(xdg, "magpie"))
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "magpie"))
	}
	return dirs
}

func browserKernelLink(directory string) string {
	data, err := os.ReadFile(filepath.Join(directory, "web-page.json"))
	if err != nil || len(data) > 4096 {
		return ""
	}
	var config struct {
		Mode string `json:"mode"`
		Port int    `json:"port"`
		Key  string `json:"key"`
	}
	if json.Unmarshal(data, &config) != nil || (config.Mode != "local" && config.Mode != "lan") || config.Key == "" {
		return ""
	}
	if config.Port == 0 {
		config.Port = 3430
	}
	if config.Port < 1024 || config.Port > 65535 {
		return ""
	}
	return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(config.Port)) + "/?k=" + url.QueryEscape(config.Key)
}

func kernelLink(link string) (*url.URL, string, bool) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "http" || !loopback(parsed.Hostname()) || parsed.User != nil || parsed.Path != "/" || parsed.Fragment != "" || parsed.Port() == "" {
		return nil, "", false
	}
	query := parsed.Query()
	key := query.Get("k")
	if key == "" || len(query) != 1 {
		return nil, "", false
	}
	parsed.RawQuery = ""
	return parsed, key, true
}

func (kernel *kernelState) attachExisting(ctx context.Context, link string) bool {
	endpoint, key, ok := kernelLink(link)
	if !ok {
		return false
	}
	kernel.mu.Lock()
	previous, previousKey := kernel.endpoint, kernel.key
	kernel.endpoint, kernel.key = endpoint, key
	kernel.mu.Unlock()
	err := kernel.authenticate(ctx)
	if err == nil {
		_, err = kernel.readState(ctx)
	}
	if err == nil {
		return true
	}
	kernel.mu.Lock()
	kernel.endpoint, kernel.key = previous, previousKey
	kernel.mu.Unlock()
	return false
}

// Magpie's web command can hand back a service's existing management link
// and exit successfully. Keep its output bounded and private; never log keys.
type kernelOutput struct {
	mu   sync.Mutex
	text string
}

func (output *kernelOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	output.text += string(data)
	if len(output.text) > 16384 {
		output.text = output.text[len(output.text)-16384:]
	}
	return len(data), nil
}

func (output *kernelOutput) links() []string {
	output.mu.Lock()
	defer output.mu.Unlock()
	var links []string
	for _, field := range strings.Fields(output.text) {
		// Terminal colour escapes may surround the URL.
		if start := strings.Index(field, "http://"); start >= 0 {
			link := strings.Split(field[start:], "\x1b")[0]
			if _, _, ok := kernelLink(link); ok {
				links = append(links, link)
			}
		}
	}
	return links
}
