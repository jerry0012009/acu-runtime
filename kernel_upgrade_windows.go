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
	asset, digest := "magpie-windows-amd64.exe", "fd89e3937968f7f93d4744d5b6b434f71f1d650e1d8376d9661017acbee75501"
	if runtime.GOARCH == "arm64" {
		asset, digest = "magpie-windows-arm64.exe", "1b99817bcd4285c5d12717b42b18dfe377fd86b1082a08e72a2d0bca074fdcfb"
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
	_, err = windows.WaitForSingleObject(process, 10000)
	return err
}
