package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
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
