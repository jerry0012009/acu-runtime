package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var assets embed.FS

var supportedAgents = []struct{ ID, Name, Config string }{
	{"codex", "Codex", ".codex/config.toml"},
	{"claude", "Claude Code", ".claude/settings.json"},
	{"workbuddy", "WorkBuddy", ".workbuddy/models.json"},
	{"hermes", "Hermes", ".hermes/config.yaml"},
}

type agentView struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Detected        bool               `json:"detected"`
	Config          bool               `json:"configFound"`
	Connected       bool               `json:"connected"`
	Model           string             `json:"model"`
	Source          string             `json:"source"`
	Path            string             `json:"path,omitempty"`
	CanConnect      bool               `json:"canConnect"`
	NeedsRepair     bool               `json:"needsRepair"`
	RestartRequired bool               `json:"restartRequired"`
	RestartKinds    []string           `json:"restartKinds,omitempty"`
	ModelOptions    []agentModelOption `json:"modelOptions,omitempty"`
	Roles           []agentRole        `json:"roles,omitempty"`
	Catalog         *agentCatalog      `json:"catalog,omitempty"`
	CatalogError    string             `json:"catalogError,omitempty"`
	LoginMode       string             `json:"loginMode,omitempty"`
}

type agentModelOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Ref   string `json:"ref,omitempty"`
	Group string `json:"group,omitempty"`
}

type kernelState struct {
	mu             sync.Mutex
	exe            string
	endpoint       *url.URL
	key            string
	client         *http.Client
	stream         *http.Client
	process        *exec.Cmd
	enforceVersion bool
	starting       bool
	lastErr        string
	phase          string
	downloaded     int64
	total          int64
}

type server struct {
	kernel           *kernelState
	token            string
	host             string
	web              http.Handler
	desktop          bool
	background       bool
	kernelProbe      func() (string, error)
	quitRequests     chan struct{}
	desktopActions   chan desktopAction
	lifecycle        context.Context
	account          accountState
	accountFile      string
	accountFileMu    sync.Mutex
	prices           priceCache
	loginRequests    chan loginTicket
	cloudTransport   http.RoundTripper
	health           keyHealth
	update           updateState
	capabilityMu     sync.Mutex
	prepareSearch    func(context.Context, string) error
	imageSkillSource func() (string, error)
	agentInstall     agentInstallState
	agentVersions    agentVersionCache
}

func secret() string {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buffer)
}

