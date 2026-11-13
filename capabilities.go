package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	VisionAuto           string            `json:"visionAuto"`
	ImageGenAuto         string            `json:"imageGenAuto"`
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
	json.Unmarshal(prefs["visionAuto"], &view.VisionAuto)
	json.Unmarshal(prefs["imageGenAuto"], &view.ImageGenAuto)
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
	view.FreeSearchAvailable = true
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
	var searcher string
	json.Unmarshal(prefs["searcher"], &searcher)
	view.SearchReady = view.SearchReady || (searcher != "" && searcher != "off")
	if !view.SearchReady {
		view.Messages = append(view.Messages, "搜索待配置")
	}
	if view.Vision == "" && view.VisionAuto == "" && len(view.VisionModels) == 0 {
		view.Messages = append(view.Messages, "暂无识图模型")
	}
	if view.ImageGen == "" && view.ImageGenAuto == "" && len(view.ImageModels) == 0 {
		view.Messages = append(view.Messages, "暂无生图模型")
	}
	return view, prefs, lib, nil
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
	if input.SearchURL != nil && *input.SearchURL != "" {
		return view, errors.New("请在高级搜索设置中配置搜索服务")
	}
	if input.ModelsOnly && (len(input.Agents) > 0 || input.SearchURL != nil || input.FreeSearch != nil) {
		return view, errors.New("模型设置不能同时修改工具或搜索服务")
	}
	// Saving shared model picks must never prepare dependencies or rewrite MCP
	// assignments. Validate and change only the fields the user selected.
	if input.ModelsOnly {
		entries := []struct {
			pick   *string
			models []capabilityModel
			field  string
		}{{input.Vision, view.VisionModels, "vision"}, {input.ImageGen, view.ImageModels, "imageGen"}}
		for _, entry := range entries {
			if entry.pick == nil {
				continue
			}
			if *entry.pick != "" && *entry.pick != "off" && !slices.ContainsFunc(entry.models, func(m capabilityModel) bool { return m.ID == *entry.pick }) {
				return view, errors.New("模型不可用，请刷新后重新选择")
			}
		}
		for _, entry := range entries {
			if entry.pick == nil {
				continue
			}
			if err := host.saveKernelSetting(ctx, entry.field, *entry.pick); err != nil {
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
	for _, entry := range []struct {
		pick   *string
		models []capabilityModel
		field  string
	}{{input.Vision, view.VisionModels, "vision"}, {input.ImageGen, view.ImageModels, "imageGen"}} {
		if entry.pick != nil && *entry.pick != "" && *entry.pick != "off" && !slices.ContainsFunc(entry.models, func(m capabilityModel) bool { return m.ID == *entry.pick }) {
			return view, fmt.Errorf("%s 模型不可用，请刷新后重新选择", entry.field)
		}
	}
	defs := []struct{ name, path string }{{"acu-vision", "vision"}, {"acu-image", "image"}}
	if view.SearchReady || (input.SearchURL != nil && *input.SearchURL != "") {
		defs = append(defs, struct{ name, path string }{"acu-search", "web-search"})
	}
	wantFree := input.FreeSearch == nil || *input.FreeSearch
	needsFree := wantFree && !view.SearchReady && (input.SearchURL == nil || *input.SearchURL == "") && len(input.Agents) > 0
	if needsFree {
		defs = append(defs, struct{ name, path string }{"acu-search", "web-search"})
	}
	// Validate the native search name too before making any configuration write.
	for _, t := range lib.Servers {
		if needsFree && t.Name == "acu-search" && t.Note != capabilityOwner {
			return view, errors.New("已有同名工具 acu-search，请先在工具目录中重命名")
		}
	}
	for _, def := range defs {
		for _, t := range lib.Servers {
			if t.Name == def.name && t.Note != capabilityOwner {
				return view, fmt.Errorf("已有同名工具 %s，请先在工具目录中重命名", def.name)
			}
		}
	}
	if len(input.Agents) > 0 {
		// Some kernels expose supported candidates but no automatic pick.
		// Use their first candidate, never a model-name capability table.
		for _, entry := range []struct {
			field, current, auto string
			models               []capabilityModel
			pick                 *string
		}{
			{"vision", view.Vision, view.VisionAuto, view.VisionModels, input.Vision},
			{"imageGen", view.ImageGen, view.ImageGenAuto, view.ImageModels, input.ImageGen},
		} {
			if entry.current == "" && entry.auto == "" && entry.pick == nil && len(entry.models) > 0 {
				if err = host.saveKernelSetting(ctx, entry.field, entry.models[0].ID); err != nil {
					return view, err
				}
			}
		}
	}
	if len(input.Agents) > 0 {
		if err := host.ensureACUContext(ctx, input.Provider); err != nil {
			return view, err
		}
		if needsFree {
			added, err := host.ensureNativeSearch(ctx, prefs)
			if err != nil {
				return view, err
			}
			if added {
				view, prefs, lib, err = host.capabilityRead(ctx, input.Provider)
				if err != nil {
					return view, err
				}
				needsFree = !view.SearchReady
			}
		}
	}
	if needsFree {
		defs = slices.DeleteFunc(defs, func(d struct{ name, path string }) bool { return d.path == "web-search" })
	}
	// Explicit picks alone are written; the kernel chooses automatic fallbacks.
	for _, entry := range []struct {
		field, current string
		pick           *string
	}{{"vision", view.Vision, input.Vision}, {"imageGen", view.ImageGen, input.ImageGen}} {
		value := entry.current
		if entry.pick != nil {
			value = *entry.pick
		}
		if entry.pick != nil && value != entry.current {
			if err := host.saveKernelSetting(ctx, entry.field, value); err != nil {
				return view, err
			}
		}
	}
	for _, def := range defs {
		if err := host.assignNativeTool(ctx, def.name, def.path, input.Agents); err != nil {
			return view, err
		}
	}
	if err := host.assignImageGuidance(ctx, input.Agents); err != nil {
		return view, err
	}
	if err := host.prepareAgentComponents(input.Agents); err != nil {
		return view, fmt.Errorf("工具已配置，PDF 组件路径同步失败：%w", err)
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
