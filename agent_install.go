package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type agentInstallView struct {
	Agent      string `json:"agent"`
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Update     bool   `json:"update,omitempty"`
	Version    string `json:"version,omitempty"`
}

type agentInstallState struct {
	mu      sync.Mutex
	view    agentInstallView
	running bool
	cancel  context.CancelFunc
}

func (s *agentInstallState) snapshot() agentInstallView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

func (s *agentInstallState) phase(phase, message string) {
	s.mu.Lock()
	s.view.Phase, s.view.Message = phase, message
	s.mu.Unlock()
}

type agentPackage struct{ URL, SHA256, Name string }

func verifiedAgentPackage(path, digest string) bool {
	if !digestPattern.MatchString(digest) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (1<<30)+1))
	return err == nil && n <= 1<<30 && hex.EncodeToString(h.Sum(nil)) == digest
}

func cachedWorkBuddyPackage(pkg agentPackage) string {
	if executable, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(executable), "agent-packages", pkg.Name)
		if verifiedAgentPackage(path, pkg.SHA256) {
			return path
		}
	}
	if dir, err := agentPackageCache(); err == nil {
		path := filepath.Join(dir, pkg.Name)
		if verifiedAgentPackage(path, pkg.SHA256) {
			return path
		}
	}
	return ""
}

// Vendor's domestic CDN, fixed version and digests from its public update API.
// No mirror operator can choose or change executable bytes.
func workBuddyPackage(platform, arch string) (agentPackage, error) {
	base := "https://download.codebuddy.cn/workbuddy/saas/"
	version := "5.9.3.42398134-d4692421"
	var flavor, suffix, digest string
	switch platform + "/" + arch {
	case "windows/amd64":
		flavor, suffix, digest = "win32-x64-user", ".exe", "b5edb52bd6c5b6c7296cf2fff697b019c86d11992a8a814c3010d611896852f9"
	case "darwin/arm64":
		flavor, suffix, digest = "darwin-arm64", ".zip", "55d73c63e44be58e31411254d10ad141faef9fd9eaf3cab67685d81f88e14835"
	case "darwin/amd64":
		flavor, suffix, digest = "darwin-x64", ".zip", "e093e214cb77b01d821451ee19b67b0e0b2bdb63f87dcea3aacbccb21bdfc686"
	default:
		return agentPackage{}, errors.New("此平台暂不支持自动安装 WorkBuddy")
	}
	name := "WorkBuddy-" + flavor + "-" + version + suffix
	return agentPackage{base + flavor + "/" + name, digest, name}, nil
}

func agentDownloadClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = updateProxy
	t.ResponseHeaderTimeout = 20 * time.Second
	return &http.Client{Transport: t, Timeout: 15 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || r.URL.Scheme != "https" || r.URL.Hostname() != "download.codebuddy.cn" || r.URL.User != nil {
			return errors.New("应用下载重定向地址不受信任")
		}
		return nil
	}}
}

