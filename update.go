package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const appVersion = "0.1.76"
const updateRepository = "https://github.com/jerry0012009/acu-runtime"
const updateAPI = "https://api.github.com/repos/jerry0012009/acu-runtime/releases/latest"

var releaseVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type updateRelease struct {
	Version string `json:"version"`
	URL     string `json:"-"`
	SHA256  string `json:"-"`
	Notes   string `json:"notes"`
	Page    string `json:"page"`
}
type updateView struct {
	Phase      string         `json:"phase"`
	Current    string         `json:"current"`
	Kernel     string         `json:"kernel"`
	Supported  bool           `json:"supported"`
	Release    *updateRelease `json:"release,omitempty"`
	Message    string         `json:"message,omitempty"`
	Downloaded int64          `json:"downloaded,omitempty"`
	Total      int64          `json:"total,omitempty"`
}
type updateState struct {
	mu   sync.Mutex
	view updateView
}

func (u *updateState) snapshot(desktop bool) updateView {
	u.mu.Lock()
	defer u.mu.Unlock()
	v := u.view
	v.Current = appVersion
	v.Kernel = kernelVersion
	v.Supported = desktop && runtime.GOOS == "windows" && runtime.GOARCH == "amd64"
	if v.Phase == "" {
		v.Phase = "idle"
	}
	return v
}
func newerVersion(candidate, current string) bool {
	a, b := releaseVersionPattern.FindStringSubmatch(candidate), releaseVersionPattern.FindStringSubmatch(current)
	if a == nil || b == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, e := strconv.ParseUint(a[i], 10, 32)
		y, f := strconv.ParseUint(b[i], 10, 32)
		if e != nil || f != nil {
			return false
		}
		if x != y {
			return x > y
		}
	}
	return false
}
func updateClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = updateProxy
	transport.ResponseHeaderTimeout = 12 * time.Second
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		h := r.URL.Hostname()
		if len(via) > 5 || r.URL.Scheme != "https" || (h != "github.com" && h != "api.github.com" && h != "release-assets.githubusercontent.com" && h != "objects.githubusercontent.com" && h != "gh-proxy.com" && h != "ghfast.top") {
			return errors.New("更新下载重定向地址不受信任")
		}
		return nil
	}}
}

func checkUpdateWithFallback(ctx context.Context, client *http.Client, endpoints []string, current, platform, arch string) (*updateRelease, error) {
	var err error
	for _, endpoint := range endpoints {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		attempt, cancel := context.WithTimeout(ctx, 12*time.Second)
		var release *updateRelease
		release, err = checkUpdate(attempt, client, endpoint, current, platform, arch)
		cancel()
		if err == nil {
			return release, nil
		}
	}
	return nil, fmt.Errorf("更新线路暂不可用，请稍后重试：%w", err)
}

func downloadUpdatePackage(ctx context.Context, client *http.Client, sources []string, target, digest string, progress func(int64, int64)) error {
	var err error
	for _, source := range sources {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A healthy mirror needs enough time for a full installer on slower links.
		timeout := 5 * time.Minute
		if strings.HasPrefix(source, updateRepository+"/") {
			timeout = 45 * time.Second
		}
		attempt, cancel := context.WithTimeout(ctx, timeout)
		if progress != nil {
			progress(0, 0)
		}
		_, err = installKernel(attempt, client, source, target, digest, progress)
		cancel()
		if err == nil {
			return nil
		}
	}
	return err
}

func updateDownloadSources(officialURL string) []string {
	return []string{"https://gh-proxy.com/" + officialURL, officialURL, "https://ghfast.top/" + officialURL}
}

func updateProxy(request *http.Request) (*url.URL, error) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		if os.Getenv(key) != "" {
			return http.ProxyFromEnvironment(request)
		}
	}
	return systemUpdateProxy(request.URL.Scheme), nil
}

