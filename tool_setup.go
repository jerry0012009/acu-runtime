package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"
)

type toolSetupView struct {
	ID         uint64   `json:"id"`
	Provider   string   `json:"provider"`
	Agents     []string `json:"agents"`
	Phase      string   `json:"phase"`
	Message    string   `json:"message"`
	Error      string   `json:"failure,omitempty"`
	BasicReady bool     `json:"basicReady"`
}

type toolSetupState struct {
	sync.Mutex
	view      toolSetupView
	revision  uint64
	videoOnly bool
}

func (host *server) setupProgress(id uint64, phase, message string, basic bool, err error) {
	host.toolSetup.Lock()
	defer host.toolSetup.Unlock()
	if host.toolSetup.view.ID != id {
		return
	}
	host.toolSetup.view.Phase, host.toolSetup.view.Message, host.toolSetup.view.BasicReady = phase, message, basic
	if err != nil {
		host.toolSetup.view.Error = err.Error()
	}
}

func (host *server) prepareCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		host.toolSetup.Lock()
		view := host.toolSetup.view
		view.Agents = slices.Clone(view.Agents)
		host.toolSetup.Unlock()
		host.json(w, 200, view)
		return
	}
	var in struct {
		Provider  string   `json:"provider"`
		Agents    []string `json:"agents"`
		VideoOnly bool     `json:"videoOnly"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || in.Provider == "" || len(in.Agents) == 0 || len(in.Agents) > len(supportedAgents) {
		host.json(w, 400, map[string]string{"error": "请选择已接入的应用"})
		return
	}
	for _, id := range in.Agents {
		if !slices.ContainsFunc(supportedAgents, func(a struct{ ID, Name, Config string }) bool { return a.ID == id }) {
			host.json(w, 400, map[string]string{"error": "请选择支持的应用"})
			return
		}
	}
	host.toolSetup.Lock()
	current := host.toolSetup.view
	if current.Phase != "" && current.Phase != "complete" && current.Phase != "error" {
		if current.Provider != in.Provider {
			host.toolSetup.Unlock()
			host.json(w, 409, map[string]string{"error": "工具正在配置，请稍后重试"})
			return
		}
		changed := host.toolSetup.videoOnly && !in.VideoOnly
		for _, agent := range in.Agents {
			if !slices.Contains(host.toolSetup.view.Agents, agent) {
				host.toolSetup.view.Agents = append(host.toolSetup.view.Agents, agent)
				changed = true
			}
		}
		if changed {
			host.toolSetup.revision++
			host.toolSetup.videoOnly = host.toolSetup.videoOnly && in.VideoOnly
		}
		current = host.toolSetup.view
		current.Agents = slices.Clone(current.Agents)
		host.toolSetup.Unlock()
		host.json(w, 202, current)
		return
	}
	id := current.ID + 1
	view := toolSetupView{ID: id, Provider: in.Provider, Agents: slices.Clone(in.Agents), Phase: "tools", Message: "正在配置图片与搜索"}
	if in.VideoOnly {
		view.Message = "正在准备视频组件"
	}
	host.toolSetup.view = view
	host.toolSetup.revision++
	host.toolSetup.videoOnly = in.VideoOnly
	host.toolSetup.Unlock()
	go func() {
		parent := host.lifecycle
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 8*time.Minute)
		defer cancel()
		started := time.Now()
		defer func() { log.Printf("tool preparation task=%d elapsed_ms=%d", id, time.Since(started).Milliseconds()) }()
		for {
			host.toolSetup.Lock()
			revision := host.toolSetup.revision
			in.Agents = slices.Clone(host.toolSetup.view.Agents)
			in.VideoOnly = host.toolSetup.videoOnly
			host.toolSetup.Unlock()
			var result capabilityView
			var err error
			host.capabilityMu.Lock()
			if in.VideoOnly {
				result, _, _, err = host.capabilityRead(ctx, in.Provider)
				if err == nil {
					valid := false
					for _, choice := range host.connection(ctx).Choices {
						if choice.ID == in.Provider {
							valid = true
						}
					}
					if !valid {
						err = errors.New("请选择可用的 ACU Key")
					}
				}
				if err == nil {
					var state map[string]json.RawMessage
					state, err = host.kernel.readState(ctx)
					var agents []setupAgent
					if err == nil {
						err = json.Unmarshal(state["agents"], &agents)
					}
					for _, id := range in.Agents {
						if err == nil && !slices.ContainsFunc(agents, func(a setupAgent) bool { return a.ID == id && !a.CLIMissing }) {
							err = errors.New("所选应用未检测到")
						}
					}
				}
			} else {
				result, err = host.capabilityApply(ctx, capabilityInput{Provider: in.Provider, Agents: in.Agents, SkipVideo: true})
			}
			host.capabilityMu.Unlock()
			if err != nil {
				host.setupProgress(id, "error", "工具配置未完成", false, err)
				return
			}
			if result.VideoReadingEnabled {
				message := "图片与搜索已就绪 · 正在准备视频组件"
				if in.VideoOnly {
					message = "正在准备视频组件"
				}
				host.setupProgress(id, "video", message, !in.VideoOnly, nil)
				ctx = context.WithValue(ctx, videoProgressKey{}, func(message string) { host.setupProgress(id, "video", message, !in.VideoOnly, nil) })
				err = host.prepareVideoTools(ctx)
				if err == nil {
					host.capabilityMu.Lock()
					// Re-read explicit opt-out after a background download before assigning.
					var latest capabilityView
					latest, _, _, err = host.capabilityRead(ctx, in.Provider)
					if err == nil && latest.VideoReadingEnabled {
						err = host.assignVideoReading(ctx, in.Agents)
					}
					if err == nil && latest.VideoReadingEnabled {
						for _, choice := range host.connection(ctx).Choices {
							if choice.ID == in.Provider && slices.Contains(choice.Models, "gemini-3.5-flash-lite") {
								err = host.configureVideoAudio(ctx, in.Provider)
								break
							}
						}
					}
					host.capabilityMu.Unlock()
				}
				if err != nil {
					host.setupProgress(id, "error", "视频准备未完成", !in.VideoOnly, err)
					return
				}
			}
			host.toolSetup.Lock()
			if host.toolSetup.revision == revision {
				host.toolSetup.view.Phase = "complete"
				host.toolSetup.view.Message = "工具已就绪 · 重新打开应用后使用"
				host.toolSetup.view.BasicReady = !in.VideoOnly
				host.toolSetup.Unlock()
				return
			}
			host.toolSetup.Unlock()
		}
	}()
	host.json(w, 202, view)
}
