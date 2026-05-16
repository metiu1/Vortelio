//go:build !windows

package progress

// enableANSI on Unix: terminals always support ANSI.
func enableANSI() bool {
	return true
}
