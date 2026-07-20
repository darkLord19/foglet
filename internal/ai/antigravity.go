package ai

import (
	"context"
	"fmt"
	"strings"
)

// Antigravity represents Google's Antigravity CLI (`agy`) coding agent.
type Antigravity struct{}

func (a *Antigravity) Name() string {
	return "antigravity"
}

func (a *Antigravity) IsAvailable() bool {
	return antigravityCommand() != ""
}

func (a *Antigravity) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	return a.ExecuteStream(ctx, ExecuteRequest{
		Workdir: workdir,
		Prompt:  prompt,
	}, nil)
}

func (a *Antigravity) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	cmdName := antigravityCommand()
	if cmdName == "" {
		return nil, fmt.Errorf("antigravity CLI not available")
	}

	streamArgs := buildAntigravityHeadlessArgs(req, true, true)
	streamOutput, conversationID, streamErr := runJSONStreamingCommand(ctx, a.Name(), req.Workdir, cmdName, streamArgs, onChunk)
	if streamErr == nil {
		return &Result{
			Success:        true,
			Output:         strings.TrimSpace(streamOutput),
			ConversationID: conversationID,
		}, nil
	}

	if looksLikeUnsupportedFlag(streamOutput) || streamOutput == "" {
		// Retry without auto-approve, which is not supported by all versions.
		retryArgs := buildAntigravityHeadlessArgs(req, true, false)
		retryOutput, retryConversationID, retryErr := runJSONStreamingCommand(ctx, a.Name(), req.Workdir, cmdName, retryArgs, onChunk)
		if retryErr == nil {
			if retryConversationID == "" {
				retryConversationID = conversationID
			}
			return &Result{
				Success:        true,
				Output:         strings.TrimSpace(retryOutput),
				ConversationID: retryConversationID,
			}, nil
		}
		if conversationID == "" {
			conversationID = retryConversationID
		}

		fallbackArgs := buildAntigravityHeadlessArgs(req, false, true)
		plainOutput, plainErr := runPlainStreamingCommand(ctx, a.Name(), req.Workdir, cmdName, fallbackArgs, onChunk)
		if plainErr != nil && (looksLikeUnsupportedFlag(plainOutput) || plainOutput == "") {
			noApproveArgs := buildAntigravityHeadlessArgs(req, false, false)
			plainOutput, plainErr = runPlainStreamingCommand(ctx, a.Name(), req.Workdir, cmdName, noApproveArgs, onChunk)
		}
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

func antigravityCommand() string {
	for _, name := range []string{"agy", "antigravity"} {
		if path := commandPath(name); path != "" {
			return path
		}
	}
	return ""
}

func buildAntigravityHeadlessArgs(req ExecuteRequest, withStreamJSON, withAutoApprove bool) []string {
	args := make([]string, 0, 8)
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if conversationID := strings.TrimSpace(req.ConversationID); conversationID != "" {
		args = append(args, "--conversation", conversationID)
	}
	if withStreamJSON {
		args = append(args, "--output-format", "stream-json")
	}
	if withAutoApprove {
		args = append(args, "--dangerously-skip-permissions")
	}
	args = append(args, "-p", strings.TrimSpace(req.Prompt))
	return args
}
