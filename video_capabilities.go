package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Ask the installed native MCP for its own model schema, without generating media.
func (host *server) nativeVideoModels(ctx context.Context, provider string) ([]capabilityModel, string, string) {
	models := []capabilityModel{}
	host.kernel.mu.Lock()
	executable := host.kernel.exe
	host.kernel.mu.Unlock()
	if executable == "" {
		executable = os.Getenv("ACU_KERNEL_PATH")
	}
	if executable == "" {
		return models, "", "请连接配套内核后刷新视频模型"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "mcp", "image")
	hideProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return models, "", "视频模型暂未读取到，请刷新重试"
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return models, "", "视频模型暂未读取到，请刷新重试"
	}
	if err := cmd.Start(); err != nil {
		return models, "", "视频模型暂未读取到，请刷新重试"
	}
	defer func() { stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if _, err := stdin.Write([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n")); err != nil {
		return models, "", "视频模型暂未读取到，请刷新重试"
	}
	var reply struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties struct {
						Model struct {
							Enum []string `json:"enum"`
						} `json:"model"`
					} `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	decoder := json.NewDecoder(io.LimitReader(stdout, 4<<20))
	for decoder.Decode(&reply) == nil {
		for _, tool := range reply.Result.Tools {
			if tool.Name != "generate_video" {
				continue
			}
			ids := tool.InputSchema.Properties.Model.Enum
			auto := ""
			if len(ids) > 0 {
				auto = ids[0]
			}
			for _, id := range ids {
				if strings.HasPrefix(id, provider+"/") {
					models = append(models, capabilityModel{ID: id, Name: strings.TrimPrefix(id, provider+"/")})
				}
			}
			return models, auto, ""
		}
	}
	return models, "", "当前内核未返回视频工具，请更新 Runtime"
}

func (host *server) setVideoToolEnabled(ctx context.Context, lib capabilityLibrary, enabled bool) error {
	for _, tool := range lib.Servers {
		if tool.Name != "acu-image" || tool.Note != capabilityOwner {
			continue
		}
		if enabled && (tool.stored["paused"] == true || len(tool.Agents) == 0) {
			return errors.New("请先在工具与应用分配中启用媒体工具并选择应用")
		}
		var disabled []string
		if raw, ok := tool.stored["disabledTools"]; ok {
			data, err := json.Marshal(raw)
			if err != nil || json.Unmarshal(data, &disabled) != nil {
				return errors.New("请在工具目录检查视频工具设置")
			}
		}
		before := slices.Clone(disabled)
		disabled = slices.DeleteFunc(disabled, func(name string) bool { return name == "generate_video" })
		if !enabled {
			disabled = append(disabled, "generate_video")
		}
		if slices.Equal(before, disabled) {
			return nil
		}
		return host.writeToolLibrary(ctx, "/api/library/servers/tools", map[string]any{"name": tool.Name, "names": disabled})
	}
	return errors.New("请先点击一键检查并补齐，接入媒体工具")
}
