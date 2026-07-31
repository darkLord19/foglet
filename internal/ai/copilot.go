package ai

import (
	"context"
	"fmt"
	"strings"
)

// Copilot represents the GitHub Copilot CLI.
type Copilot struct{}

func (c *Copilot) Name() string { return "copilot" }

func (c *Copilot) IsAvailable() bool { return copilotCommand() != "" }

func (c *Copilot) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	cmd := copilotCommand()
	if cmd == "" {
		return nil, fmt.Errorf("copilot CLI not available")
	}

	streamArgs := buildCopilotArgs(req, true)
	output, conversationID, err := runJSONStreamingCommand(ctx, c.Name(), req.Workdir, cmd, streamArgs, onChunk)
	if err == nil {
		return &Result{Success: true, Output: strings.TrimSpace(output), ConversationID: conversationID}, nil
	}

	if looksLikeUnsupportedFlag(output) || strings.TrimSpace(output) == "" {
		plainOutput, plainErr := runPlainStreamingCommand(ctx, c.Name(), req.Workdir, cmd, buildCopilotArgs(req, false), onChunk)
		return &Result{
			Success:        plainErr == nil,
			Output:         strings.TrimSpace(plainOutput),
			Error:          plainErr,
			ConversationID: conversationID,
		}, plainErr
	}

	return &Result{Success: false, Output: strings.TrimSpace(output), Error: err, ConversationID: conversationID}, err
}

func copilotCommand() string { return commandPath("copilot") }

func buildCopilotArgs(req ExecuteRequest, withJSON bool) []string {
	args := []string{"-p", "--allow-all-tools"}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if withJSON {
		args = append(args, "--output-format", "json")
	}
	args = append(args, strings.TrimSpace(req.Prompt))
	return args
}
