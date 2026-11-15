package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

const videoReadingName = "acu-video-reader"
const videoReadingVersion = "claude-video-vision-1.4.0-node-24.22.3-v1"

var videoInstallMu sync.Mutex

type videoReadingInstall struct {
	Node string `json:"node"`
	Root string `json:"root"`
}

func videoReadingDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "ACURuntime", "video-reading", videoReadingVersion), err
}

func videoNodeRelease(platform, arch string) (string, string, error) {
	key := platform + "-" + arch
	values := map[string]string{
		"windows-amd64": "e612af99f626ab415ea225af6bce5c971feb448cb5cae250bb1c3fa50c5f7d9f",
		"darwin-amd64":  "977614af0504c81045e7fd85596dbca0e5f951b96681516dadafedcca8c447f7",
		"darwin-arm64":  "ff6e17d4874b76d272212b908cc020d6e815fba5a6c7eaaa06add725840307fd",
		"linux-amd64":   "061ec3e3062115705b56b51fa9cc330ab50d34b6969387d4e4b1a50c6809acc2",
	}
	digest, ok := values[key]
	if !ok {
		return "", "", errors.New("当前平台尚未提供视频读取组件")
	}
	name := strings.ReplaceAll(platform, "windows", "win") + "-" + strings.ReplaceAll(arch, "amd64", "x64")
	ext := ".tar.gz"
	if platform == "windows" {
		ext = ".zip"
	}
	return "https://nodejs.org/dist/v24.22.3/node-v24.22.3-" + name + ext, digest, nil
}

// Extract only regular files from the verified Node distribution. npm's JS
// entrypoint is invoked directly, so no archive symlinks or shell shims are needed.
func unpackVideoNode(archive, target string) error {
	var total int64
	copyEntry := func(name string, size int64, input io.Reader) error {
		_, rel, ok := strings.Cut(name, "/")
		if !ok || rel == "" {
			return nil
		}
		clean := filepath.Clean(filepath.FromSlash(rel))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(rel, "\\") {
			return errors.New("invalid Node archive path")
		}
		total += size
		if size < 0 || size > 160<<20 || total > 400<<20 {
			return errors.New("Node archive too large")
		}
		file := filepath.Join(target, clean)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			return err
		}
		out, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			return err
		}
		_, err = io.CopyN(out, input, size)
		return errors.Join(err, out.Close())
	}
	if strings.HasSuffix(archive, ".zip") {
		z, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, file := range z.File {
			if !file.Mode().IsRegular() {
				continue
			}
			in, err := file.Open()
			if err != nil {
				return err
			}
			err = copyEntry(file.Name, int64(file.UncompressedSize64), in)
			in.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if err := copyEntry(h.Name, h.Size, tr); err != nil {
			return err
		}
	}
}

func videoNodePaths(root string) (string, string) {
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "node", "node.exe"), filepath.Join(root, "node", "node_modules", "npm", "bin", "npm-cli.js")
	}
	return filepath.Join(root, "node", "bin", "node"), filepath.Join(root, "node", "lib", "node_modules", "npm", "bin", "npm-cli.js")
}

