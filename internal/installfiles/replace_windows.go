package installfiles

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Windows may retain an executable image briefly after its process exits.
// Wait for that handle to close before treating a normal update as failed.
func Replace(source, target string) error {
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
