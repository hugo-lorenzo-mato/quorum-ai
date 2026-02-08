package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// truncateDataValue truncates a string to maxLen, appending "...[truncated]" if needed.
func truncateDataValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// truncateDataAny handles any-typed values (e.g. content.Input): if small leaves intact,
// if large serializes to JSON and truncates.
func truncateDataAny(v any, maxLen int) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return truncateDataValue(val, maxLen)
	case map[string]any:
		// Small maps: keep as-is for structured display
		if len(val) <= 3 {
			return val
		}
		// Larger maps: serialize and truncate
		b, err := json.Marshal(val)
		if err != nil {
			return truncateDataValue(fmt.Sprintf("%v", val), maxLen)
		}
		return truncateDataValue(string(b), maxLen)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return truncateDataValue(fmt.Sprintf("%v", val), maxLen)
		}
		return truncateDataValue(string(b), maxLen)
	}
}

// =============================================================================
// Claude Stream Parser
// =============================================================================

// ClaudeStreamParser parses Claude Code CLI's stream-json output format.
// Real format from `claude --print --output-format stream-json`:
//
//	{"type":"system","subtype":"init","session_id":"...","tools":["Bash","Glob",...]}
//	{"type":"assistant","message":{"content":[{"type":"tool_use","id":"...","name":"Bash","input":{...}}]}}
//	{"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}
//	{"type":"result","subtype":"success","result":"...","session_id":"..."}
type ClaudeStreamParser struct{}

// claudeStreamEvent represents a single event in Claude Code's stream-json output.
type claudeStreamEvent struct {
	Type    string         `json:"type"`
	Subtype string         `json:"subtype"`
	Message *claudeMessage `json:"message,omitempty"`
	Result  string         `json:"result,omitempty"`
	Error   string         `json:"error,omitempty"`
	Tools   []string       `json:"tools,omitempty"`
}

type claudeMessage struct {
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`  // for tool_use
	Text  string `json:"text,omitempty"`  // for text
	Input any    `json:"input,omitempty"` // for tool_use
}

// ParseLine parses a single line of Claude Code stream-json output.
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
	case "system":
		if event.Subtype == "init" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventStarted,
				"claude",
				"Initialized",
			).WithData(map[string]any{
				"tools": event.Tools,
			}))
		}

	case "assistant":
		if event.Message != nil {
			for _, content := range event.Message.Content {
				switch content.Type {
				case "tool_use":
					data := map[string]any{
						"tool": content.Name,
					}
					if content.Input != nil {
						data["args"] = truncateDataAny(content.Input, 500)
					}
					events = append(events, core.NewAgentEvent(
						core.AgentEventToolUse,
						"claude",
						"Using tool: "+content.Name,
					).WithData(data))
				case "thinking":
					thinkData := map[string]any{}
					if content.Text != "" {
						thinkData["thinking_text"] = truncateDataValue(content.Text, 200)
					}
					events = append(events, core.NewAgentEvent(
						core.AgentEventThinking,
						"claude",
						"Thinking...",
					).WithData(thinkData))
				case "text":
					if content.Text != "" {
						events = append(events, core.NewAgentEvent(
							core.AgentEventChunk,
							"claude",
							content.Text,
						))
					}
				}
			}
		}

	case "result":
		if event.Subtype == "success" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventCompleted,
				"claude",
				"Completed",
			))
		} else if event.Subtype == "error" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventError,
				"claude",
				event.Error,
			))
		}

	case "error":
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"claude",
			event.Error,
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

// GeminiStreamParser parses Gemini CLI's stream-json output format.
// Real format from `gemini --output-format stream-json`:
//
//	{"type":"init","model":"gemini-2.5-flash"}
//	{"type":"tool_use","tool_name":"read_file","args":{"path":"..."}}
//	{"type":"tool_result","tool_name":"read_file","result":"..."}
//	{"type":"text","text":"..."}
//	{"type":"result","response":"..."}
type GeminiStreamParser struct{}

// geminiStreamEvent represents a single event in Gemini's stream-json output.
type geminiStreamEvent struct {
	Type       string `json:"type"`
	Model      string `json:"model,omitempty"`     // for init
	ToolName   string `json:"tool_name,omitempty"` // for tool_use, tool_result
	Args       any    `json:"args,omitempty"`      // for tool_use
	ToolResult string `json:"result,omitempty"`    // for tool_result
	Text       string `json:"text,omitempty"`      // for text
	Response   string `json:"response,omitempty"`  // for result
	Error      string `json:"error,omitempty"`     // for error
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
	case "init":
		events = append(events, core.NewAgentEvent(
			core.AgentEventStarted,
			"gemini",
			"Initialized",
		).WithData(map[string]any{
			"model": event.Model,
		}))

	case "tool_use":
		events = append(events, core.NewAgentEvent(
			core.AgentEventToolUse,
			"gemini",
			"Using tool: "+event.ToolName,
		).WithData(map[string]any{
			"tool": event.ToolName,
			"args": event.Args,
		}))

	case "tool_result":
		resultData := map[string]any{
			"tool": event.ToolName,
		}
		if event.ToolResult != "" {
			resultData["result"] = truncateDataValue(event.ToolResult, 500)
		}
		events = append(events, core.NewAgentEvent(
			core.AgentEventProgress,
			"gemini",
			"Tool completed: "+event.ToolName,
		).WithData(resultData))

	case "thinking":
		gemThinkData := map[string]any{}
		if event.Text != "" {
			gemThinkData["thinking_text"] = truncateDataValue(event.Text, 200)
		}
		events = append(events, core.NewAgentEvent(
			core.AgentEventThinking,
			"gemini",
			"Thinking...",
		).WithData(gemThinkData))

	case "text":
		if event.Text != "" {
			events = append(events, core.NewAgentEvent(
				core.AgentEventChunk,
				"gemini",
				event.Text,
			))
		}

	case "result":
		events = append(events, core.NewAgentEvent(
			core.AgentEventCompleted,
			"gemini",
			"Completed",
		))

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

// CodexStreamParser parses OpenAI Codex CLI's --json output format.
// Real format from `codex exec --json`:
//
//	{"type":"thread.started","thread_id":"..."}
//	{"type":"turn.started"}
//	{"type":"item.completed","item":{"type":"reasoning","text":"..."}}
//	{"type":"item.started","item":{"type":"command_execution","command":"ls",...}}
//	{"type":"item.completed","item":{"type":"command_execution","command":"ls","exit_code":0,...}}
//	{"type":"item.completed","item":{"type":"agent_message","text":"..."}}
//	{"type":"turn.completed","usage":{"input_tokens":...,"output_tokens":...}}
type CodexStreamParser struct{}

// codexStreamEvent represents a single event in Codex's JSON output.
type codexStreamEvent struct {
	Type     string      `json:"type"`
	ThreadID string      `json:"thread_id,omitempty"`
	Item     *codexItem  `json:"item,omitempty"`
	Usage    *codexUsage `json:"usage,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type codexItem struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`              // "command_execution", "reasoning", "agent_message", "file_edit"
	Command  string `json:"command,omitempty"` // for command_execution
	Text     string `json:"text,omitempty"`    // for reasoning, agent_message
	Status   string `json:"status,omitempty"`  // "in_progress", "completed"
	ExitCode *int   `json:"exit_code,omitempty"`
}

type codexUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CachedInputTokens int `json:"cached_input_tokens,omitempty"`
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

	switch event.Type {
	case "thread.started":
		events = append(events, core.NewAgentEvent(
			core.AgentEventStarted,
			"codex",
			"Session started",
		).WithData(map[string]any{
			"thread_id": event.ThreadID,
		}))

	case "turn.started":
		events = append(events, core.NewAgentEvent(
			core.AgentEventProgress,
			"codex",
			"Processing...",
		))

	case "item.started":
		if event.Item != nil {
			switch event.Item.Type {
			case "command_execution":
				// Extract command name (first word or the whole thing if short)
				cmdName := event.Item.Command
				if len(cmdName) > 40 {
					// Try to get just the command name
					parts := strings.Fields(cmdName)
					if len(parts) > 0 {
						cmdName = parts[len(parts)-1] // Get last part (actual command after shell -c)
						if len(cmdName) > 30 {
							cmdName = cmdName[:30] + "..."
						}
					}
				}
				events = append(events, core.NewAgentEvent(
					core.AgentEventToolUse,
					"codex",
					"Running: "+cmdName,
				).WithData(map[string]any{
					"command": event.Item.Command,
				}))
			case "file_edit":
				events = append(events, core.NewAgentEvent(
					core.AgentEventToolUse,
					"codex",
					"Editing file",
				))
			case "agent_message":
				events = append(events, core.NewAgentEvent(
					core.AgentEventProgress,
					"codex",
					"Generating response...",
				))
			}
		}

	case "item.completed":
		if event.Item != nil {
			switch event.Item.Type {
			case "reasoning":
				// Show a snippet of the reasoning
				text := event.Item.Text
				if len(text) > 50 {
					text = text[:50] + "..."
				}
				reasonData := map[string]any{}
				if event.Item.Text != "" {
					reasonData["reasoning_text"] = truncateDataValue(event.Item.Text, 200)
				}
				events = append(events, core.NewAgentEvent(
					core.AgentEventThinking,
					"codex",
					text,
				).WithData(reasonData))
			case "command_execution":
				cmdData := map[string]any{}
				if event.Item.Command != "" {
					cmdData["command"] = event.Item.Command
				}
				if event.Item.ExitCode != nil {
					cmdData["exit_code"] = *event.Item.ExitCode
				}
				events = append(events, core.NewAgentEvent(
					core.AgentEventProgress,
					"codex",
					"Command completed",
				).WithData(cmdData))
			case "agent_message":
				msgData := map[string]any{}
				if event.Item.Text != "" {
					msgData["text"] = truncateDataValue(event.Item.Text, 500)
				}
				events = append(events, core.NewAgentEvent(
					core.AgentEventProgress,
					"codex",
					"Response complete",
				).WithData(msgData))
			}
		}

	case "turn.completed":
		data := map[string]any{}
		if event.Usage != nil {
			data["tokens_in"] = event.Usage.InputTokens
			data["tokens_out"] = event.Usage.OutputTokens

			// Debug: log suspicious values from stream
			const maxReasonableTokens = 1_000_000
			if event.Usage.InputTokens > maxReasonableTokens || event.Usage.OutputTokens > maxReasonableTokens {
				events = append(events, core.NewAgentEvent(
					core.AgentEventProgress,
					"codex",
					"[DEBUG] Stream: suspicious token values",
				).WithData(map[string]any{
					"tokens_in":  event.Usage.InputTokens,
					"tokens_out": event.Usage.OutputTokens,
					"source":     "stream_parser",
				}))
			}
		}
		events = append(events, core.NewAgentEvent(
			core.AgentEventCompleted,
			"codex",
			"Completed",
		).WithData(data))

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
	RegisterStreamParser("opencode", &OpenCodeStreamParser{})
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
	if _, err := a.chunkBuffer[agent].WriteString(text); err != nil {
		return "", false
	}

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
