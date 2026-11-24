package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	bestNetworkBase      = "https://acu.best"
	gaNetworkBase        = "https://console.acucompute.com"
	networkProviderID    = "acu-runtime"
	networkProbeTimeout  = 25 * time.Second
	networkVerifyTimeout = 12 * time.Second
	networkCheckTimeout  = networkProbeTimeout + networkVerifyTimeout + 5*time.Second
)

type networkRoute struct {
	Base  string `json:"base"`
	Proxy string `json:"proxy"`
}

type networkView struct {
	Mode     string       `json:"mode"`
	Route    networkRoute `json:"route"`
	Checked  bool         `json:"checked"`
	Checking bool         `json:"checking"`
	Managed  bool         `json:"managed"`
	Message  string       `json:"message,omitempty"`
}

type networkPrefs struct {
	Mode    string       `json:"mode"`
	Applied networkRoute `json:"applied"`
}

var uploadProbeBody = []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"OK"}],"padding":"` + strings.Repeat("0", 256<<10) + `"}`)

const networkUnverified = "线路未通过验证，已保留当前连接"

func supportedNetworkBase(base string) bool { return base == bestNetworkBase || base == gaNetworkBase }

func (host *server) configuredProvider(ctx context.Context) (localProvider, error) {
	var listed struct {
		Providers []localProvider `json:"providers"`
	}
	if err := host.kernel.exchange(ctx, http.MethodGet, "/api/providers", nil, &listed); err != nil {
		return localProvider{}, err
	}
	for _, p := range listed.Providers {
		if p.ID == networkProviderID {
			return p, nil
		}
	}
	return localProvider{}, nil
}

func (host *server) configuredNetworkKey(ctx context.Context) (string, error) {
	var result struct {
		Key string `json:"key"`
	}
	err := host.kernel.exchange(ctx, http.MethodPost, "/api/provider/key", map[string]string{"id": networkProviderID}, &result)
	if err != nil || !validConnectionKey(result.Key) {
		return "", errors.New("无法核对当前连接凭据")
	}
	return result.Key, nil
}

func (host *server) networkPrefs() networkPrefs {
	data, err := readNetworkFile(host.networkFile, 4096)
	if err == nil {
		var prefs networkPrefs
		if json.Unmarshal(data, &prefs) == nil && (prefs.Mode == "auto" || prefs.Mode == "manual") {
			return prefs
		}
	}
	return networkPrefs{}
}

func (host *server) saveNetworkPrefs(prefs networkPrefs) error {
	if host.networkFile == "" {
		return errors.New("网络设置暂不可用")
	}
	if err := os.MkdirAll(filepath.Dir(host.networkFile), 0700); err != nil {
		return errors.New("无法保存网络设置")
	}
	data, _ := json.Marshal(prefs)
	if err := writeNetworkFile(host.networkFile, data, nil); err != nil {
		return errors.New("无法保存网络设置")
	}
	return nil
}

// The short, authenticated upload uses the existing token-count endpoint.
// Its controller reads the whole synthetic body and does no model execution.
func probeNetwork(ctx context.Context, route networkRoute, key string, proxy *url.URL) (time.Duration, bool) {
	if !supportedNetworkBase(route.Base) || !validConnectionKey(key) {
		return 0, false
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if route.Proxy == "" && proxy != nil {
		transport.Proxy = http.ProxyURL(proxy)
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, route.Base+"/v1/messages/count_tokens", bytes.NewReader(uploadProbeBody))
	if err != nil {
		return 0, false
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("anthropic-version", "2023-06-01")
	begin := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return 0, false
	}
	defer response.Body.Close()
	var result struct {
		InputTokens int `json:"input_tokens"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil || result.InputTokens <= 0 {
		return 0, false
	}
	return time.Since(begin), true
}

type networkCandidate struct {
	route networkRoute
	proxy *url.URL
}

