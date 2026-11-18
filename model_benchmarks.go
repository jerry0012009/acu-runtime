package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed model_benchmarks.json
var benchmarkSnapshot []byte

type benchmarkVariant struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type modelBenchmark struct {
	Benchmark  string             `json:"benchmark"`
	Score      float64            `json:"score"`
	SourceURL  string             `json:"source_url"`
	ObservedAt string             `json:"observed_at"`
	Selection  string             `json:"selection"`
	Variants   []benchmarkVariant `json:"variants"`
}

var publicBenchmarks = func() map[string]modelBenchmark {
	var snapshot struct {
		Benchmark    string                        `json:"benchmark"`
		ObservedAt   string                        `json:"observedAt"`
		Selection    string                        `json:"selection"`
		Models       map[string][]benchmarkVariant `json:"models"`
		ModelSources map[string]struct {
			ObservedAt string `json:"observedAt"`
		} `json:"modelSources"`
	}
	if err := json.Unmarshal(benchmarkSnapshot, &snapshot); err != nil {
		panic(err)
	}
	result := make(map[string]modelBenchmark, len(snapshot.Models))
	for slug, variants := range snapshot.Models {
		if len(variants) == 0 {
			continue
		}
		observedAt := snapshot.ObservedAt
		if source := snapshot.ModelSources[slug]; source.ObservedAt != "" {
			observedAt = source.ObservedAt
		}
		result[slug] = modelBenchmark{Benchmark: snapshot.Benchmark, Score: variants[0].Score,
			SourceURL: "https://artificialanalysis.ai/models/" + slug, ObservedAt: observedAt,
			Selection: snapshot.Selection, Variants: variants}
	}
	return result
}()

func attachModelBenchmarks(models json.RawMessage) json.RawMessage {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(models, &rows) != nil {
		return models
	}
	for _, row := range rows {
		delete(row, "capability_benchmark")
		var name string
		if json.Unmarshal(row["model_name"], &name) != nil {
			continue
		}
		// Match exact model releases only. Never inherit scores for fast/preview aliases.
		if benchmark, ok := publicBenchmarks[strings.ReplaceAll(name, ".", "-")]; ok {
			row["capability_benchmark"], _ = json.Marshal(benchmark)
		}
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return models
	}
	return encoded
}
