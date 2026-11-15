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
		"windows-amd64": "acf181e09a2ca8825a338db659c970f92b60c4c044aff466cafe70dda324a12c",
		"darwin-amd64":  "7421a5fb8937ac22facce31d667876f42accf78ce12a610c89576714a5b94082",
		"darwin-arm64":  "c63b7a70c01a2049e8902505383e01107c0f22865778bced4bc51d458820978c",
		"linux-amd64":   "561ac5f11a273367d6f686c7e4780cb36b132d1a12a9eea0ab4c349511401d12"}
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
