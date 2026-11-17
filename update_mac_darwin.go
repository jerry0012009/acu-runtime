package main

import (
	"archive/zip"
	"context"
	"debug/macho"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type macUpdatePlan struct {
	Target        string `json:"target"`
	Version       string `json:"version"`
	KernelVersion string `json:"kernelVersion"`
	PID           int    `json:"pid"`
	ProcessStamp  string `json:"processStamp"`
	Device        uint64 `json:"device"`
	Inode         uint64 `json:"inode"`
}

func verifySignedMacKernel(path string) bool {
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	if path != filepath.Join(bundle, "Contents/MacOS/kernel/acu-kernel") {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	version, err := validateMacBundle(ctx, bundle, appVersion)
	return err == nil && version == kernelVersion
}

func macProcessStamp(pid int) string {
	out, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func macBundleTarget(executable string) (string, error) {
	target := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
	if filepath.Base(target) != "ACU Runtime.app" || executable != filepath.Join(target, "Contents", "MacOS", "ACU Runtime") {
		return "", errors.New("请先将 ACU Runtime.app 安装到可写的应用目录，再使用应用内更新")
	}
	clean, err := filepath.EvalSymlinks(target)
	info, statErr := os.Lstat(target)
	if err != nil || statErr != nil || info.Mode()&os.ModeSymlink != 0 || strings.HasPrefix(clean, "/Volumes/") || strings.Contains(clean, "/AppTranslocation/") {
		return "", errors.New("当前应用位于磁盘映像、临时或链接目录；请先移入 Applications 后再更新")
	}
	return clean, nil
}

func validateMacBundle(ctx context.Context, bundle, version string) (string, error) {
	for _, item := range []string{"Contents", "Contents/Info.plist", "Contents/MacOS", "Contents/MacOS/ACU Runtime", "Contents/MacOS/kernel", "Contents/MacOS/kernel/acu-kernel"} {
		info, err := os.Lstat(filepath.Join(bundle, item))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("更新包应用结构无效")
		}
	}
	for key, want := range map[string]string{"CFBundleIdentifier": "com.acucompute.runtime", "CFBundleExecutable": "ACU Runtime", "CFBundleShortVersionString": version} {
		out, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", filepath.Join(bundle, "Contents/Info.plist")).Output()
		if err != nil || strings.TrimSpace(string(out)) != want {
			return "", errors.New("更新包应用身份或版本不符")
		}
	}
	if err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle).Run(); err != nil {
		return "", errors.New("更新包签名完整性验证失败")
	}
	for _, name := range []string{"ACU Runtime", "kernel/acu-kernel"} {
		file, err := macho.Open(filepath.Join(bundle, "Contents/MacOS", name))
		if err != nil {
			return "", errors.New("更新包不是有效的 Mac 可执行文件")
		}
		cpu := macho.CpuAmd64
		if runtime.GOARCH == "arm64" {
			cpu = macho.CpuArm64
		}
		valid := file.Cpu == cpu
		file.Close()
		if !valid {
			return "", errors.New("更新包架构与当前设备不符")
		}
	}
	// The official CLI gets an ad-hoc signature during Mac packaging, which
	// changes its file hash. Trust the full ZIP digest plus the sealed bundle.
	out, err := exec.CommandContext(ctx, filepath.Join(bundle, "Contents/MacOS/ACU Runtime"), "--kernel-release-json").Output()
	var manifest struct{ Version, Name, SHA256 string }
	if err != nil || json.Unmarshal(out, &manifest) != nil || !releaseVersionPattern.MatchString(manifest.Version) || !digestPattern.MatchString(manifest.SHA256) || manifest.Name != "magpie-cli-darwin-"+runtime.GOARCH {
		return "", errors.New("更新包内核清单无效")
	}
	out, err = exec.CommandContext(ctx, filepath.Join(bundle, "Contents/MacOS/kernel/acu-kernel"), "--version").Output()
	if err != nil || strings.TrimSpace(string(out)) != "magpie "+strings.TrimPrefix(manifest.Version, "v") {
		return "", errors.New("更新包内核版本不符")
	}
	return manifest.Version, nil
}

func extractMacUpdate(ctx context.Context, archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return errors.New("更新包 ZIP 无法读取")
	}
	defer reader.Close()
	var size uint64
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || filepath.Clean(name) != name || strings.HasPrefix(name, "../") || (!strings.HasPrefix(name, "ACU Runtime.app/") && name != "ACU Runtime.app" && !strings.HasPrefix(name, "__MACOSX/")) {
			return errors.New("更新包包含不安全路径")
		}
		if file.Mode()&os.ModeSymlink != 0 || (!file.Mode().IsRegular() && !file.FileInfo().IsDir()) {
			return errors.New("更新包包含链接或特殊文件")
		}
		if file.UncompressedSize64 > 512<<20 || size > (512<<20)-file.UncompressedSize64 {
			return errors.New("更新包解压体积超限")
		}
		size += file.UncompressedSize64
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, "/usr/bin/ditto", "-x", "-k", archive, destination).Run(); err != nil {
		return errors.New("更新包解压失败，当前安装未变更")
	}
	return nil
}

