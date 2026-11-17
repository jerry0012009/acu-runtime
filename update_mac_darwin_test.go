package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMacUpdateArchiveRejectsUnsafePathsAndLinks(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "Other.app/Contents/file", "ACU Runtime.app/../escape", "ACU Runtime.app/link"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "update.zip")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(f)
			header := &zip.FileHeader{Name: name}
			header.SetMode(0600)
			if name == "ACU Runtime.app/link" {
				header.SetMode(os.ModeSymlink | 0700)
			}
			item, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			item.Write([]byte("bad"))
			writer.Close()
			f.Close()
			if extractMacUpdate(context.Background(), path, filepath.Join(root, "unpacked")) == nil {
				t.Fatal("unsafe archive extracted")
			}
		})
	}
}

func TestMacUpdateHelperProcess(t *testing.T) {
	if path := os.Getenv("ACU_MAC_UPDATE_HELPER_PLAN"); path != "" {
		launch := launchUpdatedMac
		if os.Getenv("ACU_MAC_UPDATE_FAIL_START") == "1" {
			launch = func(context.Context, string, string, string) (*os.Process, error) {
				return nil, errors.New("fixture launch failure")
			}
		}
		err := applyMacUpdateWith(path, launch, func(string) {})
		if err != nil && os.Getenv("ACU_MAC_UPDATE_FAIL_START") != "1" {
			t.Fatal(err)
		}
	}
}

func TestMacNativeUpdateHandoffAndRollback(t *testing.T) {
	bundle := os.Getenv("ACU_MAC_UPDATE_BUNDLE")
	if bundle == "" {
		t.Skip("native packaged application required")
	}
	version := appVersion
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "HERMES_HOME"} {
				p := filepath.Join(root, key)
				os.MkdirAll(p, 0700)
				t.Setenv(key, p)
			}
			// Reserve an isolated gateway address; no real user's kernel is contacted.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			listener.Close()
			t.Setenv("MAGPIE_ADDR", address)
			target := filepath.Join(root, "ACU Runtime.app")
			if err := exec.Command("/usr/bin/ditto", bundle, target).Run(); err != nil {
				t.Fatal(err)
			}
			workspace, err := os.MkdirTemp(root, ".acu-runtime-update-")
			if err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(workspace, "unpacked", "ACU Runtime.app")
			archive := filepath.Join(workspace, "update.zip")
			if err = exec.Command("/usr/bin/ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", bundle, archive).Run(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if err = extractMacUpdate(ctx, archive, filepath.Dir(stage)); err != nil {
				t.Fatal(err)
			}
			kernel, err := validateMacBundle(ctx, stage, version)
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(root, "CODEX_HOME", "config.toml")
			os.WriteFile(config, []byte("[mcp_servers.preserved]\ncommand = \"preserve-user-model-and-mcp\"\n"), 0600)
			// Fail only the new application's startup, after signature verification.
			// The helper itself still runs independently and restores the old bundle.
			t.Setenv("ACU_KERNEL_PATH", "")
			if fail {
				t.Setenv("ACU_MAC_UPDATE_FAIL_START", "1")
			}
			old := exec.Command("/bin/sleep", "120")
			if err = old.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { old.Process.Kill(); old.Wait() }()
			info, _ := os.Stat(target)
			stat := info.Sys().(*syscall.Stat_t)
			plan := macUpdatePlan{Target: target, Version: version, KernelVersion: kernel, PID: old.Process.Pid, ProcessStamp: macProcessStamp(old.Process.Pid), Device: uint64(stat.Dev), Inode: stat.Ino}
			raw, _ := json.Marshal(plan)
			path := filepath.Join(workspace, "plan.json")
			os.WriteFile(path, raw, 0600)
			helper := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMacUpdateHelperProcess$")
			helper.Env = append(os.Environ(), "ACU_MAC_UPDATE_HELPER_PLAN="+path)
			if !fail {
				binary := filepath.Join(workspace, "update-helper")
				if err = copyInstalledFile(filepath.Join(target, "Contents/MacOS/ACU Runtime"), binary, 0700); err != nil {
					t.Fatal(err)
				}
				helper = exec.CommandContext(ctx, binary, "--apply-mac-update", path)
			}
			log, err := os.Create(filepath.Join(root, "helper.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			helper.Stdout = log
			helper.Stderr = log
			if err = helper.Start(); err != nil {
				t.Fatal(err)
			}
			for {
				if _, err = os.Stat(filepath.Join(workspace, "helper-ready")); err == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("helper never ready")
				case <-time.After(100 * time.Millisecond):
				}
			}
			// The old bundle must remain untouched while its process is still active.
			if _, err = os.Stat(filepath.Join(workspace, "previous.app")); !os.IsNotExist(err) {
				t.Fatal("replacement began before process exit")
			}
			old.Process.Kill()
			old.Wait()
			err = helper.Wait()
			if !fail && err != nil {
				data, _ := os.ReadFile(filepath.Join(root, "helper.log"))
				t.Fatalf("helper failed: %v %s", err, data)
			}
			raw, err = os.ReadFile(filepath.Join(workspace, "result.json"))
			if err != nil {
				data, _ := os.ReadFile(filepath.Join(root, "helper.log"))
				t.Fatalf("missing result: %v %s", err, data)
			}
			var result struct {
				Status string `json:"status"`
				PID    int    `json:"pid"`
			}
			json.Unmarshal(raw, &result)
			if fail {
				if result.Status != "rolled_back" {
					t.Fatal(string(raw))
				}
			} else {
				if result.Status != "completed" || result.PID <= 1 {
					t.Fatal(string(raw))
				}
				p, _ := os.FindProcess(result.PID)
				p.Signal(syscall.SIGTERM)
			}
			if _, err = validateMacBundle(ctx, target, version); err != nil {
				t.Fatal("installed/restored bundle invalid", err)
			}
			if data, err := os.ReadFile(config); err != nil || !strings.Contains(string(data), "preserve-user-model-and-mcp") {
				t.Fatal("user configuration changed")
			}
		})
	}
}
