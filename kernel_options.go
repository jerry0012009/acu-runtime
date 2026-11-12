package main

import (
	"encoding/json"
	"errors"
)

// Magpie 0.1.3852 interns field options: list is a one-based optionLists
// index, whose elements are zero-based indexes into options. Expand only
// Runtime's internal snapshot; the embedded native UI reads its own format.
// Keeping options as raw JSON preserves vendor metadata and older kernels.
func expandKernelOptions(state map[string]json.RawMessage) error {
	if len(state["optionLists"]) == 0 {
		return nil
	}
	var lists [][]int
	var options []json.RawMessage
	var agents []map[string]json.RawMessage
	invalid := errors.New("内核选项清单不兼容，请核对兼容版本")
	if json.Unmarshal(state["optionLists"], &lists) != nil || json.Unmarshal(state["options"], &options) != nil || json.Unmarshal(state["agents"], &agents) != nil {
		return invalid
	}
	for _, agent := range agents {
		var fields []map[string]json.RawMessage
		if json.Unmarshal(agent["fields"], &fields) != nil {
			return invalid
		}
		for _, field := range fields {
			var list int
			if raw := field["list"]; len(raw) > 0 && json.Unmarshal(raw, &list) != nil {
				return invalid
			}
			if list == 0 {
				continue
			}
			if list < 1 || list > len(lists) {
				return invalid
			}
			expanded := make([]json.RawMessage, 0, len(lists[list-1]))
			for _, index := range lists[list-1] {
				if index < 0 || index >= len(options) || string(options[index]) == "null" {
					return invalid
				}
				expanded = append(expanded, options[index])
			}
			field["options"], _ = json.Marshal(expanded)
		}
		agent["fields"], _ = json.Marshal(fields)
	}
	state["agents"], _ = json.Marshal(agents)
	return nil
}
