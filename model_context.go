package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
)

// ACU's automatic route guarantees this window but its model list currently
// omits it. Set only that missing alias; keep each named model's catalog window
// and all explicit user overrides. Magpie owns persistence and client syncing.
func (host *server) ensureACUContext(ctx context.Context, provider string) error {
	var listed struct {
		Providers []struct {
			ID       string         `json:"id"`
			Contexts map[string]int `json:"contexts"`
			Models   []struct {
				ID      string `json:"id"`
				Context int    `json:"context"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := host.kernel.exchange(ctx, "GET", "/api/providers", nil, &listed); err != nil {
		return err
	}
	for _, p := range listed.Providers {
		if p.ID != provider {
			continue
		}
		if p.Contexts["*"] > 0 || p.Contexts["acu-auto"] > 0 {
			return nil
		}
		for _, m := range p.Models {
			if m.ID != "acu-auto" || m.Context > 0 {
				continue
			}
			host.kernel.mu.Lock()
			executable := host.kernel.exe
			host.kernel.mu.Unlock()
			if executable == "" {
				return nil
			} // External management-only development connection.
			command := exec.CommandContext(ctx, executable, "provider", "set", provider, "context.acu-auto=922000")
			hideProcess(command)
			if err := command.Run(); err != nil {
				return errors.New("无法补齐自动模型上下文，请重试")
			}
			return nil
		}
	}
	return nil
}

// Use the kernel's native, keyless search before preparing an extra package.
// An explicitly disabled/configured source is a user choice and stays intact.
func (host *server) ensureNativeSearch(ctx context.Context, prefs map[string]json.RawMessage) (bool, error) {
	var searcher string
	json.Unmarshal(prefs["searcher"], &searcher)
	if searcher != "" {
		return false, nil
	}
	var apis []json.RawMessage
	json.Unmarshal(prefs["searchAPIs"], &apis)
	if len(apis) != 0 {
		return false, nil
	}
	if err := host.kernel.exchange(ctx, "POST", "/api/settings/search-api", map[string]any{"vendor": "you", "key": ""}, nil); err != nil {
		return false, err
	}
	return true, nil
}
