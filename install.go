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
	files := []struct{ source, target string }{
		{bundled, filepath.Join(target, "kernel", name)},
		{filepath.Join(source, "ACU Runtime.exe"), filepath.Join(target, "ACU Runtime.exe")},
	}
	backups := make([]string, len(files))
	for i, file := range files {
		if _, err := os.Stat(file.target); err == nil {
			backup, err := os.CreateTemp(filepath.Dir(file.target), ".acu-update-backup-*")
			if err != nil {
				return errors.New("无法保护现有安装")
			}
			backups[i] = backup.Name()
			backup.Close()
			if err := copyInstalledFile(file.target, backups[i], 0600); err != nil {
				return errors.New("无法保护现有安装")
			}
		} else if !os.IsNotExist(err) {
			return errors.New("无法检查现有安装")
		}
	}
	rollback := func() error {
		var failures []error
		for i, file := range files {
			if backups[i] != "" {
				if err := copyInstalledFile(backups[i], file.target, 0755); err != nil {
					failures = append(failures, err)
				}
			} else {
				if err := os.Remove(file.target); err != nil && !os.IsNotExist(err) {
					failures = append(failures, err)
				}
			}
		}
		return errors.Join(failures...)
	}
	for _, file := range files {
		if err := copyInstalledFile(file.source, file.target, 0755); err != nil {
			if rollback() != nil {
				return errors.New("更新失败且无法完整恢复；原文件备份保留在安装目录的 .acu-update-backup 文件中，请重新运行安装包修复")
			}
			for _, backup := range backups {
				if backup != "" {
					_ = os.Remove(backup)
				}
			}
			return errors.New("无法更新应用或内核，已恢复原安装")
		}
	}
	for _, backup := range backups {
		if backup != "" {
			_ = os.Remove(backup)
		}
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
