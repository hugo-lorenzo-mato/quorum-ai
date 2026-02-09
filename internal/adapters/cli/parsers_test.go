package cli

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestClaudeStreamParser_ParseLine(t *testing.T) {
	parser := &ClaudeStreamParser{}

	tests := []struct {
		name      string
		line      string
		wantType  core.AgentEventType
		wantAgent string
		wantTool  string
	}{
		{
			name:      "system init",
			line:      `{"type":"system","subtype":"init","session_id":"abc123","tools":["Bash","Glob","Grep"]}`,
			wantType:  core.AgentEventStarted,
			wantAgent: "claude",
		},
		{
			name:      "tool_use",
			line:      `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"Bash","input":{"command":"ls"}}]}}`,
			wantType:  core.AgentEventToolUse,
			wantAgent: "claude",
			wantTool:  "Bash",
		},
		{
			name:      "text content",
			line:      `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`,
			wantType:  core.AgentEventChunk,
			wantAgent: "claude",
		},
		{
			name:      "thinking",
			line:      `{"type":"assistant","message":{"content":[{"type":"thinking","text":"Let me think..."}]}}`,
			wantType:  core.AgentEventThinking,
			wantAgent: "claude",
		},
		{
			name:      "result success",
			line:      `{"type":"result","subtype":"success","result":"Done","session_id":"abc123"}`,
			wantType:  core.AgentEventCompleted,
			wantAgent: "claude",
		},
		{
			name:      "result error",
			line:      `{"type":"result","subtype":"error","error":"Something failed"}`,
			wantType:  core.AgentEventError,
			wantAgent: "claude",
		},
		{
			name:      "error type",
			line:      `{"type":"error","error":"Connection failed"}`,
			wantType:  core.AgentEventError,
			wantAgent: "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.ParseLine(tt.line)
			if len(events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := events[0]
			if event.Type != tt.wantType {
				t.Errorf("type = %v, want %v", event.Type, tt.wantType)
			}
			if event.Agent != tt.wantAgent {
				t.Errorf("agent = %v, want %v", event.Agent, tt.wantAgent)
			}
			if tt.wantTool != "" {
				tool, ok := event.Data["tool"].(string)
				if !ok || tool != tt.wantTool {
					t.Errorf("tool = %v, want %v", tool, tt.wantTool)
				}
			}
		})
	}
}

func TestClaudeStreamParser_ParseLine_EmptyLine(t *testing.T) {
	parser := &ClaudeStreamParser{}
	events := parser.ParseLine("")
	if events != nil {
		t.Errorf("expected nil for empty line, got %v", events)
	}
}

func TestClaudeStreamParser_ParseLine_InvalidJSON(t *testing.T) {
	parser := &ClaudeStreamParser{}
	events := parser.ParseLine("{invalid json")
	if events != nil {
		t.Errorf("expected nil for invalid json, got %v", events)
	}
}

func TestGeminiStreamParser_ParseLine(t *testing.T) {
	parser := &GeminiStreamParser{}

	tests := []struct {
		name      string
		line      string
		wantType  core.AgentEventType
		wantAgent string
		wantTool  string
	}{
		{
			name:      "init",
			line:      `{"type":"init","model":"gemini-2.5-flash"}`,
			wantType:  core.AgentEventStarted,
			wantAgent: "gemini",
		},
		{
			name:      "tool_use",
			line:      `{"type":"tool_use","tool_name":"read_file","args":{"path":"/test.go"}}`,
			wantType:  core.AgentEventToolUse,
			wantAgent: "gemini",
			wantTool:  "read_file",
		},
		{
			name:      "tool_result",
			line:      `{"type":"tool_result","tool_name":"read_file","result":"file contents"}`,
			wantType:  core.AgentEventProgress,
			wantAgent: "gemini",
		},
		{
			name:      "text",
			line:      `{"type":"text","text":"Here is the analysis..."}`,
			wantType:  core.AgentEventChunk,
			wantAgent: "gemini",
		},
		{
			name:      "thinking",
			line:      `{"type":"thinking","text":"Processing..."}`,
			wantType:  core.AgentEventThinking,
			wantAgent: "gemini",
		},
		{
			name:      "result",
			line:      `{"type":"result","response":"Analysis complete"}`,
			wantType:  core.AgentEventCompleted,
			wantAgent: "gemini",
		},
		{
			name:      "error",
			line:      `{"type":"error","error":"API limit reached"}`,
			wantType:  core.AgentEventError,
			wantAgent: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.ParseLine(tt.line)
			if len(events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := events[0]
			if event.Type != tt.wantType {
				t.Errorf("type = %v, want %v", event.Type, tt.wantType)
			}
			if event.Agent != tt.wantAgent {
				t.Errorf("agent = %v, want %v", event.Agent, tt.wantAgent)
			}
			if tt.wantTool != "" {
				tool, ok := event.Data["tool"].(string)
				if !ok || tool != tt.wantTool {
					t.Errorf("tool = %v, want %v", tool, tt.wantTool)
				}
			}
		})
	}
}

