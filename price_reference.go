package main

import (
	"encoding/json"
	"math"
)

// Use the published reference, never a second client-local price catalog.
func attachPriceReferences(models json.RawMessage) json.RawMessage {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(models, &rows) != nil {
		return models
	}
	for _, row := range rows {
		delete(row, "comparison_reference")
		var reference struct {
			Currency string  `json:"original_currency"`
			Source   string  `json:"source_type"`
			FX       float64 `json:"fx_cny_per_usd"`
		}
		if json.Unmarshal(row["reference"], &reference) != nil {
			continue
		}
		if reference.Currency == "USD" && (reference.FX <= 0 || math.IsNaN(reference.FX) || math.IsInf(reference.FX, 0)) {
			continue
		}
		if reference.Source == "official" || reference.Source == "openrouter" || reference.Currency == "USD" {
			row["comparison_reference"] = row["reference"]
		}
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return models
	}
	return encoded
}