func findKernel() string {
	if configured := os.Getenv("ACU_KERNEL_PATH"); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
	}
	// The packaged CLI is the tested engine; a separately installed GUI can
	// be older even after Runtime's package was updated.
	if release, err := releaseFor(runtime.GOOS, runtime.GOARCH); err == nil {
		if executable, err := os.Executable(); err == nil {
			name := "acu-kernel"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			bundled := filepath.Join(filepath.Dir(executable), "kernel", name)
			if verifyKernel(bundled, release.sha256) {
				return bundled
			}
		}
		if verifyKernel(managedKernelPath(), release.sha256) {
			return managedKernelPath()
		}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "magpie", "magpie.exe"))
		registry := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\magpie.exe`, "/ve")
		hideProcess(registry)
		if output, err := registry.Output(); err == nil {
			if match := regexp.MustCompile(`REG_SZ\s+(.+)`).FindStringSubmatch(string(output)); len(match) == 2 {
				candidates = append(candidates, strings.TrimSpace(match[1]))
			}
		}
	} else if runtime.GOOS == "darwin" {
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			for _, name := range []string{"Magpie.app", "magpie.app"} {
				candidates = append(candidates, filepath.Join(base, name, "Contents", "MacOS", "magpie"))
			}
		}
	}
	if binary, err := exec.LookPath("magpie"); err == nil {
		candidates = append(candidates, binary)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	if release, err := releaseFor(runtime.GOOS, runtime.GOARCH); err == nil && verifyKernel(managedKernelPath(), release.sha256) {
		return managedKernelPath()
	}
	return ""
}

func newKernel(endpoint string) (*kernelState, error) {
	jar, _ := cookiejar.New(nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 20 * time.Second
	state := &kernelState{exe: findKernel(), client: &http.Client{Jar: jar, Transport: transport, Timeout: 20 * time.Second}}
	state.client.CheckRedirect = func(request *http.Request, previous []*http.Request) error {
		if len(previous) > 4 || request.URL.Host != previous[0].URL.Host {
			return errors.New("refusing redirect outside local kernel")
		}
		return nil
	}
	streamTransport := transport.Clone()
	streamTransport.ResponseHeaderTimeout = 2 * time.Minute
	state.stream = &http.Client{Jar: jar, Transport: streamTransport, CheckRedirect: state.client.CheckRedirect}
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || !loopback(parsed.Hostname()) {
			return nil, errors.New("kernel URL must be a loopback HTTP origin")
		}
		state.endpoint, state.key = parsed, os.Getenv("ACU_KERNEL_KEY")
	}
	return state, nil
}

func loopback(host string) bool {
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (kernel *kernelState) target(path string) (*url.URL, error) {
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	if kernel.endpoint == nil {
		return nil, errors.New("请先启动或连接内核控制通道")
	}
	result := *kernel.endpoint
	result.Path, result.RawQuery = path, ""
	return &result, nil
}

func (kernel *kernelState) authenticate(ctx context.Context) error {
	endpoint, err := kernel.target("/")
	if err != nil {
		return err
	}
	kernel.mu.Lock()
	key := kernel.key
	kernel.mu.Unlock()
	if key == "" {
		return nil
	}
	query := endpoint.Query()
	query.Set("k", key)
	endpoint.RawQuery = query.Encode()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	response, err := kernel.client.Do(request)
	if err != nil {
		return errors.New("内核控制通道尚未就绪")
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return fmt.Errorf("内核鉴权失败（HTTP %d）", response.StatusCode)
	}
	return nil
}

func (kernel *kernelState) readState(ctx context.Context) (map[string]json.RawMessage, error) {
	endpoint, err := kernel.target("/api/state")
	if err != nil {
		return nil, err
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	response, err := kernel.client.Do(request)
	if err != nil {
		return nil, errors.New("无法读取内核；控制通道可能未启动")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("内核状态读取失败（HTTP %d）", response.StatusCode)
	}
	var result map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&result); err != nil {
		return nil, errors.New("内核接口不兼容：未返回可识别的状态")
	}
	if _, ok := result["agents"]; !ok {
		return nil, errors.New("内核状态缺少 agents 字段，请核对兼容版本")
	}
	if err := expandKernelOptions(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (kernel *kernelState) begin(ctx context.Context) {
	kernel.mu.Lock()
	if kernel.starting {
		kernel.mu.Unlock()
		return
	}
	kernel.starting, kernel.lastErr, kernel.phase = true, "", "正在连接已有内核"
	kernel.mu.Unlock()
	go func() {
		startup, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		err := kernel.start(startup)
		kernel.mu.Lock()
		defer kernel.mu.Unlock()
		kernel.starting = false
		if err != nil {
			kernel.lastErr, kernel.phase = err.Error(), "连接未完成"
		} else {
			kernel.lastErr, kernel.phase = "", "已连接"
		}
	}()
}

func (kernel *kernelState) start(ctx context.Context) error {
	kernel.mu.Lock()
	if kernel.process != nil {
		kernel.mu.Unlock()
		return kernel.authenticate(ctx)
	}
	if kernel.endpoint != nil {
		kernel.mu.Unlock()
		return kernel.authenticate(ctx)
	}
	executable := kernel.exe
	kernel.mu.Unlock()
	if executable == "" {
		installed, err := kernel.install(ctx)
		if err != nil {
			return err
		}
		executable = installed
		kernel.mu.Lock()
		kernel.exe = executable
		kernel.mu.Unlock()
	}
	if err := kernel.ensureCompatibleGateway(ctx); err != nil {
		return err
	}
	policy, cancelPolicy := context.WithTimeout(ctx, 20*time.Second)
	err := pauseKernelUpdates(policy, executable)
	if err == nil {
		err = prepareKernelDetection(policy, executable)
	}
	cancelPolicy()
	if err != nil {
		return err
	}
	for _, directory := range kernelConfigDirs(executable) {
		probe, cancel := context.WithTimeout(ctx, 2*time.Second)
		attached := kernel.attachExisting(probe, browserKernelLink(directory))
		cancel()
		if attached {
			return nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	kernel.mu.Lock()
	kernel.phase = "正在启动控制通道"
	kernel.mu.Unlock()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	address := listener.Addr().String()
	listener.Close()
	endpoint, _ := url.Parse("http://" + address)
	key := secret()
	command := exec.Command(executable, "web", "--addr", address, "--no-open")
	hideProcess(command)
	command.Env = append(os.Environ(), "MAGPIE_WEB_KEY="+key)
	output := &kernelOutput{}
	command.Stdout, command.Stderr = output, io.Discard
	kernel.mu.Lock()
	if err := ctx.Err(); err != nil {
		kernel.mu.Unlock()
		return err
	}
	if err := command.Start(); err != nil {
		kernel.mu.Unlock()
		return errors.New("无法启动已有内核的控制通道")
	}
	kernel.endpoint, kernel.key, kernel.process = endpoint, key, command
	kernel.lastErr = ""
	kernel.mu.Unlock()
	go func() {
		err := command.Wait()
		kernel.mu.Lock()
		if kernel.process == command {
			kernel.process = nil
			if kernel.endpoint != nil && kernel.endpoint.Host == address {
				kernel.endpoint, kernel.key = nil, ""
			}
			if err != nil {
				kernel.lastErr = "控制通道进程已退出；未停止原有内核进程"
			}
		}
		kernel.mu.Unlock()
	}()
	for attempt := 0; attempt < 24; attempt++ {
		for _, link := range output.links() {
			probe, cancel := context.WithTimeout(ctx, time.Second)
			attached := kernel.attachExisting(probe, link)
			cancel()
			if attached {
				return nil
			}
		}
		probe, cancel := context.WithTimeout(ctx, time.Second)
		err := kernel.authenticate(probe)
		if err == nil {
			_, err = kernel.readState(probe)
		}
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			kernel.close()
			return ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	kernel.close()
	return errors.New("控制通道启动超时；请检查内核版本或已有进程，不会切换 Agent")
}

func (kernel *kernelState) close() {
	kernel.mu.Lock()
	process := kernel.process
	kernel.mu.Unlock()
	if process != nil && process.Process != nil {
		_ = process.Process.Kill()
	}
}

func (host *server) json(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	json.NewEncoder(response).Encode(value)
}

func (host *server) status(response http.ResponseWriter, request *http.Request) {
	home, _ := os.UserHomeDir()
	agents := make([]agentView, 0, len(supportedAgents))
	for _, agent := range supportedAgents {
		path := filepath.Join(home, filepath.FromSlash(agent.Config))
		if agent.ID == "hermes" && runtime.GOOS == "windows" {
			path = filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes", "config.yaml")
		}
		_, err := os.Stat(path)
		agents = append(agents, agentView{ID: agent.ID, Name: agent.Name, Config: err == nil, Path: path})
	}
	kernel := host.kernel
	kernel.mu.Lock()
	exe, starting, lastErr := kernel.exe, kernel.starting, kernel.lastErr
	phase, downloaded, total := kernel.phase, kernel.downloaded, kernel.total
	attached := kernel.endpoint != nil
	kernel.mu.Unlock()
	ready := false
	if attached && !starting {
		ctx, cancel := context.WithTimeout(request.Context(), 12*time.Second)
		defer cancel()
		if state, err := kernel.readState(ctx); err == nil {
			var current []struct {
				ID          string          `json:"id"`
				Wired       bool            `json:"wired"`
				Source      string          `json:"source"`
				Path        string          `json:"path"`
				CLIMissing  bool            `json:"cliMissing"`
				Import      string          `json:"import"`
				Drift       json.RawMessage `json:"drift"`
				Stale       int             `json:"stale"`
				AppRestart  json.RawMessage `json:"appRestart"`
				StaleCopies []struct {
					Kind string `json:"kind"`
				} `json:"staleCopies"`
				Models *struct {
					Listed int `json:"listed"`
				} `json:"models"`
				Fields []struct {
					Key     string `json:"key"`
					Value   string `json:"value"`
					Options []struct {
						Ref   string `json:"ref"`
						Value string `json:"value"`
						Label string `json:"label"`
						Group string `json:"group"`
					} `json:"options"`
				} `json:"fields"`
			}
			if json.Unmarshal(state["agents"], &current) == nil {
				ready = true
				for index := range agents {
					for _, native := range current {
						if native.ID == agents[index].ID {
							agents[index].Detected, agents[index].Connected, agents[index].Source, agents[index].Path = !native.CLIMissing, native.Wired, native.Source, native.Path
							agents[index].NeedsRepair = len(native.Drift) > 0 && string(native.Drift) != "null"
							agents[index].RestartRequired = native.Stale > 0 || len(native.AppRestart) > 0 && string(native.AppRestart) != "null"
							if len(native.AppRestart) > 0 && string(native.AppRestart) != "null" {
								agents[index].RestartKinds = append(agents[index].RestartKinds, "app")
							}
							for _, copy := range native.StaleCopies {
								switch copy.Kind {
								case "app", "ide", "daemon", "embedded", "cli":
									agents[index].RestartKinds = append(agents[index].RestartKinds, copy.Kind)
								}
							}
							connectable := native.Models != nil && native.Models.Listed > 0
							for _, field := range native.Fields {
								if field.Key == "login" {
									agents[index].LoginMode = field.Value
								}
								switch field.Key {
								case "model", "small", "opus", "sonnet", "haiku", "fable", "subagent", "subagent_model", "memories":
									if field.Value != "" {
										role := agentRole{Key: field.Key, Value: field.Value}
										for _, option := range field.Options {
											if option.Value == field.Value {
												role.Ref, role.Name, role.Group = option.Ref, option.Label, option.Group
											}
										}
										agents[index].Roles = append(agents[index].Roles, role)
									}
								}
								if field.Key == "model" {
									agents[index].Model = field.Value
									for _, option := range field.Options {
										agents[index].ModelOptions = append(agents[index].ModelOptions, agentModelOption{Value: option.Value, Label: option.Label, Ref: option.Ref, Group: option.Group})
									}
								}
								for _, option := range field.Options {
									connectable = connectable || option.Ref != "" || option.Value == "magpie"
								}
							}
							agents[index].CanConnect = connectable && !native.CLIMissing && native.Import == ""
						}
					}
				}
			}
		} else {
			lastErr = err.Error()
		}
	}
	connection := connectionView{}
	var groups []routingGroupView
	if ready {
		ctx, cancel := context.WithTimeout(request.Context(), 12*time.Second)
		connection = host.connectionHealth(host.connection(ctx))
		groups = host.modelInventory(ctx, agents)
		cancel()
	}
	host.json(response, 200, map[string]any{"version": appVersion, "platform": runtime.GOOS, "desktop": host.desktop, "kernelFound": exe != "", "kernelPath": exe, "ready": ready, "starting": starting, "phase": phase, "downloaded": downloaded, "downloadTotal": total, "error": lastErr, "agents": agents, "routingGroups": groups, "connection": connection, "account": host.account.snapshot(), "update": host.update.snapshot(host.desktop), "agentInstall": host.agentInstall.snapshot()})
}

func (host *server) proxy(response http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/kernel")
	if path == "" {
		path = "/"
	}
	if strings.Contains(path, "..") || strings.HasPrefix(path, "//") {
		http.Error(response, "invalid kernel path", 400)
		return
	}
	if !host.allowDesktopMutation(response, request, path) {
		return
	}
	endpoint, err := host.kernel.target(path)
	if err != nil {
		http.Error(response, err.Error(), 503)
		return
	}
	endpoint.RawQuery = request.URL.RawQuery
	if endpoint.Query().Has("k") {
		http.Error(response, "kernel credentials cannot be supplied through the browser", 400)
		return
	}
	ctx := request.Context()
	upstream, err := http.NewRequestWithContext(ctx, request.Method, endpoint.String(), http.MaxBytesReader(response, request.Body, 8<<20))
	if err != nil {
		http.Error(response, "invalid request", 400)
		return
	}
	for _, name := range []string{"Content-Type", "Accept"} {
		if value := request.Header.Get(name); value != "" {
			upstream.Header.Set(name, value)
		}
	}
	upstream.Header.Set("Accept-Encoding", "identity")
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		upstream.Header.Set("Origin", endpoint.Scheme+"://"+endpoint.Host)
	}
	result, err := host.kernel.stream.Do(upstream)
	if err != nil {
		http.Error(response, "内核控制通道不可用", 502)
		return
	}
	defer result.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Disposition"} {
		if value := result.Header.Get(name); value != "" {
			response.Header().Set(name, value)
		}
	}
	response.Header().Set("Cache-Control", "no-store")
	if path == "/" && strings.Contains(result.Header.Get("Content-Type"), "text/html") {
		content, err := io.ReadAll(io.LimitReader(result.Body, (4<<20)+1))
		if err != nil || len(content) > 4<<20 {
			http.Error(response, "无法读取内核界面", 502)
			return
		}
		injection := `<script src="/ui/theme.js"></script><script src="/ui/kernel-host.js"></script><link rel="stylesheet" href="/ui/theme.css"><link rel="stylesheet" href="/ui/kernel-embed.css"><script src="/ui/kernel-bridge.js" defer></script>`
		html := strings.Replace(string(content), "</head>", injection+"</head>", 1)
		response.WriteHeader(result.StatusCode)
		io.WriteString(response, html)
		return
	}
	response.WriteHeader(result.StatusCode)
	if strings.HasPrefix(result.Header.Get("Content-Type"), "text/event-stream") {
		buffer := make([]byte, 32<<10)
		for {
			count, readErr := result.Body.Read(buffer)
			if count > 0 {
				if _, err := response.Write(buffer[:count]); err != nil {
					return
				}
				if flusher, ok := response.(http.Flusher); ok {
					flusher.Flush()
				}
			}
			if readErr != nil {
				return
			}
		}
	}
	io.Copy(response, result.Body)
}

func (host *server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Host != host.host {
		http.Error(response, "invalid host", 403)
		return
	}
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Frame-Options", "SAMEORIGIN")
	response.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	if origin := request.Header.Get("Origin"); origin != "" && origin != "http://"+host.host {
		http.Error(response, "cross-origin requests are forbidden", 403)
		return
	}
	if request.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(response, "cross-site requests are forbidden", 403)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/runtime/") {
		if request.Method != http.MethodGet && request.Header.Get("X-ACU-Token") != host.token {
			host.json(response, 403, map[string]string{"error": "invalid local session"})
			return
		}
		switch {
		case host.desktop && (request.Method == http.MethodGet || request.Method == http.MethodPost) && request.URL.Path == "/runtime/startup":
			host.startupSettings(response, request)
		case request.Method == http.MethodGet && request.URL.Path == "/runtime/agents/versions":
			host.agentVersionStatus(response, request)
		case (request.Method == http.MethodGet || request.Method == http.MethodPut) && strings.HasPrefix(request.URL.Path, "/runtime/preferences/"):
			host.accountPreferences(response, request)
		case request.Method == http.MethodGet && request.URL.Path == "/runtime/billing/prices":
			host.modelPrices(response, request)
		case request.Method == http.MethodGet && request.URL.Path == "/runtime/billing/account":
			host.accountBilling(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/account/logout":
			host.forgetAccount()
			host.json(response, 200, map[string]bool{"ok": true})
		case (request.URL.Path == "/runtime/agents/install" && (request.Method == http.MethodGet || request.Method == http.MethodPost)) || (request.URL.Path == "/runtime/agents/install/cancel" && request.Method == http.MethodPost):
			host.agentInstaller(response, request)
		case request.URL.Path == "/runtime/capabilities" && (request.Method == http.MethodGet || request.Method == http.MethodPost):
			host.capabilities(response, request)
		case (request.Method == http.MethodGet && request.URL.Path == "/runtime/update") || (request.Method == http.MethodPost && (request.URL.Path == "/runtime/update/check" || request.URL.Path == "/runtime/update/install")):
			host.updates(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/desktop/action":
			host.desktopAction(response, request)
		case request.Method == http.MethodGet && request.URL.Path == "/runtime/status":
			host.status(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/account/login":
			host.beginAccountLogin(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/account/cancel":
			host.account.mu.Lock()
			host.account.ticket = loginTicket{}
			host.account.mu.Unlock()
			host.json(response, 200, map[string]bool{"ok": true})
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/connection":
			host.configureACU(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/agents/configure":
			host.configureAgent(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/kernel/start":
			lifecycle := host.lifecycle
			if lifecycle == nil {
				lifecycle = context.Background()
			}
			host.kernel.begin(lifecycle)
			host.status(response, request)
		case request.Method == http.MethodPost && request.URL.Path == "/runtime/desktop/quit" && host.desktop:
			host.json(response, 200, map[string]bool{"ok": true})
			select {
			case host.quitRequests <- struct{}{}:
			default:
			}
		default:
			http.NotFound(response, request)
		}
		return
	}
	if request.URL.Path == "/" || request.URL.Path == "/panel" {
		if request.Method != http.MethodGet {
			response.WriteHeader(405)
			return
		}
		page := "web/index.html"
		if request.URL.Path == "/panel" {
			page = "web/panel.html"
		}
		body, _ := assets.ReadFile(page)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		io.WriteString(response, strings.Replace(string(body), "__ACU_TOKEN__", host.token, 1))
		return
	}
	if strings.HasPrefix(request.URL.Path, "/ui/") {
		host.web.ServeHTTP(response, request)
		return
	}
	host.proxy(response, request)
}

func main() {
	address := flag.String("listen", "", "local interface address; a random port in the desktop app")
	endpoint := flag.String("kernel-url", "", "existing loopback kernel management origin")
	webMode := flag.Bool("web", !desktopEnabled, "developer browser mode instead of the native desktop window")
	inspectPort := flag.Int("inspect-port", 0, "developer WebView inspection port; disabled by default")
	background := flag.Bool("background", false, "start in the tray at sign-in")
	installTo := flag.String("install-to", "", "installer-only destination directory")
	agentPackagesFrom := flag.String("agent-packages-from", "", "optional offline vendor packages beside the installer")
	agentPackagesJSON := flag.Bool("agent-packages-json", false, "print official desktop package metadata")
	releaseJSON := flag.Bool("kernel-release-json", false, "print the pinned kernel package metadata")
	searchReleaseJSON := flag.Bool("search-runtime-release-json", false, "print the pinned search runtime metadata")
	flag.Parse()
	if *agentPackagesJSON {
		workbuddy, _ := workBuddyPackage(runtime.GOOS, runtime.GOARCH)
		chatgpt, _ := chatGPTPackage(runtime.GOOS, runtime.GOARCH)
		json.NewEncoder(os.Stdout).Encode(map[string]agentPackage{"workbuddy": workbuddy, "chatgpt": chatgpt})
		return
	}
	if *searchReleaseJSON {
		release, err := searchRuntimeRelease(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"version": searchRuntimeVersion, "name": release.name, "sha256": release.sha256})
		return
	}
	if *releaseJSON {
		release, err := releaseFor(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"version": kernelVersion, "name": release.name, "sha256": release.sha256})
		return
	}
	if *installTo != "" {
		if runtime.GOOS != "windows" {
			log.Fatal("this installer target is Windows-only")
		}
		executable, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		release, err := releaseFor(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			log.Fatal(err)
		}
		if err := prepareInstallation(filepath.Dir(executable), *installTo, release); err != nil {
			log.Fatal(err)
		}
		if *agentPackagesFrom != "" {
			if err := copyOfflineAgentPackages(*agentPackagesFrom, *installTo); err != nil {
				log.Fatal(err)
			}
		}
		return
	}
	if *address == "" {
		*address = "127.0.0.1:0"
		if *webMode {
			*address = "127.0.0.1:4317"
		}
	}
	listenHost, _, err := net.SplitHostPort(*address)
	if err != nil || !loopback(listenHost) {
		log.Fatal("listen address must use a loopback IP")
	}
	kernel, err := newKernel(*endpoint)
	if err != nil {
		log.Fatal(err)
	}
	kernel.enforceVersion = desktopEnabled && *endpoint == "" && os.Getenv("ACU_KERNEL_PATH") == ""
	defer kernel.close()
	if *endpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := kernel.authenticate(ctx)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	}
	web, _ := fs.Sub(assets, "web")
	host := &server{kernel: kernel, token: secret(), host: *address, desktop: !*webMode, background: *background, quitRequests: make(chan struct{}, 1), desktopActions: make(chan desktopAction, 16), loginRequests: make(chan loginTicket, 1), web: http.StripPrefix("/ui/", http.FileServer(http.FS(web)))}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}
	host.host = listener.Addr().String()
	service := &http.Server{Handler: host, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	host.lifecycle = ctx
	defer stop()
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			stop()
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			service.Shutdown(shutdown)
			kernel.close()
		})
	}
	defer shutdown()
	serviceErrors := make(chan error, 1)
	go func() { serviceErrors <- service.Serve(listener) }()
	if !*webMode {
		if err := initializeStartup(); err != nil {
			log.Printf("开机自启动设置未完成：%v", err)
		}
		kernel.begin(ctx)
		if err := repairConnectedWorkBuddyPDF(); err != nil {
			log.Printf("WorkBuddy PDF 组件路径同步未完成：%v", err)
		}
		if err := runDesktop(ctx, host, "http://"+host.host, *inspectPort, host.quitRequests, shutdown); err != nil {
			log.Print(err)
		}
		return
	}
	log.Printf("ACU Runtime v%s developer web mode at http://%s", appVersion, host.host)
	select {
	case <-ctx.Done():
	case err := <-serviceErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Print(err)
		}
	}
}
