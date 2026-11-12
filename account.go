package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type loginTicket struct {
	Preferences bool      `json:"preferences,omitempty"`
	Site        string    `json:"site"`
	State       string    `json:"state"`
	Register    bool      `json:"register"`
	Restore     bool      `json:"restore,omitempty"`
	SessionID   string    `json:"sessionId,omitempty"`
	Expires     time.Time `json:"-"`
}

type accountKey struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

type accountView struct {
	Identity    string       `json:"identity,omitempty"`
	LoggedIn    bool         `json:"loggedIn"`
	Username    string       `json:"username,omitempty"`
	Site        string       `json:"site,omitempty"`
	Keys        []accountKey `json:"keys"`
	Total       int          `json:"total"`
	Selected    int          `json:"selected"`
	Pending     bool         `json:"pending"`
	WindowReady bool         `json:"windowReady"`
	Refreshing  bool         `json:"refreshing"`
	Restoring   bool         `json:"restoring"`
	Error       string       `json:"error,omitempty"`
}

type accountState struct {
	mu           sync.Mutex
	view         accountView
	bearer       string
	expires      int64
	ticket       loginTicket
	session      loginTicket
	userID       int
	sessionID    string
	renewalError bool
}

func (account *accountState) snapshot() accountView {
	account.mu.Lock()
	defer account.mu.Unlock()
	view := account.view
	if view.LoggedIn {
		view.Identity = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprint(account.userID)+"/"+account.session.State+"/"+account.sessionID)))
	}
	view.Refreshing = view.LoggedIn && account.expires <= time.Now().Unix()
	view.Pending = account.ticket.State != "" && time.Now().Before(account.ticket.Expires)
	view.Restoring = view.Pending && account.ticket.Restore
	return view
}

func (account *accountState) sessionActive(state string) bool {
	account.mu.Lock()
	defer account.mu.Unlock()
	return state != "" && account.session.State == state
}

func (account *accountState) refreshDue(state string) bool {
	account.mu.Lock()
	defer account.mu.Unlock()
	return state != "" && account.session.State == state && account.expires <= time.Now().Add(2*time.Minute).Unix()
}

func (account *accountState) pending(state string) bool {
	account.mu.Lock()
	defer account.mu.Unlock()
	return account.ticket.State == state && time.Now().Before(account.ticket.Expires)
}

func (account *accountState) cancel(state string) {
	account.mu.Lock()
	defer account.mu.Unlock()
	if account.ticket.State == state {
		account.ticket = loginTicket{}
	}
}

func accountSite(site string) (string, error) {
	switch site {
	case "acu.best", "https://acu.best":
		return "https://acu.best", nil
	case "acucompute.com", "https://acucompute.com", "https://console.acucompute.com":
		return "https://console.acucompute.com", nil
	default:
		return "", errors.New("请选择 ACU 官方站点")
	}
}

func acuBase(value string) (string, error) {
	address, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil || address.Scheme != "https" || address.User != nil || address.RawQuery != "" || address.Fragment != "" || address.Port() != "" && address.Port() != "443" {
		return "", errors.New("ACU 地址须为官方域名下的 HTTPS API 地址")
	}
	hostname := strings.ToLower(address.Hostname())
	if hostname != "acu.best" && hostname != "acucompute.com" && !strings.HasSuffix(hostname, ".acu.best") && !strings.HasSuffix(hostname, ".acucompute.com") {
		return "", errors.New("这里仅连接 ACU；其他模型来源请使用高级配置")
	}
	switch address.Path {
	case "", "/v1", "/acu", "/acu/v1":
		address.Path = strings.TrimSuffix(address.Path, "/v1")
	default:
		return "", errors.New("请填写 API 根地址或以 /v1 结尾的地址")
	}
	return strings.TrimRight(address.String(), "/"), nil
}

type connectionView struct {
	Known      bool               `json:"known"`
	Configured bool               `json:"configured"`
	Provider   string             `json:"provider,omitempty"`
	Base       string             `json:"base,omitempty"`
	Masked     string             `json:"masked,omitempty"`
	Models     int                `json:"models"`
	Error      string             `json:"error,omitempty"`
	Choices    []connectionChoice `json:"choices"`
}

type connectionChoice struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Masked string   `json:"masked"`
	Models []string `json:"models"`
}

type localProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Chat      string `json:"chat"`
	Responses string `json:"responses"`
	Anthropic string `json:"anthropic"`
	Off       bool   `json:"off"`
	Key       struct {
		Set    bool   `json:"set"`
		Masked string `json:"masked"`
	} `json:"key"`
	Models []struct {
		ID string `json:"id"`
		On bool   `json:"on"`
	} `json:"models"`
}

