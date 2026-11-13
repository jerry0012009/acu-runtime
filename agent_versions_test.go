package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDesktopVersionsCompareActualBuilds(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{{"26.1112.10.0", "26.1106.6628.0", true}, {"26.1106.6628.0", "26.1112.10.0", false}, {"26.1106.6628.0", "26.1106.6628.0", false}, {"5.9.4", "5.9.3", true}, {"", "26.1106.6628.0", false}, {"unknown", "unknown", false}} {
		if got := desktopVersionNewer(tc.latest, tc.current); got != tc.want {
			t.Fatalf("%s vs %s: %v", tc.latest, tc.current, got)
		}
	}
}

func TestDesktopUpdateVerifiesRegistrationAndInvalidatesOldCache(t *testing.T) {
	for _, tc := range []struct {
		current   string
		wantError bool
	}{
		{"26.1106.6628.0", true}, {"", true}, {"unknown", true}, {"26.1111.2228.0", false}, {"26.1112.1.0", false},
	} {
		t.Run(tc.current, func(t *testing.T) {
			host := &server{}
			host.agentVersions.at = time.Now()
			host.agentVersions.desktop = map[string]desktopVersion{"chatgpt": {Version: "26.1106.6628.0", Update: true}}
			host.agentVersions.probe = func(context.Context) map[string]string { return map[string]string{"chatgpt": tc.current} }
			host.agentVersions.latest = func(context.Context, string) (string, error) { return "26.1111.2228.0", nil }
			got, err := host.verifyDesktopUpdate(context.Background(), "chatgpt", "26.1111.2228.0")
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected validation %v %+v", err, got)
			}
			if !host.agentVersions.at.IsZero() {
				t.Fatal("old cache retained")
			}
			if !tc.wantError {
				fresh := host.desktopAgentVersions(context.Background())["chatgpt"]
				if fresh.Update || fresh.Version != tc.current {
					t.Fatalf("stale update offer %+v", fresh)
				}
			}
		})
	}
}
func TestChatGPTInstalledVersionNeverConsultsAnIndependentUpdateFeed(t *testing.T) {
	host := &server{}
	probes := 0
	host.agentVersions.probe = func(context.Context) map[string]string {
		probes++
		return map[string]string{"chatgpt": "26.1106.6628.0"}
	}
	host.agentVersions.latest = func(context.Context, string) (string, error) {
		t.Fatal("ChatGPT desktop queried an independent latest feed")
		return "26.1111.2228.0", nil
	}
	got := host.desktopAgentVersions(context.Background())["chatgpt"]
	if got.Version != "26.1106.6628.0" || !got.Installed || got.Update || got.Latest != "" || got.Error != "" {
		t.Fatalf("unsolicited desktop update: %+v", got)
	}
	host.desktopAgentVersions(context.Background())
	if probes != 1 {
		t.Fatal("version probe cache not reused")
	}
}

func TestAgentVersionEndpointPreservesMagpieUpdateDecision(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprint(update), func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/agents/cli" || r.Method != "GET" {
					t.Fatalf("unexpected kernel call %s %s", r.Method, r.URL.Path)
				}
				json.NewEncoder(w).Encode(map[string]any{"agents": map[string]any{"codex": map[string]any{"version": "0.167.0", "latest": "0.168.0", "update": update, "via": "npm", "app": "26.1106.6628.0"}}, "pending": false})
			}))
			host.agentVersions.probe = func(context.Context) map[string]string { return map[string]string{"chatgpt": "26.1106.6628.0"} }
			host.agentVersions.latest = func(context.Context, string) (string, error) {
				t.Fatal("independent ChatGPT feed called")
				return "", nil
			}
			res := httptest.NewRecorder()
			host.agentVersionStatus(res, httptest.NewRequest("GET", "/runtime/agents/versions", nil))
			var got struct {
				Agents map[string]struct {
					Update                    bool
					Version, Latest, Via, App string
				}
			}
			if json.Unmarshal(res.Body.Bytes(), &got) != nil || res.Code != 200 {
				t.Fatal("invalid version response")
			}
			if got.Agents["codex"].Update != update || got.Agents["codex"].Via != "npm" || got.Agents["codex"].App != "26.1106.6628.0" {
				t.Fatalf("Magpie decision changed: %+v", got)
			}
			if got.Agents["chatgpt"].Update || got.Agents["chatgpt"].Latest != "" {
				t.Fatal("desktop update injected")
			}
		})
	}
}
func TestChatGPTDesktopUpdateCannotBypassNativeUpdateBoundary(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("client update touched model configuration") }))
	host.agentVersions.probe = func(context.Context) map[string]string { t.Fatal("disabled updater probed desktop"); return nil }
	host.account.view.Selected = 9
	host.desktop = true
	req := httptest.NewRequest("POST", "/runtime/agents/install", strings.NewReader(`{"agent":"codex","update":true}`))
	res := httptest.NewRecorder()
	host.agentInstaller(res, req)
	if res.Code != 400 || host.agentInstall.running {
		t.Fatalf("desktop update accepted: %d", res.Code)
	}
	host.runAgentInstall(context.Background(), "codex", "", "", true)
	if host.agentInstall.snapshot().Phase != "error" || host.account.view.Selected != 9 {
		t.Fatal("disabled desktop update did not preserve configured account")
	}
}

func TestWorkBuddyBuildCommitResolvesFullVersion(t *testing.T) {
	for _, tc := range []struct {
		commit    string
		wantError bool
	}{{"d4692421", false}, {"", true}, {"12345678", true}} {
		host := &server{}
		host.agentVersions.probe = func(context.Context) map[string]string {
			return map[string]string{"workbuddy-desktop": "5.9.3", "workbuddy-commit": tc.commit}
		}
		host.agentVersions.latest = func(context.Context, string) (string, error) { return "5.9.3.42398134+d4692421", nil }
		got := host.desktopAgentVersions(context.Background())["workbuddy-desktop"]
		if got.Update || (got.Error != "") != tc.wantError {
			t.Fatalf("unreliable build comparison: %+v", got)
		}
		if !tc.wantError && got.Version != got.Latest {
			t.Fatalf("matching official build not resolved: %+v", got)
		}
	}
}
