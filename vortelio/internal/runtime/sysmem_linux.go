//go:build linux

package runtime

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// SystemRAM returns total and available physical RAM in bytes, read from
// /proc/meminfo. Returns 0,0 on failure.
func SystemRAM() (total, available int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = kb * 1024
		case "MemAvailable:":
			available = kb * 1024
		}
	}
	if available == 0 {
		available = total
	}
	return total, available
}
