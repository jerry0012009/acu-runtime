package main

import (
	"crypto/subtle"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (host *server) accountRestoreError(windowName, sender, top, message string) error {
	ticket, err := host.accountWindowTicket(windowName, sender, top)
	if err != nil {
		return err
	}
	var input struct {
		State   string `json:"state"`
		Expired bool   `json:"expired"`
	}
	if !ticket.Restore || json.Unmarshal([]byte(message), &input) != nil || subtle.ConstantTimeCompare([]byte(input.State), []byte(ticket.State)) != 1 {
		return os.ErrPermission
	}
	if input.Expired {
		host.forgetAccount()
	}
	host.account.mu.Lock()
	if input.Expired {
		host.account.view.Error = "官网登录已过期，请重新登录；已配置的 API Key 不受影响。"
	} else {
		host.account.view.Error = "正在重试恢复官网登录；请检查网络，或点击登录完成验证。"
	}
	host.account.mu.Unlock()
	return nil
}

// Authentication cookies stay in the system WebView. This file contains no
// access/refresh token or model API Key; it only pins the identity to restore.
type rememberedAccount struct {
	Site      string `json:"site"`
	UserID    int    `json:"userId"`
	SessionID string `json:"sessionId"`
	Selected  int    `json:"selected"`
}

func (host *server) rememberAccount() {
	host.accountFileMu.Lock()
	defer host.accountFileMu.Unlock()
	if host.accountFile == "" {
		return
	}
	host.account.mu.Lock()
	value := rememberedAccount{host.account.view.Site, host.account.userID, host.account.sessionID, host.account.view.Selected}
	valid := host.account.view.LoggedIn
	host.account.mu.Unlock()
	if !valid {
		return
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(host.accountFile), 0700)
	}
	if err == nil {
		err = os.WriteFile(host.accountFile, encoded, 0600)
	}
	if err != nil {
		host.account.mu.Lock()
		host.account.view.Error = "账号已连接，但无法保存自动登录设置；下次启动请重新登录。"
		host.account.mu.Unlock()
	}
}

func (host *server) forgetAccount() {
	host.accountFileMu.Lock()
	defer host.accountFileMu.Unlock()
	host.account.mu.Lock()
	host.account.view = accountView{}
	host.account.bearer, host.account.sessionID = "", ""
	host.account.session, host.account.ticket = loginTicket{}, loginTicket{}
	host.account.userID, host.account.expires = 0, 0
	host.account.mu.Unlock()
	if host.accountFile != "" {
		_ = os.Remove(host.accountFile)
	}
}

func (host *server) restoreAccount() {
	if host.accountFile == "" {
		return
	}
	encoded, err := os.ReadFile(host.accountFile)
	var saved rememberedAccount
	if err != nil || len(encoded) > 4096 || json.Unmarshal(encoded, &saved) != nil || saved.UserID <= 0 {
		return
	}
	site, err := accountSite(saved.Site)
	if err != nil {
		return
	}
	ticket := loginTicket{Site: site, State: secret(), Restore: true, SessionID: saved.SessionID, Expires: time.Now().Add(10 * time.Minute)}
	host.account.mu.Lock()
	host.account.userID, host.account.sessionID = saved.UserID, saved.SessionID
	host.account.view = accountView{Site: site, Selected: saved.Selected}
	host.account.ticket = ticket
	host.account.mu.Unlock()
	select {
	case host.loginRequests <- ticket:
	default:
		host.account.cancel(ticket.State)
	}
}
