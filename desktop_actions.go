package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type desktopAction struct {
	Action string `json:"action"`
	Query  string `json:"query,omitempty"`
	Height int    `json:"height,omitempty"`
}

func cleanDesktopQuery(raw string) (string, bool) {
	if len(raw) > 2048 {
		return "", false
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", false
	}
	allowedViews := map[string]bool{"": true, "agents": true, "overview": true, "usage": true, "routing": true, "providers": true, "settings": true, "library": true, "plugins": true, "masking": true}
	if !allowedViews[values.Get("view")] {
		return "", false
	}
	clean := url.Values{}
	for _, key := range []string{"view", "tab", "req", "provider", "computer", "period", "day", "newgroup", "q"} {
		if value := values.Get(key); value != "" {
			clean.Set(key, value)
		}
	}
	return clean.Encode(), true
}

func (host *server) desktopAction(response http.ResponseWriter, request *http.Request) {
	var action desktopAction
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&action) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		host.json(response, 400, map[string]string{"error": "invalid desktop action"})
		return
	}
	query, valid := cleanDesktopQuery(action.Query)
	if !valid || (action.Action != "main" && action.Action != "panel" && action.Action != "hide" && action.Action != "fit") || (action.Action == "fit" && (action.Height < 1 || action.Height > 100000)) {
		host.json(response, 400, map[string]string{"error": "unsupported desktop action"})
		return
	}
	action.Query = query
	if !host.desktop || host.desktopActions == nil {
		host.json(response, 409, map[string]string{"error": "请在 ACU Runtime 桌面应用中操作"})
		return
	}
	select {
	case host.desktopActions <- action:
		host.json(response, 200, map[string]bool{"ok": true})
	default:
		host.json(response, 503, map[string]string{"error": "窗口正在处理请求，请重试"})
	}
}
