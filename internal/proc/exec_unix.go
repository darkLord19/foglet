//go:build unix

package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// ErrCanceled is returned when a process is stopped due to context cancellation.
var ErrCanceled = errors.New("process canceled")

// Run executes a command in its own process group and returns combined stdout/stderr.
func Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanceled, err)
	}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return out.Bytes(), err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return out.Bytes(), err
	case <-ctx.Done():
		killProcessGroup(cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(1500 * time.Millisecond):
			killProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return out.Bytes(), fmt.Errorf("%w: %v", ErrCanceled, ctx.Err())
	}
}

func killProcessGroup(pid int, signal syscall.Signal) {
	if pid <= 0 {
		return
	}
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, signal)
		return
	}
	_ = syscall.Kill(pid, signal)
}
