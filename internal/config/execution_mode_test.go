package config

import "testing"

func TestExecutionMode_Constants(t *testing.T) {
	t.Parallel()
	// Verify constant values are as expected
	if ExecutionModeMultiAgent != "multi_agent" {
		t.Errorf("ExecutionModeMultiAgent = %q, want %q", ExecutionModeMultiAgent, "multi_agent")
	}
	if ExecutionModeSingleAgent != "single_agent" {
		t.Errorf("ExecutionModeSingleAgent = %q, want %q", ExecutionModeSingleAgent, "single_agent")
	}
}

func TestExecutionMode_IsValid(t *testing.T) {
	t.Parallel()
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
		{"case sensitive - uppercase invalid", ExecutionMode("MULTI_AGENT"), false},
		{"whitespace is invalid", ExecutionMode("  "), false},
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
	t.Parallel()
	tests := []struct {
		name     string
		mode     ExecutionMode
		expected bool
	}{
		{"single_agent returns true", ExecutionModeSingleAgent, true},
		{"multi_agent returns false", ExecutionModeMultiAgent, false},
		{"empty returns false", ExecutionMode(""), false},
		{"unknown mode returns false", ExecutionMode("unknown"), false},
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
	t.Parallel()
	if got := DefaultExecutionMode(); got != ExecutionModeMultiAgent {
		t.Errorf("DefaultExecutionMode() = %v, want %v", got, ExecutionModeMultiAgent)
	}
}

func TestExecutionMode_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mode     ExecutionMode
		expected string
	}{
		{"multi_agent", ExecutionModeMultiAgent, "multi_agent"},
		{"single_agent", ExecutionModeSingleAgent, "single_agent"},
		{"empty string", ExecutionMode(""), ""},
		{"custom value", ExecutionMode("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("ExecutionMode(%q).String() = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}
