package api

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func TestInferErrorCode_AllPatterns(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"required", "field is required", ErrCodeRequired},
		{"enum", "must be one of: a, b, c", ErrCodeInvalidEnum},
		{"duration", "invalid duration format", ErrCodeInvalidDuration},
		{"range between", "must be between 0 and 100", ErrCodeInvalidRange},
		{"range >=", "must be >= 0", ErrCodeInvalidRange},
		{"range positive", "must be positive", ErrCodeInvalidRange},
		{"invalid path", "invalid file path", ErrCodeInvalidPath},
		{"dependency", "field X requires field Y", ErrCodeDependencyChain},
		{"mutual exclusion", "cannot be true when Z is enabled", ErrCodeMutualExclusion},
		{"agent not enabled", "agent must be enabled", ErrCodeAgentNotEnabled},
		{"unknown agent", "unknown agent specified", ErrCodeUnknownAgent},
		{"unknown phase", "unknown phase name", ErrCodeUnknownPhase},
		{"generic", "something went wrong", "VALIDATION_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferErrorCode("field", tt.message)
			if got != tt.want {
				t.Errorf("inferErrorCode(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestConvertValidationErrors_Comprehensive(t *testing.T) {
	errs := config.ValidationErrors{
		{Field: "timeout", Value: -1, Message: "must be positive"},
		{Field: "model", Value: "", Message: "field is required"},
	}

	response := convertValidationErrors(errs)

	if response.Message == "" {
		t.Error("expected non-empty message")
	}
	if len(response.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(response.Errors))
	}

	// First error
	if response.Errors[0].Field != "timeout" {
		t.Errorf("got field %q", response.Errors[0].Field)
	}
	if response.Errors[0].Code != ErrCodeInvalidRange {
		t.Errorf("got code %q, want %q", response.Errors[0].Code, ErrCodeInvalidRange)
	}

	// Second error
	if response.Errors[1].Code != ErrCodeRequired {
		t.Errorf("got code %q, want %q", response.Errors[1].Code, ErrCodeRequired)
	}
}

func TestValidateBlueprint_Nil(t *testing.T) {
	err := ValidateBlueprint(nil, config.AgentsConfig{})
	if err != nil {
		t.Errorf("nil blueprint should be valid, got %v", err)
	}
}

func TestValidateBlueprint_InvalidConsensusThreshold(t *testing.T) {
	bp := &BlueprintDTO{ConsensusThreshold: -0.1}
	err := ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for negative threshold")
	}
	if err.Field != "consensus_threshold" {
		t.Errorf("got field %q", err.Field)
	}

	bp.ConsensusThreshold = 1.5
	err = ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for threshold > 1")
	}
}

func TestValidateBlueprint_InvalidMaxRetries(t *testing.T) {
	bp := &BlueprintDTO{MaxRetries: -1}
	err := ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for negative retries")
	}

	bp.MaxRetries = 11
	err = ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for retries > 10")
	}
}

func TestValidateBlueprint_InvalidTimeout(t *testing.T) {
	bp := &BlueprintDTO{TimeoutSeconds: -5}
	err := ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestValidateBlueprint_InvalidExecutionMode(t *testing.T) {
	bp := &BlueprintDTO{ExecutionMode: "invalid_mode"}
	err := ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if err.Code != ErrCodeInvalidEnum {
		t.Errorf("got code %q", err.Code)
	}
}

func TestValidateBlueprint_ValidModes(t *testing.T) {
	for _, mode := range []string{"", "multi_agent", "interactive"} {
		bp := &BlueprintDTO{ExecutionMode: mode}
		err := ValidateBlueprint(bp, config.AgentsConfig{})
		if err != nil {
			t.Errorf("mode %q should be valid, got: %v", mode, err)
		}
	}
}

func TestValidateBlueprint_SingleAgent_MissingName(t *testing.T) {
	bp := &BlueprintDTO{ExecutionMode: "single_agent"}
	err := ValidateBlueprint(bp, config.AgentsConfig{})
	if err == nil {
		t.Fatal("expected error for missing agent name")
	}
	if err.Code != ErrCodeRequired {
		t.Errorf("got code %q", err.Code)
	}
}

func TestValidateBlueprint_SingleAgent_UnknownAgent(t *testing.T) {
	bp := &BlueprintDTO{
		ExecutionMode:   "single_agent",
		SingleAgentName: "nonexistent",
	}
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{Enabled: true},
	}
	err := ValidateBlueprint(bp, agents)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if err.Code != ErrCodeUnknownAgent {
		t.Errorf("got code %q", err.Code)
	}
}

func TestValidateBlueprint_SingleAgent_DisabledAgent(t *testing.T) {
	bp := &BlueprintDTO{
		ExecutionMode:   "single_agent",
		SingleAgentName: "claude",
	}
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{Enabled: false},
	}
	err := ValidateBlueprint(bp, agents)
	if err == nil {
		t.Fatal("expected error for disabled agent")
	}
	if err.Code != ErrCodeAgentNotEnabled {
		t.Errorf("got code %q", err.Code)
	}
}

func TestValidateBlueprint_SingleAgent_Valid(t *testing.T) {
	bp := &BlueprintDTO{
		ExecutionMode:   "single_agent",
		SingleAgentName: "claude",
	}
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{Enabled: true},
	}
	err := ValidateBlueprint(bp, agents)
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}
