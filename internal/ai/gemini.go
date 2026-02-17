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

	streamArgs := buildGeminiHeadlessArgs(req, true)
	streamOutput, conversationID, streamErr := runJSONStreamingCommand(ctx, req.Workdir, cmdName, streamArgs, onChunk)
	if streamErr == nil {
		return &Result{
			Success:        true,
			Output:         strings.TrimSpace(streamOutput),
			ConversationID: conversationID,
		}, nil
	}

	if looksLikeUnsupportedFlag(streamOutput) || streamOutput == "" {
		fallbackArgs := buildGeminiHeadlessArgs(req, false)
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

func geminiCommand() string {
	for _, name := range []string{"gemini", "gemini-cli"} {
		if path := commandPath(name); path != "" {
			return path
		}
	}
	return ""
}

func buildGeminiHeadlessArgs(req ExecuteRequest, withStreamJSON bool) []string {
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
	args = append(args, "-p", strings.TrimSpace(req.Prompt))
	return args
}
