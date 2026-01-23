package cli

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.parser.AgentName(); got != tt.name {
				t.Errorf("AgentName() = %v, want %v", got, tt.name)
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
			name: "codex agent_message",
			line: `{"type":"item.completed","item":{"type":"agent_message","text":"Codex says hello"}}`,
			want: "Codex says hello",
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
