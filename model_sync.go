package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Native management runs without a gateway while the owned gateway is paused.
// Reserving its port prevents a second watcher from changing the desktop route.
func nativeCatalogManagement(ctx context.Context, executable string, action func(*kernelState) error) error {
	gateway, _ := url.Parse(gatewayBase())
	reserved, err := net.Listen("tcp", gateway.Host)
	if err != nil {
		return errors.New("内核端口仍在使用，模型列表未修改")
	}
	defer reserved.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address := listener.Addr().String()
	listener.Close()
	control, err := newKernel("http://" + address)
	if err != nil {
		return err
	}
	control.key = secret()
	control.exe = executable
	defer control.client.CloseIdleConnections()
	command := exec.CommandContext(ctx, executable, "web", "--addr", address, "--no-open")
	command.Env = append(os.Environ(), "MAGPIE_WEB_KEY="+control.key)
	hideProcess(command)
	if err = command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = control.exchange(cleanup, "POST", "/api/window/quit", map[string]any{}, nil)
		select {
		case <-done:
		case <-cleanup.Done():
			_ = command.Process.Kill()
			<-done
		}
	}()
	for {
		if err = control.authenticate(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return action(control)
}

func (host *server) catalogMutation(ctx context.Context, action func(*kernelState) error) error {
	if !host.desktop || runtime.GOOS != "windows" {
		return action(host.kernel)
	}
	_, err := host.kernel.changeCodexQuietlyWith(ctx, nil, false, func(executable string) error {
		return nativeCatalogManagement(ctx, executable, action)
	})
	return err
}

func runCatalogCLI(ctx context.Context, executable string, args []string) error {
	command := exec.CommandContext(ctx, executable, args...)
	hideProcess(command)
	return command.Run()
}

type syncProvider struct {
	ID        string  `json:"id"`
	Chat      string  `json:"chat"`
	Responses string  `json:"responses"`
	Anthropic string  `json:"anthropic"`
	Socket    *string `json:"socket"`
	Off       bool    `json:"off"`
	Models    []struct {
		ID string `json:"id"`
		On bool   `json:"on"`
	} `json:"models"`
}

func (host *server) prepareModels(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider       string            `json:"provider"`
		Models         []string          `json:"models"`
		ModelPrefixes  []string          `json:"modelPrefixes"`
		ExcludedModels []string          `json:"excludedModels"`
		Superseded     map[string]string `json:"superseded"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.Provider == "" || len(input.Models) > 100 || len(input.ExcludedModels) > 100 {
		host.json(w, 400, map[string]string{"error": "无法读取模型同步配置"})
		return
	}
	input.Models = slices.DeleteFunc(input.Models, func(id string) bool {
		return slices.Contains(input.ExcludedModels, id)
	})
	if host.desktop && runtime.GOOS == "darwin" && !host.allowCodexDesktopChange(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	if host.desktop && runtime.GOOS == "windows" {
		if !host.allowCodexDesktopChange(w) {
			return
		}
		valid := false
		for _, choice := range host.connection(ctx).Choices {
			if choice.ID == input.Provider {
				valid = true
			}
		}
		if !valid {
			host.json(w, http.StatusBadRequest, map[string]string{"error": "所选 ACU Key 不可用，未更改配置"})
			return
		}
		if err := host.prepareCodexSetupRoute(ctx, input.Provider, input.Models); err != nil {
			host.json(w, http.StatusConflict, map[string]string{"error": "一键配置自动修复未完成：" + err.Error()})
			return
		}
	}
	err := host.catalogMutation(ctx, func(k *kernelState) error {
		var providers struct {
			Providers []syncProvider `json:"providers"`
		}
		if err := k.exchange(ctx, "GET", "/api/providers", nil, &providers); err != nil {
			return err
		}
		found, setTransport := false, false
		for _, p := range providers.Providers {
			if p.ID != input.Provider || p.Off {
				continue
			}
			for _, address := range []string{p.Chat, p.Responses, p.Anthropic} {
				if _, err := acuBase(address); err == nil {
					found = true
					_, responseErr := acuBase(p.Responses)
					setTransport = p.ID == "acu-runtime" && (p.Socket == nil || *p.Socket == "") && responseErr == nil
				}
			}
		}
		if !found {
			return errors.New("请选择已启用的 ACU Key")
		}
		if setTransport {
			k.mu.Lock()
			executable := k.exe
			k.mu.Unlock()
			if err := runCatalogCLI(ctx, executable, []string{"provider", "set", input.Provider, "socket=off"}); err != nil {
				return errors.New("无法设置 ACU 流式连接，请重试")
			}
		}
		var refreshed struct {
			Provider syncProvider `json:"provider"`
		}
		if err := k.exchange(ctx, "POST", "/api/provider/models", map[string]string{"id": input.Provider}, &refreshed); err != nil {
			return err
		}
		if setTransport && (refreshed.Provider.Socket == nil || *refreshed.Provider.Socket != "off") {
			return errors.New("ACU 流式连接未通过核验，请重试")
		}
		wanted := map[string]bool{}
		for _, id := range input.Models {
			wanted[id] = true
		}
		args := []string{"provider", "models", input.Provider}
		available := map[string]bool{}
		for _, m := range refreshed.Provider.Models {
			available[m.ID] = true
		}
		for _, m := range refreshed.Provider.Models {
			if slices.Contains(input.ExcludedModels, m.ID) {
				continue
			}
			if successor := input.Superseded[m.ID]; successor != "" && available[successor] {
				continue
			}
			matches := wanted[m.ID]
			for _, prefix := range input.ModelPrefixes {
				if prefix == "gemini-4-" && strings.HasPrefix(m.ID, prefix) {
					matches = true
				}
			}
			if !m.On && matches && m.ID != "" && !strings.ContainsAny(m.ID, "\r\n, ") {
				args = append(args, "+"+m.ID)
			}
		}
		if len(args) == 3 {
			return nil
		}
		k.mu.Lock()
		executable := k.exe
		k.mu.Unlock()
		if executable == "" {
			return errors.New("无法找到配套内核")
		}
		if err := runCatalogCLI(ctx, executable, args); err != nil {
			return errors.New("常用模型同步未完成，请重试")
		}
		var verified struct {
			Providers []syncProvider `json:"providers"`
		}
		if err := k.exchange(ctx, "GET", "/api/providers", nil, &verified); err != nil {
			return err
		}
		for _, argument := range args[3:] {
			id, found := strings.TrimPrefix(argument, "+"), false
			for _, p := range verified.Providers {
				if p.ID == input.Provider {
					for _, m := range p.Models {
						if m.ID == id && m.On {
							found = true
						}
					}
				}
			}
			if !found {
				return errors.New("常用模型尚未启用，请重试配置")
			}
		}
		return nil
	})
	if err != nil {
		host.json(w, 409, map[string]string{"error": err.Error()})
		return
	}
	host.json(w, 200, map[string]bool{"ok": true})
}

func (host *server) syncAgentModels(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Agent      string             `json:"agent"`
		Hidden     *[]string          `json:"hidden"`
		Shown      *[]string          `json:"shown"`
		Only       *bool              `json:"only"`
		OwnHidden  *bool              `json:"ownHidden"`
		Order      *[]string          `json:"order"`
		Widen      []string           `json:"widen"`
		Operations []catalogOperation `json:"operations"`
		Expected   *agentCatalog      `json:"expected"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input) != nil {
		host.json(w, 400, map[string]string{"error": "无法读取模型列表"})
		return
	}
	allowed := false
	for _, a := range supportedAgents {
		if a.ID == input.Agent {
			allowed = true
		}
	}
	if !allowed {
		host.json(w, 400, map[string]string{"error": "请选择支持的应用"})
		return
	}
	if len(input.Operations) > 5 {
		host.json(w, 400, map[string]string{"error": "模型列表修改过多"})
		return
	}
	for _, operation := range input.Operations {
		fields := 0
		for _, present := range []bool{operation.Hidden != nil, operation.Shown != nil, operation.Only != nil, operation.OwnHidden != nil, operation.Order != nil} {
			if present {
				fields++
			}
		}
		if fields != 1 {
			host.json(w, 400, map[string]string{"error": "模型列表修改格式无效"})
			return
		}
	}
	if input.Agent == "codex" && !host.allowCodexDesktopChange(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var result map[string]any
	err := host.catalogMutation(ctx, func(k *kernelState) error {
		if input.Expected != nil {
			var current agentCatalog
			if err := k.exchange(ctx, "GET", "/api/agent-models/"+input.Agent, nil, &current); err != nil {
				return err
			}
			if !reflect.DeepEqual(current, *input.Expected) {
				return errors.New("模型列表已在其他位置变化，请取消后重新打开再修改")
			}
		}
		if len(input.Operations) > 0 {
			for _, operation := range input.Operations {
				if err := k.exchange(ctx, "POST", "/api/agent-models/"+input.Agent, operation, nil); err != nil {
					return err
				}
			}
			return k.exchange(ctx, "GET", "/api/agent-models/"+input.Agent, nil, &result)
		}
		body := map[string]any{}
		if input.Hidden != nil {
			body["hidden"] = *input.Hidden
		}
		if input.Shown != nil {
			body["shown"] = *input.Shown
		}
		if input.Only != nil {
			body["only"] = *input.Only
		}
		if input.OwnHidden != nil {
			body["ownHidden"] = *input.OwnHidden
		}
		if input.Order != nil {
			body["order"] = *input.Order
		}
		if len(input.Widen) > 0 {
			body["widen"] = input.Widen
		}
		return k.exchange(ctx, "POST", "/api/agent-models/"+input.Agent, body, &result)
	})
	if err != nil {
		host.json(w, 409, map[string]string{"error": err.Error()})
		return
	}
	host.json(w, 200, result)
}

type catalogOperation struct {
	Hidden    *[]string `json:"hidden,omitempty"`
	Shown     *[]string `json:"shown,omitempty"`
	Only      *bool     `json:"only,omitempty"`
	OwnHidden *bool     `json:"ownHidden,omitempty"`
	Order     *[]string `json:"order,omitempty"`
	Widen     []string  `json:"widen,omitempty"`
}
