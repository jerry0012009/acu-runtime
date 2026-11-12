package main

import (
	"errors"
	"os"
	"path/filepath"
)

type routeFile struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}
type routeSnapshot []routeFile

func codexConfigDirectory() string {
	if p := os.Getenv("CODEX_HOME"); filepath.IsAbs(p) {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}
func snapshotCodexRoute(executable string) (routeSnapshot, error) {
	dir := codexConfigDirectory()
	paths := []string{filepath.Join(dir, "config.toml"), filepath.Join(dir, "magpie-models.json"), filepath.Join(dir, "magpie-desktop-models.json")}
	for _, base := range kernelConfigDirs(executable) {
		for _, name := range []string{"stash.json", "applied.json"} {
			paths = append(paths, filepath.Join(base, name))
		}
	}
	seen := map[string]bool{}
	var snapshot routeSnapshot
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		f := routeFile{path: path}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			snapshot = append(snapshot, f)
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 16<<20 {
			return nil, errors.New("configuration is not a bounded regular file")
		}
		f.exists = true
		f.mode = info.Mode().Perm()
		f.data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		snapshot = append(snapshot, f)
	}
	return snapshot, nil
}
func (snapshot routeSnapshot) restore() error {
	for _, f := range snapshot {
		if !f.exists {
			if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(f.path), 0700); err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(f.path), ".acu-restore-*")
		if err != nil {
			return err
		}
		name := tmp.Name()
		if err = tmp.Chmod(f.mode); err == nil {
			_, err = tmp.Write(f.data)
		}
		closeErr := tmp.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = replaceRouteFile(name, f.path)
		}
		if err != nil {
			os.Remove(name)
			return err
		}
	}
	return nil
}

func replaceRouteFile(source, target string) error { return os.Rename(source, target) }
