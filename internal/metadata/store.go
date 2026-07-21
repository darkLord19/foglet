package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/darkLord19/foglet/internal/git"
)

// Store manages worktree metadata
type Store struct {
	path string
	mu   sync.RWMutex
}

// Metadata contains all stored data
type Metadata struct {
	Worktrees map[string]*WorktreeMetadata `json:"worktrees"`
	Version   string                       `json:"version"`
}

// WorktreeMetadata stores metadata for a single worktree
type WorktreeMetadata struct {
	Path         string    `json:"path"`
	CreatedAt    time.Time `json:"created_at"`
	LastOpened   time.Time `json:"last_opened"`
	DevCommand   string    `json:"dev_command,omitempty"`
	Ports        []int     `json:"ports,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	SetupRan     bool      `json:"setup_ran"`
	SetupOutput  string    `json:"setup_output,omitempty"`
	LastValidate time.Time `json:"last_validate"`
	ValidatePass bool      `json:"validate_pass"`
}

// New creates a new metadata store
func New(repoPath string) (*Store, error) {
	// Metadata is stored in the git common dir so all worktrees share one store.
	gitDir, err := resolveGitCommonDir(repoPath)
	if err != nil {
		return nil, err
	}
	wtxDir := filepath.Join(gitDir, "wtx")

	// Create wtx directory if it doesn't exist
	if err := os.MkdirAll(wtxDir, 0755); err != nil {
		return nil, fmt.Errorf("create wtx dir: %w", err)
	}

	metaPath := filepath.Join(wtxDir, "metadata.json")

	return &Store{path: metaPath}, nil
}

func resolveGitCommonDir(repoPath string) (string, error) {
	gitDir, err := git.New(repoPath).CommonDir()
	if err != nil {
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	return gitDir, nil
}

// Get retrieves all metadata
func (s *Store) Get() (*Metadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If file doesn't exist, return empty metadata
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return &Metadata{
			Worktrees: make(map[string]*WorktreeMetadata),
			Version:   "1.0",
		}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	// Ensure map is initialized
	if meta.Worktrees == nil {
		meta.Worktrees = make(map[string]*WorktreeMetadata)
	}

	return &meta, nil
}

// Update modifies metadata atomically
func (s *Store) Update(fn func(*Metadata) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.get()
	if err != nil {
		return err
	}

	if err := fn(meta); err != nil {
		return err
	}

	return s.save(meta)
}

// get retrieves metadata without lock (internal use only)
func (s *Store) get() (*Metadata, error) {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return &Metadata{
			Worktrees: make(map[string]*WorktreeMetadata),
			Version:   "1.0",
		}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	if meta.Worktrees == nil {
		meta.Worktrees = make(map[string]*WorktreeMetadata)
	}

	return &meta, nil
}

// save writes metadata atomically
func (s *Store) save(meta *Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Write to temp file first
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// GetWorktree retrieves metadata for a specific worktree
func (s *Store) GetWorktree(name string) (*WorktreeMetadata, error) {
	meta, err := s.Get()
	if err != nil {
		return nil, err
	}

	wt, ok := meta.Worktrees[name]
	if !ok {
		return nil, nil
	}

	return wt, nil
}

// SetWorktree updates metadata for a worktree
func (s *Store) SetWorktree(name string, wtMeta *WorktreeMetadata) error {
	return s.Update(func(meta *Metadata) error {
		meta.Worktrees[name] = wtMeta
		return nil
	})
}

// DeleteWorktree removes metadata for a worktree
func (s *Store) DeleteWorktree(name string) error {
	return s.Update(func(meta *Metadata) error {
		delete(meta.Worktrees, name)
		return nil
	})
}

// UpdateLastOpened updates the last opened timestamp
func (s *Store) UpdateLastOpened(name string) error {
	return s.Update(func(meta *Metadata) error {
		if wt, ok := meta.Worktrees[name]; ok {
			wt.LastOpened = time.Now()
		}
		return nil
	})
}
