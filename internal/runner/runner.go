package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Result holds the captured output and exit code of a command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes cmd with args, capturing both stdout and stderr.
// stdin is forwarded from os.Stdin.
// A non-zero exit code is not treated as an error.
func Run(cmd string, args []string) (*Result, error) {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin

	var stderrBuf bytes.Buffer
	c.Stderr = &stderrBuf

	out, err := c.Output()
	stderr := stderrBuf.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{
				Stdout:   string(out),
				Stderr:   stderr,
				ExitCode: exitErr.ExitCode(),
			}, nil
		}
		return nil, err
	}
	return &Result{Stdout: string(out), Stderr: stderr, ExitCode: 0}, nil
}

// PassthroughCounted runs cmd forwarding stdin/stderr directly and stdout via
// a counting writer, returning exit code and stdout byte count (for token
// estimation). Used on the miss path to track output size without buffering.
func PassthroughCounted(cmd string, args []string) (exitCode, outputBytes int) {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr

	cw := &countingWriter{w: os.Stdout}
	c.Stdout = cw

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), int(cw.n)
		}
		return 1, int(cw.n)
	}
	return 0, int(cw.n)
}

// countingWriter forwards writes to w while counting total bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	cw.n += int64(len(p))
	return cw.w.Write(p)
}

// Passthrough runs cmd with args, forwarding all stdin/stdout/stderr directly.
// Returns the exit code. This is the transparent fallback path.
func Passthrough(cmd string, args []string) int {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// LogError appends a handler failure to ~/.local/share/tko/errors.log.
func LogError(cmd string, args []string, err error) {
	home, herr := os.UserHomeDir()
	if herr != nil {
		return
	}
	dir := filepath.Join(home, ".local", "share", "tko")
	if merr := os.MkdirAll(dir, 0o755); merr != nil {
		return
	}
	f, ferr := os.OpenFile(filepath.Join(dir, "errors.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s cmd=%q args=%v err=%v\n", time.Now().Format(time.RFC3339), cmd, args, err)
}
