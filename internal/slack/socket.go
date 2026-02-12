package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/gorilla/websocket"
)

const (
	defaultConnectionsOpenURL = "https://slack.com/api/apps.connections.open"
	defaultPostMessageURL     = "https://slack.com/api/chat.postMessage"
)

var mentionPattern = regexp.MustCompile(`<@[^>]+>`)

// SocketMode handles Slack Socket Mode events.
type SocketMode struct {
	handler            *Handler
	appToken           string
	botToken           string
	httpClient         *http.Client
	dialer             *websocket.Dialer
	connectionsOpenURL string
	postMessageURL     string
}

// NewSocketMode creates a new socket mode server.
func NewSocketMode(r *runner.Runner, stateStore *state.Store, appToken, botToken string) *SocketMode {
	return &SocketMode{
		handler:            New(r, stateStore, ""),
		appToken:           strings.TrimSpace(appToken),
		botToken:           strings.TrimSpace(botToken),
		httpClient:         &http.Client{Timeout: 20 * time.Second},
		dialer:             websocket.DefaultDialer,
		connectionsOpenURL: defaultConnectionsOpenURL,
		postMessageURL:     defaultPostMessageURL,
	}
}

// Run starts Socket Mode and reconnects automatically on disconnect.
func (s *SocketMode) Run(ctx context.Context) error {
	if s.appToken == "" {
		return fmt.Errorf("slack app token is required")
	}
	if s.botToken == "" {
		return fmt.Errorf("slack bot token is required")
	}

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}

		err := s.runOnce(ctx)
		if err == nil {
			backoff = time.Second
		} else {
			log.Printf("slack socket mode disconnected: %v", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}

func (s *SocketMode) runOnce(ctx context.Context) error {
	wsURL, err := s.openSocketURL(ctx)
	if err != nil {
		return err
	}

	conn, _, err := s.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial slack socket: %w", err)
	}
	defer conn.Close()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var envelope socketEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}

		if envelope.EnvelopeID != "" {
			ack := map[string]string{"envelope_id": envelope.EnvelopeID}
			if err := conn.WriteJSON(ack); err != nil {
				return fmt.Errorf("ack slack envelope: %w", err)
			}
		}

		switch envelope.Type {
		case "slash_commands":
			go s.handleSlashEnvelope(envelope.Payload)
		case "events_api":
			go s.handleEventsEnvelope(envelope.Payload)
		case "disconnect":
			return fmt.Errorf("slack requested disconnect")
		}
	}
}

func (s *SocketMode) openSocketURL(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.connectionsOpenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.appToken)
	req.Header.Set("User-Agent", "fogd")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("open socket mode session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("open socket mode session failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		OK    bool   `json:"ok"`
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode socket mode open response: %w", err)
	}
	if !payload.OK || strings.TrimSpace(payload.URL) == "" {
		if payload.Error == "" {
			payload.Error = "missing websocket url"
		}
		return "", fmt.Errorf("open socket mode session failed: %s", payload.Error)
	}
	return payload.URL, nil
}

func (s *SocketMode) handleSlashEnvelope(raw json.RawMessage) {
	var payload socketSlashPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	parsed, err := parseCommandText(payload.Text)
	if err != nil {
		s.sendWebhookError(payload.ResponseURL, err.Error())
		return
	}

	t, repoPath, err := s.handler.buildTask(parsed)
	if err != nil {
		s.sendWebhookError(payload.ResponseURL, err.Error())
		return
	}

	t.Options.SlackChannel = payload.ChannelID
	t.Options.Async = true
	attachSlackMetadata(t, parsed.Repo, payload.ChannelID, "")

	s.sendWebhookAck(payload.ResponseURL, t)

	go func() {
		err := s.handler.runner.ExecuteInRepo(repoPath, t)
		s.handler.sendCompletionNotification(payload.ResponseURL, t, err)
	}()
}

