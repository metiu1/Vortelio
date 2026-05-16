package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	createNoWindow        = 0x08000000
	startfUseshowwindow   = 0x00000001
)

func HideWindow(cmd *exec.Cmd) *exec.Cmd {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
	cmd.SysProcAttr.HideWindow = true
	si := &syscall.StartupInfo{}
	si.Flags = startfUseshowwindow
	si.ShowWindow = 0
	_ = si
	_ = unsafe.Sizeof(si)
	return cmd
}

func RunWithOutput(cmd *exec.Cmd, stdout, stderr io.Writer) error {
	HideWindow(cmd)
	stdoutPipe, err1 := cmd.StdoutPipe()
	stderrPipe, err2 := cmd.StderrPipe()
	if err1 != nil || err2 != nil {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{}, 2)
	go func() {
		if stdout != nil { io.Copy(stdout, stdoutPipe) }
		done <- struct{}{}
	}()
	go func() {
		if stderr != nil { io.Copy(stderr, stderrPipe) }
		done <- struct{}{}
	}()
	<-done; <-done
	return cmd.Wait()
}

func LaunchDetached(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = createNoWindow | syscall.CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr.HideWindow = true
	cmd.Stdout = nil; cmd.Stderr = nil; cmd.Stdin = nil
	return cmd.Start()
}

func SafePrintf(format string, args ...interface{}) {
	if os.Stdout != nil { _ = format }
}

func RunWithCapture(cmd *exec.Cmd) error {
	HideWindow(cmd)
	var buf bytes.Buffer
	stdoutPipe, err1 := cmd.StdoutPipe()
	stderrPipe, err2 := cmd.StderrPipe()
	if err1 != nil || err2 != nil {
		cmd.Stdout = &buf; cmd.Stderr = &buf
		return wrapCaptureErr(cmd.Run(), buf.String())
	}
	if err := cmd.Start(); err != nil { return err }
	done := make(chan struct{}, 2)
	go func() { io.Copy(&buf, stdoutPipe); done <- struct{}{} }()
	go func() { io.Copy(&buf, stderrPipe); done <- struct{}{} }()
	<-done; <-done
	return wrapCaptureErr(cmd.Wait(), buf.String())
}

func wrapCaptureErr(err error, output string) error {
	if err == nil { return nil }
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 30 { lines = lines[len(lines)-30:] }
	tail := strings.TrimSpace(strings.Join(lines, "\n"))
	if tail == "" { return fmt.Errorf("%w", err) }
	return fmt.Errorf("%w\n\nDettagli processo:\n%s", err, tail)
}

type ProgressEvent struct {
	Percent int
	Message string
}

func RunWithProgress(cmd *exec.Cmd, progress chan<- ProgressEvent) error {
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		if progress != nil { close(progress) }
		return pipeErr
	}
	if err := cmd.Start(); err != nil {
		if progress != nil { close(progress) }
		return err
	}
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 512*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VORTELIO_PROGRESS:") {
			parts := strings.SplitN(strings.TrimPrefix(line, "VORTELIO_PROGRESS:"), ":", 2)
			pct := 0
			fmt.Sscanf(parts[0], "%d", &pct)
			msg := ""
			if len(parts) > 1 { msg = parts[1] }
			if progress != nil { progress <- ProgressEvent{Percent: pct, Message: msg} }
		}
	}
	err := cmd.Wait()
	if progress != nil { close(progress) }
	if err != nil { return wrapCaptureErr(err, errBuf.String()) }
	return nil
}
