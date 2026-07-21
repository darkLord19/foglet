package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
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

	opts, err := h.buildSessionOptions(parsed)
	if err != nil {
		h.sendErrorResponse(w, err.Error())
		return
	}

	session, run, err := h.runner.Launch(opts)
	if err != nil {
		h.sendErrorResponse(w, err.Error())
		return
	}

	// Store Slack metadata as a run event for thread context lookup
	if h.stateStore != nil {
		_ = h.stateStore.AppendRunEvent(state.RunEvent{
			RunID: run.ID,
			Type:  "slack_metadata",
			Data:  fmt.Sprintf(`{"channel_id":"%s","root_ts":"%s","repo":"%s"}`, cmd.ChannelID, "", opts.RepoName),
		})
	}

	h.sendAckResponse(w, session, run)

	go func() {
		// Poll for run completion
		for {
			time.Sleep(2 * time.Second)
			if h.stateStore == nil {
				return
			}
			currentRun, found, err := h.stateStore.GetRun(run.ID)
			if err != nil || !found {
				return
			}
			if isTerminalRunState(currentRun.State) {
				session, _, _ := h.stateStore.GetSession(session.ID)
				h.sendCompletionNotification(cmd.ResponseURL, &session, &currentRun)
				return
			}
		}
	}()
}

func isTerminalRunState(state string) bool {
	switch strings.TrimSpace(state) {
	case "COMPLETED", "FAILED", "CANCELLED":
		return true
	default:
		return false
	}
}

func (h *Handler) buildSessionOptions(parsed *parsedCommand) (runner.LaunchRequest, error) {
	if h.stateStore == nil {
		return runner.LaunchRequest{}, fmt.Errorf("state store is not configured")
	}
	return runner.LaunchRequest{
		Entrypoint: "slack",
		RepoName:   parsed.Repo,
		Prompt:     parsed.Prompt,
		Tool:       parsed.Tool,
		Model:      parsed.Model,
		BranchName: parsed.BranchName,
		AutoPR:     parsed.AutoPR,
		CommitMsg:  parsed.CommitMsg,
		// A Slack command is issued by whoever is in the channel, not by the
		// person at this machine, so it may not target an integration branch.
		RejectProtectedBranch: true,
		Async:                 true,
	}, nil
}

// sendAckResponse sends immediate acknowledgment.
func (h *Handler) sendAckResponse(w http.ResponseWriter, session state.Session, run state.Run) {
	response := map[string]any{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("🚀 Starting session on branch `%s`", session.Branch),
		"attachments": []map[string]any{
			{
				"text":  run.Prompt,
				"color": "good",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends an error response.
func (h *Handler) sendErrorResponse(w http.ResponseWriter, errorMsg string) {
	response := map[string]any{
		"response_type": "ephemeral",
		"text":          "❌ Error: " + errorMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// sendCompletionNotification sends completion notification to Slack.
func (h *Handler) sendCompletionNotification(responseURL string, session *state.Session, run *state.Run) {
	switch {
	case run.State == "FAILED" || run.State == "CANCELLED":
		text := fmt.Sprintf("❌ Session %s: `%s`", run.State, session.Branch)
		if run.Error != "" {
			text += "\n" + run.Error
		}
		message := map[string]any{
			"response_type": "in_channel",
			"text":          text,
		}
		payload, _ := json.Marshal(message)
		_, _ = http.Post(responseURL, "application/json", strings.NewReader(string(payload)))

	default:
		duration := ""
		if run.CompletedAt != nil {
			duration = run.CompletedAt.Sub(run.CreatedAt).Round(time.Second).String()
		}

		fields := []map[string]any{
			{"title": "Branch", "value": session.Branch, "short": true},
		}
		if duration != "" {
			fields = append(fields, map[string]any{"title": "Duration", "value": duration, "short": true})
		}
		if session.PRURL != "" {
			fields = append(fields, map[string]any{"title": "Pull Request", "value": session.PRURL, "short": false})
		}

		attachment := map[string]any{
			"color":  "good",
			"fields": fields,
		}

		if session.WorktreePath != "" {
			attachment["actions"] = []map[string]any{
				{
					"type":  "button",
					"text":  "Open Branch",
					"url":   fmt.Sprintf("vscode://file/%s", session.WorktreePath),
					"style": "primary",
				},
			}
		}

		message := map[string]any{
			"response_type": "in_channel",
			"text":          fmt.Sprintf("✅ Session completed: `%s`", session.Branch),
			"attachments":   []map[string]any{attachment},
		}

		payload, _ := json.Marshal(message)
		_, _ = http.Post(responseURL, "application/json", strings.NewReader(string(payload)))
	}
}