func (kernel *kernelState) exchange(ctx context.Context, method, path string, input, output any) error {
	endpoint, err := kernel.target(path)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet {
		request.Header.Set("Origin", endpoint.Scheme+"://"+endpoint.Host)
	}
	response, err := kernel.client.Do(request)
	if err != nil {
		return errors.New("本机服务暂时不可用，请检查内核状态")
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return fmt.Errorf("本机配置未完成（HTTP %d），请在高级配置中检查", response.StatusCode)
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return errors.New("无法读取本机配置结果")
	}
	var failure struct {
		Error string `json:"error"`
	}
	if len(encoded) > 0 && json.Unmarshal(encoded, &failure) == nil && failure.Error != "" {
		return errors.New("内核拒绝了配置，请在高级配置中检查")
	}
	if output != nil && json.Unmarshal(encoded, output) != nil {
		return errors.New("无法识别内核配置结果")
	}
	return nil
}

func (host *server) connection(ctx context.Context) connectionView {
	var result struct {
		Providers []localProvider `json:"providers"`
	}
	if err := host.kernel.exchange(ctx, http.MethodGet, "/api/providers", nil, &result); err != nil {
		return connectionView{Error: "暂时无法读取 ACU 连接，点击重新检测。"}
	}
	view := connectionView{Known: true, Choices: []connectionChoice{}}
	for _, provider := range result.Providers {
		if provider.Off || !provider.Key.Set {
			continue
		}
		for _, address := range []string{provider.Chat, provider.Responses, provider.Anthropic} {
			base, err := acuBase(address)
			if err != nil {
				continue
			}
			current := connectionView{Known: true, Configured: true, Provider: provider.ID, Base: base + "/v1", Masked: provider.Key.Masked}
			choice := connectionChoice{ID: provider.ID, Name: provider.Name, Masked: provider.Key.Masked, Models: []string{}}
			for _, model := range provider.Models {
				if model.On {
					current.Models++
					if model.ID != "" {
						choice.Models = append(choice.Models, model.ID)
					}
				}
			}
			choices := append(view.Choices, choice)
			if !view.Configured || provider.ID == "acu-runtime" {
				view = current
			}
			view.Choices = choices
			break
		}
	}
	return view
}

func (host *server) cloudJSON(ctx context.Context, method, address, bearer string, output any) error {
	parsed, err := url.Parse(address)
	if err != nil {
		return errors.New("无效的 ACU 请求")
	}
	if _, err := acuBase(parsed.Scheme + "://" + parsed.Host); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, address, nil)
	if err != nil {
		return errors.New("无效的 ACU 请求")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	request.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("ACU 认证请求不跟随重定向") }}
	if host.cloudTransport != nil {
		client.Transport = host.cloudTransport
	}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("暂时无法连接 ACU，请重试或切换官方站点")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return errors.New("登录会话或 API Key 已失效，请重新连接")
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("ACU 请求未完成（HTTP %d），请检查权限或在网站完成验证", response.StatusCode)
	}
	var envelope map[string]json.RawMessage
	if json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope) != nil {
		return errors.New("请在 ACU 登录窗口完成网站验证，或直接使用 API Key")
	}
	if success, exists := envelope["success"]; exists && string(success) != "true" {
		return errors.New("ACU 未接受该请求，请检查登录状态与 Key 权限")
	}
	if _, exists := envelope["error"]; exists {
		return errors.New("ACU 未接受该请求，请检查 Key 权限")
	}
	if json.Unmarshal(envelope["data"], output) != nil {
		return errors.New("ACU 返回的数据暂时无法识别")
	}
	return nil
}

func (host *server) beginAccountLogin(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Site        string `json:"site"`
		Register    bool   `json:"register"`
		Preferences bool   `json:"preferences"`
	}
	if json.NewDecoder(http.MaxBytesReader(response, request.Body, 4096)).Decode(&input) != nil {
		host.json(response, 400, map[string]string{"error": "请选择登录站点"})
		return
	}
	site, err := accountSite(input.Site)
	if err != nil {
		host.json(response, 400, map[string]string{"error": err.Error()})
		return
	}
	if !host.desktop || host.loginRequests == nil {
		host.json(response, 400, map[string]string{"error": "请在 ACU Runtime 桌面应用中登录"})
		return
	}
	ticket := loginTicket{Site: site, State: secret(), Register: input.Register, Preferences: input.Preferences, Expires: time.Now().Add(10 * time.Minute)}
	host.account.mu.Lock()
	host.account.ticket = ticket
	host.account.view.Error = ""
	host.account.view.WindowReady = false
	host.account.mu.Unlock()
	select {
	case host.loginRequests <- ticket:
		host.json(response, 200, map[string]bool{"ok": true})
	default:
		host.account.cancel(ticket.State)
		host.json(response, 409, map[string]string{"error": "登录窗口正在准备，请稍后重试"})
	}
}

