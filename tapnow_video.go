package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func tapnowVideoURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || (u.Host != "files.tapnow.media" && u.Host != "files.tapnow.top") {
		return nil, errors.New("视频来源不在 TapNow 素材域名内")
	}
	return u, nil
}

func tapnowVideoClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("video redirects refused") }, Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
					return nil, errors.New("private video address refused")
				}
			}
			for _, ip := range ips {
				conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, errors.New("video host unavailable")
		},
	}}
}

type tapnowVideoReply struct {
	OK            bool              `json:"ok"`
	Error         string            `json:"error,omitempty"`
	Content       []videoMCPContent `json:"content,omitempty"`
	Omitted       int               `json:"omitted"`
	AudioAnalyzed bool              `json:"audioAnalyzed"`
}

// Native Messaging carries a URL and bounded samples, never browser credentials
// or video bytes. This action is unavailable to the backup edition.
func runTapnowVideo(ctx context.Context, origin string, msg tapnowNativeMessage) tapnowVideoReply {
	fail := func(message string) tapnowVideoReply { return tapnowVideoReply{Error: message} }
	if tapnowOriginEdition(origin) != "companion" {
		return fail("备份插件不支持视频审阅")
	}
	u, err := tapnowVideoURL(msg.URL)
	if err != nil {
		return fail(err.Error())
	}
	root, err := videoReadingDirectory()
	if err != nil {
		return fail("视频工具目录不可用")
	}
	if _, err := os.Stat(filepath.Join(root, ".ready")); err != nil {
		return fail("请先在 ACU Runtime 一键配置视频读取")
	}
	node, _ := videoNodePaths(root)
	dir, err := os.MkdirTemp("", "acu-tapnow-video-")
	if err != nil {
		return fail("无法准备临时视频")
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "input.mov")
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return fail("视频地址无效")
	}
	req.Header.Set("Referer", "https://app.tapnow.ai/")
	res, err := tapnowVideoClient().Do(req)
	if err != nil {
		return fail("视频下载失败，请重试")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")), "video/") {
		return fail("TapNow 未返回可读取的视频")
	}
	const maxVideo = 100 << 20
	if res.ContentLength > maxVideo {
		return fail("视频超过 100 MiB，暂未读取")
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail("无法保存临时视频")
	}
	n, copyErr := io.Copy(f, io.LimitReader(res.Body, maxVideo+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n == 0 || n > maxVideo {
		return fail("视频下载不完整或超过 100 MiB")
	}
	content, err := readVideoFrames(ctx, videoReadingInstall{Node: node, Root: root}, file)
	if err != nil {
		return fail(err.Error())
	}
	reply := tapnowVideoReply{OK: true, Content: []videoMCPContent{}}
	bytes := 0
	for _, part := range content {
		encoded, _ := json.Marshal(part)
		if bytes+len(encoded) > 750<<10 {
			if part.Type == "image" {
				reply.Omitted++
			}
			continue
		}
		reply.Content = append(reply.Content, part)
		bytes += len(encoded)
	}
	images := 0
	for _, part := range reply.Content {
		if part.Type == "image" {
			images++
		}
	}
	if images == 0 {
		return fail("未取得视频画面，不能进行视觉评价")
	}
	return reply
}
