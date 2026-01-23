package workflow

import (
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func TestAnalyzer_Run_AgentExecutionError(t *testing.T) {
	config := ModeratorConfig{
		Enabled: false, // Disable moderator for this test
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	registry := &mockAgentRegistry{}
	agent := &mockAgent{
		err: errors.New("execution failed"),
	}
	registry.Register("claude", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints:  []core.Checkpoint{},
			Metrics:      &core.StateMetrics{},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			Sandbox:      true,
			DefaultAgent: "claude",
		},
	}

	err = analyzer.Run(t.Context(), wctx)
	if err == nil {
		t.Fatal("expected error when agent execution fails")
	}
}

func TestAnalysisOutput_Fields(t *testing.T) {
	output := AnalysisOutput{
		AgentName:       "claude",
		RawOutput:       "Analysis result",
		Claims:          []string{"claim1", "claim2"},
		Risks:           []string{"risk1"},
		Recommendations: []string{"rec1", "rec2", "rec3"},
	}

	if output.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", output.AgentName, "claude")
	}
	if output.RawOutput != "Analysis result" {
		t.Errorf("RawOutput = %q, want %q", output.RawOutput, "Analysis result")
	}
	if len(output.Claims) != 2 {
		t.Errorf("len(Claims) = %d, want 2", len(output.Claims))
	}
	if len(output.Risks) != 1 {
		t.Errorf("len(Risks) = %d, want 1", len(output.Risks))
	}
	if len(output.Recommendations) != 3 {
		t.Errorf("len(Recommendations) = %d, want 3", len(output.Recommendations))
	}
}

func TestParseAnalysisOutput_PartialJSON(t *testing.T) {
	// Test JSON with only some fields
	result := &core.ExecuteResult{
		Output: `{"claims":["claim1"],"other_field":"ignored"}`,
	}
	output := parseAnalysisOutput("test-agent", result)

	if output.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want %q", output.AgentName, "test-agent")
	}
	if len(output.Claims) != 1 {
		t.Errorf("len(Claims) = %d, want 1", len(output.Claims))
	}
	if len(output.Risks) != 0 {
		t.Errorf("len(Risks) = %d, want 0", len(output.Risks))
	}
	if len(output.Recommendations) != 0 {
		t.Errorf("len(Recommendations) = %d, want 0", len(output.Recommendations))
	}
}

func TestParseAnalysisOutput_MalformedJSON(t *testing.T) {
	result := &core.ExecuteResult{
		Output: `{"claims": [invalid`,
	}
	output := parseAnalysisOutput("test-agent", result)

	// Should still set agent name and raw output
	if output.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want %q", output.AgentName, "test-agent")
	}
	if output.RawOutput != `{"claims": [invalid` {
		t.Errorf("RawOutput not preserved")
	}
	// Claims/Risks/Recommendations should be nil/empty
	if len(output.Claims) != 0 {
		t.Errorf("len(Claims) = %d, want 0 for malformed JSON", len(output.Claims))
	}
}

func TestGetConsolidatedAnalysis_InvalidJSON(t *testing.T) {
	state := &core.WorkflowState{
		Checkpoints: []core.Checkpoint{
			{
				Type: "consolidated_analysis",
				Data: []byte(`invalid json`),
			},
		},
	}

	result := GetConsolidatedAnalysis(state)
	if result != "" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want empty for invalid JSON", result)
	}
}

func TestGetConsolidatedAnalysis_MissingContent(t *testing.T) {
	state := &core.WorkflowState{
		Checkpoints: []core.Checkpoint{
			{
				Type: "consolidated_analysis",
				Data: []byte(`{"agent_count": 2}`), // content field is missing
			},
		},
	}

	result := GetConsolidatedAnalysis(state)
	if result != "" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want empty when content missing", result)
	}
}
