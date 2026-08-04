package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/sandbox"
)

// nopBackend is the host-guard backend used when VM sandboxing is disabled.
//
// It delegates to the existing ai.Tool.ExecuteStream path, which internally
// applies the filesystem deny-list and environment filtering via
// internal/ai/guarded.go. No behavioral change from the pre-backend path.
type nopBackend struct {
	tools ToolFactory
}

func (b *nopBackend) Name() string { return "host" }

// Stop is a no-op: cancellation is handled by the run context, which causes
// proc.RunStreamingEnv to send SIGTERM/SIGKILL to the child process group.
func (b *nopBackend) Stop(_ context.Context, _ string) error { return nil }

func (b *nopBackend) RunTool(ctx context.Context, req sandbox.BackendRunRequest) (sandbox.BackendRunResult, error) {
	tool, err := b.tools(req.ToolName)
	if err != nil {
		return sandbox.BackendRunResult{}, err
	}
	if !tool.IsAvailable() {
		return sandbox.BackendRunResult{}, fmt.Errorf("AI tool %s not available", req.ToolName)
	}

	result, err := tool.ExecuteStream(ctx, ai.ExecuteRequest{
		Workdir:        req.WorktreePath,
		Prompt:         req.Prompt,
		Model:          req.Model,
		ConversationID: req.ConversationID,
		MCPConfigFile:  req.MCPConfigFile,
	}, req.OnChunk)

	if result == nil {
		return sandbox.BackendRunResult{}, err
	}
	out := sandbox.BackendRunResult{
		Output:         strings.TrimSpace(result.Output),
		ConversationID: strings.TrimSpace(result.ConversationID),
	}
	if err != nil {
		return out, err
	}
	if !result.Success {
		return out, fmt.Errorf("AI execution failed: %s", out.Output)
	}
	return out, nil
}

// compile-time proof that nopBackend satisfies the Backend interface.
var _ sandbox.Backend = (*nopBackend)(nil)
