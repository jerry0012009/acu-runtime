package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in acceptance downloads public packages only, into a disposable directory.
func TestTapnowPublishedChannelPreparesBothEditions(t *testing.T) {
	expected := os.Getenv("ACU_TAPNOW_RELEASE_VERSION")
	if expected == "" {
		t.Skip("set ACU_TAPNOW_RELEASE_VERSION for public package acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := tapnowHTTPClient()
	body, err := tapnowRead(ctx, client, tapnowCatalogURL, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var catalog tapnowCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != 1 || len(catalog.Packages) != 2 {
		t.Fatal("unexpected public channel schema or editions")
	}
	state := &tapnowState{root: t.TempDir()}
	for _, edition := range []string{"companion", "backup"} {
		pkg := catalog.Packages[edition]
		if pkg.Version != expected || !validTapnowPackage(edition, pkg) {
			t.Fatalf("%s: expected valid published %s package, got %s", edition, expected, pkg.Version)
		}
		if err := state.prepare(ctx, client, edition, pkg); err != nil {
			t.Fatalf("%s package preparation failed: %v", edition, err)
		}
		directory := filepath.Join(state.root, edition)
		if readTapnowManifest(directory) != expected || !tapnowPreparedIdentity(directory, edition) {
			t.Fatalf("%s prepared version/identity mismatch", edition)
		}
		t.Logf("%s %s: public download, digest, safe extraction and fixed identity passed", edition, expected)
	}
}
