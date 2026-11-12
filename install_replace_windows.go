package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"time"
)

// Windows may retain an executable image briefly after its process exits.
// Wait for that handle to close before treating a normal update as failed.
func replaceInstalledPath(source, target string) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := os.Rename(source, target)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) || (!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION)) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
}
