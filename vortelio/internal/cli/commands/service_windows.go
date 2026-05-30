//go:build windows

package commands

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

// IsServiceRunning checks if the Vortelio server is already running on the given port.
func IsServiceRunning(port string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get("http://localhost:" + port + "/api/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// LaunchServiceDetached starts vortelio-server.exe as a fully detached windowless process.
func LaunchServiceDetached(port string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(self)

	// Try vortelio-server.exe (installer copy) then fall back to self
	candidates := []string{
		filepath.Join(dir, "vortelio-server.exe"),
		self,
	}
	serverExe := self
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			serverExe = c
			break
		}
	}

	cmd := exec.Command(serverExe, "serve", "--port", port, "--no-browser")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}

// LaunchServiceDetachedWithArgs launches vortelio serve with the given args as a detached windowless process.
// Returns the PID and writes it to ~/.vortelio/vortelio.pid.
func LaunchServiceDetachedWithArgs(extraArgs []string) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	dir := filepath.Dir(self)
	candidates := []string{filepath.Join(dir, "vortelio-server.exe"), self}
	serverExe := self
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			serverExe = c
			break
		}
	}
	cmdArgs := append([]string{"serve", "--no-browser"}, extraArgs...)
	cmd := exec.Command(serverExe, cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	pidPath := filepath.Join(config.HomeDir(), "vortelio.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644)
	return pid, nil
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// EnsureServiceRunning ensures the server is running, starting it detached if needed.
func EnsureServiceRunning(port string) (alreadyRunning bool, err error) {
	if IsServiceRunning(port) {
		return true, nil
	}
	fmt.Printf("🚀  Starting Vortelio service in background (port %s)...\n", port)
	if err := LaunchServiceDetached(port); err != nil {
		return false, fmt.Errorf("could not start service: %w", err)
	}
	// Wait up to 10 seconds for the server to respond
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if IsServiceRunning(port) {
			fmt.Printf("✅  Servizio attivo su http://localhost:%s\n\n", port)
			return false, nil
		}
	}
	return false, fmt.Errorf("service did not respond after startup")
}
