package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mirror metadata locates bytes; only the OS-verified OpenAI signature grants
// installation trust. The official small update manifest detects mirror lag.
type chatGPTMirrorRelease struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Sources     struct {
		Windows struct {
			Architectures map[string]struct {
				Version       string `json:"version"`
				ContentLength int64  `json:"contentLength"`
				Downloadable  bool   `json:"downloadable"`
				Catalog       struct {
					Hash string `json:"hash"`
				} `json:"catalog"`
			} `json:"architectures"`
		} `json:"windows"`
	} `json:"sources"`
}

type chatGPTDownloadChoice struct {
	URL, Version, SHA256 string
	Size                 int64
}

func readChatGPTMetadata(ctx context.Context, client *http.Client, source string, value any) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cache-Control", "no-cache")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("版本信息暂不可用")
	}
	return json.NewDecoder(io.LimitReader(res.Body, 256<<10)).Decode(value)
}

func selectChatGPTMirror(ctx context.Context, client *http.Client, arch string) (chatGPTDownloadChoice, error) {
	var official struct {
		BuildVersion    string `json:"buildVersion"`
		PackageIdentity string `json:"packageIdentity"`
	}
	officialErr := readChatGPTMetadata(ctx, client, "https://persistent.oaistatic.com/codex-app-prod/windows-store-update.json", &official)
	var manifest chatGPTMirrorRelease
	if err := readChatGPTMetadata(ctx, client, "https://codexapp.agentsmirror.com/latest/manifest", &manifest); err != nil {
		return chatGPTDownloadChoice{}, err
	}
	key := "x64"
	if arch == "arm64" {
		key = "arm64"
	}
	release, ok := manifest.Sources.Windows.Architectures[key]
	hash, err := base64.StdEncoding.DecodeString(release.Catalog.Hash)
	age := time.Since(manifest.GeneratedAt)
	if !ok || !release.Downloadable || err != nil || len(hash) != 32 || release.ContentLength <= 0 || release.ContentLength > 2<<30 {
		return chatGPTDownloadChoice{}, errors.New("镜像版本信息无效")
	}
	if !validAppxVersion(release.Version) || age < -time.Hour || (officialErr != nil && age > 7*24*time.Hour) {
		return chatGPTDownloadChoice{}, errors.New("镜像版本信息已过期")
	}
	if officialErr == nil && (official.PackageIdentity != "OpenAI.Codex" || !validAppxVersion(official.BuildVersion) || release.Version != official.BuildVersion) {
		return chatGPTDownloadChoice{}, errors.New("镜像尚未同步官方最新版，切换官方线路")
	}
	return chatGPTDownloadChoice{URL: "https://codexapp.agentsmirror.com/latest/win-" + key, Version: release.Version, SHA256: hex.EncodeToString(hash), Size: release.ContentLength}, nil
}

func validAppxVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, e := strconv.ParseUint(p, 10, 16); e != nil {
			return false
		}
	}
	return true
}

func verifyChatGPTDownload(path string, choice chatGPTDownloadChoice) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != choice.Size || choice.Size > 2<<30 {
		return errors.New("镜像文件摘要或大小不符")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(h, io.LimitReader(f, (2<<30)+1))
	f.Close()
	if err != nil || hex.EncodeToString(h.Sum(nil)) != choice.SHA256 {
		return errors.New("镜像文件摘要不符")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == "AppxManifest.xml" {
			r, e := f.Open()
			if e != nil {
				return e
			}
			defer r.Close()
			var manifest struct {
				Identity struct {
					Version string `xml:"Version,attr"`
				} `xml:"Identity"`
			}
			if e = xml.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&manifest); e != nil {
				return e
			}
			if manifest.Identity.Version != choice.Version {
				return errors.New("镜像安装包版本与最新清单不一致")
			}
			return nil
		}
	}
	return errors.New("缺少应用清单")
}

func prepareLatestChatGPT(ctx context.Context, platform, arch, target string, progress func(int64, int64), phase func(string)) error {
	client := chatGPTDownloadClient()
	if platform == "windows" {
		choice, err := selectChatGPTMirror(ctx, client, arch)
		if err == nil {
			phase("正在通过国内可访问镜像下载 ChatGPT " + choice.Version + "（约 1 GB），支持续传")
			// A matching signed-package candidate can be reused only after refreshing
			// latest metadata. Never silently install an arbitrary old cache.
			if verifyChatGPTDownload(target, choice) == nil {
				return nil
			}
			if err = downloadChatGPTPackage(ctx, client, choice.URL, target, progress); err == nil {
				if err = verifyChatGPTDownload(target, choice); err == nil {
					return nil
				}
				_ = os.Remove(target + ".download.json")
			} else {
				// Retain same-source progress for the next attempt. Switching to
				// official would overwrite it with a different ETag/source.
				if info, e := os.Stat(target + ".partial"); e == nil && info.Size() > 0 {
					return err
				}
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	pkg, err := chatGPTPackage(platform, arch)
	if err != nil {
		return err
	}
	phase("正在通过 OpenAI 官方线路下载最新 ChatGPT 完整包（约 1 GB），无需 Microsoft Store")
	return downloadChatGPTPackage(ctx, client, pkg.URL, target, progress)
}
