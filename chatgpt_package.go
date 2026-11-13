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
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// OpenAI's documented complete packages, independent of Microsoft Store's
// bootstrapper. These URLs follow latest; Windows verifies Store signature,
// publisher, identity and architecture before deployment, not a mutable hash.
func chatGPTPackage(platform, arch string) (agentPackage, error) {
	name := ""
	switch platform + "/" + arch {
	case "windows/amd64":
		name = "ChatGPT-x64.msix"
	case "windows/arm64":
		name = "ChatGPT-arm64.msix"
	case "darwin/arm64", "darwin/amd64":
		name = "ChatGPT.dmg"
	default:
		return agentPackage{}, errors.New("此平台暂不支持 ChatGPT 桌面安装")
	}
	return agentPackage{URL: "https://persistent.oaistatic.com/codex-app-prod/" + name, Name: name}, nil
}

func trustedChatGPTURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Host == "codexapp.agentsmirror.com" && u.RawQuery == "" {
		return u.Path == "/latest/manifest" || u.Path == "/latest/win-x64" || u.Path == "/latest/win-arm64"
	}
	if u.Host == "fgws3-ocloud.ihep.ac.cn" {
		return u.Path == "/20830-codex/latest/manifest" || u.Path == "/20830-codex/latest/win-x64" || u.Path == "/20830-codex/latest/win-arm64"
	}
	if u.Host != "persistent.oaistatic.com" || u.RawQuery != "" {
		return false
	}
	if u.Path == "/codex-app-prod/windows-store-update.json" {
		return true
	}
	return u.Path == "/codex-app-prod/ChatGPT-x64.msix" || u.Path == "/codex-app-prod/ChatGPT-arm64.msix" || u.Path == "/codex-app-prod/ChatGPT.dmg"
}

func chatGPTDownloadClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = updateProxy
	t.ResponseHeaderTimeout = 20 * time.Second
	return &http.Client{Transport: t, Timeout: 40 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || !trustedChatGPTURL(r.URL.String()) {
			return errors.New("ChatGPT 下载重定向地址不受信任")
		}
		return nil
	}}
}

func agentPackageCache() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "ACURuntime", "agent-packages")
	return dir, os.MkdirAll(dir, 0700)
}

