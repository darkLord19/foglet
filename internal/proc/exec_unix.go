//go:build unix

package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
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

// RunStreaming executes a command and emits output chunks as they arrive.
func RunStreaming(ctx context.Context, dir, name string, onChunk func([]byte), args ...string) ([]byte, error) {
	return RunStreamingEnv(ctx, dir, nil, name, onChunk, args...)
}

// RunStreamingEnv is RunStreaming with an explicit environment for the child
// process. A nil env inherits the parent's; a non-nil env replaces it entirely,
// including when empty.
func RunStreamingEnv(ctx context.Context, dir string, env []string, name string, onChunk func([]byte), args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanceled, err)
	}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	var outMu sync.Mutex
	appendChunk := func(chunk []byte) {
		if len(chunk) == 0 {
			return
		}
		copied := make([]byte, len(chunk))
		copy(copied, chunk)
		outMu.Lock()
		_, _ = out.Write(copied)
		outMu.Unlock()
		if onChunk != nil {
			onChunk(copied)
		}
	}

	readerErrCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go streamPipe(stdout, appendChunk, &wg, readerErrCh)
	go streamPipe(stderr, appendChunk, &wg, readerErrCh)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(1500 * time.Millisecond):
			killProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		waitErr = fmt.Errorf("%w: %v", ErrCanceled, ctx.Err())
	}

	wg.Wait()
	close(readerErrCh)

	var readerErr error
	for err := range readerErrCh {
		if err != nil {
			readerErr = err
			break
		}
	}

	outMu.Lock()
	result := append([]byte(nil), out.Bytes()...)
	outMu.Unlock()

	if waitErr != nil {
		return result, waitErr
	}
	if readerErr != nil {
		return result, readerErr
	}
	return result, nil
}

func streamPipe(reader io.Reader, onChunk func([]byte), wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			onChunk(buf[:n])
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			errCh <- nil
			return
		}
		errCh <- err
		return
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
