package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/state"
)

// newLaunchRunner wires only what resolveLaunch needs. It deliberately has no
// RunStore: these tests exercise resolution, not execution.
func newLaunchRunner(repos fakeRepos, settings fakeSettings) *Runner {
	return &Runner{
		repos:    repos,
		settings: settings,
		baseCtx:  context.Background(),
		power:    newSilentInhibitor(),
		active:   map[string]*activeRun{},
	}
}

func launchRepos() fakeRepos {
	return fakeRepos{
		"acme/api": {
			Name:             "acme/api",
			BaseWorktreePath: "", // empty disables the branch uniqueness check
			DefaultBranch:    "develop",
		},
	}
}

func validRequest() LaunchRequest {
	return LaunchRequest{
		RepoName: "acme/api",
		Prompt:   "Add OTP login",
		Tool:     "claude",
	}
}

func TestResolveLaunchRequiresRepoAndPrompt(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	for name, mutate := range map[string]func(*LaunchRequest){
		"missing repo":   func(q *LaunchRequest) { q.RepoName = "" },
		"blank repo":     func(q *LaunchRequest) { q.RepoName = "   " },
		"missing prompt": func(q *LaunchRequest) { q.Prompt = "" },
		"blank prompt":   func(q *LaunchRequest) { q.Prompt = "  " },
	} {
		t.Run(name, func(t *testing.T) {
			req := validRequest()
			mutate(&req)
			_, err := r.resolveLaunch(req)
			if !errors.Is(err, ErrInvalidLaunch) {
				t.Fatalf("error = %v, want ErrInvalidLaunch", err)
			}
		})
	}
}

func TestResolveLaunchRejectsUnknownRepo(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})
	req := validRequest()
	req.RepoName = "nobody/nothing"

	_, err := r.resolveLaunch(req)
	if !errors.Is(err, ErrUnknownRepo) {
		t.Fatalf("error = %v, want ErrUnknownRepo", err)
	}
}

func TestResolveLaunchBaseBranchFallback(t *testing.T) {
	// The rule that previously existed verbatim at six call sites: requested,
	// then the repo default, then "main".
	tests := []struct {
		name        string
		requested   string
		repoDefault string
		want        string
	}{
		{"explicit request wins", "release/2.0", "develop", "release/2.0"},
		{"falls back to repo default", "", "develop", "develop"},
		{"falls back to main", "", "", "main"},
		{"blank request is not a request", "   ", "develop", "develop"},
		{"blank repo default falls through", "", "  ", "main"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repos := fakeRepos{"acme/api": {Name: "acme/api", DefaultBranch: tc.repoDefault}}
			r := newLaunchRunner(repos, fakeSettings{})

			req := validRequest()
			req.BaseBranch = tc.requested

			opts, err := r.resolveLaunch(req)
			if err != nil {
				t.Fatalf("resolveLaunch: %v", err)
			}
			if opts.BaseBranch != tc.want {
				t.Errorf("BaseBranch = %q, want %q", opts.BaseBranch, tc.want)
			}
		})
	}
}

func TestResolveLaunchToolFallsBackToDefault(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{"default_tool": "cursor"})

	req := validRequest()
	req.Tool = "" // no explicit tool

	opts, err := r.resolveLaunch(req)
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Tool != "cursor" {
		t.Errorf("Tool = %q, want the configured default %q", opts.Tool, "cursor")
	}
}

func TestResolveLaunchExplicitToolWins(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{"default_tool": "cursor"})

	opts, err := r.resolveLaunch(validRequest()) // Tool: "claude"
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Tool != "claude" {
		t.Errorf("Tool = %q, want the explicit %q", opts.Tool, "claude")
	}
}

func TestResolveLaunchFailsWithNoToolAndNoDefault(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	req := validRequest()
	req.Tool = ""

	_, err := r.resolveLaunch(req)
	if !errors.Is(err, ErrInvalidLaunch) {
		t.Fatalf("error = %v, want ErrInvalidLaunch", err)
	}
}

func TestResolveLaunchDerivesBranchFromPrompt(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	opts, err := r.resolveLaunch(validRequest())
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Branch != "fog/add-otp-login" {
		t.Errorf("Branch = %q, want it derived from the prompt", opts.Branch)
	}
}

func TestResolveLaunchHonoursExplicitBranch(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	req := validRequest()
	req.BranchName = "feature/manual"

	opts, err := r.resolveLaunch(req)
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Branch != "feature/manual" {
		t.Errorf("Branch = %q, want the explicit name", opts.Branch)
	}
}

func TestResolveLaunchRejectsInvalidBranch(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	req := validRequest()
	req.BranchName = "bad branch name"

	_, err := r.resolveLaunch(req)
	if !errors.Is(err, ErrInvalidLaunch) {
		t.Fatalf("error = %v, want ErrInvalidLaunch", err)
	}
}

// The divergence that motivated this module: the board's launch path built a
// 7-field options struct where the sessions API built a 13-field one, so a card
// could not open a PR or run validation. One resolver means one option set.
func TestResolveLaunchCarriesEveryRunOption(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	opts, err := r.resolveLaunch(LaunchRequest{
		RepoName:    "acme/api",
		Prompt:      "Add OTP login",
		Tool:        "claude",
		Model:       "sonnet",
		AutoPR:      true,
		SetupCmd:    "make deps",
		Validate:    true,
		ValidateCmd: "make test",
		CommitMsg:   "feat: otp",
		PRTitle:     "Add OTP login",
	})
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}

	for name, got := range map[string]any{
		"RepoName":    opts.RepoName,
		"Tool":        opts.Tool,
		"Model":       opts.Model,
		"AutoPR":      opts.AutoPR,
		"SetupCmd":    opts.SetupCmd,
		"Validate":    opts.Validate,
		"ValidateCmd": opts.ValidateCmd,
		"CommitMsg":   opts.CommitMsg,
		"PRTitle":     opts.PRTitle,
	} {
		if got == "" || got == false {
			t.Errorf("%s was dropped during resolution (got %v)", name, got)
		}
	}
}

