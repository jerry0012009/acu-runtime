package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Spawned by scripts/test-tapnow-native.cjs with an isolated HOME and browser.
func TestTapnowNativeBrowserHarness(t *testing.T) {
	if os.Getenv("ACU_TAPNOW_BROWSER_FIXTURE") != "1" {
		t.Skip("isolated browser harness only")
	}
	fixtureTapnowIDs(t)
	tapnowCompanionID = os.Getenv("ACU_TAPNOW_COMPANION_ID")
	tapnowBackupID = os.Getenv("ACU_TAPNOW_BACKUP_ID")
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/providers":
			io.WriteString(w, `{"providers":[{"id":"acu","responses":"https://acu.best/v1","key":{"set":true,"masked":"***"},"models":[{"id":"acu-auto","on":true}]}]}`)
		case "/api/provider/models":
			io.WriteString(w, `{}`)
		case "/api/provider/key":
			io.WriteString(w, `{"key":"sk-browser-native-fixture"}`)
		default:
			io.WriteString(w, `{"agents":[]}`)
		}
	}))
	host.desktop = true
	host.prepareVideoReading = func(context.Context) (videoReadingInstall, error) {
		return videoReadingInstall{Node: "fixture", Root: t.TempDir()}, nil
	}
	host.tapnow.register = func() error { return nil }
	host.tapnow.openStore = func(context.Context, string) error { return nil }
	host.token = "isolated-native-browser-session"
	service := httptest.NewUnstartedServer(host)
	host.host = service.Listener.Addr().String()
	service.Start()
	defer service.Close()
	fmt.Printf("TAPNOW_FIXTURE_READY=%s\n", service.URL)
	done := filepathForTapnowSmoke(t)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(done); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("native browser fixture timed out")
}

func filepathForTapnowSmoke(t *testing.T) string {
	t.Helper()
	file := os.Getenv("ACU_TAPNOW_SMOKE_DONE")
	if file == "" || !strings.Contains(file, "tapnow-native-") {
		t.Fatal("invalid smoke sentinel")
	}
	return file
}
