package ai

import (
	"context"
	"fmt"
	"strings"
)

// Codex represents the OpenAI Codex CLI.
type Codex struct{}

func (c *Codex) Name() string { return "codex" }

func (c *Codex) IsAvailable() bool { return codexCommand() != "" }

func (c *Codex) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	cmd := codexCommand()
	if cmd == "" {
		return nil, fmt.Errorf("codex CLI not available")
	}

	streamArgs := buildCodexArgs(req, true)
	output, conversationID, err := runJSONStreamingCommand(ctx, c.Name(), req.Workdir, cmd, streamArgs, onChunk)
	if err == nil {
		return &Result{Success: true, Output: strings.TrimSpace(output), ConversationID: conversationID}, nil
	}

	// Older Codex CLI versions may not understand --json. Keep plain output
	// as a compatibility path, matching the other adapters.
	if looksLikeUnsupportedFlag(output) || strings.TrimSpace(output) == "" {
		plainOutput, plainErr := runPlainStreamingCommand(ctx, c.Name(), req.Workdir, cmd, buildCodexArgs(req, false), onChunk)
		return &Result{
			Success:        plainErr == nil,
			Output:         strings.TrimSpace(plainOutput),
			Error:          plainErr,
			ConversationID: conversationID,
		}, plainErr
	}

	return &Result{Success: false, Output: strings.TrimSpace(output), Error: err, ConversationID: conversationID}, err
}

func codexCommand() string { return commandPath("codex") }

func buildCodexArgs(req ExecuteRequest, withJSON bool) []string {
	args := []string{"exec"}
	if conversationID := strings.TrimSpace(req.ConversationID); conversationID != "" {
		args = append(args, "resume", conversationID)
	}
	if withJSON {
		args = append(args, "--json")
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, strings.TrimSpace(req.Prompt))
	return args
}