func ensureVideoReading(ctx context.Context) (videoReadingInstall, error) {
	videoInstallMu.Lock()
	defer videoInstallMu.Unlock()
	root, err := videoReadingDirectory()
	if err != nil {
		return videoReadingInstall{}, err
	}
	node, _ := videoNodePaths(root)
	if _, err := os.Stat(filepath.Join(root, ".ready")); err == nil {
		for _, file := range []string{node, filepath.Join(root, "launch.cjs"), filepath.Join(root, "bin", videoProgram("ffmpeg")), filepath.Join(root, "bin", videoProgram("ffprobe")), filepath.Join(root, "node_modules", "claude-video-vision", "dist", "index.js")} {
			if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
				return videoReadingInstall{}, errors.New("视频组件不完整，请重新准备")
			}
		}
		return videoReadingInstall{node, root}, nil
	}
	url, digest, err := videoNodeRelease(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return videoReadingInstall{}, err
	}
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return videoReadingInstall{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(root), ".prepare-")
	if err != nil {
		return videoReadingInstall{}, err
	}
	defer os.RemoveAll(stage)
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	archive := filepath.Join(stage, "node"+ext)
	if _, err := installKernel(ctx, updateClient(0), url, archive, digest, nil); err != nil {
		return videoReadingInstall{}, errors.New("视频运行环境下载失败，请重试")
	}
	if err := unpackVideoNode(archive, filepath.Join(stage, "node")); err != nil {
		return videoReadingInstall{}, err
	}
	_ = os.Remove(archive)
	for _, name := range []string{"package.json", "package-lock.json", "launch.cjs"} {
		data, err := assets.ReadFile("web/video-mcp/" + name)
		if err != nil {
			return videoReadingInstall{}, err
		}
		if err := os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
			return videoReadingInstall{}, err
		}
	}
	nodeStage, npm := videoNodePaths(stage)
	cmd := exec.CommandContext(ctx, nodeStage, npm, "ci", "--ignore-scripts", "--include=optional", "--no-audit", "--no-fund")
	cmd.Dir = stage
	cmd.Env = append(os.Environ(), "npm_config_registry=https://registry.npmjs.org", "npm_config_update_notifier=false")
	hideProcess(cmd)
	stop, err := startVideoCommand(cmd)
	if err != nil {
		return videoReadingInstall{}, errors.New("视频工具安装未能启动")
	}
	err = cmd.Wait()
	stop()
	if err != nil {
		return videoReadingInstall{}, errors.New("视频工具下载失败，请重试；未修改系统 Node 或应用配置")
	}
	platform := map[string]string{"windows": "win32", "darwin": "darwin", "linux": "linux"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	for _, program := range []string{"ffmpeg", "ffprobe"} {
		source := filepath.Join(stage, "node_modules", "@"+program+"-installer", platform+"-"+arch, videoProgram(program))
		if err := copyInstalledFile(source, filepath.Join(stage, "bin", videoProgram(program)), 0700); err != nil {
			return videoReadingInstall{}, fmt.Errorf("视频解码组件未准备完成：%s", program)
		}
		check := exec.CommandContext(ctx, filepath.Join(stage, "bin", videoProgram(program)), "-version")
		hideProcess(check)
		if err := check.Run(); err != nil {
			return videoReadingInstall{}, fmt.Errorf("视频组件无法启动：%s", program)
		}
	}
	// This is a private tool HOME, never the user's existing plugin configuration.
	if err := writeTapnowPrivateJSON(filepath.Join(stage, "home", ".claude-video-vision", "config.json"), map[string]any{"frame_resolution": 768, "max_frames": 24, "enable_index": false}); err != nil {
		return videoReadingInstall{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, ".ready"), []byte(videoReadingVersion), 0600); err != nil {
		return videoReadingInstall{}, err
	}
	license,err:=assets.ReadFile("web/THIRD-PARTY-NOTICES.txt")
	if err!=nil {return videoReadingInstall{},err}
	if err:=os.WriteFile(filepath.Join(stage,"THIRD-PARTY-NOTICES.txt"),license,0600);err!=nil{return videoReadingInstall{},err}
	if err := os.Rename(stage, root); err != nil {
		return videoReadingInstall{}, err
	}
	return videoReadingInstall{node, root}, nil
}