func (s *SocketMode) handleEventsEnvelope(raw json.RawMessage) {
	var payload socketEventsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	evt := payload.Event
	if evt.Type != "app_mention" {
		return
	}
	if evt.BotID != "" || evt.Subtype != "" {
		return
	}

	prompt := stripMentions(evt.Text)
	if prompt == "" {
		return
	}

	rootTS := strings.TrimSpace(evt.ThreadTS)
	if rootTS == "" {
		rootTS = strings.TrimSpace(evt.TS)
	}
	if rootTS == "" || strings.TrimSpace(evt.Channel) == "" {
		return
	}

	isFollowUp := evt.ThreadTS != "" && evt.ThreadTS != evt.TS
	if isFollowUp {
		s.handleThreadFollowUp(evt.Channel, rootTS, prompt)
		return
	}

	parsed, err := parseCommandText(prompt)
	if err != nil {
		_, _ = s.postMessage(evt.Channel, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	t, repoPath, err := s.handler.buildTask(parsed)
	if err != nil {
		_, _ = s.postMessage(evt.Channel, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	t.Options.SlackChannel = evt.Channel
	t.Options.Async = true
	attachSlackMetadata(t, parsed.Repo, evt.Channel, rootTS)

	s.runTaskInThread(evt.Channel, rootTS, repoPath, t)
}

func (s *SocketMode) handleThreadFollowUp(channelID, rootTS, rawPrompt string) {
	prompt, err := normalizeFollowUpPrompt(rawPrompt)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	ctx, found, err := findLatestThreadContext(s.handler.runner, channelID, rootTS)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}
	if !found {
		_, _ = s.postMessage(channelID, rootTS, "❌ Could not find a prior Fog task in this thread. Start with `@fog [repo='name'] prompt`.")
		return
	}

	parsed := &parsedCommand{
		Repo:   ctx.Repo,
		Tool:   ctx.Tool,
		Prompt: prompt,
	}

	t, repoPath, err := s.handler.buildTask(parsed)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	// Follow-ups run from the previous task's worktree so the new branch starts
	// from the latest thread context, while preserving worktree isolation.
	execRepoPath := repoPath
	if strings.TrimSpace(ctx.WorktreePath) != "" {
		execRepoPath = strings.TrimSpace(ctx.WorktreePath)
	}

	t.Options.SlackChannel = channelID
	t.Options.Async = true
	attachSlackMetadata(t, ctx.Repo, channelID, rootTS)
	if t.Metadata == nil {
		t.Metadata = map[string]interface{}{}
	}
	t.Metadata["parent_task_id"] = ctx.TaskID
	t.Metadata["parent_branch"] = ctx.Branch

	s.runTaskInThread(channelID, rootTS, execRepoPath, t)
}

func (s *SocketMode) runTaskInThread(channelID, rootTS, repoPath string, t *task.Task) {
	start := fmt.Sprintf("🚀 Starting task on branch `%s`\n%s", t.Branch, t.Prompt)
	_, _ = s.postMessage(channelID, rootTS, start)

	go func() {
		err := s.handler.runner.ExecuteInRepo(repoPath, t)
		msg := completionText(t, err)
		_, _ = s.postMessage(channelID, rootTS, msg)
	}()
}

func (s *SocketMode) sendWebhookAck(responseURL string, t *task.Task) {
	response := map[string]interface{}{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("🚀 Starting task on branch `%s`", t.Branch),
	}
	_ = postWebhookJSON(s.httpClient, responseURL, response)
}

func (s *SocketMode) sendWebhookError(responseURL, errorMsg string) {
	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "❌ Error: " + errorMsg,
	}
	_ = postWebhookJSON(s.httpClient, responseURL, response)
}

func postWebhookJSON(client *http.Client, responseURL string, payload map[string]interface{}) error {
	if strings.TrimSpace(responseURL) == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := client.Post(responseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *SocketMode) postMessage(channelID, threadTS, text string) (string, error) {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(text) == "" {
		return "", nil
	}

	payload := map[string]string{
		"channel": channelID,
		"text":    text,
	}
	if strings.TrimSpace(threadTS) != "" {
		payload["thread_ts"] = threadTS
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, s.postMessageURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.botToken)
	req.Header.Set("User-Agent", "fogd")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("chat.postMessage failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("chat.postMessage failed: %s", result.Error)
	}
	return result.TS, nil
}

func stripMentions(input string) string {
	out := mentionPattern.ReplaceAllString(input, "")
	return strings.TrimSpace(out)
}

func normalizeFollowUpPrompt(input string) (string, error) {
	prompt := strings.TrimSpace(input)
	if prompt == "" {
		return "", fmt.Errorf("follow-up prompt is required")
	}
	if strings.HasPrefix(prompt, "[") {
		return "", fmt.Errorf("follow-up messages must be plain prompts; options are only allowed for the initial task")
	}
	return prompt, nil
}

func attachSlackMetadata(t *task.Task, repoName, channelID, rootTS string) {
	if t.Metadata == nil {
		t.Metadata = map[string]interface{}{}
	}
	t.Metadata["repo"] = repoName
	if strings.TrimSpace(channelID) != "" {
		t.Metadata["slack_channel_id"] = channelID
	}
	if strings.TrimSpace(rootTS) != "" {
		t.Metadata["slack_root_ts"] = rootTS
	}
}

func completionText(t *task.Task, err error) string {
	if err != nil {
		return fmt.Sprintf("❌ Task failed: `%s`\n%s", t.Branch, err.Error())
	}

	msg := fmt.Sprintf("✅ Task completed: `%s` (%s)", t.Branch, t.Duration().Round(time.Second))
	if prURL, ok := t.Metadata["pr_url"].(string); ok && strings.TrimSpace(prURL) != "" {
		msg += "\nPR: " + prURL
	}
	return msg
}

type threadContext struct {
	TaskID       string
	Repo         string
	Tool         string
	Branch       string
	WorktreePath string
}

func findLatestThreadContext(r *runner.Runner, channelID, rootTS string) (threadContext, bool, error) {
	tasks, err := r.ListTasks()
	if err != nil {
		return threadContext{}, false, err
	}
	ctx, found := findLatestThreadContextFromTasks(tasks, channelID, rootTS)
	return ctx, found, nil
}

func findLatestThreadContextFromTasks(tasks []*task.Task, channelID, rootTS string) (threadContext, bool) {
	for _, t := range tasks {
		if strings.TrimSpace(t.Options.SlackChannel) != strings.TrimSpace(channelID) {
			continue
		}
		if strings.TrimSpace(metadataValue(t, "slack_root_ts")) != strings.TrimSpace(rootTS) {
			continue
		}
		repo := strings.TrimSpace(metadataValue(t, "repo"))
		if repo == "" {
			continue
		}
		return threadContext{
			TaskID:       t.ID,
			Repo:         repo,
			Tool:         strings.TrimSpace(t.AITool),
			Branch:       strings.TrimSpace(t.Branch),
			WorktreePath: strings.TrimSpace(t.WorktreePath),
		}, true
	}
	return threadContext{}, false
}

func metadataValue(t *task.Task, key string) string {
	if t == nil || t.Metadata == nil {
		return ""
	}
	raw, ok := t.Metadata[key]
	if !ok {
		return ""
	}
	out, ok := raw.(string)
	if !ok {
		return ""
	}
	return out
}

type socketEnvelope struct {
	EnvelopeID string          `json:"envelope_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
}

type socketSlashPayload struct {
	ChannelID   string `json:"channel_id"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
}

type socketEventsPayload struct {
	Event struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Channel  string `json:"channel"`
		TS       string `json:"ts"`
		ThreadTS string `json:"thread_ts"`
		BotID    string `json:"bot_id"`
		Subtype  string `json:"subtype"`
	} `json:"event"`
}
