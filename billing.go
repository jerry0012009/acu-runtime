package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const billingSite = "https://console.acucompute.com"

type billingLog struct {
	ID           int             `json:"id"`
	Created      int64           `json:"created_at"`
	Model        string          `json:"model_name"`
	KeyName      string          `json:"token_name"`
	Quota        int64           `json:"quota"`
	Input        int64           `json:"prompt_tokens"`
	Output       int64           `json:"completion_tokens"`
	Charge       json.RawMessage `json:"chargeCny,omitempty"`
	PriceVersion string          `json:"priceVersion,omitempty"`
}

func (host *server) accountBilling(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	if request.Header.Get("X-ACU-Token") != host.token {
		host.json(response, 403, map[string]string{"error": "invalid local session"})
		return
	}
	days, err := strconv.Atoi(request.URL.Query().Get("days"))
	if err != nil || (days != 1 && days != 7 && days != 30) {
		days = 7
	}
	page, _ := strconv.Atoi(request.URL.Query().Get("page"))
	if page < 1 || page > 10000 {
		page = 1
	}
	host.account.mu.Lock()
	view, bearer, expires, userID := host.account.view, host.account.bearer, host.account.expires, host.account.userID
	host.account.mu.Unlock()
	if !view.LoggedIn || bearer == "" || expires <= time.Now().Unix() {
		host.json(response, 200, map[string]any{"loggedIn": false, "message": "登录后查看账户余额与云端历史消费；本机用量与公开价格可继续查看。"})
		return
	}
	result := map[string]any{"loggedIn": true, "site": view.Site, "username": view.Username, "days": days, "page": page, "updatedAt": time.Now().Unix()}
	warnings := map[string]string{}
	var config struct {
		QuotaPerUnit float64 `json:"quota_per_unit"`
		FX           float64 `json:"usd_exchange_rate"`
	}
	if err := host.cloudJSON(request.Context(), http.MethodGet, view.Site+"/api/status", "", &config); err != nil {
		warnings["currency"] = err.Error()
	} else if config.QuotaPerUnit > 0 && config.FX > 0 && !math.IsInf(config.FX/config.QuotaPerUnit, 0) {
		result["cnyPerQuota"] = config.FX / config.QuotaPerUnit
	}
	var user struct {
		ID    int    `json:"id"`
		Quota *int64 `json:"quota"`
	}
	if err := host.cloudJSON(request.Context(), http.MethodGet, view.Site+"/api/user/self", bearer, &user); err != nil {
		warnings["balance"] = err.Error()
	} else if user.ID != userID {
		host.json(response, 409, map[string]string{"error": "账户已变化，请重新登录。"})
		return
	} else {
		result["balanceQuota"] = user.Quota
	}
	if view.Selected > 0 {
		var key struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Remaining *int64 `json:"remain_quota"`
			Unlimited bool   `json:"unlimited_quota"`
			Expires   int64  `json:"expired_time"`
			Status    int    `json:"status"`
		}
		if err := host.cloudJSON(request.Context(), http.MethodGet, view.Site+"/api/token/"+strconv.Itoa(view.Selected), bearer, &key); err != nil {
			warnings["key"] = err.Error()
		} else {
			result["key"] = key
		}
	}
	now := time.Now()
	start := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
	query := url.Values{"type": {"2"}, "start_timestamp": {strconv.FormatInt(start, 10)}, "end_timestamp": {strconv.FormatInt(now.Unix(), 10)}}
	var summary struct {
		Charge *string `json:"user_charge_cny"`
		Exact  int64   `json:"exact_charge_count"`
		Legacy int64   `json:"legacy_charge_count"`
	}
	if err := host.cloudJSON(request.Context(), http.MethodGet, view.Site+"/api/log/self/stat?"+query.Encode(), bearer, &summary); err != nil {
		warnings["summary"] = err.Error()
	} else {
		result["summary"] = summary
	}
	query.Set("p", strconv.Itoa(page))
	query.Set("page_size", "20")
	var logs struct {
		Total int `json:"total"`
		Items []struct {
			ID      int    `json:"id"`
			Created int64  `json:"created_at"`
			Model   string `json:"model_name"`
			KeyName string `json:"token_name"`
			Quota   int64  `json:"quota"`
			Input   int64  `json:"prompt_tokens"`
			Output  int64  `json:"completion_tokens"`
			Other   string `json:"other"`
		} `json:"items"`
	}
	if err := host.cloudJSON(request.Context(), http.MethodGet, view.Site+"/api/log/self?"+query.Encode(), bearer, &logs); err != nil {
		warnings["logs"] = err.Error()
	} else {
		rows := []billingLog{}
		for _, log := range logs.Items {
			row := billingLog{ID: log.ID, Created: log.Created, Model: log.Model, KeyName: log.KeyName, Quota: log.Quota, Input: log.Input, Output: log.Output}
			var other struct {
				Charge    json.RawMessage `json:"user_charge_cny"`
				Breakdown struct {
					Charge  json.RawMessage `json:"user_charge_cny"`
					Version string          `json:"retail_price_version"`
				} `json:"acu_cost_breakdown"`
			}
			if json.Unmarshal([]byte(log.Other), &other) == nil {
				row.Charge, row.PriceVersion = other.Charge, other.Breakdown.Version
				if len(row.Charge) == 0 || string(row.Charge) == "null" {
					row.Charge = other.Breakdown.Charge
				}
			}
			rows = append(rows, row)
		}
		result["logs"], result["total"] = rows, logs.Total
	}
	// A response begun under an old login must never populate a new account page.
	host.account.mu.Lock()
	valid := host.account.view.LoggedIn && host.account.userID == userID && host.account.bearer == bearer
	host.account.mu.Unlock()
	if !valid {
		host.json(response, 409, map[string]string{"error": "账户状态已变化，请刷新。"})
		return
	}
	result["warnings"] = warnings
	host.json(response, 200, result)
}