func parseSystemProxy(value, scheme string) *url.URL {
	selected := strings.TrimSpace(value)
	if strings.Contains(value, "=") {
		selected = ""
		for _, entry := range strings.Split(value, ";") {
			parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], scheme) {
				selected = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	if selected == "" {
		return nil
	}
	if !strings.Contains(selected, "://") {
		selected = "http://" + selected
	}
	parsed, err := url.Parse(selected)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5") {
		return nil
	}
	return parsed
}
func checkUpdate(ctx context.Context, client *http.Client, endpoint, current, platform, arch string) (*updateRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("检查更新失败，请检查网络后重试")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, errors.New("暂未发布正式更新，请稍后重试")
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("检查更新失败（HTTP %d），请稍后重试", res.StatusCode)
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Body       string `json:"body"`
		Assets     []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"assets"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&release); err != nil {
		return nil, errors.New("更新信息无法识别")
	}
	if !releaseVersionPattern.MatchString(release.Tag) || release.Draft || release.Prerelease {
		return nil, errors.New("更新版本尚未正式发布")
	}
	if !newerVersion(release.Tag, current) {
		return nil, nil
	}
	version := strings.TrimPrefix(release.Tag, "v")
	name := fmt.Sprintf("ACU-Runtime-%s-%s-%s-Setup.exe", version, platform, arch)
	for _, a := range release.Assets {
		if a.Name != name {
			continue
		}
		expected := updateRepository + "/releases/download/" + release.Tag + "/" + name
		digest := strings.TrimPrefix(a.Digest, "sha256:")
		if platform != "windows" || a.URL != expected || !digestPattern.MatchString(digest) || a.Size <= 0 || a.Size > 128<<20 {
			return nil, errors.New("更新包缺少可信校验信息，未下载")
		}
		notes := release.Body
		if len(notes) > 6000 {
			notes = notes[:6000]
		}
		return &updateRelease{Version: version, URL: a.URL, SHA256: digest, Notes: notes, Page: updateRepository + "/releases/tag/" + release.Tag}, nil
	}
	return nil, errors.New("新版尚未提供适合此设备的安装包，当前版本可继续使用")
}
func (host *server) updates(response http.ResponseWriter, request *http.Request) {
	u := &host.update
	if request.Method == http.MethodGet {
		host.json(response, 200, u.snapshot(host.desktop))
		return
	}
	if !host.desktop || runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		host.json(response, 400, map[string]string{"error": "此平台暂未提供应用内更新安装包"})
		return
	}
	u.mu.Lock()
	if u.view.Phase == "checking" || u.view.Phase == "downloading" || u.view.Phase == "restarting" {
		u.mu.Unlock()
		host.json(response, 409, map[string]string{"error": "更新正在进行，请稍候"})
		return
	}
	action := strings.TrimPrefix(request.URL.Path, "/runtime/update/")
	if action == "install" {
		host.agentInstall.mu.Lock()
		installingAgent := host.agentInstall.running
		host.agentInstall.mu.Unlock()
		if installingAgent {
			u.mu.Unlock()
			host.json(response, 409, map[string]string{"error": "请等待 Agent 安装完成后再更新 Runtime"})
			return
		}
	}
	candidate := u.view.Release
	switch action {
	case "check":
		u.view = updateView{Phase: "checking"}
	case "install":
		if candidate == nil || !newerVersion(candidate.Version, appVersion) {
			u.mu.Unlock()
			host.json(response, 400, map[string]string{"error": "请先检查更新"})
			return
		}
		u.view.Phase = "downloading"
		u.view.Message = ""
		u.view.Downloaded = 0
		u.view.Total = 0
	default:
		u.mu.Unlock()
		http.NotFound(response, request)
		return
	}
	u.mu.Unlock()
	host.json(response, 202, u.snapshot(host.desktop))
	go func() {
		ctx := host.lifecycle
		if ctx == nil {
			ctx = context.Background()
		}
		var err error
		if action == "check" {
			var found *updateRelease
			found, err = checkUpdateWithFallback(ctx, updateClient(15*time.Second), []string{updateAPI, "https://gh-proxy.com/" + updateAPI}, appVersion, runtime.GOOS, runtime.GOARCH)
			u.mu.Lock()
			u.view.Release = found
			if err == nil {
				if found == nil {
					u.view.Phase = "current"
				} else {
					u.view.Phase = "available"
				}
			}
			u.mu.Unlock()
		} else {
			err = host.downloadUpdate(ctx, candidate)
		}
		if err != nil {
			u.mu.Lock()
			u.view.Phase = "error"
			u.view.Message = err.Error()
			u.mu.Unlock()
		}
	}()
}
func (host *server) downloadUpdate(ctx context.Context, release *updateRelease) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	expectedTarget := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")
	if !strings.EqualFold(filepath.Clean(filepath.Dir(executable)), filepath.Clean(expectedTarget)) {
		return errors.New("当前是便携或自定义安装，请先使用官方安装包安装到默认目录")
	}
	directory, err := os.MkdirTemp("", "acu-runtime-update-")
	if err != nil {
		return errors.New("无法创建更新目录")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			os.RemoveAll(directory)
		}
	}()
	installer := filepath.Join(directory, "ACU Runtime Setup.exe")
	err = downloadUpdatePackage(ctx, updateClient(6*time.Minute), updateDownloadSources(release.URL), installer, release.SHA256, func(done, total int64) {
		host.update.mu.Lock()
		host.update.view.Downloaded = done
		host.update.view.Total = total
		host.update.mu.Unlock()
	})
	if err != nil {
		return fmt.Errorf("更新包下载或校验失败，当前安装未变更：%w", err)
	}
	// The verified installer waits for this exact process before touching files.
	command := exec.Command(installer, "--wait-pid", strconv.Itoa(os.Getpid()), "--target", filepath.Dir(executable))
	hideProcess(command)
	if err = command.Start(); err != nil {
		return errors.New("无法启动更新安装程序，当前安装未变更")
	}
	handedOff = true
	host.update.mu.Lock()
	host.update.view.Phase = "restarting"
	host.update.mu.Unlock()
	go command.Wait()
	select {
	case host.quitRequests <- struct{}{}:
	case <-ctx.Done():
	}
	return nil
}
