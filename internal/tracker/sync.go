package tracker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/task"
	"github.com/google/uuid"
)

// Store is the slice of state.Store the syncer needs. Narrowing it keeps the
// sync logic testable without a database.
type Store interface {
	ListTasks() ([]state.Task, error)
	CreateTask(state.Task) (state.Task, error)
	GetTaskByExternal(provider, externalID string) (state.Task, error)
	MoveTask(id, status string, index int) error
	UpdateTask(state.Task) error
	MarkTaskSynced(id, externalStatus string, at time.Time) error
}

// Syncer reconciles one provider against the local board.
type Syncer struct {
	provider Provider
	store    Store
	statuses StatusMap
}

// NewSyncer builds a syncer for a provider.
func NewSyncer(p Provider, store Store, statuses StatusMap) *Syncer {
	return &Syncer{provider: p, store: store, statuses: statuses}
}

// Result summarises one sync pass.
type Result struct {
	Imported int
	Updated  int
	Pushed   int
	Skipped  int
	Unmapped []string
}

// Sync runs one full reconciliation: pull upstream issues in, then push local
// status changes back out.
//
// Every status change this applies locally is task.OriginRemote. That is the
// whole point: an issue moving into In Progress on Linear must reclassify the
// card here WITHOUT launching an agent, because the person who moved it may not
// be the person sitting at this machine. See task.AutoStarts.
func (s *Syncer) Sync(ctx context.Context) (Result, error) {
	var res Result

	issues, err := s.provider.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list issues: %w", err)
	}

	seen := make(map[string]bool, len(issues))

	for _, issue := range issues {
		seen[issue.ID] = true

		fogStatus, mapped := s.statuses.ToFog(issue.Status)
		if !mapped {
			// Refuse to guess. An unrecognised state left alone is recoverable;
			// one guessed wrong gets written back upstream on the next push.
			res.Unmapped = append(res.Unmapped, issue.Status)
			res.Skipped++
			continue
		}

		existing, err := s.store.GetTaskByExternal(string(s.provider.Name()), issue.ID)
		switch {
		case errors.Is(err, state.ErrTaskNotFound):
			if err := s.importIssue(issue, fogStatus); err != nil {
				return res, err
			}
			res.Imported++

		case err != nil:
			return res, err

		default:
			changed, err := s.reconcile(existing, issue, fogStatus)
			if err != nil {
				return res, err
			}
			if changed {
				res.Updated++
			}
		}
	}

	pushed, err := s.pushLocalChanges(ctx, seen)
	if err != nil {
		return res, err
	}
	res.Pushed = pushed

	return res, nil
}

func (s *Syncer) importIssue(issue Issue, status task.Status) error {
	_, err := s.store.CreateTask(state.Task{
		ID:             uuid.NewString(),
		Title:          issue.Title,
		Body:           issue.Body,
		Status:         status.String(),
		Provider:       string(s.provider.Name()),
		ExternalID:     issue.ID,
		ExternalKey:    issue.Key,
		ExternalURL:    issue.URL,
		ExternalStatus: issue.Status,
		SyncedAt:       ptr(time.Now().UTC()),
	})
	if err != nil {
		return fmt.Errorf("import issue %s: %w", issue.Key, err)
	}
	return nil
}

// reconcile brings a mirrored task in line with upstream.
//
// Upstream wins on title, body and status. That is the trade the user accepted
// by choosing the tracker as the source of truth: if you want a different
// title, change it in Linear or Jira.
func (s *Syncer) reconcile(local state.Task, issue Issue, status task.Status) (bool, error) {
	changed := false

	if local.Title != issue.Title || local.Body != issue.Body {
		local.Title = issue.Title
		local.Body = issue.Body
		if err := s.store.UpdateTask(local); err != nil {
			return false, fmt.Errorf("update task %s: %w", local.ID, err)
		}
		changed = true
	}

	// Only move the card when upstream actually changed state, so a sync pass
	// doesn't stomp a local reorder within the same column.
	if local.ExternalStatus != issue.Status && local.Status != status.String() {
		// Appended to the end of the target column: sync has no opinion about
		// where in a column a card belongs, and guessing would fight the
		// user's own ordering.
		if err := s.store.MoveTask(local.ID, status.String(), 1<<30); err != nil {
			return false, fmt.Errorf("move task %s: %w", local.ID, err)
		}
		changed = true
	}

	if err := s.store.MarkTaskSynced(local.ID, issue.Status, time.Now().UTC()); err != nil {
		return false, err
	}
	return changed, nil
}

// pushLocalChanges writes Fog-side status changes back upstream.
//
// A task is due for a push when its Fog status no longer agrees with the
// external status recorded at the last sync — that difference can only have
// come from someone moving the card here.
func (s *Syncer) pushLocalChanges(ctx context.Context, seen map[string]bool) (int, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return 0, err
	}

	pushed := 0
	for _, t := range tasks {
		if t.Provider != string(s.provider.Name()) || t.ExternalID == "" {
			continue
		}
		if !seen[t.ExternalID] {
			// Not in the upstream result set — deleted, moved out of scope, or
			// filtered. Leave it alone rather than pushing into the unknown.
			continue
		}

		fogStatus, err := task.ParseStatus(t.Status)
		if err != nil {
			continue
		}

		currentRemote, mapped := s.statuses.ToFog(t.ExternalStatus)
		if mapped && currentRemote == fogStatus {
			continue // already in agreement
		}

		if err := s.provider.SetStatus(ctx, t.ExternalID, fogStatus); err != nil {
			// One issue failing to push must not abort the whole pass.
			log.Printf("tracker: push %s status for %s: %v", s.provider.Name(), t.ExternalKey, err)
			continue
		}

		remoteName, _ := s.statuses.ToRemote(fogStatus)
		if err := s.store.MarkTaskSynced(t.ID, remoteName, time.Now().UTC()); err != nil {
			return pushed, err
		}
		pushed++
	}

	return pushed, nil
}

func ptr[T any](v T) *T { return &v }
