package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveBenchmarkPriceCatalog(t *testing.T) {
	path := os.Getenv("ACU_BENCHMARK_PRICE_FIXTURE")
	if path == "" {
		t.Skip("explicit read-only public catalog check")
	}
	host := &server{}
	models, err := host.fetchPrices(context.Background(), billingSite)
	if err != nil {
		t.Fatal(err)
	}
	models = attachModelBenchmarks(models)
	models = attachPriceReferences(models)
	var rows []struct {
		Name      string          `json:"model_name"`
		Benchmark *modelBenchmark `json:"capability_benchmark"`
	}
	if err := json.Unmarshal(models, &rows); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, row := range rows {
		if row.Benchmark != nil {
			matched++
		}
	}
	if matched < 10 {
		t.Fatalf("too few matched public models: %d", matched)
	}
	envelope, _ := json.Marshal(map[string]any{"success": true, "data": json.RawMessage(models)})
	if err := os.WriteFile(path, envelope, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("real price catalog: %d models, %d exact benchmark matches", len(rows), matched)
}

func TestModelBenchmarksMatchOnlyExactRelease(t *testing.T) {
	input := json.RawMessage(`[{"model_name":"gpt-6.1-sol","payable":{"output_cny_per_million":3.37}},{"model_name":"claude-fable-5-2"},{"model_name":"gpt-6.1-sol-fast"},{"model_name":"acu-auto"},{"model_name":"unknown","capability_benchmark":{"score":99}}]`)
	var rows []struct {
		Name      string             `json:"model_name"`
		Benchmark *modelBenchmark    `json:"capability_benchmark"`
		Payable   map[string]float64 `json:"payable"`
	}
	if err := json.Unmarshal(attachModelBenchmarks(input), &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[:2] {
		if row.Benchmark == nil || !strings.Contains(row.Benchmark.Benchmark, "v5.0.1") || row.Benchmark.Score <= 0 || row.Benchmark.SourceURL == "" {
			t.Fatalf("missing real benchmark: %+v", row)
		}
	}
	if rows[0].Payable["output_cny_per_million"] != 3.37 {
		t.Fatal("price changed")
	}
	for _, row := range rows[2:] {
		if row.Benchmark != nil {
			t.Fatalf("invented alias benchmark: %s", row.Name)
		}
	}
	if !strings.Contains(string(input), `"score":99`) {
		t.Fatal("input mutated")
	}
}

func TestBenchmarkSnapshotHasComparableSortedVariants(t *testing.T) {
	if len(publicBenchmarks) < 20 {
		t.Fatal("incomplete snapshot")
	}
	for slug, benchmark := range publicBenchmarks {
		last := 101.0
		for _, variant := range benchmark.Variants {
			if variant.Score < 0 || variant.Score > 100 || variant.Score > last || variant.Name == "" {
				t.Fatalf("invalid variant: %s %+v", slug, variant)
			}
			last = variant.Score
		}
		if benchmark.Score != benchmark.Variants[0].Score {
			t.Fatal("sort score differs")
		}
	}
}

func TestGpt62PriceBenchmarksPreservePublishedPrices(t *testing.T) {
	input := json.RawMessage(`[{"model_name":"gpt-6.2-astra","payable":{"model_multiplier":"0.04","input_cny_per_million":2.1568,"output_cny_per_million":10.784,"cached_input_cny_per_million":0.10784,"cache_write_cny_per_million":2.696,"context_tiers":{"thresholdTokens":272000,"longContext":{"inputPricePerMillion":4.3136,"outputPricePerMillion":16.176}}}},{"model_name":"gpt-6.2-sol","payable":{"model_multiplier":"0.05","input_cny_per_million":0.674,"output_cny_per_million":3.37}},{"model_name":"gpt-6.2-sol-fast"},{"model_name":"gpt-6.2-astra-preview"}]`)
	var before, after []map[string]json.RawMessage
	if err := json.Unmarshal(input, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(attachModelBenchmarks(input), &after); err != nil {
		t.Fatal(err)
	}
	for index, expected := range []struct {
		name  string
		score float64
	}{{"GPT-6.2 Astra (Max)", 62.1387990593969}, {"GPT-6.2 Sol (Max)", 57.2576209885477}} {
		var benchmark modelBenchmark
		if err := json.Unmarshal(after[index]["capability_benchmark"], &benchmark); err != nil {
			t.Fatal("new model missing benchmark", err)
		}
		if benchmark.Score != expected.score || benchmark.Variants[0].Name != expected.name || len(benchmark.Variants) != 5 || benchmark.Benchmark != "Artificial Analysis Intelligence Index v5.0.1" {
			t.Fatalf("incorrect same-board result: %+v", benchmark)
		}
		if benchmark.ObservedAt != "2026-11-18" || benchmark.Selection != "highest_published_configuration" {
			t.Fatalf("new benchmark provenance missing: %+v", benchmark)
		}
		if string(before[index]["payable"]) != string(after[index]["payable"]) {
			t.Fatal("benchmark attachment changed published prices or tiers")
		}
	}
	for _, row := range after[2:] {
		if _, exists := row["capability_benchmark"]; exists {
			t.Fatal("score inferred for an unmeasured alias")
		}
	}
}

func TestGpt62BenchmarksAttachToExistingPriceCacheWithoutNetwork(t *testing.T) {
	host := &server{}
	host.prices.priceSnapshot = priceSnapshot{
		Models:  json.RawMessage(`[{"model_name":"gpt-6.2-astra","capability_benchmark":{"score":0},"payable":{"output_cny_per_million":10.784}},{"model_name":"gpt-6.2-sol","payable":{"output_cny_per_million":3.37}}]`),
		Updated: time.Now().Unix(),
	}
	out := httptest.NewRecorder()
	host.modelPrices(out, httptest.NewRequest("GET", "/runtime/billing/prices?cached=1", nil))
	var response struct {
		Models []struct {
			Benchmark modelBenchmark     `json:"capability_benchmark"`
			Payable   map[string]float64 `json:"payable"`
		} `json:"models"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Models) != 2 || response.Models[0].Benchmark.Score != 62.1387990593969 || response.Models[1].Benchmark.Score != 57.2576209885477 {
		t.Fatalf("upgraded Runtime did not enrich cached models: %s", out.Body)
	}
	if response.Models[0].Payable["output_cny_per_million"] != 10.784 || response.Models[1].Payable["output_cny_per_million"] != 3.37 {
		t.Fatal("cached retail rates changed")
	}
}
