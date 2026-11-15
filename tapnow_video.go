package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type tapnowVideoReply struct {
	OK            bool              `json:"ok"`
	Error         string            `json:"error,omitempty"`
	Content       []json.RawMessage `json:"content,omitempty"`
	Omitted       int               `json:"omitted"`
	AudioAnalyzed bool              `json:"audioAnalyzed"`
}

// Authorize the extension and forward one fixed operation. URL validation,
// downloads, temporary files, sampling and MCP execution are owned by Magpie.
func (host *server) tapnowVideoFrames(w http.ResponseWriter, r *http.Request, msg tapnowNativeMessage) {
	if tapnowOriginEdition(msg.Origin) != "companion" {
		host.json(w, 403, tapnowVideoReply{Error: "备份插件不支持视频审阅"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 95*time.Second)
	defer cancel()
	var result struct {
		Schema int              `json:"schema"`
		Result tapnowVideoReply `json:"result"`
	}
	if err := host.kernel.exchange(ctx, "POST", "/api/library/video/frames", map[string]string{"url": msg.URL}, &result); err != nil {
		host.json(w, 200, tapnowVideoReply{Error: "Magpie 视频读取暂不可用，请检查配套内核"})
		return
	}
	if result.Schema != 1 {
		host.json(w, 200, tapnowVideoReply{Error: "Magpie 视频读取协议不兼容"})
		return
	}
	host.json(w, 200, result.Result)
}