func downloadAgentPackage(ctx context.Context, client *http.Client, source, path, digest string, progress func(int64, int64)) error {
	if !trustedWorkBuddyURL(source) {
		return errors.New("应用下载地址不受信任")
	}
	if !digestPattern.MatchString(digest) {
		return errors.New("应用校验信息无效")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	r, err := client.Do(req)
	if err != nil {
		return errors.New("应用下载暂不可用，请检查网络后重试")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("应用下载失败（HTTP %d），请重试", r.StatusCode)
	}
	const limit = int64(1 << 30)
	if r.ContentLength > limit {
		return errors.New("应用安装包过大")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	good := false
	defer func() {
		f.Close()
		if !good {
			os.Remove(path)
		}
	}()
	h := sha256.New()
	w := &kernelDownloadWriter{writer: io.MultiWriter(f, h), total: r.ContentLength, progress: progress}
	n, err := io.Copy(w, io.LimitReader(r.Body, limit+1))
	if err != nil {
		return errors.New("应用下载中断，请重试")
	}
	if n > limit || hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("应用安装包校验失败，未执行安装")
	}
	if err = f.Close(); err != nil {
		return err
	}
	good = true
	return nil
}

func (host *server) detectedAgent(ctx context.Context, id string) (bool, error) {
	probe, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := host.kernel.exchange(probe, "POST", "/api/agents/rescan", map[string]any{}, nil); err != nil {
		return false, err
	}
	state, err := host.kernel.readState(probe)
	if err != nil {
		return false, err
	}
	var agents []setupAgent
	if json.Unmarshal(state["agents"], &agents) != nil {
		return false, errors.New("无法读取应用状态")
	}
	for _, a := range agents {
		if a.ID == id {
			return !a.CLIMissing, nil
		}
	}
	return false, nil
}

// Reuse the existing configuration handler and kernel APIs. Do not implement
// another client config writer or touch sign-in credentials.
type installResponse struct {
	header http.Header
	body   strings.Builder
	status int
}

func (r *installResponse) Header() http.Header         { return r.header }
func (r *installResponse) WriteHeader(n int)           { r.status = n }
func (r *installResponse) Write(b []byte) (int, error) { return r.body.Write(b) }

func (host *server) configureInstalledAgent(ctx context.Context, id, provider, model string) error {
	b, _ := json.Marshal(map[string]any{"agent": id, "provider": provider, "model": model, "preserve": true})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://"+host.host+"/runtime/agents/configure", strings.NewReader(string(b)))
	w := &installResponse{header: make(http.Header)}
	host.configureAgent(w, req)
	if w.status != 200 {
		var e struct{ Error string }
		json.Unmarshal([]byte(w.body.String()), &e)
		if e.Error != "" {
			return errors.New(e.Error)
		}
		return errors.New("应用已安装，模型配置未完成，请重试")
	}
	host.capabilityMu.Lock()
	defer host.capabilityMu.Unlock()
	toolsCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	_, err := host.capabilityApply(toolsCtx, capabilityInput{Provider: provider, Agents: []string{id}})
	return err
}

func (host *server) runAgentInstall(ctx context.Context, id, provider, model string, update bool) {
	s := &host.agentInstall
	defer func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }()
	fail := func(err error) { s.phase("error", err.Error()) }
	if update {
		defer func() {
			host.agentVersions.Lock()
			host.agentVersions.at = time.Time{}
			host.agentVersions.Unlock()
		}()
		key, label := "chatgpt", "ChatGPT"
		if id == "workbuddy" {
			key, label = "workbuddy-desktop", "WorkBuddy"
		}
		info, ok := host.desktopAgentVersions(ctx)[key]
		if !ok || !info.Installed {
			fail(errors.New("未检测到 " + label + " 桌面端，请使用安装入口"))
			return
		}
		if info.Error != "" || info.Latest == "" {
			fail(errors.New("最新版本尚未确认，请重新检查更新"))
			return
		}
		if !info.Update {
			s.phase("complete", label+" 已是最新版本（"+info.Version+"）")
			return
		}
		if id == "codex" {
			if running, probeErr := codexDesktopOpen(); probeErr != nil || running {
				fail(errors.New("更新客户端前请保存并退出 ChatGPT；更新只替换客户端，不修改 ACU 配置"))
				return
			}
			if err := host.prepareChatGPT(ctx, true); err != nil {
				fail(err)
				return
			}
		} else {
			pkg, err := latestWorkBuddyPackage(ctx, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				fail(err)
				return
			}
			file := cachedWorkBuddyPackage(pkg)
			if file == "" {
				dir, err := agentPackageCache()
				if err != nil {
					fail(err)
					return
				}
				file = filepath.Join(dir, pkg.Name)
				if regularFile(file) {
					if err = os.Remove(file); err != nil {
						fail(err)
						return
					}
				}
				s.phase("downloading", "正在下载 WorkBuddy 官方更新")
				err = downloadAgentPackage(ctx, agentDownloadClient(), pkg.URL, file, pkg.SHA256, nil)
				if err != nil {
					fail(err)
					return
				}
			}
			s.phase("installing", "正在更新 WorkBuddy，请按官方安装器提示保存工作")
			if err = installWorkBuddy(ctx, file); err != nil {
				fail(err)
				return
			}
		}
		if id == "codex" {
			verified, err := host.verifyDesktopUpdate(ctx, key, info.Latest)
			if err != nil {
				fail(err)
				return
			}
			s.mu.Lock()
			s.view.Version = verified.Version
			s.mu.Unlock()
			s.phase("complete", label+" 已更新至 "+verified.Version)
		} else {
			s.phase("complete", label+" 桌面端已更新，账号与 ACU 接入配置保留。")
		}
		return
	}

	exists, err := host.detectedAgent(ctx, id)
	if err != nil {
		fail(err)
		return
	}
	if id == "codex" && (!chatGPTInstalled() || update) {
		if err = host.prepareChatGPT(ctx, update); err != nil {
			fail(err)
			return
		}
		// Native installation registers the app before its first launch. Open
		// only after configuration below, so a fresh user needn't sign in first.
		exists, err = host.detectedAgent(ctx, id)
		if err != nil {
			fail(err)
			return
		}
	}
	if !exists {
		if id == "workbuddy" && !workBuddyInstalled() {
			s.phase("checking", "正在检查 WorkBuddy 官方最新版本")
			pkg, e := latestWorkBuddyPackage(ctx, runtime.GOOS, runtime.GOARCH)
			if e != nil {
				fail(e)
				return
			}
			path := cachedWorkBuddyPackage(pkg)
			if path == "" {
				dir, e := agentPackageCache()
				if e != nil {
					fail(e)
					return
				}
				path = filepath.Join(dir, pkg.Name)
				// Only the fixed cache candidate is removed, never a user app.
				if regularFile(path) {
					if e := os.Remove(path); e != nil {
						fail(e)
						return
					}
				}
				s.phase("downloading", "正在从国内官方下载线路准备 WorkBuddy")
				err = downloadAgentPackage(ctx, agentDownloadClient(), pkg.URL, path, pkg.SHA256, func(done, total int64) { s.mu.Lock(); s.view.Downloaded, s.view.Total = done, total; s.mu.Unlock() })
				if err != nil {
					fail(err)
					return
				}
			}
			s.phase("installing", "正在安装 WorkBuddy；如有系统确认，请按提示完成")
			if err = installWorkBuddy(ctx, path); err != nil {
				fail(err)
				return
			}
			// Let the vendor's first launch initialize its data directory.
			if err = openInstalledAgent(ctx, id); err != nil {
				fail(err)
				return
			}
			s.phase("waiting", "应用已安装，正在等待首次启动；如出现登录页，请按应用提示登录。")
		} else if id == "codex" {
			s.phase("waiting", "请完成系统安装提示并打开一次 ChatGPT；无需先登录 ChatGPT 账号，检测完成后将配置 ACU。")
			if err := openInstalledAgent(ctx, id); err != nil {
				fail(err)
				return
			}
		} else {
			s.phase("waiting", "检测到已有 WorkBuddy，正在等待应用初始化；账号和登录状态保留。")
			if err = openInstalledAgent(ctx, id); err != nil {
				fail(err)
				return
			}
		}
		deadline := time.NewTimer(15 * time.Minute)
		defer deadline.Stop()
		tick := time.NewTicker(4 * time.Second)
		defer tick.Stop()
		for !exists {
			select {
			case <-ctx.Done():
				fail(errors.New("安装等待已结束，可重新检查并配置"))
				return
			case <-deadline.C:
				fail(errors.New("尚未检测到应用，请完成安装后重试"))
				return
			case <-tick.C:
				probe, cancel := context.WithTimeout(ctx, 15*time.Second)
				exists, _ = host.detectedAgent(probe, id)
				cancel()
			}
		}
	}
	s.phase("configuring", "应用已检测到，正在通过 ACU 内核配置模型与常用工具")
	if err = host.configureInstalledAgent(ctx, id, provider, model); err != nil {
		fail(err)
		return
	}
	message := "接入配置已完成，已有账号和登录状态保留。首次使用请按应用提示确认工具权限。"
	if id == "codex" {
		message += " 没有 ChatGPT 账号时，ACU 内核自动使用 API 模式。已有窗口请保存工作后重新打开；云端账号功能仍需官方登录。"
	}
	if id == "workbuddy" {
		message += " 如应用要求登录，请使用微信、手机号等官方方式完成；ACU 模型已准备。请保存工作后重新打开应用，让新增模型和工具生效。"
	}
	s.phase("complete", message)
	if err = openInstalledAgent(ctx, id); err != nil {
		s.phase("complete", message+" 请从开始菜单或应用程序中打开应用。")
	}
}

