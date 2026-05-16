package runtime

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultOutputDir returns the user's Downloads folder, or home if not found.
func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	// Windows: %USERPROFILE%\Downloads
	// macOS/Linux: ~/Downloads
	dl := filepath.Join(home, "Downloads")
	if _, err := os.Stat(dl); err == nil {
		return dl
	}
	// Italian Windows: Scaricati
	if runtime.GOOS == "windows" {
		for _, name := range []string{"Scaricati", "Téléchargements", "Descargas"} {
			p := filepath.Join(home, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return home
}

// ResolveOutputPath returns the full path for an output file.
// If outputFile is already absolute or relative with a dir, return as-is.
// Otherwise place it in the default output dir.
func ResolveOutputPath(outputFile, defaultName string) string {
	if outputFile == "" {
		outputFile = defaultName
	}
	// If it's just a filename (no directory component), put in Downloads
	if filepath.Dir(outputFile) == "." {
		return filepath.Join(DefaultOutputDir(), outputFile)
	}
	return outputFile
}
