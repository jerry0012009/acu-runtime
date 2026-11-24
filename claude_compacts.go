package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"time"
)

const claudeCompactRecommendation = 750000

func claudeFamily(id string) bool {
	for _, family := range []string{"opus", "fable", "sonnet", "haiku"} {
		if strings.HasPrefix(strings.ToLower(id), "claude-"+family+"-") {
			return true
		}
	}
	return false
}

func recommendedCompacts(p localProvider) map[string]int {
	if p.ID != networkProviderID || p.Off || !p.Key.Set || p.Compacts["*"] > 0 || p.Contexts["*"] > 0 {
		return nil
	}
	next := maps.Clone(p.Compacts)
	if next == nil {
		next = map[string]int{}
	}
	for _, m := range p.Models {
		wire := m.Vendor
		if wire == "" {
			wire = m.ID
		}
		if m.On && m.Context > claudeCompactRecommendation && claudeFamily(wire) && p.Contexts[m.ID] == 0 && p.Compacts[m.ID] == 0 {
			next[m.ID] = claudeCompactRecommendation
		}
	}
	if maps.Equal(next, p.Compacts) {
		return nil
	}
	return next
}

func (host *server) applyClaudeCompacts(ctx context.Context) error {
	host.networkMu.Lock()
	defer host.networkMu.Unlock()
	return host.applyClaudeCompactsLocked(ctx)
}

func (host *server) applyClaudeCompactsLocked(ctx context.Context) error {
	if host.networkFile == "" || !host.desktop {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := host.configuredProvider(ctx)
	if err != nil {
		return err
	}
	next := recommendedCompacts(p)
	if next == nil {
		return nil
	}
	current, err := host.managedNetwork(p)
	if err != nil {
		return nil
	} // custom or externally managed connection
	var prefs struct {
		CompactAt                    int
		FullContext                  bool
		AgentCompacts, AgentContexts map[string]json.RawMessage
		ContextCap                   int
	}
	if err = host.kernel.exchange(ctx, "GET", "/api/settings", nil, &prefs); err != nil {
		return err
	}
	if prefs.CompactAt != 0 || prefs.FullContext || prefs.ContextCap != 0 || len(prefs.AgentContexts) > 0 || len(prefs.AgentCompacts) > 0 {
		return nil
	}
	// Native provider/save is an editor save, not a partial PATCH. Only replay
	// a known, plain Runtime connection; leave advanced/newer configurations alone.
	body := map[string]json.RawMessage{}
	for field, value := range current.fields {
		switch field {
		case "id", "name", "chat", "responses", "anthropic", "modelsURL", "models", "socket", "proxy", "contexts", "icon", "pickedFrom", "delisted", "delistedAt", "keyName", "keyProtocol", "keyWeight", "keyPlan", "iconFrom", "was", "fetched", "autoInclude", "refreshEvery":
			body[field] = value
		case "key": // native editor keeps the secret on omission
		default:
			return nil
		}
	}
	before, err := readNetworkFile(current.path, 16<<20)
	if err != nil || !bytes.Equal(before, current.data) {
		return errors.New("模型来源已变化，未更新压缩推荐")
	}
	body["compacts"], err = json.Marshal(next)
	if err != nil {
		return err
	}
	if err = host.kernel.exchange(ctx, "POST", "/api/provider/save", body, nil); err != nil {
		return err
	}
	actual, err := host.configuredProvider(ctx)
	if err != nil {
		return err
	}
	if !maps.Equal(actual.Compacts, next) {
		return errors.New("Claude 压缩推荐未通过核验")
	}
	return nil
}
