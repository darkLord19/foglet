//go:build cgo

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	microsandbox "github.com/superradcompany/microsandbox/sdk/go"

	"github.com/darkLord19/foglet/internal/ai"
	"github.com/darkLord19/foglet/internal/sandbox"
)

const (
	sandboxMemoryMiB     = 2048
	anthropicAPIEndpoint = "api.anthropic.com"
	// claudeImage is the GHCR image built by Slice 5.
	claudeImage = "ghcr.io/darklord19/fog-claude:latest"
)

// MicrosandboxBackend runs AI tools inside libkrun microVMs via the
// Microsandbox SDK. It is compiled only when CGO is available.
type MicrosandboxBackend struct{}

// NewMicrosandboxBackend returns a ready-to-use MicrosandboxBackend.
func NewMicrosandboxBackend() *MicrosandboxBackend { return &MicrosandboxBackend{} }

func (b *MicrosandboxBackend) Name() string { return "microsandbox" }

// EnsureReady downloads the msb + libkrunfw runtime on first call (~100 MB).
// Idempotent; safe to call at daemon startup.
func (b *MicrosandboxBackend) EnsureReady(ctx context.Context) error {
	return microsandbox.EnsureInstalled(ctx)
}

// Stop destroys the named sandbox for the given session. Best-effort: errors
// are swallowed so they never block run cleanup.
func (b *MicrosandboxBackend) Stop(ctx context.Context, sessionID string) error {
	_ = microsandbox.RemoveSandbox(ctx, sandboxName(sessionID))
	return nil
}

// RunTool launches a fresh (or resumed) microVM, runs the Claude CLI inside it
// with the session prompt, streams JSON output back to the caller, and returns
// the accumulated output + conversation ID.
func (b *MicrosandboxBackend) RunTool(ctx context.Context, req sandbox.BackendRunRequest) (sandbox.BackendRunResult, error) {
	apiKey, err := resolveAnthropicKey()
	if err != nil {
		return sandbox.BackendRunResult{}, fmt.Errorf("sandbox: resolve API key: %w", err)
	}

	name := sandboxName(req.SessionID)

	// Named volume for Claude session state (ConversationID, local cache).
	// "ensure-exists" creates it on first use; subsequent runs reuse it.
	stateVolName := "fog-claude-state-" + sanitizeVolumeName(req.SessionID)
	stateMount := microsandbox.Mount.NamedWith(stateVolName, microsandbox.MountOptions{},
		microsandbox.NamedVolumeOptions{Mode: "ensure-exists"})

	// Bind-mount the worktree read-write so Claude can see and modify sources.
	worktreeMount := microsandbox.Mount.Bind(req.WorktreePath, microsandbox.MountOptions{})

	sb, err := microsandbox.CreateSandbox(ctx, name,
		microsandbox.WithImage(claudeImage),
		microsandbox.WithRootDisk(microsandbox.RootDisk.Managed(4096)),
		microsandbox.WithMemory(sandboxMemoryMiB),
		microsandbox.WithReplace(),
		microsandbox.WithMounts(map[string]microsandbox.MountConfig{
			"/workspace":    worktreeMount,
			"/root/.claude": stateMount,
		}),
		microsandbox.WithNetwork(&microsandbox.NetworkConfig{
			DefaultEgress:  microsandbox.PolicyActionDeny,
			DefaultIngress: microsandbox.PolicyActionDeny,
			Rules: []microsandbox.PolicyRule{
				microsandbox.Rule.AllowDNS(),
				{
					Action:      microsandbox.PolicyActionAllow,
					Direction:   microsandbox.PolicyDirectionEgress,
					Destination: anthropicAPIEndpoint,
				},
			},
		}),
		microsandbox.WithSecrets(microsandbox.Secret.Env(
			"ANTHROPIC_API_KEY",
			apiKey,
			microsandbox.SecretEnvOptions{
				AllowHosts: []string{anthropicAPIEndpoint},
			},
		)),
	)
	if err != nil {
		return sandbox.BackendRunResult{}, fmt.Errorf("sandbox: create: %w", err)
	}
	defer sb.Close()

	cmd := buildClaudeCommand(req)
	handle, err := sb.ShellStream(ctx, cmd)
	if err != nil {
		return sandbox.BackendRunResult{}, fmt.Errorf("sandbox: shell stream: %w", err)
	}
	defer handle.Close()

	parser := ai.NewJSONStreamParser(req.OnChunk)
	for {
		ev, err := handle.Recv(ctx)
		if err != nil {
			return sandbox.BackendRunResult{}, fmt.Errorf("sandbox: recv: %w", err)
		}
		switch ev.Kind {
		case microsandbox.ExecEventStdout:
			parser.Feed(ev.Data)
		case microsandbox.ExecEventExited:
			parser.Close()
			if ev.ExitCode != 0 {
				out := strings.TrimSpace(parser.Output())
				return sandbox.BackendRunResult{
					Output:         out,
					ConversationID: parser.ConversationID(),
				}, fmt.Errorf("claude exited %d: %s", ev.ExitCode, truncate(out, 400))
			}
			return sandbox.BackendRunResult{
				Output:         strings.TrimSpace(parser.Output()),
				ConversationID: parser.ConversationID(),
			}, nil
		case microsandbox.ExecEventFailed:
			return sandbox.BackendRunResult{}, fmt.Errorf("sandbox: exec failed: %v", ev.Failure)
		case microsandbox.ExecEventDone:
			parser.Close()
			return sandbox.BackendRunResult{
				Output:         strings.TrimSpace(parser.Output()),
				ConversationID: parser.ConversationID(),
			}, nil
		}
	}
}

// compile-time proof that MicrosandboxBackend satisfies the Backend interface.
var _ sandbox.Backend = (*MicrosandboxBackend)(nil)

// buildClaudeCommand assembles the shell command to run inside the VM.
// Shell-quotes the prompt to safely handle any characters.
func buildClaudeCommand(req sandbox.BackendRunRequest) string {
	var sb strings.Builder
	sb.WriteString("claude -p ")
	sb.WriteString(shellQuote(req.Prompt))
	if model := strings.TrimSpace(req.Model); model != "" {
		sb.WriteString(" --model ")
		sb.WriteString(shellQuote(model))
	}
	if cid := strings.TrimSpace(req.ConversationID); cid != "" {
		sb.WriteString(" --resume ")
		sb.WriteString(shellQuote(cid))
	}
	sb.WriteString(" --output-format stream-json")
	return sb.String()
}

// shellQuote single-quotes s for safe use in a POSIX shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// sandboxName returns the msb sandbox name for a session.
// Names are capped at 64 bytes to stay well under msb limits.
func sandboxName(sessionID string) string {
	name := "fog-" + sessionID
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// sanitizeVolumeName trims the session ID to a safe named-volume suffix
// (≤ 60 bytes; the prefix "fog-claude-state-" is 18 bytes, total ≤ 78).
func sanitizeVolumeName(sessionID string) string {
	if len(sessionID) > 60 {
		return sessionID[:60]
	}
	return sessionID
}

// resolveAnthropicKey returns the Anthropic API key from the environment or,
// as a fallback, from ~/.claude.json. The key is injected as a secret and
// never reaches the guest VM directly.
func resolveAnthropicKey() (string, error) {
	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		return key, nil
	}
	return readClaudeJSONKey()
}

// readClaudeJSONKey extracts the API key from ~/.claude.json.
func readClaudeJSONKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set and ~/.claude.json not readable: %w", err)
	}
	var cfg struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse ~/.claude.json: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set and no apiKey in ~/.claude.json")
	}
	return strings.TrimSpace(cfg.APIKey), nil
}
