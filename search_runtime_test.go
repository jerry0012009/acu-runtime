package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchRuntimePlatformManifest(t *testing.T) {
	for _, platform := range []struct{ os, arch string }{{"windows", "amd64"}, {"darwin", "amd64"}, {"darwin", "arm64"}} {
		r, err := searchRuntimeRelease(platform.os, platform.arch)
		if err != nil || len(r.sha256) != 64 {
			t.Fatalf("bad pinned release %+v", r)
		}
	}
	if _, err := searchRuntimeRelease("linux", "amd64"); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}
func TestSearchRuntimeExtractionKeepsOnlyExecutables(t *testing.T) {
	for _, format := range []string{"zip", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			var b bytes.Buffer
			files := map[string]string{"nested/uv": "uv-fixture", "nested/uvx": "uvx-fixture", "../../outside": "escape", "nested/README": "notice"}
			if format == "zip" {
				w := zip.NewWriter(&b)
				for name, text := range files {
					f, _ := w.Create(name)
					f.Write([]byte(text))
				}
				w.Close()
			} else {
				g := gzip.NewWriter(&b)
				w := tar.NewWriter(g)
				for name, text := range files {
					w.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(text)), Typeflag: tar.TypeReg})
					w.Write([]byte(text))
				}
				w.Close()
				g.Close()
			}
			root := t.TempDir()
			target := filepath.Join(root, "bin")
			if err := extractSearchRuntime(b.Bytes(), "fixture."+format, target); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"uv", "uvx"} {
				if _, err := os.Stat(filepath.Join(target, name)); err != nil {
					t.Fatal(err)
				}
			}
			items, _ := os.ReadDir(target)
			if len(items) != 2 {
				t.Fatal("extracted unwanted file")
			}
			if _, err := os.Stat(filepath.Join(root, "outside")); !os.IsNotExist(err) {
				t.Fatal("escaped extraction directory")
			}
		})
	}
}
func TestSearchRuntimeRejectsIncompleteArchive(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create("uvx")
	f.Write([]byte("fixture"))
	w.Close()
	if extractSearchRuntime(b.Bytes(), "fixture.zip", t.TempDir()) == nil {
		t.Fatal("accepted missing uv executable")
	}
}
