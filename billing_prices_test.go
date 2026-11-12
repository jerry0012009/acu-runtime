package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixturePriceCatalog = `{"success":true,"data":[{"model_name":"fixture","payable":{"input_cny_per_million":0}}]}`

func TestPricesFallbackUsesOfficialSitesWithoutCredentials(t *testing.T) {
	for _, preferred := range []string{billingSite, "https://acu.best"} {
		t.Run(preferred, func(t *testing.T) {
			host := &server{}
			host.account.view.Site = preferred
			host.account.bearer = "must-not-send"
			var sites []string
			host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
				sites = append(sites, r.URL.Scheme+"://"+r.URL.Host)
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("public catalog received credentials")
				}
				if len(sites) == 1 {
					return nil, errors.New("offline")
				}
				return cloudFixture(fixturePriceCatalog), nil
			})
			out := httptest.NewRecorder()
			host.modelPrices(out, httptest.NewRequest("GET", "/runtime/billing/prices", nil))
			if len(sites) != 2 || sites[0] != preferred || sites[1] == preferred || !strings.Contains(out.Body.String(), "fixture") || strings.Contains(out.Body.String(), "warning") {
				t.Fatalf("%v: %s", sites, out.Body.String())
			}
		})
	}
}

func TestSlowPriceSiteDoesNotBlockAlternate(t *testing.T) {
	host := &server{}
	canceled := make(chan struct{})
	host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "console.acucompute.com" {
			<-r.Context().Done()
			close(canceled)
			return nil, r.Context().Err()
		}
		return cloudFixture(fixturePriceCatalog), nil
	})
	start := time.Now()
	out := httptest.NewRecorder()
	host.modelPrices(out, httptest.NewRequest("GET", "/runtime/billing/prices", nil))
	if time.Since(start) > 2*time.Second || !strings.Contains(out.Body.String(), "fixture") {
		t.Fatalf("slow fallback: %s", out.Body.String())
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("losing request was not canceled")
	}
}

func TestPricesPersistAndReadCacheWithoutWaitingForNetwork(t *testing.T) {
	file := filepath.Join(t.TempDir(), "model-prices.json")
	host := &server{}
	host.prices.File = file
	host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) { return cloudFixture(fixturePriceCatalog), nil })
	host.modelPrices(httptest.NewRecorder(), httptest.NewRequest("GET", "/runtime/billing/prices", nil))
	restarted := &server{}
	restarted.prices.File = file
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	restarted.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "console.acucompute.com" {
			close(started)
		}
		<-release
		return nil, errors.New("offline")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() {
		restarted.modelPrices(httptest.NewRecorder(), httptest.NewRequest("GET", "/runtime/billing/prices?refresh=1", nil).WithContext(ctx))
		close(finished)
	}()
	<-started
	out := httptest.NewRecorder()
	restarted.modelPrices(out, httptest.NewRequest("GET", "/runtime/billing/prices?cached=1", nil))
	if !strings.Contains(out.Body.String(), "fixture") {
		t.Fatal("restart lost persisted catalog")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("canceled local request stuck")
	}
}

func TestPricesCoalesceConcurrentRefreshes(t *testing.T) {
	host := &server{}
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	host.cloudTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return cloudFixture(fixturePriceCatalog), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		host.modelPrices(httptest.NewRecorder(), httptest.NewRequest("GET", "/?refresh=1", nil).WithContext(ctx))
		close(done)
	}()
	<-started
	// A canceled caller must reuse the in-flight fetch, not launch another.
	cancel()
	<-done
	for i := 0; i < 10; i++ {
		host.modelPrices(httptest.NewRecorder(), httptest.NewRequest("GET", "/?refresh=1", nil).WithContext(ctx))
	}
	host.prices.Lock()
	refreshDone := host.prices.inflight
	host.prices.Unlock()
	close(release)
	<-refreshDone
	if calls.Load() != 1 {
		t.Fatalf("duplicate requests: %d", calls.Load())
	}
}

func TestInvalidPricesNeverReplaceGoodCatalog(t *testing.T) {
	for _, body := range []string{`{"success":true,"data":[]}`, `{"success":true,"data":[{}]}`, `{"success":false,"data":[{"model_name":"bad"}]}`, `<html>verify</html>`} {
		host := &server{}
		host.prices.priceSnapshot = priceSnapshot{Models: []byte(`[{"model_name":"saved"}]`), Updated: time.Now().Unix()}
		host.cloudTransport = accountTransport(func(*http.Request) (*http.Response, error) { return cloudFixture(body), nil })
		out := httptest.NewRecorder()
		host.modelPrices(out, httptest.NewRequest("GET", "/?refresh=1", nil))
		if !strings.Contains(out.Body.String(), "saved") || !strings.Contains(out.Body.String(), "warning") {
			t.Fatal(out.Body.String())
		}
	}
}

func TestPricesIgnoreCorruptAndExpiredDiskCache(t *testing.T) {
	for _, body := range []string{`broken`, `{"models":[{"model_name":"old"}],"updatedAt":1}`, `{"models":[null],"updatedAt":1}`} {
		host := &server{}
		host.prices.File = filepath.Join(t.TempDir(), "prices.json")
		if err := os.WriteFile(host.prices.File, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		out := httptest.NewRecorder()
		host.modelPrices(out, httptest.NewRequest("GET", "/?cached=1", nil))
		if !strings.Contains(out.Body.String(), `"models":null`) {
			t.Fatal(out.Body.String())
		}
	}
}

func TestLivePublicPrices(t *testing.T) {
	if os.Getenv("ACU_PRICE_LIVE_ORIGIN") != "1" {
		t.Skip("explicit read-only origin probe")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.1:443")
	}
	defer transport.CloseIdleConnections()
	host := &server{cloudTransport: transport}
	models, err := host.fetchPrices(context.Background(), billingSite)
	if err != nil || !validPriceModels(models) {
		t.Fatalf("live catalog: %v", err)
	}
	t.Logf("validated real public catalog: %d bytes", len(models))
}
