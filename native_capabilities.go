package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"slices"
)

func (host *server) saveKernelSetting(ctx context.Context, key, value string) error {
	if host.setKernelSetting != nil {
		return host.setKernelSetting(ctx, key, value)
	}
	host.kernel.mu.Lock()
	executable := host.kernel.exe
	host.kernel.mu.Unlock()
	if executable == "" {
		executable = os.Getenv("ACU_KERNEL_PATH")
	}
	if executable == "" {
		return errors.New("当前连接不支持保存模型设置，请在高级配置中操作")
	}
	command := exec.CommandContext(ctx, executable, "settings", "set", key, value)
	hideProcess(command)
	if err := command.Run(); err != nil {
		return errors.New("模型设置未保存，请重试")
	}
	return nil
}

func (host *server) readToolLibrary(ctx context.Context) (capabilityLibrary, error) {
	var lib capabilityLibrary
	err := host.kernel.exchange(ctx, "GET", "/api/library", nil, &lib)
	return lib, err
}

func (host *server) writeToolLibrary(ctx context.Context, path string, input any) error {
	var result capabilityLibrary
	if err := host.kernel.exchange(ctx, "POST", path, input, &result); err != nil {
		return err
	}
	if len(result.Problems) > 0 || len(result.Result.Problems) > 0 {
		return errors.New("工具已保存，但部分应用写入失败，请重试")
	}
	return nil
}

// Definition and command come from the native market. Only branding and user
// assignments are applied here; native options are not reconstructed.
func (host *server) assignNativeTool(ctx context.Context, name, kind string, agents []string) error {
	if len(agents) == 0 {
		return nil
	}
	marketID := map[string]string{"vision": "magpie-vision", "image": "magpie-image", "web-search": "magpie-web-search"}[kind]
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	var old *capabilityTool
	for i := range lib.Servers {
		if lib.Servers[i].Name == name {
			old = &lib.Servers[i]
			break
		}
	}
	ids := slices.Clone(agents)
	if kind == "web-search" {
		for _, t := range lib.Servers {
			if migratableFreeSearch(t) {
				for _, id := range t.Agents {
					if !slices.Contains(ids, id) {
						ids = append(ids, id)
					}
				}
			}
		}
	}
	if old != nil {
		if old.Note != capabilityOwner {
			return errors.New("已有同名工具，请在工具目录检查 " + name)
		}
		requested := ids
		ids = slices.Clone(old.Agents)
		for _, id := range requested {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		_, hasCommand := old.stored["command"]
		// User-customized endpoints, launchers and advanced choices remain intact.
		u, parseErr := url.Parse(old.URL)
		legacy := parseErr == nil && old.Transport == "http" && u.Scheme == "http" && loopback(u.Hostname()) && u.Path == "/mcp/magpie/"+kind && u.User == nil && u.RawQuery == "" && u.Fragment == "" && !hasCommand
		if !legacy {
			if !sameAssignments(ids, old.Agents) {
				if err = host.writeToolLibrary(ctx, "/api/library/servers/agents", map[string]any{"name": name, "agents": ids}); err != nil {
					return err
				}
			}
			return host.finishNativeSearchMigration(ctx, kind, name)
		}
	}
	var market struct {
		Items []struct{ ID, Name, Have string } `json:"items"`
	}
	if err = host.kernel.exchange(ctx, "GET", "/api/library/market/servers", nil, &market); err != nil {
		return err
	}
	nativeName := ""
	for _, item := range market.Items {
		if item.ID == marketID {
			nativeName = item.Have
			if nativeName == "" {
				nativeName = item.Name
			}
			break
		}
	}
	if nativeName == "" {
		return errors.New("当前内核缺少原生工具，请更新 Runtime")
	}
	var native *capabilityTool
	for i := range lib.Servers {
		if lib.Servers[i].Name == nativeName {
			native = &lib.Servers[i]
			break
		}
	}
	if native == nil {
		if err = host.writeToolLibrary(ctx, "/api/library/market/server", map[string]any{"id": marketID, "agents": []string{}}); err != nil {
			return err
		}
		lib, err = host.readToolLibrary(ctx)
		if err != nil {
			return err
		}
		for i := range lib.Servers {
			if lib.Servers[i].Name == nativeName {
				native = &lib.Servers[i]
				break
			}
		}
	}
	if native == nil {
		return errors.New("未读到原生工具安装结果")
	}
	for _, id := range native.Agents {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	tool := map[string]any{}
	for key, value := range native.stored {
		tool[key] = value
	}
	for _, key := range []string{"problems", "signIn", "icon"} {
		delete(tool, key)
	}
	tool["name"], tool["note"], tool["agents"] = name, capabilityOwner, ids
	if old != nil {
		for _, key := range []string{"disabledTools", "startupTimeout", "hub", "hidden", "paused", "local"} {
			if v, ok := old.stored[key]; ok {
				tool[key] = v
			}
		}
	}
	replace := nativeName
	if old != nil {
		replace = name
	}
	if err = host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"old": replace, "server": tool}); err != nil {
		return err
	}
	checked, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(checked.Servers, func(t capabilityTool) bool {
		return t.Name == name && t.Note == capabilityOwner && sameAssignments(t.Agents, ids)
	}) {
		return errors.New("工具分配未通过回读检查")
	}
	if old != nil && nativeName != name {
		if err = host.writeToolLibrary(ctx, "/api/library/servers/remove", map[string]string{"name": nativeName}); err != nil {
			return err
		}
	}
	return host.finishNativeSearchMigration(ctx, kind, name)
}

func sameAssignments(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, id := range left {
		if !slices.Contains(right, id) {
			return false
		}
	}
	return true
}

func migratableFreeSearch(tool capabilityTool) bool {
	if tool.Name != "acu-free-search" || tool.Note != capabilityOwner || tool.stored["hidden"] == true || tool.stored["paused"] == true {
		return false
	}
	if tool.stored["hub"] != nil && tool.stored["hub"] != "" {
		return false
	}
	if disabled, ok := tool.stored["disabledTools"].([]any); ok && len(disabled) > 0 {
		return false
	}
	args, _ := tool.stored["args"].([]any)
	return slices.Contains(args, any("duckduckgo-mcp-server[browser]==0.7.0")) && slices.Contains(args, any("duckduckgo-mcp-server"))
}

func (host *server) finishNativeSearchMigration(ctx context.Context, kind, name string) error {
	if kind != "web-search" {
		return nil
	}
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	var replacement *capabilityTool
	for i := range lib.Servers {
		if lib.Servers[i].Name == name {
			replacement = &lib.Servers[i]
		}
	}
	if replacement == nil || replacement.Transport != "stdio" || replacement.stored["hidden"] == true {
		return nil
	}
	for _, legacy := range lib.Servers {
		if !migratableFreeSearch(legacy) || len(legacy.Agents) == 0 {
			continue
		}
		covered := true
		for _, id := range legacy.Agents {
			covered = covered && slices.Contains(replacement.Agents, id)
		}
		if covered {
			if err = host.writeToolLibrary(ctx, "/api/library/servers/agents", map[string]any{"name": legacy.Name, "agents": []string{}}); err != nil {
				return err
			}
		}
	}
	return nil
}
