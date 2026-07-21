package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/git"
	"github.com/darkLord19/foglet/internal/proc"
	"github.com/darkLord19/foglet/internal/state"
)

func (r *Runner) runTool(ctx context.Context, toolName, workdir, prompt string) (string, error) {
	output, _, err := r.runToolWithOptions(ctx, toolName, workdir, prompt, "", "", nil)
	return output, err
}

func (r *Runner) runToolWithOptions(
	ctx context.Context,
	toolName, workdir, prompt, model, conversationID string,
	onChunk func(string),
) (string, string, error) {
	tool, err := r.tools(toolName)
	if err != nil {
		return "", "", err
	}
	if !tool.IsAvailable() {
		return "", "", fmt.Errorf("AI tool %s not available", toolName)
	}

	result, err := ai.ExecuteWithOptionalStream(ctx, tool, ai.ExecuteRequest{
		Workdir:        workdir,
		Prompt:         prompt,
		Model:          model,
		ConversationID: conversationID,
	}, onChunk)
	if result == nil {
		return "", "", err
	}

	output := strings.TrimSpace(result.Output)
	nextConversationID := strings.TrimSpace(result.ConversationID)

	// When the tool returns an error, preserve any output so the caller can
	// persist logs for debugging.
	if err != nil {
		return output, nextConversationID, err
	}
	if !result.Success {
		return output, nextConversationID, fmt.Errorf("AI execution failed: %s", output)
	}
	return output, nextConversationID, nil
}

func (r *Runner) runShell(ctx context.Context, workdir, cmdline string) error {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return nil
	}

	output, err := proc.Run(ctx, workdir, "sh", "-c", cmdline)
	if err != nil {
		return withOutput(err, output)
	}
	return nil
}

func (r *Runner) generateCommitMessage(ctx context.Context, toolName, workdir, prompt string) (string, error) {
	summary, err := stagedDiffSummary(ctx, workdir)
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "fog-commit-msg-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	commitPrompt := strings.TrimSpace(fmt.Sprintf(
		"Generate a git commit message for the staged changes.\n"+
			"Rules:\n"+
			"- Use Conventional Commits style.\n"+
			"- Return plain text only.\n"+
			"- First line <= 72 chars.\n"+
			"- Optional body allowed.\n"+
			"- Do not include code fences.\n\n"+
			"Task prompt:\n%s\n\n"+
			"Staged changes summary:\n%s\n",
		strings.TrimSpace(prompt),
		summary,
	))
	raw, err := r.runTool(ctx, toolName, tempDir, commitPrompt)
	if err != nil {
		return "", err
	}

	msg := normalizeCommitMessage(raw)
	if msg == "" {
		return "", fmt.Errorf("empty commit message generated")
	}
	return msg, nil
}

func stagedDiffSummary(ctx context.Context, workdir string) (string, error) {
	diff, err := git.New(workdir).WithContext(ctx).StagedChanges()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(
		"Name status:\n" + diff.NameStatus +
			"\n\nStat:\n" + diff.Stat +
			"\n\nPatch (truncated):\n" + truncate(diff.Patch, 12000),
	), nil
}

func normalizeCommitMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if after, ok := strings.CutPrefix(msg, "```"); ok {
		msg = after
		msg = strings.TrimSpace(msg)
		if idx := strings.LastIndex(msg, "```"); idx >= 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		msg = strings.TrimPrefix(msg, "git")
		msg = strings.TrimPrefix(msg, "commit")
		msg = strings.TrimSpace(msg)
	}
	return truncate(msg, 5000)
}

func (r *Runner) generateForkSummary(sourceSession state.Session, forkPrompt, toolName string) (string, error) {
	if r.runs == nil {
		return "", errors.New("state store not configured")
	}
	runs, err := r.runs.ListRuns(sourceSession.ID)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", nil
	}
	latest := runs[0]
	events, err := r.runs.ListRunEvents(latest.ID, 200)
	if err != nil {
		return "", err
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Source session:\n")
	contextBuilder.WriteString("- Repo: " + sourceSession.RepoName + "\n")
	contextBuilder.WriteString("- Branch: " + sourceSession.Branch + "\n")
	contextBuilder.WriteString("- Tool: " + sourceSession.Tool + "\n")
	contextBuilder.WriteString("- Latest run state: " + latest.State + "\n")
	contextBuilder.WriteString("- Latest run prompt: " + truncate(latest.Prompt, 500) + "\n")
	if strings.TrimSpace(latest.CommitSHA) != "" {
		contextBuilder.WriteString("- Latest commit SHA: " + latest.CommitSHA + "\n")
	}
	if strings.TrimSpace(latest.CommitMsg) != "" {
		contextBuilder.WriteString("- Latest commit message: " + truncate(latest.CommitMsg, 300) + "\n")
	}
	contextBuilder.WriteString("\nRecent run events:\n")
	for _, event := range events {
		line := event.Type + ": " + truncate(strings.TrimSpace(event.Message+" "+event.Data), 300)
		contextBuilder.WriteString("- " + strings.TrimSpace(line) + "\n")
	}

	summaryPrompt := strings.TrimSpace(fmt.Sprintf(
		"You are preparing context for a forked coding session.\n"+
			"Summarize the source session in concise bullet points.\n"+
			"Requirements:\n"+
			"- Focus on implemented behavior, pending work, and risks.\n"+
			"- Keep under 250 words.\n"+
			"- Plain text only.\n\n"+
			"Upcoming fork request:\n%s\n\n"+
			"Source data:\n%s",
		strings.TrimSpace(forkPrompt),
		contextBuilder.String(),
	))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "fog-fork-summary-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	summary, err := r.runTool(ctx, toolName, tempDir, summaryPrompt)
	if err != nil {
		return "", err
	}
	return truncate(summary, 4000), nil
}
