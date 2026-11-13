package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Keep the native provider ID and edit only its display string, preserving
// formatting, comments, credentials, routing and every other TOML field.
func brandCodexProvider() error {
	path := filepath.Join(codexConfigDirectory(), "config.toml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var parsed map[string]any
	if toml.Unmarshal(data, &parsed) != nil {
		return nil
	}
	var parser unstable.Parser
	parser.Reset(data)
	var table []string
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.ArrayTable && node.Kind != unstable.KeyValue {
			continue
		}
		keys := []string{}
		for it := node.Key(); it.Next(); {
			keys = append(keys, string(it.Node().Data))
		}
		if node.Kind != unstable.KeyValue {
			table = keys
			if node.Kind == unstable.ArrayTable {
				table = nil
			}
			continue
		}
		full := append(slices.Clone(table), keys...)
		value := node.Value()
		if !slices.Equal(full, []string{"model_providers", "magpie", "name"}) || value.Kind != unstable.String || !bytes.EqualFold(value.Data, []byte("magpie")) {
			continue
		}
		start, end := int(value.Raw.Offset), int(value.Raw.Offset+value.Raw.Length)
		updated := append(bytes.Clone(data[:start]), []byte(`"ACU"`)...)
		updated = append(updated, data[end:]...)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return (routeSnapshot{{path: path, data: updated, mode: info.Mode().Perm(), exists: true}}).restore()
	}
	return nil
}
