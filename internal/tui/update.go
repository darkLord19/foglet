package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
)

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
		
	case tea.KeyMsg:
		// Don't match any of the keys if we're filtering
		if m.list.FilterState() == list.Filtering {
			break
		}
		
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
			
		case "enter":
			// Open selected worktree
			if i, ok := m.list.SelectedItem().(WorktreeItem); ok {
				return m, m.openWorktree(i)
			}
			
		case "d":
			// Delete worktree (TODO: implement)
			return m, nil
			
		case "r":
			// Refresh list
			return m, m.loadWorktrees
		}
		
	case worktreesLoaded:
		m.worktrees = msg.items
		items := make([]list.Item, len(msg.items))
		for i, item := range msg.items {
			items[i] = item
		}
		cmd := m.list.SetItems(items)
		return m, cmd
		
	case errMsg:
		m.err = msg.err
		return m, nil
	}
	
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the UI
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	
	return m.list.View()
}

// openWorktree opens a worktree in the configured editor
func (m *Model) openWorktree(item WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		if m.editor == nil {
			return errMsg{fmt.Errorf("no editor configured")}
		}
		
		if err := m.editor.Open(item.worktree.Path, m.config.ReuseWindow); err != nil {
			return errMsg{err}
		}
		
		// Update last opened timestamp
		m.metadata.UpdateLastOpened(item.worktree.Name)
		
		// Quit after opening
		return tea.Quit()
	}
}
