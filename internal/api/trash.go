package api

import (
	"context"
	"log"
	"strconv"
	"time"
)

const (
	// settingTrashRetentionDays is the store key for how long trashed tasks
	// stay recoverable before a purge reclaims their worktree and branch.
	settingTrashRetentionDays = "trash_retention_days"

	// defaultTrashRetentionDays is the shipped default when the user has not
	// chosen one.
	defaultTrashRetentionDays = 7

	// trashJanitorInterval is how often the daemon sweeps for expired trash.
	trashJanitorInterval = 1 * time.Hour
)

// trashRetentionDays reads the configured retention, falling back to the default
// for an unset, malformed, or non-positive value.
func (s *Server) trashRetentionDays() int {
	val, found, err := s.stateStore.GetSetting(settingTrashRetentionDays)
	if err != nil || !found {
		return defaultTrashRetentionDays
	}
	days, err := strconv.Atoi(val)
	if err != nil || days < 1 {
		return defaultTrashRetentionDays
	}
	return days
}

// StartTrashJanitor runs an immediate purge, then keeps purging on an interval
// until ctx is cancelled. The daemon owns the lifecycle via the context it
// passes to app.Build.
func (s *Server) StartTrashJanitor(ctx context.Context) {
	if n, err := s.purgeExpiredTrash(); err != nil {
		log.Printf("trash janitor: initial purge failed: %v", err)
	} else if n > 0 {
		log.Printf("trash janitor: purged %d expired task(s)", n)
	}

	go func() {
		ticker := time.NewTicker(trashJanitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := s.purgeExpiredTrash(); err != nil {
					log.Printf("trash janitor: purge failed: %v", err)
				} else if n > 0 {
					log.Printf("trash janitor: purged %d expired task(s)", n)
				}
			}
		}
	}()
}

// purgeExpiredTrash permanently deletes tasks whose retention has lapsed,
// reclaiming each linked session's worktree and branch first. It is best-effort
// per task: a worktree that fails to remove is logged but does not block the
// row's deletion, so a wedged worktree never keeps a card stuck in trash.
func (s *Server) purgeExpiredTrash() (int, error) {
	cutoff := time.Now().Add(-time.Duration(s.trashRetentionDays()) * 24 * time.Hour)
	expired, err := s.stateStore.ListTrashedBefore(cutoff)
	if err != nil {
		return 0, err
	}

	purged := 0
	for _, t := range expired {
		if t.SessionID != "" {
			if err := s.runner.RemoveSessionArtifacts(t.SessionID); err != nil {
				log.Printf("trash janitor: task %s: %v", t.ID, err)
			}
		}
		if err := s.stateStore.DeleteTask(t.ID); err != nil {
			log.Printf("trash janitor: delete task %s: %v", t.ID, err)
			continue
		}
		purged++
	}
	return purged, nil
}
