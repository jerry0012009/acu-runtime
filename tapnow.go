package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const tapnowCatalogURL = "https://raw.githubusercontent.com/jerry0012009/acu-tapnow/main/runtime-channel.json"
const tapnowRepository = "https://github.com/jerry0012009/acu-tapnow"

type tapnowPackage struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type tapnowCatalog struct {
	Schema   int                      `json:"schema"`
	Packages map[string]tapnowPackage `json:"packages"`
}

type tapnowEditionView struct {
	Edition          string `json:"edition"`
	Prepared         string `json:"prepared,omitempty"`
	Latest           string `json:"latest,omitempty"`
	Path             string `json:"path,omitempty"`
	Update           bool   `json:"update"`
	InstallAvailable bool   `json:"installAvailable"`
	Connection       string `json:"connection"`
	Version          string `json:"version,omitempty"`
	ConfirmedAt      string `json:"confirmedAt,omitempty"`
	ConnectionError  string `json:"connectionError,omitempty"`
}

type tapnowView struct {
	Editions  []tapnowEditionView `json:"editions"`
	Checked   bool                `json:"checked"`
	Busy      bool                `json:"busy"`
	Message   string              `json:"message,omitempty"`
	Preparing string              `json:"preparing,omitempty"`
	Phase     string              `json:"phase,omitempty"`
}

type tapnowState struct {
	sync.Mutex
	catalog   *tapnowCatalog
	busy      bool
	message   string
	preparing string
	phase     string
	cancel    context.CancelFunc
	// Test overrides never come from an HTTP request.
	root        string
	client      *http.Client
	catalogURL  string
	open        func(context.Context, string) error
	grants      map[string]*tapnowGrant
	nativeToken string
	register    func() error
	openStore   func(context.Context, string) error
}

func (s *tapnowState) directory() (string, error) {
	if s.root != "" {
		return s.root, nil
	}
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "ACURuntime", "tapnow"), err
}

func tapnowEdition(edition string) bool { return edition == "backup" || edition == "companion" }

func validTapnowPackage(edition string, p tapnowPackage) bool {
	if !tapnowEdition(edition) || !releaseVersionPattern.MatchString(p.Version) || strings.HasPrefix(p.Version, "v") || !digestPattern.MatchString(p.SHA256) || p.Size < 1 || p.Size > 32<<20 {
		return false
	}
	name := "tapnow-" + edition + "-" + p.Version + "-chrome.zip"
	return p.URL == tapnowRepository+"/releases/download/v"+p.Version+"/"+name
}

func tapnowHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = updateProxy
	transport.ResponseHeaderTimeout = 15 * time.Second
	return &http.Client{Transport: transport, Timeout: 2 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		u := r.URL
		if len(via) > 5 || u.Scheme != "https" || u.User != nil || u.Port() != "" {
			return errors.New("插件下载地址不受信任")
		}
		switch u.Hostname() {
		case "raw.githubusercontent.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
		return errors.New("插件下载地址不受信任")
	}}
}

func tapnowRead(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("TapNow 下载暂不可用，请稍后重试")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("TapNow 下载失败（HTTP %d）", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, errors.New("插件下载不完整或超出大小限制")
	}
	return body, nil
}

func readTapnowManifest(dir string) string {
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var m struct {
		Version         string `json:"version"`
		ManifestVersion int    `json:"manifest_version"`
	}
	if err != nil || json.Unmarshal(body, &m) != nil || m.ManifestVersion != 3 || !releaseVersionPattern.MatchString(m.Version) {
		return ""
	}
	return m.Version
}

func (s *tapnowState) snapshot() tapnowView {
	s.Lock()
	defer s.Unlock()
	v := tapnowView{Checked: s.catalog != nil, Busy: s.busy, Message: s.message, Preparing: s.preparing, Phase: s.phase, Editions: []tapnowEditionView{}}
	root, err := s.directory()
	for _, edition := range []string{"companion", "backup"} {
		e := tapnowEditionView{Edition: edition}
		e.InstallAvailable = tapnowID(edition) != ""
		e.Connection = "unknown"
		if g := s.grants[edition]; g != nil {
			e.Connection, e.Version, e.ConnectionError = g.Phase, g.Version, g.Error
			if !g.Confirmed.IsZero() {
				e.ConfirmedAt = g.Confirmed.UTC().Format(time.RFC3339)
			}
			if time.Now().After(g.Expires) && (g.Phase == "waiting" || g.Phase == "saving") {
				e.Connection = "expired"
			}
		}
		if err == nil {
			e.Path = filepath.Join(root, edition)
			e.Prepared = readTapnowManifest(e.Path)
		}
		if s.catalog != nil {
			e.Latest = s.catalog.Packages[edition].Version
			e.Update = e.Prepared != "" && newerVersion(e.Latest, e.Prepared)
		}
		v.Editions = append(v.Editions, e)
	}
	return v
}

