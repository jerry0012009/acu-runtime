//go:build !windows

package installfiles

import "os"

func Replace(source, target string) error { return os.Rename(source, target) }
