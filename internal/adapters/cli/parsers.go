package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// Claude Stream Parser
// =============================================================================

// ClaudeStreamParser parses Claude's stream-json output format.
type ClaudeStreamParser struct{}

// claudeStreamEvent represents a single event in Claude's stream-json output.
type claudeStreamEvent struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

// ParseLine parses a single line of Claude stream-json output.
func (p *ClaudeStreamParser) ParseLine(line string) []core.AgentEvent {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil
	}

	var event claudeStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil
	}

	var events []core.AgentEvent

	switch event.Type {
	case "message_start":
		events = append(events, core.NewAgentEvent(
			core.AgentEventStarted,
			"claude",
			"Started processing",
		).WithData(map[string]any{
			"model": event.Message.Model,
		}))

	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventToolUse,
				"claude",
				"Using tool: "+event.ContentBlock.Name,
			).WithData(map[string]any{
				"tool": event.ContentBlock.Name,
			}))
		}

	case "content_block_delta":
		if event.Delta.Type == "thinking_delta" || event.Delta.Type == "thinking" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventThinking,
				"claude",
				"Thinking...",
			))
		} else if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventChunk,
				"claude",
				event.Delta.Text,
			))
		}

	case "message_stop", "message_delta":
		if event.Message.Usage.OutputTokens > 0 {
			events = append(events, core.NewAgentEvent(
				core.AgentEventCompleted,
				"claude",
				"Completed",
			).WithData(map[string]any{
				"tokens_in":  event.Message.Usage.InputTokens,
				"tokens_out": event.Message.Usage.OutputTokens,
			}))
		}

	case "error":
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"claude",
			"Error occurred",
		))
	}

	return events
}

// AgentName returns the agent name.
func (p *ClaudeStreamParser) AgentName() string {
	return "claude"
}

// =============================================================================
// Gemini Stream Parser
// =============================================================================

// GeminiStreamParser parses Gemini's stream-json output format.
type GeminiStreamParser struct{}

// geminiStreamEvent represents a single event in Gemini's stream-json output.
type geminiStreamEvent struct {
	Type   string `json:"type"`
	Action struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Tool   string `json:"tool"`
	} `json:"action"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error string `json:"error"`
}

// ParseLine parses a single line of Gemini stream-json output.
func (p *GeminiStreamParser) ParseLine(line string) []core.AgentEvent {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil
	}

	var event geminiStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil
	}

	var events []core.AgentEvent

	switch event.Type {
	case "start", "started":
		events = append(events, core.NewAgentEvent(
			core.AgentEventStarted,
			"gemini",
			"Started processing",
		))

	case "action", "tool_use":
		if event.Action.Tool != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventToolUse,
				"gemini",
				"Using tool: "+event.Action.Tool,
			).WithData(map[string]any{
				"tool":   event.Action.Tool,
				"status": event.Action.Status,
			}))
		}

	case "thinking":
		events = append(events, core.NewAgentEvent(
			core.AgentEventThinking,
			"gemini",
			"Thinking...",
		))

	case "content", "text":
		if event.Content.Text != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventChunk,
				"gemini",
				event.Content.Text,
			))
		}

	case "done", "complete", "finished":
		events = append(events, core.NewAgentEvent(
			core.AgentEventCompleted,
			"gemini",
			"Completed",
		).WithData(map[string]any{
			"tokens_in":  event.Usage.InputTokens,
			"tokens_out": event.Usage.OutputTokens,
		}))

	case "error":
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"gemini",
			event.Error,
		))
	}

	return events
}

// AgentName returns the agent name.
func (p *GeminiStreamParser) AgentName() string {
	return "gemini"
}

// =============================================================================
// Codex Stream Parser
// =============================================================================

// CodexStreamParser parses Codex's --json output format.
type CodexStreamParser struct{}

// codexStreamEvent represents a single event in Codex's JSON output.
type codexStreamEvent struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Message string `json:"message"`
	Tool    struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"tool"`
	Content string `json:"content"`
	Usage   struct {
		Input  int `json:"input"`
		Output int `json:"output"`
	} `json:"usage"`
	Error string `json:"error"`
}

// ParseLine parses a single line of Codex JSON output.
func (p *CodexStreamParser) ParseLine(line string) []core.AgentEvent {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil
	}

	var event codexStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil
	}

	var events []core.AgentEvent

	// Codex uses "event" or "type" field
	eventType := event.Type
	if eventType == "" {
		eventType = event.Event
	}

	switch eventType {
	case "start", "session_start":
		events = append(events, core.NewAgentEvent(
			core.AgentEventStarted,
			"codex",
			"Started processing",
		))

	case "tool_call", "function_call":
		if event.Tool.Name != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventToolUse,
				"codex",
				"Using tool: "+event.Tool.Name,
			).WithData(map[string]any{
				"tool": event.Tool.Name,
				"args": event.Tool.Args,
			}))
		}

	case "thinking", "reasoning":
		events = append(events, core.NewAgentEvent(
			core.AgentEventThinking,
			"codex",
			"Thinking...",
		))

	case "content", "message", "text":
		content := event.Content
		if content == "" {
			content = event.Message
		}
		if content != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventChunk,
				"codex",
				content,
			))
		}

	case "done", "complete", "session_end":
		events = append(events, core.NewAgentEvent(
			core.AgentEventCompleted,
			"codex",
			"Completed",
		).WithData(map[string]any{
			"tokens_in":  event.Usage.Input,
			"tokens_out": event.Usage.Output,
		}))

	case "error":
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"codex",
			event.Error,
		))
	}

	return events
}