// A completed file is only a cache candidate: the platform installer MUST
// verify its signature again, including on offline reuse. A failed or partial
// download never replaces a previously completed package.
func downloadChatGPTPackage(ctx context.Context, client *http.Client, source, target string, progress func(int64, int64)) error {
	if !trustedChatGPTURL(source) {
		return errors.New("ChatGPT 下载地址不受信任")
	}
	const limit = int64(2 << 30)
	partial, metadata := target+".partial", target+".download.json"
	type downloadIdentity struct {
		Source   string
		ETag     string
		Size     int64
		Complete bool
	}
	var saved downloadIdentity
	if data, err := os.ReadFile(metadata); err == nil {
		_ = json.Unmarshal(data, &saved)
	}
	// A strong ETag identifies one immutable representation behind the latest
	// URL. If it changes, restart rather than concatenate different releases.
	probe, _ := http.NewRequestWithContext(ctx, "HEAD", source, nil)
	probe.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	var current downloadIdentity
	if response, err := client.Do(probe); err == nil {
		if response.StatusCode == 200 {
			current = downloadIdentity{Source: source, ETag: response.Header.Get("ETag"), Size: response.ContentLength}
		}
		response.Body.Close()
	}
	strong := func(tag string) bool { return strings.HasPrefix(tag, `"`) && strings.HasSuffix(tag, `"`) }
	if current.Source != "" && strong(current.ETag) && saved.Source == source && saved.ETag == current.ETag && saved.Size == current.Size && saved.Complete {
		if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() && info.Size() == saved.Size {
			if progress != nil {
				progress(saved.Size, saved.Size)
			}
			return nil // Caller still checks the platform signature.
		}
	}
	var offset int64
	if !saved.Complete && saved.Source == source && strong(saved.ETag) && (current.Source == "" || (current.ETag == saved.ETag && current.Size == saved.Size)) {
		if info, err := os.Lstat(partial); err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() < saved.Size {
			offset = info.Size()
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		req.Header.Set("If-Range", saved.ETag)
	}
	r, err := client.Do(req)
	if err != nil {
		return errors.New("ChatGPT 官方下载暂不可达，请重试或选择 WorkBuddy")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 && r.StatusCode != 206 {
		return fmt.Errorf("ChatGPT 官方下载失败（HTTP %d）", r.StatusCode)
	}
	total := r.ContentLength
	if r.StatusCode == 206 {
		var start, end int64
		if offset == 0 || r.Header.Get("ETag") != saved.ETag {
			return errors.New("ChatGPT 续传版本已变化，请重试")
		}
		if n, _ := fmt.Sscanf(r.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); n != 3 || start != offset || end != total-1 || total != saved.Size || r.ContentLength != total-offset {
			return errors.New("ChatGPT 续传数据无效，请重试")
		}
	} else {
		offset = 0
	}
	if total > limit || total == 0 {
		return errors.New("ChatGPT 安装包大小无效")
	}
	if info, err := os.Lstat(partial); err == nil && !info.Mode().IsRegular() {
		return errors.New("ChatGPT 缓存路径无效")
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if offset > 0 {
		flags = os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(partial, flags, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	identity := downloadIdentity{Source: source, ETag: r.Header.Get("ETag"), Size: total}
	resume := strong(identity.ETag) && total > 0
	if resume {
		encoded, _ := json.Marshal(identity)
		if err := os.WriteFile(metadata, encoded, 0600); err != nil {
			return err
		}
	}
	w := &kernelDownloadWriter{writer: f, written: offset, total: total, progress: progress}
	if progress != nil {
		progress(offset, total)
	}
	n, err := io.Copy(w, io.LimitReader(r.Body, limit-offset+1))
	if err != nil || n == 0 || n+offset > limit || (total >= 0 && n+offset != total) {
		if !resume || n+offset > limit {
			f.Close()
			_ = os.Remove(partial)
			_ = os.Remove(metadata)
			return errors.New("ChatGPT 下载未完成，请重试")
		}
		return errors.New("ChatGPT 下载未完成，已保留可续传进度，请重试")
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Windows Rename cannot replace an existing file. Keep the old candidate
	// until the complete new download exists; neither is executable yet.
	if regularFile(target) {
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	if err := os.Rename(partial, target); err != nil {
		return err
	}
	if resume {
		identity.Complete = true
		encoded, _ := json.Marshal(identity)
		_ = os.WriteFile(metadata, encoded, 0600)
	} else {
		_ = os.Remove(metadata)
	}
	return nil
}

func (host *server) prepareChatGPT(ctx context.Context, update bool) error {
	pkg, err := chatGPTPackage(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	dir, err := agentPackageCache()
	if err != nil {
		return err
	}
	target := filepath.Join(dir, pkg.Name)
	expected := ""
	if update && runtime.GOOS == "windows" {
		expected = host.desktopAgentVersions(ctx)["chatgpt"].Latest
	}
	progress := func(done, total int64) {
		s := &host.agentInstall
		s.mu.Lock()
		s.view.Downloaded, s.view.Total = done, total
		s.mu.Unlock()
	}
	if !update {
		// An optional offline distribution can place unchanged vendor packages
		// alongside Runtime. The same publisher/identity checks still apply.
		candidates := []string{}
		if executable, err := os.Executable(); err == nil {
			candidates = append([]string{filepath.Join(filepath.Dir(executable), "agent-packages", pkg.Name)}, candidates...)
		}
		for _, path := range candidates {
			if regularFile(path) && verifyChatGPTPackage(ctx, path) == nil {
				host.agentInstall.phase("installing", "正在使用已验证的离线 ChatGPT 安装包；如有系统确认，请按提示完成")
				return installChatGPT(ctx, path)
			}
		}
	}
	host.agentInstall.phase("checking", "正在检查 ChatGPT 最新版本和下载线路")
	if err := prepareLatestChatGPT(ctx, runtime.GOOS, runtime.GOARCH, target, progress, func(message string) { host.agentInstall.phase("downloading", message) }, expected); err != nil {
		return err
	}
	if expected != "" {
		if err := requireChatGPTTarget(target, expected); err != nil {
			return err
		}
	}
	host.agentInstall.phase("checking", "正在验证 ChatGPT 官方签名和应用身份")
	if err := verifyChatGPTPackage(ctx, target); err != nil {
		// A damaged cache must not keep satisfying an unchanged ETag on retry.
		_ = os.Remove(target + ".download.json")
		return err
	}
	host.agentInstall.phase("installing", "正在安装 ChatGPT；Windows 可能要求管理员确认，请先保存并关闭已有 ChatGPT 窗口")
	return installChatGPT(ctx, target)
}
