//go:build windows

package runtime

import (
	"syscall"
	"unsafe"
)

// SystemRAM returns total and currently-available physical RAM in bytes.
// Uses GlobalMemoryStatusEx (kernel32). Returns 0,0 on failure.
func SystemRAM() (total, available int64) {
	type memoryStatusEx struct {
		Length               uint32
		MemoryLoad           uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}
	mod := syscall.NewLazyDLL("kernel32.dll")
	proc := mod.NewProc("GlobalMemoryStatusEx")
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, 0
	}
	return int64(m.TotalPhys), int64(m.AvailPhys)
}
