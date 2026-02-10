package cli

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// --- truncateDataAny edge cases ---

func TestTruncateDataAny_Nil(t *testing.T) {
	if got := truncateDataAny(nil, 100); got != nil {
		t.Errorf("nil: got %v", got)
	}
}

func TestTruncateDataAny_LargeMap(t *testing.T) {
	largeMap := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}
	got := truncateDataAny(largeMap, 1000)
	if _, ok := got.(string); !ok {
		t.Errorf("large map should be serialized to string, got %T", got)
	}
}

func TestTruncateDataAny_OtherType(t *testing.T) {
	got := truncateDataAny(42, 100)
	if got != "42" {
		t.Errorf("int: got %v", got)
	}
}

// --- Parser AgentName methods ---

func TestClaudeStreamParser_AgentName(t *testing.T) {
	if (&ClaudeStreamParser{}).AgentName() != "claude" {
		t.Error("wrong agent name")
	}
}

func TestGeminiStreamParser_AgentName(t *testing.T) {
	if (&GeminiStreamParser{}).AgentName() != "gemini" {
		t.Error("wrong agent name")
	}
}

func TestCodexStreamParser_AgentName(t *testing.T) {
	if (&CodexStreamParser{}).AgentName() != "codex" {
		t.Error("wrong agent name")
	}
}

func TestCopilotLogParser_AgentName(t *testing.T) {
	if NewCopilotLogParser().AgentName() != "copilot" {
		t.Error("wrong agent name")
	}
}

// --- extractToolName ---

