package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSetupWithLockedInstalledFile(t *testing.T) {
	const app = "ACU Runtime.exe"
	kernel := filepath.Join("kernel", "acu-kernel.exe")
	for _, locked := range []string{app, kernel} {
		t.Run(filepath.Base(locked), func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			for dir, version := range map[string]string{source: "new", target: "old"} {
				if err := os.Mkdir(filepath.Join(dir, "kernel"), 0700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{app, kernel} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(version), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			path, err := windows.UTF16PtrFromString(filepath.Join(target, locked))
			if err != nil {
				t.Fatal(err)
			}
			// Omit FILE_SHARE_DELETE to reproduce a blocked Windows replacement.
			handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { windows.CloseHandle(handle) })
			err = prepareInstallation(source, target, kernelRelease{sha256: fmt.Sprintf("%x", sha256.Sum256([]byte("new")))})
			if err == nil || strings.Contains(err.Error(), "无法完整恢复") || !strings.Contains(err.Error(), filepath.Base(locked)) {
				t.Fatalf("expected replacement diagnostic for %s without a rollback failure; got %v", locked, err)
			}
			for _, name := range []string{app, kernel} {
				if data, err := os.ReadFile(filepath.Join(target, name)); err != nil || string(data) != "old" {
					t.Fatalf("original %s was not preserved: %q %v", name, data, err)
				}
				matches, err := filepath.Glob(filepath.Join(filepath.Dir(filepath.Join(target, name)), ".acu-update-backup-*"))
				if err != nil || len(matches) != 0 {
					t.Fatalf("unexpected retained backups: %v %v", matches, err)
				}
			}
		})
	}
}
