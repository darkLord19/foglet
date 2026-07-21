package cloudrelay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/cloud"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
)

type RelayConfig struct {
	PollInterval time.Duration
}

type Relay struct {
	client       *Client
	runner       *runner.Runner
	stateStore   *state.Store
	pollInterval time.Duration
}

func New(client *Client, run *runner.Runner, stateStore *state.Store, cfg RelayConfig) (*Relay, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	if run == nil {
		return nil, errors.New("runner is required")
	}
	if stateStore == nil {
		return nil, errors.New("state store is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	return &Relay{
		client:       client,
		runner:       run,
		stateStore:   stateStore,
		pollInterval: cfg.PollInterval,
	}, nil
}

func (r *Relay) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if ctx.Err() != nil {
			return nil
		}

		processed, err := r.processOne(ctx)
		if err != nil {
			log.Printf("cloud relay error: %v", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(r.pollInterval):
			}
			continue
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(r.pollInterval):
		}
	}
}

func (r *Relay) processOne(ctx context.Context) (bool, error) {
	job, found, err := r.client.ClaimJob(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	payload := r.handleJob(job)
	if err := r.client.CompleteJob(ctx, job.ID, payload); err != nil {
		return true, err
	}
	return true, nil
}

func (r *Relay) handleJob(job cloud.Job) CompletePayload {
	switch strings.TrimSpace(job.Kind) {
	case "start_session":
		return r.handleStartSession(job)
	case "follow_up":
		return r.handleFollowUp(job)
	default:
		return CompletePayload{
			Success: false,
			Error:   fmt.Sprintf("unknown job kind %q", strings.TrimSpace(job.Kind)),
		}
	}
}

func (r *Relay) handleStartSession(job cloud.Job) CompletePayload {
	session, run, err := r.runner.Launch(runner.LaunchRequest{
		Entrypoint: "cloud",
		RepoName:   job.Repo,
		Prompt:     job.Prompt,
		Tool:       job.Tool,
		Model:      job.Model,
		BranchName: job.BranchName,
		AutoPR:     job.AutoPR,
		CommitMsg:  job.CommitMsg,
		// A relayed job did not originate at this machine.
		RejectProtectedBranch: true,
	})
	if err != nil {
		return CompletePayload{Success: false, Error: err.Error()}
	}
	return CompletePayload{
		Success:   true,
		SessionID: session.ID,
		RunID:     run.ID,
		Branch:    session.Branch,
		PRURL:     strings.TrimSpace(session.PRURL),
		CommitSHA: strings.TrimSpace(run.CommitSHA),
		CommitMsg: strings.TrimSpace(run.CommitMsg),
	}
}

func (r *Relay) handleFollowUp(job cloud.Job) CompletePayload {
	sessionID := strings.TrimSpace(job.SessionID)
	if sessionID == "" {
		return CompletePayload{Success: false, Error: "missing session_id"}
	}
	run, err := r.runner.ContinueSession(sessionID, strings.TrimSpace(job.Prompt))
	session, _, _ := r.runner.GetSession(sessionID)
	if err != nil {
		return CompletePayload{
			Success:   false,
			Error:     err.Error(),
			SessionID: sessionID,
			RunID:     strings.TrimSpace(run.ID),
			Branch:    strings.TrimSpace(session.Branch),
			PRURL:     strings.TrimSpace(session.PRURL),
			CommitSHA: strings.TrimSpace(run.CommitSHA),
			CommitMsg: strings.TrimSpace(run.CommitMsg),
		}
	}
	return CompletePayload{
		Success:   true,
		SessionID: sessionID,
		RunID:     run.ID,
		Branch:    strings.TrimSpace(session.Branch),
		PRURL:     strings.TrimSpace(session.PRURL),
		CommitSHA: strings.TrimSpace(run.CommitSHA),
		CommitMsg: strings.TrimSpace(run.CommitMsg),
	}
}
