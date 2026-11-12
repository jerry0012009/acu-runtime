package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const priceTimeout = 8 * time.Second
const priceHedgeDelay = 600 * time.Millisecond

type priceSnapshot struct {
	Models  json.RawMessage `json:"models"`
	Updated int64           `json:"updatedAt"`
	Warning string          `json:"warning,omitempty"`
}

type priceCache struct {
	sync.Mutex
	priceSnapshot
	File     string
	loaded   bool
	inflight chan struct{}
}

var priceTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = updateProxy
	return transport
}()

func validPriceModels(models json.RawMessage) bool {
	var rows []struct {
		Name string `json:"model_name"`
	}
	if json.Unmarshal(models, &rows) != nil || len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if row.Name == "" {
			return false
		}
	}
	return true
}

// Only the public catalog is persisted; account credentials never enter this file.
func (cache *priceCache) restore() {
	if cache.loaded {
		return
	}
	cache.loaded = true
	file, err := os.Open(cache.File)
	if err != nil {
		return
	}
	defer file.Close()
	var saved priceSnapshot
	if json.NewDecoder(io.LimitReader(file, 2<<20)).Decode(&saved) == nil && validPriceModels(saved.Models) && saved.Updated > 0 && saved.Updated <= time.Now().Unix() && time.Now().Unix()-saved.Updated < 30*24*3600 {
		saved.Warning = ""
		cache.priceSnapshot = saved
	}
}

func (cache *priceCache) persist() {
	if cache.File == "" {
		return
	}
	encoded, err := json.Marshal(cache.priceSnapshot)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(cache.File), "prices-*.tmp")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, err = file.Write(encoded)
	closeErr := file.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(file.Name(), cache.File)
	}
}

func (host *server) fetchPrices(ctx context.Context, site string) (json.RawMessage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, site+"/api/pricing", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	transport := http.RoundTripper(priceTransport)
	if host.cloudTransport != nil {
		transport = host.cloudTransport
	}
	client := http.Client{Transport: transport, Timeout: priceTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope); err != nil {
		return nil, err
	}
	if !envelope.Success || !validPriceModels(envelope.Data) {
		return nil, fmt.Errorf("invalid public price catalog")
	}
	return envelope.Data, nil
}

func (host *server) refreshPrices(done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), priceTimeout)
	defer cancel()
	host.account.mu.Lock()
	preferred, _ := accountSite(host.account.view.Site)
	host.account.mu.Unlock()
	if preferred == "" {
		preferred = billingSite
	}
	alternate := "https://acu.best"
	if preferred == alternate {
		alternate = billingSite
	}
	type result struct {
		models json.RawMessage
		err    error
		site   string
	}
	results := make(chan result, 2)
	start := func(site string) {
		go func() { models, err := host.fetchPrices(ctx, site); results <- result{models, err, site} }()
	}
	start(preferred)
	timer := time.NewTimer(priceHedgeDelay)
	defer timer.Stop()
	started, finished := 1, 0
	var models json.RawMessage
fetch:
	for {
		select {
		case value := <-results:
			finished++
			if value.err == nil {
				models = value.models
				break fetch
			}
			log.Printf("Public model prices: %s: %v", value.site, value.err)
			if started == 1 {
				start(alternate)
				started++
				timer.Stop()
			}
			if finished == 2 {
				break fetch
			}
		case <-timer.C:
			if started == 1 {
				start(alternate)
				started++
			}
		case <-ctx.Done():
			break fetch
		}
	}
	cancel()
	host.prices.Lock()
	defer host.prices.Unlock()
	if models != nil {
		host.prices.priceSnapshot = priceSnapshot{Models: models, Updated: time.Now().Unix()}
		host.prices.persist()
	} else {
		host.prices.Warning = "暂时无法获取最新价格，请稍后重试。"
	}
	host.prices.inflight = nil
	close(done)
}

func (host *server) modelPrices(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	host.prices.Lock()
	host.prices.restore()
	cacheOnly := request.URL.Query().Get("cached") == "1"
	if !cacheOnly && (host.prices.Models == nil || time.Now().Unix()-host.prices.Updated >= 300 || request.URL.Query().Get("refresh") == "1") && host.prices.inflight == nil {
		host.prices.inflight = make(chan struct{})
		go host.refreshPrices(host.prices.inflight)
	}
	done := host.prices.inflight
	host.prices.Unlock()
	if !cacheOnly && done != nil {
		select {
		case <-done:
		case <-request.Context().Done():
			return
		}
	}
	host.prices.Lock()
	value := host.prices.priceSnapshot
	host.prices.Unlock()
	host.json(response, http.StatusOK, value)
}
