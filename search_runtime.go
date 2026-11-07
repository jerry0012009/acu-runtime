package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const searchRuntimeVersion = "0.13.7"

func searchRuntimeRelease(platform, architecture string) (kernelRelease, error) {
	releases := map[string]kernelRelease{
		"windows/amd64": {"uv-x86_64-pc-windows-msvc.zip", "0cdc5b38fe5a8168ed6a25814f5ba43c21d313dd3187314fccb37b7cec46e021"},
		"darwin/arm64":  {"uv-aarch64-apple-darwin.tar.gz", "81b811b57e4624df7ed3aac148e0359100d9962f3d9dce00aec533dc2e086731"},
		"darwin/amd64":  {"uv-x86_64-apple-darwin.tar.gz", "20c47e4c1f630b8abaff8c291fa9b9aa300fef92c4fbff95adf198b55287cda7"},
	}
	if release, ok := releases[platform+"/"+architecture]; ok {
		return release, nil
	}
	return kernelRelease{}, errors.New("此平台暂不支持自动准备搜索工具")
}
func managedSearchDirectory() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ACURuntime", "tools", "uv", searchRuntimeVersion, runtime.GOOS+"-"+runtime.GOARCH)
}
func searchRuntimeName() string {
	if runtime.GOOS == "windows" {
		return "uvx.exe"
	}
	return "uvx"
}

// Extract only the two fixed executable basenames from a checksum-verified
// official archive. Paths, symlinks and all other archive entries are ignored.
func extractSearchRuntime(data []byte, archiveName, target string) error {
	found := map[string]bool{}
	write := func(name string, reader io.Reader, size int64) error {
		base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
		if base != "uv" && base != "uvx" && base != "uv.exe" && base != "uvx.exe" {
			return nil
		}
		if found[base] || size <= 0 || size > 100<<20 {
			return errors.New("搜索工具压缩包内容无效")
		}
		if err := os.MkdirAll(target, 0700); err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join(target, base), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(file, io.LimitReader(reader, (100<<20)+1))
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != size {
			return errors.New("搜索工具文件不完整")
		}
		found[base] = true
		return nil
	}
	if strings.HasSuffix(archiveName, ".zip") {
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		for _, entry := range archive.File {
			if !entry.Mode().IsRegular() {
				continue
			}
			r, err := entry.Open()
			if err != nil {
				return err
			}
			err = write(entry.Name, r, int64(entry.UncompressedSize64))
			r.Close()
			if err != nil {
				return err
			}
		}
	} else {
		zipped, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		defer zipped.Close()
		archive := tar.NewReader(zipped)
		for {
			entry, err := archive.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if entry.Typeflag != tar.TypeReg {
				continue
			}
			if err = write(entry.Name, archive, entry.Size); err != nil {
				return err
			}
		}
	}
	if !(found["uv"] && found["uvx"]) && !(found["uv.exe"] && found["uvx.exe"]) {
		return errors.New("搜索工具压缩包缺少程序")
	}
	return nil
}

func ensureSearchCommand(ctx context.Context) (string, error) {
	if command := freeSearchCommand(); command != "" {
		return command, nil
	}
	release, err := searchRuntimeRelease(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	dir := managedSearchDirectory()
	if dir == "" {
		return "", errors.New("无法创建搜索工具目录")
	}
	if err = os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dir), ".prepare-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage) // stage is created directly under the private tools directory.
	archive := filepath.Join(stage, release.name)
	bundled, bundledErr := assets.ReadFile("web/search-runtime.archive")
	bundledHash := sha256.Sum256(bundled)
	bundledOK := bundledErr == nil && hex.EncodeToString(bundledHash[:]) == release.sha256
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false
	client := &http.Client{Timeout: 90 * time.Second, Transport: transport}
	defer transport.CloseIdleConnections()
	source := "https://github.com/astral-sh/uv/releases/download/" + searchRuntimeVersion + "/" + release.name
	assetIDs := map[string]string{"windows/amd64": "704009796", "darwin/arm64": "704009442", "darwin/amd64": "704009775"}
	apiSource := "https://api.github.com/repos/astral-sh/uv/releases/assets/" + assetIDs[runtime.GOOS+"/"+runtime.GOARCH]
	if bundledOK {
		err = os.WriteFile(archive, bundled, 0600)
	}
	for attempt := 0; !bundledOK && attempt < 3; attempt++ {
		address := apiSource
		if attempt == 1 {
			address = source
		}
		_, err = installKernel(ctx, client, address, archive, release.sha256, func(int64, int64) {})
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	// Use the operating system's TLS-enabled downloader when this network
	// rejects Go's TLS handshake. The pinned checksum is still mandatory.
	if err != nil && ctx.Err() == nil {
		curl, lookupErr := exec.LookPath("curl")
		if lookupErr == nil {
			process := exec.CommandContext(ctx, curl, "--fail", "--silent", "--show-error", "--location", "--proto", "=https", "--proto-redir", "=https", "--connect-timeout", "20", "--max-time", "180", "--retry", "1", "--max-filesize", "134217728", "--header", "Accept: application/octet-stream", "--output", archive, apiSource)
			hideProcess(process)
			if runErr := process.Run(); runErr == nil && verifyKernel(archive, release.sha256) {
				err = nil
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("搜索运行环境下载失败，请重试：%w", err)
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		return "", err
	}
	bin := filepath.Join(stage, "bin")
	if err = extractSearchRuntime(data, release.name, bin); err != nil {
		return "", err
	}
	// A marker is written only after verified extraction. A later launch reuses
	// the private runtime, with no PATH or system Python changes.
	if err = os.WriteFile(filepath.Join(bin, ".verified"), []byte(release.sha256), 0600); err != nil {
		return "", err
	}
	if err = os.Rename(bin, dir); err != nil {
		return "", errors.New("无法保存搜索运行环境，请重试")
	}
	return filepath.Join(dir, searchRuntimeName()), nil
}

func prepareFreeSearch(ctx context.Context, command string) error {
	// uv manages its own package cache and Python download. Running --help first
	// completes dependency preparation before any Agent is given the MCP entry.
	process := exec.CommandContext(ctx, command, "--native-tls", "--python", "3.12", "--from", "duckduckgo-mcp-server[browser]==0.7.0", "duckduckgo-mcp-server", "--help")
	hideProcess(process)
	process.Env = append(os.Environ(), "UV_HTTP_TIMEOUT=45", "UV_HTTP_RETRIES=1")
	if err := process.Run(); err != nil {
		if ctx.Err() != nil {
			return errors.New("准备搜索工具超时，请检查网络后重试")
		}
		return errors.New("搜索工具依赖准备失败，请检查网络后重试（无需手动安装）")
	}
	return nil
}