func (host *server) accountWindowTicket(windowName, sender, top string) (loginTicket, error) {
	host.account.mu.Lock()
	ticket := host.account.ticket
	host.account.mu.Unlock()
	if ticket.State == "" || time.Now().After(ticket.Expires) || windowName != "acu-login-"+ticket.State {
		return loginTicket{}, errors.New("登录请求已失效，请重新打开登录窗口")
	}
	for _, source := range []string{sender, top} {
		address, err := url.Parse(source)
		if err != nil || address.User != nil || address.Scheme+"://"+address.Host != ticket.Site {
			return loginTicket{}, errors.New("仅接受 ACU 官方登录窗口的连接请求")
		}
	}
	return ticket, nil
}

func (host *server) acknowledgeAccountWindow(windowName, sender, top, message string) error {
	ticket, err := host.accountWindowTicket(windowName, sender, top)
	if err != nil {
		return err
	}
	var input struct {
		Type  string `json:"type"`
		State string `json:"state"`
	}
	if json.Unmarshal([]byte(message), &input) != nil || input.Type != "acu-account-ready" || subtle.ConstantTimeCompare([]byte(input.State), []byte(ticket.State)) != 1 {
		return errors.New("登录窗口连接信息不匹配")
	}
	host.account.mu.Lock()
	defer host.account.mu.Unlock()
	if host.account.ticket.State != ticket.State {
		return errors.New("登录已取消")
	}
	host.account.view.WindowReady = true
	return nil
}

func (host *server) completeAccountLogin(ctx context.Context, windowName, sender, top, message string) error {
	ticket, err := host.accountWindowTicket(windowName, sender, top)
	if err != nil {
		return err
	}
	var input struct {
		Type        string `json:"type"`
		State       string `json:"state"`
		AccessToken string `json:"accessToken"`
		Expires     int64  `json:"expiresAt"`
		SessionID   string `json:"sessionId"`
	}
	if len(message) > 32768 || json.Unmarshal([]byte(message), &input) != nil || input.Type != "acu-account-login" || subtle.ConstantTimeCompare([]byte(input.State), []byte(ticket.State)) != 1 || input.AccessToken == "" || len(input.AccessToken) > 16384 || input.Expires <= time.Now().Unix() {
		return errors.New("登录信息不完整或已失效，请重新登录")
	}
	var user struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	if err := host.cloudJSON(ctx, http.MethodGet, ticket.Site+"/api/user/self", input.AccessToken, &user); err != nil {
		return err
	}
	if user.ID <= 0 || user.Username == "" {
		return errors.New("无法确认 ACU 账号")
	}
	if ticket.Restore {
		host.account.mu.Lock()
		matches := host.account.userID == user.ID && (host.account.sessionID == "" || host.account.sessionID == input.SessionID)
		host.account.mu.Unlock()
		if !matches {
			host.forgetAccount()
			return errors.New("官网会话已切换，请重新登录 ACU")
		}
	}
	var listed struct {
		Total int `json:"total"`
		Items []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Status    int    `json:"status"`
			Expired   int64  `json:"expired_time"`
			Remaining int64  `json:"remain_quota"`
			Unlimited bool   `json:"unlimited_quota"`
		} `json:"items"`
	}
	keyError := host.cloudJSON(ctx, http.MethodGet, ticket.Site+"/api/token/?p=1&page_size=100", input.AccessToken, &listed)
	view := accountView{LoggedIn: true, Username: user.Username, Site: ticket.Site, Total: listed.Total, Keys: []accountKey{}}
	if ticket.Restore {
		host.account.mu.Lock()
		view.Selected = host.account.view.Selected
		host.account.mu.Unlock()
	}
	for _, key := range listed.Items {
		available := key.ID > 0 && key.Status == 1 && (key.Expired <= 0 || key.Expired > time.Now().Unix()) && (key.Unlimited || key.Remaining > 0)
		view.Keys = append(view.Keys, accountKey{ID: key.ID, Name: key.Name, Available: available})
		if view.Selected == 0 && available && !ticket.Restore && !ticket.Preferences {
			view.Selected = key.ID
		}
	}
	if keyError != nil {
		view.Error = "账号已登录，但 Key 列表读取失败；可重试登录或直接使用 API Key。"
	} else if view.Selected == 0 && !ticket.Restore && !ticket.Preferences {
		view.Error = "账号下没有可用 Key，请在网站创建后重新登录。"
	}
	host.account.mu.Lock()
	if host.account.ticket.State != ticket.State || time.Now().After(ticket.Expires) {
		host.account.mu.Unlock()
		return errors.New("此登录已取消，请使用最新的登录窗口")
	}
	host.account.bearer, host.account.expires, host.account.view, host.account.ticket = input.AccessToken, input.Expires, view, loginTicket{}
	host.account.session, host.account.userID, host.account.sessionID = ticket, user.ID, input.SessionID
	host.account.renewalError = false
	host.account.mu.Unlock()
	if view.Selected > 0 && !ticket.Restore && !ticket.Preferences {
		if err := host.useAccountKey(ctx, view.Selected); err != nil {
			host.account.mu.Lock()
			host.account.view.Error = "已登录，但默认 Key 未连接：" + err.Error()
			host.account.mu.Unlock()
		}
	}
	host.rememberAccount()
	return nil
}

