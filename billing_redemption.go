package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func billingStoreURL(raw string) string {
	address, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil || (address.Port() != "" && address.Port() != "443") {
		return ""
	}
	return address.String()
}

func (host *server) openBillingStore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !host.desktop {
		host.json(w, 409, map[string]string{"error": "请在桌面应用中打开商店。"})
		return
	}
	host.account.mu.Lock()
	view, bearer, expires, identity := host.account.view, host.account.bearer, host.account.expires, host.account.loginIdentity()
	host.account.mu.Unlock()
	site, err := accountSite(view.Site)
	if err != nil || !view.LoggedIn || bearer == "" || expires <= time.Now().Unix() {
		host.json(w, 401, map[string]string{"error": "请重新登录 ACU 后打开商店。"})
		return
	}
	var topup billingTopupInfo
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if host.cloudJSON(ctx, http.MethodGet, site+"/api/user/topup/info", bearer, &topup) != nil || topup.EnableRedemption != nil && !*topup.EnableRedemption {
		host.json(w, 503, map[string]string{"error": "暂时无法读取商店入口，请刷新账户后重试。"})
		return
	}
	address := billingStoreURL(topup.TopupLink)
	host.account.mu.Lock()
	current := host.account.view.LoggedIn && host.account.loginIdentity() == identity
	host.account.mu.Unlock()
	if !current {
		host.json(w, 409, map[string]string{"error": "登录账户已变化，请返回账户页重试。"})
		return
	}
	if address == "" || openBillingBrowser(address) != nil {
		host.json(w, 502, map[string]string{"error": "无法打开 ACU 商店，请稍后重试。"})
		return
	}
	host.json(w, 200, map[string]bool{"ok": true})
}

func (host *server) redeemAccountCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Code     string `json:"code"`
		Identity string `json:"identity"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		host.json(w, 400, map[string]string{"error": "兑换请求无效。"})
		return
	}
	code := strings.TrimSpace(input.Code)
	if code == "" || len(code) > 128 {
		host.json(w, 400, map[string]string{"error": "请输入有效的兑换码。"})
		return
	}
	host.account.mu.Lock()
	view, bearer, expires, identity := host.account.view, host.account.bearer, host.account.expires, host.account.loginIdentity()
	host.account.mu.Unlock()
	site, err := accountSite(view.Site)
	if err != nil || !view.LoggedIn || bearer == "" || expires <= time.Now().Unix() {
		host.json(w, 401, map[string]string{"error": "请重新登录 ACU 后兑换。"})
		return
	}
	if input.Identity != identity {
		host.json(w, 409, map[string]string{"error": "登录账户已变化，请确认当前账户后重新输入兑换码。"})
		return
	}
	payload, _ := json.Marshal(map[string]string{"key": code})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, site+"/api/user/topup", bytes.NewReader(payload))
	if err != nil {
		host.json(w, 500, map[string]string{"error": "无法创建兑换请求。"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "zh-CN")
	req.Header.Set("Origin", site)
	client := http.Client{Transport: host.cloudTransport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	unknown := "无法确认兑换结果，请先刷新账户余额，再决定是否重试。"
	res, err := client.Do(req)
	if err != nil {
		host.json(w, 502, map[string]string{"error": unknown})
		return
	}
	defer res.Body.Close()
	var result struct {
		Success *bool  `json:"success"`
		Added   *int64 `json:"data"`
	}
	switch {
	case res.StatusCode == 401 || res.StatusCode == 403:
		host.json(w, 401, map[string]string{"error": "会话已失效或无兑换权限，请重新登录 ACU。"})
	case res.StatusCode == 429:
		host.json(w, 429, map[string]string{"error": "提交过于频繁，请稍后重试。"})
	case res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&result) != nil || result.Success == nil:
		host.json(w, 502, map[string]string{"error": unknown})
	case !*result.Success:
		host.json(w, 422, map[string]string{"error": "兑换未成功，请核对兑换码，或前往官网钱包查看。"})
	case result.Added == nil:
		host.json(w, 502, map[string]string{"error": unknown})
	default:
		host.json(w, 200, map[string]int64{"addedQuota": *result.Added})
	}
}
