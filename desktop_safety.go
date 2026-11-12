package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// The pinned kernel can end Codex's app-server when its route changes,
// then relaunch the entire app after its hosted identity fails. Require
// the user to close the app before making those changes from Runtime.
func (host *server) allowCodexDesktopChange(w http.ResponseWriter) bool {
	if !host.desktop {
		return true
	}
	probe := host.desktopProbe
	if probe == nil {
		probe = codexDesktopOpen
	}
	open, err := probe()
	if err != nil || open {
		host.json(w, http.StatusConflict, map[string]string{"error": "请先保存工作并完全退出 ChatGPT / Codex 桌面端，再修改接入或模型配置；当前内核在应用运行时改配置可能导致反复重启。"})
		return false
	}
	return true
}

func (host *server) allowDesktopMutation(w http.ResponseWriter, r *http.Request, path string) bool {
	if !host.desktop || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	for _, prefix := range []string{"/api/agents/connect/", "/api/agents/disconnect/", "/api/agents/disconnect-offline/", "/api/agents/stage/", "/api/agents/restart/", "/api/agent-models/"} {
		if path == prefix+"codex" {
			return host.allowCodexDesktopChange(w)
		}
	}
	if path != "/api/set" {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	if err != nil || len(body) > 8<<20 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	var input struct {
		Agent string `json:"agent"`
	}
	if json.Unmarshal(body, &input) != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return !strings.EqualFold(input.Agent, "codex") || host.allowCodexDesktopChange(w)
}
