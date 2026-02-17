package ai

import (
	"context"
	"fmt"
	"strings"
)

// Cursor represents the Cursor AI tool.
type Cursor struct{}

func (c *Cursor) Name() string {
	return "cursor"
}

func (c *Cursor) IsAvailable() bool {
	return cursorAgentCommand() != ""
}

func (c *Cursor) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	return c.ExecuteStream(ctx, ExecuteRequest{
		Workdir: workdir,
		Prompt:  prompt,
	}, nil)
}

func (c *Cursor) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	cmdName := cursorAgentCommand()
	if cmdName == "" {
		return nil, fmt.Errorf("cursor agent CLI not available")
	}

	streamArgs := buildCursorHeadlessArgs(req, true)
	streamOutput, conversationID, streamErr := runJSONStreamingCommand(ctx, req.Workdir, cmdName, streamArgs, onChunk)
	if streamErr == nil {
		return &Result{
			Success:        true,
			Output:         strings.TrimSpace(streamOutput),
			ConversationID: conversationID,
		}, nil
	}

	if looksLikeUnsupportedFlag(streamOutput) || streamOutput == "" {
		fallbackArgs := buildCursorHeadlessArgs(req, false)
		plainOutput, plainErr := runPlainStreamingCommand(ctx, req.Workdir, cmdName, fallbackArgs, onChunk)
		return &Result{
			Success:        plainErr == nil,
			Output:         strings.TrimSpace(plainOutput),
			Error:          plainErr,
			ConversationID: conversationID,
		}, plainErr
	}

	return &Result{
		Success:        false,
		Output:         strings.TrimSpace(streamOutput),
		Error:          streamErr,
		ConversationID: conversationID,
	}, streamErr
}

func cursorAgentCommand() string {
	for _, name := range []string{"cursor-agent", "agent"} {
		if path := commandPath(name); path != "" {
			return path
		}
	}
	return ""
}

func buildCursorHeadlessArgs(req ExecuteRequest, withStreamJSON bool) []string {
	args := []string{"-p", "--force"}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if conversationID := strings.TrimSpace(req.ConversationID); conversationID != "" {
		args = append(args, "--resume", conversationID)
	}
	if withStreamJSON {
		args = append(args, "--output-format", "stream-json")
	}
	args = append(args, strings.TrimSpace(req.Prompt))
	return args
}
