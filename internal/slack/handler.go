package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/darkLord19/wtx/internal/runner"
	"github.com/darkLord19/wtx/internal/task"
	"github.com/google/uuid"
)

// Handler handles Slack slash commands and interactions
type Handler struct {
	runner        *runner.Runner
	signingSecret string
}

// New creates a new Slack handler
func New(runner *runner.Runner, signingSecret string) *Handler {
	return &Handler{
		runner:        runner,
		signingSecret: signingSecret,
	}
}

// SlackCommand represents a Slack slash command
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

// HandleCommand handles Slack slash commands
func (h *Handler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
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

	// Parse command text
	t, err := h.parseCommand(cmd.Text)
	if err != nil {
		h.sendErrorResponse(w, err.Error())
		return
	}

	// Set Slack context
	t.Options.SlackChannel = cmd.ChannelID
	t.Options.Async = true // Slack commands are always async

	// Send immediate acknowledgment
	h.sendAckResponse(w, t)

	// Execute task asynchronously
	go func() {
		if err := h.runner.Execute(t); err != nil {
			h.sendCompletionNotification(cmd.ResponseURL, t, err)
			return
		}
		h.sendCompletionNotification(cmd.ResponseURL, t, nil)
	}()
}

// parseCommand parses Slack command text into a task
// Example: "create branch feature-otp and add otp login using redis"
func (h *Handler) parseCommand(text string) (*task.Task, error) {
	text = strings.TrimSpace(text)

	// Match pattern: "create branch <n> and <prompt>"
	re := regexp.MustCompile(`(?i)create\s+branch\s+(\S+)\s+and\s+(.+)`)
	matches := re.FindStringSubmatch(text)

	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid command format. Use: create branch <n> and <prompt>")
	}

	branch := matches[1]
	prompt := matches[2]

	t := &task.Task{
		ID:        uuid.New().String(),
		State:     task.StateCreated,
		Branch:    branch,
		Prompt:    prompt,
		AITool:    "claude", // Default
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Options: task.Options{
			Commit:     true,  // Auto-commit for Slack
			CreatePR:   false, // User can create PR via buttons
			Validate:   false,
			BaseBranch: "main",
		},
	}

	return t, nil
}

// sendAckResponse sends immediate acknowledgment
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
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends an error response
func (h *Handler) sendErrorResponse(w http.ResponseWriter, errorMsg string) {
	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "❌ Error: " + errorMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendCompletionNotification sends completion notification to Slack
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
		// Success message
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

		// Add PR URL if available
		if prURL, ok := t.Metadata["pr_url"].(string); ok {
			attachment["fields"] = append(attachment["fields"].([]map[string]interface{}), map[string]interface{}{
				"title": "Pull Request",
				"value": prURL,
				"short": false,
			})
		}

		// Add action buttons
		actions := []map[string]interface{}{
			{
				"type":  "button",
				"text":  "Open Branch",
				"url":   fmt.Sprintf("vscode://file/%s", t.WorktreePath),
				"style": "primary",
			},
		}

		if _, ok := t.Metadata["pr_url"]; !ok {
			// Add Create PR button if PR not created yet
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

	// Send to response URL
	payload, _ := json.Marshal(message)
	http.Post(responseURL, "application/json", strings.NewReader(string(payload)))
}
