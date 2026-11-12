//go:build windows && setup

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed payload.zip
var payload []byte

// These pins come from the exact staged Runtime during packaging. Empty
// values fail closed, so a manually built installer cannot silently drift.
var setupKernelVersion string
var setupKernelName string
var setupKernelSHA256 string

type kernelManifest struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	SHA256  string `json:"sha256"`
}

func validateKernelManifest(actual, expected kernelManifest) error {
	if expected.Version == "" || expected.Name == "" || len(expected.SHA256) != 64 {
		return fmt.Errorf("安装包缺少构建校验清单，请重新构建安装包")
	}
	if expected.Name != "magpie-cli-windows-amd64.exe" && expected.Name != "magpie-cli-windows-arm64.exe" {
		return fmt.Errorf("安装包内核架构清单无效")
	}
	if actual != expected {
		return fmt.Errorf("安装包清单不一致（打包内核 %s，应用内核 %s），请重新构建安装包", expected.Version, actual.Version)
	}
	if _, err := hex.DecodeString(expected.SHA256); err != nil {
		return fmt.Errorf("安装包内核摘要清单无效")
	}
	return nil
}

func verifyBundledKernel(path, digest string) bool {
	if len(digest) != 64 {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]) == digest
}

func install(waitPID int, requestedTarget string, verifyOnly bool) error {
	if waitPID > 0 {
		handle, err := syscall.OpenProcess(0x00100000, false, uint32(waitPID))
		if err != nil && err != syscall.Errno(87) {
			return fmt.Errorf("无法等待 Runtime 退出: %w", err)
		}
		if err == nil {
			defer syscall.CloseHandle(handle)
			exited := false
			for attempt := 0; attempt < 120; attempt++ {
				state, err := syscall.WaitForSingleObject(handle, 500)
				if err != nil {
					return fmt.Errorf("无法确认 Runtime 已退出: %w", err)
				}
				if state == 0 {
					exited = true
					break
				}
			}
			if !exited {
				return fmt.Errorf("Runtime 尚未退出，取消更新")
			}
		}
	}
	staging, err := os.MkdirTemp("", "acu-setup-*")
	if err != nil {
		return err
	}
	if filepath.Dir(staging) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(staging), "acu-setup-") {
		return fmt.Errorf("无效的临时安装目录")
	}
	defer os.RemoveAll(staging)
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}
	allowed := map[string]bool{"ACU Runtime.exe": true, "kernel/acu-kernel.exe": true, "THIRD-PARTY-NOTICES.txt": true}
	for _, entry := range archive.File {
		if !allowed[entry.Name] || entry.UncompressedSize64 > 160<<20 {
			return fmt.Errorf("无效的安装包内容")
		}
		target := filepath.Join(staging, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, io.LimitReader(input, (160<<20)+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	stagedApp := filepath.Join(staging, "ACU Runtime.exe")
	command := exec.Command(stagedApp, "--kernel-release-json")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	manifestBytes, err := command.Output()
	if err != nil {
		return fmt.Errorf("安装包校验清单无效")
	}
	var manifest kernelManifest
	if json.Unmarshal(manifestBytes, &manifest) != nil {
		return fmt.Errorf("安装包校验清单格式无效")
	}
	expected := kernelManifest{setupKernelVersion, setupKernelName, setupKernelSHA256}
	if err := validateKernelManifest(manifest, expected); err != nil {
		return err
	}
	if !verifyBundledKernel(filepath.Join(staging, "kernel", "acu-kernel.exe"), expected.SHA256) {
		return fmt.Errorf("安装包内核校验失败")
	}
	if verifyOnly {
		fmt.Printf("PASS: installer payload verified: %s %s\n", expected.Version, expected.Name)
		return nil
	}
	target := requestedTarget
	if target == "" {
		target = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")
	}
	if !filepath.IsAbs(target) || filepath.Clean(target) != filepath.Clean(filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")) {
		return fmt.Errorf("更新目录不受信任")
	}
	args := []string{"--install-to", target}
	if executable, err := os.Executable(); err == nil {
		offline := filepath.Join(filepath.Dir(executable), "agent-packages")
		if info, err := os.Stat(offline); err == nil && info.IsDir() {
			args = append(args, "--agent-packages-from", offline)
		}
	}
	command = exec.Command(filepath.Join(staging, "ACU Runtime.exe"), args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", output)
	}
	if requestedTarget == "" {
		shortcut := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$shell=New-Object -ComObject WScript.Shell; $link=$shell.CreateShortcut([IO.Path]::Combine([Environment]::GetFolderPath('Programs'),'ACU Runtime.lnk')); $link.TargetPath=$env:ACU_INSTALLED_EXE; $link.WorkingDirectory=[IO.Path]::GetDirectoryName($env:ACU_INSTALLED_EXE); $link.Save()`)
		shortcut.Env = append(os.Environ(), "ACU_INSTALLED_EXE="+filepath.Join(target, "ACU Runtime.exe"))
		shortcut.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := shortcut.Run(); err != nil {
			return fmt.Errorf("应用已安装在 %s，但无法创建开始菜单快捷方式", target)
		}
	}
	return exec.Command(filepath.Join(target, "ACU Runtime.exe")).Start()
}

func main() {
	waitPID := flag.Int("wait-pid", 0, "wait for this Runtime process to exit")
	target := flag.String("target", "", "existing installation directory")
	verifyOnly := flag.Bool("verify-only", false, "verify the embedded payload without installing or opening Runtime")
	flag.Parse()
	if err := install(*waitPID, *target, *verifyOnly); err != nil {
		if *verifyOnly {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		message, _ := syscall.UTF16PtrFromString(err.Error())
		title, _ := syscall.UTF16PtrFromString("ACU Runtime 安装未完成")
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}