func (host *server) agentInstaller(w http.ResponseWriter, r *http.Request) {
	if !host.desktop {
		host.json(w, 409, map[string]string{"error": "请在 ACU Runtime 桌面应用中安装"})
		return
	}
	s := &host.agentInstall
	if r.Method == "GET" {
		host.json(w, 200, s.snapshot())
		return
	}
	if r.URL.Path == "/runtime/agents/install/cancel" {
		s.mu.Lock()
		if (s.view.Phase == "waiting" || s.view.Phase == "downloading" || s.view.Phase == "checking") && s.cancel != nil {
			s.cancel()
		}
		s.mu.Unlock()
		host.json(w, 200, s.snapshot())
		return
	}
	var in struct {
		Agent, Provider, Model string
		Update                 bool
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF || (in.Agent != "workbuddy" && in.Agent != "codex") {
		host.json(w, 400, map[string]string{"error": "不支持的应用安装请求"})
		return
	}
	if in.Update && in.Agent != "codex" && in.Agent != "workbuddy" {
		host.json(w, 400, map[string]string{"error": "此应用请使用客户端内更新"})
		return
	}
	if !in.Update && (in.Provider == "" || in.Model == "") {
		host.json(w, 400, map[string]string{"error": "请先连接 ACU 并选择可用模型"})
		return
	}
	host.kernel.mu.Lock()
	ready := host.kernel.endpoint != nil && !host.kernel.starting
	host.kernel.mu.Unlock()
	if !ready {
		host.json(w, 409, map[string]string{"error": "请先等待 ACU 内核连接完成"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	valid := in.Update
	if !in.Update {
		for _, p := range host.connection(ctx).Choices {
			if p.ID == in.Provider {
				for _, m := range p.Models {
					valid = valid || m == in.Model
				}
			}
		}
	}
	cancel()
	if !valid {
		host.json(w, 400, map[string]string{"error": "ACU Key 或模型不可用，请重新连接"})
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		host.json(w, 409, map[string]string{"error": "已有应用正在安装，请等待完成"})
		return
	}
	s.running = true
	s.view = agentInstallView{Agent: in.Agent, Phase: "checking", Message: "正在检查已有应用；已有安装和登录状态将保留", Update: in.Update}
	base := host.lifecycle
	if base == nil {
		base = context.Background()
	}
	ctxRun, cancelRun := context.WithTimeout(base, 60*time.Minute)
	s.cancel = cancelRun
	s.mu.Unlock()
	go func() {
		defer cancelRun()
		host.runAgentInstall(ctxRun, in.Agent, in.Provider, in.Model, in.Update)
	}()
	host.json(w, 202, s.snapshot())
}

// URL parsing helper kept here so download source validation can be tested
// independently from platform installers.
func trustedWorkBuddyURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == "download.codebuddy.cn" && u.User == nil && u.Port() == ""
}

func appendEnvironment(key, value string) []string {
	env := os.Environ()
	for i := len(env) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.SplitN(env[i], "=", 2)[0], key) {
			env = append(env[:i], env[i+1:]...)
		}
	}
	return append(env, key+"="+value)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func workBuddyExecutableCandidates() []string {
	return []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "WorkBuddy", "WorkBuddy.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "WorkBuddy", "WorkBuddy.exe"),
	}
}
