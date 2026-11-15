package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const tapnowNativeName = "com.acucompute.tapnow"

// Stable unpacked-extension identities from the ZIP manifest public keys.
// These identify callers; they do not authenticate unsigned extension code.
var tapnowCompanionID = "illffggofpdcjmdcenajnbmnokoclabh"
var tapnowBackupID = "lohhkeighnafehcjelhlimegfgpelghg"
var tapnowIDPattern = regexp.MustCompile(`^[a-p]{32}$`)

func tapnowID(edition string) string {
	id := ""
	switch edition {
	case "companion":
		id = tapnowCompanionID
	case "backup":
		id = tapnowBackupID
	}
	if !tapnowIDPattern.MatchString(id) {
		return ""
	}
	return id
}

func tapnowOriginEdition(origin string) string {
	for _, edition := range []string{"companion", "backup"} {
		if id := tapnowID(edition); id != "" && origin == "chrome-extension://"+id+"/" {
			return edition
		}
	}
	return ""
}

func tapnowAPIBase(raw string) (string, error) {
	base, err := acuBase(raw)
	if err != nil {
		return "", errors.New("不支持的 ACU 地址")
	}
	return base + "/v1", nil
}

type tapnowGrant struct {
	ID              string
	Provider        string
	Model           string
	Phase           string
	Version         string
	Error           string
	Expires         time.Time
	Confirmed       time.Time
	RequiredVersion string
	ReloadRequired  bool
}

type tapnowNativeSession struct {
	Origin  string    `json:"origin"`
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

type tapnowNativeMessage struct {
	Action  string `json:"action"`
	Origin  string `json:"origin,omitempty"`
	Grant   string `json:"grant,omitempty"`
	Version string `json:"version"`
	Success bool   `json:"success,omitempty"`
	URL     string `json:"url,omitempty"`
}

type tapnowNativeReply struct {
	OK        bool                    `json:"ok"`
	Confirmed bool                    `json:"confirmed,omitempty"`
	Grant     string                  `json:"grant,omitempty"`
	Error     string                  `json:"error,omitempty"`
	Config    *tapnowCredentialConfig `json:"config,omitempty"`
}

func writeTapnowPrivateJSON(path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("本机连接目录无效")
	}
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("本机连接文件无效")
	}
	file, err := os.CreateTemp(dir, ".native-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(body)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceInstalledPath(file.Name(), path)
}

