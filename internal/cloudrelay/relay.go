package cloudrelay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/branchname"
	"github.com/darkLord19/foglet/internal/cloud"
	"github.com/darkLord19/foglet/internal/git"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/toolcfg"
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
	repo, found, err := r.stateStore.GetRepoByName(strings.TrimSpace(job.Repo))
	if err != nil {
		return CompletePayload{Success: false, Error: err.Error()}
	}
	if !found {
		return CompletePayload{Success: false, Error: fmt.Sprintf("unknown repo: %s", strings.TrimSpace(job.Repo))}
	}
	if strings.TrimSpace(repo.BaseWorktreePath) == "" {
		return CompletePayload{Success: false, Error: fmt.Sprintf("repo %s has no base worktree path", repo.Name)}
	}

	tool, err := toolcfg.ResolveTool(strings.TrimSpace(job.Tool), r.stateStore, "cloud")
	if err != nil {
		return CompletePayload{Success: false, Error: err.Error()}
	}
	branch, err := r.resolveBranchName(repo.BaseWorktreePath, strings.TrimSpace(job.BranchName), strings.TrimSpace(job.Prompt))
	if err != nil {
		return CompletePayload{Success: false, Error: err.Error()}
	}

	baseBranch := strings.TrimSpace(repo.DefaultBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}

	session, run, err := r.runner.StartSession(runner.StartSessionOptions{
		RepoName:   repo.Name,
		RepoPath:   repo.BaseWorktreePath,
		Branch:     branch,
		Tool:       tool,
		Model:      strings.TrimSpace(job.Model),
		Prompt:     strings.TrimSpace(job.Prompt),
		AutoPR:     job.AutoPR,
		BaseBranch: baseBranch,
		CommitMsg:  strings.TrimSpace(job.CommitMsg),
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

// resolveBranchName resolves a unique branch name for a relayed job.
//
// repoPath is the repository the branch will be created in; uniqueness is checked
// against it. An empty repoPath disables the check.
func (r *Relay) resolveBranchName(repoPath, requested, prompt string) (string, error) {
	var exists func(string) bool
	if strings.TrimSpace(repoPath) != "" {
		exists = git.New(repoPath).BranchExists
	}

	prefix := ""
	if stored, found, err := r.stateStore.GetSetting("branch_prefix"); err == nil && found {
		prefix = strings.TrimSpace(stored)
	}

	return branchname.Resolve(requested, prefix, prompt, exists)
}
