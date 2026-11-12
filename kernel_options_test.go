package main

import (
	"encoding/json"
	"testing"
)

func TestKernelInternedOptionsPreserveModelSelection(t *testing.T) {
	var state map[string]json.RawMessage
	json.Unmarshal([]byte(`{"agents":[{"id":"codex","fields":[{"key":"model","list":1,"options":null},{"key":"login","options":[{"value":"api"}]}]}],"optionLists":[[1,0]],"options":[{"value":"native","label":"Original"},{"value":"acu-runtime/acu-auto","ref":"acu-runtime/acu-auto","label":"ACU"}]}`), &state)
	if err := expandKernelOptions(state); err != nil {
		t.Fatal(err)
	}
	var agents []setupAgent
	if err := json.Unmarshal(state["agents"], &agents); err != nil {
		t.Fatal(err)
	}
	if got := pickACU(agents[0].Fields, "model", "acu-runtime", "acu-auto"); got != "acu-runtime/acu-auto" {
		t.Fatal(got)
	}
	if agents[0].Fields[1].Options[0].Value != "api" {
		t.Fatal("lost inline options")
	}
	// Repeated reads and older kernels remain safe.
	if err := expandKernelOptions(state); err != nil {
		t.Fatal(err)
	}
	delete(state, "optionLists")
	if err := expandKernelOptions(state); err != nil {
		t.Fatal(err)
	}
}

func TestKernelInternedOptionsRejectInvalidIndexes(t *testing.T) {
	for _, body := range []string{
		`{"agents":[{"fields":[{"list":2}]}],"optionLists":[[0]],"options":[{}]}`,
		`{"agents":[{"fields":[{"list":1}]}],"optionLists":[[-1]],"options":[{}]}`,
		`{"agents":[{"fields":[{"list":1}]}],"optionLists":[[1]],"options":[{}]}`,
	} {
		var state map[string]json.RawMessage
		json.Unmarshal([]byte(body), &state)
		if expandKernelOptions(state) == nil {
			t.Fatal("accepted invalid option list", body)
		}
	}
}