func (host *server) tapnowConnect(w http.ResponseWriter, r *http.Request, edition, provider, model string) {
	if !tapnowEdition(edition) || tapnowID(edition) == "" {
		host.json(w, 409, map[string]string{"error": "TapNow 插件身份无效，暂不能接入"})
		return
	}
	s := &host.tapnow
	if r.URL.Path == "/runtime/tapnow/cancel" {
		s.Lock()
		if s.preparing == edition && s.cancel != nil {
			s.cancel()
		}
		delete(s.grants, edition)
		s.Unlock()
		host.json(w, 200, s.snapshot())
		return
	}
	if !host.desktop {
		host.json(w, 409, map[string]string{"error": "请在 ACU Runtime 桌面应用中接入"})
		return
	}
	install := r.URL.Path == "/runtime/tapnow/install" || r.URL.Path == "/runtime/tapnow/configure"
	requiredVersion := ""
	reloadRequired := false
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Minute)
	defer cancel()
	s.Lock()
	if s.busy {
		s.Unlock()
		host.json(w, 409, map[string]string{"error": "TapNow 操作正在进行"})
		return
	}
	s.busy = true
	// A failed new configuration must not leave an earlier success visible.
	if install {
		delete(s.grants, edition)
	}
	s.preparing, s.phase, s.message, s.cancel = edition, "checking", "", cancel
	if s.nativeToken == "" {
		s.nativeToken = secret()
	}
	nativeToken := s.nativeToken
	client := s.client
	if client == nil {
		client = tapnowHTTPClient()
	}
	endpoint := s.catalogURL
	if endpoint == "" {
		endpoint = tapnowCatalogURL
	}
	s.Unlock()
	defer func() {
		s.Lock()
		s.busy = false
		s.preparing = ""
		s.phase = ""
		s.cancel = nil
		s.Unlock()
	}()
	fail := func(err error) {
		s.Lock()
		if ctx.Err() != nil {
			s.message = "准备已停止，已准备的插件文件保留"
		} else {
			s.message = err.Error()
		}
		message := s.message
		s.Unlock()
		host.json(w, 400, map[string]string{"error": message})
	}
	if edition == "companion" {
		// Validate before opening Chrome, but discard the credential immediately.
		if _, err := host.tapnowCredential(ctx, provider, model); err != nil {
			fail(err)
			return
		}
	}
	if install {
		body, err := tapnowRead(ctx, client, endpoint, 64<<10)
		var catalog tapnowCatalog
		if err == nil && (json.Unmarshal(body, &catalog) != nil || catalog.Schema != 1 || len(catalog.Packages) != 2 || !validTapnowPackage("companion", catalog.Packages["companion"]) || !validTapnowPackage("backup", catalog.Packages["backup"])) {
			err = errors.New("插件发布清单无效")
		}
		if err == nil {
			root, dirErr := s.directory()
			if dirErr != nil {
				fail(dirErr)
				return
			}
			requiredVersion = catalog.Packages[edition].Version
			reloadRequired = readTapnowManifest(filepath.Join(root, edition)) != requiredVersion
			s.Lock()
			s.phase = "downloading"
			s.Unlock()
			err = s.prepare(ctx, client, edition, catalog.Packages[edition])
		}
		if err != nil {
			fail(err)
			return
		}
		s.Lock()
		s.catalog = &catalog
		s.Unlock()
		root, dirErr := s.directory()
		if dirErr != nil || !tapnowPreparedIdentity(filepath.Join(root, edition), edition) {
			host.json(w, 400, map[string]string{"error": "ZIP 插件身份不匹配，未开启 Key 接入"})
			return
		}
	}
	root, err := s.directory()
	if err == nil && edition == "companion" {
		s.Lock()
		s.phase = "preparing-video"
		s.Unlock()
		if videoErr := host.prepareVideoTools(ctx); videoErr != nil {
			fail(videoErr)
			return
		}
	}
	if requiredVersion == "" && err == nil && tapnowPreparedIdentity(filepath.Join(root, edition), edition) {
		requiredVersion = readTapnowManifest(filepath.Join(root, edition))
	}
	if ctx.Err() != nil {
		fail(ctx.Err())
		return
	}
	s.Lock()
	s.phase = "registering"
	s.Unlock()
	if err == nil {
		err = writeTapnowPrivateJSON(filepath.Join(root, "native-session.json"), tapnowNativeSession{"http://" + host.host, nativeToken, time.Now().Add(10 * time.Minute)})
	}
	if err == nil {
		if s.register != nil {
			err = s.register()
		} else {
			err = registerTapnowHost()
		}
	}
	if err != nil {
		host.json(w, 400, map[string]string{"error": "本机连接组件未准备完成，请重试"})
		return
	}
	grant := &tapnowGrant{ID: secret(), Provider: provider, Model: model, Phase: "waiting", RequiredVersion: requiredVersion, ReloadRequired: reloadRequired, Expires: time.Now().Add(5 * time.Minute)}
	s.Lock()
	if ctx.Err() != nil {
		s.Unlock()
		fail(ctx.Err())
		return
	}
	if s.grants == nil {
		s.grants = map[string]*tapnowGrant{}
	}
	s.grants[edition] = grant
	s.Unlock()
	address := "chrome-extension://" + tapnowID(edition) + "/connect.html"
	if install && reloadRequired {
		// Chrome filters privileged internal URLs passed by external launchers.
		// Launch a normal page and let the guide direct address-bar navigation.
		address = "about:blank"
		s.Lock()
		s.message = "插件文件已准备。请在 Chrome 地址栏输入 chrome://extensions/ 打开扩展页"
		s.Unlock()
		if s.open != nil {
			err = s.open(ctx, filepath.Join(root, edition))
		} else {
			err = openTapnowDirectory(ctx, filepath.Join(root, edition))
		}
		if err != nil {
			s.Lock()
			s.message = "插件已准备，但目录未能打开；可复制下方目录在 Chrome 中选择"
			s.Unlock()
		}
	}
	if ctx.Err() != nil {
		fail(ctx.Err())
		return
	}
	if s.openStore != nil {
		err = s.openStore(ctx, address)
	} else {
		err = openTapnowChrome(ctx, address)
	}
	if err != nil {
		s.Lock()
		if s.grants[edition] == grant {
			delete(s.grants, edition)
		}
		s.Unlock()
		host.json(w, 400, map[string]string{"error": "无法打开 Google Chrome，请先安装 Chrome 后重试；未发送 Key"})
		return
	}
	if ctx.Err() != nil {
		s.Lock()
		if s.grants[edition] == grant {
			delete(s.grants, edition)
		}
		s.Unlock()
		fail(ctx.Err())
		return
	}
	view := s.snapshot()
	view.Busy = false
	view.Preparing, view.Phase = "", ""
	host.json(w, 200, view)
}

