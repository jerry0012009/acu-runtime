package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type videoPackage struct{ URL, SHA256 string }

func videoMCPRelease(platform, arch string) (videoPackage, error) {
	values := map[string]string{
		"windows-amd64": "85d9142e17b27e6e506be43fdfb31f665bb1a9d2d2a0970a491b045c8bbc18c9",
		"darwin-amd64":  "fbca746c3e6dfe594e20b2d39cc8eb955a32eb65649f0b6f5bef5af4ee951246",
		"darwin-arm64":  "c8572cebafe035d599f5e0422e7c4e4b256be4e5cda322c9a2db63ea156de537",
		"linux-amd64":   "d0b98aa28109d7d682941ae916a938df0e0f83286ea436a13144966c6dde2a89"}
	key := platform + "-" + arch
	digest, ok := values[key]
	if !ok {
		return videoPackage{}, errors.New("当前平台暂无自包含视频工具")
	}
	name := "acu-video-mcp-" + key
	if platform == "windows" {
		name += ".exe"
	}
	return videoPackage{URL: "https://github.com/jerry0012009/acu-video-mcp/releases/download/v" + videoMCPVersion + "/" + name, SHA256: digest}, nil
}
func verifyVideoMCP(file, digest string) bool {
	if len(digest) != 64 {
		return false
	}
	f, e := os.Open(file)
	if e != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, (256<<20)+1))
	return e == nil && n <= 256<<20 && hex.EncodeToString(h.Sum(nil)) == digest
}
func installVideoMCP(ctx context.Context, p videoPackage, target string) (string, error) {
	client := updateClient(3 * time.Minute)
	defer client.CloseIdleConnections()
	var err error
	for _, source := range updateDownloadSources(p.URL) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		attempt, cancel := context.WithTimeout(ctx, 2*time.Minute)
		var installed string
		installed, err = downloadVideoMCP(attempt, client, videoPackage{URL: source, SHA256: p.SHA256}, target)
		cancel()
		if err == nil {
			return installed, nil
		}
	}
	return "", err
}

func downloadVideoMCP(ctx context.Context, client *http.Client, p videoPackage, target string) (string, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if e != nil {
		return "", e
	}
	res, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.ContentLength > 256<<20 {
		return "", errors.New("视频工具下载失败")
	}
	if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(filepath.Dir(target), ".video-download-")
	if e != nil {
		return "", e
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	last := -1
	writer := &kernelDownloadWriter{writer: io.MultiWriter(f, h), total: res.ContentLength, progress: func(done, total int64) {
		if total > 0 {
			percent := int(done * 100 / total)
			if percent != last {
				last = percent
				videoProgress(ctx, fmt.Sprintf("正在下载视频组件 · %d%%", percent))
			}
		}
	}}
	n, e := io.Copy(writer, io.LimitReader(res.Body, (256<<20)+1))
	ce := f.Close()
	if e != nil || ce != nil || n > 256<<20 || hex.EncodeToString(h.Sum(nil)) != p.SHA256 {
		return "", errors.New("视频工具校验失败，原文件保留")
	}
	if e := os.Chmod(f.Name(), 0700); e != nil {
		return "", e
	}
	if e := replaceInstalledPath(f.Name(), target); e != nil {
		return "", e
	}
	return target, nil
}
