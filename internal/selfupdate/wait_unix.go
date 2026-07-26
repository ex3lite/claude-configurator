//go:build !windows

package selfupdate

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

func waitForProcessExit(processID int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(processID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit within %s", processID, timeout)
}
