//go:build darwin

package runtime

import (
	"os/exec"
	"strconv"
	"strings"
)

// SystemRAM returns total physical RAM (hw.memsize) and an estimate of
// available RAM (free + inactive pages from vm_stat) in bytes.
func SystemRAM() (total, available int64) {
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if v, e := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); e == nil {
			total = v
		}
	}
	available = darwinAvailRAM()
	if available == 0 {
		available = total
	}
	return total, available
}

// darwinAvailRAM sums free + inactive pages from vm_stat (a reasonable proxy
// for memory a new process can claim).
func darwinAvailRAM() int64 {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}
	pageSize := int64(4096)
	var freePages, inactivePages int64
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.Index(line, "page size of "); i >= 0 {
			fields := strings.Fields(line[i+len("page size of "):])
			if len(fields) > 0 {
				if v, e := strconv.ParseInt(fields[0], 10, 64); e == nil {
					pageSize = v
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
		n, _ := strconv.ParseInt(val, 10, 64)
		switch key {
		case "Pages free":
			freePages = n
		case "Pages inactive":
			inactivePages = n
		}
	}
	return (freePages + inactivePages) * pageSize
}
