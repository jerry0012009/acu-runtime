package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKernelUpgradeRejectsUnknownConcurrency(t *testing.T) {
	for _, body := range []string{`broken`, `{"object":"concurrency","requests":0}`, `{"object":"unrelated","requests":0,"tools":0}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/magpie/concurrency" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			t.Setenv("MAGPIE_ADDR", strings.TrimPrefix(server.URL, "http://"))
			if err := waitGatewayIdle(context.Background()); err == nil {
				t.Fatal("unknown request state allowed an upgrade")
			}
		})
	}
}

func TestKernelUpgradeBusyRequestsOrToolsCannotBeTreatedAsIdle(t *testing.T) {
	for _, body := range []string{
		`{"object":"concurrency","requests":1,"waiting":0,"tools":0}`,
		`{"object":"concurrency","requests":0,"waiting":1,"tools":0}`,
		`{"object":"concurrency","requests":0,"waiting":0,"tools":1}`,
	} {
		t.Run(body, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
				cancel()
			}))
			defer server.Close()
			t.Setenv("MAGPIE_ADDR", strings.TrimPrefix(server.URL, "http://"))
			if err := waitGatewayIdle(ctx); err == nil {
				t.Fatal("active work was accepted for kernel replacement")
			}
		})
	}
}
