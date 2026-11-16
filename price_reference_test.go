package main

import (
	"encoding/json"
	"testing"
)

func TestPublicReferenceSeparatesCreditCostFromUSDComparison(t *testing.T) {
	input := json.RawMessage(`[{"model_name":"gpt-6-astra-fast","payable":{"input_cny_per_million":2,"output_cny_per_million":10,"model_multiplier":"0.1"},"reference":{"source_type":"official","input_cny_per_million":134.8,"output_cny_per_million":674,"original_currency":"USD","fx_cny_per_usd":6.74}}]`)
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(attachPriceReferences(input), &rows); err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Input  float64 `json:"input_cny_per_million"`
		Output float64 `json:"output_cny_per_million"`
		Source string  `json:"source_type"`
	}
	if err := json.Unmarshal(rows[0]["comparison_reference"], &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Input != 134.8 || reference.Output != 674 || reference.Source != "official" {
		t.Fatalf("incorrect public reference: %+v", reference)
	}
	var original map[string]any
	json.Unmarshal(rows[0]["payable"], &original)
	if original["model_multiplier"] != "0.1" || original["output_cny_per_million"] != float64(10) {
		t.Fatal("comparison must not change billing")
	}
}

func TestNativeCNYCreditReferenceIsNotAnOfficialComparison(t *testing.T) {
	input := json.RawMessage(`[{"model_name":"unknown","reference":{"original_currency":"CNY","input_cny_per_million":1,"output_cny_per_million":5,"fx_cny_per_usd":6.74}}]`)
	var rows []map[string]json.RawMessage
	json.Unmarshal(attachPriceReferences(input), &rows)
	if _, ok := rows[0]["comparison_reference"]; ok {
		t.Fatal("unknown credit reference was presented as public tariff")
	}
}

func TestPublicReferenceNeedsFXAndDoesNotStripNonGPTFastNames(t *testing.T) {
	for _, input := range []json.RawMessage{
		json.RawMessage(`[{"model_name":"gpt-6-astra-fast","reference":{"original_currency":"CNY","fx_cny_per_usd":0}}]`),
		json.RawMessage(`[{"model_name":"gpt-6-astra","reference":{"original_currency":"CNY"}}]`),
		json.RawMessage(`[{"model_name":"claude-opus-4-6-fast","reference":{"original_currency":"CNY","fx_cny_per_usd":6.74}}]`),
	} {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(attachPriceReferences(input), &rows); err != nil {
			t.Fatal(err)
		}
		if _, ok := rows[0]["comparison_reference"]; ok {
			t.Fatalf("unverified comparison: %s", input)
		}
	}
}
