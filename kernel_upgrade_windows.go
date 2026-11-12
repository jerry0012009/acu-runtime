package main

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The known per-user GUI installation can keep serving an older engine after
// Runtime itself was updated. Keep its GUI binary compatible for future starts,
// retire only that verified process while idle, then Runtime starts its own CLI.
func upgradeOldGateway(ctx context.Context, info servingKernel) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, info.PID)
	if err != nil {
		return errors.New("无法更新正在运行的旧内核")
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(process, 0, &buffer[0], &size) != nil {
		return errors.New("无法核实旧内核位置")
	}
	target := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "magpie", "magpie.exe")
	actual := windows.UTF16ToString(buffer[:size])
	if !strings.EqualFold(filepath.Clean(actual), filepath.Clean(target)) {
		return errors.New("正在运行的内核位于自定义目录，无法安全完成自动升级")
	}
	asset, digest := "magpie-windows-amd64.exe", "d6f7e482139956c3d6da7a113a4100fc0c5a0c43d0d7c6068e5058d909472951"
	if runtime.GOARCH == "arm64" {
		asset, digest = "magpie-windows-arm64.exe", "6bd287759daed7a7aff9c2ee6c589e352344f9e53ec3d34337f05a729fa2349d"
	}
	cache := filepath.Join(filepath.Dir(managedKernelPath()), asset)
	if !verifyKernel(cache, digest) {
		client := &http.Client{Transport: &http.Transport{Proxy: updateProxy}}
		source := "https://github.com/yetone/magpie/releases/download/" + kernelVersion + "/" + asset
		_, err = installKernel(ctx, client, source, cache, digest, nil)
		if err != nil {
			_, err = installKernel(ctx, client, "https://gh-proxy.com/"+source, cache, digest, nil)
		}
		if err != nil {
			return err
		}
	}
	if err = waitGatewayIdle(ctx); err != nil {
		return err
	}
	backup := target + ".acu-before-" + strings.TrimPrefix(kernelVersion, "v")
	if _, err = os.Stat(backup); err == nil {
		return errors.New("旧内核升级备份已存在，保留现场以避免覆盖")
	}
	// A mapped Windows executable can be renamed before its replacement is copied.
	if err = os.Rename(target, backup); err != nil {
		return errors.New("无法保护旧内核文件，未停止服务")
	}
	if err = copyInstalledFile(cache, target, 0755); err != nil {
		_ = os.Rename(backup, target)
		return errors.New("无法更新内核，已保留原程序")
	}
	if err = windows.TerminateProcess(process, 0); err != nil {
		return errors.New("内核已更新，但旧进程尚未退出")
	}
	result, err := windows.WaitForSingleObject(process, 10000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("旧内核退出超时，未改变 Codex 配置")
	}
	return nil
}

// Quiet Codex transactions require an owned gateway, including when a
// separately launched GUI happens to have the compatible or a newer version.
// Keep its on-disk binary; retire only the verified idle default GUI process.
func retireCompatibleGateway(ctx context.Context, info servingKernel) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, info.PID)
	if err != nil {
		return errors.New("无法接管正在运行的独立内核")
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(process, 0, &buffer[0], &size) != nil {
		return errors.New("无法核实独立内核位置，未停止进程")
	}
	target := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "magpie", "magpie.exe")
	if !strings.EqualFold(filepath.Clean(windows.UTF16ToString(buffer[:size])), filepath.Clean(target)) {
		return errors.New("正在运行的内核位于自定义目录，已保留；请退出该内核后重试")
	}
	if err = waitGatewayIdle(ctx); err != nil {
		return err
	}
	if err = windows.TerminateProcess(process, 0); err != nil {
		return errors.New("独立内核尚未退出，未改变 Codex 配置")
	}
	result, err := windows.WaitForSingleObject(process, 10000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("独立内核退出超时，未改变 Codex 配置")
	}
	return nil
}
