package main

import (
	"context"
	"errors"
)

const videoReadingName = "acu-video-reader"

// Runtime presents the workflow; the kernel owns dependencies and MCP processes.
// Unsupported kernels fail explicitly, without a Runtime implementation fallback.
func (host *server) prepareVideoTools(ctx context.Context) error {
	var result struct {
		Schema int  `json:"schema"`
		Ready  bool `json:"ready"`
	}
	if err := host.kernel.exchange(ctx, "POST", "/api/library/video/prepare", map[string]any{}, &result); err != nil {
		return errors.New("视频读取未准备完成，请使用支持视频读取的配套 Magpie 内核并重试")
	}
	if result.Schema != 1 || !result.Ready {
		return errors.New("Magpie 尚未确认视频读取就绪")
	}
	return nil
}

func (host *server) assignVideoReading(ctx context.Context, agents []string) error {
	if len(agents) == 0 {
		return nil
	}
	if err := host.prepareVideoTools(ctx); err != nil {
		return err
	}
	return host.assignNativeTool(ctx, videoReadingName, "video", agents)
}
