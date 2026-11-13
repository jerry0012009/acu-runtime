package main

import (
	"context"
	"errors"
	"net/http"
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
func TestUnconfirmedOrCurrentDesktopHasNoUpdateOffer(t *testing.T) {
	for _, tc := range []struct {
		latest string
		err    error
		want   bool
	}{{"26.1112.10.0", nil, true}, {"26.1106.6628.0", nil, false}, {"26.1101.1.0", nil, false}, {"", errors.New("offline"), false}} {
		host := &server{}
		probes := 0
		host.agentVersions.probe = func(context.Context) map[string]string {
			probes++
			return map[string]string{"chatgpt": "26.1106.6628.0"}
		}
		host.agentVersions.latest = func(context.Context, string) (string, error) { return tc.latest, tc.err }
		got := host.desktopAgentVersions(context.Background())["chatgpt"]
		if got.Update != tc.want {
			t.Fatalf("unexpected update: %+v", got)
		}
		host.desktopAgentVersions(context.Background())
		if probes != 1 {
			t.Fatal("version check did not reuse its bounded cache")
		}
	}
}
func TestClientUpdateDoesNotConfigureAnAlreadyCurrentAgent(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("client update touched model configuration") }))
	host.agentVersions.probe = func(context.Context) map[string]string { return map[string]string{"chatgpt": "26.1106.6628.0"} }
	host.agentVersions.latest = func(context.Context, string) (string, error) { return "26.1106.6628.0", nil }
	host.account.view.Selected = 9
	host.runAgentInstall(context.Background(), "codex", "", "", true)
	if host.agentInstall.snapshot().Phase != "complete" || host.account.view.Selected != 9 {
		t.Fatal("current desktop update did not preserve configured account")
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
