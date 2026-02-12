package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/wtx/pkg/fog/runner"
	"github.com/yourusername/wtx/pkg/fog/task"
)

// Server provides HTTP API for Fog
type Server struct {
	runner *runner.Runner
	port   int
}

// New creates a new API server
func New(runner *runner.Runner, port int) *Server {
	return &Server{
		runner: runner,
		port:   port,
	}
}

// RegisterRoutes registers API routes on the provided mux
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/create", s.handleCreateTask)
	mux.HandleFunc("/api/tasks/", s.handleTaskDetail)
	mux.HandleFunc("/health", s.handleHealth)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting Fog API server on %s\n", addr)
	
	return http.ListenAndServe(addr, s.corsMiddleware(mux))
}

// handleTasks lists all tasks
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	tasks, err := s.runner.ListTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// CreateTaskRequest represents a task creation request
type CreateTaskRequest struct {
	Branch      string       `json:"branch"`
	Prompt      string       `json:"prompt"`
	AITool      string       `json:"ai_tool"`
	Options     task.Options `json:"options"`
}

// handleCreateTask creates a new task
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Validate request
	if req.Branch == "" || req.Prompt == "" {
		http.Error(w, "branch and prompt are required", http.StatusBadRequest)
		return
	}
	
	if req.AITool == "" {
		req.AITool = "claude"
	}
	
	// Create task
	t := &task.Task{
		ID:        uuid.New().String(),
		State:     task.StateCreated,
		Branch:    req.Branch,
		Prompt:    req.Prompt,
		AITool:    req.AITool,
		Options:   req.Options,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Execute asynchronously if requested
	if req.Options.Async {
		go s.runner.Execute(t)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"task_id": t.ID,
			"status":  "accepted",
		})
		return
	}
	
	// Execute synchronously
	if err := s.runner.Execute(t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleTaskDetail gets task details
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract task ID from path
	taskID := r.URL.Path[len("/api/tasks/"):]
	if taskID == "" {
		http.Error(w, "task ID required", http.StatusBadRequest)
		return
	}
	
	t, err := s.runner.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// handleHealth returns server health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
