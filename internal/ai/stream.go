package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
)

type streamJSONParser struct {
	pending        bytes.Buffer
	output         bytes.Buffer
	conversationID string
	onChunk        func(string)
}

func newStreamJSONParser(onChunk func(string)) *streamJSONParser {
	return &streamJSONParser{onChunk: onChunk}
}

func (p *streamJSONParser) Feed(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	_, _ = p.pending.Write(chunk)
	p.consumeLines(false)
}

func (p *streamJSONParser) Close() {
	p.consumeLines(true)
}

func (p *streamJSONParser) consumeLines(flush bool) {
	for {
		data := p.pending.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := string(data[:idx])
		p.pending.Next(idx + 1)
		p.processLine(line)
	}
	if flush && p.pending.Len() > 0 {
		line := p.pending.String()
		p.pending.Reset()
		p.processLine(line)
	}
}

func (p *streamJSONParser) processLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		if p.onChunk != nil {
			p.onChunk(line + "\n")
		}
		return
	}

	if p.conversationID == "" {
		p.conversationID = extractConversationID(payload)
	}

	text := extractStreamText(payload)
	if strings.TrimSpace(text) == "" {
		return
	}
	_, _ = p.output.WriteString(text)
	if p.onChunk != nil {
		p.onChunk(text)
	}
}

func (p *streamJSONParser) Output() string {
	return strings.TrimSpace(p.output.String())
}

func (p *streamJSONParser) ConversationID() string {
	return strings.TrimSpace(p.conversationID)
}

func runJSONStreamingCommand(ctx context.Context, workdir, cmdName string, args []string, onChunk func(string)) (output, conversationID string, err error) {
	parser := newStreamJSONParser(onChunk)
	raw, err := runGuardedStreaming(ctx, workdir, cmdName, parser.Feed, args)
	parser.Close()

	output = parser.Output()
	if output == "" {
		output = strings.TrimSpace(string(raw))
	}
	return output, parser.ConversationID(), err
}

func runPlainStreamingCommand(ctx context.Context, workdir, cmdName string, args []string, onChunk func(string)) (string, error) {
	var out bytes.Buffer
	_, err := runGuardedStreaming(ctx, workdir, cmdName, func(chunk []byte) {
		if len(chunk) == 0 {
			return
		}
		_, _ = out.Write(chunk)
		if onChunk != nil {
			onChunk(string(chunk))
		}
	}, args)
	return strings.TrimSpace(out.String()), err
}

func extractConversationID(payload map[string]any) string {
	for _, key := range []string{"session_id", "sessionId", "conversation_id", "conversationId"} {
		if value := deepFindString(payload, key, 5); value != "" {
			return value
		}
	}
	return ""
}

func deepFindString(value any, targetKey string, depth int) string {
	if depth < 0 {
		return ""
	}
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if strings.EqualFold(key, targetKey) {
				if s, ok := child.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		for _, child := range node {
			if s := deepFindString(child, targetKey, depth-1); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range node {
			if s := deepFindString(child, targetKey, depth-1); s != "" {
				return s
			}
		}
	}
	return ""
}

func extractStreamText(payload map[string]any) string {
	eventType := strings.ToLower(strings.TrimSpace(firstString(payload, "type", "event_type", "event")))
	role := strings.ToLower(strings.TrimSpace(firstString(payload, "role", "speaker")))
	if role == "user" || strings.Contains(eventType, "user") {
		return ""
	}

	for _, key := range []string{"output_text", "text", "delta", "content", "message"} {
		if text := flattenText(payload[key], 6); text != "" {
			return text
		}
	}
	for _, key := range []string{"result", "data", "payload"} {
		if text := flattenText(payload[key], 6); text != "" {
			return text
		}
	}
	return ""
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func flattenText(value any, depth int) string {
	if depth < 0 {
		return ""
	}
	switch node := value.(type) {
	case string:
		return node
	case []any:
		var b strings.Builder
		for _, item := range node {
			if text := flattenText(item, depth-1); text != "" {
				b.WriteString(text)
			}
		}
		return b.String()
	case map[string]any:
		for _, key := range []string{"output_text", "text", "delta", "content", "message", "value"} {
			if text := flattenText(node[key], depth-1); text != "" {
				return text
			}
		}
		if text := flattenText(node["parts"], depth-1); text != "" {
			return text
		}
		for _, child := range node {
			if text := flattenText(child, depth-1); text != "" {
				return text
			}
		}
	}
	return ""
}

func looksLikeUnsupportedFlag(output string) bool {
	value := strings.ToLower(strings.TrimSpace(output))
	if value == "" {
		return false
	}
	return strings.Contains(value, "unknown flag") || strings.Contains(value, "flag provided but not defined")
}
