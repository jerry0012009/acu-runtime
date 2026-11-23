//go:build windows && setup

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jerry0012009/acu-runtime/internal/installfiles"
	"golang.org/x/sys/windows"
)

//go:embed payload.zip
var payload []byte

// These pins come from the exact staged Runtime during packaging. Empty
// values fail closed, so a manually built installer cannot silently drift.
var setupKernelVersion string
var setupKernelName string
var setupKernelSHA256 string
var setupAppSHA256 string
var setupWorkBuddyPackage string
var setupChatGPTPackage string

func verifyFile(path, digest string) bool {
	if len(digest) != 64 {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	return err == nil && hex.EncodeToString(hash.Sum(nil)) == digest
}

func installerCommandError(stage string, err error, output []byte) error {
	var code syscall.Errno
	if errors.As(err, &code) {
		stage = fmt.Sprintf("%s（Windows 错误 %d）", stage, code)
	}
	detail := strings.TrimSpace(strings.ReplaceAll(string(output), "\x00", ""))
	if detail != "" {
		return fmt.Errorf("%s: %w\n%s", stage, err, detail)
	}
	return fmt.Errorf("%s: %w", stage, err)
}

func createStartMenuShortcut(executable string) error {
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	powershell := filepath.Join(systemDir, "WindowsPowerShell", "v1.0", "powershell.exe")
	shortcut := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; $shell=New-Object -ComObject WScript.Shell; $link=$shell.CreateShortcut([IO.Path]::Combine([Environment]::GetFolderPath('Programs'),'ACU Runtime.lnk')); $link.TargetPath=$env:ACU_INSTALLED_EXE; $link.WorkingDirectory=[IO.Path]::GetDirectoryName($env:ACU_INSTALLED_EXE); $link.Save()`)
	shortcut.Env = append(os.Environ(), "ACU_INSTALLED_EXE="+executable)
	shortcut.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := shortcut.CombinedOutput(); err != nil {
		return installerCommandError("创建开始菜单快捷方式失败", err, output)
	}
	return nil
}

func install(waitPID int, requestedTarget string, verifyOnly bool) (warning, failure error) {
	if err := waitForRuntime(waitPID); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp("", "acu-setup-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时安装目录失败: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := verifyPayload(staging, payload, setupAppSHA256, setupKernelSHA256); err != nil {
		return nil, fmt.Errorf("验证安装包失败: %w", err)
	}
	if verifyOnly {
		fmt.Printf("PASS: installer payload verified: %s %s\n", setupKernelVersion, setupKernelName)
		return nil, nil
	}
	defaultTarget := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")
	target := requestedTarget
	if target == "" {
		target = defaultTarget
	}
	if !filepath.IsAbs(defaultTarget) {
		return nil, fmt.Errorf("无法定位当前用户的 LocalAppData 安装目录")
	}
	if !filepath.IsAbs(target) || filepath.Clean(target) != filepath.Clean(defaultTarget) {
		return nil, fmt.Errorf("更新目录不受信任")
	}
	if err := installfiles.Install(staging, target, "acu-kernel.exe", nil); err != nil {
		return nil, installerCommandError("安装 Runtime 文件失败", err, nil)
	}
	var warnings []error
	if executable, err := os.Executable(); err == nil {
		offline := filepath.Join(filepath.Dir(executable), "agent-packages")
		if info, err := os.Stat(offline); err == nil && info.IsDir() {
			if err := installfiles.CopyAgentPackages(offline, target, setupWorkBuddyPackage, setupChatGPTPackage); err != nil {
				warnings = append(warnings, err)
			}
		}
	}
	executable := filepath.Join(target, "ACU Runtime.exe")
	if requestedTarget == "" {
		if err := createStartMenuShortcut(executable); err != nil {
			warnings = append(warnings, err)
		}
	}
	command := exec.Command(executable)
	command.Dir = target
	if err := command.Start(); err != nil {
		warnings = append(warnings, installerCommandError("无法自动启动 Runtime", err, nil))
	} else {
		_ = command.Process.Release()
	}
	if warning := errors.Join(warnings...); warning != nil {
		return fmt.Errorf("应用已安装。安装位置：%s\n%w", executable, warning), nil
	}
	return nil, nil
}

func waitForRuntime(waitPID int) error {
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
	return nil
}

func verifyPayload(staging string, data []byte, appDigest, kernelDigest string) error {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	allowed := map[string]bool{"ACU Runtime.exe": true, "kernel/acu-kernel.exe": true, "THIRD-PARTY-NOTICES.txt": true}
	for _, entry := range archive.File {
		if !allowed[entry.Name] || !entry.Mode().IsRegular() || entry.UncompressedSize64 > 160<<20 {
			return fmt.Errorf("无效的安装包内容")
		}
		delete(allowed, entry.Name)
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
		written, copyErr := io.Copy(output, io.LimitReader(input, (160<<20)+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if uint64(written) != entry.UncompressedSize64 {
			return fmt.Errorf("安装包文件大小不一致: %s", entry.Name)
		}
	}
	if len(allowed) != 0 {
		return fmt.Errorf("安装包内容不完整")
	}
	if !verifyFile(filepath.Join(staging, "ACU Runtime.exe"), appDigest) {
		return fmt.Errorf("安装包应用校验失败")
	}
	if !verifyFile(filepath.Join(staging, "kernel", "acu-kernel.exe"), kernelDigest) {
		return fmt.Errorf("安装包内核校验失败")
	}
	return nil
}

func reportSetupIssue(err error, failed, showDialog bool) {
	detail := strings.ReplaceAll(err.Error(), "\x00", "")
	fmt.Fprintln(os.Stderr, detail)
	if !showDialog {
		return
	}
	if log, err := os.CreateTemp("", "acu-runtime-setup-*.log"); err == nil {
		_, writeErr := fmt.Fprintln(log, detail)
		closeErr := log.Close()
		if writeErr == nil && closeErr == nil {
			detail += "\n\n诊断日志: " + log.Name()
		} else {
			_ = os.Remove(log.Name())
		}
	}
	titleText, icon := "ACU Runtime 已安装", uintptr(0x30)
	if failed {
		titleText, icon = "ACU Runtime 安装未完成", 0x10
	}
	message, _ := syscall.UTF16PtrFromString(detail)
	title, _ := syscall.UTF16PtrFromString(titleText)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), icon)
}

func main() {
	waitPID := flag.Int("wait-pid", 0, "wait for this Runtime process to exit")
	target := flag.String("target", "", "existing installation directory")
	verifyOnly := flag.Bool("verify-only", false, "verify the embedded payload without installing or opening Runtime")
	noDialog := flag.Bool("no-dialog", false, "write installation diagnostics to stderr instead of showing dialogs")
	flag.Parse()
	warning, err := install(*waitPID, *target, *verifyOnly)
	showDialog := !*verifyOnly && !*noDialog
	if err != nil {
		reportSetupIssue(err, true, showDialog)
		os.Exit(1)
	}
	if warning != nil {
		reportSetupIssue(warning, false, showDialog)
	}
}
