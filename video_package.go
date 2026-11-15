package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type videoPackage struct{ URL, SHA256 string }

func videoMCPRelease(platform, arch string) (videoPackage, error) {
	values := map[string]string{
		"windows-amd64": "af238e0170ca4d58493e387196f8eccda8732f25b0e44333f50322698088eaff",
		"darwin-amd64":  "dac882fda66fb4e3ba37edb4f9a12fc3d690615828a9b096216b2e5bc7148a47",
		"darwin-arm64":  "1a758205db5fb9c79e2d19b5a1c72589bcda3a2442fca65a169fa792196ca4ff",
		"linux-amd64":   "0e61d0529c428f8d4dc5a2be46bc0445247613155b2560fd88a4108dd4ea8f9d"}
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
	req, e := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if e != nil {
		return "", e
	}
	res, e := tapnowHTTPClient().Do(req)
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
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, (256<<20)+1))
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
