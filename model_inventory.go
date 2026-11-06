package main

import (
	"context"
	"sync"
	"time"
)

type agentCatalogModel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Group  string `json:"group"`
	Hidden bool   `json:"hidden,omitempty"`
	InUse  bool   `json:"inUse,omitempty"`
	Own    bool   `json:"own,omitempty"`
}

type agentCatalog struct {
	Models    []agentCatalogModel `json:"models"`
	Own       bool                `json:"own"`
	OwnHidden bool                `json:"ownHidden"`
	Only      bool                `json:"only"`
}

type agentRole struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Ref   string `json:"ref,omitempty"`
	Name  string `json:"name,omitempty"`
	Group string `json:"group,omitempty"`
}

type routingGroupView struct {
	Ref      string   `json:"ref"`
	Name     string   `json:"name"`
	Members  []string `json:"members"`
	Patterns []string `json:"patterns"`
}

func (host *server) modelInventory(ctx context.Context, agents []agentView) []routingGroupView {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var pending sync.WaitGroup
	for index := range agents {
		if !agents[index].Detected {
			continue
		}
		pending.Add(1)
		go func() {
			defer pending.Done()
			var catalog agentCatalog
			if err := host.kernel.exchange(ctx, "GET", "/api/agent-models/"+agents[index].ID, nil, &catalog); err != nil || catalog.Models == nil {
				agents[index].CatalogError = "模型清单暂未读取到，请重新检测"
				return
			}
			agents[index].Catalog = &catalog
		}()
	}
	var groups struct {
		Groups []routingGroupView `json:"groups"`
	}
	_ = host.kernel.exchange(ctx, "GET", "/api/groups", nil, &groups)
	pending.Wait()
	return groups.Groups
}
