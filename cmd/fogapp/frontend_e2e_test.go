package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestDesktopFrontendSmokeFlows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e smoke in short mode")
	}

	chromePath := findChromeBinary()
	if chromePath == "" {
		t.Skip("skipping desktop e2e smoke: no Chrome/Chromium binary found")
	}

	mockAPI := newMockFogAPI()
	apiServer := httptest.NewServer(mockAPI)
	defer apiServer.Close()

	frontendServer := newFrontendHarnessServer(t, apiServer.URL)
	defer frontendServer.Close()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1400,1000"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 55*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate(frontendServer.URL),
		chromedp.WaitVisible(".home-view", chromedp.ByQuery),
		waitTextContains(".status-pill", "connected"),
		waitTextContains(".session-history", "Initial prompt"),

		chromedp.Click(".session-card", chromedp.ByQuery),
		waitTextContains("#detail-title", "Initial prompt"),

		chromedp.SetValue("#followup-prompt", "Add regression tests", chromedp.ByQuery),
		chromedp.Click("#followup-submit", chromedp.ByQuery),
		waitToastContains("Queued run"),

		chromedp.Click("#detail-stop", chromedp.ByQuery),
		waitToastContains("Cancel requested"),

		chromedp.Click("#detail-open", chromedp.ByQuery),
		waitToastContains("Opened in"),

		chromedp.Click("#detail-rerun", chromedp.ByQuery),
		waitToastContains("Queued re-run"),

		chromedp.Click(".brand-btn", chromedp.ByQuery),

		// In the new UI, we might need to select repo first if it's empty
		// Mock API returns repos, ChatBox auto-selects if one.
		// We'll assume auto-selection or just type prompt.
		chromedp.SetValue("#chat-prompt", "Implement desktop smoke flow", chromedp.ByQuery),
		chromedp.Click("#chat-submit", chromedp.ByQuery),
		waitToastContains("Session started"),
		waitTextContains("#detail-title", "Implement desktop smoke flow"),

		chromedp.Click("#nav-settings", chromedp.ByQuery),
		chromedp.WaitVisible("#settings-save", chromedp.ByQuery),

		chromedp.Click("#settings-discover", chromedp.ByQuery),
		waitToastContains("Found"),

		chromedp.Click("#settings-import", chromedp.ByQuery),
		waitToastContains("Imported"),

		chromedp.SetValue("#settings-branch-prefix", "team", chromedp.ByQuery),
		chromedp.Click("#settings-save", chromedp.ByQuery),
		waitToastContains("Settings saved"),
	)
	if err != nil {
		t.Fatalf("desktop e2e flow failed: %v", err)
	}

	stats := mockAPI.stats()
	if stats.createSessionCount < 1 {
		t.Fatalf("expected create session call, got %+v", stats)
	}
	if stats.followupCount < 1 {
		t.Fatalf("expected follow-up call, got %+v", stats)
	}
	if stats.discoverCount < 1 || stats.importCount < 1 {
		t.Fatalf("expected repo discover/import calls, got %+v", stats)
	}
	if stats.settingsPutCount < 1 {
		t.Fatalf("expected settings update call, got %+v", stats)
	}
	if stats.cancelCount < 1 {
		t.Fatalf("expected cancel call, got %+v", stats)
	}
	if stats.openCount < 1 {
		t.Fatalf("expected open-in-editor call, got %+v", stats)
	}
}

type e2eStats struct {
	createSessionCount int
	followupCount      int
	forkCount          int
	discoverCount      int
	importCount        int
	settingsPutCount   int
	cancelCount        int
	openCount          int
}

type mockFogAPI struct {
	mu sync.Mutex

	counters e2eStats

	settings map[string]interface{}
	repos    []map[string]interface{}
	sessions []map[string]interface{}
	runs     map[string][]map[string]interface{}
	events   map[string][]map[string]interface{}
}

