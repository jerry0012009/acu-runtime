package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Read-only validation of a real vendor package; never installs or changes
// the active ChatGPT application, even when a package is supplied.
func TestOfficialChatGPTPackageSignature(t *testing.T) {
	path := os.Getenv("ACU_CHATGPT_VERIFY_PACKAGE")
	if path == "" {
		t.Skip("set ACU_CHATGPT_VERIFY_PACKAGE for read-only official package verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := verifyChatGPTPackage(ctx, path); err != nil {
		probe := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", chatGPTVerifyScript)
		probe.Env = chatGPTInstallerEnv(path)
		out, _ := probe.CombinedOutput()
		t.Log(string(out))
		t.Fatal(err)
	}
	if !chatGPTInstalled() {
		t.Log("package valid; client not installed in this user profile")
	}
}

func TestChatGPTSignatureRejectsUnsignedFile(t *testing.T) {
	path := t.TempDir() + "/fake.msix"
	os.WriteFile(path, []byte("not a signed MSIX"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if verifyChatGPTPackage(ctx, path) == nil {
		t.Fatal("unsigned package accepted")
	}
}
