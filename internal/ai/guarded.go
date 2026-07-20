package ai

import (
	"context"
	"os"

	"github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/proc"
	"github.com/darkLord19/foglet/internal/sandbox"
)

// hostGuard builds the deny-list applied to every AI CLI invocation.
//
// A failure to determine either directory degrades to a narrower guard rather
// than to no guard at all, so an unresolvable FOG_HOME still leaves SSH and
// cloud credentials protected.
func hostGuard() sandbox.Guard {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	fogHome, err := env.FogHome()
	if err != nil {
		fogHome = ""
	}
	return sandbox.DefaultGuard(homeDir, fogHome)
}

// runGuardedStreaming executes an AI CLI under host-level restrictions.
//
// Guard setup errors are deliberately non-fatal: the command still runs, just
// unrestricted. Refusing to run would let a transient temp-file failure break a
// user's session, which is a worse outcome than the exposure it avoids.
func runGuardedStreaming(
	ctx context.Context,
	workdir, cmdName string,
	onChunk func([]byte),
	args []string,
) ([]byte, error) {
	wrapped, _ := hostGuard().Wrap(cmdName, args)
	defer wrapped.Cleanup()
	return proc.RunStreaming(ctx, workdir, wrapped.Name, onChunk, wrapped.Args...)
}
