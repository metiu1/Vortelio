//go:build !windows

package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func HideWindow(cmd *exec.Cmd) *exec.Cmd { return cmd }

func RunWithOutput(cmd *exec.Cmd, stdout, stderr io.Writer) error {
	cmd.Stdout = stdout; cmd.Stderr = stderr
	return cmd.Run()
}

func LaunchDetached(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdout = nil; cmd.Stderr = nil; cmd.Stdin = nil
	return cmd.Start()
}

func RunWithCapture(cmd *exec.Cmd) error {
	var buf bytes.Buffer
	cmd.Stdout = &buf; cmd.Stderr = &buf
	err := cmd.Run()
	return wrapCaptureErr(err, buf.String())
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
