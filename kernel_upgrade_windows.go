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
	asset, digest := "magpie-windows-amd64.exe", "892fdcfca5535effc188832e9399b091d4680a1685735e41ef1e9e9be204aaef"
	if runtime.GOARCH == "arm64" {
		asset, digest = "magpie-windows-arm64.exe", "80bcae40a181499bfb548264d1b70f7150e5a75fb2eb969ba7a29eac0dae547a"
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
	if err = waitGatewayIdleForChange(ctx); err != nil {
		return errors.New("无法确认本机网关已空闲。请等现有请求完成或停止卡住的请求，再重试内核更新")
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