func TestExtractToolName_Patterns(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"tool_call", "tool_call: read_file", "read_file"},
		{"executing", "executing: write_file", "write_file"},
		{"running", "running: bash", "bash"},
		{"no match", "normal text", ""},
		{"function_call", "function_call: myFunc", "myFunc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolName(tt.line)
			if got != tt.want {
				t.Errorf("extractToolName(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// --- EventAggregator BufferChunk/FlushChunks ---

func TestEventAggregator_BufferChunk_FirstFlush(t *testing.T) {
	agg := NewEventAggregator()

	text, flush := agg.BufferChunk("claude", "hello ")
	if !flush {
		t.Error("first chunk should flush")
	}
	if text != "hello " {
		t.Errorf("got %q", text)
	}
}

func TestEventAggregator_BufferChunk_RateLimited(t *testing.T) {
	agg := NewEventAggregator()

	// First call flushes
	agg.BufferChunk("claude", "hello ")

	// Immediate second should buffer
	text, flush := agg.BufferChunk("claude", "world")
	if flush {
		t.Error("immediate second should buffer")
	}
	if text != "" {
		t.Errorf("expected empty, got %q", text)
	}
}

func TestEventAggregator_FlushChunks_Empty(t *testing.T) {
	agg := NewEventAggregator()
	if got := agg.FlushChunks("claude"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestEventAggregator_FlushChunks_WithContent(t *testing.T) {
	agg := NewEventAggregator()

	// Buffer some content
	agg.BufferChunk("claude", "first")
	// Set lastEvent in the past to allow buffering
	agg.lastEvent["claude:chunk"] = time.Now()
	agg.BufferChunk("claude", " second")

	got := agg.FlushChunks("claude")
	if got != " second" {
		t.Errorf("got %q", got)
	}

	// After flush, should be empty
	if got := agg.FlushChunks("claude"); got != "" {
		t.Errorf("expected empty after flush, got %q", got)
	}
}

// --- ShouldEmit for always-emit types ---

func TestEventAggregator_ShouldEmit_AlwaysEmit(t *testing.T) {
	agg := NewEventAggregator()

	for _, evType := range []core.AgentEventType{
		core.AgentEventCompleted,
		core.AgentEventError,
		core.AgentEventToolUse,
		core.AgentEventStarted,
	} {
		ev := core.NewAgentEvent(evType, "claude", "test")
		if !agg.ShouldEmit(ev) {
			t.Errorf("should always emit %s", evType)
		}
		// Call again immediately - should still emit
		if !agg.ShouldEmit(ev) {
			t.Errorf("should always emit %s even when repeated", evType)
		}
	}
}

// --- Streaming helpers ---

func TestGetStreamParser_AllRegistered(t *testing.T) {
	for _, name := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		if GetStreamParser(name) == nil {
			t.Errorf("expected parser for %q", name)
		}
	}
	if GetStreamParser("unknown") != nil {
		t.Error("expected nil for unknown parser")
	}
}

func TestGetStreamConfig_Known(t *testing.T) {
	cfg := GetStreamConfig("claude")
	if cfg.Method != StreamMethodJSONStdout {
		t.Errorf("claude method = %q", cfg.Method)
	}

	cfg = GetStreamConfig("copilot")
	if cfg.Method != StreamMethodLogFile {
		t.Errorf("copilot method = %q", cfg.Method)
	}

	cfg = GetStreamConfig("unknown")
	if cfg.Method != StreamMethodNone {
		t.Errorf("unknown method = %q", cfg.Method)
	}
}

// --- Registry helpers ---

func TestGetTokenDiscrepancyThreshold_Configured(t *testing.T) {
	if got := GetTokenDiscrepancyThreshold(0.5); got != 0.5 {
		t.Errorf("got %f, want 0.5", got)
	}
}

func TestGetTokenDiscrepancyThreshold_Default(t *testing.T) {
	if got := GetTokenDiscrepancyThreshold(0); got != DefaultTokenDiscrepancyThreshold {
		t.Errorf("got %f, want default %f", got, DefaultTokenDiscrepancyThreshold)
	}
	if got := GetTokenDiscrepancyThreshold(-1); got != DefaultTokenDiscrepancyThreshold {
		t.Errorf("got %f, want default", got)
	}
}

func TestIsPhaseInList_Cases(t *testing.T) {
	tests := []struct {
		name   string
		phases []string
		phase  string
		want   bool
	}{
		{"found", []string{"analyze", "execute"}, "execute", true},
		{"not found", []string{"analyze", "execute"}, "plan", false},
		{"empty list", []string{}, "execute", false},
		{"nil list", nil, "execute", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPhaseInList(tt.phases, tt.phase); got != tt.want {
				t.Errorf("isPhaseInList(%v, %q) = %v, want %v", tt.phases, tt.phase, got, tt.want)
			}
		})
	}
}

// --- Codex long command truncation ---

func TestCodexStreamParser_LongCommand(t *testing.T) {
	p := &CodexStreamParser{}
	longCmd := `{"type":"item.started","item":{"type":"command_execution","command":"` +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
		`"}}`
	events := p.ParseLine(longCmd)
	if len(events) != 1 {
		t.Fatalf("got %d events", len(events))
	}
}

// --- Gemini thinking empty ---

func TestGeminiStreamParser_ThinkingEmpty(t *testing.T) {
	p := &GeminiStreamParser{}
	events := p.ParseLine(`{"type":"thinking"}`)
	if len(events) != 1 {
		t.Fatalf("got %d events", len(events))
	}
	if events[0].Type != core.AgentEventThinking {
		t.Errorf("type = %q", events[0].Type)
	}
}

// --- Codex item.completed with exit_code and missing fields ---

func TestCodexStreamParser_ItemCompleted_CommandExecution(t *testing.T) {
	p := &CodexStreamParser{}
	events := p.ParseLine(`{"type":"item.completed","item":{"type":"command_execution","command":"ls","exit_code":0}}`)
	if len(events) != 1 {
		t.Fatalf("got %d events", len(events))
	}
	if events[0].Type != core.AgentEventProgress {
		t.Errorf("type = %q", events[0].Type)
	}
}

func TestCodexStreamParser_ItemCompleted_AgentMessage(t *testing.T) {
	p := &CodexStreamParser{}
	events := p.ParseLine(`{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`)
	if len(events) != 1 {
		t.Fatalf("got %d events", len(events))
	}
}

func TestCodexStreamParser_TurnCompleted_SuspiciousTokens(t *testing.T) {
	p := &CodexStreamParser{}
	events := p.ParseLine(`{"type":"turn.completed","usage":{"input_tokens":2000000,"output_tokens":50}}`)
	// Should emit a debug event + completed event
	if len(events) < 2 {
		t.Fatalf("expected >= 2 events for suspicious tokens, got %d", len(events))
	}
}
