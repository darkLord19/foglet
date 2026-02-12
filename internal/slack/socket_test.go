package slack

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/darkLord19/foglet/internal/task"
)

func TestStripMentions(t *testing.T) {
	got := stripMentions("<@U123> [repo='acme-api'] add auth")
	if got != "[repo='acme-api'] add auth" {
		t.Fatalf("unexpected stripped text: %q", got)
	}
}

func TestNormalizeFollowUpPrompt(t *testing.T) {
	if _, err := normalizeFollowUpPrompt("   "); err == nil {
		t.Fatalf("expected empty prompt error")
	}
	if _, err := normalizeFollowUpPrompt("[repo='x'] do thing"); err == nil {
		t.Fatalf("expected options rejection for follow-up prompt")
	}
	got, err := normalizeFollowUpPrompt("fix error handling")
	if err != nil {
		t.Fatalf("expected valid follow-up prompt, got error: %v", err)
	}
	if got != "fix error handling" {
		t.Fatalf("unexpected prompt normalization: %q", got)
	}
}

func TestFindLatestThreadContextFromTasks(t *testing.T) {
	now := time.Now()
	tasks := []*task.Task{
		{
			ID:           "latest",
			AITool:       "aider",
			Branch:       "fog/latest",
			WorktreePath: "/tmp/worktree-latest",
			CreatedAt:    now.Add(-1 * time.Minute),
			Options: task.Options{
				SlackChannel: "C1",
			},
			Metadata: map[string]interface{}{
				"repo":          "acme-api",
				"slack_root_ts": "123.456",
			},
		},
		{
			ID:        "older",
			AITool:    "claude",
			Branch:    "fog/older",
			CreatedAt: now.Add(-2 * time.Minute),
			Options: task.Options{
				SlackChannel: "C1",
			},
			Metadata: map[string]interface{}{
				"repo":          "acme-api",
				"slack_root_ts": "123.456",
			},
		},
	}

	ctx, found := findLatestThreadContextFromTasks(tasks, "C1", "123.456")
	if !found {
		t.Fatalf("expected thread context to be found")
	}
	if ctx.TaskID != "latest" {
		t.Fatalf("expected first matching task in descending order, got: %s", ctx.TaskID)
	}
	if ctx.Repo != "acme-api" || ctx.Tool != "aider" || ctx.Branch != "fog/latest" {
		t.Fatalf("unexpected context: %+v", ctx)
	}
}

func TestCompletionText(t *testing.T) {
	start := time.Now().Add(-4 * time.Second)
	end := time.Now()
	tsk := &task.Task{
		Branch:      "fog/auth",
		CreatedAt:   start,
		CompletedAt: &end,
		Metadata: map[string]interface{}{
			"pr_url": "https://github.com/acme/repo/pull/1",
		},
	}
	ok := completionText(tsk, nil)
	if !strings.Contains(ok, "Task completed") || !strings.Contains(ok, "PR: https://github.com/acme/repo/pull/1") {
		t.Fatalf("unexpected completion text: %s", ok)
	}

	fail := completionText(tsk, errors.New("boom"))
	if !strings.Contains(fail, "Task failed") {
		t.Fatalf("unexpected failure text: %s", fail)
	}
}
