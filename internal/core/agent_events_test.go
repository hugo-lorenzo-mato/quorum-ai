package core

import (
	"testing"
	"time"
)

func TestNewAgentEvent(t *testing.T) {
	before := time.Now()
	ev := NewAgentEvent(AgentEventStarted, "claude", "starting task")
	after := time.Now()

	if ev.Type != AgentEventStarted {
		t.Errorf("Type = %q, want %q", ev.Type, AgentEventStarted)
	}
	if ev.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", ev.Agent, "claude")
	}
	if ev.Message != "starting task" {
		t.Errorf("Message = %q", ev.Message)
	}
	if ev.Timestamp.Before(before) || ev.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", ev.Timestamp, before, after)
	}
	if ev.Data != nil {
		t.Errorf("Data should be nil, got %v", ev.Data)
	}
}

func TestNewAgentEvent_AllTypes(t *testing.T) {
	types := []AgentEventType{
		AgentEventStarted,
		AgentEventToolUse,
		AgentEventThinking,
		AgentEventChunk,
		AgentEventProgress,
		AgentEventCompleted,
		AgentEventError,
	}

	for _, evType := range types {
		t.Run(string(evType), func(t *testing.T) {
			ev := NewAgentEvent(evType, "gemini", "test")
			if ev.Type != evType {
				t.Errorf("Type = %q, want %q", ev.Type, evType)
			}
		})
	}
}

func TestAgentEvent_WithData(t *testing.T) {
	ev := NewAgentEvent(AgentEventToolUse, "codex", "using tool")
	data := map[string]any{"tool": "read_file", "args": map[string]any{"path": "/tmp/test"}}

	ev2 := ev.WithData(data)

	// Original should not have data (value receiver returns copy)
	if ev.Data != nil {
		t.Error("original event should not have data after WithData")
	}

	// New event should have data
	if ev2.Data == nil {
		t.Fatal("WithData should set data")
	}
	if ev2.Data["tool"] != "read_file" {
		t.Errorf("Data[tool] = %v, want read_file", ev2.Data["tool"])
	}

	// Other fields preserved
	if ev2.Type != AgentEventToolUse {
		t.Errorf("Type = %q after WithData", ev2.Type)
	}
	if ev2.Agent != "codex" {
		t.Errorf("Agent = %q after WithData", ev2.Agent)
	}
}

func TestAgentEvent_WithData_Nil(t *testing.T) {
	ev := NewAgentEvent(AgentEventCompleted, "claude", "done")
	ev2 := ev.WithData(nil)
	if ev2.Data != nil {
		t.Error("WithData(nil) should set data to nil")
	}
}
