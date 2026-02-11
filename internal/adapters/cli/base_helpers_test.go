package cli

import (
	"path/filepath"
	"testing"
)

// --- pathWithin ---

func TestPathWithin_Inside(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "subdir", "file.txt")
	if !pathWithin(tmpDir, target) {
		t.Errorf("pathWithin(%q, %q) = false, want true", tmpDir, target)
	}
}

func TestPathWithin_Same(t *testing.T) {
	tmpDir := t.TempDir()
	if !pathWithin(tmpDir, tmpDir) {
		t.Error("same path should be within")
	}
}

func TestPathWithin_Escape(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "..", "..", "etc", "passwd")
	if pathWithin(tmpDir, target) {
		t.Error("path traversal should not be within")
	}
}

func TestPathWithin_Parent(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "..")
	if pathWithin(tmpDir, target) {
		t.Error("parent should not be within")
	}
}

// --- extractErrorFromOutput ---

func TestExtractErrorFromOutput_JSONErrorString(t *testing.T) {
	output := `{"error":"something went wrong"}`
	got := extractErrorFromOutput(output)
	if got != "something went wrong" {
		t.Errorf("got %q, want %q", got, "something went wrong")
	}
}

func TestExtractErrorFromOutput_JSONErrorObject(t *testing.T) {
	output := `{"error":{"message":"detailed error"}}`
	got := extractErrorFromOutput(output)
	if got != "detailed error" {
		t.Errorf("got %q, want %q", got, "detailed error")
	}
}

func TestExtractErrorFromOutput_ClaudeResultError(t *testing.T) {
	output := `{"type":"result","subtype":"error","error":"claude error msg"}`
	got := extractErrorFromOutput(output)
	if got != "claude error msg" {
		t.Errorf("got %q, want %q", got, "claude error msg")
	}
}

func TestExtractErrorFromOutput_ClaudeTypeError(t *testing.T) {
	output := `{"type":"error","error":"type error msg"}`
	got := extractErrorFromOutput(output)
	if got != "type error msg" {
		t.Errorf("got %q, want %q", got, "type error msg")
	}
}

func TestExtractErrorFromOutput_PlainText(t *testing.T) {
	output := "some output\nERROR: something failed"
	got := extractErrorFromOutput(output)
	if got != "ERROR: something failed" {
		t.Errorf("got %q, want %q", got, "ERROR: something failed")
	}
}

func TestExtractErrorFromOutput_Empty(t *testing.T) {
	got := extractErrorFromOutput("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractErrorFromOutput_LongLine(t *testing.T) {
	long := ""
	for i := 0; i < 250; i++ {
		long += "x"
	}
	got := extractErrorFromOutput(long)
	if len(got) > 204 { // 200 + "..."
		t.Errorf("should truncate long lines, got length %d", len(got))
	}
}

func TestExtractErrorFromOutput_MultilineJSON(t *testing.T) {
	output := "Starting...\nprogress line\n{\"error\":\"found at end\"}"
	got := extractErrorFromOutput(output)
	if got != "found at end" {
		t.Errorf("got %q, want %q", got, "found at end")
	}
}

// --- containsAny ---

func TestContainsAny_Found(t *testing.T) {
	if !containsAny("hello world", []string{"moon", "world"}) {
		t.Error("should find 'world'")
	}
}

func TestContainsAny_NotFound(t *testing.T) {
	if containsAny("hello world", []string{"moon", "sun"}) {
		t.Error("should not find any")
	}
}

func TestContainsAny_Empty(t *testing.T) {
	if containsAny("hello", []string{}) {
		t.Error("empty list should not match")
	}
	if containsAny("", []string{"a"}) {
		t.Error("empty string should not match")
	}
}

// --- AgentConfig.IsEnabledForPhase ---

func TestAgentConfig_IsEnabledForPhase(t *testing.T) {
	cfg := AgentConfig{
		Phases: map[string]bool{
			"analyze": true,
			"plan":    false,
		},
	}

	if !cfg.IsEnabledForPhase("analyze") {
		t.Error("analyze should be enabled")
	}
	if cfg.IsEnabledForPhase("plan") {
		t.Error("plan should be disabled")
	}
	if cfg.IsEnabledForPhase("execute") {
		t.Error("unspecified phase should be disabled")
	}
}

func TestAgentConfig_IsEnabledForPhase_NilMap(t *testing.T) {
	cfg := AgentConfig{}
	if cfg.IsEnabledForPhase("analyze") {
		t.Error("nil phases map should disable all phases")
	}
}

// --- AgentConfig.GetReasoningEffort ---

func TestAgentConfig_GetReasoningEffort_Default(t *testing.T) {
	cfg := AgentConfig{ReasoningEffort: "high"}
	if got := cfg.GetReasoningEffort("analyze"); got != "high" {
		t.Errorf("got %q, want %q", got, "high")
	}
}

func TestAgentConfig_GetReasoningEffort_PhaseOverride(t *testing.T) {
	cfg := AgentConfig{
		ReasoningEffort: "high",
		ReasoningEffortPhases: map[string]string{
			"analyze": "low",
		},
	}
	if got := cfg.GetReasoningEffort("analyze"); got != "low" {
		t.Errorf("got %q, want %q", got, "low")
	}
	// Falls back to default for non-overridden phases
	if got := cfg.GetReasoningEffort("plan"); got != "high" {
		t.Errorf("got %q, want %q", got, "high")
	}
}

func TestAgentConfig_GetReasoningEffort_Empty(t *testing.T) {
	cfg := AgentConfig{}
	if got := cfg.GetReasoningEffort("analyze"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