func tapnowPreparedIdentity(dir, edition string) bool {
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var manifest struct {
		Key string `json:"key"`
	}
	if err != nil || json.Unmarshal(body, &manifest) != nil {
		return false
	}
	key, err := base64.StdEncoding.DecodeString(manifest.Key)
	if err != nil || len(key) < 32 {
		return false
	}
	digest := sha256.Sum256(key)
	id := make([]byte, 32)
	for i, b := range digest[:16] {
		id[2*i] = 'a' + (b >> 4)
		id[2*i+1] = 'a' + (b & 15)
	}
	return string(id) == tapnowID(edition)
}

func (host *server) tapnowNativeAPI(w http.ResponseWriter, r *http.Request) {
	s := &host.tapnow
	s.Lock()
	token := s.nativeToken
	s.Unlock()
	if r.Method != "POST" || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(r.Header.Get("X-ACU-Tapnow-Token"))) != 1 || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		host.json(w, 403, tapnowNativeReply{Error: "invalid native session"})
		return
	}
	var msg tapnowNativeMessage
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if dec.Decode(&msg) != nil || dec.Decode(&struct{}{}) != io.EOF || !releaseVersionPattern.MatchString(msg.Version) {
		host.json(w, 400, tapnowNativeReply{Error: "invalid native message"})
		return
	}
	edition := tapnowOriginEdition(msg.Origin)
	if msg.Action == "video-frames" {
		if edition != "companion" {
			host.json(w, 403, tapnowVideoReply{Error: "备份插件不支持视频审阅"})
			return
		}
		host.tapnowVideoFrames(w, r, msg)
		return
	}
	s.Lock()
	g := s.grants[edition]
	if edition == "" || g == nil || time.Now().After(g.Expires) {
		s.Unlock()
		host.json(w, 409, tapnowNativeReply{Error: "请先在 ACU Runtime 点击接入；授权五分钟内有效"})
		return
	}
	if g.RequiredVersion != "" && msg.Version != g.RequiredVersion {
		s.Unlock()
		host.json(w, 409, tapnowNativeReply{Error: "请先在 Chrome 加载或重新加载插件至 " + g.RequiredVersion + "，再继续配置 API Key"})
		return
	}
	if msg.Action == "ack" {
		if g.ID == msg.Grant && g.Phase == "configured" && msg.Success && g.Version == msg.Version {
			s.Unlock()
			host.json(w, 200, tapnowNativeReply{OK: true})
			return
		}
		if g.ID != msg.Grant || g.Phase != "saving" {
			s.Unlock()
			host.json(w, 409, tapnowNativeReply{Error: "接入回执已失效"})
			return
		}
		if g.Version != msg.Version {
			s.Unlock()
			host.json(w, 409, tapnowNativeReply{Error: "插件版本已改变，请重新接入"})
			return
		}
		if msg.Success {
			g.Phase = "configured"
			g.ReloadRequired = false
			g.Confirmed = time.Now()
			g.Error = ""
		} else {
			g.Phase = "error"
			g.Error = "插件配置保存失败，请重新接入"
		}
		s.Unlock()
		host.json(w, 200, tapnowNativeReply{OK: true})
		return
	}
	if msg.Action == "connect" && msg.Grant == "" && g.Phase == "configured" && g.Version == msg.Version {
		s.Unlock()
		host.json(w, 200, tapnowNativeReply{OK: true, Confirmed: true, Grant: g.ID})
		return
	}
	if msg.Action != "connect" || msg.Grant != "" || g.Phase != "waiting" {
		s.Unlock()
		host.json(w, 409, tapnowNativeReply{Error: "没有可领取的接入授权"})
		return
	}
	g.Phase = "saving"
	g.Version = msg.Version
	provider, model, id := g.Provider, g.Model, g.ID
	s.Unlock()
	var config *tapnowCredentialConfig
	var err error
	if edition == "companion" {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		config, err = host.tapnowCredential(ctx, provider, model)
	}
	s.Lock()
	defer s.Unlock()
	if s.grants[edition] != g || time.Now().After(g.Expires) || g.Phase != "saving" {
		host.json(w, 409, tapnowNativeReply{Error: "接入授权已取消或过期"})
		return
	}
	if err != nil {
		g.Phase = "error"
		g.Error = "所选 Key 或模型不可用，请重新接入"
		host.json(w, 400, tapnowNativeReply{Error: g.Error})
		return
	}
	host.json(w, 200, tapnowNativeReply{OK: true, Grant: id, Config: config})
}

