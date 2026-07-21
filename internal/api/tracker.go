package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/task"
	"github.com/darkLord19/foglet/internal/tracker"
)

// Settings keys for tracker configuration. Credentials go in the encrypted
// secret store instead; only non-sensitive config lives here.
const (
	trackerProviderKey  = "tracker.provider"
	trackerStatusMapKey = "tracker.status_map"
	trackerLinearTeam   = "tracker.linear.team"
	trackerJiraURL      = "tracker.jira.url"
	trackerJiraEmail    = "tracker.jira.email"
	trackerJiraJQL      = "tracker.jira.jql"

	trackerLinearTokenSecret = "tracker.linear.token"
	trackerJiraTokenSecret   = "tracker.jira.token"
)

// TrackerConfig is the tracker setup as the UI sees it.
//
// Tokens are never returned — only whether one is stored. Echoing a secret back
// to any client, even a local one, is how secrets end up in logs and screen
// recordings.
type TrackerConfig struct {
	Provider   string            `json:"provider"`
	HasToken   bool              `json:"has_token"`
	StatusMap  tracker.StatusMap `json:"status_map"`
	LinearTeam string            `json:"linear_team,omitempty"`
	JiraURL    string            `json:"jira_url,omitempty"`
	JiraEmail  string            `json:"jira_email,omitempty"`
	JiraJQL    string            `json:"jira_jql,omitempty"`
}

// UpdateTrackerRequest configures the tracker. A blank token leaves the stored
// one untouched, so the UI can save other fields without re-entering it.
type UpdateTrackerRequest struct {
	Provider   string             `json:"provider"`
	Token      string             `json:"token,omitempty"`
	StatusMap  *tracker.StatusMap `json:"status_map,omitempty"`
	LinearTeam string             `json:"linear_team,omitempty"`
	JiraURL    string             `json:"jira_url,omitempty"`
	JiraEmail  string             `json:"jira_email,omitempty"`
	JiraJQL    string             `json:"jira_jql,omitempty"`
}

func (s *Server) handleTracker(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getTracker(w)
	case http.MethodPut:
		s.updateTracker(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTrackerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	res, err := s.SyncTracker(r.Context())
	if err != nil {
		if errors.Is(err, tracker.ErrNotConfigured) {
			writeErr(w, http.StatusBadRequest, "no tracker configured")
			return
		}
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) getTracker(w http.ResponseWriter) {
	cfg, err := s.trackerConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) updateTracker(w http.ResponseWriter, r *http.Request) {
	var req UpdateTrackerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	provider := task.Provider(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = task.ProviderLocal
	}
	if !provider.Valid() {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown provider %q", req.Provider))
		return
	}

	if req.StatusMap != nil {
		if err := req.StatusMap.Validate(); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		encoded, err := json.Marshal(req.StatusMap)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.stateStore.SetSetting(trackerStatusMapKey, string(encoded)); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	settings := map[string]string{
		trackerProviderKey: string(provider),
		trackerLinearTeam:  strings.TrimSpace(req.LinearTeam),
		trackerJiraURL:     strings.TrimRight(strings.TrimSpace(req.JiraURL), "/"),
		trackerJiraEmail:   strings.TrimSpace(req.JiraEmail),
		trackerJiraJQL:     strings.TrimSpace(req.JiraJQL),
	}
	for key, value := range settings {
		if err := s.stateStore.SetSetting(key, value); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if token := strings.TrimSpace(req.Token); token != "" {
		secretKey := trackerLinearTokenSecret
		if provider == task.ProviderJira {
			secretKey = trackerJiraTokenSecret
		}
		if err := s.stateStore.SaveSecret(secretKey, token); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	s.getTracker(w)
}

func (s *Server) trackerConfig() (TrackerConfig, error) {
	get := func(key string) string {
		v, _, err := s.stateStore.GetSetting(key)
		if err != nil {
			return ""
		}
		return v
	}

	cfg := TrackerConfig{
		Provider:   get(trackerProviderKey),
		LinearTeam: get(trackerLinearTeam),
		JiraURL:    get(trackerJiraURL),
		JiraEmail:  get(trackerJiraEmail),
		JiraJQL:    get(trackerJiraJQL),
		StatusMap:  tracker.DefaultStatusMap(),
	}
	if cfg.Provider == "" {
		cfg.Provider = string(task.ProviderLocal)
	}

	if raw := get(trackerStatusMapKey); raw != "" {
		var m tracker.StatusMap
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			cfg.StatusMap = m
		}
	}

	secretKey := trackerLinearTokenSecret
	if cfg.Provider == string(task.ProviderJira) {
		secretKey = trackerJiraTokenSecret
	}
	has, err := s.stateStore.HasSecret(secretKey)
	if err != nil {
		return cfg, err
	}
	cfg.HasToken = has

	return cfg, nil
}

// buildProvider constructs the configured tracker provider, or returns
// tracker.ErrNotConfigured when there is nothing to sync with.
func (s *Server) buildProvider() (tracker.Provider, tracker.StatusMap, error) {
	cfg, err := s.trackerConfig()
	if err != nil {
		return nil, tracker.StatusMap{}, err
	}

	switch task.Provider(cfg.Provider) {
	case task.ProviderLinear:
		token, found, err := s.stateStore.GetSecret(trackerLinearTokenSecret)
		if err != nil {
			return nil, cfg.StatusMap, err
		}
		if !found {
			return nil, cfg.StatusMap, tracker.ErrNotConfigured
		}
		p, err := tracker.NewLinear(token, cfg.LinearTeam, cfg.StatusMap)
		return p, cfg.StatusMap, err

	case task.ProviderJira:
		token, found, err := s.stateStore.GetSecret(trackerJiraTokenSecret)
		if err != nil {
			return nil, cfg.StatusMap, err
		}
		if !found {
			return nil, cfg.StatusMap, tracker.ErrNotConfigured
		}
		p, err := tracker.NewJira(cfg.JiraURL, cfg.JiraEmail, token, cfg.JiraJQL, cfg.StatusMap)
		return p, cfg.StatusMap, err

	default:
		return nil, cfg.StatusMap, tracker.ErrNotConfigured
	}
}

// SyncTracker runs one reconciliation pass. Exported so the daemon's periodic
// worker can call it on the same path the manual button uses.
func (s *Server) SyncTracker(ctx context.Context) (tracker.Result, error) {
	provider, statuses, err := s.buildProvider()
	if err != nil {
		return tracker.Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return tracker.NewSyncer(provider, s.stateStore, statuses).Sync(ctx)
}