func newMockFogAPI() *mockFogAPI {
	now := time.Now().UTC()
	sessions := []map[string]interface{}{
		{
			"id":            "session-1",
			"repo_name":     "owner/repo",
			"branch":        "fog/session-one",
			"worktree_path": "/tmp/owner-repo/worktrees/fog-session-one-run-1",
			"tool":          "claude",
			"status":        "COMPLETED",
			"busy":          false,
			"autopr":        false,
			"pr_url":        "",
			"updated_at":    now.Format(time.RFC3339),
		},
	}
	runs := map[string][]map[string]interface{}{
		"session-1": {
			{
				"id":            "run-1",
				"session_id":    "session-1",
				"prompt":        "Initial prompt",
				"worktree_path": "/tmp/owner-repo/worktrees/fog-session-one-run-1",
				"state":         "COMPLETED",
				"created_at":    now.Add(-3 * time.Minute).Format(time.RFC3339),
				"updated_at":    now.Add(-2 * time.Minute).Format(time.RFC3339),
			},
		},
	}
	events := map[string][]map[string]interface{}{
		"run-1": {
			{"id": 1, "run_id": "run-1", "ts": now.Add(-3 * time.Minute).Format(time.RFC3339), "type": "ai_start", "message": "Running AI tool"},
			{"id": 2, "run_id": "run-1", "ts": now.Add(-2 * time.Minute).Format(time.RFC3339), "type": "complete", "message": "Run completed"},
		},
	}

	return &mockFogAPI{
		settings: map[string]interface{}{
			"default_tool":        "claude",
			"default_model":       "",
			"default_models":      map[string]string{},
			"default_autopr":      false,
			"default_notify":      false,
			"branch_prefix":       "fog",
			"gh_installed":        true,
			"gh_authenticated":    true,
			"onboarding_required": false,
			"available_tools":     []string{"claude", "cursor"},
		},
		repos: []map[string]interface{}{
			{
				"name":               "owner/repo",
				"url":                "https://github.com/owner/repo.git",
				"default_branch":     "main",
				"base_worktree_path": "/tmp/owner-repo/base",
			},
		},
		sessions: sessions,
		runs:     runs,
		events:   events,
	}
}

func (m *mockFogAPI) statsSnapshot() e2eStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters
}

func (m *mockFogAPI) stats() e2eStats {
	return m.statsSnapshot()
}

func firstRunID(runs []map[string]interface{}) string {
	if len(runs) == 0 {
		return ""
	}
	if id, ok := runs[0]["id"].(string); ok {
		return id
	}
	return ""
}

func (m *mockFogAPI) sessionSummariesLocked() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(m.sessions))
	for _, session := range m.sessions {
		copySession := map[string]interface{}{}
		for k, v := range session {
			copySession[k] = v
		}
		sid, _ := session["id"].(string)
		runs := m.runs[sid]
		if len(runs) > 0 {
			latest := map[string]interface{}{}
			for k, v := range runs[0] {
				latest[k] = v
			}
			copySession["latest_run"] = latest
		}
		out = append(out, copySession)
	}
	return out
}

