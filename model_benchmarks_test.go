package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
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
