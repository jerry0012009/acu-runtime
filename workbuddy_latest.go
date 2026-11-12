package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func latestWorkBuddyPackage(ctx context.Context, platform, arch string) (agentPackage, error) {
	pinned, err := workBuddyPackage(platform, arch)
	if err != nil {
		return pinned, err
	}
	flavor := "win32-x64-user"
	if platform == "darwin" {
		flavor = "darwin-x64"
		if arch == "arm64" {
			flavor = "darwin-arm64"
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", "https://www.workbuddy.cn/v2/update?platform=workbuddy-"+flavor, nil)
	req.Header.Set("Cache-Control", "no-cache")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = updateProxy
	client := &http.Client{Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || r.URL.Scheme != "https" || r.URL.Host != "www.workbuddy.cn" {
			return errors.New("WorkBuddy 版本线路无效")
		}
		return nil
	}}
	response, err := client.Do(req)
	if err != nil {
		return agentPackage{}, errors.New("WorkBuddy 最新版本检查暂不可用，请重试")
	}
	defer response.Body.Close()
	var release struct {
		URL    string `json:"url"`
		SHA256 string `json:"sha256hash"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&release) != nil {
		return agentPackage{}, errors.New("WorkBuddy 版本信息无效")
	}
	u, err := url.Parse(release.URL)
	if err != nil || !trustedWorkBuddyURL(release.URL) || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/workbuddy/saas/"+flavor+"/WorkBuddy-"+flavor+"-") || path.Ext(u.Path) != path.Ext(pinned.Name) || !digestPattern.MatchString(release.SHA256) {
		return agentPackage{}, errors.New("WorkBuddy 官方安装包信息无效")
	}
	return agentPackage{URL: release.URL, SHA256: release.SHA256, Name: path.Base(u.Path)}, nil
}
