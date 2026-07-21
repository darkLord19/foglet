package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/ghcli"
	"github.com/darkLord19/foglet/internal/git"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
)

// Server provides HTTP API for Fog
type Server struct {
	runner        *runner.Runner
	stateStore    *state.Store
	port          int
	skipToolCheck bool // for testing: bypass isToolAvailable
}

// New creates a new API server
func New(runner *runner.Runner, stateStore *state.Store, port int) *Server {
	return &Server{
		runner:     runner,
		stateStore: stateStore,
		port:       port,
	}
}

// RegisterRoutes registers API routes on the provided mux
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionDetail)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskDetail)
	mux.HandleFunc("/api/tracker", s.handleTracker)
	mux.HandleFunc("/api/tracker/sync", s.handleTrackerSync)
	mux.HandleFunc("/api/repos", s.handleRepos)
	mux.HandleFunc("/api/repos/branches", s.handleListBranches)
	mux.HandleFunc("/api/repos/discover", s.handleDiscoverRepos)
	mux.HandleFunc("/api/repos/import", s.handleImportRepos)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/gh/status", s.handleGhStatus)
	mux.HandleFunc("/api/cloud", s.handleCloud)
	mux.HandleFunc("/api/cloud/pair", s.handleCloudPair)
	mux.HandleFunc("/api/cloud/unpair", s.handleCloudUnpair)
	mux.HandleFunc("/health", s.handleHealth)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting Fog API server on %s\n", addr)

	return http.ListenAndServe(addr, WithCORS(mux))
}

// handleHealth returns server health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

type SettingsResponse struct {
	DefaultTool        string            `json:"default_tool,omitempty"`
	DefaultModel       string            `json:"default_model,omitempty"`
	DefaultModels      map[string]string `json:"default_models"`
	DefaultAutoPR      bool              `json:"default_autopr"`
	DefaultNotify      bool              `json:"default_notify"`
	KeepAwake          bool              `json:"keep_awake"`
	BranchPrefix       string            `json:"branch_prefix,omitempty"`
	GhInstalled        bool              `json:"gh_installed"`
	GhAuthenticated    bool              `json:"gh_authenticated"`
	OnboardingRequired bool              `json:"onboarding_required"`
	AvailableTools     []string          `json:"available_tools"`
}

