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

const setupKernelVersion = "v0.1.3337"

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

func install(waitPID int, requestedTarget string) error {
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
	var manifest struct {
		Version string `json:"version"`
		Name    string `json:"name"`
		SHA256  string `json:"sha256"`
	}
	if json.Unmarshal(manifestBytes, &manifest) != nil || manifest.Version != setupKernelVersion || !strings.EqualFold(manifest.Name, "magpie-cli-windows-amd64.exe") && !strings.EqualFold(manifest.Name, "magpie-cli-windows-arm64.exe") {
		return fmt.Errorf("安装包版本不兼容")
	}
	if !verifyBundledKernel(filepath.Join(staging, "kernel", "acu-kernel.exe"), manifest.SHA256) {
		return fmt.Errorf("安装包内核校验失败")
	}
	target := requestedTarget
	if target == "" {
		target = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")
	}
	if !filepath.IsAbs(target) || filepath.Clean(target) != filepath.Clean(filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")) {
		return fmt.Errorf("更新目录不受信任")
	}
	command = exec.Command(filepath.Join(staging, "ACU Runtime.exe"), "--install-to", target)
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
	flag.Parse()
	if err := install(*waitPID, *target); err != nil {
		message, _ := syscall.UTF16PtrFromString(err.Error())
		title, _ := syscall.UTF16PtrFromString("ACU Runtime 安装未完成")
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}
