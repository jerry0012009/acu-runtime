package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
)

const videoReadingName = "acu-video-reader"
const videoMCPVersion = "0.2.3"

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
	if host.videoToolPath != nil {
		_, err := host.videoToolPath(ctx)
		return err
	}
	target, err := videoMCPPath()
	if err != nil {
		return err
	}
	p, err := videoMCPRelease(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if !verifyVideoMCP(target, p.SHA256) {
		if _, err := installVideoMCP(ctx, p, target); err != nil {
			return err
		}
	}
	for _, arg := range []string{"--prepare", "--register-native"} {
		c := exec.CommandContext(ctx, target, arg)
		hideProcess(c)
		if err := c.Run(); err != nil {
			return errors.New("视频工具准备失败，请重试")
		}
	}
	return nil
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
	config, err := host.tapnowCredential(ctx, provider, "gemini-3.5-flash-lite")
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
		stored["env"] = env
		return host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": tool.Name, "server": stored})
	}
	return nil
}
