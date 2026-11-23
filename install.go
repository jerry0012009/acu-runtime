package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jerry0012009/acu-runtime/internal/installfiles"
)

// Vendor packages are inert sidecars, never executed by Runtime's installer.
// Agent installation independently revalidates the digest/signature. Only
// fixed first-party filenames can be copied, not arbitrary user files.
func copyOfflineAgentPackages(source, target string) error {
	workbuddy, _ := workBuddyPackage(runtime.GOOS, runtime.GOARCH)
	chatgpt, _ := chatGPTPackage(runtime.GOOS, runtime.GOARCH)
	return installfiles.CopyAgentPackages(source, target, workbuddy.Name, chatgpt.Name)
}

func copyInstalledFile(source, target string, mode os.FileMode) error {
	return installfiles.Copy(source, target, mode)
}

func replaceInstalledPath(source, target string) error { return installfiles.Replace(source, target) }

func prepareInstallation(source, target string, release kernelRelease) error {
	name := filepath.Base(managedKernelPath())
	if !verifyKernel(filepath.Join(source, "kernel", name), release.sha256) {
		return errors.New("安装包内核校验失败，请重新获取安装包")
	}
	return installfiles.Install(source, target, name, assets)
}
