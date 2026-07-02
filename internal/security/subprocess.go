package security

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// runResult 是一次子进程运行的结果。
type runResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	TimedOut bool
	Err      error
}

func runSubprocess(ctx context.Context, name string, args []string, dir string, timeout time.Duration) runResult {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	r := runResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Err: err}
	if ctx.Err() == context.DeadlineExceeded {
		r.TimedOut = true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		r.ExitCode = exitErr.ExitCode()
	}
	return r
}

// commandExists 检测二进制是否在 PATH。
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