func TestCodexStreamParser_ParseLine(t *testing.T) {
	parser := &CodexStreamParser{}

	tests := []struct {
		name      string
		line      string
		wantType  core.AgentEventType
		wantAgent string
	}{
		{
			name:      "thread.started",
			line:      `{"type":"thread.started","thread_id":"019bde04-1651-7321-b935-210ff5945460"}`,
			wantType:  core.AgentEventStarted,
			wantAgent: "codex",
		},
		{
			name:      "turn.started",
			line:      `{"type":"turn.started"}`,
			wantType:  core.AgentEventProgress,
			wantAgent: "codex",
		},
		{
			name:      "item.started command_execution",
			line:      `{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"/usr/bin/zsh -lc ls","status":"in_progress"}}`,
			wantType:  core.AgentEventToolUse,
			wantAgent: "codex",
		},
		{
			name:      "item.started file_edit",
			line:      `{"type":"item.started","item":{"id":"item_2","type":"file_edit"}}`,
			wantType:  core.AgentEventToolUse,
			wantAgent: "codex",
		},
		{
			name:      "item.started agent_message",
			line:      `{"type":"item.started","item":{"id":"item_3","type":"agent_message"}}`,
			wantType:  core.AgentEventProgress,
			wantAgent: "codex",
		},
		{
			name:      "item.completed reasoning",
			line:      `{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"**Listing files in the directory**"}}`,
			wantType:  core.AgentEventThinking,
			wantAgent: "codex",
		},
		{
			name:      "item.completed command_execution",
			line:      `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls","exit_code":0,"status":"completed"}}`,
			wantType:  core.AgentEventProgress,
			wantAgent: "codex",
		},
		{
			name:      "item.completed agent_message",
			line:      `{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Here is my analysis..."}}`,
			wantType:  core.AgentEventProgress,
			wantAgent: "codex",
		},
		{
			name:      "turn.completed",
			line:      `{"type":"turn.completed","usage":{"input_tokens":69277,"cached_input_tokens":39168,"output_tokens":100}}`,
			wantType:  core.AgentEventCompleted,
			wantAgent: "codex",
		},
		{
			name:      "error",
			line:      `{"type":"error","error":"Rate limit exceeded"}`,
			wantType:  core.AgentEventError,
			wantAgent: "codex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.ParseLine(tt.line)
			if len(events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := events[0]
			if event.Type != tt.wantType {
				t.Errorf("type = %v, want %v", event.Type, tt.wantType)
			}
			if event.Agent != tt.wantAgent {
				t.Errorf("agent = %v, want %v", event.Agent, tt.wantAgent)
			}
		})
	}
}

func TestCopilotLogParser_ParseLine(t *testing.T) {
	parser := NewCopilotLogParser()

	tests := []struct {
		name     string
		line     string
		wantType core.AgentEventType
	}{
		{
			name:     "request",
			line:     "sending request to API...",
			wantType: core.AgentEventProgress,
		},
		{
			name:     "response",
			line:     "response (Request-ID: xyz) received",
			wantType: core.AgentEventProgress,
		},
		{
			name:     "tool_call",
			line:     "tool_call: read_file executed",
			wantType: core.AgentEventToolUse,
		},
		{
			name:     "error",
			line:     "error: connection failed",
			wantType: core.AgentEventError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.ParseLine(tt.line)
			if len(events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := events[0]
			if event.Type != tt.wantType {
				t.Errorf("type = %v, want %v", event.Type, tt.wantType)
			}
			if event.Agent != "copilot" {
				t.Errorf("agent = %v, want copilot", event.Agent)
			}
		})
	}
}

func TestEventAggregator_ShouldEmit(t *testing.T) {
	agg := NewEventAggregator()

	// Important events should always be emitted
	importantTypes := []core.AgentEventType{
		core.AgentEventCompleted,
		core.AgentEventError,
		core.AgentEventToolUse,
		core.AgentEventStarted,
	}

	for _, eventType := range importantTypes {
		event := core.NewAgentEvent(eventType, "test", "test message")
		if !agg.ShouldEmit(event) {
			t.Errorf("expected %v event to be emitted", eventType)
		}
	}
}

func TestEventAggregator_RateLimiting(t *testing.T) {
	agg := NewEventAggregator()

	// First event should be emitted
	event1 := core.NewAgentEvent(core.AgentEventProgress, "test", "message 1")
	if !agg.ShouldEmit(event1) {
		t.Error("expected first progress event to be emitted")
	}

	// Immediate second event should be rate limited
	event2 := core.NewAgentEvent(core.AgentEventProgress, "test", "message 2")
	if agg.ShouldEmit(event2) {
		t.Error("expected second progress event to be rate limited")
	}
}