// AgentName returns the agent name.
func (p *CodexStreamParser) AgentName() string {
	return "codex"
}

// =============================================================================
// Copilot Log Parser
// =============================================================================

// CopilotLogParser parses Copilot's log file output.
type CopilotLogParser struct {
	// Regex patterns for parsing log lines
	requestPattern  *regexp.Regexp
	responsePattern *regexp.Regexp
	toolPattern     *regexp.Regexp
	errorPattern    *regexp.Regexp
}

// NewCopilotLogParser creates a new Copilot log parser.
func NewCopilotLogParser() *CopilotLogParser {
	return &CopilotLogParser{
		requestPattern:  regexp.MustCompile(`(?i)sending\s+request|making\s+api\s+call|request\s+to`),
		responsePattern: regexp.MustCompile(`(?i)response\s*\(Request-ID|received\s+response|api\s+response`),
		toolPattern:     regexp.MustCompile(`(?i)tool[_\s]?call|function[_\s]?call|executing|running`),
		errorPattern:    regexp.MustCompile(`(?i)error|failed|exception|fatal`),
	}
}

// ParseLine parses a single line from Copilot's log file.
func (p *CopilotLogParser) ParseLine(line string) []core.AgentEvent {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var events []core.AgentEvent

	// Check for request start
	if p.requestPattern.MatchString(line) {
		events = append(events, core.NewAgentEvent(
			core.AgentEventProgress,
			"copilot",
			"Sending request to API...",
		))
	}

	// Check for response received
	if p.responsePattern.MatchString(line) {
		events = append(events, core.NewAgentEvent(
			core.AgentEventProgress,
			"copilot",
			"Received API response",
		))
	}

	// Check for tool usage
	if p.toolPattern.MatchString(line) {
		// Try to extract tool name
		toolName := extractToolName(line)
		msg := "Executing action"
		if toolName != "" {
			msg = "Using tool: " + toolName
		}
		events = append(events, core.NewAgentEvent(
			core.AgentEventToolUse,
			"copilot",
			msg,
		).WithData(map[string]any{
			"tool": toolName,
		}))
	}

	// Check for errors
	if p.errorPattern.MatchString(line) {
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"copilot",
			line,
		))
	}

	return events
}

// AgentName returns the agent name.
func (p *CopilotLogParser) AgentName() string {
	return "copilot"
}

// extractToolName tries to extract a tool name from a log line.
func extractToolName(line string) string {
	// Common patterns for tool names in logs
	patterns := []string{
		`tool[_\s]?call[:\s]+["']?(\w+)["']?`,
		`function[_\s]?call[:\s]+["']?(\w+)["']?`,
		`executing[:\s]+["']?(\w+)["']?`,
		`running[:\s]+["']?(\w+)["']?`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if matches := re.FindStringSubmatch(line); len(matches) >= 2 {
			return matches[1]
		}
	}

	return ""
}

// =============================================================================
// Parser Registration
// =============================================================================

func init() {
	// Register all parsers at package initialization
	RegisterStreamParser("claude", &ClaudeStreamParser{})
	RegisterStreamParser("gemini", &GeminiStreamParser{})
	RegisterStreamParser("codex", &CodexStreamParser{})
	RegisterStreamParser("copilot", NewCopilotLogParser())
}

// =============================================================================
// Event Aggregator (for deduplication and rate limiting)
// =============================================================================

// EventAggregator helps deduplicate and rate-limit events.
type EventAggregator struct {
	lastEvent     map[string]time.Time
	minInterval   time.Duration
	chunkBuffer   map[string]*strings.Builder
	chunkInterval time.Duration
}

// NewEventAggregator creates a new event aggregator.
func NewEventAggregator() *EventAggregator {
	return &EventAggregator{
		lastEvent:     make(map[string]time.Time),
		minInterval:   100 * time.Millisecond, // Don't emit same event type more than 10x/sec
		chunkBuffer:   make(map[string]*strings.Builder),
		chunkInterval: 200 * time.Millisecond, // Buffer chunks for 200ms
	}
}

// ShouldEmit returns true if the event should be emitted (not too frequent).
func (a *EventAggregator) ShouldEmit(event core.AgentEvent) bool {
	key := string(event.Type) + ":" + event.Agent

	// Always emit completed, error, and tool_use events
	switch event.Type {
	case core.AgentEventCompleted, core.AgentEventError, core.AgentEventToolUse, core.AgentEventStarted:
		a.lastEvent[key] = time.Now()
		return true
	}

	// Rate limit other events
	if last, ok := a.lastEvent[key]; ok {
		if time.Since(last) < a.minInterval {
			return false
		}
	}

	a.lastEvent[key] = time.Now()
	return true
}

// BufferChunk adds a text chunk to the buffer and returns true if it's time to flush.
func (a *EventAggregator) BufferChunk(agent, text string) (string, bool) {
	key := agent + ":chunk"

	// Initialize buffer if needed
	if a.chunkBuffer[agent] == nil {
		a.chunkBuffer[agent] = &strings.Builder{}
	}
	a.chunkBuffer[agent].WriteString(text)

	last, ok := a.lastEvent[key]
	if !ok || time.Since(last) >= a.chunkInterval {
		buffered := a.chunkBuffer[agent].String()
		a.chunkBuffer[agent].Reset()
		a.lastEvent[key] = time.Now()
		return buffered, true
	}

	return "", false
}

// FlushChunks returns any remaining buffered chunks for an agent.
func (a *EventAggregator) FlushChunks(agent string) string {
	if a.chunkBuffer[agent] == nil {
		return ""
	}
	buffered := a.chunkBuffer[agent].String()
	a.chunkBuffer[agent].Reset()
	return buffered
}
