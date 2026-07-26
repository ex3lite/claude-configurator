//go:build windows

package selfupdate

import (
	"fmt"
	"syscall"
	"time"
)

func waitForProcessExit(processID int, timeout time.Duration) error {
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(processID))
	if err != nil {
		if err == syscall.Errno(87) { // ERROR_INVALID_PARAMETER: process is already gone.
			return nil
		}
		return err
	}
	defer syscall.CloseHandle(handle)

	result, err := syscall.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return err
	}
	if result == syscall.WAIT_OBJECT_0 {
		return nil
	}
	if result == syscall.WAIT_TIMEOUT {
		return fmt.Errorf("process %d did not exit within %s", processID, timeout)
	}
	return fmt.Errorf("unexpected process wait result: %d", result)
}