func videoProgram(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func (host *server) assignVideoReading(ctx context.Context, agents []string) error {
	if len(agents) == 0 {
		return nil
	}
	lib, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	for _, tool := range lib.Servers {
		if tool.Name != videoReadingName {
			continue
		}
		if tool.Note != capabilityOwner {
			return errors.New("已有同名视频工具，未覆盖；请在工具目录检查 acu-video-reader")
		}
		ids := slices.Clone(tool.Agents)
		for _, id := range agents {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if sameAssignments(ids, tool.Agents) {
			return nil
		}
		return host.writeToolLibrary(ctx, "/api/library/servers/agents", map[string]any{"name": videoReadingName, "agents": ids})
	}
	prepare := host.prepareVideoReading
	if prepare == nil {
		prepare = ensureVideoReading
	}
	installed, err := prepare(ctx)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	tool := map[string]any{"name": videoReadingName, "note": capabilityOwner, "transport": "stdio", "command": executable, "args": []string{"--video-reader"}, "agents": agents, "startupTimeout": 60}
	if installed.Node == "" {
		return errors.New("视频运行环境未就绪")
	}
	if err := host.writeToolLibrary(ctx, "/api/library/servers/save", map[string]any{"server": tool}); err != nil {
		return err
	}
	checked, err := host.readToolLibrary(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(checked.Servers, func(t capabilityTool) bool { return t.Name == videoReadingName && sameAssignments(t.Agents, agents) }) {
		return errors.New("视频工具分配回读失败")
	}
	return nil
}

func runVideoReader(ctx context.Context, input io.Reader, output io.Writer) error {
	root, err := videoReadingDirectory()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".ready")); err != nil {
		return errors.New("请先在 ACU Runtime 一键配置视频读取")
	}
	node, _ := videoNodePaths(root)
	cmd := exec.CommandContext(ctx, node, filepath.Join(root, "launch.cjs"))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, output, os.Stderr
	hideProcess(cmd)
	stop, err := startVideoCommand(cmd)
	if err != nil {
		return err
	}
	defer stop()
	return cmd.Wait()
}

func (host *server) prepareVideoTools(ctx context.Context) (videoReadingInstall, error) {
	if host.prepareVideoReading != nil {
		return host.prepareVideoReading(ctx)
	}
	return ensureVideoReading(ctx)
}

// Decode tool content without interpreting pixels; all media work is upstream MCP.
type videoMCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func readVideoFrames(ctx context.Context, installed videoReadingInstall, file string) ([]videoMCPContent, error) {
	cmd := exec.CommandContext(ctx, installed.Node, filepath.Join(installed.Root, "launch.cjs"))
	hideProcess(cmd)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	stop, err := startVideoCommand(cmd)
	if err != nil {
		in.Close()
		return nil, err
	}
	defer func() { in.Close(); stop(); _ = cmd.Wait() }()
	enc := json.NewEncoder(in)
	request := func(id int, method string, params any) error {
		return enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	}
	if err := request(1, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "acu-runtime", "version": appVersion}}); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(io.LimitReader(out, 8<<20))
	for {
		var reply struct {
			ID     int             `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Content []videoMCPContent `json:"content"`
				IsError bool              `json:"isError"`
			} `json:"result"`
		}
		if err := dec.Decode(&reply); err != nil {
			return nil, errors.New("视频读取工具未完成，请重试")
		}
		if len(reply.Error) > 0 || reply.Result.IsError {
			return nil, errors.New("视频解码失败，当前编码可能不受支持")
		}
		if reply.ID == 1 {
			if err := enc.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
				return nil, err
			}
			if err := request(3, "tools/call", map[string]any{"name": "video_info", "arguments": map[string]any{"path": file}}); err != nil {
				return nil, err
			}
		}
		if reply.ID == 3 {
			var metadata struct {
				Duration float64 `json:"duration_seconds"`
			}
			if len(reply.Result.Content) != 1 || json.Unmarshal([]byte(reply.Result.Content[0].Text), &metadata) != nil || metadata.Duration <= 0 || metadata.Duration > 1800 {
				return nil, errors.New("本次视频读取支持 30 分钟以内的视频")
			}
			fps := 12 / metadata.Duration
			if fps > 2 {
				fps = 2
			}
			if err := request(2, "tools/call", map[string]any{"name": "video_watch", "arguments": map[string]any{"path": file, "fps": fps, "view_sample": 12, "resolution": 640, "skip_audio": true}}); err != nil {
				return nil, err
			}
		}
		if reply.ID == 2 {
			return reply.Result.Content, nil
		}
	}
}