type networkSample struct {
	route    networkRoute
	duration time.Duration
	ok       bool
}

func chooseNetwork(samples []networkSample, current networkRoute) (networkRoute, bool) {
	var best, incumbent networkSample
	for _, sample := range samples {
		if !sample.ok {
			continue
		}
		if sample.route == current {
			incumbent = sample
		}
		if !best.ok || sample.duration < best.duration || (sample.duration == best.duration && sample.route == current) {
			best = sample
		}
	}
	if !best.ok {
		return current, false
	}
	// Within the timing noise of a short upload, keep a working current path.
	if incumbent.ok && (incumbent.duration-best.duration < 250*time.Millisecond || best.duration*5 > incumbent.duration*4) {
		return current, true
	}
	return best.route, true
}

type networkProxyStatus struct {
	Address string `json:"proxyNow"`
	Source  string `json:"proxySource"`
}

func parseNetworkProxy(address string) (*url.URL, bool) {
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" || parsed.Port() == "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, false
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
		return parsed, true
	}
	return nil, false
}

func networkBypasses(host string, entries []string) bool {
	for _, raw := range entries {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "*" || entry == host {
			return true
		}
		if strings.HasPrefix(entry, "*.") || strings.HasPrefix(entry, ".") {
			suffix := strings.TrimPrefix(entry, "*")
			if host == suffix[1:] || strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

// Use a system proxy only when its configured address also matches the
// active gateway settings. A redacted, named, or otherwise ambiguous proxy
// must not be guessed from its display label.
func networkCandidates(current networkRoute, status networkProxyStatus, systemProxy func() (string, []string, bool)) []networkCandidate {
	var proxy *url.URL
	var bypass []string
	switch status.Source {
	case "none", "down", "off":
	case "system":
		address, exclusions, ok := systemProxy()
		reported, validReported := parseNetworkProxy(status.Address)
		actual, validActual := parseNetworkProxy(address)
		if !ok || !validReported || !validActual || actual.User != nil || reported.String() != actual.String() {
			return nil
		}
		proxy, bypass = actual, exclusions
	case "environment", "settings":
		// The helper may have a different login-shell environment or a
		// saved proxy with credentials. Preserve its policy conservatively.
		return nil
	default:
		return nil
	}
	var candidates []networkCandidate
	for _, base := range []string{bestNetworkBase, gaNetworkBase} {
		global := proxy
		// Fixed official URLs only.
		if networkBypasses(strings.TrimPrefix(base, "https://"), bypass) {
			global = nil
		}
		if global == nil {
			// Both modes use the same transport here. Keep the existing
			// policy in a tie so a later network change behaves as before.
			candidates = append(candidates, networkCandidate{route: networkRoute{base, current.Proxy}})
			continue
		}
		candidates = append(candidates, networkCandidate{networkRoute{base, "direct"}, nil}, networkCandidate{networkRoute{base, ""}, global})
	}
	return candidates
}

func (host *server) checkNetwork(ctx context.Context, current networkRoute, key string) (networkRoute, bool) {
	var active networkProxyStatus
	ctx, cancel := context.WithTimeout(ctx, networkProbeTimeout)
	defer cancel()
	settingsCtx, stop := context.WithTimeout(ctx, 650*time.Millisecond)
	err := host.kernel.exchange(settingsCtx, http.MethodGet, "/api/settings", nil, &active)
	stop()
	if err != nil {
		return current, false
	}
	candidates := networkCandidates(current, active, func() (string, []string, bool) {
		return systemNetworkProxy(ctx)
	})
	probes := make(chan networkSample, len(candidates))
	for _, candidate := range candidates {
		go func() {
			duration, ok := probeNetwork(ctx, candidate.route, key, candidate.proxy)
			probes <- networkSample{candidate.route, duration, ok}
		}()
	}
	var samples []networkSample
	var settle *time.Timer
	var settled <-chan time.Time
	defer func() {
		if settle != nil {
			settle.Stop()
		}
	}()
	for len(samples) < len(candidates) {
		select {
		case sample := <-probes:
			samples = append(samples, sample)
			if sample.ok && settle == nil {
				// A slower incumbent can still win within the margin.
				settle = time.NewTimer(max(250*time.Millisecond, sample.duration/4))
				settled = settle.C
			}
		case <-settled:
			return chooseNetwork(samples, current)
		case <-ctx.Done():
			return chooseNetwork(samples, current)
		}
	}
	return chooseNetwork(samples, current)
}

func readNetworkFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("配置文件不可用")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("配置文件已变化")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if int64(len(data)) > limit {
		return nil, errors.New("配置文件不可用")
	}
	return data, err
}

func systemProxyAddress(address string, port string, scheme string) string {
	if address == "" || port == "" {
		return ""
	}
	return scheme + "://" + net.JoinHostPort(strings.Trim(address, "[]"), port)
}

func writeNetworkFile(path string, data, before []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".acu-network-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if before != nil {
		current, err := readNetworkFile(path, 16<<20)
		if err != nil || !bytes.Equal(current, before) {
			return errors.New("连接配置已变化，请重试")
		}
	}
	return os.Rename(file.Name(), path)
}

type managedNetworkProvider struct {
	path      string
	data      []byte
	document  map[string]json.RawMessage
	providers []json.RawMessage
	fields    map[string]json.RawMessage
	index     int
	key       string
	route     networkRoute
}

func fieldString(fields map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(fields[key], &value)
	return value
}

func standardNetworkProvider(fields map[string]json.RawMessage) (networkRoute, bool) {
	base, err := acuBase(fieldString(fields, "responses"))
	if err != nil || !supportedNetworkBase(base) {
		return networkRoute{}, false
	}
	for name, expected := range map[string]string{"chat": base + "/v1", "responses": base + "/v1", "anthropic": base, "modelsURL": base + "/v1/models"} {
		if actual := fieldString(fields, name); actual != expected {
			return networkRoute{}, false
		}
	}
	for _, name := range []string{"accountProxies", "keys", "resolve", "headers", "gemini", "decide", "clientCert", "clientKey", "serverCA", "serverPin", "insecureTLS", "http1", "authHeader", "preset", "vertex", "modelHeaders", "passHeaders", "accessKeyID", "secretAccessKey"} {
		switch string(fields[name]) {
		case "", "null", "[]", "{}", `""`, "false":
		default:
			return networkRoute{}, false
		}
	}
	proxy := fieldString(fields, "proxy")
	return networkRoute{base, proxy}, proxy == "" || proxy == "direct"
}

// Match the live local API to exactly one configuration. Preserve all
// unrelated fields, including newer fields the Runtime does not interpret.
func (host *server) managedNetwork(p localProvider) (*managedNetworkProvider, error) {
	host.kernel.mu.Lock()
	exe, local := host.kernel.exe, host.kernel.enforceVersion
	host.kernel.mu.Unlock()
	if host.networkFile == "" || exe == "" || !local || p.ID != networkProviderID || !p.Key.Set || p.Off {
		return nil, errors.New("当前连接由高级设置管理")
	}
	var matched *managedNetworkProvider
	for _, dir := range kernelConfigDirs(exe) {
		path := filepath.Join(dir, "providers.json")
		data, err := readNetworkFile(path, 16<<20)
		if err != nil {
			continue
		}
		current := &managedNetworkProvider{path: path, data: data}
		if json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &current.document) != nil || json.Unmarshal(current.document["providers"], &current.providers) != nil {
			continue
		}
		for i, raw := range current.providers {
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) != nil || fieldString(fields, "id") != p.ID || fieldString(fields, "name") != p.Name || fieldString(fields, "responses") != p.Responses || fieldString(fields, "chat") != p.Chat || fieldString(fields, "anthropic") != p.Anthropic || fieldString(fields, "modelsURL") != p.ModelsURL || fieldString(fields, "proxy") != p.Proxy {
				continue
			}
			route, standard := standardNetworkProvider(fields)
			key := fieldString(fields, "key")
			if !standard || !validConnectionKey(key) || matched != nil {
				return nil, errors.New("当前连接由高级设置管理")
			}
			current.index, current.fields, current.key, current.route = i, fields, key, route
			matched = current
		}
	}
	if matched == nil {
		return nil, errors.New("无法核对当前连接配置")
	}
	return matched, nil
}

