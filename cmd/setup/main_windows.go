//go:build windows && setup

package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
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

func install() error {
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
	target := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ACU Runtime")
	command := exec.Command(filepath.Join(staging, "ACU Runtime.exe"), "--install-to", target)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", output)
	}
	shortcut := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$shell=New-Object -ComObject WScript.Shell; $link=$shell.CreateShortcut([IO.Path]::Combine([Environment]::GetFolderPath('Programs'),'ACU Runtime.lnk')); $link.TargetPath=$env:ACU_INSTALLED_EXE; $link.WorkingDirectory=[IO.Path]::GetDirectoryName($env:ACU_INSTALLED_EXE); $link.Save()`)
	shortcut.Env = append(os.Environ(), "ACU_INSTALLED_EXE="+filepath.Join(target, "ACU Runtime.exe"))
	shortcut.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := shortcut.Run(); err != nil {
		return fmt.Errorf("应用已安装在 %s，但无法创建开始菜单快捷方式", target)
	}
	return exec.Command(filepath.Join(target, "ACU Runtime.exe")).Start()
}

func main() {
	if err := install(); err != nil {
		message, _ := syscall.UTF16PtrFromString(err.Error())
		title, _ := syscall.UTF16PtrFromString("ACU Runtime 安装未完成")
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}
