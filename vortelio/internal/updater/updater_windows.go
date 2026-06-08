//go:build windows

package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func StartDetached(restartGUI bool) (StartResult, error) {
	uv, err := exec.LookPath("uv")
	if err != nil {
		return StartResult{}, errors.New("uv non trovato nel PATH. Installa uv e riprova")
	}
	return startWindowsUpdater(uv, os.Getpid(), restartGUI, filepath.Join(os.TempDir(), "vortelio-update.log"))
}

func startWindowsUpdater(uv string, pid int, restartGUI bool, logPath string) (StartResult, error) {
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("vortelio-update-%d.ps1", pid))
	restartLine := ""
	if restartGUI {
		restartLine = "if ($code -eq 0) { Start-Process -FilePath 'vortelio' -ArgumentList 'gui' -WindowStyle Hidden }\n"
	}
	script := fmt.Sprintf(`$ErrorActionPreference = 'Continue'
$log = %q
"Vortelio update started at $(Get-Date -Format o)" | Out-File -FilePath $log -Encoding utf8
try { Wait-Process -Id %d -ErrorAction SilentlyContinue } catch {}
Start-Sleep -Milliseconds 1200
& %q tool install --force %q *> $log
$code = $LASTEXITCODE
"Exit code: $code" | Out-File -FilePath $log -Append -Encoding utf8
%sexit $code
`, logPath, pid, uv, RepoInstallSpec, restartLine)
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return StartResult{}, err
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	if err := cmd.Start(); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "powershell") {
			return StartResult{}, fmt.Errorf("impossibile avviare PowerShell per l'aggiornamento: %w", err)
		}
		return StartResult{}, err
	}
	return StartResult{
		Started: true,
		Message: "Aggiornamento avviato. Vortelio si chiudera' e uv installera' la nuova versione.",
		LogPath: logPath,
	}, nil
}
