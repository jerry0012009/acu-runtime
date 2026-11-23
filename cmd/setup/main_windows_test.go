//go:build windows && setup

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestPayloadVerifiedWithoutExecutingStagedApp(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for _, name := range []string{"ACU Runtime.exe", "kernel/acu-kernel.exe", "THIRD-PARTY-NOTICES.txt"} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprint(entry, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	app := fmt.Sprintf("%x", sha256.Sum256([]byte("ACU Runtime.exe")))
	kernel := fmt.Sprintf("%x", sha256.Sum256([]byte("kernel/acu-kernel.exe")))
	if err := verifyPayload(t.TempDir(), data.Bytes(), app, kernel); err != nil {
		t.Fatal(err)
	}
	for _, hashes := range [][2]string{{"", kernel}, {kernel, kernel}, {app, app}} {
		if err := verifyPayload(t.TempDir(), data.Bytes(), hashes[0], hashes[1]); err == nil {
			t.Fatal("missing or mismatched build pins accepted")
		}
	}
}

func TestInstallerCommandErrorWithoutChildOutput(t *testing.T) {
	output, cause := exec.Command(filepath.Join(t.TempDir(), "not-found.exe")).CombinedOutput()
	if cause == nil || len(output) != 0 {
		t.Fatalf("expected a startup error without child output, got %v / %q", cause, output)
	}
	err := installerCommandError("install", cause, output)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "install") {
		t.Fatalf("startup failure lost its cause or stage: %v", err)
	}
}

func TestInstallerCommandErrorWithWindowsCodeAndChildOutput(t *testing.T) {
	err := installerCommandError("install", syscall.ERROR_ACCESS_DENIED, []byte("  failed to write\x00\r\n"))
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) || !strings.Contains(err.Error(), "Windows 错误 5") ||
		!strings.HasSuffix(err.Error(), "failed to write") || strings.ContainsRune(err.Error(), '\x00') {
		t.Fatalf("unexpected diagnostic: %q", err)
	}
}
