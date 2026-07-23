//go:build windows

package tui

import (
	"syscall"
	"unsafe"
)

func platformLocale() string {
	buffer := make([]uint16, 85)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	length, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer)
}
