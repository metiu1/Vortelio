package progress

import (
	"fmt"
	"strings"
)

// Bar renders a single-line progress bar that stays on one line.
// Uses ANSI \033[2K to erase the line on supported terminals (Windows 10+, all Unix).
// Falls back to space-padding on older terminals.
type Bar struct {
	label   string
	width   int
	lastLen int
	ansiOK  bool
}

func NewBar(label string) *Bar {
	// Shorten label if too long (keep terminal line under ~100 chars)
	l := label
	if len(l) > 40 {
		l = "..." + l[len(l)-37:]
	}
	return &Bar{label: l, width: 35, ansiOK: enableANSI()}
}

func (b *Bar) Update(downloaded, total int64) {
	var line string
	if total <= 0 {
		line = fmt.Sprintf("⏬  %s %.1f MB", b.label, float64(downloaded)/1e6)
	} else {
		pct := float64(downloaded) / float64(total)
		if pct > 1 {
			pct = 1
		}
		filled := int(pct * float64(b.width))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", b.width-filled)
		dlMB := float64(downloaded) / 1e6
		totMB := float64(total) / 1e6
		// Use GB for large files
		if totMB > 1000 {
			line = fmt.Sprintf("⏬  %s [%s] %.1f%% (%.2f/%.2f GB)",
				b.label, bar, pct*100, dlMB/1000, totMB/1000)
		} else {
			line = fmt.Sprintf("⏬  %s [%s] %.1f%% (%.0f/%.0f MB)",
				b.label, bar, pct*100, dlMB, totMB)
		}
	}
	b.print(line)
}

func (b *Bar) Done() {
	bar := strings.Repeat("█", b.width)
	b.print(fmt.Sprintf("✅  %s [%s] 100%%", b.label, bar))
	fmt.Println()
}

func (b *Bar) Fail() {
	b.print(fmt.Sprintf("❌  %s fallito.", b.label))
	fmt.Println()
}

func (b *Bar) print(line string) {
	if b.ansiOK {
		// \r = carriage return, \033[2K = erase entire current line
		fmt.Printf("\r\033[2K%s", line)
	} else {
		// Pad with spaces to overwrite any leftover chars from a longer previous line
		runes := []rune(line)
		pad := 0
		if b.lastLen > len(runes) {
			pad = b.lastLen - len(runes)
		}
		fmt.Printf("\r%s%s", line, strings.Repeat(" ", pad))
		b.lastLen = len(runes)
	}
}
