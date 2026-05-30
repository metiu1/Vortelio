package runtime

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

// SDCppBin returns path to the sd binary, or "" if not installed.
func SDCppBin() string {
	if p := sdcppVorteBinPath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	name := "sd"
	if runtime.GOOS == "windows" {
		name = "sd.exe"
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func sdcppVorteBinPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	name := "sd"
	if runtime.GOOS == "windows" {
		name = "sd.exe"
	}
	return filepath.Join(home, ".vortelio", "bin", name)
}

// InstallSDCpp downloads the latest stable-diffusion.cpp release into ~/.vortelio/bin/.
func InstallSDCpp(hw *Hardware) error {
	url, assetName, err := sdcppLatestAssetURL(hw)
	if err != nil {
		return fmt.Errorf("cannot find release: %w", err)
	}

	fmt.Printf("📦  Download stable-diffusion.cpp (%s)...\n", assetName)

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp("", "sdcpp-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("download write error: %w", err)
	}
	tmp.Close()

	dest := sdcppVorteBinPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return sdcppExtract(tmpPath, dest)
}

// sdcppExtract finds sd/sd.exe inside the zip and writes it to dest.
func sdcppExtract(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("cannot open zip: %w", err)
	}
	defer r.Close()

	binName := "sd"
	if runtime.GOOS == "windows" {
		binName = "sd.exe"
	}

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if !strings.EqualFold(name, binName) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
		fmt.Printf("✅  stable-diffusion.cpp installed: %s\n", dest)
		return nil
	}
	return fmt.Errorf("sd binary not found in zip archive")
}

type ghRelease struct {
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// sdcppLatestAssetURL queries the GitHub releases API and picks the right asset.
func sdcppLatestAssetURL(hw *Hardware) (url, name string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/leejet/stable-diffusion.cpp/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vortelio")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}

	pattern := sdcppAssetPattern(hw)
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		matched := true
		for _, part := range pattern {
			if !strings.Contains(lower, part) {
				matched = false
				break
			}
		}
		if matched {
			return a.BrowserDownloadURL, a.Name, nil
		}
	}

	// Fallback: CPU build
	fallback := sdcppAssetPatternCPU()
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		matched := true
		for _, part := range fallback {
			if !strings.Contains(lower, part) {
				matched = false
				break
			}
		}
		if matched {
			return a.BrowserDownloadURL, a.Name, nil
		}
	}

	return "", "", fmt.Errorf("no compatible binary found in release assets")
}

func sdcppAssetPattern(hw *Hardware) []string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "windows":
		if hw.Backend == BackendCUDA {
			return []string{"win", "cuda", "x64"}
		}
		if goarch == "arm64" {
			return []string{"win", "arm64"}
		}
		return []string{"win", "avx2", "x64"}
	case "darwin":
		if goarch == "arm64" {
			return []string{"osx", "arm64"}
		}
		return []string{"osx", "x86_64"}
	default: // linux
		if hw.Backend == BackendCUDA {
			return []string{"linux", "cuda", "x64"}
		}
		if hw.Backend == BackendROCm {
			return []string{"linux", "rocm", "x64"}
		}
		return []string{"linux", "avx2", "x64"}
	}
}

func sdcppAssetPatternCPU() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"win", "x64"}
	case "darwin":
		return []string{"osx"}
	default:
		return []string{"linux", "x64"}
	}
}
