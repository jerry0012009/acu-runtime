package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const kernelVersion = "v0.1.3909"

type kernelRelease struct {
	name   string
	sha256 string
}

func releaseFor(platform, architecture string) (kernelRelease, error) {
	releases := map[string]kernelRelease{
		"windows/amd64": {"magpie-cli-windows-amd64.exe", "4764c41e57da50371ba8b664f5b387a2b17fb26deac0937725fd9674789ae2cf"},
		"windows/arm64": {"magpie-cli-windows-arm64.exe", "0a94008b8115e4c00cdea283275ff458598c2528567fc306393212a1ed807894"},
		"darwin/amd64":  {"magpie-cli-darwin-amd64", "99bcc2a118a16e4e461867cc856814d361723d0387da645ca6559e5c71990727"},
		"darwin/arm64":  {"magpie-cli-darwin-arm64", "33dc4b92910cd0cce867b1f2ed46927d9043e743cbc06a786efb468e12ecbc16"},
	}
	release, ok := releases[platform+"/"+architecture]
	if !ok {
		return kernelRelease{}, errors.New("此平台尚未提供自动内核安装")
	}
	return release, nil
}

func managedKernelPath() string {
	directory, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	name := "acu-kernel"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(directory, "ACURuntime", "kernel", kernelVersion, runtime.GOOS+"-"+runtime.GOARCH, name)
}

func verifyKernel(path, digest string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, (128<<20)+1)); err != nil {
		return false
	}
	return hex.EncodeToString(hash.Sum(nil)) == digest
}

func installKernel(ctx context.Context, client *http.Client, source, target, digest string, progress func(int64, int64)) (string, error) {
	if verifyKernel(target, digest) {
		return target, nil
	}
	if len(digest) != 64 || target == "" {
		return "", errors.New("无效的内核安装信息")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return "", errors.New("无法创建内核目录")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "ACU-Runtime/"+appVersion)
	request.Header.Set("Accept", "application/octet-stream")
	response, err := client.Do(request)
	if err != nil {
		return "", errors.New("内核下载失败，请检查网络后重试")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("内核下载失败（HTTP %d），请稍后重试", response.StatusCode)
	}
	if response.ContentLength > 128<<20 {
		return "", errors.New("内核文件超出大小限制")
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".download-*")
	if err != nil {
		return "", errors.New("无法写入内核目录")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	writer := &kernelDownloadWriter{writer: io.MultiWriter(file, hash), total: response.ContentLength, progress: progress}
	count, copyErr := io.Copy(writer, io.LimitReader(response.Body, (128<<20)+1))
	if copyErr != nil {
		return "", errors.New("内核下载中断，请重试")
	}
	if count > 128<<20 || hex.EncodeToString(hash.Sum(nil)) != digest {
		return "", errors.New("内核文件校验失败，未安装，请重试")
	}
	if err := file.Chmod(0755); err != nil {
		return "", errors.New("无法设置内核运行权限")
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(file.Name(), target); err != nil {
		return "", errors.New("无法保存内核文件；未修改已有安装")
	}
	return target, nil
}

type kernelDownloadWriter struct {
	writer   io.Writer
	written  int64
	total    int64
	progress func(int64, int64)
}

func (writer *kernelDownloadWriter) Write(data []byte) (int, error) {
	count, err := writer.writer.Write(data)
	writer.written += int64(count)
	if writer.progress != nil {
		writer.progress(writer.written, writer.total)
	}
	return count, err
}

func (kernel *kernelState) install(ctx context.Context) (string, error) {
	release, err := releaseFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	if executable, err := os.Executable(); err == nil {
		name := "acu-kernel"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		bundled := filepath.Join(filepath.Dir(executable), "kernel", name)
		if verifyKernel(bundled, release.sha256) {
			return bundled, nil
		}
	}
	kernel.mu.Lock()
	kernel.phase = "正在下载内核"
	kernel.mu.Unlock()
	client := &http.Client{Timeout: 8 * time.Minute, CheckRedirect: func(request *http.Request, previous []*http.Request) error {
		if len(previous) > 5 || request.URL.Scheme != "https" {
			return errors.New("不安全的下载重定向")
		}
		return nil
	}}
	source := "https://github.com/yetone/magpie/releases/download/" + kernelVersion + "/" + release.name
	return installKernel(ctx, client, source, managedKernelPath(), release.sha256, func(downloaded, total int64) {
		kernel.mu.Lock()
		kernel.downloaded, kernel.total = downloaded, total
		kernel.mu.Unlock()
	})
}
