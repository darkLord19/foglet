package ai

import (
	"context"
	"os"

	"github.com/darkLord19/foglet/internal/env"
	"github.com/darkLord19/foglet/internal/proc"
	"github.com/darkLord19/foglet/internal/sandbox"
)

// toolEnvPrefixes names the environment families each tool legitimately needs.
//
// Scoping per tool keeps one agent's credentials out of another's environment.
// Deliberately absent: GOOGLE_, which would admit GOOGLE_APPLICATION_CREDENTIALS
// and hand Antigravity a path to the user's GCP service account key — exactly
// the cross-service leak this filtering exists to prevent.
var toolEnvPrefixes = map[string][]string{
	"claude":      {"ANTHROPIC_", "CLAUDE_"},
	"cursor":      {"CURSOR_"},
	"antigravity": {"ANTIGRAVITY_", "AGY_", "GEMINI_"},
}

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

// runGuardedStreaming executes an AI CLI under host-level restrictions: a
// filesystem deny-list plus an environment reduced to what the named tool needs.
//
// Guard setup errors are deliberately non-fatal — the command still runs, just
// unrestricted. Refusing to run would let a transient temp-file failure break a
// user's session, which is a worse outcome than the exposure it avoids.
func runGuardedStreaming(
	ctx context.Context,
	toolName, workdir, cmdName string,
	onChunk func([]byte),
	args []string,
) ([]byte, error) {
	wrapped, _ := hostGuard().Wrap(cmdName, args)
	defer wrapped.Cleanup()

	childEnv := sandbox.FilterEnv(os.Environ(), toolEnvPrefixes[toolName])
	return proc.RunStreamingEnv(ctx, workdir, childEnv, wrapped.Name, onChunk, wrapped.Args...)
}
