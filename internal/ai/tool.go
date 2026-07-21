package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/darkLord19/foglet/internal/binpath"
)

// Tool represents an AI coding tool.
//
// There was briefly a second interface here: Tool carried a non-streaming
// Execute, StreamTool embedded it and added ExecuteStream, and a helper
// type-asserted between them. Every adapter implemented both, Execute was an
// arity adapter that called ExecuteStream with a nil callback, and no caller
// ever wanted the non-streaming form. One shape of adapter behind a seam is a
// hypothetical seam, so the two collapsed into one.
//
// onChunk may be nil when the caller does not want incremental output.
type Tool interface {
	Name() string
	IsAvailable() bool
	ExecuteStream(ctx context.Context, req ExecuteRequest, onChunk func(string)) (*Result, error)
}

// ExecuteRequest configures one tool execution call.
type ExecuteRequest struct {
	Workdir        string
	Prompt         string
	Model          string
	ConversationID string
}

// Result contains the AI execution result
type Result struct {
	Success        bool
	Output         string
	Error          error
	ConversationID string
}

// GetTool returns an AI tool by name
func GetTool(name string) (Tool, error) {
	switch normalizeToolName(name) {
	case "cursor":
		return &Cursor{}, nil
	case "claude", "claude-code":
		return &ClaudeCode{}, nil
	case "antigravity", "agy":
		return &Antigravity{}, nil
	default:
		return nil, fmt.Errorf("unknown AI tool: %s", name)
	}
}

// DetectTool finds an available AI tool
func DetectTool(preferred string) (Tool, error) {
	tools := []Tool{
		&Cursor{},
		&ClaudeCode{},
		&Antigravity{},
	}

	// Try preferred first
	if preferred != "" {
		tool, err := GetTool(preferred)
		if err == nil && tool.IsAvailable() {
			return tool, nil
		}
	}

	// Fall back to first available
	for _, tool := range tools {
		if tool.IsAvailable() {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("no AI tool available")
}

// AvailableToolNames returns canonical tool names supported by Fog.
func AvailableToolNames() []string {
	return []string{"cursor", "claude", "antigravity"}
}

func normalizeToolName(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	switch value {
	case "claude-code":
		return "claude"
	default:
		return value
	}
}

// commandExists checks if a command is available
func commandExists(name string) bool {
	return commandPath(name) != ""
}

var commandPathCache sync.Map

func commandPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if cached, ok := commandPathCache.Load(name); ok {
		if path, ok := cached.(string); ok {
			return path
		}
	}

	if path, err := exec.LookPath(name); err == nil && strings.TrimSpace(path) != "" {
		commandPathCache.Store(name, path)
		return path
	}

	for _, dir := range binpath.FallbackBinDirs() {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS == "windows" {
			commandPathCache.Store(name, candidate)
			return candidate
		}
		if info.Mode()&0o111 != 0 {
			commandPathCache.Store(name, candidate)
			return candidate
		}
	}

	commandPathCache.Store(name, "")
	return ""
}