func readNativeFrame(in io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(in, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 || length > 16<<10 {
		return nil, errors.New("invalid native frame")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(in, body)
	return body, err
}

func writeNativeFrame(out io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(body) > 900<<10 {
		return errors.New("native reply too large")
	}
	if err = binary.Write(out, binary.LittleEndian, uint32(len(body))); err != nil {
		return err
	}
	_, err = out.Write(body)
	return err
}

// Chrome starts the same executable with its extension origin as the first
// argument. This branch runs before desktop startup and never logs credentials.
func runTapnowNative(origin string, in io.Reader, out io.Writer) error {
	fail := func(message string) error { return writeNativeFrame(out, tapnowNativeReply{Error: message}) }
	if tapnowOriginEdition(origin) == "" {
		return fail("untrusted extension origin")
	}
	body, err := readNativeFrame(in)
	if err != nil {
		return fail("invalid native frame")
	}
	var msg tapnowNativeMessage
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if dec.Decode(&msg) != nil || dec.Decode(&struct{}{}) != io.EOF || msg.Origin != "" || !releaseVersionPattern.MatchString(msg.Version) || (msg.Action != "connect" && msg.Action != "ack" && msg.Action != "video-frames") {
		return fail("invalid native message")
	}
	root, err := (&tapnowState{}).directory()
	if err != nil {
		return fail("请先打开 ACU Runtime 并点击接入")
	}
	file := filepath.Join(root, "native-session.json")
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return fail("请先打开 ACU Runtime 并点击接入")
	}
	payload, err := os.ReadFile(file)
	var session tapnowNativeSession
	if err != nil || json.Unmarshal(payload, &session) != nil || (msg.Action != "video-frames" && !time.Now().Before(session.Expires)) || session.Expires.After(time.Now().Add(11*time.Minute)) || !digestPattern.MatchString(session.Token) {
		return fail("本机授权已过期，请重新接入")
	}
	u, err := url.Parse(session.Origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fail("invalid local service")
	}
	msg.Origin = origin
	body, _ = json.Marshal(msg)
	timeout := 30 * time.Second
	if msg.Action == "video-frames" {
		timeout = 100 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", session.Origin+"/runtime/tapnow/native", bytes.NewReader(body))
	if err != nil {
		return fail("invalid local service")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ACU-Tapnow-Token", session.Token)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	res, err := client.Do(req)
	if err != nil {
		return fail("ACU Runtime 暂不可连接，请重新接入")
	}
	defer res.Body.Close()
	var reply json.RawMessage
	if json.NewDecoder(io.LimitReader(res.Body, 900<<10)).Decode(&reply) != nil {
		return fail("invalid native reply")
	}
	return writeNativeFrame(out, reply)
}

func tapnowHostManifest(executable string) map[string]any {
	origins := []string{}
	for _, edition := range []string{"companion", "backup"} {
		if id := tapnowID(edition); id != "" {
			origins = append(origins, "chrome-extension://"+id+"/")
		}
	}
	return map[string]any{"name": tapnowNativeName, "description": "ACU Runtime TapNow connection", "path": executable, "type": "stdio", "allowed_origins": origins}
}

func checkTapnowRelease() error {
	for _, edition := range []string{"companion", "backup"} {
		if tapnowID(edition) == "" {
			return fmt.Errorf("%s has no fixed ZIP extension identity", edition)
		}
	}
	if tapnowID("companion") == tapnowID("backup") {
		return errors.New("plugin ZIP identities must differ")
	}
	return nil
}

func isTapnowNativeInvocation(args []string) bool {
	return len(args) > 1 && strings.HasPrefix(args[1], "chrome-extension://")
}
