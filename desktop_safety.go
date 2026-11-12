package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// A GUI kernel left running on an old version can restart Codex repeatedly
// as its catalog changes. Verify the serving engine before changing routes.
func (host *server) allowCodexDesktopChange(w http.ResponseWriter) bool {
	if !host.desktop {
		return true
	}
	probe := host.kernelProbe
	if probe == nil {
		probe = func() (string, error) { info, err := readServingKernel(context.Background()); return info.Version, err }
	}
	version, err := probe()
	if err != nil || version == "" || desktopVersionNewer(kernelVersion, version) {
		host.json(w, http.StatusConflict, map[string]string{"error": "正在准备兼容内核，完成后即可开启 ChatGPT 接入，请稍后重试。"})
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

// All desktop entry points that change Codex's route use the same quiet
// transaction; the advanced embedded UI cannot bypass the workbench fix.
func (host *server) handleQuietCodexMutation(w http.ResponseWriter, r *http.Request, path string) bool {
	if !host.desktop || runtime.GOOS != "windows" || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}
	var changes []codexFieldChange
	disconnect := false
	switch path {
	case "/api/agents/restart/codex":
		host.json(w, 409, map[string]string{"error": "请保存工作后手动重新打开 ChatGPT，使新配置生效。"})
		return true
	case "/api/agent-models/codex":
		// This native endpoint rewrites the live catalog and can end the
		// hosted app-server just like Connect. Never forward it unguarded.
		host.json(w, 409, map[string]string{"error": "请在 Codex 卡片选择模型完成安全配置。"})
		return true
	case "/api/set":
		body, err := io.ReadAll(io.LimitReader(r.Body, 8193))
		if err != nil || len(body) > 8192 {
			host.json(w, 400, map[string]string{"error": "无法读取 Agent 配置"})
			return true
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var input struct{ Agent, Field, Value string }
		if json.Unmarshal(body, &input) != nil {
			host.json(w, 400, map[string]string{"error": "无法读取 Agent 配置"})
			return true
		}
		if !strings.EqualFold(input.Agent, "codex") {
			return false
		}
		if input.Field == "" {
			host.json(w, 400, map[string]string{"error": "未指定 Codex 设置"})
			return true
		}
		changes = []codexFieldChange{{input.Field, input.Value}}
	case "/api/agents/disconnect/codex", "/api/agents/disconnect-offline/codex":
		disconnect = true
	case "/api/agents/connect/codex":
		state, err := host.kernel.readState(r.Context())
		if err != nil {
			host.json(w, 503, map[string]string{"error": "无法读取 Codex 配置"})
			return true
		}
		var agents []setupAgent
		json.Unmarshal(state["agents"], &agents)
		for _, a := range agents {
			if a.ID == "codex" {
				value := pickACU(a.Fields, "model", "acu-runtime", "acu-auto")
				if value != "" {
					changes = []codexFieldChange{{"model", value}}
				}
			}
		}
		if len(changes) == 0 {
			host.json(w, 409, map[string]string{"error": "请在 Codex 卡片选择 ACU 模型完成接入"})
			return true
		}
	case "/api/agents/stage/codex":
		host.json(w, 400, map[string]string{"error": "Codex 请使用模型选择完成配置"})
		return true
	default:
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	state, err := host.kernel.changeCodexQuietly(ctx, changes, disconnect)
	if err != nil {
		host.json(w, 409, map[string]string{"error": err.Error()})
		return true
	}
	state["runtimeSetup"], _ = json.Marshal(map[string]any{"reopenDesktop": true})
	host.json(w, 200, state)
	return true
}