func TestParserAgentNames(t *testing.T) {
	tests := []struct {
		parser StreamParser
		name   string
	}{
		{&ClaudeStreamParser{}, "claude"},
		{&GeminiStreamParser{}, "gemini"},
		{&CodexStreamParser{}, "codex"},
		{NewCopilotLogParser(), "copilot"},
		{&OpenCodeStreamParser{}, "opencode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.parser.AgentName(); got != tt.name {
				t.Errorf("AgentName() = %v, want %v", got, tt.name)
			}
		})
	}
}

func TestOpenCodeStreamParser_ParseLine(t *testing.T) {
	parser := &OpenCodeStreamParser{}

	tests := []struct {
		name      string
		line      string
		wantType  core.AgentEventType
		wantAgent string
	}{
		{
			name:      "thinking",
			line:      `{"thinking":"analyzing project"}`,
			wantType:  core.AgentEventThinking,
			wantAgent: "opencode",
		},
		{
			name:      "chunk",
			line:      `{"content":"hello"}`,
			wantType:  core.AgentEventChunk,
			wantAgent: "opencode",
		},
		{
			name:      "error",
			line:      `{"error":"failed"}`,
			wantType:  core.AgentEventError,
			wantAgent: "opencode",
		},
		{
			name:      "tool_use",
			line:      `{"tool":"read_file"}`,
			wantType:  core.AgentEventToolUse,
			wantAgent: "opencode",
		},
		{
			name:      "plain text thinking",
			line:      "Thinking about the solution...",
			wantType:  core.AgentEventThinking,
			wantAgent: "opencode",
		},
		{
			name:      "plain text chunk",
			line:      "regular output",
			wantType:  core.AgentEventChunk,
			wantAgent: "opencode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.ParseLine(tt.line)
			if len(events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := events[0]
			if event.Type != tt.wantType {
				t.Errorf("type = %v, want %v", event.Type, tt.wantType)
			}
			if event.Agent != tt.wantAgent {
				t.Errorf("agent = %v, want %v", event.Agent, tt.wantAgent)
			}
		})
	}
}

func TestExtractTextFromJSONLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "empty line",
			line: "",
			want: "",
		},
		{
			name: "non-json line",
			line: "just some text",
			want: "",
		},
		{
			name: "claude system init (no text)",
			line: `{"type":"system","subtype":"init","session_id":"abc","tools":["Bash"]}`,
			want: "",
		},
		{
			name: "claude result success",
			line: `{"type":"result","subtype":"success","result":"Hello, I can help you with that!","session_id":"abc"}`,
			want: "Hello, I can help you with that!",
		},
		{
			name: "claude assistant text",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"This is a response"}]}}`,
			want: "This is a response",
		},
		{
			name: "claude tool_use (no text)",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"Bash"}]}}`,
			want: "",
		},
		{
			name: "gemini text event",
			line: `{"type":"text","text":"Gemini response text"}`,
			want: "Gemini response text",
		},
		{
			name: "gemini result with response",
			line: `{"type":"result","subtype":"success","response":"Final gemini response"}`,
			want: "Final gemini response",
		},
		{
			name: "codex agent_message (with trailing newline)",
			line: `{"type":"item.completed","item":{"type":"agent_message","text":"Codex says hello"}}`,
			want: "Codex says hello\n",
		},
		{
			name: "codex reasoning (no text extracted)",
			line: `{"type":"item.completed","item":{"type":"reasoning","text":"Thinking about it"}}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromJSONLine(tt.line)
			if got != tt.want {
				t.Errorf("extractTextFromJSONLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeStreamParser_ThinkingWithData(t *testing.T) {
	parser := &ClaudeStreamParser{}
	events := parser.ParseLine(`{"type":"assistant","message":{"content":[{"type":"thinking","text":"Let me analyze the code structure..."}]}}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	if event.Type != core.AgentEventThinking {
		t.Errorf("type = %v, want %v", event.Type, core.AgentEventThinking)
	}
	text, ok := event.Data["thinking_text"].(string)
	if !ok {
		t.Fatal("expected thinking_text in data")
	}
	if text != "Let me analyze the code structure..." {
		t.Errorf("thinking_text = %q, want %q", text, "Let me analyze the code structure...")
	}
}

