package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type desktopVersion struct {
	Version   string `json:"version"`
	Latest    string `json:"latest,omitempty"`
	Update    bool   `json:"update"`
	Installed bool   `json:"installed"`
	Error     string `json:"error,omitempty"`
	Desktop   bool   `json:"desktop"`
}
type agentVersionCache struct {
	sync.Mutex
	at      time.Time
	desktop map[string]desktopVersion
	probe   func(context.Context) map[string]string
	latest  func(context.Context, string) (string, error)
}

var versionDigits = regexp.MustCompile(`\d+`)

// Numeric components distinguish Store builds; lexical comparison does not.
func desktopVersionNewer(latest, current string) bool {
	a, b := versionDigits.FindAllString(latest, -1), versionDigits.FindAllString(current, -1)
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y uint64
		if i < len(a) {
			x, _ = strconv.ParseUint(a[i], 10, 32)
		}
		if i < len(b) {
			y, _ = strconv.ParseUint(b[i], 10, 32)
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func latestDesktopVersion(ctx context.Context, id string) (string, error) {
	if id != "workbuddy-desktop" {
		return "", http.ErrNotSupported
	}
	pkg, err := latestWorkBuddyPackage(ctx, "windows", "amd64")
	if err != nil {
		return "", err
	}
	v := strings.TrimSuffix(strings.TrimPrefix(pkg.Name, "WorkBuddy-win32-x64-user-"), ".exe")
	parts := strings.SplitN(v, "-", 2)
	if len(parts) == 2 {
		return parts[0] + "+" + parts[1], nil
	}
	return parts[0], nil
}

func (host *server) desktopAgentVersions(ctx context.Context) map[string]desktopVersion {
	cache := &host.agentVersions
	cache.Lock()
	defer cache.Unlock()
	if time.Since(cache.at) < time.Minute && cache.desktop != nil {
		return cache.desktop
	}
	probe := cache.probe
	if probe == nil {
		probe = installedDesktopVersions
	}
	latest := cache.latest
	if latest == nil {
		latest = latestDesktopVersion
	}
	installed := probe(ctx)
	result := map[string]desktopVersion{}
	for _, id := range []string{"chatgpt", "workbuddy-desktop"} {
		v := installed[id]
		if v == "" {
			continue
		}
		info := desktopVersion{Version: v, Installed: true, Desktop: true}
		// ChatGPT desktop updates belong to its own updater. Magpie's CLI
		// decision is returned separately, without consulting another release feed.
		if id == "chatgpt" {
			result[id] = info
			continue
		}
		next, err := latest(ctx, id)
		if err != nil {
			info.Error = "最新版本暂时无法确认"
		} else {
			version, commit, _ := strings.Cut(next, "+")
			info.Latest = version
			if id == "workbuddy-desktop" && commit != "" && installed["workbuddy-commit"] == commit {
				info.Version = version
				info.Update = false
			} else {
				info.Update = desktopVersionNewer(version, v)
				if id == "workbuddy-desktop" && len(versionDigits.FindAllString(v, -1)) < 4 && strings.HasPrefix(version, v+".") {
					info.Update = false
					info.Error = "当前构建号暂时无法确认"
				}
			}
		}
		result[id] = info
	}
	cache.desktop, cache.at = result, time.Now()
	return result
}

func (host *server) verifyDesktopUpdate(ctx context.Context, id, target string) (desktopVersion, error) {
	cache := &host.agentVersions
	cache.Lock()
	probe := cache.probe
	if probe == nil {
		probe = installedDesktopVersions
	}
	current := probe(ctx)[id]
	// Re-read the registration, not the pre-install snapshot, on the next UI poll.
	cache.at = time.Time{}
	cache.Unlock()
	if !validAppxVersion(current) || desktopVersionNewer(target, current) {
		if current == "" {
			return desktopVersion{}, fmt.Errorf("更新后暂未读到已安装版本，请检查系统安装结果后重新检查更新")
		}
		return desktopVersion{}, fmt.Errorf("更新尚未生效：当前版本 %s，目标版本 %s；请检查系统安装结果后重试", current, target)
	}
	return desktopVersion{Version: current, Latest: target, Installed: true, Desktop: true}, nil
}

func (host *server) agentVersionStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var native map[string]json.RawMessage
	if err := host.kernel.exchange(ctx, "GET", "/api/agents/cli", nil, &native); err != nil {
		native = map[string]json.RawMessage{}
	}
	var agents map[string]json.RawMessage
	json.Unmarshal(native["agents"], &agents)
	if agents == nil {
		agents = map[string]json.RawMessage{}
	}
	for id, version := range host.desktopAgentVersions(ctx) {
		agents[id], _ = json.Marshal(version)
	}
	native["agents"], _ = json.Marshal(agents)
	host.json(w, 200, native)
}
