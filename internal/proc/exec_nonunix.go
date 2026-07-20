//go:build !unix

package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// ErrCanceled is returned when a process is stopped due to context cancellation.
var ErrCanceled = errors.New("process canceled")

// Run executes a command and returns combined stdout/stderr.
func Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("%w: %v", ErrCanceled, ctx.Err())
	}
	return out, err
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

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env

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
	go streamPipeNonUnix(stdout, appendChunk, &wg, readerErrCh)
	go streamPipeNonUnix(stderr, appendChunk, &wg, readerErrCh)

	waitErr := cmd.Wait()
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

	if ctx.Err() != nil {
		return result, fmt.Errorf("%w: %v", ErrCanceled, ctx.Err())
	}
	if waitErr != nil {
		return result, waitErr
	}
	if readerErr != nil {
		return result, readerErr
	}
	return result, nil
}

func streamPipeNonUnix(reader io.Reader, onChunk func([]byte), wg *sync.WaitGroup, errCh chan<- error) {
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
