package ghcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverReposIncludesOrgRepos(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "success")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	got, err := DiscoverRepos()
	if err != nil {
		t.Fatalf("DiscoverRepos returned error: %v", err)
	}

	names := make([]string, 0, len(got))
	for _, repo := range got {
		names = append(names, repo.NameWithOwner)
	}

	want := []string{"me/personal", "acme/service", "openai/gpt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected repo list: got %v want %v", names, want)
	}
}

func TestDiscoverReposDedupesByFullName(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "dedupe")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	got, err := DiscoverRepos()
	if err != nil {
		t.Fatalf("DiscoverRepos returned error: %v", err)
	}

	names := make([]string, 0, len(got))
	for _, repo := range got {
		names = append(names, repo.NameWithOwner)
	}

	want := []string{"acme/service"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected repo list: got %v want %v", names, want)
	}
}

func TestCloneRepoUsesBloblessFilter(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "clone_success_filter")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	destPath := filepath.Join(t.TempDir(), "repo.git")
	if err := CloneRepo("acme/service", destPath); err != nil {
		t.Fatalf("CloneRepo returned error: %v", err)
	}
}

func TestCloneRepoFallsBackWhenFilterUnsupported(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "clone_fallback")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	destPath := filepath.Join(t.TempDir(), "repo.git")
	if err := CloneRepo("acme/service", destPath); err != nil {
		t.Fatalf("CloneRepo returned error: %v", err)
	}
}

func TestDiscoverReposOrgRepoListErrorMentionsOwner(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "org_repo_list_error")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	_, err := DiscoverRepos()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gh repo list acme failed") {
		t.Fatalf("expected error to mention owner: %v", err)
	}
}

func TestCreatePRReturnsURL(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "pr_create_success")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	got, err := CreatePR(t.TempDir(), "my title", "my body", "main", "feature", false)
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if got != "https://example.com/pr/123" {
		t.Fatalf("unexpected PR URL: got %q", got)
	}
}

