package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"
)

const videoReadingName = "acu-video-reader"
const videoMCPVersion = "0.2.3"

type videoPreparationState struct {
	sync.Mutex
	path     string
	size     int64
	mod      time.Time
	prepared time.Time
}

type videoProgressKey struct{}

func videoProgress(ctx context.Context, message string) {
	if report, ok := ctx.Value(videoProgressKey{}).(func(string)); ok {
		report(message)
	}
}

func (host *server) setVideoReadingEnabled(ctx context.Context, provider string, agents []string, enabled bool) error {
	if enabled {
		if len(agents) == 0 {
			return errors.New("请先接入一个应用，再配置视频读取")
		}
		var state map[string]json.RawMessage
		var listed []setupAgent
		var err error
		state, err = host.kernel.readState(ctx)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(state["agents"], &listed); err != nil {
			return err
		}
		for _, id := range agents {
			if !slices.ContainsFunc(listed, func(a setupAgent) bool { return a.ID == id && !a.CLIMissing }) || !slices.ContainsFunc(supportedAgents, func(a struct{ ID, Name, Config string }) bool { return a.ID == id }) {
				return errors.New("所选应用未检测到")
			}
		}
		if err := host.assignVideoReading(ctx, agents); err != nil {
			return err
		}
	}
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	for _, tool := range lib.Servers {
		if tool.Name != videoReadingName {
			continue
		}
		if tool.Note != capabilityOwner {
			return errors.New("已有同名视频工具，未覆盖")
		}
		stored := map[string]any{}
		for key, value := range tool.stored {
			stored[key] = value
		}
		for _, key := range []string{"problems", "signIn", "icon"} {
			delete(stored, key)
		}
		stored["paused"] = !enabled
		if enabled {
			stored["hidden"] = false
			var disabled []string
			body, _ := json.Marshal(stored["disabledTools"])
			if string(body) != "null" && json.Unmarshal(body, &disabled) != nil {
				return errors.New("视频工具设置无效")
			}
			stored["disabledTools"] = slices.DeleteFunc(disabled, func(name string) bool { return name == "video_watch" })
		}
		if err := host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": tool.Name, "server": stored}); err != nil {
			return err
		}
		if enabled {
			connection := host.connection(ctx)
			for _, choice := range connection.Choices {
				if choice.ID == provider && slices.Contains(choice.Models, "gemini-3.5-flash-lite") {
					return host.configureVideoAudio(ctx, provider)
				}
			}
		}
		return nil
	}
	if !enabled {
		target, err := videoMCPPath()
		if err != nil {
			return err
		}
		return host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": "", "server": map[string]any{
			"name": videoReadingName, "note": capabilityOwner, "transport": "stdio", "command": target, "args": []string{}, "agents": []string{}, "paused": true, "local": true}})
	}
	return errors.New("视频工具尚未准备")
}

func videoMCPPath() (string, error) {
	d, e := os.UserConfigDir()
	name := "acu-video-mcp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(d, "ACURuntime", "tools", "video-mcp", videoMCPVersion, name), e
}

