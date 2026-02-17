package ai

import (
	"context"
	"fmt"
	"strings"
)

// Gemini represents the Gemini CLI tool.
type Gemini struct{}

func (g *Gemini) Name() string {
	return "gemini"
}

func (g *Gemini) IsAvailable() bool {
	return geminiCommand() != ""
}

func (g *Gemini) Execute(ctx context.Context, workdir, prompt string) (*Result, error) {
	return g.ExecuteStream(ctx, ExecuteRequest{
		Workdir: workdir,
		Prompt:  prompt,
	}, nil)
}

func (g *Gemini) ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error) {
	cmdName := geminiCommand()
	if cmdName == "" {
		return nil, fmt.Errorf("gemini CLI not available")
	}

	streamArgs := buildGeminiHeadlessArgs(req, true, true)
	streamOutput, conversationID, streamErr := runJSONStreamingCommand(ctx, req.Workdir, cmdName, streamArgs, onChunk)
	if streamErr == nil {
		return &Result{
			Success:        true,
			Output:         strings.TrimSpace(streamOutput),
			ConversationID: conversationID,
		}, nil
	}

	if looksLikeUnsupportedFlag(streamOutput) || streamOutput == "" {
		// Retry without --yolo, which is not supported by all versions.
		retryArgs := buildGeminiHeadlessArgs(req, true, false)
		retryOutput, retryConversationID, retryErr := runJSONStreamingCommand(ctx, req.Workdir, cmdName, retryArgs, onChunk)
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

		fallbackArgs := buildGeminiHeadlessArgs(req, false, true)
		plainOutput, plainErr := runPlainStreamingCommand(ctx, req.Workdir, cmdName, fallbackArgs, onChunk)
		if plainErr != nil && (looksLikeUnsupportedFlag(plainOutput) || plainOutput == "") {
			noYoloArgs := buildGeminiHeadlessArgs(req, false, false)
			plainOutput, plainErr = runPlainStreamingCommand(ctx, req.Workdir, cmdName, noYoloArgs, onChunk)
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

func geminiCommand() string {
	for _, name := range []string{"gemini", "gemini-cli"} {
		if path := commandPath(name); path != "" {
			return path
		}
	}
	return ""
}

func buildGeminiHeadlessArgs(req ExecuteRequest, withStreamJSON, withYolo bool) []string {
	args := make([]string, 0, 8)
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if conversationID := strings.TrimSpace(req.ConversationID); conversationID != "" {
		args = append(args, "--resume", conversationID)
	}
	if withStreamJSON {
		args = append(args, "--output-format", "stream-json")
	}
	if withYolo {
		args = append(args, "--yolo")
	}
	args = append(args, "-p", strings.TrimSpace(req.Prompt))
	return args
}
