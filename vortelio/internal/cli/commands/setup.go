package commands

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SetupCommand downloads and installs llama.cpp automatically.
type SetupCommand struct{}

func NewSetupCommand() *SetupCommand { return &SetupCommand{} }

func (c *SetupCommand) Name() string { return "setup" }

func (c *SetupCommand) Run(args []string) error {
	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	fmt.Println("🔧  Vortelio Setup — configurazione automatica")
	fmt.Println()

	// ── Step 1: Verifica llama.cpp ───────────────────────────
	fmt.Print("1. Checking llama.cpp... ")
	llamaBin := findLlamaCLI()
	if llamaBin != "" && !force {
		fmt.Printf("✅  found at %s\n", llamaBin)
	} else {
		if force {
			fmt.Println("(re-installing)")
		} else {
			fmt.Println("❌  not found")
		}
		fmt.Println("   → Download llama.cpp in corso...")
		if err := downloadLlama(); err != nil {
			fmt.Printf("   ⚠️   Download failed: %v\n", err)
			fmt.Println("   Install manually from: https://github.com/ggerganov/llama.cpp/releases")
		} else {
			fmt.Println("   ✅  llama.cpp installed")
		}
	}

	// ── Step 2: Verifica Python ──────────────────────────────
	fmt.Print("2. Checking Python 3... ")
	py := findPython()
	if py != "" {
		out, _ := exec.Command(py, "--version").CombinedOutput()
		fmt.Printf("✅  %s\n", strings.TrimSpace(string(out)))
	} else {
		fmt.Println("⚠️   not found")
		fmt.Println("   Necessario per immagini, audio e video.")
		printPythonInstallHint()
	}

	// ── Step 3: Verifica pacchetti Python ────────────────────
	if py != "" {
		fmt.Print("3. Checking Python packages (diffusers, torch, whisper)... ")
		missing := checkPythonPackages(py)
		if len(missing) == 0 {
			fmt.Println("✅  all installed")
		} else {
			fmt.Printf("⚠️   mancanti: %s\n", strings.Join(missing, ", "))
			fmt.Println("   → Installing...")
			installPythonPackages(py, missing)
		}
	}

	// ── Step 4: PATH check ───────────────────────────────────
	fmt.Print("4. Checking system PATH... ")
	selfDir := ""
	if exe, err := os.Executable(); err == nil {
		selfDir = filepath.Dir(exe)
	}
	if isInPath(selfDir) {
		fmt.Println("✅  vortelio available globally")
	} else {
		fmt.Printf("⚠️   %s is not in PATH\n", selfDir)
		if runtime.GOOS == "windows" {
			fmt.Println("   → Adding to system PATH...")
			addToPathWindows(selfDir)
		} else {
			fmt.Printf("   Add manually: export PATH=\"%s:$PATH\"\n", selfDir)
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅  Setup complete! Open a new terminal and try:")
	fmt.Println()
	fmt.Println("   vortelio pull llm/mistral:7b")
	fmt.Println("   vortelio run llm/mistral:7b \"hello!\"")
	fmt.Println()
	fmt.Println("   vortelio pull image/sdxl")
	fmt.Println("   vortelio run image/sdxl \"a sunset over the sea\"")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return nil
}

// ── llama.cpp download ──────────────────────────────────────

func downloadLlama() error {
	binDir := llamaBinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	// Get latest release from GitHub API
	url, err := getLatestLlamaURL()
	if err != nil {
		return fmt.Errorf("could not get release URL: %w", err)
	}

	fmt.Printf("   URL: %s\n", url)
	zipPath := filepath.Join(os.TempDir(), "llama-cpp-download.zip")

	// Download
	if err := downloadFile(url, zipPath, true); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	// Extract
	fmt.Print("   Estrazione... ")
	if err := extractZip(zipPath, binDir); err != nil {
		return err
	}
	fmt.Println("✅")

	// Add to PATH on Windows
	if runtime.GOOS == "windows" {
		addToPathWindows(binDir)
	} else {
		// Symlink on Unix
		for _, name := range []string{"llama-cli", "llama", "main"} {
			src := filepath.Join(binDir, name)
			if _, err := os.Stat(src); err == nil {
				os.Chmod(src, 0755)
				dest := "/usr/local/bin/llama-cli"
				os.Remove(dest)
				os.Symlink(src, dest)
				break
			}
		}
	}
	return nil
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func getLatestLlamaURL() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/ggerganov/llama.cpp/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}

	// Pick platform-appropriate asset
	var want []string
	switch runtime.GOOS {
	case "windows":
		want = []string{"win", "cpu", "x64", ".zip"}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			want = []string{"macos", "arm64", ".zip"}
		} else {
			want = []string{"macos", "x64", ".zip"}
		}
	default:
		want = []string{"linux", "x64", ".zip"}
	}

	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		match := true
		for _, w := range want {
			if !strings.Contains(name, w) {
				match = false
				break
			}
		}
		if match {
			return a.BrowserDownloadURL, nil
		}
	}

	// Fallback: any zip for the platform
	goos := runtime.GOOS
	if goos == "darwin" {
		goos = "macos"
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, goos) && strings.HasSuffix(name, ".zip") {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no asset found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
}