// Extract only regular files into a new staging directory. Never merge a ZIP
// into a live extension, browser profile, or user-selected backup directory.
func unpackTapnow(body []byte, dir, version string) error {
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil || len(z.File) > 2048 {
		return errors.New("插件压缩包无效")
	}
	seen := map[string]bool{}
	var expanded uint64
	for _, f := range z.File {
		name := f.Name
		if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
			return errors.New("插件包含不安全路径")
		}
		for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
			stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
			if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part || strings.ContainsAny(part, "<>\"|?*") || stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9') {
				return errors.New("插件包含不安全路径")
			}
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || seen[strings.ToLower(clean)] {
			return errors.New("插件包含不安全或重复路径")
		}
		seen[strings.ToLower(clean)] = true
		if f.Mode().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() || f.UncompressedSize64 > 32<<20 {
			return errors.New("插件包含不支持的文件")
		}
		expanded += f.UncompressedSize64
		if expanded > 64<<20 {
			return errors.New("插件解压大小超出限制")
		}
		target := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := extractTapnowFile(f, target); err != nil {
			return err
		}
	}
	if readTapnowManifest(dir) != version {
		return errors.New("插件版本与发布清单不符")
	}
	return nil
}

func extractTapnowFile(f *zip.File, target string) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(in, 32<<20+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (s *tapnowState) prepare(ctx context.Context, client *http.Client, edition string, p tapnowPackage) error {
	if !validTapnowPackage(edition, p) {
		return errors.New("插件清单缺少可信校验信息")
	}
	root, err := s.directory()
	if err != nil {
		return err
	}
	target := filepath.Join(root, edition)
	current := readTapnowManifest(target)
	if newerVersion(current, p.Version) {
		return errors.New("本地插件版本较新，未降级")
	}
	body, err := tapnowRead(ctx, client, p.URL, p.Size)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	if int64(len(body)) != p.Size || hex.EncodeToString(digest[:]) != p.SHA256 {
		return errors.New("插件 SHA-256 校验失败，原版本未变更")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("插件根目录不是普通目录")
	}
	staging, err := os.MkdirTemp(root, ".prepare-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := unpackTapnow(body, staging, p.Version); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	previous := target + ".previous"
	// Refuse symlink targets; these paths are owned only by Runtime.
	for _, path := range []string{target, previous} {
		if info, err := os.Lstat(path); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return errors.New("插件目录不是 Runtime 管理的普通目录")
		}
	}
	if err := os.RemoveAll(previous); err != nil {
		return err
	}
	hadPrevious := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, previous); err != nil {
			return errors.New("请关闭插件页面后重试更新")
		}
		hadPrevious = true
	}
	if err := os.Rename(staging, target); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(previous, target); restoreErr != nil {
				return fmt.Errorf("更新未完成，原版本保留于 %s", previous)
			}
		}
		return err
	}
	// A rename interrupted by process termination can leave .previous available
	// for manual recovery. Never claim Chrome has reloaded these new files.
	return nil
}

