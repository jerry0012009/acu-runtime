package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const billingProbeTTL = 5 * time.Minute
const billingProbeMaxAge = 30 * time.Minute
const billingProbeTimeout = 3 * time.Second

type billingProbeStats struct {
	Buckets   string `json:"buckets"`
	LatencyMs int    `json:"latencyMs,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
}

type billingProbeSnapshot struct {
	GeneratedAt int64                        `json:"generatedAt"`
	Models      map[string]billingProbeStats `json:"models"`
}

type billingProbeCache struct {
	sync.Mutex
	snapshot  billingProbeSnapshot
	nextFetch time.Time
	inflight  chan struct{}
	stale     bool
}

func (host *server) fetchModelProbes(ctx context.Context) (billingProbeSnapshot, error) {
	var snapshot billingProbeSnapshot
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, billingSite+"/api/pricing/model-probe-summary", nil)
	if err != nil {
		return snapshot, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	transport := http.RoundTripper(priceTransport)
	if host.cloudTransport != nil {
		transport = host.cloudTransport
	}
	client := http.Client{Transport: transport, Timeout: billingProbeTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return snapshot, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return snapshot, errors.New("model probe snapshot unavailable")
	}
	var envelope struct {
		Success bool                 `json:"success"`
		Data    billingProbeSnapshot `json:"data"`
	}
	const limit = 128 << 10
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(raw) > limit || json.Unmarshal(raw, &envelope) != nil || !envelope.Success || envelope.Data.Models == nil {
		return snapshot, errors.New("invalid model probe snapshot")
	}
	delta := time.Since(time.Unix(envelope.Data.GeneratedAt, 0))
	if delta < -time.Minute || delta >= billingProbeMaxAge {
		return snapshot, errors.New("model probe snapshot outside freshness window")
	}
	for _, row := range envelope.Data.Models {
		if len(row.Buckets) != 8 || strings.Trim(row.Buckets, "gnr") != "" || row.LatencyMs < 0 {
			return snapshot, errors.New("invalid model probe data")
		}
	}
	return envelope.Data, nil
}

func (host *server) refreshModelProbes(done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), billingProbeTimeout)
	defer cancel()
	snapshot, err := host.fetchModelProbes(ctx)
	cache := &host.modelProbes
	cache.Lock()
	defer cache.Unlock()
	if err == nil && snapshot.GeneratedAt >= cache.snapshot.GeneratedAt {
		cache.snapshot = snapshot
		cache.stale = time.Since(time.Unix(snapshot.GeneratedAt, 0)) >= 2*billingProbeTTL
		cache.nextFetch = time.Now().Add(billingProbeTTL)
	} else {
		cache.stale = true
		cache.nextFetch = time.Now().Add(time.Minute)
	}
	cache.inflight = nil
	close(done)
}

func (host *server) modelProbeSummary(response http.ResponseWriter, request *http.Request) {
	cache := &host.modelProbes
	cache.Lock()
	if !time.Now().Before(cache.nextFetch) && cache.inflight == nil {
		cache.inflight = make(chan struct{})
		go host.refreshModelProbes(cache.inflight)
	}
	done := cache.inflight
	cache.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-request.Context().Done():
			return
		}
	}
	cache.Lock()
	snapshot, stale := cache.snapshot, cache.stale
	cache.Unlock()
	if time.Since(time.Unix(snapshot.GeneratedAt, 0)) >= billingProbeMaxAge {
		snapshot = billingProbeSnapshot{}
		stale = false
	}
	host.json(response, http.StatusOK, struct {
		billingProbeSnapshot
		Stale bool `json:"stale,omitempty"`
	}{snapshot, stale})
}