func TestCreatePRDraftAddsDraftFlag(t *testing.T) {
	t.Setenv("FOG_GHCLI_TEST_CASE", "pr_create_draft")

	origExec := execCommand
	origPath := ghPathFn
	t.Cleanup(func() {
		execCommand = origExec
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }
	execCommand = stubExecCommand()

	got, err := CreatePR(t.TempDir(), "my title", "my body", "main", "feature", true)
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if got != "https://example.com/pr/123" {
		t.Fatalf("unexpected PR URL: got %q", got)
	}
}

func TestCreatePRWithContextBuildsArgsAndWrapsOutput(t *testing.T) {
	origProcRun := procRun
	origPath := ghPathFn
	t.Cleanup(func() {
		procRun = origProcRun
		ghPathFn = origPath
	})

	ghPathFn = func() string { return "/test/gh" }

	var gotDir, gotName string
	var gotArgs []string
	sentinel := errors.New("boom")
	procRun = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		gotDir = dir
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("nope\n"), sentinel
	}

	_, err := CreatePRWithContext(context.Background(), "/repo", "my title", "my body", "main", "feature", true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error to wrap sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "gh pr create failed") {
		t.Fatalf("expected error to include command context: %v", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error to include output: %v", err)
	}
	if gotDir != "/repo" {
		t.Fatalf("unexpected dir: got %q", gotDir)
	}
	if gotName != "/test/gh" {
		t.Fatalf("unexpected command: got %q", gotName)
	}

	wantArgs := []string{"pr", "create", "--base", "main", "--head", "feature", "--title", "my title", "--body", "my body", "--draft"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func stubExecCommand() func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+2 > len(args) {
		os.Exit(2)
	}

	// args[sep+1] is the executable path passed to execCommand ("gh" path); ignore it.
	ghArgs := args[sep+2:]
	if len(ghArgs) < 2 {
		os.Exit(2)
	}

	testCase := os.Getenv("FOG_GHCLI_TEST_CASE")
	switch ghArgs[0] {
	case "org":
		if ghArgs[1] != "list" {
			os.Exit(2)
		}
		switch testCase {
		case "success":
			_, _ = os.Stdout.WriteString("acme\nopenai\n\n")
			os.Exit(0)
		case "dedupe", "org_repo_list_error":
			_, _ = os.Stdout.WriteString("acme\n")
			os.Exit(0)
		default:
			os.Exit(2)
		}
	case "repo":
		switch ghArgs[1] {
		case "list":
			// gh repo list [OWNER] --json ... --limit ...
			owner := ""
			if len(ghArgs) >= 3 && !strings.HasPrefix(ghArgs[2], "-") {
				owner = ghArgs[2]
			}

			switch testCase {
			case "success":
				switch owner {
				case "":
					_, _ = os.Stdout.WriteString(`[{"id":"R1","name":"personal","nameWithOwner":"me/personal","url":"https://github.com/me/personal","isPrivate":true,"defaultBranchRef":{"name":"main"},"owner":{"login":"me"}}]`)
					os.Exit(0)
				case "acme":
					_, _ = os.Stdout.WriteString(`[{"id":"R2","name":"service","nameWithOwner":"acme/service","url":"https://github.com/acme/service","isPrivate":false,"defaultBranchRef":{"name":"master"},"owner":{"login":"acme"}}]`)
					os.Exit(0)
				case "openai":
					_, _ = os.Stdout.WriteString(`[{"id":"R3","name":"gpt","nameWithOwner":"openai/gpt","url":"https://github.com/openai/gpt","isPrivate":true,"defaultBranchRef":{"name":"main"},"owner":{"login":"openai"}}]`)
					os.Exit(0)
				default:
					os.Exit(2)
				}
			case "dedupe":
				switch owner {
				case "":
					_, _ = os.Stdout.WriteString(`[{"id":"R2","name":"service","nameWithOwner":"acme/service","url":"https://github.com/acme/service","isPrivate":false,"defaultBranchRef":{"name":"master"},"owner":{"login":"acme"}}]`)
					os.Exit(0)
				case "acme":
					_, _ = os.Stdout.WriteString(`[{"id":"R2","name":"service","nameWithOwner":"acme/service","url":"https://github.com/acme/service","isPrivate":false,"defaultBranchRef":{"name":"master"},"owner":{"login":"acme"}}]`)
					os.Exit(0)
				default:
					os.Exit(2)
				}
			case "org_repo_list_error":
				switch owner {
				case "":
					_, _ = os.Stdout.WriteString(`[]`)
					os.Exit(0)
				case "acme":
					_, _ = os.Stderr.WriteString("boom\n")
					os.Exit(1)
				default:
					os.Exit(2)
				}
			default:
				os.Exit(2)
			}
		case "clone":
			if len(ghArgs) < 4 {
				os.Exit(2)
			}
			hasBare := false
			hasFilter := false
			for _, a := range ghArgs[4:] {
				if a == "--bare" {
					hasBare = true
				}
				if a == "--filter=blob:none" {
					hasFilter = true
				}
			}
			if !hasBare {
				os.Exit(2)
			}

			switch testCase {
			case "clone_success_filter":
				if !hasFilter {
					os.Exit(2)
				}
				os.Exit(0)
			case "clone_fallback":
				if hasFilter {
					_, _ = os.Stderr.WriteString("error: unknown option `--filter=blob:none`\n")
					os.Exit(1)
				}
				os.Exit(0)
			case "clone_fail":
				_, _ = os.Stderr.WriteString("boom\n")
				os.Exit(1)
			default:
				os.Exit(2)
			}
		default:
			os.Exit(2)
		}
	case "pr":
		if ghArgs[1] != "create" {
			os.Exit(2)
		}
		hasDraft := false
		for _, a := range ghArgs[2:] {
			if a == "--draft" {
				hasDraft = true
			}
		}
		switch testCase {
		case "pr_create_success":
			if hasDraft {
				os.Exit(2)
			}
			_, _ = os.Stdout.WriteString("https://example.com/pr/123\n")
			os.Exit(0)
		case "pr_create_draft":
			if !hasDraft {
				os.Exit(2)
			}
			_, _ = os.Stdout.WriteString("https://example.com/pr/123\n")
			os.Exit(0)
		default:
			os.Exit(2)
		}
	default:
		os.Exit(2)
	}
}