func downloadFile(url, dest string, showProgress bool) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			f.Write(buf[:n])
			downloaded += int64(n)
			if showProgress && total > 0 {
				pct := float64(downloaded) / float64(total) * 100
				fmt.Printf("\r   Download: %.0f%% (%.1f/%.1f MB)", pct, float64(downloaded)/1e6, float64(total)/1e6)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if showProgress {
		fmt.Println()
	}
	return nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		// Only extract exe/dll/so files (skip dirs and unneeded files)
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".exe" && ext != ".dll" && ext != ".so" && ext == "" {
			// On Unix, no extension = binary
			if runtime.GOOS == "windows" {
				continue
			}
		}

		dest := filepath.Join(destDir, name)
		if err := extractFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	if err != nil {
		return err
	}
	// Make executable on Unix
	if runtime.GOOS != "windows" {
		out.Chmod(0755)
	}
	return nil
}

// ── Helpers ────────────────────────────────────────────────

func llamaBinDir() string {
	// Put llama.cpp next to vortelio.exe in bin/
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vortelio", "bin")
}

func findLlamaCLI() string {
	names := []string{"llama-cli", "llama-cli.exe", "llama", "llama.exe", "main", "main.exe"}
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	// Check bin/ next to vortelio.exe
	binDir := llamaBinDir()
	for _, name := range names {
		p := filepath.Join(binDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findPython() string {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func checkPythonPackages(py string) []string {
	packages := []string{"diffusers", "torch", "whisper", "transformers"}
	var missing []string
	for _, pkg := range packages {
		out, _ := exec.Command(py, "-c", "import "+pkg).CombinedOutput()
		if len(out) > 0 {
			missing = append(missing, pkg)
		}
	}
	return missing
}

func installPythonPackages(py string, pkgs []string) {
	args := append([]string{"-m", "pip", "install", "--quiet"}, pkgs...)
	cmd := exec.Command(py, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("   ⚠️   Partial installation: %v\n", err)
	} else {
		fmt.Println("   ✅  Packages installed")
	}
}

func isInPath(dir string) bool {
	pathEnv := os.Getenv("PATH")
	if runtime.GOOS == "windows" {
		pathEnv = strings.ToLower(pathEnv)
		dir = strings.ToLower(dir)
	}
	for _, p := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if p == dir {
			return true
		}
	}
	return false
}

func addToPathWindows(dir string) {
	cmd := exec.Command("powershell", "-NonInteractive", "-Command",
		fmt.Sprintf(`$p=[Environment]::GetEnvironmentVariable("Path","Machine"); if($p -notlike "*%s*"){[Environment]::SetEnvironmentVariable("Path","$p;%s","Machine"); Write-Host "PATH updated"}else{Write-Host "PATH already configured"}`, dir, dir),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func printPythonInstallHint() {
	switch runtime.GOOS {
	case "windows":
		fmt.Println("   Download from: https://www.python.org/downloads/")
		fmt.Println("   Oppure: winget install Python.Python.3.12")
	case "darwin":
		fmt.Println("   brew install python@3.12")
	default:
		fmt.Println("   sudo apt install python3 python3-pip  # Ubuntu/Debian")
	}
}