func (host *server) tapnowAPI(w http.ResponseWriter, r *http.Request) {
	s := &host.tapnow
	if r.Method == http.MethodGet && r.URL.Path == "/runtime/tapnow" {
		host.json(w, 200, s.snapshot())
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var input struct {
		Edition  string `json:"edition"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(&struct{}{}) != io.EOF {
		host.json(w, 400, map[string]string{"error": "无法读取 TapNow 操作"})
		return
	}
	if r.URL.Path == "/runtime/tapnow/config" {
		host.tapnowConfig(w, r, input.Provider, input.Model)
		return
	}
	if r.URL.Path == "/runtime/tapnow/connect" || r.URL.Path == "/runtime/tapnow/install" || r.URL.Path == "/runtime/tapnow/cancel" {
		host.tapnowConnect(w, r, input.Edition, input.Provider, input.Model)
		return
	}
	if r.URL.Path == "/runtime/tapnow/browser" && tapnowEdition(input.Edition) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var err error
		if !host.desktop {
			err = errors.New("请在桌面应用中打开 Chrome")
		} else if s.openStore != nil {
			err = s.openStore(ctx, "chrome://extensions/")
		} else {
			err = openTapnowChrome(ctx, "chrome://extensions/")
		}
		if err != nil {
			host.json(w, 400, map[string]string{"error": "无法打开 Chrome，请在浏览器地址栏打开 chrome://extensions/"})
			return
		}
		host.json(w, 200, s.snapshot())
		return
	}
	if r.URL.Path != "/runtime/tapnow/check" && r.URL.Path != "/runtime/tapnow/prepare" && r.URL.Path != "/runtime/tapnow/open" {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path != "/runtime/tapnow/check" && !tapnowEdition(input.Edition) {
		host.json(w, 400, map[string]string{"error": "请选择备份或副驾驶插件"})
		return
	}
	s.Lock()
	if s.busy {
		s.Unlock()
		host.json(w, 409, map[string]string{"error": "TapNow 操作正在进行"})
		return
	}
	s.busy = true
	s.message = ""
	client := s.client
	if client == nil {
		client = tapnowHTTPClient()
	}
	endpoint := s.catalogURL
	if endpoint == "" {
		endpoint = tapnowCatalogURL
	}
	catalog := s.catalog
	s.Unlock()
	defer func() { s.Lock(); s.busy = false; s.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	var err error
	switch r.URL.Path {
	case "/runtime/tapnow/check":
		var body []byte
		body, err = tapnowRead(ctx, client, endpoint, 64<<10)
		if err == nil {
			var c tapnowCatalog
			if json.Unmarshal(body, &c) != nil || c.Schema != 1 || len(c.Packages) != 2 || !validTapnowPackage("backup", c.Packages["backup"]) || !validTapnowPackage("companion", c.Packages["companion"]) {
				err = errors.New("TapNow 发布清单无效，原版本未变更")
			} else {
				s.Lock()
				s.catalog = &c
				s.Unlock()
			}
		}
	case "/runtime/tapnow/prepare":
		if catalog == nil {
			err = errors.New("请先检查插件版本")
		} else {
			err = s.prepare(ctx, client, input.Edition, catalog.Packages[input.Edition])
		}
	case "/runtime/tapnow/open":
		root, dirErr := s.directory()
		err = dirErr
		if err == nil {
			path := filepath.Join(root, input.Edition)
			if readTapnowManifest(path) == "" {
				err = errors.New("请先准备插件")
			} else if s.open != nil {
				err = s.open(ctx, path)
			} else if host.desktop {
				err = openTapnowDirectory(ctx, path)
			} else {
				err = errors.New("开发预览中请使用所示本地目录")
			}
		}
	}
	s.Lock()
	if err != nil {
		s.message = err.Error()
	}
	s.Unlock()
	if err != nil {
		host.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	view := s.snapshot()
	view.Busy = false
	host.json(w, 200, view)
}

// Credentials are exported only after an explicit, session-protected POST.
// They never enter URLs, status responses, release archives, or browser profiles.
func (host *server) tapnowConfig(w http.ResponseWriter, r *http.Request, provider, model string) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	config, err := host.tapnowCredential(ctx, provider, model)
	if err != nil {
		host.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="acu-tapnow-config.json"`)
	host.json(w, 200, config)
}

type tapnowConfigSettings struct {
	Base     string `json:"llmBaseUrl"`
	Protocol string `json:"llmProtocol"`
	Model    string `json:"llmModel"`
}
type tapnowCredentialConfig struct {
	Kind     string               `json:"kind"`
	Schema   int                  `json:"schema"`
	APIKey   string               `json:"apiKey"`
	Settings tapnowConfigSettings `json:"settings"`
}

func (host *server) tapnowCredential(ctx context.Context, provider, model string) (*tapnowCredentialConfig, error) {
	if provider == "" || model == "" {
		return nil, errors.New("请选择 ACU Key 和模型")
	}
	if err := host.checkProvider(ctx, provider); err != nil {
		return nil, err
	}
	var data struct {
		Providers []localProvider `json:"providers"`
	}
	if host.kernel.exchange(ctx, "GET", "/api/providers", nil, &data) != nil {
		return nil, errors.New("无法读取 ACU 来源")
	}
	base, protocol := "", "responses"
	for _, p := range data.Providers {
		if p.ID != provider || p.Off || !p.Key.Set {
			continue
		}
		available := false
		for _, m := range p.Models {
			if m.ID == model && m.On {
				available = true
			}
		}
		if !available {
			continue
		}
		for _, endpoint := range []struct{ url, protocol string }{{p.Responses, "responses"}, {p.Chat, "chat_completions"}} {
			if b, err := tapnowAPIBase(endpoint.url); err == nil {
				base = b
				protocol = endpoint.protocol
				break
			}
		}
	}
	if base == "" {
		return nil, errors.New("此插件暂不支持所选 ACU 地址或模型，请选择可用来源")
	}
	var credential struct {
		Key string `json:"key"`
	}
	if host.kernel.exchange(ctx, "POST", "/api/provider/key", map[string]string{"id": provider}, &credential) != nil || strings.TrimSpace(credential.Key) == "" {
		return nil, errors.New("无法读取所选 Key，未发送配置")
	}
	return &tapnowCredentialConfig{Kind: "acu-tapnow-companion", Schema: 1, APIKey: credential.Key, Settings: tapnowConfigSettings{base, protocol, model}}, nil
}
