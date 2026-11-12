package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
