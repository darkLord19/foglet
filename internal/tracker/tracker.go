// Package tracker mirrors issues from an external work tracker into Fog's
// board, and pushes Fog's status changes back.
//
// The central problem this package solves is vocabulary. Fog has exactly four
// statuses (internal/task). Linear has workflow states with a coarse type;
// Jira has states that are configured per project and cannot be predicted at
// all. StatusMap translates at the boundary so no foreign status ever reaches
// the rest of the app.
//
// The security rule from internal/task applies to everything here: issues
// arriving from a tracker are OriginRemote, and remote transitions never start
// an agent. A card that lands in a working column via sync gets a Start button;
// a human decides.
package tracker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/task"
)

// Issue is one work item as the upstream tracker sees it.
type Issue struct {
	// ID is the provider's stable identifier, used for reconciliation.
	ID string
	// Key is what a human recognises: "ENG-421".
	Key       string
	Title     string
	Body      string
	Status    string
	URL       string
	UpdatedAt time.Time
}

// Provider is one tracker integration.
type Provider interface {
	// Name identifies the provider for storage and display.
	Name() task.Provider

	// List returns the issues Fog should mirror. Implementations scope this
	// to the configured project/team and the authenticated user — syncing an
	// entire Jira instance into a personal board helps nobody.
	List(ctx context.Context) ([]Issue, error)

	// SetStatus pushes a Fog status change upstream. Implementations resolve
	// the Fog status to a provider state via their StatusMap.
	SetStatus(ctx context.Context, issueID string, status task.Status) error
}

// ErrNotConfigured is returned when a provider has no credentials.
var ErrNotConfigured = errors.New("tracker not configured")

// StatusMap translates between Fog's four statuses and a provider's own.
//
// Each Fog status lists the remote status names that mean it. Matching is
// case-insensitive and whitespace-tolerant, because tracker admins write
// "In Progress", "in-progress" and "IN PROGRESS" interchangeably.
//
// The FIRST entry in each list is what Fog writes back upstream, so order is
// meaningful: put the canonical name first and the aliases after.
type StatusMap struct {
	Todo       []string `json:"todo"`
	InProgress []string `json:"in_progress"`
	InReview   []string `json:"in_review"`
	Done       []string `json:"done"`
}

// DefaultStatusMap covers the naming most teams actually use. Jira setups that
// diverge need an explicit map; see the Jira provider.
func DefaultStatusMap() StatusMap {
	return StatusMap{
		Todo:       []string{"Todo", "To Do", "Backlog", "Open", "Unstarted"},
		InProgress: []string{"In Progress", "Started", "Doing"},
		InReview:   []string{"In Review", "Review", "Code Review", "Reviewing"},
		Done:       []string{"Done", "Completed", "Closed", "Merged", "Resolved"},
	}
}

func normalise(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(s, "-", " ")), " "))
}

// ToFog maps a provider status onto a Fog status.
//
// Unknown statuses deliberately return false rather than defaulting to Todo:
// silently dragging an issue into Todo because Fog didn't recognise its state
// would misrepresent the upstream board, and on a bidirectional sync that
// misrepresentation gets written back.
func (m StatusMap) ToFog(remote string) (task.Status, bool) {
	target := normalise(remote)
	if target == "" {
		return "", false
	}

	for _, candidate := range []struct {
		names  []string
		status task.Status
	}{
		{m.InReview, task.StatusInReview},
		{m.InProgress, task.StatusInProgress},
		{m.Done, task.StatusDone},
		{m.Todo, task.StatusTodo},
	} {
		for _, name := range candidate.names {
			if normalise(name) == target {
				return candidate.status, true
			}
		}
	}
	return "", false
}

// ToRemote maps a Fog status onto the provider status Fog should write.
func (m StatusMap) ToRemote(status task.Status) (string, bool) {
	var names []string
	switch status {
	case task.StatusTodo:
		names = m.Todo
	case task.StatusInProgress:
		names = m.InProgress
	case task.StatusInReview:
		names = m.InReview
	case task.StatusDone:
		names = m.Done
	default:
		return "", false
	}

	for _, n := range names {
		if strings.TrimSpace(n) != "" {
			return n, true
		}
	}
	return "", false
}

// Validate reports whether the map can round-trip every Fog status.
func (m StatusMap) Validate() error {
	missing := []string{}
	for _, s := range task.Statuses() {
		if _, ok := m.ToRemote(s); !ok {
			missing = append(missing, s.String())
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("status map has no remote state for: %s", strings.Join(missing, ", "))
	}
	return nil
}