func (host *server) useAccountKey(ctx context.Context, selected int) error {
	view := host.account.snapshot()
	available := false
	for _, key := range view.Keys {
		if key.ID == selected && key.Available {
			available = true
		}
	}
	if !view.LoggedIn || !available {
		return errors.New("请登录后选择一个可用的 API Key")
	}
	host.account.mu.Lock()
	bearer, site := host.account.bearer, host.account.view.Site
	expires := host.account.expires
	host.account.mu.Unlock()
	if expires <= time.Now().Unix() {
		return errors.New("账号正在自动续期，请稍后再切换 Key；已配置的 API Key 不受影响。")
	}
	var revealed struct {
		Key string `json:"key"`
	}
	if err := host.cloudJSON(ctx, http.MethodPost, site+"/api/token/"+strconv.Itoa(selected)+"/key", bearer, &revealed); err != nil {
		return err
	}
	if err := host.saveACUConnection(ctx, site, revealed.Key); err != nil {
		return err
	}
	host.account.mu.Lock()
	host.account.view.Selected = selected
	host.account.view.Error = ""
	host.account.mu.Unlock()
	host.rememberAccount()
	return nil
}

func (host *server) saveACUConnection(ctx context.Context, address, key string) error {
	base, err := acuBase(address)
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n\t ") {
		return errors.New("请输入完整的 ACU API Key")
	}
	var models []struct {
		ID string `json:"id"`
	}
	if err := host.cloudJSON(ctx, http.MethodGet, base+"/v1/models", key, &models); err != nil {
		return err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, model := range models {
		if model.ID != "" && len(model.ID) <= 256 && !strings.ContainsAny(model.ID, "\r\n\t") && !seen[model.ID] {
			ids = append(ids, model.ID)
			seen[model.ID] = true
		}
	}
	if len(ids) == 0 {
		return errors.New("此 Key 没有可用模型，请在 ACU 网站检查权限")
	}
	var existing struct {
		Providers []localProvider `json:"providers"`
	}
	if err := host.kernel.exchange(ctx, http.MethodGet, "/api/providers", nil, &existing); err != nil {
		return err
	}
	found, off := false, false
	for _, provider := range existing.Providers {
		if provider.ID != "acu-runtime" {
			continue
		}
		if _, err := acuBase(provider.Chat); err != nil {
			return errors.New("已有同名模型来源，请先在高级配置中检查，未覆盖原配置")
		}
		found, off = true, provider.Off
	}
	body := map[string]any{"id": "acu-runtime", "name": "ACU", "key": key, "chat": base + "/v1", "responses": base + "/v1", "anthropic": base, "modelsURL": base + "/v1/models", "models": ids, "new": !found}
	if err := host.kernel.exchange(ctx, http.MethodPost, "/api/provider/save", body, nil); err != nil {
		return err
	}
	if off {
		return host.kernel.exchange(ctx, http.MethodPost, "/api/provider/on", map[string]string{"id": "acu-runtime"}, nil)
	}
	return nil
}

func (host *server) configureACU(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Base string `json:"base"`
		Key  string `json:"key"`
		ID   int    `json:"id"`
	}
	if json.NewDecoder(http.MaxBytesReader(response, request.Body, 8192)).Decode(&input) != nil {
		host.json(response, 400, map[string]string{"error": "无法读取连接配置"})
		return
	}
	if input.ID > 0 {
		ctx, cancel := context.WithTimeout(request.Context(), 45*time.Second)
		defer cancel()
		if err := host.useAccountKey(ctx, input.ID); err != nil {
			host.json(response, 400, map[string]string{"error": err.Error()})
			return
		}
		host.json(response, 200, map[string]bool{"ok": true})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 45*time.Second)
	defer cancel()
	if err := host.saveACUConnection(ctx, input.Base, input.Key); err != nil {
		host.json(response, 400, map[string]string{"error": err.Error()})
		return
	}
	host.json(response, 200, map[string]bool{"ok": true})
}
