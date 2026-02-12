package cloud

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	optionPattern  = regexp.MustCompile(`([a-zA-Z-]+)=('([^']*)'|"([^"]*)"|[^\s]+)`)
	mentionPattern = regexp.MustCompile(`<@[^>]+>`)
)

type parsedCommand struct {
	Repo       string
	Tool       string
	Model      string
	AutoPR     bool
	BranchName string
	CommitMsg  string
	Prompt     string
}

func parseCommandText(raw string) (*parsedCommand, error) {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(text), "@fog") {
		text = strings.TrimSpace(text[len("@fog"):])
	}
	if text == "" {
		return nil, fmt.Errorf("invalid command format. Use: @fog [repo='' tool='' model='' autopr=true/false branch-name='' commit-msg=''] prompt")
	}

	if !strings.HasPrefix(text, "[") {
		return nil, fmt.Errorf("options block is required. Use: @fog [repo='...'] prompt")
	}
	end := strings.Index(text, "]")
	if end == -1 {
		return nil, fmt.Errorf("invalid options block: missing closing ]")
	}

	optsText := strings.TrimSpace(text[1:end])
	prompt := strings.TrimSpace(text[end+1:])
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	opts, err := parseOptions(optsText)
	if err != nil {
		return nil, err
	}

	repo := strings.TrimSpace(opts["repo"])
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	autopr := false
	if rawVal, ok := opts["autopr"]; ok && strings.TrimSpace(rawVal) != "" {
		val := strings.ToLower(strings.TrimSpace(rawVal))
		switch val {
		case "true":
			autopr = true
		case "false":
			autopr = false
		default:
			return nil, fmt.Errorf("invalid autopr value %q, expected true/false", rawVal)
		}
	}

	return &parsedCommand{
		Repo:       repo,
		Tool:       strings.TrimSpace(opts["tool"]),
		Model:      strings.TrimSpace(opts["model"]),
		AutoPR:     autopr,
		BranchName: strings.TrimSpace(opts["branch-name"]),
		CommitMsg:  strings.TrimSpace(opts["commit-msg"]),
		Prompt:     prompt,
	}, nil
}

func parseOptions(input string) (map[string]string, error) {
	allowed := map[string]struct{}{
		"repo":        {},
		"tool":        {},
		"model":       {},
		"autopr":      {},
		"branch-name": {},
		"commit-msg":  {},
	}

	matches := optionPattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 && strings.TrimSpace(input) != "" {
		return nil, fmt.Errorf("invalid options format")
	}

	options := make(map[string]string)
	cursor := 0
	for _, m := range matches {
		if strings.TrimSpace(input[cursor:m[0]]) != "" {
			return nil, fmt.Errorf("invalid options format near %q", strings.TrimSpace(input[cursor:m[0]]))
		}

		key := strings.ToLower(input[m[2]:m[3]])
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("unknown option key: %s", key)
		}

		value := input[m[4]:m[5]]
		value = strings.TrimPrefix(value, "'")
		value = strings.TrimSuffix(value, "'")
		value = strings.TrimPrefix(value, "\"")
		value = strings.TrimSuffix(value, "\"")
		options[key] = value
		cursor = m[1]
	}

	if strings.TrimSpace(input[cursor:]) != "" {
		return nil, fmt.Errorf("invalid options format near %q", strings.TrimSpace(input[cursor:]))
	}

	return options, nil
}

func stripMentions(input string) string {
	return strings.TrimSpace(mentionPattern.ReplaceAllString(input, ""))
}

func normalizeFollowUpPrompt(input string) (string, error) {
	prompt := strings.TrimSpace(input)
	if prompt == "" {
		return "", fmt.Errorf("follow-up prompt is required")
	}
	if strings.HasPrefix(prompt, "[") {
		return "", fmt.Errorf("follow-up messages must be plain prompts; options are only allowed for the initial task")
	}
	return prompt, nil
}
