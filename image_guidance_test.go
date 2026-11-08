package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestImageGuidancePreservesAssignmentsAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	assigned := []string{"claude"}
	writes := 0
	host, _ := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/library":
			json.NewEncoder(w).Encode(map[string]any{"skills": []any{map[string]any{"name": "acu-image-tools", "source": dir, "agents": assigned}}})
		case "/api/library/skills/agents":
			var in struct{ Agents []string }
			json.NewDecoder(r.Body).Decode(&in)
			assigned = in.Agents
			writes++
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected kernel action: %s", r.URL.Path)
		}
	}))
	host.imageSkillSource = func() (string, error) { return dir, nil }
	for range 2 {
		if err := host.assignImageGuidance(context.Background(), []string{"workbuddy"}); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 || len(assigned) != 2 || assigned[0] != "claude" || assigned[1] != "workbuddy" {
		t.Fatal("lost assignments or rewrote unchanged configuration")
	}
}