func (host *server) downloadMacUpdate(ctx context.Context, release *updateRelease) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	target, err := macBundleTarget(executable)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp(filepath.Dir(target), ".acu-runtime-update-")
	if err != nil {
		return errors.New("应用目录不可写；请将应用移入用户 Applications 目录后重试，当前安装未变更")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			os.RemoveAll(root)
		}
	}()
	archive := filepath.Join(root, "update.zip")
	err = downloadUpdatePackage(ctx, updateClient(6*time.Minute), updateDownloadSources(release.URL), archive, release.SHA256, func(done, total int64) {
		host.update.mu.Lock()
		host.update.view.Downloaded = done
		host.update.view.Total = total
		host.update.mu.Unlock()
	})
	if err != nil {
		return fmt.Errorf("更新包下载或校验失败，当前安装未变更：%w", err)
	}
	stage := filepath.Join(root, "unpacked")
	if err = extractMacUpdate(ctx, archive, stage); err != nil {
		return err
	}
	candidate := filepath.Join(stage, "ACU Runtime.app")
	verify, cancel := context.WithTimeout(ctx, 30*time.Second)
	kernelVersion, err := validateMacBundle(verify, candidate, release.Version)
	cancel()
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	stat := info.Sys().(*syscall.Stat_t)
	plan := macUpdatePlan{Target: target, Version: release.Version, KernelVersion: kernelVersion, PID: os.Getpid(), ProcessStamp: macProcessStamp(os.Getpid()), Device: uint64(stat.Dev), Inode: stat.Ino}
	if plan.ProcessStamp == "" {
		return errors.New("无法核实当前应用进程，未退出更新")
	}
	// Finish an active model/tool request before the updater asks Runtime to quit.
	host.kernel.mu.Lock()
	attached := host.kernel.endpoint != nil
	host.kernel.mu.Unlock()
	if attached {
		idle, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err = waitGatewayIdle(idle)
		cancel()
		if err != nil {
			return errors.New("当前内核仍有任务或状态未知，未退出更新；请稍后重试")
		}
	}
	data, _ := json.Marshal(plan)
	path := filepath.Join(root, "plan.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	helper := filepath.Join(root, "update-helper")
	if err = copyInstalledFile(executable, helper, 0700); err != nil {
		return err
	}
	command := exec.Command(helper, "--apply-mac-update", path)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	log, err := os.OpenFile(filepath.Join(root, "updater.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	command.Stdout = log
	command.Stderr = log
	if err = command.Start(); err != nil {
		return errors.New("无法启动更新助手，当前安装未变更")
	}
	defer func() {
		if !handedOff {
			_ = command.Process.Kill()
		}
	}()
	go command.Wait()
	for attempt := 0; attempt < 300; attempt++ {
		if _, err = os.Stat(filepath.Join(root, "helper-ready")); err == nil {
			handedOff = true
			host.update.mu.Lock()
			host.update.view.Phase = "restarting"
			host.update.mu.Unlock()
			select {
			case host.quitRequests <- struct{}{}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	_ = command.Process.Kill()
	return errors.New("更新助手未就绪，当前安装未变更")
}

func applyMacUpdate(path string) error {
	return applyMacUpdateWith(path, launchUpdatedMac, func(target string) { _ = exec.Command("/usr/bin/open", "-n", target).Run() })
}

func applyMacUpdateWith(path string, launch func(context.Context, string, string, string) (*os.Process, error), restartOld func(string)) error {
	root := filepath.Dir(path)
	if !filepath.IsAbs(path) || filepath.Base(path) != "plan.json" || !strings.HasPrefix(filepath.Base(root), ".acu-runtime-update-") {
		return errors.New("Invalid update plan path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var plan macUpdatePlan
	if json.Unmarshal(data, &plan) != nil || plan.PID <= 1 || plan.ProcessStamp == "" || !releaseVersionPattern.MatchString(plan.Version) {
		return errors.New("Invalid update plan")
	}
	if filepath.Dir(plan.Target) != filepath.Dir(root) || filepath.Base(plan.Target) != "ACU Runtime.app" {
		return errors.New("Invalid update target")
	}
	candidate := filepath.Join(root, "unpacked", "ACU Runtime.app")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	version, err := validateMacBundle(ctx, candidate, plan.Version)
	if err != nil || version != plan.KernelVersion {
		return errors.New("Candidate validation failed")
	}
	info, err := os.Stat(plan.Target)
	if err != nil {
		return err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if uint64(stat.Dev) != plan.Device || stat.Ino != plan.Inode {
		return errors.New("Target changed during update")
	}
	if macProcessStamp(plan.PID) != plan.ProcessStamp {
		return errors.New("Original Runtime process identity changed")
	}
	if err = os.WriteFile(filepath.Join(root, "helper-ready"), []byte("ready"), 0600); err != nil {
		return err
	}
	deadline := time.Now().Add(120 * time.Second)
	for macProcessStamp(plan.PID) == plan.ProcessStamp {
		if time.Now().After(deadline) {
			return errors.New("Old Runtime did not exit; installation unchanged")
		}
		time.Sleep(200 * time.Millisecond)
	}
	info, err = os.Lstat(plan.Target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Update target became unavailable")
	}
	stat = info.Sys().(*syscall.Stat_t)
	if uint64(stat.Dev) != plan.Device || stat.Ino != plan.Inode {
		return errors.New("Update target changed before replacement")
	}
	if _, err = macBundleTarget(filepath.Join(plan.Target, "Contents/MacOS/ACU Runtime")); err != nil {
		return err
	}
	// Renames stay on the same volume and preserve a full rollback bundle.
	backup := filepath.Join(root, "previous.app")
	if err = os.Rename(plan.Target, backup); err != nil {
		return err
	}
	if err = os.Rename(candidate, plan.Target); err != nil {
		if restoreErr := os.Rename(backup, plan.Target); restoreErr != nil {
			return fmt.Errorf("Old bundle retained at %s: %w", backup, restoreErr)
		}
		restartOld(plan.Target)
		return err
	}
	process, healthErr := launch(ctx, plan.Target, plan.Version, plan.KernelVersion)
	if healthErr != nil {
		if process != nil {
			_ = process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() { _, _ = process.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				return fmt.Errorf("New process did not exit; old bundle retained at %s", backup)
			}
		}
		if err = os.Rename(plan.Target, filepath.Join(root, "failed.app")); err != nil {
			return fmt.Errorf("New application failed; backup retained at %s: %w", backup, err)
		}
		if err = os.Rename(backup, plan.Target); err != nil {
			return fmt.Errorf("Rollback bundle retained at %s: %w", backup, err)
		}
		restartOld(plan.Target)
		_ = os.WriteFile(filepath.Join(root, "result.json"), []byte(`{"status":"rolled_back"}`), 0600)
		return healthErr
	}
	result, _ := json.Marshal(map[string]any{"status": "completed", "pid": process.Pid})
	_ = process.Release()
	_ = os.WriteFile(filepath.Join(root, "result.json"), result, 0600)
	// Retain only the small result/log; executable backups have served their purpose.
	_ = os.RemoveAll(backup)
	_ = os.RemoveAll(filepath.Join(root, "unpacked"))
	_ = os.Remove(filepath.Join(root, "update.zip"))
	_ = os.Remove(filepath.Join(root, "update-helper"))
	return nil
}

func launchUpdatedMac(ctx context.Context, target, version, kernel string) (*os.Process, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	address := listener.Addr().String()
	listener.Close()
	command := exec.Command(filepath.Join(target, "Contents/MacOS/ACU Runtime"), "--listen", address)
	if err = command.Start(); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+address+"/runtime/status", nil)
		response, e := client.Do(req)
		if e == nil {
			var status struct {
				Version string `json:"version"`
				PID     int    `json:"pid"`
				Desktop bool   `json:"desktop"`
				Ready   bool   `json:"ready"`
				Update  struct {
					Kernel string `json:"kernel"`
				} `json:"update"`
			}
			e = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status)
			response.Body.Close()
			if e == nil && status.PID == command.Process.Pid && status.Desktop && status.Ready && status.Version == version && status.Update.Kernel == kernel {
				return command.Process, nil
			}
		}
		select {
		case <-ctx.Done():
			return command.Process, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return command.Process, errors.New("新版未完成启动验证，已尝试恢复旧应用")
}
