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
}

// Server provides multi-tenant Slack install/event handling.
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
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
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
	_, _ = w.Write([]byte("Fog Slack app installed successfully. Return to Fog desktop app to finish pairing."))
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
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

	_, paired, err := s.store.GetPairing(teamID, event.User)
	if err != nil {
		return err
	}
	if !paired {
		return s.postEphemeral(teamID, event.Channel, event.User, "Fog is not paired for this workspace. Pair your device from Fog desktop app.")
	}

	return s.postEphemeral(teamID, event.Channel, event.User, "Pairing found. Device routing will start in the next slice.")
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
	Type    string `json:"type"`
	User    string `json:"user"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
	TS      string `json:"ts"`
}
