//go:build !unix

package proc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
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
