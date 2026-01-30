package config

import "testing"

func TestExecutionMode_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		mode     ExecutionMode
		expected bool
	}{
		{"multi_agent is valid", ExecutionModeMultiAgent, true},
		{"single_agent is valid", ExecutionModeSingleAgent, true},
		{"empty string is valid", ExecutionMode(""), true},
		{"unknown mode is invalid", ExecutionMode("hybrid"), false},
		{"typo is invalid", ExecutionMode("single_agnet"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.expected {
				t.Errorf("ExecutionMode(%q).IsValid() = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestExecutionMode_IsSingleAgent(t *testing.T) {
	tests := []struct {
		name     string
		mode     ExecutionMode
		expected bool
	}{
		{"single_agent returns true", ExecutionModeSingleAgent, true},
		{"multi_agent returns false", ExecutionModeMultiAgent, false},
		{"empty returns false", ExecutionMode(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.IsSingleAgent(); got != tt.expected {
				t.Errorf("ExecutionMode(%q).IsSingleAgent() = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestDefaultExecutionMode(t *testing.T) {
	if got := DefaultExecutionMode(); got != ExecutionModeMultiAgent {
		t.Errorf("DefaultExecutionMode() = %v, want %v", got, ExecutionModeMultiAgent)
	}
}
