//go:build !windows && !linux && !darwin

package runtime

// SystemRAM is unsupported on this platform.
func SystemRAM() (total, available int64) { return 0, 0 }
