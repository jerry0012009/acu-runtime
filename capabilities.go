package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const capabilityOwner = "ACU Runtime recommended tools v1"

type capabilityModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type capabilityTool struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url"`
	Note      string   `json:"note"`
	Agents    []string `json:"agents"`
	Custom    bool     `json:"custom,omitempty"`
	stored    map[string]any
}

func (tool *capabilityTool) UnmarshalJSON(data []byte) error {
	type plain capabilityTool
	if err := json.Unmarshal(data, (*plain)(tool)); err != nil {
		return err
	}
	return json.Unmarshal(data, &tool.stored)
}

type capabilityLibrary struct {
	Servers  []capabilityTool  `json:"servers"`
	Problems []json.RawMessage `json:"problems"`
	Result   struct {
		Problems []json.RawMessage `json:"problems"`
	} `json:"result"`
}
type capabilityView struct {
	Provider             string            `json:"provider"`
	Vision               string            `json:"vision"`
	ImageGen             string            `json:"imageGen"`
	VisionModels         []capabilityModel `json:"visionModels"`
	ImageModels          []capabilityModel `json:"imageModels"`
	Tools                []capabilityTool  `json:"tools"`
	SearchReady          bool              `json:"searchReady"`
	SearchURL            string            `json:"searchURL"`
	FreeSearchAvailable  bool              `json:"freeSearchAvailable"`
	FreeSearchConfigured bool              `json:"freeSearchConfigured"`
	Messages             []string          `json:"messages"`
}
type capabilityInput struct {
	Provider   string   `json:"provider"`
	Agents     []string `json:"agents"`
	Vision     *string  `json:"vision"`
	ImageGen   *string  `json:"imageGen"`
	SearchURL  *string  `json:"searchURL"`
	FreeSearch *bool    `json:"freeSearch"`
	ModelsOnly bool     `json:"modelsOnly"`
}

