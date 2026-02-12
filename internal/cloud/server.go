package cloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultOAuthAccessURL = "https://slack.com/api/oauth.v2.access"
	defaultAPIBaseURL     = "https://slack.com/api"
	defaultStateTTL       = 10 * time.Minute
)

// Config configures fog cloud server behavior.
type Config struct {
	ClientID      string
	ClientSecret  string
	SigningSecret string
	PublicURL     string
	Scopes        []string
	HTTPClient    *http.Client

	OAuthAccessURL string
	APIBaseURL     string
	StateTTL       time.Duration
	PairingCodeTTL time.Duration
}

// Server provides multi-tenant Slack install/event handling and device routing APIs.
type Server struct {
	store      *Store
	cfg        Config
	httpClient *http.Client

	stateMu     sync.Mutex
	oauthStates map[string]time.Time
}

// NewServer creates a new fog cloud server.
func NewServer(store *Store, cfg Config) (*Server, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}

	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.SigningSecret = strings.TrimSpace(cfg.SigningSecret)
	cfg.PublicURL = strings.TrimSpace(cfg.PublicURL)
	if cfg.PublicURL == "" {
		return nil, errors.New("public_url is required")
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.SigningSecret == "" {
		return nil, errors.New("slack client id, client secret, and signing secret are required")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"app_mentions:read", "chat:write"}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if strings.TrimSpace(cfg.OAuthAccessURL) == "" {
		cfg.OAuthAccessURL = defaultOAuthAccessURL
	}
	if strings.TrimSpace(cfg.APIBaseURL) == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.StateTTL <= 0 {
		cfg.StateTTL = defaultStateTTL
	}
	if cfg.PairingCodeTTL <= 0 {
		cfg.PairingCodeTTL = 10 * time.Minute
	}

	return &Server{
		store:       store,
		cfg:         cfg,
		httpClient:  cfg.HTTPClient,
		oauthStates: make(map[string]time.Time),
	}, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/slack/install", s.handleInstall)
	mux.HandleFunc("/slack/oauth/callback", s.handleOAuthCallback)
	mux.HandleFunc("/slack/events", s.handleEvents)
	mux.HandleFunc("/v1/pair/claim", s.handlePairClaim)
	mux.HandleFunc("/v1/pair/unpair", s.handlePairUnpair)
	mux.HandleFunc("/v1/device/jobs/claim", s.handleDeviceClaimJob)
	mux.HandleFunc("/v1/device/jobs/", s.handleDeviceJobDetail)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stateToken, err := randomToken(16)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.setOAuthState(stateToken)

	redirectURI := strings.TrimRight(s.cfg.PublicURL, "/") + "/slack/oauth/callback"
	q := url.Values{}
	q.Set("client_id", s.cfg.ClientID)
	q.Set("scope", strings.Join(s.cfg.Scopes, ","))
	q.Set("redirect_uri", redirectURI)
	q.Set("state", stateToken)

	target := "https://slack.com/oauth/v2/authorize?" + q.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if stateToken == "" || code == "" {
		http.Error(w, "state and code are required", http.StatusBadRequest)
		return
	}
	if !s.consumeOAuthState(stateToken) {
		http.Error(w, "invalid or expired oauth state", http.StatusBadRequest)
		return
	}

	resp, err := s.exchangeOAuthCode(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.SaveInstallation(resp.Team.ID, resp.BotUserID, resp.AccessToken); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Fog Slack app installed successfully. Return to Fog device and finish pairing."))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if err := verifySlackSignature(s.cfg.SigningSecret, r.Header, body, time.Now().UTC()); err != nil {
		http.Error(w, "invalid slack signature", http.StatusUnauthorized)
		return
	}

	var envelope slackEventsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	switch envelope.Type {
	case "url_verification":
		writeJSON(w, http.StatusOK, map[string]string{
			"challenge": envelope.Challenge,
		})
		return
	case "event_callback":
		if strings.TrimSpace(envelope.TeamID) == "" || strings.TrimSpace(envelope.EventID) == "" {
			http.Error(w, "team_id and event_id are required", http.StatusBadRequest)
			return
		}
		isNew, err := s.store.RecordEventID(envelope.TeamID, envelope.EventID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !isNew {
			w.WriteHeader(http.StatusOK)
			return
		}
		if envelope.Event.Type == "app_mention" {
			_ = s.handleAppMention(envelope.TeamID, envelope.Event)
		}
		w.WriteHeader(http.StatusOK)
		return
	default:
		w.WriteHeader(http.StatusOK)
		return
	}
}

func (s *Server) handleAppMention(teamID string, event slackInnerEvent) error {
	if strings.TrimSpace(event.Channel) == "" || strings.TrimSpace(event.User) == "" {
		return nil
	}
	if strings.TrimSpace(event.Subtype) != "" || strings.TrimSpace(event.BotID) != "" {
		return nil
	}

	rootTS := strings.TrimSpace(event.ThreadTS)
	if rootTS == "" {
		rootTS = strings.TrimSpace(event.TS)
	}
	if rootTS == "" {
		return nil
	}

	deviceID, paired, err := s.store.GetPairing(teamID, event.User)
	if err != nil {
		return err
	}
	if !paired {
		req, reqErr := s.store.CreatePairingRequest(teamID, event.User, event.Channel, rootTS, s.cfg.PairingCodeTTL)
		if reqErr != nil {
			return reqErr
		}
		text := fmt.Sprintf(
			"Fog is not paired yet.\nOpen your Fog device and pair with code `%s`.\nCode expires in %d minutes.",
			req.Code,
			int(s.cfg.PairingCodeTTL.Minutes()),
		)
		return s.postEphemeral(teamID, event.Channel, event.User, text)
	}

	rawPrompt := stripMentions(event.Text)
	isFollowUp := strings.TrimSpace(event.ThreadTS) != "" && strings.TrimSpace(event.ThreadTS) != strings.TrimSpace(event.TS)
	job := Job{
		DeviceID:    deviceID,
		TeamID:      teamID,
		ChannelID:   event.Channel,
		RootTS:      rootTS,
		SlackUserID: event.User,
	}

	if isFollowUp {
		prompt, parseErr := normalizeFollowUpPrompt(rawPrompt)
		if parseErr != nil {
			_ = s.postMessage(teamID, event.Channel, rootTS, "❌ "+parseErr.Error())
			return nil
		}
		sessionID, found, getErr := s.store.GetThreadSession(teamID, event.Channel, rootTS)
		if getErr != nil {
			return getErr
		}
		if !found {
			_ = s.postMessage(teamID, event.Channel, rootTS, "❌ No active Fog session found for this thread. Start with an initial command.")
			return nil
		}
		job.Kind = jobKindFollowUp
		job.SessionID = sessionID
		job.Prompt = prompt
	} else {
		parsed, parseErr := parseCommandText(rawPrompt)
		if parseErr != nil {
			_ = s.postMessage(teamID, event.Channel, rootTS, "❌ "+parseErr.Error())
			return nil
		}
		job.Kind = jobKindStartSession
		job.Repo = parsed.Repo
		job.Tool = parsed.Tool
		job.Model = parsed.Model
		job.AutoPR = parsed.AutoPR
		job.BranchName = parsed.BranchName
		job.CommitMsg = parsed.CommitMsg
		job.Prompt = parsed.Prompt
	}

	enqueued, err := s.store.EnqueueJob(job)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("🚀 Queued on your paired Fog device (job `%s`).", enqueued.ID)
	return s.postMessage(teamID, event.Channel, rootTS, text)
}

func (s *Server) handlePairClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code        string `json:"code"`
		DeviceID    string `json:"device_id"`
		DeviceToken string `json:"device_token,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	result, err := s.store.ClaimPairingRequest(req.Code, req.DeviceID, req.DeviceToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"team_id":       result.TeamID,
		"slack_user_id": result.SlackUserID,
		"device_id":     result.DeviceID,
		"device_token":  result.DeviceToken,
	})
}

func (s *Server) handlePairUnpair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID, token, err := deviceAuthFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := s.store.AuthenticateDevice(deviceID, token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		TeamID      string `json:"team_id"`
		SlackUserID string `json:"slack_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.store.UnpairStrict(req.TeamID, req.SlackUserID, deviceID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeviceClaimJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deviceID, token, err := deviceAuthFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := s.store.AuthenticateDevice(deviceID, token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	job, found, err := s.store.ClaimNextJob(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleDeviceJobDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/device/jobs/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "complete" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	jobID := strings.TrimSpace(parts[0])
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}

	deviceID, token, err := deviceAuthFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := s.store.AuthenticateDevice(deviceID, token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		Success   bool   `json:"success"`
		Error     string `json:"error,omitempty"`
		SessionID string `json:"session_id,omitempty"`
		RunID     string `json:"run_id,omitempty"`
		Branch    string `json:"branch,omitempty"`
		PRURL     string `json:"pr_url,omitempty"`
		CommitSHA string `json:"commit_sha,omitempty"`
		CommitMsg string `json:"commit_msg,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	job, err := s.store.CompleteJob(JobCompletion{
		JobID:     jobID,
		DeviceID:  deviceID,
		Success:   req.Success,
		Error:     req.Error,
		SessionID: req.SessionID,
		RunID:     req.RunID,
		Branch:    req.Branch,
		PRURL:     req.PRURL,
		CommitSHA: req.CommitSHA,
		CommitMsg: req.CommitMsg,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Success && strings.TrimSpace(job.Kind) == jobKindStartSession && strings.TrimSpace(job.SessionID) != "" {
		_ = s.store.UpsertThreadSession(job.TeamID, job.ChannelID, job.RootTS, job.SessionID)
	}

	if req.Success {
		text := fmt.Sprintf("✅ Completed on branch `%s`.", fallback(job.Branch, job.BranchName))
		if strings.TrimSpace(job.PRURL) != "" {
			text += "\nPR: " + job.PRURL
		}
		_ = s.postMessage(job.TeamID, job.ChannelID, job.RootTS, text)
	} else {
		errText := fallback(job.Error, req.Error)
		_ = s.postMessage(job.TeamID, job.ChannelID, job.RootTS, "❌ Task failed: "+fallback(errText, "unknown error"))
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"job_id": job.ID,
	})
}

func deviceAuthFromRequest(r *http.Request) (string, string, error) {
	deviceID := strings.TrimSpace(r.Header.Get("X-Fog-Device-ID"))
	if deviceID == "" {
		return "", "", errors.New("missing X-Fog-Device-ID header")
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return "", "", errors.New("missing bearer token")
	}
	token := strings.TrimSpace(auth[len("Bearer "):])
	if token == "" {
		return "", "", errors.New("missing bearer token")
	}
	return deviceID, token, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func fallback(v, alt string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return strings.TrimSpace(alt)
	}
	return v
}

func (s *Server) postEphemeral(teamID, channelID, userID, text string) error {
	inst, found, err := s.store.GetInstallation(teamID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("installation not found for team %s", teamID)
	}

	body, err := json.Marshal(map[string]string{
		"channel": channelID,
		"user":    userID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.cfg.APIBaseURL, "/")+"/chat.postEphemeral", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+inst.BotToken)
	req.Header.Set("User-Agent", "fogcloud")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("chat.postEphemeral failed: %s", out.Error)
	}
	return nil
}

func (s *Server) postMessage(teamID, channelID, threadTS, text string) error {
	inst, found, err := s.store.GetInstallation(teamID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("installation not found for team %s", teamID)
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
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.cfg.APIBaseURL, "/")+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+inst.BotToken)
	req.Header.Set("User-Agent", "fogcloud")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("chat.postMessage failed: %s", out.Error)
	}
	return nil
}

type oauthAccessResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID string `json:"id"`
	} `json:"team"`
}

func (s *Server) exchangeOAuthCode(code string) (*oauthAccessResponse, error) {
	redirectURI := strings.TrimRight(s.cfg.PublicURL, "/") + "/slack/oauth/callback"
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.cfg.ClientID)
	form.Set("client_secret", s.cfg.ClientSecret)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, s.cfg.OAuthAccessURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "fogcloud")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out oauthAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("oauth exchange failed: %s", strings.TrimSpace(out.Error))
	}
	if strings.TrimSpace(out.Team.ID) == "" || strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("oauth exchange missing team/token")
	}
	return &out, nil
}

func verifySlackSignature(signingSecret string, headers http.Header, body []byte, now time.Time) error {
	signingSecret = strings.TrimSpace(signingSecret)
	if signingSecret == "" {
		return errors.New("signing secret is required")
	}
	ts := strings.TrimSpace(headers.Get("X-Slack-Request-Timestamp"))
	sig := strings.TrimSpace(headers.Get("X-Slack-Signature"))
	if ts == "" || sig == "" {
		return errors.New("missing slack signature headers")
	}

	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid slack timestamp: %w", err)
	}
	requestTime := time.Unix(tsInt, 0).UTC()
	if now.UTC().Sub(requestTime) > 5*time.Minute || requestTime.Sub(now.UTC()) > 5*time.Minute {
		return errors.New("slack timestamp out of range")
	}

	base := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return errors.New("signature mismatch")
	}
	return nil
}

func randomToken(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Server) setOAuthState(token string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.oauthStates[token] = time.Now().Add(s.cfg.StateTTL)
}

func (s *Server) consumeOAuthState(token string) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	expiresAt, ok := s.oauthStates[token]
	if !ok {
		return false
	}
	delete(s.oauthStates, token)
	return expiresAt.After(time.Now())
}

type slackEventsEnvelope struct {
	Type      string          `json:"type"`
	Challenge string          `json:"challenge"`
	TeamID    string          `json:"team_id"`
	EventID   string          `json:"event_id"`
	Event     slackInnerEvent `json:"event"`
}

type slackInnerEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
	BotID    string `json:"bot_id"`
	Subtype  string `json:"subtype"`
}
