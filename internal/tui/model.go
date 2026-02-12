package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourusername/wtx/internal/config"
	"github.com/yourusername/wtx/internal/editor"
	"github.com/yourusername/wtx/internal/git"
	"github.com/yourusername/wtx/internal/metadata"
)

// Model represents the TUI state
type Model struct {
	list         list.Model
	git          *git.Git
	editor       editor.Editor
	config       *config.Config
	metadata     *metadata.Store
	worktrees    []WorktreeItem
	err          error
	quitting     bool
	width        int
	height       int
}

// WorktreeItem wraps a worktree for the list
type WorktreeItem struct {
	worktree git.Worktree
	status   *git.Status
	metadata *metadata.WorktreeMetadata
}

// FilterValue implements list.Item
func (i WorktreeItem) FilterValue() string {
	return i.worktree.Name
}

// Title returns the item title
func (i WorktreeItem) Title() string {
	return i.worktree.Name
}

// Description returns the item description
func (i WorktreeItem) Description() string {
	var parts []string
	
	if i.worktree.Branch != "" {
		parts = append(parts, i.worktree.Branch)
	}
	
	if i.status != nil {
		if i.status.Dirty {
			parts = append(parts, "● dirty")
		} else {
			parts = append(parts, "✓ clean")
		}
		
		if i.status.Ahead > 0 {
			parts = append(parts, fmt.Sprintf("↑%d", i.status.Ahead))
		}
		if i.status.Behind > 0 {
			parts = append(parts, fmt.Sprintf("↓%d", i.status.Behind))
		}
	}
	
	return strings.Join(parts, " • ")
}

// New creates a new TUI model
func New(repoPath string) (*Model, error) {
	g := git.New(repoPath)
	
	if !g.IsRepo() {
		return nil, fmt.Errorf("not a git repository")
	}
	
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	
	root, err := g.GetRepoRoot()
	if err != nil {
		return nil, err
	}
	
	store, err := metadata.New(root)
	if err != nil {
		return nil, err
	}
	
	ed, err := editor.Detect(cfg.Editor)
	if err != nil {
		ed = nil // Non-fatal
	}
	
	// Create list
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Worktree Manager"
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	
	m := &Model{
		list:     l,
		git:      g,
		editor:   ed,
		config:   cfg,
		metadata: store,
	}
	
	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return m.loadWorktrees
}

// loadWorktrees loads the list of worktrees
func (m *Model) loadWorktrees() tea.Msg {
	worktrees, err := m.git.ListWorktrees()
	if err != nil {
		return errMsg{err}
	}
	
	// Load metadata and status for each
	items := make([]WorktreeItem, len(worktrees))
	for i, wt := range worktrees {
		status, _ := m.git.GetStatus(wt.Path)
		meta, _ := m.metadata.GetWorktree(wt.Name)
		
		items[i] = WorktreeItem{
			worktree: wt,
			status:   status,
			metadata: meta,
		}
	}
	
	return worktreesLoaded{items}
}

// Messages
type errMsg struct{ err error }
type worktreesLoaded struct{ items []WorktreeItem }

func (e errMsg) Error() string { return e.err.Error() }
