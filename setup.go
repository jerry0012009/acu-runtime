package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"time"
)

func (host *server) checkProvider(ctx context.Context, provider string) error {
	view := host.connection(ctx)
	found := false
	for _, choice := range view.Choices {
		if choice.ID == provider {
			found = true
		}
	}
	if !found {
		return errors.New("所选 ACU Key 已删除或停用，请重新选择")
	}
	return host.kernel.exchange(ctx, "POST", "/api/provider/models", map[string]string{"id": provider}, nil)
}

type setupOption struct {
	Value string `json:"value"`
	Ref   string `json:"ref"`
}

type setupField struct {
	Key     string        `json:"key"`
	Value   string        `json:"value"`
	Options []setupOption `json:"options"`
}

type setupAgent struct {
	ID         string       `json:"id"`
	Wired      bool         `json:"wired"`
	CLIMissing bool         `json:"cliMissing"`
	Fields     []setupField `json:"fields"`
}

func pickACU(fields []setupField, field, provider, model string) string {
	for _, current := range fields {
		if current.Key != field {
			continue
		}
		for _, option := range current.Options {
			if option.Ref == provider+"/"+model && option.Value != "" {
				return option.Value
			}
		}
	}
	return ""
}

func (host *server) configureAgent(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Agent     string `json:"agent"`
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		Auxiliary bool   `json:"auxiliary"`
		Preserve  bool   `json:"preserve"`
	}
	if json.NewDecoder(http.MaxBytesReader(response, request.Body, 4096)).Decode(&input) != nil {
		host.json(response, 400, map[string]string{"error": "无法读取配置"})
		return
	}
	supported := false
	for _, agent := range supportedAgents {
		if agent.ID == input.Agent {
			supported = true
		}
	}
	if !supported || input.Provider == "" || input.Model == "" {
		host.json(response, 400, map[string]string{"error": "请选择 Agent、Key 和模型"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 45*time.Second)
	defer cancel()
	if input.Agent == "codex" && !host.allowCodexDesktopChange(response) {
		return
	}
	available := false
	for _, choice := range host.connection(ctx).Choices {
		if choice.ID != input.Provider {
			continue
		}
		for _, model := range choice.Models {
			if model == input.Model {
				available = true
			}
		}
	}
	if !available {
		host.json(response, 400, map[string]string{"error": "所选 Key 或模型已删除、停用，请重新选择"})
		return
	}
	if err := host.checkProvider(ctx, input.Provider); err != nil {
		host.json(response, 400, map[string]string{"error": err.Error()})
		return
	}
	state, err := host.kernel.readState(ctx)
	var agents []setupAgent
	if err != nil || json.Unmarshal(state["agents"], &agents) != nil {
		host.json(response, 400, map[string]string{"error": "无法读取 Agent 配置"})
		return
	}
	var target *setupAgent
	for index := range agents {
		if agents[index].ID == input.Agent {
			target = &agents[index]
		}
	}
	if target == nil || target.CLIMissing {
		host.json(response, 400, map[string]string{"error": "未检测到此 Agent"})
		return
	}
	if err := host.prepareAgentComponents([]string{input.Agent}); err != nil {
		host.json(response, 400, map[string]string{"error": "PDF 组件路径同步失败：" + err.Error()})
		return
	}
	if input.Preserve && target.Wired && host.desktop && runtime.GOOS == "windows" && input.Agent == "codex" {
		for _, field := range target.Fields {
			if field.Key != "model" {
				continue
			}
			for _, option := range field.Options {
				if option.Value == field.Value && strings.HasPrefix(option.Ref, input.Provider+"/") {
					input.Model = strings.TrimPrefix(option.Ref, input.Provider+"/")
					input.Preserve = false
					break
				}
			}
		}
	}
	if input.Preserve && target.Wired {
		if err := host.ensureACUContext(ctx, input.Provider); err != nil {
			host.json(response, 400, map[string]string{"error": err.Error()})
			return
		}
		host.json(response, 200, state)
		return
	}
	main := pickACU(target.Fields, "model", input.Provider, input.Model)
	if input.Agent != "workbuddy" && main == "" {
		host.json(response, 400, map[string]string{"error": "此 Agent 暂不支持所选 ACU 模型，请更换模型"})
		return
	}
	if err := host.ensureACUContext(ctx, input.Provider); err != nil {
		host.json(response, 400, map[string]string{"error": err.Error()})
		return
	}
	var result map[string]json.RawMessage
	if host.desktop && runtime.GOOS == "windows" && input.Agent == "codex" {
		fields := []string{"model"}
		if input.Auxiliary {
			fields = append(fields, "small", "opus", "sonnet", "haiku", "fable", "subagent", "subagent_model")
		}
		changes := []codexFieldChange{}
		for _, field := range fields {
			if value := pickACU(target.Fields, field, input.Provider, input.Model); value != "" {
				changes = append(changes, codexFieldChange{field, value})
			}
		}
		result, err = host.kernel.changeCodexQuietly(ctx, changes, false)
		if err != nil {
			host.json(response, 409, map[string]string{"error": err.Error()})
			return
		}
		result["runtimeSetup"], _ = json.Marshal(map[string]any{"reopenDesktop": true, "auxiliaryApplied": len(changes) - 1})
		host.json(response, 200, result)
		return
	}
	if !target.Wired {
		if err := host.kernel.exchange(ctx, "POST", "/api/agents/connect/"+input.Agent, map[string]any{}, &result); err != nil {
			host.json(response, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	fields := []string{"model"}
	if input.Auxiliary {
		fields = append(fields, "small", "opus", "sonnet", "haiku", "fable", "subagent", "subagent_model")
	}
	auxiliaryApplied := 0
	for _, field := range fields {
		value := pickACU(target.Fields, field, input.Provider, input.Model)
		if value == "" {
			continue
		}
		if err := host.kernel.exchange(ctx, "POST", "/api/set", map[string]string{"agent": input.Agent, "field": field, "value": value}, &result); err != nil {
			host.json(response, 400, map[string]string{"error": "部分配置已写入，请重试：" + err.Error()})
			return
		}
		if field != "model" {
			auxiliaryApplied++
		}
	}
	if result == nil {
		result, err = host.kernel.readState(ctx)
	}
	if err != nil {
		host.json(response, 400, map[string]string{"error": "配置后读取状态失败，请重新检测"})
		return
	}
	var verified []setupAgent
	confirmed := false
	if json.Unmarshal(result["agents"], &verified) == nil {
		for _, agent := range verified {
			if agent.ID != input.Agent || !agent.Wired {
				continue
			}
			confirmed = input.Agent == "workbuddy"
			for _, field := range agent.Fields {
				if field.Key == "model" && field.Value == main {
					confirmed = true
				}
			}
		}
	}
	if !confirmed {
		host.json(response, 400, map[string]string{"error": "未确认所选模型已生效，部分配置可能已写入，请重新检测"})
		return
	}
	result["runtimeSetup"], _ = json.Marshal(map[string]int{"auxiliaryApplied": auxiliaryApplied})
	host.json(response, 200, result)
}
