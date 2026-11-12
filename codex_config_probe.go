package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// No threads, tools or paid requests: ask the installed Codex process to parse
// the complete configuration and list its actual models, then end only this probe.
func probeCodexConfiguration(ctx context.Context, executable, model string) error {
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
		return errors.New("无法启动 Codex 配置检查，未应用接入")
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
	var auth struct {
		Mode string `json:"auth_mode"`
	}
	if data, e := os.ReadFile(filepath.Join(codexConfigDirectory(), "auth.json")); e == nil {
		json.Unmarshal(data, &auth)
	}
	if auth.Mode == "chatgpt" {
		raw, e := call(4, "account/read", map[string]bool{"refreshToken": false})
		var account struct {
			Account json.RawMessage `json:"account"`
		}
		if e != nil || json.Unmarshal(raw, &account) != nil || len(account.Account) == 0 || string(account.Account) == "null" {
			return errors.New("当前配置无法保留 ChatGPT 登录身份，已取消本次接入")
		}
	}
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
	if model == "" {
		return nil
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
				return nil
			}
		}
		return errors.New("Codex 模型目录尚未包含所选 ACU 模型，已取消本次接入")
	}
	return nil
}
