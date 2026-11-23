package installfiles

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func CopyAgentPackages(source, target string, names ...string) error {
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) {
		return errors.New("离线安装包目录无效")
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if filepath.Base(name) != name {
			return errors.New("离线安装包文件名无效")
		}
		path := filepath.Join(source, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<30 {
			return fmt.Errorf("离线应用包无效: %s", name)
		}
		if err := Copy(path, filepath.Join(target, "agent-packages", name), 0600); err != nil {
			return fmt.Errorf("无法保存离线应用安装包: %w", err)
		}
	}
	return nil
}

func Copy(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	return write(input, target, mode)
}

func write(input io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(target), ".install-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return Replace(output.Name(), target)
}

// Install commits verified staged files without executing them from staging.
// Legacy Runtime invocations supply the notices from their embedded web files.
func Install(source, target, kernelName string, legacyNotices fs.FS) error {
	if !filepath.IsAbs(target) || filepath.Clean(source) == filepath.Clean(target) {
		return errors.New("无效的安装目录")
	}
	const notice = "THIRD-PARTY-NOTICES.txt"
	files := []string{"ACU Runtime.exe", filepath.Join("kernel", kernelName), notice}
	backups := make([]string, len(files))
	keepBackups := false
	defer func() {
		if !keepBackups {
			for _, name := range backups {
				if name != "" {
					_ = os.Remove(name)
				}
			}
		}
	}()
	for i, name := range files {
		path := filepath.Join(target, name)
		if _, err := os.Stat(path); err == nil {
			backup, err := os.CreateTemp(filepath.Dir(path), ".acu-update-backup-*")
			if err != nil {
				return fmt.Errorf("无法创建现有安装的备份: %w", err)
			}
			backups[i] = backup.Name()
			backup.Close()
			if err := Copy(path, backups[i], 0600); err != nil {
				return fmt.Errorf("无法备份现有安装: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("无法检查现有安装: %w", err)
		}
	}
	modes := []os.FileMode{0755, 0755, 0644}
	copyFile := func(i int, name string) error {
		destination := filepath.Join(target, name)
		if name == notice && legacyNotices != nil {
			input, err := legacyNotices.Open("web/" + notice)
			if err != nil {
				return err
			}
			defer input.Close()
			return write(input, destination, modes[i])
		}
		return Copy(filepath.Join(source, name), destination, modes[i])
	}
	for i, name := range files {
		if err := copyFile(i, name); err != nil {
			var rollbackErrors []error
			// Restore only files already replaced, not the file still locked.
			for j := i - 1; j >= 0; j-- {
				path := filepath.Join(target, files[j])
				if backups[j] != "" {
					if restoreErr := Copy(backups[j], path, modes[j]); restoreErr != nil {
						rollbackErrors = append(rollbackErrors, restoreErr)
					}
				} else if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
					rollbackErrors = append(rollbackErrors, removeErr)
				}
			}
			if len(rollbackErrors) != 0 {
				keepBackups = true
				return fmt.Errorf("安装失败且无法完整恢复；请保留 .acu-update-backup-* 文件: %w", errors.Join(append([]error{err}, rollbackErrors...)...))
			}
			return fmt.Errorf("无法写入 %s；原安装未被更改。若 Runtime 正在运行，请从系统托盘退出后重试: %w", name, err)
		}
	}
	return nil
}