// Reuse an existing uvx installation. Dependency installation belongs to the
// tool's package manager, not a second package manager embedded in Runtime.
func freeSearchCommand() string {
	dir := managedSearchDirectory()
	if release, err := searchRuntimeRelease(runtime.GOOS, runtime.GOARCH); err == nil && dir != "" {
		marker, _ := os.ReadFile(filepath.Join(dir, ".verified"))
		if string(marker) == release.sha256 {
			if info, err := os.Stat(filepath.Join(dir, searchRuntimeName())); err == nil && !info.IsDir() {
				return filepath.Join(dir, searchRuntimeName())
			}
		}
	}
	if path, err := exec.LookPath("uvx"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	paths := []string{filepath.Join(home, ".local", "bin", "uvx"), "/opt/homebrew/bin/uvx", "/usr/local/bin/uvx"}
	if runtime.GOOS == "windows" {
		paths = []string{filepath.Join(home, ".local", "bin", "uvx.exe"), filepath.Join(home, "AppData", "Local", "hermes", "bin", "uvx.exe")}
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func preferredCapability(models []capabilityModel, names ...string) string {
	for _, name := range names {
		for _, m := range models {
			if strings.HasSuffix(m.ID, "/"+name) {
				return m.ID
			}
		}
	}
	return ""
}

func (host *server) capabilityRead(ctx context.Context, provider string) (capabilityView, map[string]json.RawMessage, capabilityLibrary, error) {
	view := capabilityView{Provider: provider, VisionModels: []capabilityModel{}, ImageModels: []capabilityModel{}, Tools: []capabilityTool{}, Messages: []string{}}
	var prefs map[string]json.RawMessage
	var lib capabilityLibrary
	if err := host.kernel.exchange(ctx, "GET", "/api/settings", nil, &prefs); err != nil {
		return view, nil, lib, err
	}
	if err := host.kernel.exchange(ctx, "GET", "/api/library", nil, &lib); err != nil {
		return view, nil, lib, err
	}
	json.Unmarshal(prefs["vision"], &view.Vision)
	json.Unmarshal(prefs["imageGen"], &view.ImageGen)
	var images, vision []capabilityModel
	json.Unmarshal(prefs["visionModels"], &vision)
	json.Unmarshal(prefs["imageGenModels"], &images)
	for _, m := range vision {
		if strings.HasPrefix(m.ID, provider+"/") {
			view.VisionModels = append(view.VisionModels, m)
		}
	}
	for _, m := range images {
		if strings.HasPrefix(m.ID, provider+"/") {
			view.ImageModels = append(view.ImageModels, m)
		}
	}
	// The upstream catalog may still be warming up. Only add the audited vision
	// candidate when this key actually lists it; do not label all models visual.
	for _, choice := range host.connection(ctx).Choices {
		if choice.ID == provider && slices.Contains(choice.Models, "gpt-6.1-sol") && !slices.ContainsFunc(view.VisionModels, func(m capabilityModel) bool { return m.ID == provider+"/gpt-6.1-sol" }) {
			view.VisionModels = append(view.VisionModels, capabilityModel{provider + "/gpt-6.1-sol", "GPT-6.1 Sol"})
		}
		if choice.ID == provider {
			for _, id := range []string{"gpt-image-2.5-sunburst-2026-09-08", "gpt-image-2.5-sunburst"} {
				if slices.Contains(choice.Models, id) && !slices.ContainsFunc(view.ImageModels, func(m capabilityModel) bool { return m.ID == provider+"/"+id }) {
					view.ImageModels = append(view.ImageModels, capabilityModel{provider + "/" + id, "Sunburst"})
				}
			}
		}
	}
	for _, tool := range lib.Servers {
		if tool.Note == capabilityOwner {
			tool.Custom = tool.stored["hub"] != nil && tool.stored["hub"] != ""
			if disabled, ok := tool.stored["disabledTools"].([]any); ok {
				tool.Custom = tool.Custom || (tool.Name != "acu-image" && len(disabled) > 0) || (tool.Name == "acu-image" && (len(disabled) != 1 || disabled[0] != "generate_video"))
			}
			view.Tools = append(view.Tools, tool)
			if tool.Name == "acu-free-search" && len(tool.Agents) > 0 {
				view.FreeSearchConfigured = true
			}
		}
	}
	view.FreeSearchAvailable = freeSearchCommand() != ""
	var apis []struct {
		Vendor, URL string
		Ready, Off  bool
	}
	json.Unmarshal(prefs["searchAPIs"], &apis)
	for _, api := range apis {
		if api.Ready && !api.Off {
			view.SearchReady = true
		}
		if api.Vendor == "searxng" {
			view.SearchURL = api.URL
		}
	}
	if !view.SearchReady {
		view.Messages = append(view.Messages, "ACU 搜索待配置；可使用免费的 DuckDuckGo MCP，搜索内容发送至 DuckDuckGo，无需 ACU Key。")
		if !view.FreeSearchAvailable {
			view.Messages = append(view.Messages, "首次配置会自动准备免费搜索工具，可能需要几分钟。")
		}
	}
	if view.Vision == "" && preferredCapability(view.VisionModels, "gpt-6.1-sol") == "" {
		view.Messages = append(view.Messages, "当前 Key 未提供推荐识图模型，请手动选择可用模型。")
	}
	if view.ImageGen == "" && preferredCapability(view.ImageModels, "gpt-image-2.5-sunburst-2026-09-08", "gpt-image-2.5-sunburst") == "" {
		view.Messages = append(view.Messages, "当前 Key 未提供推荐生图模型，请手动选择可用模型。")
	}
	view.Messages = append(view.Messages, "配置完成后请重新打开 Agent；WorkBuddy 首次需在 MCP 设置中信任工具。", "PDF 继续使用应用文件工具；扫描件需要转换或 OCR。", "工具已配置不等于已验证可用；生图使用额度，HTTP 返回的图片需保存到项目。")
	return view, prefs, lib, nil
}

func validateSearchURL(value string) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" {
		return errors.New("请填写 ACU 搜索服务的 HTTPS 地址")
	}
	h := strings.ToLower(u.Hostname())
	if h != "acu.best" && !strings.HasSuffix(h, ".acu.best") && h != "acucompute.com" && !strings.HasSuffix(h, ".acucompute.com") {
		return errors.New("搜索服务须使用 ACU 官方域名，避免将 ACU Key 发送给其他服务")
	}
	return nil
}

func (host *server) capabilityApply(ctx context.Context, input capabilityInput) (capabilityView, error) {
	view, prefs, lib, err := host.capabilityRead(ctx, input.Provider)
	if err != nil {
		return view, err
	}
	var choice *connectionChoice
	connection := host.connection(ctx)
	for i := range connection.Choices {
		if connection.Choices[i].ID == input.Provider {
			choice = &connection.Choices[i]
		}
	}
	if choice == nil {
		return view, errors.New("请选择可用的 ACU Key")
	}
	if input.ModelsOnly && (len(input.Agents) > 0 || input.SearchURL != nil || input.FreeSearch != nil) {
		return view, errors.New("模型设置不能同时修改工具或搜索服务")
	}
	// Saving shared model picks must never prepare dependencies or rewrite MCP
	// assignments. Validate and change only the fields the user selected.
	if input.ModelsOnly {
		changed := false
		for _, entry := range []struct {
			pick   *string
			models []capabilityModel
			field  string
		}{{input.Vision, view.VisionModels, "vision"}, {input.ImageGen, view.ImageModels, "imageGen"}} {
			if entry.pick == nil {
				continue
			}
			if *entry.pick != "off" && !slices.ContainsFunc(entry.models, func(m capabilityModel) bool { return m.ID == *entry.pick }) {
				return view, errors.New("模型不可用，请刷新后重新选择")
			}
			prefs[entry.field], _ = json.Marshal(*entry.pick)
			changed = true
		}
		if changed {
			if err := host.kernel.exchange(ctx, "POST", "/api/settings", prefs, nil); err != nil {
				return view, err
			}
		}
		view, _, _, err = host.capabilityRead(ctx, input.Provider)
		return view, err
	}
	state, err := host.kernel.readState(ctx)
	if err != nil {
		return view, err
	}
	var agents []setupAgent
	json.Unmarshal(state["agents"], &agents)
	for _, id := range input.Agents {
		if !slices.ContainsFunc(supportedAgents, func(a struct{ ID, Name, Config string }) bool { return a.ID == id }) || !slices.ContainsFunc(agents, func(a setupAgent) bool { return a.ID == id && !a.CLIMissing }) {
			return view, errors.New("所选 Agent 未检测到")
		}
	}
	if input.SearchURL != nil && *input.SearchURL != "" {
		if err := validateSearchURL(*input.SearchURL); err != nil {
			return view, err
		}
	}
	for _, entry := range []struct {
		pick   *string
		models []capabilityModel
		field  string
	}{{input.Vision, view.VisionModels, "vision"}, {input.ImageGen, view.ImageModels, "imageGen"}} {
		if entry.pick != nil && *entry.pick != "off" && !slices.ContainsFunc(entry.models, func(m capabilityModel) bool { return m.ID == *entry.pick }) {
			return view, fmt.Errorf("%s 模型不可用，请刷新后重新选择", entry.field)
		}
	}
	var gateway string
	json.Unmarshal(prefs["gateway"], &gateway)
	u, err := url.Parse(gateway)
	if err != nil || u.Scheme != "http" || !loopback(u.Hostname()) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return view, errors.New("无法确认本机工具网关地址")
	}
	defs := []struct{ name, path string }{{"acu-vision", "vision"}, {"acu-image", "image"}}
	if view.SearchReady || (input.SearchURL != nil && *input.SearchURL != "") {
		defs = append(defs, struct{ name, path string }{"acu-search", "web-search"})
	}
	freeCommand := freeSearchCommand()
	wantFree := input.FreeSearch == nil || *input.FreeSearch
	needsFree := wantFree && !view.SearchReady && (input.SearchURL == nil || *input.SearchURL == "") && len(input.Agents) > 0
	if needsFree {
		defs = append(defs, struct{ name, path string }{"acu-free-search", "free-search"})
	}
	for _, def := range defs {
		for _, t := range lib.Servers {
			if t.Name == def.name && t.Note != capabilityOwner {
				return view, fmt.Errorf("已有同名工具 %s，请先在工具目录中重命名", def.name)
			}
		}
	}
	if needsFree {
		freeCommand, err = ensureSearchCommand(ctx)
		if err != nil {
			return view, err
		}
		prepare := host.prepareSearch
		if prepare == nil {
			prepare = prepareFreeSearch
		}
		if err = prepare(ctx, freeCommand); err != nil {
			return view, err
		}
	}
	// Keep all unrelated preferences and explicit existing picks. Defaults are
	// pinned to this ACU provider, rather than an implicit native-account fallback.
	changed := false
	for _, entry := range []struct {
		field, current, defaultValue string
		pick                         *string
	}{{"vision", view.Vision, preferredCapability(view.VisionModels, "gpt-6.1-sol"), input.Vision}, {"imageGen", view.ImageGen, preferredCapability(view.ImageModels, "gpt-image-2.5-sunburst-2026-09-08", "gpt-image-2.5-sunburst"), input.ImageGen}} {
		value := entry.current
		if entry.pick != nil {
			value = *entry.pick
		} else if value == "" {
			value = entry.defaultValue
		}
		if value != "" && value != entry.current {
			prefs[entry.field], _ = json.Marshal(value)
			changed = true
		}
	}
	if changed {
		if err := host.kernel.exchange(ctx, "POST", "/api/settings", prefs, nil); err != nil {
			return view, err
		}
	}
	if input.SearchURL != nil && *input.SearchURL != "" {
		var secret struct {
			Key string `json:"key"`
		}
		if err := host.kernel.exchange(ctx, "POST", "/api/provider/key", map[string]string{"id": input.Provider}, &secret); err != nil {
			return view, err
		}
		if secret.Key == "" {
			return view, errors.New("ACU Key 无法读取")
		}
		if err := host.kernel.exchange(ctx, "POST", "/api/settings/search-api", map[string]any{"vendor": "searxng", "url": *input.SearchURL, "key": secret.Key}, nil); err != nil {
			return view, err
		}
		if err := host.kernel.exchange(ctx, "POST", "/api/settings/search-api", map[string]any{"vendor": "searxng", "on": true}, nil); err != nil {
			return view, err
		}
	}
	for _, def := range defs {
		ids := []string{}
		old := ""
		var stored map[string]any
		for _, t := range lib.Servers {
			if t.Name == def.name {
				ids = append(ids, t.Agents...)
				old = t.Name
				stored = t.stored
			}
		}
		for _, id := range input.Agents {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			continue
		}
		tool := map[string]any{"name": def.name, "transport": "http", "url": strings.TrimRight(gateway, "/") + "/mcp/magpie/" + def.path, "agents": ids, "note": capabilityOwner, "local": true, "startupTimeout": 60}
		if def.path == "free-search" {
			tool = map[string]any{"name": def.name, "transport": "stdio", "command": freeCommand, "args": []string{"--native-tls", "--python", "3.12", "--from", "duckduckgo-mcp-server[browser]==0.7.0", "duckduckgo-mcp-server"}, "agents": ids, "note": capabilityOwner, "local": true, "startupTimeout": 120}
		}
		if stored != nil {
			// The kernel owns transport and tool options; preserve later edits made
			// in Library instead of resetting them on every bulk configuration.
			tool = stored
			tool["agents"] = ids
		} else if def.path == "image" {
			tool["disabledTools"] = []string{"generate_video"}
		}
		var result capabilityLibrary
		if err := host.kernel.exchange(ctx, "POST", "/api/library/servers/save", map[string]any{"old": old, "server": tool}, &result); err != nil {
			return view, fmt.Errorf("部分配置可能已保存：%w", err)
		}
		if len(result.Problems) > 0 || len(result.Result.Problems) > 0 {
			return view, errors.New("工具已保存，但部分 Agent 写入失败，请在工具目录检查并重试")
		}
	}
	view, _, _, err = host.capabilityRead(ctx, input.Provider)
	return view, err
}

func (host *server) capabilities(w http.ResponseWriter, r *http.Request) {
	host.capabilityMu.Lock()
	defer host.capabilityMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Minute)
	defer cancel()
	var view capabilityView
	var err error
	if r.Method == "GET" {
		view, _, _, err = host.capabilityRead(ctx, r.URL.Query().Get("provider"))
	} else {
		var input capabilityInput
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&input) != nil {
			host.json(w, 400, map[string]string{"error": "无法读取能力配置"})
			return
		}
		view, err = host.capabilityApply(ctx, input)
	}
	if err != nil {
		host.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	host.json(w, 200, view)
}
