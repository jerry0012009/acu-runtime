package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Magpie owns the catalog and preserves these standard, user-owned Codex keys.
// Run only inside the quiet route transaction so its snapshot owns rollback.
func configureCodexGeminiTools() error {
	dir := codexConfigDirectory()
	raw, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var config struct {
		Catalog string `toml:"model_catalog_json"`
	}
	if err := toml.Unmarshal(raw, &config); err != nil {
		return err
	}
	if config.Catalog == "" {
		return nil
	}
	path := config.Catalog
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	// Never modify a separately managed or user-supplied catalog.
	if filepath.Clean(path) != filepath.Join(dir, "magpie-models.json") && filepath.Clean(path) != filepath.Join(dir, "magpie-desktop-models.json") {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return errors.New("Codex 模型目录无法安全更新")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, changed, err := codexGeminiToolCatalog(data)
	if err != nil || !changed {
		return err
	}
	return (routeSnapshot{{path: path, data: updated, mode: info.Mode().Perm(), exists: true}}).restore()
}

func codexGeminiToolCatalog(data []byte) ([]byte, bool, error) {
	var catalog map[string]json.RawMessage
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, false, err
	}
	var models []map[string]json.RawMessage
	if err := json.Unmarshal(catalog["models"], &models); err != nil {
		return nil, false, err
	}
	changed := false
	for _, model := range models {
		var slug string
		json.Unmarshal(model["slug"], &slug)
		if slug != "acu-runtime/gemini-4-hydrogen" {
			continue
		}
		var mode string
		json.Unmarshal(model["tool_mode"], &mode)
		// Respect an explicit user mode, including direct and code_mode_only.
		if mode != "" && mode != "code_mode" {
			continue
		}
		if mode == "code_mode" && strings.TrimSpace(string(model["use_responses_lite"])) == "false" {
			continue
		}
		model["tool_mode"] = json.RawMessage(`"code_mode"`)
		model["use_responses_lite"] = json.RawMessage(`false`)
		changed = true
	}
	if !changed {
		return data, false, nil
	}
	encoded, err := json.Marshal(models)
	if err != nil {
		return nil, false, err
	}
	catalog["models"] = encoded
	updated, err := json.MarshalIndent(catalog, "", "  ")
	return updated, true, err
}