func (current *managedNetworkProvider) replacement(route networkRoute) ([]byte, error) {
	if current.route.Base != route.Base {
		for name, value := range map[string]string{"chat": route.Base + "/v1", "responses": route.Base + "/v1", "anthropic": route.Base, "modelsURL": route.Base + "/v1/models"} {
			if len(current.fields[name]) != 0 {
				current.fields[name], _ = json.Marshal(value)
			}
		}
	}
	if route.Proxy != current.route.Proxy {
		current.fields["proxy"], _ = json.Marshal(route.Proxy)
	}
	current.providers[current.index], _ = json.Marshal(current.fields)
	current.document["providers"], _ = json.Marshal(current.providers)
	data, err := json.MarshalIndent(current.document, "", "  ")
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(current.data, []byte{0xef, 0xbb, 0xbf}) {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return append(data, '\n'), nil
}

func (host *server) verifyNetworkConfig(ctx context.Context, route networkRoute, key string) bool {
	p, err := host.configuredProvider(ctx)
	if err != nil || p.Responses != route.Base+"/v1" || p.Chat != route.Base+"/v1" || p.Anthropic != route.Base || p.ModelsURL != route.Base+"/v1/models" || p.Proxy != route.Proxy || p.Off {
		return false
	}
	actualKey, err := host.configuredNetworkKey(ctx)
	return err == nil && key == actualKey
}

func (host *server) applyNetworkConfig(ctx context.Context, current *managedNetworkProvider, route networkRoute, prefs networkPrefs) error {
	if route == current.route {
		return host.saveNetworkPrefs(prefs)
	}
	if !host.verifyNetworkConfig(ctx, current.route, current.key) {
		return errors.New("当前连接配置已变化，请重试")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := current.replacement(route)
	if err != nil {
		return errors.New("无法生成连接配置")
	}
	if err := writeNetworkFile(current.path, data, current.data); err != nil {
		return errors.New("无法保存当前连接配置，请重试")
	}
	// Finish the readback or rollback even if the user closed Settings.
	verifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2200*time.Millisecond)
	defer cancel()
	for attempt := 0; attempt < 4; attempt++ {
		if host.verifyNetworkConfig(verifyCtx, route, current.key) {
			if host.saveNetworkPrefs(prefs) == nil {
				return nil
			}
			break
		}
		select {
		case <-verifyCtx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
	if err := writeNetworkFile(current.path, current.data, data); err != nil {
		return errors.New("无法确认已保存的连接，请检查网络设置")
	}
	return errors.New(networkUnverified)
}

func (host *server) refreshNetwork(ctx context.Context, mode string, requested networkRoute, p localProvider) error {
	current, err := host.managedNetwork(p)
	if err != nil {
		return err
	}
	key, err := host.configuredNetworkKey(ctx)
	if err != nil || key != current.key {
		return errors.New("无法核对当前连接凭据")
	}
	next := requested
	if mode == "auto" {
		var ok bool
		next, ok = host.checkNetwork(ctx, current.route, key)
		if !ok {
			return host.keepNetworkAuto(current.route)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if next != current.route {
		// Verify availability through the actual gateway and exact candidate
		// before changing either URL or proxy. Do not refresh the live catalogue.
		listCtx, cancel := context.WithTimeout(ctx, networkVerifyTimeout)
		_, err := host.listConnectionModels(listCtx, next, key)
		cancel()
		if err != nil {
			if mode == "auto" && ctx.Err() == nil {
				return host.keepNetworkAuto(current.route)
			}
			return errors.New(networkUnverified)
		}
	}
	if err := host.applyNetworkConfig(ctx, current, next, networkPrefs{mode, next}); err != nil {
		return err
	}
	host.networkMessage = "已检查当前网络"
	if mode == "manual" {
		host.networkMessage = "连接设置已保存"
	}
	return nil
}

func (host *server) keepNetworkAuto(route networkRoute) error {
	if err := host.saveNetworkPrefs(networkPrefs{"auto", route}); err != nil {
		return err
	}
	host.networkMessage = "本次保留当前线路"
	return nil
}

func effectiveNetworkMode(prefs networkPrefs, current networkRoute) string {
	if prefs.Mode == "manual" || (prefs.Mode == "auto" && prefs.Applied != current) {
		return "manual"
	}
	return "auto"
}

func (host *server) startNetworkCheck(ctx context.Context) {
	if host.networkFile == "" {
		return
	}
	go func() {
		host.networkMu.Lock()
		defer host.networkMu.Unlock()
		if host.networkChecked {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, networkCheckTimeout)
		defer cancel()
		p, err := host.configuredProvider(ctx)
		if err != nil || p.ID == "" {
			return
		}
		host.networkChecked = true
		prefs := host.networkPrefs()
		actual := networkRoute{strings.TrimSuffix(p.Responses, "/v1"), p.Proxy}
		if effectiveNetworkMode(prefs, actual) != "auto" {
			return
		}
		if err := host.refreshNetwork(ctx, "auto", networkRoute{}, p); err != nil {
			host.networkMessage = "本次保留当前线路"
		}
	}()
}

func (host *server) networkSettings(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		response.Header().Set("Cache-Control", "no-store")
	}
	if !host.networkMu.TryLock() {
		if request.Method == http.MethodPost {
			host.json(response, http.StatusConflict, map[string]string{"error": "正在检查连接，请稍后重试"})
		} else {
			host.json(response, http.StatusOK, networkView{Checking: true})
		}
		return
	}
	defer host.networkMu.Unlock()
	ctx, cancel := context.WithTimeout(request.Context(), networkCheckTimeout)
	defer cancel()
	p, err := host.configuredProvider(ctx)
	if err != nil {
		host.json(response, http.StatusServiceUnavailable, map[string]string{"error": "暂时无法读取连接设置"})
		return
	}
	if request.Method == http.MethodPost {
		var input struct {
			Mode         string
			NetworkRoute networkRoute `json:"route"`
		}
		if json.NewDecoder(http.MaxBytesReader(response, request.Body, 2048)).Decode(&input) != nil || (input.Mode != "auto" && (input.Mode != "manual" || !supportedNetworkBase(input.NetworkRoute.Base) || (input.NetworkRoute.Proxy != "" && input.NetworkRoute.Proxy != "direct"))) {
			host.json(response, http.StatusBadRequest, map[string]string{"error": "请选择有效的 ACU 连接方式"})
			return
		}
		if err := host.refreshNetwork(ctx, input.Mode, input.NetworkRoute, p); err != nil {
			host.json(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		host.networkChecked = true
		p, err = host.configuredProvider(ctx)
		if err != nil {
			host.json(response, http.StatusServiceUnavailable, map[string]string{"error": "暂时无法读取连接设置"})
			return
		}
	}
	current, err := host.managedNetwork(p)
	view := networkView{Checked: host.networkChecked, Managed: err == nil, Message: host.networkMessage}
	if current != nil {
		view.Route = current.route
		view.Mode = effectiveNetworkMode(host.networkPrefs(), current.route)
	}
	host.json(response, http.StatusOK, view)
}
