package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type servingKernel struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	PID     uint32 `json:"pid"`
}

func gatewayBase() string {
	if value := os.Getenv("MAGPIE_ADDR"); value != "" {
		if host, port, err := net.SplitHostPort(value); err == nil && loopback(host) {
			if n, e := strconv.Atoi(port); e == nil && n > 0 && n <= 65535 {
				return "http://" + net.JoinHostPort(host, port)
			}
		}
	}

	port := 3425
	home, _ := os.UserHomeDir()
	var settings struct {
		Port int `json:"gatewayPort"`
	}
	b, _ := os.ReadFile(filepath.Join(home, ".config", "magpie", "settings.json"))
	if json.Unmarshal(b, &settings) == nil && settings.Port >= 1024 && settings.Port <= 65535 {
		port = settings.Port
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}
func readServingKernel(ctx context.Context) (servingKernel, error) {
	var info servingKernel
	c := &http.Client{Timeout: 2 * time.Second}
	r, e := http.NewRequestWithContext(ctx, "GET", gatewayBase()+"/", nil)
	if e != nil {
		return info, e
	}
	res, e := c.Do(r)
	if e != nil {
		return info, e
	}
	defer res.Body.Close()
	e = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&info)
	if e != nil || info.Name != "magpie" || info.Version == "" {
		return info, errors.New("无法确认运行中的内核版本")
	}
	return info, nil
}
func waitGatewayIdle(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	quiet := 0
	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", gatewayBase()+"/v1/magpie/concurrency", nil)
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		var b struct {
			Object   string `json:"object"`
			Requests *int   `json:"requests"`
			Waiting  *int   `json:"waiting"`
			Tools    *int   `json:"tools"`
		}
		err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&b)
		res.Body.Close()
		if err != nil || b.Object != "concurrency" || b.Requests == nil || b.Tools == nil {
			return errors.New("无法确认当前请求状态，暂不重启内核")
		}
		if *b.Requests == 0 && *b.Tools == 0 && (b.Waiting == nil || *b.Waiting == 0) {
			quiet++
		} else {
			quiet = 0
		}
		if quiet >= 3 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
func (kernel *kernelState) ensureCompatibleGateway(ctx context.Context) error {
	if !kernel.enforceVersion {
		return nil
	}
	info, err := readServingKernel(ctx)
	if err != nil {
		return nil
	}
	if !desktopVersionNewer(kernelVersion, info.Version) {
		kernel.mu.Lock()
		owned := kernel.process != nil && kernel.process.Process != nil && uint32(kernel.process.Process.Pid) == info.PID
		kernel.mu.Unlock()
		if runtime.GOOS == "windows" && !owned {
			return retireCompatibleGateway(ctx, info)
		}
		return nil
	}
	kernel.mu.Lock()
	kernel.phase = "正在更新配套内核 " + info.Version + " → " + strings.TrimPrefix(kernelVersion, "v") + "；等待当前请求完成后自动连接"
	kernel.mu.Unlock()
	return upgradeOldGateway(ctx, info)
}
