//go:build desktop && windows

package main

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestNativeLoginSmoke(t *testing.T) {
	if os.Getenv("ACU_NATIVE_LOGIN_SMOKE") != "1" {
		t.Skip("interactive isolated WebView smoke test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		t.Fatal(err)
	}
	defer windows.CoUninitialize()
	cache := os.Getenv("ACU_NATIVE_PROFILE")
	if cache == "" {
		var err error
		cache, err = os.MkdirTemp("", "acu-native-smoke-*")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			// WebView2 releases its metrics file shortly after app.Run returns.
			deadline := time.Now().Add(10 * time.Second)
			for {
				if err := os.RemoveAll(cache); err == nil {
					return
				} else if time.Now().After(deadline) {
					t.Errorf("remove isolated WebView profile: %v", err)
					return
				}
				time.Sleep(200 * time.Millisecond)
			}
		})
	}
	t.Setenv("LOCALAPPDATA", cache)
	restoring := os.Getenv("ACU_NATIVE_RESTORE_SMOKE") == "1"
	var mu sync.Mutex
	var provider map[string]any
	providerWrites, renewals := 0, 0
	expiredWhileLoggedIn := false
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch request.URL.Path {
		case "/api/provider/save":
			providerWrites++
			var input struct {
				Key string `json:"key"`
			}
			json.NewDecoder(request.Body).Decode(&input)
			provider = map[string]any{"id": "acu-runtime", "name": "ACU", "chat": "https://acu.best/v1", "key": map[string]any{"set": true, "masked": "sk-***fixture"}, "models": []map[string]any{{"id": "acu-auto", "on": true}}}
			io.WriteString(response, `{}`)
		case "/api/providers":
			providers := []map[string]any{}
			if provider != nil {
				providers = append(providers, provider)
			}
			json.NewEncoder(response).Encode(map[string]any{"providers": providers})
		case "/api/provider/key":
			io.WriteString(response, `{"key":"fixture-model-key"}`)
		case "/api/state":
			io.WriteString(response, `{"agents":[{"id":"codex","fields":[]},{"id":"claude","fields":[]},{"id":"workbuddy","fields":[]},{"id":"hermes","fields":[]}]}`)
		case "/api/groups", "/api/agents/cli", "/api/agent-models/codex", "/api/agent-models/claude", "/api/agent-models/workbuddy", "/api/agent-models/hermes":
			if request.Method != http.MethodGet {
				t.Errorf("unexpected mutation: %s", request.URL.Path)
			}
			io.WriteString(response, `{}`)
		case "/":
			io.WriteString(response, `<!doctype html><title>隔离内核</title>`)
		case "/favicon.ico":
			response.WriteHeader(404)
		default:
			t.Errorf("unexpected smoke-test kernel write: %s", request.URL.Path)
			response.WriteHeader(400)
		}
	}))
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/user/self/acu-preference-profile":
			return cloudFixture(`{"success":true,"data":{"status":"ready","generatedAt":"2026-11-12T12:00:00Z","profile":{"summary":"偏好可核验的交付。","rays":[{"id":"native-ray","label":"交付验收","category":"交付","summary":"交付应可安装并完成启动验证。","scope":"软件交付","sourceIds":["native-source"]}]}}}`), nil
		case "/api/user/self/acu-preference-library":
			return cloudFixture(`{"success":true,"data":{"version":1,"records":[{"id":"native-source","summary":"交付应可安装并完成启动验证。"}]}}`), nil
		case "/api/user/self/acu-preference-controls":
			return cloudFixture(`{"success":true,"data":{"records":[]}}`), nil
		case "/api/user/self/acu-preference-activity":
			return cloudFixture(`{"success":true,"data":{"version":1,"events":[{"trigger":"explicit_user_preference","recordedAt":"2026-11-12T12:00:00Z","changes":[{"kind":"updated","before":"交付应可安装。","after":"交付应可安装并完成启动验证。"}]}],"partial":false,"moreAvailable":false}}`), nil
		case "/api/user/self/acu-advisor/settings":
			return cloudFixture(`{"success":true,"data":{"learningEnabled":false,"injectionEnabled":false}}`), nil
		case "/api/user/self":
			if request.Header.Get("Authorization") == "Bearer fixture-renewed-access" {
				mu.Lock()
				renewals++
				mu.Unlock()
			}
			return cloudFixture(`{"success":true,"data":{"id":42,"username":"Native fixture"}}`), nil
		case "/api/token/":
			return cloudFixture(`{"success":true,"data":{"total":2,"items":[{"id":7,"name":"First fixture Key","status":1,"unlimited_quota":true},{"id":9,"name":"Second fixture Key","status":1,"unlimited_quota":true}]}}`), nil
		case "/api/token/7/key", "/api/token/9/key":
			return cloudFixture(`{"success":true,"data":{"key":"fixture-model-key"}}`), nil
		case "/v1/models":
			return cloudFixture(`{"data":[{"id":"acu-auto"}]}`), nil
		default:
			t.Errorf("unexpected cloud request: %s", request.URL.Path)
			return cloudFixture(`{}`), nil
		}
	})
	host.desktop = true
	host.accountFile = filepath.Join(cache, "ACURuntime", "account-session.json")
	host.quitRequests = make(chan struct{}, 1)
	host.loginRequests = make(chan loginTicket, 1)
	web, _ := fs.Sub(assets, "web")
	host.web = http.StripPrefix("/ui/", http.FileServer(http.FS(web)))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host.host = listener.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	host.lifecycle = ctx
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if view := host.account.snapshot(); view.LoggedIn && view.Refreshing {
					mu.Lock()
					expiredWhileLoggedIn = true
					mu.Unlock()
				}
			}
		}
	}()
	service := &http.Server{Handler: host, ReadHeaderTimeout: 5 * time.Second}
	defer service.Close()
	go service.Serve(listener)
	port, _ := strconv.Atoi(os.Getenv("ACU_NATIVE_INSPECT_PORT"))
	if err := runDesktop(ctx, host, "http://"+host.host, port, host.quitRequests, cancel); err != nil {
		t.Fatal(err)
	}
	view := host.account.snapshot()
	if !view.LoggedIn || view.Selected != 9 || view.Error != "" {
		t.Fatalf("native login and Key switching did not complete: %+v", view)
	}
	mu.Lock()
	defer mu.Unlock()
	if restoring {
		if providerWrites != 0 || renewals == 0 {
			t.Fatalf("restore rewrote Key or failed to validate renewed identity: writes=%d renewals=%d", providerWrites, renewals)
		}
		return
	}
	if provider == nil {
		t.Fatal("native login never saved the default Key")
	}
	if renewals == 0 || !expiredWhileLoggedIn || providerWrites != 2 {
		t.Fatalf("native background renewal did not survive expiry without rewriting Keys: renewals=%d expiredWhileLoggedIn=%v providerWrites=%d", renewals, expiredWhileLoggedIn, providerWrites)
	}
}
