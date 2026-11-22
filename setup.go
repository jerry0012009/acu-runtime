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
	Drift      *agentDrift  `json:"drift,omitempty"`
}

type agentDrift struct {
	Kind string `json:"kind"`
}

func (a setupAgent) needsRepair() bool {
	return a.Drift != nil && a.Drift.Kind != "" && a.Drift.Kind != "bypassed"
}

func (host *server) repairPreservedCodex(ctx context.Context, fallback ...codexFieldChange) (map[string]json.RawMessage, error) {
	before, err := host.kernel.readState(ctx)
	if err != nil {
		return nil, err
	}
	var agents []setupAgent
	if err := json.Unmarshal(before["agents"], &agents); err != nil {
		return nil, err
	}
	var changes []codexFieldChange
	found := false
	for _, agent := range agents {
		if agent.ID != "codex" {
			continue
		}
		found = true
		for _, field := range agent.Fields {
			changes = append(changes, codexFieldChange{field.Key, field.Value})
		}
	}
	if !found {
		return nil, errors.New("无法找到 Codex 配置，未更改接入")
	}
	if len(fallback) > 0 {
		changes = fallback
	}
	var result map[string]json.RawMessage
	repair := func(k *kernelState) error {
		current, err := k.readState(ctx)
		if err != nil {
			return err
		}
		var agents []setupAgent
		if json.Unmarshal(current["agents"], &agents) != nil {
			return errors.New("无法重新核对 Codex 配置")
		}
		if len(fallback) == 0 {
			if err := verifyCodexPreservedFields(agents, changes); err != nil {
				return err
			}
		}
		// Repair wiring first without restoring a historically replaced model.
		for _, agent := range agents {
			if agent.ID == "codex" && (agent.Drift == nil || agent.Drift.Kind == "unwired") {
				if err := k.exchange(ctx, "POST", "/api/agents/reapply/codex", map[string]any{}, &result); err != nil && len(fallback) == 0 {
					return err
				}
			}
		}
		if result != nil && verifyCodexRouteState(result, changes, false) == nil {
			return nil
		}
		if len(fallback) == 0 {
			return errors.New("Codex 自动修复未通过核验")
		}
		for _, change := range changes {
			if err := k.exchange(ctx, "POST", "/api/set", map[string]string{"agent": "codex", "field": change.Field, "value": change.Value}, &result); err != nil {
				return err
			}
		}
		return verifyCodexRouteState(result, changes, false)
	}
	if host.desktop && runtime.GOOS == "windows" {
		return host.kernel.changeCodexQuietlyWith(ctx, changes, false, func(executable string) error {
			return nativeCatalogManagement(ctx, executable, repair)
		})
	}
	err = repair(host.kernel)
	if err == nil {
		err = verifyCodexRouteState(result, changes, false)
	}
	return result, err
}

