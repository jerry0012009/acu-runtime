package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGatewayProcessHelper(t *testing.T) {
	if os.Getenv("ACU_GATEWAY_PROCESS_HELPER") != "1" {
		return
	}
	time.Sleep(60 * time.Second)
}
func TestCompatibleDefaultGUIKeepsServingGateway(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	source, _ := os.Executable()
	target := filepath.Join(root, "Programs", "magpie", "magpie.exe")
	if err := copyInstalledFile(source, target, 0700); err != nil {
		t.Fatal(err)
	}
	original, _ := os.Stat(target)
	command := exec.Command(target, "-test.run=^TestGatewayProcessHelper$")
	command.Env = append(os.Environ(), "ACU_GATEWAY_PROCESS_HELPER=1")
	hideProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			json.NewEncoder(w).Encode(servingKernel{Name: "magpie", Version: "9.9.9", PID: uint32(command.Process.Pid)})
			return
		}
		w.Write([]byte(`{"object":"concurrency","requests":0,"waiting":0,"tools":0}`))
	}))
	defer server.Close()
	t.Setenv("MAGPIE_ADDR", strings.TrimPrefix(server.URL, "http://"))
	k := &kernelState{enforceVersion: true}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reuse, err := k.prepareGateway(ctx)
	if err != nil || !reuse {
		t.Fatalf("expected to use the running Magpie gateway, got reuse=%v, err=%v", reuse, err)
	}
	select {
	case <-done:
		t.Fatal("Runtime stopped the running Magpie gateway")
	case <-time.After(100 * time.Millisecond):
	}
	after, _ := os.Stat(target)
	if after.Size() != original.Size() || !after.ModTime().Equal(original.ModTime()) {
		t.Fatal("newer GUI binary was replaced")
	}
}
