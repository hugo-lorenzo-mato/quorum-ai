package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func TestAnalyzer_selectCritiqueAgent(t *testing.T) {
	analyzer := NewAnalyzer(&mockConsensusEvaluator{score: 0.8})

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})
	registry.Register("gemini", &mockAgent{})

	wctx := &Context{
		Agents: registry,
		Logger: logging.NewNop(),
	}

	t.Run("selects different agent", func(t *testing.T) {
		result := analyzer.selectCritiqueAgent(context.Background(), wctx, "claude")
		if result == "claude" {
			t.Error("should select a different agent when available")
		}
	})

	t.Run("returns original when no other available", func(t *testing.T) {
		singleRegistry := &mockAgentRegistry{}
		singleRegistry.Register("claude", &mockAgent{})
		singleCtx := &Context{
			Agents: singleRegistry,
			Logger: logging.NewNop(),
		}

		result := analyzer.selectCritiqueAgent(context.Background(), singleCtx, "claude")
		if result != "claude" {
			t.Errorf("should return original agent when no other available, got %q", result)
		}
	})
}

func TestAnalyzer_Run_NeedsHumanReview(t *testing.T) {
	// Setup mocks with score below human threshold
	consensus := &mockConsensusEvaluator{
		score:            0.3,
		needsV3:          true,
		needsHumanReview: true,
		threshold:        0.75,
		v2Threshold:      0.60,
		humanThreshold:   0.50,
	}
	analyzer := NewAnalyzer(consensus)

	registry := &mockAgentRegistry{}
	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    `{"claims":["c1"],"risks":["r1"],"recommendations":["rec1"]}`,
			TokensIn:  100,
			TokensOut: 50,
		},
	}
	registry.Register("claude", agent)
	registry.Register("gemini", agent)

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
			V3Agent:      "claude",
		},
	}

	err := analyzer.Run(context.Background(), wctx)
	if err == nil {
		t.Fatal("expected error for human review required")
	}

	// Check it's the right type of error
	domErr, ok := err.(*core.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if domErr.Category != core.ErrCatConsensus {
		t.Errorf("expected consensus error category, got %v", domErr.Category)
	}
}

func TestAnalyzer_Run_AgentExecutionError(t *testing.T) {
	consensus := &mockConsensusEvaluator{score: 0.9, threshold: 0.75}
	analyzer := NewAnalyzer(consensus)

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
			V3Agent:      "claude",
		},
	}

	err := analyzer.Run(context.Background(), wctx)
	if err == nil {
		t.Fatal("expected error when agent execution fails")
	}
}

func TestConsensusResult_Fields(t *testing.T) {
	result := ConsensusResult{
		Score:            0.85,
		NeedsV3:          true,
		NeedsHumanReview: false,
		Divergences: []Divergence{
			{Category: "risk", Agent1: "claude", Agent2: "gemini"},
			{Category: "priority", Agent1: "claude", Agent2: "gemini"},
		},
	}

	if result.Score != 0.85 {
		t.Errorf("Score = %v, want 0.85", result.Score)
	}
	if !result.NeedsV3 {
		t.Error("NeedsV3 should be true")
	}
	if result.NeedsHumanReview {
		t.Error("NeedsHumanReview should be false")
	}
	if len(result.Divergences) != 2 {
		t.Errorf("len(Divergences) = %d, want 2", len(result.Divergences))
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

func TestMockConsensusEvaluator_DefaultThresholds(t *testing.T) {
	evaluator := &mockConsensusEvaluator{
		score:     0.8,
		threshold: 0.75,
		// v2Threshold and humanThreshold not set
	}

	if evaluator.V2Threshold() != 0.60 {
		t.Errorf("V2Threshold() = %v, want 0.60 (default)", evaluator.V2Threshold())
	}
	if evaluator.HumanThreshold() != 0.50 {
		t.Errorf("HumanThreshold() = %v, want 0.50 (default)", evaluator.HumanThreshold())
	}
}
