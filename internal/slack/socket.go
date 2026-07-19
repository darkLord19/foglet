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

	opts, err := s.handler.buildSessionOptions(parsed)
	if err != nil {
		s.sendWebhookError(payload.ResponseURL, err.Error())
		return
	}

	session, run, err := s.handler.runner.StartSessionAsync(opts)
	if err != nil {
		s.sendWebhookError(payload.ResponseURL, err.Error())
		return
	}

	// Store Slack metadata as a run event for thread context lookup
	if s.handler.stateStore != nil {
		_ = s.handler.stateStore.AppendRunEvent(state.RunEvent{
			RunID: run.ID,
			Type:  "slack_metadata",
			Data:  fmt.Sprintf(`{"channel_id":"%s","root_ts":"%s","repo":"%s"}`, payload.ChannelID, "", opts.RepoName),
		})
	}

	s.sendWebhookAck(payload.ResponseURL, session, run)

	go func() {
		for {
			time.Sleep(2 * time.Second)
			if s.handler.stateStore == nil {
				return
			}
			currentRun, found, err := s.handler.stateStore.GetRun(run.ID)
			if err != nil || !found {
				return
			}
			if isTerminalRunState(currentRun.State) {
				currentSession, _, _ := s.handler.stateStore.GetSession(session.ID)
				s.handler.sendCompletionNotification(payload.ResponseURL, &currentSession, &currentRun)
				return
			}
		}
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

	opts, err := s.handler.buildSessionOptions(parsed)
	if err != nil {
		_, _ = s.postMessage(evt.Channel, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	session, run, err := s.handler.runner.StartSessionAsync(opts)
	if err != nil {
		_, _ = s.postMessage(evt.Channel, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	// Store Slack metadata for thread context lookup
	if s.handler.stateStore != nil {
		_ = s.handler.stateStore.AppendRunEvent(state.RunEvent{
			RunID: run.ID,
			Type:  "slack_metadata",
			Data:  fmt.Sprintf(`{"channel_id":"%s","root_ts":"%s","repo":"%s"}`, evt.Channel, rootTS, opts.RepoName),
		})
	}

	s.runSessionInThread(evt.Channel, rootTS, session, run)
}

func (s *SocketMode) handleThreadFollowUp(channelID, rootTS, rawPrompt string) {
	prompt, err := normalizeFollowUpPrompt(rawPrompt)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	sessionID, found, err := findLatestSessionForThread(s.handler.runner, s.handler.stateStore, channelID, rootTS)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}
	if !found {
		_, _ = s.postMessage(channelID, rootTS, "❌ Could not find a prior Fog session in this thread. Start with `@fog [repo='name'] prompt`.")
		return
	}

	// Continue the existing session with the follow-up prompt
	run, err := s.handler.runner.ContinueSessionAsync(sessionID, prompt)
	if err != nil {
		_, _ = s.postMessage(channelID, rootTS, fmt.Sprintf("❌ %s", err.Error()))
		return
	}

	start := fmt.Sprintf("🚀 Continuing session with: %s", prompt)
	_, _ = s.postMessage(channelID, rootTS, start)

	go func() {
		for {
			time.Sleep(2 * time.Second)
			if s.handler.stateStore == nil {
				return
			}
			currentRun, found, err := s.handler.stateStore.GetRun(run.ID)
			if err != nil || !found {
				return
			}
			if isTerminalRunState(currentRun.State) {
				session, _, _ := s.handler.stateStore.GetSession(sessionID)
				msg := completionTextFromSession(&session, &currentRun)
				_, _ = s.postMessage(channelID, rootTS, msg)
				return
			}
		}
	}()
}

func (s *SocketMode) runSessionInThread(channelID, rootTS string, session state.Session, run state.Run) {
	start := fmt.Sprintf("🚀 Starting session on branch `%s`\n%s", session.Branch, run.Prompt)
	_, _ = s.postMessage(channelID, rootTS, start)

	go func() {
		for {
			time.Sleep(2 * time.Second)
			if s.handler.stateStore == nil {
				return
			}
			currentRun, found, err := s.handler.stateStore.GetRun(run.ID)
			if err != nil || !found {
				return
			}
			if isTerminalRunState(currentRun.State) {
				currentSession, _, _ := s.handler.stateStore.GetSession(session.ID)
				msg := completionTextFromSession(&currentSession, &currentRun)
				_, _ = s.postMessage(channelID, rootTS, msg)
				return
			}
		}
	}()
}

func (s *SocketMode) sendWebhookAck(responseURL string, session state.Session, run state.Run) {
	response := map[string]any{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("🚀 Starting session on branch `%s`", session.Branch),
	}
	_ = postWebhookJSON(s.httpClient, responseURL, response)
}

func (s *SocketMode) sendWebhookError(responseURL, errorMsg string) {
	response := map[string]any{
		"response_type": "ephemeral",
		"text":          "❌ Error: " + errorMsg,
	}
	_ = postWebhookJSON(s.httpClient, responseURL, response)
}

func postWebhookJSON(client *http.Client, responseURL string, payload map[string]any) error {
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

func completionTextFromSession(session *state.Session, run *state.Run) string {
	if run.State == "FAILED" || run.State == "CANCELLED" {
		msg := fmt.Sprintf("❌ Session %s: `%s`", run.State, session.Branch)
		if run.Error != "" {
			msg += "\n" + run.Error
		}
		return msg
	}

	duration := ""
	if run.CompletedAt != nil {
		duration = run.CompletedAt.Sub(run.CreatedAt).Round(time.Second).String()
	}
	msg := fmt.Sprintf("✅ Session completed: `%s` (%s)", session.Branch, duration)
	if session.PRURL != "" {
		msg += "\nPR: " + session.PRURL
	}
	return msg
}

func findLatestSessionForThread(r *runner.Runner, store *state.Store, channelID, rootTS string) (string, bool, error) {
	if store == nil {
		return "", false, nil
	}

	sessions, err := r.ListSessions()
	if err != nil {
		return "", false, err
	}

	for _, session := range sessions {
		runs, err := store.ListRuns(session.ID)
		if err != nil {
			continue
		}
		for _, run := range runs {
			events, err := store.ListRunEvents(run.ID, 200)
			if err != nil {
				continue
			}
			for _, event := range events {
				if event.Type != "slack_metadata" {
					continue
				}
				var md struct {
					ChannelID string `json:"channel_id"`
					RootTS    string `json:"root_ts"`
				}
				if err := json.Unmarshal([]byte(event.Data), &md); err != nil {
					continue
				}
				if md.ChannelID == channelID && md.RootTS == rootTS {
					return session.ID, true, nil
				}
			}
			// Only check the first (latest) run — metadata is always on run 0
			break
		}
	}
	return "", false, nil
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
