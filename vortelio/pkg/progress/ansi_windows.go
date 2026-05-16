//go:build windows

package progress

import (
	"os"
	"syscall"
	"unsafe"
)

func enableANSI() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle := os.Stdout.Fd()
	var mode uint32
	r, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return false
	}
	const ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	r, _, _ = setConsoleMode.Call(handle, uintptr(mode|ENABLE_VIRTUAL_TERMINAL_PROCESSING))
	return r != 0
}
