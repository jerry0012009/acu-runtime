package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Personal reads use only the official dashboard session, never a model Key.
func (host *server) accountPreferences(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("X-ACU-Token") != host.token {
		host.json(w, 403, map[string]string{"error": "invalid local session"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/runtime/preferences/")
	paths := map[string]string{"profile": "acu-preference-profile", "library": "acu-preference-library", "activity": "acu-preference-activity", "settings": "acu-advisor/settings"}
	path, ok := paths[name]
	if !ok {
		host.json(w, 404, map[string]string{"error": "unknown preference view"})
		return
	}
	host.account.mu.Lock()
	view, bearer, expires, userID, session := host.account.view, host.account.bearer, host.account.expires, host.account.userID, host.account.session.State
	host.account.mu.Unlock()
	if !view.LoggedIn || bearer == "" {
		host.json(w, 401, map[string]string{"error": "请登录 ACU 后查看个人偏好。"})
		return
	}
	if expires <= time.Now().Unix() {
		host.json(w, 503, map[string]string{"error": "正在续期官网登录，请稍后重试。"})
		return
	}
	site, err := accountSite(view.Site)
	if err != nil {
		host.json(w, 502, map[string]string{"error": "无法识别官方登录站点。"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, site+"/api/user/self/"+path, nil)
	if err != nil {
		host.json(w, 502, map[string]string{"error": "无法读取偏好。"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second, Transport: host.cloudTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		host.json(w, 502, map[string]string{"error": "暂时无法连接 ACU，请重试。"})
		return
	}
	defer res.Body.Close()
	content, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	host.account.mu.Lock()
	valid := host.account.view.LoggedIn && host.account.userID == userID && host.account.session.State == session && host.account.bearer == bearer
	host.account.mu.Unlock()
	if !valid {
		host.json(w, 409, map[string]string{"error": "账户状态已变化，请刷新。"})
		return
	}
	if res.StatusCode == 404 {
		host.json(w, 404, map[string]string{"error": "当前服务暂不支持此偏好视图。"})
		return
	}
	if res.StatusCode == 401 || res.StatusCode == 403 {
		// Clear only the session that received the revocation, never a newer login.
		host.account.mu.Lock()
		if host.account.bearer == bearer && host.account.session.State == session {
			host.account.view = accountView{Error: "偏好访问会话已失效或无权限，请重新登录。"}
			host.account.bearer = ""
			host.account.session = loginTicket{}
		}
		host.account.mu.Unlock()
		host.json(w, 401, map[string]string{"error": "偏好访问会话已失效或无权限，请重新登录。"})
		return
	}
	if err != nil || len(content) > 2<<20 || res.StatusCode != 200 {
		host.json(w, 502, map[string]string{"error": "偏好读取失败，请稍后重试。"})
		return
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(content, &envelope) != nil || !envelope.Success || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		host.json(w, 502, map[string]string{"error": "偏好数据格式暂不可用。"})
		return
	}
	if name == "library" || name == "activity" {
		var version struct {
			Version int `json:"version"`
		}
		if json.Unmarshal(envelope.Data, &version) != nil || version.Version != 1 {
			host.json(w, 502, map[string]string{"error": "偏好接口版本暂不兼容。"})
			return
		}
	}
	host.json(w, 200, envelope.Data)
}
