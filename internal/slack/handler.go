package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/darkLord19/foglet/internal/toolcfg"
	"github.com/google/uuid"
)

// Handler handles Slack slash commands and interactions.
type Handler struct {
	stateStore    *state.Store
	runner        *runner.Runner
	signingSecret string
}

// New creates a new Slack handler.
func New(runner *runner.Runner, stateStore *state.Store, signingSecret string) *Handler {
	return &Handler{
		stateStore:    stateStore,
		runner:        runner,
		signingSecret: signingSecret,
	}
}

// SlackCommand represents a Slack slash command payload.
type SlackCommand struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	Command     string `json:"command"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
}

// HandleCommand handles Slack slash commands.
func (h *Handler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := SlackCommand{
		Token:       r.FormValue("token"),
		TeamID:      r.FormValue("team_id"),
		TeamDomain:  r.FormValue("team_domain"),
		ChannelID:   r.FormValue("channel_id"),
		ChannelName: r.FormValue("channel_name"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
		Command:     r.FormValue("command"),
		Text:        r.FormValue("text"),
		ResponseURL: r.FormValue("response_url"),
	}

	parsed, err := parseCommandText(cmd.Text)
	if err != nil {
		h.sendErrorResponse(w, err.Error())
		return
	}

	t, repoPath, err := h.buildTask(parsed)
	if err != nil {
		h.sendErrorResponse(w, err.Error())
		return
	}

	t.Options.SlackChannel = cmd.ChannelID
	t.Options.Async = true

	h.sendAckResponse(w, t)

	go func() {
		if err := h.runner.ExecuteInRepo(repoPath, t); err != nil {
			h.sendCompletionNotification(cmd.ResponseURL, t, err)
			return
		}
		h.sendCompletionNotification(cmd.ResponseURL, t, nil)
	}()
}

func (h *Handler) buildTask(parsed *parsedCommand) (*task.Task, string, error) {
	if h.stateStore == nil {
		return nil, "", fmt.Errorf("state store is not configured")
	}

	repo, found, err := h.stateStore.GetRepoByName(parsed.Repo)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", fmt.Errorf("unknown repo: %s", parsed.Repo)
	}
	if strings.TrimSpace(repo.BaseWorktreePath) == "" {
		return nil, "", fmt.Errorf("repo %s has no base worktree path", parsed.Repo)
	}

	tool, err := toolcfg.ResolveTool(parsed.Tool, h.stateStore, "slack")
	if err != nil {
		return nil, "", err
	}

	branch := strings.TrimSpace(parsed.BranchName)
	if branch == "" {
		branchPrefix := "fog"
		if configured, ok, err := h.stateStore.GetSetting("branch_prefix"); err == nil && ok && strings.TrimSpace(configured) != "" {
			branchPrefix = configured
		}
		branch = generateBranchName(branchPrefix, parsed.Prompt)
	}

	if isProtectedBranch(branch) {
		return nil, "", fmt.Errorf("protected branch %q is not allowed", branch)
	}
	if !isValidBranchName(repo.BaseWorktreePath, branch) {
		return nil, "", fmt.Errorf("invalid branch name: %s", branch)
	}

	baseBranch := strings.TrimSpace(repo.DefaultBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}

	t := &task.Task{
		ID:        uuid.New().String(),
		State:     task.StateCreated,
		Branch:    branch,
		Prompt:    parsed.Prompt,
		AITool:    tool,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Options: task.Options{
			Commit:     true,
			CreatePR:   parsed.AutoPR,
			Validate:   false,
			BaseBranch: baseBranch,
			CommitMsg:  parsed.CommitMsg,
		},
	}

	if parsed.Model != "" {
		t.Metadata = map[string]interface{}{"model": parsed.Model}
	}

	return t, repo.BaseWorktreePath, nil
}

// sendAckResponse sends immediate acknowledgment.
func (h *Handler) sendAckResponse(w http.ResponseWriter, t *task.Task) {
	response := map[string]interface{}{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("🚀 Starting task on branch `%s`", t.Branch),
		"attachments": []map[string]interface{}{
			{
				"text":  t.Prompt,
				"color": "good",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends an error response.
func (h *Handler) sendErrorResponse(w http.ResponseWriter, errorMsg string) {
	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "❌ Error: " + errorMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// sendCompletionNotification sends completion notification to Slack.
func (h *Handler) sendCompletionNotification(responseURL string, t *task.Task, err error) {
	var message map[string]interface{}

	if err != nil {
		message = map[string]interface{}{
			"response_type": "in_channel",
			"text":          fmt.Sprintf("❌ Task failed: %s", t.Branch),
			"attachments": []map[string]interface{}{
				{
					"text":  err.Error(),
					"color": "danger",
				},
			},
		}
	} else {
		duration := t.Duration()

		attachment := map[string]interface{}{
			"color": "good",
			"fields": []map[string]interface{}{
				{
					"title": "Branch",
					"value": t.Branch,
					"short": true,
				},
				{
					"title": "Duration",
					"value": duration.String(),
					"short": true,
				},
			},
		}

		if prURL, ok := t.Metadata["pr_url"].(string); ok {
			attachment["fields"] = append(attachment["fields"].([]map[string]interface{}), map[string]interface{}{
				"title": "Pull Request",
				"value": prURL,
				"short": false,
			})
		}

		actions := []map[string]interface{}{
			{
				"type":  "button",
				"text":  "Open Branch",
				"url":   fmt.Sprintf("vscode://file/%s", t.WorktreePath),
				"style": "primary",
			},
		}

		if _, ok := t.Metadata["pr_url"]; !ok {
			actions = append(actions, map[string]interface{}{
				"type":  "button",
				"text":  "Create PR",
				"name":  "create_pr",
				"value": t.ID,
				"style": "default",
			})
		}

		attachment["actions"] = actions

		message = map[string]interface{}{
			"response_type": "in_channel",
			"text":          fmt.Sprintf("✅ Task completed: %s", t.Branch),
			"attachments":   []map[string]interface{}{attachment},
		}
	}

	payload, _ := json.Marshal(message)
	_, _ = http.Post(responseURL, "application/json", strings.NewReader(string(payload)))
}

func isValidBranchName(repoPath, branch string) bool {
	cmd := exec.Command("git", "-C", repoPath, "check-ref-format", "--branch", branch)
	return cmd.Run() == nil
}