func TestResolveLaunchTrimsPassThroughFields(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	opts, err := r.resolveLaunch(LaunchRequest{
		RepoName:  "  acme/api  ",
		Prompt:    "  Add OTP login  ",
		Tool:      "claude",
		Model:     "  sonnet  ",
		CommitMsg: "  feat: otp  ",
		PRTitle:   "  Add OTP  ",
	})
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Model != "sonnet" {
		t.Errorf("Model = %q, want it trimmed", opts.Model)
	}
	if opts.CommitMsg != "feat: otp" {
		t.Errorf("CommitMsg = %q, want it trimmed", opts.CommitMsg)
	}
	if opts.PRTitle != "Add OTP" {
		t.Errorf("PRTitle = %q, want it trimmed", opts.PRTitle)
	}
	if opts.Prompt != "Add OTP login" {
		t.Errorf("Prompt = %q, want it trimmed", opts.Prompt)
	}
}

func TestResolveLaunchUsesRepoIdentityNotRequestSpelling(t *testing.T) {
	repos := fakeRepos{"acme/api": {
		Name:             "acme/api",
		BaseWorktreePath: "/managed/acme/api/base",
		DefaultBranch:    "develop",
	}}
	r := newLaunchRunner(repos, fakeSettings{})

	opts, err := r.resolveLaunch(validRequest())
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	// The run must use the managed repo's own path, never anything the caller
	// supplied — this is what keeps agents out of the user's main checkout.
	if opts.RepoPath != "/managed/acme/api/base" {
		t.Errorf("RepoPath = %q, want the managed base worktree", opts.RepoPath)
	}
}

func TestResolveLaunchWithoutStoreReportsConfigError(t *testing.T) {
	r := New(nil)
	_, err := r.resolveLaunch(validRequest())
	if err == nil {
		t.Fatal("expected an error when no store is configured")
	}
	// Not a caller error: the daemon is misconfigured.
	if errors.Is(err, ErrInvalidLaunch) || errors.Is(err, ErrUnknownRepo) {
		t.Errorf("error = %v, want a configuration error, not a 4xx-shaped one", err)
	}
}

// The Runner holds no *state.Store, so there is no way to build one that is
// half-wired. This used to be a live hazard: a Runner literal setting only
// `state` read no settings and had no run store, and it silently did nothing.
func TestRunnerBuiltFromFakesIsFullyFunctional(t *testing.T) {
	store := newFakeRunStore()
	repos := fakeRepos{"acme/api": {Name: "acme/api", DefaultBranch: "develop"}}
	settings := fakeSettings{"default_tool": "claude"}

	r := &Runner{
		runs:     store,
		repos:    repos,
		settings: settings,
		tools:    toolFactory(&fakeTool{name: "claude", available: true}),
		baseCtx:  context.Background(),
		power:    newSilentInhibitor(),
		active:   map[string]*activeRun{},
	}

	opts, err := r.resolveLaunch(LaunchRequest{RepoName: "acme/api", Prompt: "Add login"})
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Tool != "claude" {
		t.Errorf("Tool = %q, want the faked default", opts.Tool)
	}
	if opts.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want the faked repo default", opts.BaseBranch)
	}

	// Reads go through the same seam, so a fake-built Runner answers queries.
	if err := store.CreateSession(state.Session{ID: "s1", RepoName: "acme/api"}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessions, err := r.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Errorf("ListSessions = %+v, want the seeded session", sessions)
	}
}

func TestResolveLaunchRejectsProtectedBranchWhenAsked(t *testing.T) {
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	req := validRequest()
	req.BranchName = "main"
	req.RejectProtectedBranch = true

	_, err := r.resolveLaunch(req)
	if !errors.Is(err, ErrInvalidLaunch) {
		t.Fatalf("error = %v, want ErrInvalidLaunch", err)
	}
	if !strings.Contains(err.Error(), "protected") {
		t.Errorf("error = %v, want it to mention the protected branch", err)
	}
}

func TestResolveLaunchAllowsProtectedBranchByDefault(t *testing.T) {
	// A launch from the desktop UI on this machine is trusted; only remote
	// origins set the guard.
	r := newLaunchRunner(launchRepos(), fakeSettings{})

	req := validRequest()
	req.BranchName = "main"

	opts, err := r.resolveLaunch(req)
	if err != nil {
		t.Fatalf("resolveLaunch: %v", err)
	}
	if opts.Branch != "main" {
		t.Errorf("Branch = %q, want main", opts.Branch)
	}
}

func TestResolveLaunchProtectedGuardAppliesToDerivedBranches(t *testing.T) {
	// The guard must run on the resolved name, not the requested one — a prompt
	// that slugifies to a protected name must still be refused.
	r := newLaunchRunner(fakeRepos{"acme/api": {Name: "acme/api"}}, fakeSettings{})

	req := LaunchRequest{
		RepoName:              "acme/api",
		Prompt:                "main",
		Tool:                  "claude",
		BranchName:            "develop",
		RejectProtectedBranch: true,
	}
	if _, err := r.resolveLaunch(req); !errors.Is(err, ErrInvalidLaunch) {
		t.Fatalf("error = %v, want ErrInvalidLaunch", err)
	}
}
