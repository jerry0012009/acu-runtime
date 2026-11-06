package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestModelInventoryProjectsOnlySafeCatalogFields(t *testing.T) {
	host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Error("inventory must be read-only")
		}
		switch request.URL.Path {
		case "/api/agent-models/workbuddy":
			io.WriteString(response, `{"models":[{"id":"acu-runtime/acu-auto","name":"acu-auto","group":"ACU","inUse":true,"key":"private-secret"},{"id":"auto/image","name":"Image","group":"Routing groups","hidden":true}],"own":true,"only":true,"private":"private-secret"}`)
		case "/api/groups":
			io.WriteString(response, `{"groups":[{"ref":"auto/image","name":"Image","members":["acu-runtime/image"],"patterns":["image-*"],"key":"private-secret"}]}`)
		default:
			t.Errorf("unexpected endpoint %s", request.URL.Path)
		}
	}))
	agents := []agentView{{ID: "workbuddy", Detected: true}, {ID: "codex"}}
	groups := host.modelInventory(context.Background(), agents)
	if agents[0].Catalog == nil || len(agents[0].Catalog.Models) != 2 || !agents[0].Catalog.Models[1].Hidden || !agents[0].Catalog.Only {
		t.Fatalf("catalog projection failed: %+v", agents[0])
	}
	if agents[1].Catalog != nil || len(groups) != 1 || groups[0].Members[0] != "acu-runtime/image" {
		t.Fatal("missing group candidates or incorrect undetected Agent catalog")
	}
	encoded, err := json.Marshal(struct {
		Agents []agentView
		Groups []routingGroupView
	}{agents, groups})
	if err != nil || strings.Contains(string(encoded), "private-secret") {
		t.Fatal("inventory leaked unapproved fields")
	}
}

func TestModelInventoryDistinguishesUnavailableFromEmpty(t *testing.T) {
	for _, payload := range []string{`{}`, `{"models":[]}`, `invalid`} {
		t.Run(payload, func(t *testing.T) {
			host, _ := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				io.WriteString(response, payload)
			}))
			agents := []agentView{{ID: "codex", Detected: true}}
			host.modelInventory(context.Background(), agents)
			if payload == `{"models":[]}` {
				if agents[0].Catalog == nil || agents[0].CatalogError != "" {
					t.Fatal("valid empty catalog reported unavailable")
				}
			} else if agents[0].Catalog != nil || agents[0].CatalogError == "" {
				t.Fatal("unavailable catalog falsely reported as empty success")
			}
		})
	}
}
