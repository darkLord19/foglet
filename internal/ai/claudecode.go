package ai

import (
	"context"
	"fmt"
	"strings"
)

// ClaudeCode represents the Claude Code AI tool
type ClaudeCode struct{}

func (c *ClaudeCode) Name() string {
	return "claude"
}

func (c *ClaudeCode) IsAvailable() bool {
	return commandExists("claude") || commandExists("claude-code")
}

func (c *ClaudeCode) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	return c.ExecuteStream(ctx, ExecuteRequest{
		Workdir: workdir,
		Prompt:  prompt,
	}, nil)
}

func (c *ClaudeCode) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("claude not available")
	}

	cmdName := claudeCommand()
	args := []string{"-p", strings.TrimSpace(req.Prompt)}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if conversationID := strings.TrimSpace(req.ConversationID); conversationID != "" {
		args = append(args, "--resume", conversationID)
	}

	streamArgs := append(append([]string{}, args...), "--output-format", "stream-json")
	output, conversationID, err := runJSONStreamingCommand(ctx, req.Workdir, cmdName, streamArgs, onChunk)

	if err != nil && (looksLikeUnsupportedFlag(output) || strings.TrimSpace(output) == "") {
		plainOutput, plainErr := runPlainStreamingCommand(ctx, req.Workdir, cmdName, args, onChunk)
		result := &Result{
			Success:        plainErr == nil,
			Output:         strings.TrimSpace(plainOutput),
			Error:          plainErr,
			ConversationID: conversationID,
		}
		if plainErr != nil {
			return result, plainErr
		}
		return result, nil
	}

	result := &Result{
		Success:        err == nil,
		Output:         strings.TrimSpace(output),
		Error:          err,
		ConversationID: conversationID,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func claudeCommand() string {
	if path := commandPath("claude"); path != "" {
		return path
	}
	return commandPath("claude-code")
}
