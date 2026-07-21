package runner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/darkLord19/foglet/internal/branchname"
	"github.com/darkLord19/foglet/internal/state"
	"github.com/darkLord19/foglet/internal/toolcfg"
)

// LaunchRequest is un-resolved intent: what a person or an external caller
// asked for, before Fog has decided which repo, tool, branch or base branch
// that actually means.
//
// Only RepoName and Prompt are required. Every other field is either optional
// with a resolution rule, or a straight pass-through to the run.
type LaunchRequest struct {
	// Entrypoint names the caller in error messages ("api", "task", "cloud").
	Entrypoint string

	RepoName string
	Prompt   string

	// Tool falls back to the configured default_tool.
	Tool  string
	Model string

	// BranchName is derived from Prompt when empty.
	BranchName string
	// BaseBranch falls back to the repo's default branch, then "main".
	BaseBranch string

	AutoPR      bool
	SetupCmd    string
	Validate    bool
	ValidateCmd string
	CommitMsg   string
	PRTitle     string

	// RejectProtectedBranch refuses to run against an integration branch.
	//
	// Set it for launches that did not originate at this machine. A Slack
	// command is issued by whoever is in the channel, so it must not be able to
	// point an agent at main; a request from the desktop UI on this machine may.
	RejectProtectedBranch bool

	// Async schedules the run in the background and returns immediately.
	Async bool
}

// ErrUnknownRepo is returned when the named repo is not managed by Fog.
var ErrUnknownRepo = errors.New("unknown repo")

// ErrInvalidLaunch is returned when a request is missing something required, or
// a resolution rule rejects it. Callers map this to a 4xx; anything else is a
// genuine failure.
var ErrInvalidLaunch = errors.New("invalid launch request")

// Launch resolves a LaunchRequest and starts a session for it.
//
// This is the single place the repo, tool, branch and base-branch rules live.
// They previously existed at four call sites — the sessions API, the task
// board, the cloud relay and Slack — which had already drifted: the board's
// copy silently omitted AutoPR, SetupCmd, Validate, ValidateCmd, CommitMsg and
// PRTitle, so a card started from the board could not open a PR or run
// validation.
//
// The returned run is the first run of the new session. When Async is set it is
// the pre-execution record and the caller must poll for progress.
func (r *Runner) Launch(req LaunchRequest) (state.Session, state.Run, error) {
	opts, err := r.resolveLaunch(req)
	if err != nil {
		return state.Session{}, state.Run{}, err
	}
	if req.Async {
		return r.StartSessionAsync(opts)
	}
	return r.StartSession(opts)
}

// resolveLaunch turns intent into a fully-resolved StartSessionOptions.
func (r *Runner) resolveLaunch(req LaunchRequest) (StartSessionOptions, error) {
	if r.repos == nil || r.settings == nil {
		return StartSessionOptions{}, errors.New("state store not configured")
	}

	repoName := strings.TrimSpace(req.RepoName)
	prompt := strings.TrimSpace(req.Prompt)
	if repoName == "" {
		return StartSessionOptions{}, fmt.Errorf("%w: repo is required", ErrInvalidLaunch)
	}
	if prompt == "" {
		return StartSessionOptions{}, fmt.Errorf("%w: prompt is required", ErrInvalidLaunch)
	}

	repo, found, err := r.repos.GetRepoByName(repoName)
	if err != nil {
		return StartSessionOptions{}, err
	}
	if !found {
		return StartSessionOptions{}, fmt.Errorf("%w: %s", ErrUnknownRepo, repoName)
	}

	entrypoint := strings.TrimSpace(req.Entrypoint)
	if entrypoint == "" {
		entrypoint = "api"
	}
	tool, err := toolcfg.ResolveTool(req.Tool, r.settings, entrypoint)
	if err != nil {
		return StartSessionOptions{}, fmt.Errorf("%w: %s", ErrInvalidLaunch, err)
	}

	branch, err := r.ResolveBranch(repo.BaseWorktreePath, req.BranchName, prompt)
	if err != nil {
		return StartSessionOptions{}, fmt.Errorf("%w: %s", ErrInvalidLaunch, err)
	}
	if req.RejectProtectedBranch && branchname.IsProtected(branch) {
		return StartSessionOptions{}, fmt.Errorf("%w: protected branch %q is not allowed", ErrInvalidLaunch, branch)
	}

	return StartSessionOptions{
		RepoName:    repo.Name,
		RepoPath:    repo.BaseWorktreePath,
		Branch:      branch,
		Tool:        tool,
		Model:       strings.TrimSpace(req.Model),
		Prompt:      prompt,
		AutoPR:      req.AutoPR,
		SetupCmd:    strings.TrimSpace(req.SetupCmd),
		Validate:    req.Validate,
		ValidateCmd: strings.TrimSpace(req.ValidateCmd),
		BaseBranch:  resolveBaseBranch(req.BaseBranch, repo.DefaultBranch),
		CommitMsg:   strings.TrimSpace(req.CommitMsg),
		PRTitle:     strings.TrimSpace(req.PRTitle),
	}, nil
}

// resolveBaseBranch applies the requested base, then the repo's default, then
// "main". This rule previously appeared verbatim at six call sites.
func resolveBaseBranch(requested, repoDefault string) string {
	if branch := strings.TrimSpace(requested); branch != "" {
		return branch
	}
	if branch := strings.TrimSpace(repoDefault); branch != "" {
		return branch
	}
	return "main"
}
