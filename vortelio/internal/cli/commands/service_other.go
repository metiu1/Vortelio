//go:build !windows

package commands

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

func IsServiceRunning(port string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://localhost:" + port + "/api/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func LaunchServiceDetached(port string) error {
	_, err := LaunchServiceDetachedWithArgs([]string{"--port", port})
	return err
}

func LaunchServiceDetachedWithArgs(extraArgs []string) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmdArgs := append([]string{"serve", "--no-browser"}, extraArgs...)
	cmd := exec.Command(self, cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	pidPath := config.HomeDir() + "/vortelio.pid"
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644)
	return pid, nil
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func EnsureServiceRunning(port string) (bool, error) {
	if IsServiceRunning(port) {
		return true, nil
	}
	fmt.Printf("Avvio servizio Vortelio in background (porta %s)...\n", port)
	if err := LaunchServiceDetached(port); err != nil {
		return false, fmt.Errorf("impossibile avviare il servizio: %w", err)
	}
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if IsServiceRunning(port) {
			fmt.Printf("Servizio attivo su http://localhost:%s\n\n", port)
			return false, nil
		}
	}
	return false, fmt.Errorf("il servizio non risponde dopo l'avvio")
}
