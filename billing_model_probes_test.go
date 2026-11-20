package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelProbeSummaryReusesOnePublicFetchAndExpiresStaleData(t *testing.T) {
	host := &server{host: "runtime.test"}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	snapshot := fmt.Sprintf(`{"success":true,"data":{"generatedAt":%d,"models":{"model":{"buckets":"grnnnnng","latencyMs":2000,"protocol":"responses"}}}}`, time.Now().Unix())
	host.cloudTransport = accountTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != billingSite+"/api/pricing/model-probe-summary" || request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
			t.Error("unexpected probe request destination or credentials")
		}
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return cloudFixture(snapshot), nil
		}
		return cloudFixture(`{"success":true,"data":{"generatedAt":0,"models":{}}}`), nil
	})
	read := func(ctx context.Context) *httptest.ResponseRecorder {
		out := httptest.NewRecorder()
		host.ServeHTTP(out, httptest.NewRequest("GET", "http://runtime.test/runtime/billing/model-probe-summary", nil).WithContext(ctx))
		return out
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() { read(ctx); close(finished) }()
	<-started
	cancel()
	<-finished
	read(ctx)
	close(release)
	check := func(wantData, stale bool) {
		t.Helper()
		out := read(context.Background())
		var result struct {
			billingProbeSnapshot
			Stale bool `json:"stale"`
		}
		if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil || out.Code != 200 || (result.Models["model"].LatencyMs == 2000) != wantData || result.Stale != stale {
			t.Fatalf("unexpected result (%v): %s", err, out.Body.String())
		}
	}
	check(true, false)
	check(true, false)
	if calls.Load() != 1 {
		t.Fatal("summary fetch was not coalesced or cached")
	}
	host.modelProbes.nextFetch = time.Time{}
	check(true, true)
	check(true, true)
	if calls.Load() != 2 {
		t.Fatal("failed refresh bypassed retry backoff")
	}
	host.modelProbes.snapshot.GeneratedAt = time.Now().Add(-billingProbeMaxAge).Unix()
	check(false, false)
}