func verifyCodexPreservedFields(agents []setupAgent, changes []codexFieldChange) error {
	for _, agent := range agents {
		if agent.ID != "codex" {
			continue
		}
		if agent.needsRepair() && agent.Drift.Kind != "unwired" {
			return errors.New("Codex 配置在修复前已变化，请重新核对")
		}
		for _, change := range changes {
			found := false
			for _, field := range agent.Fields {
				if field.Key == change.Field && field.Value == change.Value {
					found = true
					break
				}
			}
			if !found {
				return errors.New("Codex 配置在修复前已变化，请重新核对")
			}
		}
		return nil
	}
	return errors.New("无法找到 Codex 配置")
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

func (host *server) prepareCodexSetupRoute(ctx context.Context, provider string, models []string) error {
	state, err := host.kernel.readState(ctx)
	if err != nil {
		return err
	}
	var agents []setupAgent
	if json.Unmarshal(state["agents"], &agents) != nil {
		return errors.New("无法读取 Codex 自动配置状态")
	}
	for _, agent := range agents {
		if agent.ID != "codex" || !agent.Wired || !agent.needsRepair() {
			continue
		}
		model := ""
		for _, candidate := range models {
			if model = pickACU(agent.Fields, "model", provider, candidate); model != "" {
				break
			}
		}
		for _, field := range agent.Fields {
			if field.Key != "model" {
				continue
			}
			for _, option := range field.Options {
				if option.Value == field.Value && strings.HasPrefix(option.Ref, provider+"/") {
					model = field.Value
				}
			}
		}
		if model == "" {
			return errors.New("当前 Key 未提供可用于 Codex 自动配置的模型")
		}
		_, err := host.repairPreservedCodex(ctx, codexFieldChange{"model", model})
		return err
	}
	return nil
}

func (host *server) configureAgent(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Agent      string            `json:"agent"`
		Provider   string            `json:"provider"`
		Model      string            `json:"model"`
		Auxiliary  bool              `json:"auxiliary"`
		Preserve   bool              `json:"preserve"`
		Migrations map[string]string `json:"migrations"`
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
	ctx, cancel := context.WithTimeout(request.Context(), 120*time.Second)
	defer cancel()
	if input.Agent == "codex" && !host.allowCodexDesktopChange(response) {
		return
	}
	available := false
	availableModels := map[string]bool{}
	for _, choice := range host.connection(ctx).Choices {
		if choice.ID != input.Provider {
			continue
		}
		for _, model := range choice.Models {
			availableModels[model] = true
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
	migrations, migrationErr := setupMigrations(target.Fields, input.Provider, input.Migrations, availableModels)
	if migrationErr != nil {
		host.json(response, 400, map[string]string{"error": migrationErr.Error()})
		return
	}
	if err := host.prepareAgentComponents([]string{input.Agent}); err != nil {
		host.json(response, 400, map[string]string{"error": "PDF 组件路径同步失败：" + err.Error()})
		return
	}
	if input.Agent == "codex" && input.Preserve && target.Wired && target.needsRepair() {
		model := pickACU(target.Fields, "model", input.Provider, input.Model)
		for _, field := range target.Fields {
			if field.Key != "model" {
				continue
			}
			for _, option := range field.Options {
				if option.Value == field.Value && strings.HasPrefix(option.Ref, input.Provider+"/") {
					model = field.Value
				}
			}
		}
		if model == "" {
			host.json(response, http.StatusConflict, map[string]string{"error": "当前 Key 无可用 Codex 模型，自动配置未写入"})
			return
		}
		result, repairErr := host.repairPreservedCodex(ctx, append([]codexFieldChange{{"model", model}}, migrations...)...)
		if repairErr != nil {
			host.json(response, http.StatusConflict, map[string]string{"error": "Codex 自动修复未完成：" + repairErr.Error()})
			return
		}
		result["runtimeSetup"], _ = json.Marshal(map[string]any{"reopenDesktop": host.desktop})
		host.json(response, http.StatusOK, result)
		return
	}
	if input.Preserve && target.Wired && target.Drift == nil && host.desktop && runtime.GOOS == "windows" && input.Agent == "codex" {
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
	if input.Preserve && target.Wired && len(migrations) == 0 {
		if err := host.ensureACUContext(ctx, input.Provider); err != nil {
			host.json(response, 400, map[string]string{"error": err.Error()})
			return
		}
		host.json(response, 200, state)
		return
	}
	main := pickACU(target.Fields, "model", input.Provider, input.Model)
	if input.Preserve && target.Wired {
		for _, field := range target.Fields {
			if field.Key == "model" {
				main = field.Value
			}
		}
	}
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
			if _, migrating := input.Migrations[field]; migrating {
				continue
			}
			if input.Preserve && target.Wired {
				if field == "model" {
					changes = append(changes, codexFieldChange{field, main})
				}
				continue
			}
			if value := pickACU(target.Fields, field, input.Provider, input.Model); value != "" {
				changes = append(changes, codexFieldChange{field, value})
			}
		}
		changes = append(changes, migrations...)
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
	changes := []codexFieldChange{}
	for _, field := range fields {
		if _, migrating := input.Migrations[field]; migrating {
			continue
		}
		if input.Preserve && target.Wired {
			continue
		}
		value := pickACU(target.Fields, field, input.Provider, input.Model)
		if value == "" {
			continue
		}
		changes = append(changes, codexFieldChange{field, value})
	}
	changes = append(changes, migrations...)
	for _, change := range changes {
		if err := host.kernel.exchange(ctx, "POST", "/api/set", map[string]string{"agent": input.Agent, "field": change.Field, "value": change.Value}, &result); err != nil {
			host.json(response, 400, map[string]string{"error": "部分配置已写入，请重试：" + err.Error()})
			return
		}
		if change.Field != "model" {
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
			if agent.ID != input.Agent || !agent.Wired || agent.needsRepair() {
				continue
			}
			confirmed = input.Agent == "workbuddy"
			for _, field := range agent.Fields {
				if field.Key == "model" && field.Value == main {
					confirmed = true
				}
			}
			for _, change := range changes {
				matched := false
				for _, field := range agent.Fields {
					if field.Key == change.Field && field.Value == change.Value {
						matched = true
					}
				}
				confirmed = confirmed && matched
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

func setupMigrations(fields []setupField, provider string, requested map[string]string, available map[string]bool) ([]codexFieldChange, error) {
	var changes []codexFieldChange
	for _, field := range fields {
		model, ok := requested[field.Key]
		if !ok {
			continue
		}
		switch field.Key {
		case "small", "opus", "sonnet", "haiku", "fable", "subagent", "subagent_model", "memories":
		default:
			return nil, errors.New("不支持迁移此模型角色")
		}
		owned := false
		for _, option := range field.Options {
			if option.Value == field.Value && strings.HasPrefix(option.Ref, provider+"/") {
				owned = true
			}
		}
		value := pickACU(fields, field.Key, provider, model)
		if !owned || !available[model] || value == "" {
			return nil, errors.New("辅助模型已变化或上位模型不可用，未写入迁移")
		}
		changes = append(changes, codexFieldChange{field.Key, value})
	}
	if len(changes) != len(requested) {
		return nil, errors.New("无法找到要迁移的模型角色")
	}
	return changes, nil
}
