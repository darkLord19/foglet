//go:build !cgo

package runner

import (
	"context"
	"errors"

	"github.com/darkLord19/foglet/internal/sandbox"
)

// MicrosandboxBackend is a compile-time stub used when CGO is unavailable.
// All methods return errors so callers fail fast rather than silently
// falling back to the host backend.
type MicrosandboxBackend struct{}

func NewMicrosandboxBackend() *MicrosandboxBackend { return &MicrosandboxBackend{} }

func (b *MicrosandboxBackend) Name() string { return "microsandbox" }

func (b *MicrosandboxBackend) EnsureReady(_ context.Context) error {
	return errors.New("microsandbox requires CGO; rebuild with CGO_ENABLED=1")
}

func (b *MicrosandboxBackend) Stop(_ context.Context, _ string) error { return nil }

func (b *MicrosandboxBackend) RunTool(_ context.Context, _ sandbox.BackendRunRequest) (sandbox.BackendRunResult, error) {
	return sandbox.BackendRunResult{}, errors.New("microsandbox requires CGO; rebuild with CGO_ENABLED=1")
}

var _ sandbox.Backend = (*MicrosandboxBackend)(nil)