func (m *mockFogAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	writeJSON := func(code int, payload interface{}) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(payload)
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/settings":
		writeJSON(http.StatusOK, m.settings)
		return
	case r.Method == http.MethodPut && r.URL.Path == "/api/settings":
		m.counters.settingsPutCount++
		var in map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if v, ok := in["default_tool"].(string); ok && strings.TrimSpace(v) != "" {
			m.settings["default_tool"] = v
		}
		if v, ok := in["branch_prefix"].(string); ok && strings.TrimSpace(v) != "" {
			m.settings["branch_prefix"] = v
		}
		if v, ok := in["default_model"].(string); ok {
			m.settings["default_model"] = v
		}
		if v, ok := in["default_models"].(map[string]interface{}); ok {
			m.settings["default_models"] = v
		}
		if v, ok := in["default_autopr"].(bool); ok {
			m.settings["default_autopr"] = v
		}
		if v, ok := in["default_notify"].(bool); ok {
			m.settings["default_notify"] = v
		}
		writeJSON(http.StatusOK, m.settings)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/repos":
		writeJSON(http.StatusOK, m.repos)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/repos/branches":
		repoName := strings.TrimSpace(r.URL.Query().Get("name"))
		if repoName == "" {
			http.Error(w, "repo name required", http.StatusBadRequest)
			return
		}
		defaultBranch := "main"
		for _, repo := range m.repos {
			if repo["name"] == repoName {
				if branch, ok := repo["default_branch"].(string); ok && strings.TrimSpace(branch) != "" {
					defaultBranch = strings.TrimSpace(branch)
				}
				break
			}
		}
		writeJSON(http.StatusOK, []map[string]interface{}{
			{"name": defaultBranch, "is_default": true},
			{"name": "develop", "is_default": false},
		})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/repos/discover":
		m.counters.discoverCount++
		writeJSON(http.StatusOK, []map[string]interface{}{
			{
				"id":            "repo-1",
				"name":          "new-repo",
				"nameWithOwner": "owner/new-repo",
				"url":           "https://github.com/owner/new-repo",
				"isPrivate":     false,
				"defaultBranchRef": map[string]interface{}{
					"name": "main",
				},
				"owner": map[string]interface{}{
					"login": "owner",
				},
			},
		})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/repos/import":
		m.counters.importCount++
		// Simulate repo appearing in managed list after import.
		m.repos = append([]map[string]interface{}{
			{
				"name":               "owner/new-repo",
				"url":                "https://github.com/owner/new-repo.git",
				"default_branch":     "main",
				"base_worktree_path": "/tmp/owner-new-repo/base",
			},
		}, m.repos...)
		writeJSON(http.StatusOK, map[string]interface{}{
			"imported": []string{"owner/new-repo"},
		})
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/sessions":
		writeJSON(http.StatusOK, m.sessionSummariesLocked())
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/sessions":
		m.counters.createSessionCount++
		var in map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&in)
		prompt := "New session"
		if v, ok := in["prompt"].(string); ok && strings.TrimSpace(v) != "" {
			prompt = strings.TrimSpace(v)
		}
		id := "session-" + strconv.Itoa(len(m.sessions)+1)
		runID := "run-" + strconv.Itoa(len(m.events)+1)
		now := time.Now().UTC().Format(time.RFC3339)
		session := map[string]interface{}{
			"id":            id,
			"repo_name":     "owner/repo",
			"branch":        "fog/new-session",
			"worktree_path": "/tmp/owner-repo/worktrees/" + id + "-" + runID,
			"tool":          "claude",
			"status":        "CREATED",
			"busy":          true,
			"autopr":        false,
			"pr_url":        "",
			"updated_at":    now,
		}
		m.sessions = append([]map[string]interface{}{session}, m.sessions...)
		m.runs[id] = []map[string]interface{}{
			{
				"id":            runID,
				"session_id":    id,
				"prompt":        prompt,
				"worktree_path": "/tmp/owner-repo/worktrees/" + id + "-" + runID,
				"state":         "CREATED",
				"created_at":    now,
				"updated_at":    now,
			},
		}
		m.events[runID] = []map[string]interface{}{
			{"id": 1, "run_id": runID, "ts": now, "type": "setup", "message": "queued"},
		}
		writeJSON(http.StatusAccepted, map[string]interface{}{
			"session_id": id,
			"run_id":     runID,
			"status":     "accepted",
		})
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/sessions/") {
		path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 && r.Method == http.MethodGet {
			sid := parts[0]
			session := map[string]interface{}{}
			for _, s := range m.sessions {
				if s["id"] == sid {
					session = s
					break
				}
			}
			writeJSON(http.StatusOK, map[string]interface{}{
				"session": session,
				"runs":    m.runs[sid],
			})
			return
		}
		if len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodPost {
			m.counters.followupCount++
			sid := parts[0]
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			prompt := "Follow-up"
			if v, ok := in["prompt"].(string); ok && strings.TrimSpace(v) != "" {
				prompt = strings.TrimSpace(v)
			}
			runID := "run-" + strconv.Itoa(len(m.events)+1)
			now := time.Now().UTC().Format(time.RFC3339)
			m.runs[sid] = append([]map[string]interface{}{
				{
					"id":            runID,
					"session_id":    sid,
					"prompt":        prompt,
					"worktree_path": "/tmp/owner-repo/worktrees/" + sid + "-" + runID,
					"state":         "CREATED",
					"created_at":    now,
					"updated_at":    now,
				},
			}, m.runs[sid]...)
			m.events[runID] = []map[string]interface{}{
				{"id": 1, "run_id": runID, "ts": now, "type": "ai_start", "message": "queued"},
			}
			for _, s := range m.sessions {
				if s["id"] == sid {
					s["busy"] = true
					s["status"] = "AI_RUNNING"
					s["worktree_path"] = "/tmp/owner-repo/worktrees/" + sid + "-" + runID
					s["updated_at"] = now
					break
				}
			}
			writeJSON(http.StatusAccepted, map[string]interface{}{
				"run_id":  runID,
				"status":  "accepted",
				"session": sid,
			})
			return
		}
		if len(parts) == 2 && parts[1] == "fork" && r.Method == http.MethodPost {
			m.counters.forkCount++
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			prompt := "Forked session"
			if v, ok := in["prompt"].(string); ok && strings.TrimSpace(v) != "" {
				prompt = strings.TrimSpace(v)
			}

			id := "session-" + strconv.Itoa(len(m.sessions)+1)
			runID := "run-" + strconv.Itoa(len(m.events)+1)
			now := time.Now().UTC().Format(time.RFC3339)
			session := map[string]interface{}{
				"id":            id,
				"repo_name":     "owner/repo",
				"branch":        "fog/forked-session",
				"worktree_path": "/tmp/owner-repo/worktrees/" + id + "-" + runID,
				"tool":          "claude",
				"status":        "CREATED",
				"busy":          true,
				"autopr":        false,
				"pr_url":        "",
				"updated_at":    now,
			}
			m.sessions = append([]map[string]interface{}{session}, m.sessions...)
			m.runs[id] = []map[string]interface{}{
				{
					"id":            runID,
					"session_id":    id,
					"prompt":        prompt,
					"worktree_path": "/tmp/owner-repo/worktrees/" + id + "-" + runID,
					"state":         "CREATED",
					"created_at":    now,
					"updated_at":    now,
				},
			}
			m.events[runID] = []map[string]interface{}{
				{"id": 1, "run_id": runID, "ts": now, "type": "fork", "message": "fork started"},
			}
			writeJSON(http.StatusAccepted, map[string]interface{}{
				"session_id": id,
				"run_id":     runID,
				"status":     "accepted",
			})
			return
		}
		if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
			m.counters.cancelCount++
			sid := parts[0]
			latest := firstRunID(m.runs[sid])
			now := time.Now().UTC().Format(time.RFC3339)
			for _, run := range m.runs[sid] {
				if run["id"] == latest {
					run["state"] = "CANCELLED"
					run["updated_at"] = now
					break
				}
			}
			m.events[latest] = append(m.events[latest], map[string]interface{}{
				"id": len(m.events[latest]) + 1, "run_id": latest, "ts": now, "type": "cancelled", "message": "Run canceled",
			})
			for _, s := range m.sessions {
				if s["id"] == sid {
					s["busy"] = false
					s["status"] = "CANCELLED"
					s["updated_at"] = now
					break
				}
			}
			writeJSON(http.StatusAccepted, map[string]interface{}{
				"status": "cancel_requested",
				"run_id": latest,
			})
			return
		}
		if len(parts) == 2 && parts[1] == "open" && r.Method == http.MethodPost {
			m.counters.openCount++
			sid := parts[0]
			session := map[string]interface{}{}
			for _, s := range m.sessions {
				if s["id"] == sid {
					session = s
					break
				}
			}
			writeJSON(http.StatusOK, map[string]interface{}{
				"status":        "opened",
				"editor":        "cursor",
				"worktree_path": session["worktree_path"],
			})
			return
		}
		if len(parts) == 2 && parts[1] == "diff" && r.Method == http.MethodGet {
			writeJSON(http.StatusOK, map[string]interface{}{
				"base_branch":   "main",
				"branch":        "fog/session-one",
				"worktree_path": "/tmp/owner-repo/worktrees",
				"stat":          "1 file changed, 2 insertions(+)",
				"patch":         "diff --git a/main.go b/main.go\\n+fmt.Println(\\\"hello\\\")",
			})
			return
		}
		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "events" && r.Method == http.MethodGet {
			runID := parts[2]
			writeJSON(http.StatusOK, m.events[runID])
			return
		}
		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "stream" && r.Method == http.MethodGet {
			runID := parts[2]
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if events := m.events[runID]; len(events) > 0 {
				payload, _ := json.Marshal(events[len(events)-1])
				_, _ = w.Write([]byte("event: run_event\n"))
				_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
			}
			_, _ = w.Write([]byte("event: done\n"))
			_, _ = w.Write([]byte("data: \"COMPLETED\"\n\n"))
			return
		}
	}

	http.NotFound(w, r)
}

