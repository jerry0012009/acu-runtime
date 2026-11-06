package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func copyInstalledFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(target), ".install-*")
	if err != nil {
		return err
	}
	defer output.Close()
	defer os.Remove(output.Name())
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), target)
}

func prepareInstallation(source, target string, release kernelRelease) error {
	if !filepath.IsAbs(target) || filepath.Clean(source) == filepath.Clean(target) {
		return errors.New("无效的安装目录")
	}
	name := filepath.Base(managedKernelPath())
	bundled := filepath.Join(source, "kernel", name)
	if !verifyKernel(bundled, release.sha256) {
		return errors.New("安装包内核校验失败，请重新获取安装包")
	}
	if err := copyInstalledFile(bundled, filepath.Join(target, "kernel", name), 0755); err != nil {
		return errors.New("无法安装内核，请检查安装目录权限")
	}
	if err := copyInstalledFile(filepath.Join(source, "ACU Runtime.exe"), filepath.Join(target, "ACU Runtime.exe"), 0755); err != nil {
		return errors.New("无法安装应用；如已运行 ACU Runtime，请先从托盘退出再安装")
	}
	notice, err := assets.ReadFile("web/THIRD-PARTY-NOTICES.txt")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "THIRD-PARTY-NOTICES.txt"), notice, 0644); err != nil {
		return errors.New("无法保存本地许可文件")
	}
	return nil
}