func TestCodexStreamParser_ReasoningWithData(t *testing.T) {
	parser := &CodexStreamParser{}
	events := parser.ParseLine(`{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"**Listing files in the directory**"}}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	if event.Type != core.AgentEventThinking {
		t.Errorf("type = %v, want %v", event.Type, core.AgentEventThinking)
	}
	text, ok := event.Data["reasoning_text"].(string)
	if !ok {
		t.Fatal("expected reasoning_text in data")
	}
	if text != "**Listing files in the directory**" {
		t.Errorf("reasoning_text = %q", text)
	}
}

func TestCodexStreamParser_CommandCompletedWithData(t *testing.T) {
	parser := &CodexStreamParser{}
	events := parser.ParseLine(`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls -la","exit_code":0,"status":"completed"}}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	if event.Type != core.AgentEventProgress {
		t.Errorf("type = %v, want %v", event.Type, core.AgentEventProgress)
	}
	cmd, ok := event.Data["command"].(string)
	if !ok || cmd != "ls -la" {
		t.Errorf("command = %v, want %v", cmd, "ls -la")
	}
	// exit_code is deserialized as float64 from JSON
	exitCode, ok := event.Data["exit_code"].(int)
	if !ok || exitCode != 0 {
		t.Errorf("exit_code = %v (type %T), want 0", event.Data["exit_code"], event.Data["exit_code"])
	}
}

func TestCodexStreamParser_CommandCompletedNonZeroExit(t *testing.T) {
	parser := &CodexStreamParser{}
	events := parser.ParseLine(`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"false","exit_code":1,"status":"completed"}}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	exitCode, ok := event.Data["exit_code"].(int)
	if !ok || exitCode != 1 {
		t.Errorf("exit_code = %v (type %T), want 1", event.Data["exit_code"], event.Data["exit_code"])
	}
}

func TestCodexStreamParser_AgentMessageWithData(t *testing.T) {
	parser := &CodexStreamParser{}
	events := parser.ParseLine(`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"Here is my analysis of the code..."}}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	text, ok := event.Data["text"].(string)
	if !ok {
		t.Fatal("expected text in data")
	}
	if text != "Here is my analysis of the code..." {
		t.Errorf("text = %q", text)
	}
}

func TestGeminiStreamParser_ThinkingWithData(t *testing.T) {
	parser := &GeminiStreamParser{}
	events := parser.ParseLine(`{"type":"thinking","text":"Processing the request..."}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	if event.Type != core.AgentEventThinking {
		t.Errorf("type = %v, want %v", event.Type, core.AgentEventThinking)
	}
	text, ok := event.Data["thinking_text"].(string)
	if !ok {
		t.Fatal("expected thinking_text in data")
	}
	if text != "Processing the request..." {
		t.Errorf("thinking_text = %q", text)
	}
}

func TestGeminiStreamParser_ToolResultWithData(t *testing.T) {
	parser := &GeminiStreamParser{}
	events := parser.ParseLine(`{"type":"tool_result","tool_name":"read_file","result":"package main\nfunc main() {}"}`)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	event := events[0]
	if event.Type != core.AgentEventProgress {
		t.Errorf("type = %v, want %v", event.Type, core.AgentEventProgress)
	}
	tool, ok := event.Data["tool"].(string)
	if !ok || tool != "read_file" {
		t.Errorf("tool = %v, want read_file", tool)
	}
	result, ok := event.Data["result"].(string)
	if !ok {
		t.Fatal("expected result in data")
	}
	if result != "package main\nfunc main() {}" {
		t.Errorf("result = %q", result)
	}
}

func TestTruncateDataValue(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 200, "hello"},
		{"exact length", "abc", 3, "abc"},
		{"truncated", "hello world this is long", 10, "hello worl...[truncated]"},
		{"empty", "", 200, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateDataValue(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateDataValue(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestTruncateDataAny(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		got := truncateDataAny(nil, 500)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("string value", func(t *testing.T) {
		got := truncateDataAny("hello", 500)
		if got != "hello" {
			t.Errorf("expected hello, got %v", got)
		}
	})

	t.Run("small map preserved", func(t *testing.T) {
		input := map[string]any{"command": "ls"}
		got := truncateDataAny(input, 500)
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", got)
		}
		if m["command"] != "ls" {
			t.Errorf("expected command=ls, got %v", m["command"])
		}
	})

	t.Run("large map serialized and truncated", func(t *testing.T) {
		input := map[string]any{
			"a": "value1",
			"b": "value2",
			"c": "value3",
			"d": "value4",
		}
		got := truncateDataAny(input, 20)
		s, ok := got.(string)
		if !ok {
			t.Fatalf("expected string, got %T", got)
		}
		if !strings.HasSuffix(s, "...[truncated]") {
			t.Errorf("expected truncated suffix, got %q", s)
		}
	})

	t.Run("long string truncated", func(t *testing.T) {
		input := strings.Repeat("x", 600)
		got := truncateDataAny(input, 500)
		s, ok := got.(string)
		if !ok {
			t.Fatalf("expected string, got %T", got)
		}
		if len(s) > 520 { // 500 + len("...[truncated]")
			t.Errorf("expected truncated string, got len=%d", len(s))
		}
	})
}
