package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

func (host *server) completeAccountRefresh(ctx context.Context, windowName, sender, top, message string) error {
	var input struct {
		Type        string `json:"type"`
		State       string `json:"state"`
		AccessToken string `json:"accessToken"`
		Expires     int64  `json:"expiresAt"`
		SessionID   string `json:"sessionId"`
		Expired     bool   `json:"expired"`
	}
	if len(message) > 32768 || json.Unmarshal([]byte(message), &input) != nil || (input.Type != "acu-account-refresh" && input.Type != "acu-account-refresh-error") {
		return errors.New("无效的账号续期消息")
	}
	host.account.mu.Lock()
	session, userID, sessionID := host.account.session, host.account.userID, host.account.sessionID
	host.account.mu.Unlock()
	if session.State == "" || windowName != "acu-login-"+session.State || subtle.ConstantTimeCompare([]byte(input.State), []byte(session.State)) != 1 {
		return errors.New("账号续期窗口已失效")
	}
	for _, source := range []string{sender, top} {
		address, err := url.Parse(source)
		if err != nil || address.User != nil || address.Scheme+"://"+address.Host != session.Site {
			return errors.New("仅接受 ACU 官方窗口的续期消息")
		}
	}
	if input.Type == "acu-account-refresh-error" {
		if input.Expired {
			host.forgetAccount()
			host.account.mu.Lock()
			host.account.view.Error = "登录已失效，请重新登录；已配置的 API Key 不受影响。"
			host.account.mu.Unlock()
			return nil
		}
		host.account.mu.Lock()
		defer host.account.mu.Unlock()
		if host.account.session.State != session.State {
			return errors.New("账号续期已取消")
		}
		if host.account.expires <= time.Now().Unix() {
			host.account.renewalError = true
			host.account.view.Error = "账号续期暂未完成，正在自动重试；已配置的 API Key 不受影响。"
		}
		return nil
	}
	if input.AccessToken == "" || len(input.AccessToken) > 16384 || input.Expires <= time.Now().Unix() || (sessionID != "" && input.SessionID != sessionID) {
		return errors.New("账号续期信息不完整或会话不匹配")
	}
	var user struct {
		ID int `json:"id"`
	}
	if err := host.cloudJSON(ctx, http.MethodGet, session.Site+"/api/user/self", input.AccessToken, &user); err != nil {
		return err
	}
	if user.ID != userID || userID <= 0 {
		return errors.New("续期账号发生变化，请重新登录")
	}
	host.account.mu.Lock()
	defer host.account.mu.Unlock()
	if host.account.session.State != session.State {
		return errors.New("账号续期已取消")
	}
	if input.Expires < host.account.expires {
		return errors.New("忽略已被替代的账号续期")
	}
	host.account.bearer, host.account.expires = input.AccessToken, input.Expires
	if host.account.renewalError {
		host.account.view.Error = ""
		host.account.renewalError = false
	}
	return nil
}