type UpdateSettingsRequest struct {
	DefaultTool   *string           `json:"default_tool"`
	DefaultModel  *string           `json:"default_model"`
	DefaultModels map[string]string `json:"default_models"`
	DefaultAutoPR *bool             `json:"default_autopr"`
	DefaultNotify *bool             `json:"default_notify"`
	KeepAwake     *bool             `json:"keep_awake,omitempty"`
	BranchPrefix  *string           `json:"branch_prefix"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getSettings(w)
	case http.MethodPut:
		s.updateSettings(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getSettings(w http.ResponseWriter) {
	availTools := detectAvailableTools()
	resp := SettingsResponse{
		AvailableTools: availTools,
		DefaultModels:  make(map[string]string, len(availTools)),
	}

	if tool, found, err := s.stateStore.GetDefaultTool(); err == nil && found {
		resp.DefaultTool = tool
	}
	if model, found, err := s.stateStore.GetSetting("default_model"); err == nil && found {
		resp.DefaultModel = model
	}
	// Per-tool default models
	for _, toolName := range availTools {
		if model, found, err := s.stateStore.GetSetting("default_model_" + toolName); err == nil && found && model != "" {
			resp.DefaultModels[toolName] = model
		}
	}
	if autopr, found, err := s.stateStore.GetSetting("default_autopr"); err == nil && found {
		resp.DefaultAutoPR = autopr == "true"
	}
	if notify, found, err := s.stateStore.GetSetting("default_notify"); err == nil && found {
		resp.DefaultNotify = notify == "true"
	}
	if prefix, found, err := s.stateStore.GetSetting("branch_prefix"); err == nil && found {
		resp.BranchPrefix = prefix
	}

	resp.GhInstalled = ghcli.IsGhAvailable()
	if resp.GhInstalled {
		resp.GhAuthenticated = ghcli.IsGhAuthenticated()
	}

	if keepAwake, found, err := s.stateStore.GetSetting("keep_awake"); err == nil && found {
		resp.KeepAwake = keepAwake == "true"
	}

	resp.OnboardingRequired = !resp.GhAuthenticated || strings.TrimSpace(resp.DefaultTool) == ""

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.DefaultTool != nil {
		tool := strings.TrimSpace(*req.DefaultTool)
		if tool == "" {
			http.Error(w, "default_tool cannot be empty", http.StatusBadRequest)
			return
		}
		if !s.skipToolCheck && !isToolAvailable(tool) {
			http.Error(w, fmt.Sprintf("default_tool %q is not available", tool), http.StatusBadRequest)
			return
		}
		if err := s.stateStore.SetDefaultTool(tool); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.DefaultModel != nil {
		if err := s.stateStore.SetSetting("default_model", strings.TrimSpace(*req.DefaultModel)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	for toolName, model := range req.DefaultModels {
		if err := s.stateStore.SetSetting("default_model_"+strings.TrimSpace(toolName), strings.TrimSpace(model)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.DefaultAutoPR != nil {
		val := "false"
		if *req.DefaultAutoPR {
			val = "true"
		}
		if err := s.stateStore.SetSetting("default_autopr", val); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.DefaultNotify != nil {
		val := "false"
		if *req.DefaultNotify {
			val = "true"
		}
		if err := s.stateStore.SetSetting("default_notify", val); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.KeepAwake != nil {
		val := "false"
		if *req.KeepAwake {
			val = "true"
		}
		if err := s.stateStore.SetSetting("keep_awake", val); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if req.BranchPrefix != nil {
		prefix := strings.TrimSpace(*req.BranchPrefix)
		if prefix == "" {
			http.Error(w, "branch_prefix cannot be empty", http.StatusBadRequest)
			return
		}
		if err := s.stateStore.SetSetting("branch_prefix", prefix); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.getSettings(w)
}

func detectAvailableTools() []string {
	names := ai.AvailableToolNames()
	out := make([]string, 0, len(names))
	for _, name := range names {
		if isToolAvailable(name) {
			out = append(out, name)
		}
	}
	return out
}

func isToolAvailable(name string) bool {
	tool, err := ai.GetTool(name)
	if err != nil {
		return false
	}
	return tool.IsAvailable()
}

// writeJSON writes a JSON response with the given status code and payload.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a JSON error response with the given status code and message.
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// allowedCORSOrigin returns the origin if it matches the local desktop allowlist,
// or empty string if the origin is not permitted.
func allowedCORSOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	lower := strings.ToLower(origin)
	switch {
	case lower == "wails://wails",
		lower == "http://wails.localhost",
		strings.HasPrefix(lower, "http://wails.localhost:"),
		strings.HasPrefix(lower, "http://localhost:"),
		strings.HasPrefix(lower, "http://127.0.0.1:"),
		lower == "http://localhost",
		lower == "http://127.0.0.1":
		return origin
	default:
		return ""
	}
}

// WithCORS adds CORS headers restricted to local desktop/web clients.
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := allowedCORSOrigin(origin)
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

const defaultMaxBodyBytes int64 = 1 << 20 // 1 MB

// WithBodyLimit restricts request body size for non-GET requests.
func WithBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodOptions && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// handleListBranches lists branches for a repo
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "repo name required", http.StatusBadRequest)
		return
	}

	repo, found, err := s.stateStore.GetRepoByName(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, fmt.Sprintf("unknown repo: %s", name), http.StatusNotFound)
		return
	}

	g := git.New(repo.BaseWorktreePath)
	branches, err := g.ListBranches()
	if err != nil {
		http.Error(w, fmt.Sprintf("git branch failed: %v", err), http.StatusInternalServerError)
		return
	}

	type Branch struct {
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}

	out := make([]Branch, 0, len(branches))
	for _, b := range branches {
		out = append(out, Branch{
			Name:      b,
			IsDefault: b == repo.DefaultBranch,
		})
	}

	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGhStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]any{
		"installed":     ghcli.IsGhAvailable(),
		"authenticated": false,
		"os":            runtimeOS(),
	}

	if status["installed"].(bool) {
		status["authenticated"] = ghcli.IsGhAuthenticated()
	}

	s.writeJSON(w, http.StatusOK, status)
}

func runtimeOS() string {
	return runtime.GOOS
}
