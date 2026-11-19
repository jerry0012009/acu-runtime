package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var errCodexProbeStart = errors.New("无法启动 Codex 配置检查，未应用接入")
var errDesktopProbeUnavailable = errors.New("已安装的 ChatGPT 无法单独启动配置检查")

const codexHydrogenModel = "acu-runtime/gemini-4-hydrogen"

// The kernel catalog describes what it serves. If the desktop is configured
// to use a catalog file instead, confirm that file actually lists the model.
func verifyDesktopCodexFiles(model string) error {
	dir := codexConfigDirectory()
	var config struct {
		Model   string `toml:"model"`
		Catalog string `toml:"model_catalog_json"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil || toml.Unmarshal(raw, &config) != nil || config.Model != model {
		return errors.New("ChatGPT 磁盘配置与所选模型不一致")
	}
	if config.Catalog == "" {
		return nil // signed-in route uses the live kernel list
	}
	path, managed := managedCodexCatalog(dir, config.Catalog)
	if !managed {
		return errors.New("无法独立核验 ChatGPT 的自定义模型目录")
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode().IsRegular() && info.Size() <= 16<<20 {
		var catalog struct {
			Models []struct {
				Slug string `json:"slug"`
			} `json:"models"`
		}
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &catalog) == nil {
			for _, m := range catalog.Models {
				if m.Slug == model {
					return nil
				}
			}
		}
	}
	return errors.New("ChatGPT 的本地模型目录未能核实所选模型")
}

// No threads, tools or paid requests: ask the installed Codex process to parse
// the complete configuration and list its actual models, then end only this probe.
func probeCodexConfiguration(ctx context.Context, executable, model string) error {
	authPath := filepath.Join(codexConfigDirectory(), "auth.json")
	authBefore, authErr := os.ReadFile(authPath)
	if authErr != nil && !os.IsNotExist(authErr) {
		return errors.New("无法读取 Codex 登录配置，请重试")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, executable, "app-server")
	hideProcess(c)
	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	cleanup, err := startOwnedCommand(c)
	if err != nil {
		return errCodexProbeStart
	}
	defer cleanup()
	defer func() {
		stdin.Close()
		// app-server stays alive after stdin closes. All responses have already
		// been checked; end only our job instead of waiting three seconds.
		cleanup()
		_ = c.Wait()
	}()
	enc := json.NewEncoder(stdin)
	dec := json.NewDecoder(io.LimitReader(stdout, 16<<20))
	call := func(id int, method string, params any) (json.RawMessage, error) {
		if err := enc.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for {
			var r struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := dec.Decode(&r); err != nil {
				return nil, err
			}
			if r.ID != id {
				continue
			}
			if len(r.Error) > 0 && string(r.Error) != "null" {
				return nil, errors.New("Codex 拒绝当前配置")
			}
			return r.Result, nil
		}
	}
	if _, err = call(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "acu_config_validation", "version": appVersion}, "capabilities": map[string]bool{"experimentalApi": true}}); err != nil {
		return errors.New("Codex 配置初始化失败，已取消本次接入")
	}
	enc.Encode(map[string]any{"method": "initialized", "params": map[string]any{}})
	// account/read may legitimately return null in the kernel's API or
	// exhausted-account mode. The native kernel owns that policy, not the
	// historical auth_mode in auth.json. Check configuration and models here.
	raw, err := call(2, "config/read", map[string]bool{"includeLayers": false})
	if err != nil {
		return errors.New("Codex 无法读取配置，已取消本次接入")
	}
	var config struct {
		Config struct {
			Model string `json:"model"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return errors.New("无法核对 Codex 当前配置")
	}
	if model != "" && config.Config.Model != model {
		return errors.New("Codex 读取的模型与所选模型不一致")
	}
	if model == codexHydrogenModel {
		name := "codex-code-mode-host"
		if filepath.Ext(executable) == ".exe" {
			name += ".exe"
		}
		info, hostErr := os.Stat(filepath.Join(filepath.Dir(executable), name))
		if hostErr != nil || !info.Mode().IsRegular() {
			return errors.New("当前 Codex 缺少配套 Code Mode 组件，请更新 ChatGPT / Codex 后再选择 Gemini")
		}
	}
	if model == "" {
		return verifyCodexAuthUnchanged(authPath, authBefore, authErr)
	}
	raw, err = call(3, "model/list", map[string]bool{"includeHidden": true})
	if err != nil {
		return errors.New("Codex 无法读取模型目录，已取消本次接入")
	}
	var models struct {
		Data []struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &models) != nil || len(models.Data) == 0 {
		return errors.New("Codex 模型目录为空，未应用接入")
	}
	if model != "" {
		for _, m := range models.Data {
			if m.ID == model || m.Model == model {
				return verifyCodexAuthUnchanged(authPath, authBefore, authErr)
			}
		}
		return errors.New("Codex 模型目录尚未包含所选 ACU 模型，已取消本次接入")
	}
	return nil
}

func verifyCodexAuthUnchanged(path string, before []byte, beforeErr error) error {
	after, err := os.ReadFile(path)
	if os.IsNotExist(beforeErr) && os.IsNotExist(err) {
		return nil
	}
	if err != nil || beforeErr != nil || !bytes.Equal(before, after) {
		return errors.New("Codex 登录配置已变化，请重新尝试接入")
	}
	return nil
}
