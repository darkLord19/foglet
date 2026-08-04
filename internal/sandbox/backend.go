package sandbox

import "context"

// Backend is the process isolation layer for AI tool execution.
//
// nopBackend (runner package) wraps the existing host-guard path.
// MicrosandboxBackend (Slice 2) runs the tool inside a libkrun microVM.
//
// The runner routes the AI step of each session run through the active backend.
// Internal runner calls (commit-message generation, fork summary) remain
// host-side and do not go through the backend.
type Backend interface {
	// Name identifies the backend in logging and run events ("host", "microsandbox").
	Name() string
	// RunTool executes the AI CLI under isolation, streaming chunks via req.OnChunk.
	RunTool(ctx context.Context, req BackendRunRequest) (BackendRunResult, error)
	// Stop signals a running tool for the given session to terminate.
	// Called on every run exit — including cancel — with a fresh context that
	// is not the (already-cancelled) run context.
	Stop(ctx context.Context, sessionID string) error
}

// BackendRunRequest carries all parameters needed for one AI tool invocation.
type BackendRunRequest struct {
	SessionID      string
	ToolName       string
	WorktreePath   string
	Prompt         string
	Model          string
	ConversationID string
	// MCPConfigFile is the pre-resolved path to the MCP JSON config file.
	// The caller owns the file's lifecycle (cleanup is deferred at the call site).
	MCPConfigFile string
	OnChunk       func(string)
}

// BackendRunResult is the output of a successful RunTool call.
type BackendRunResult struct {
	Output         string
	ConversationID string
}
