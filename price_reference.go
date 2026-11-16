package main

import (
	_ "embed"
	"encoding/json"
	"math"
	"strings"
)

//go:embed price_references.json
var priceReferences []byte

type referenceRates struct {
	Input  float64  `json:"inputPricePerMillion"`
	Output float64  `json:"outputPricePerMillion"`
	Cached *float64 `json:"cachedInputPricePerMillion,omitempty"`
	Write  *float64 `json:"cacheWritePricePerMillion,omitempty"`
}

type referenceTiers struct {
	Threshold int            `json:"thresholdTokens"`
	Standard  referenceRates `json:"standard"`
	Long      referenceRates `json:"longContext"`
}

type publicReferenceSnapshot struct {
	Observed string `json:"observedAt"`
	Models   map[string]struct {
		referenceRates
		Source string          `json:"source"`
		Tiers  *referenceTiers `json:"contextTiers"`
	} `json:"models"`
}

var publicPriceReferences = func() publicReferenceSnapshot {
	var snapshot publicReferenceSnapshot
	if err := json.Unmarshal(priceReferences, &snapshot); err != nil {
		panic(err)
	}
	return snapshot
}()

func attachPriceReferences(models json.RawMessage) json.RawMessage {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(models, &rows) != nil {
		return models
	}
	for _, row := range rows {
		delete(row, "comparison_reference")
		var name string
		var original struct {
			Currency string  `json:"original_currency"`
			FX       float64 `json:"fx_cny_per_usd"`
		}
		if json.Unmarshal(row["model_name"], &name) != nil || json.Unmarshal(row["reference"], &original) != nil {
			continue
		}
		fx := original.FX
		if fx <= 0 || math.IsNaN(fx) || math.IsInf(fx, 0) {
			continue
		}
		var kind struct {
			Source string `json:"source_type"`
		}
		if json.Unmarshal(row["reference"], &kind) == nil && kind.Source == "official" {
			row["comparison_reference"] = row["reference"]
			continue
		}
		// A fast service has its own official price tier. Until the catalog
		// supplies that tier explicitly, never substitute Standard pricing.
		if strings.HasPrefix(name, "gpt-") && strings.HasSuffix(name, "-fast") {
			continue
		}
		// Fast is a service alias, not a separate official priority tariff.
		entry, ok := publicPriceReferences.Models[name]
		if !ok {
			// Frozen USD references remain explicitly platform references. Native CNY
			// credit baselines cannot be reconstructed into official tariffs.
			if original.Currency == "USD" {
				row["comparison_reference"] = row["reference"]
			}
			continue
		}
		scale := func(rates referenceRates) referenceRates {
			rates.Input *= fx
			rates.Output *= fx
			if rates.Cached != nil {
				value := *rates.Cached * fx
				rates.Cached = &value
			}
			if rates.Write != nil {
				value := *rates.Write * fx
				rates.Write = &value
			}
			return rates
		}
		rates := scale(entry.referenceRates)
		var tiers *referenceTiers
		if entry.Tiers != nil {
			tiers = &referenceTiers{Threshold: entry.Tiers.Threshold, Standard: scale(entry.Tiers.Standard), Long: scale(entry.Tiers.Long)}
		}
		row["comparison_reference"], _ = json.Marshal(map[string]any{
			"input_cny_per_million": rates.Input, "output_cny_per_million": rates.Output,
			"cached_input_cny_per_million": rates.Cached, "cache_write_cny_per_million": rates.Write,
			"context_tiers": tiers, "source_type": "public", "source_name": entry.Source,
			"observed_at": publicPriceReferences.Observed, "original_currency": "USD", "fx_cny_per_usd": fx,
		})
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return models
	}
	return encoded
}