func newFrontendHarnessServer(t *testing.T, apiBaseURL string) *httptest.Server {
	t.Helper()

	// Serve the Vite build output directory with API base URL injected into index.html.
	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(distDir); err != nil {
		t.Fatalf("dist directory not found – run 'npm run build' in frontend first: %v", err)
	}

	indexRaw, err := os.ReadFile(filepath.Join(distDir, "index.html"))
	if err != nil {
		t.Fatalf("read frontend dist/index.html failed: %v", err)
	}

	// Inject the mock API base URL before the first <script tag.
	injectScript := `<script>window.__FOG_API_BASE_URL__ = ` + strconv.Quote(apiBaseURL) + `;</script>`
	index := strings.Replace(string(indexRaw), "<script", injectScript+"\n<script", 1)

	fs := http.FileServer(http.Dir(distDir))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(index))
			return
		}
		fs.ServeHTTP(w, r)
	})

	return httptest.NewServer(mux)
}

func waitTextContains(selector, substring string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		deadline := time.Now().Add(12 * time.Second)
		for {
			var out string
			err := chromedp.Run(ctx, chromedp.Text(selector, &out, chromedp.ByQuery))
			if err == nil && strings.Contains(out, substring) {
				return nil
			}
			if time.Now().After(deadline) {
				if err != nil {
					return fmt.Errorf("waitTextContains(%s): %w", selector, err)
				}
				return fmt.Errorf("waitTextContains(%s): %q does not include %q", selector, out, substring)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(120 * time.Millisecond):
			}
		}
	})
}

func waitToastContains(substring string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// svelte-sonner renders toasts as <li data-sonner-toast> inside an <ol data-sonner-toaster>.
		deadline := time.Now().Add(12 * time.Second)
		for {
			var out string
			err := chromedp.Run(ctx, chromedp.Text("[data-sonner-toaster]", &out, chromedp.ByQuery))
			if err == nil && strings.Contains(out, substring) {
				return nil
			}
			if time.Now().After(deadline) {
				if err != nil {
					return fmt.Errorf("waitToastContains: %w (looking for %q)", err, substring)
				}
				return fmt.Errorf("waitToastContains: toast text %q does not include %q", out, substring)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
		}
	})
}

func findChromeBinary() string {
	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}
