package toolcfg

import (
	"fmt"
	"strings"
)

// DefaultToolReader provides the minimal store interface needed by resolver.
type DefaultToolReader interface {
	GetDefaultTool() (tool string, found bool, err error)
}

// ResolveTool picks the requested tool when provided, otherwise loads default_tool from store.
func ResolveTool(requested string, store DefaultToolReader, entrypoint string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested, nil
	}

	if store == nil {
		return "", fmt.Errorf("AI tool is required (%s): pass a tool or configure default_tool", entrypoint)
	}

	tool, found, err := store.GetDefaultTool()
	if err != nil {
		return "", fmt.Errorf("load default tool: %w", err)
	}
	tool = strings.TrimSpace(tool)
	if !found || tool == "" {
		return "", fmt.Errorf("AI tool is required (%s): run `fog setup` to set default_tool or provide tool explicitly", entrypoint)
	}
	return tool, nil
}