// Only acquire an already-built, verified tool distribution. The tool owns its
// embedded dependencies, cache, MCP lifecycle and browser native registration.
func (host *server) prepareVideoTools(ctx context.Context) error {
	host.videoPreparation.Lock()
	defer host.videoPreparation.Unlock()
	if host.videoToolPath != nil {
		_, err := host.videoToolPath(ctx)
		return err
	}
	target, err := videoMCPPath()
	if err != nil {
		return err
	}
	if st, err := os.Stat(target); err == nil && host.videoPreparation.path == target && host.videoPreparation.size == st.Size() && host.videoPreparation.mod.Equal(st.ModTime()) && time.Since(host.videoPreparation.prepared) < 10*time.Minute {
		return nil
	}
	p, err := videoMCPRelease(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if !verifyVideoMCP(target, p.SHA256) {
		videoProgress(ctx, "正在下载视频组件")
		if _, err := installVideoMCP(ctx, p, target); err != nil {
			return err
		}
	}
	for _, arg := range []string{"--prepare"} {
		videoProgress(ctx, "正在准备视频组件")
		c := exec.CommandContext(ctx, target, arg)
		hideProcess(c)
		if err := c.Run(); err != nil {
			return errors.New("视频工具准备失败，请重试")
		}
	}
	if st, err := os.Stat(target); err == nil {
		host.videoPreparation.path, host.videoPreparation.size, host.videoPreparation.mod = target, st.Size(), st.ModTime()
		host.videoPreparation.prepared = time.Now()
	}
	return nil
}

func (host *server) registerVideoNative(ctx context.Context) error {
	if host.videoToolPath != nil {
		return nil
	}
	target, err := videoMCPPath()
	if err != nil {
		return err
	}
	c := exec.CommandContext(ctx, target, "--register-native")
	hideProcess(c)
	return c.Run()
}
func (host *server) assignVideoReading(ctx context.Context, agents []string) error {
	if len(agents) == 0 {
		return nil
	}
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	var old *capabilityTool
	for i := range lib.Servers {
		if lib.Servers[i].Name == videoReadingName {
			old = &lib.Servers[i]
		}
	}
	if old != nil && old.Note != capabilityOwner {
		return errors.New("已有同名视频工具，未覆盖")
	}
	if err := host.prepareVideoTools(ctx); err != nil {
		return err
	}
	target, err := videoMCPPath()
	if host.videoToolPath != nil {
		target, err = host.videoToolPath(ctx)
	}
	if err != nil {
		return err
	}
	ids := slices.Clone(agents)
	if old != nil {
		ids = slices.Clone(old.Agents)
		for _, id := range agents {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		cmd, _ := old.stored["command"].(string)
		args, _ := old.stored["args"].([]any)
		self, _ := os.Executable()
		legacy := cmd == self && len(args) == 1 && args[0] == "--video-reader"
		legacy = legacy || (filepath.Base(cmd) == filepath.Base(target) && filepath.Dir(filepath.Dir(cmd)) == filepath.Dir(filepath.Dir(target)) && cmd != target)
		if !legacy {
			if sameAssignments(ids, old.Agents) {
				return nil
			}
			return host.writeToolLibrary(ctx, "/api/library/servers/agents", map[string]any{"name": videoReadingName, "agents": ids})
		}
	}
	tool := map[string]any{"name": videoReadingName, "note": capabilityOwner, "transport": "stdio", "command": target, "args": []string{}, "agents": ids, "local": true, "startupTimeout": 60}
	replace := ""
	if old != nil {
		replace = old.Name
		for _, key := range []string{"disabledTools", "startupTimeout", "hub", "hidden", "paused"} {
			if v, ok := old.stored[key]; ok {
				tool[key] = v
			}
		}
	}
	if err := host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": replace, "server": tool}); err != nil {
		return err
	}
	check, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(check.Servers, func(t capabilityTool) bool {
		return t.Name == videoReadingName && t.stored["command"] == target && sameAssignments(t.Agents, ids)
	}) {
		return errors.New("视频工具分配回读失败")
	}
	return nil
}

// Only configure the external MCP through the official Library environment.
// Media processing and model requests remain entirely inside that tool.
func (host *server) configureVideoAudio(ctx context.Context, provider string) error {
	config, err := host.tapnowCredentialFromCatalog(ctx, provider, "gemini-3.5-flash-lite", false)
	if err != nil {
		return err
	}
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	target, err := videoMCPPath()
	if err != nil {
		return err
	}
	if host.videoToolPath != nil {
		target, err = host.videoToolPath(ctx)
	}
	if err != nil {
		return err
	}
	for _, tool := range lib.Servers {
		if tool.Name != videoReadingName || tool.Note != capabilityOwner || tool.stored["command"] != target {
			continue
		}
		stored := map[string]any{}
		for key, value := range tool.stored {
			stored[key] = value
		}
		for _, key := range []string{"problems", "signIn", "icon"} {
			delete(stored, key)
		}
		env := map[string]string{}
		if raw, ok := tool.stored["env"].(map[string]any); ok {
			for key, value := range raw {
				if text, ok := value.(string); ok {
					env[key] = text
				}
			}
		}
		if raw, ok := tool.stored["env"].(map[string]string); ok {
			for key, value := range raw {
				env[key] = value
			}
		}
		env["ACU_AUDIO_API_KEY"] = config.APIKey
		env["ACU_AUDIO_BASE_URL"] = config.Settings.Base
		env["ACU_AUDIO_MODEL"] = "gemini-3.5-flash-lite"
		previous, _ := json.Marshal(tool.stored["env"])
		next, _ := json.Marshal(env)
		if string(previous) == string(next) {
			return nil
		}
		stored["env"] = env
		return host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": tool.Name, "server": stored})
	}
	return nil
}
