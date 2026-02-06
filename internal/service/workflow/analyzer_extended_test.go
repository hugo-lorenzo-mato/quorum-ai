package workflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
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
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
				Prompt:     "test prompt",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Tasks:        make(map[core.TaskID]*core.TaskState),
				TaskOrder:    []core.TaskID{},
				Checkpoints:  []core.Checkpoint{},
				Metrics:      &core.StateMetrics{},
			},
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
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type: "consolidated_analysis",
					Data: []byte(`invalid json`),
				},
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
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type: "consolidated_analysis",
					Data: []byte(`{"agent_count": 2}`), // content field is missing
				},
			},
		},
	}

	result := GetConsolidatedAnalysis(state)
	if result != "" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want empty when content missing", result)
	}
}

// ============================================================================
// Tests for checkpoint-based analysis caching
// ============================================================================

func TestComputePromptHash_Deterministic(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Prompt: "Analyze this codebase",
		},
	}

	hash1 := computePromptHash(state)
	hash2 := computePromptHash(state)

	if hash1 != hash2 {
		t.Errorf("computePromptHash() not deterministic: %s != %s", hash1, hash2)
	}

	// Hash should be 64 characters (SHA256 hex)
	if len(hash1) != 64 {
		t.Errorf("computePromptHash() length = %d, want 64", len(hash1))
	}
}

func TestComputePromptHash_DifferentPrompts(t *testing.T) {
	state1 := &core.WorkflowState{WorkflowDefinition: core.WorkflowDefinition{Prompt: "Analyze this"}}
	state2 := &core.WorkflowState{WorkflowDefinition: core.WorkflowDefinition{Prompt: "Analyze that"}}

	hash1 := computePromptHash(state1)
	hash2 := computePromptHash(state2)

	if hash1 == hash2 {
		t.Error("computePromptHash() should produce different hashes for different prompts")
	}
}

func TestComputeContentHash_Deterministic(t *testing.T) {
	content := "Some analysis content"

	hash1 := computeContentHash(content)
	hash2 := computeContentHash(content)

	if hash1 != hash2 {
		t.Errorf("computeContentHash() not deterministic: %s != %s", hash1, hash2)
	}
}

func TestGetAnalysisCheckpoint_Found(t *testing.T) {
	promptHash := "abc123"
	meta := AnalysisCheckpointMetadata{
		AgentName:   "claude",
		Model:       "claude-3-opus",
		Round:       1,
		FilePath:    "/tmp/analysis.md",
		PromptHash:  promptHash,
		TokensIn:    1000,
		TokensOut:   500,
		DurationMS:  5000,
		ContentHash: "xyz789",
	}
	metaBytes, _ := json.Marshal(meta)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type:  string(service.CheckpointAnalysisComplete),
					Phase: core.PhaseAnalyze,
					Data:  metaBytes,
				},
			},
		},
	}

	result := getAnalysisCheckpoint(state, "claude", 1, promptHash)

	if result == nil {
		t.Fatal("getAnalysisCheckpoint() returned nil, want checkpoint")
	}
	if result.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "claude")
	}
	if result.TokensIn != 1000 {
		t.Errorf("TokensIn = %d, want 1000", result.TokensIn)
	}
}

func TestGetAnalysisCheckpoint_PromptHashMismatch(t *testing.T) {
	meta := AnalysisCheckpointMetadata{
		AgentName:  "claude",
		Round:      1,
		PromptHash: "old-prompt-hash",
	}
	metaBytes, _ := json.Marshal(meta)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type:  string(service.CheckpointAnalysisComplete),
					Phase: core.PhaseAnalyze,
					Data:  metaBytes,
				},
			},
		},
	}

	// Looking for a different prompt hash - should invalidate cache
	result := getAnalysisCheckpoint(state, "claude", 1, "new-prompt-hash")

	if result != nil {
		t.Error("getAnalysisCheckpoint() should return nil when prompt hash doesn't match")
	}
}

func TestGetAnalysisCheckpoint_WrongAgent(t *testing.T) {
	meta := AnalysisCheckpointMetadata{
		AgentName:  "gemini",
		Round:      1,
		PromptHash: "abc123",
	}
	metaBytes, _ := json.Marshal(meta)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type:  string(service.CheckpointAnalysisComplete),
					Phase: core.PhaseAnalyze,
					Data:  metaBytes,
				},
			},
		},
	}

	// Looking for claude but checkpoint is for gemini
	result := getAnalysisCheckpoint(state, "claude", 1, "abc123")

	if result != nil {
		t.Error("getAnalysisCheckpoint() should return nil when agent doesn't match")
	}
}

func TestGetAnalysisCheckpoint_WrongRound(t *testing.T) {
	meta := AnalysisCheckpointMetadata{
		AgentName:  "claude",
		Round:      1, // V1 checkpoint
		PromptHash: "abc123",
	}
	metaBytes, _ := json.Marshal(meta)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Checkpoints: []core.Checkpoint{
				{
					Type:  string(service.CheckpointAnalysisComplete),
					Phase: core.PhaseAnalyze,
					Data:  metaBytes,
				},
			},
		},
	}

	// Looking for V2 round
	result := getAnalysisCheckpoint(state, "claude", 2, "abc123")

	if result != nil {
		t.Error("getAnalysisCheckpoint() should return nil when round doesn't match")
	}
}

func TestRestoreAnalysisFromCheckpoint_Success(t *testing.T) {
	// Create a temporary file with content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "analysis.md")
	content := "# Analysis\n\nThis is the analysis content."
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	meta := &AnalysisCheckpointMetadata{
		AgentName:   "claude",
		Model:       "claude-3-opus",
		Round:       1,
		FilePath:    tmpFile,
		PromptHash:  "abc123",
		TokensIn:    1500,
		TokensOut:   800,
		DurationMS:  7500,
		ContentHash: computeContentHash(content), // Correct hash
	}

	output, err := restoreAnalysisFromCheckpoint(meta)

	if err != nil {
		t.Fatalf("restoreAnalysisFromCheckpoint() error = %v", err)
	}
	if output.AgentName != "claude" {
		t.Errorf("AgentName = %q, want %q", output.AgentName, "claude")
	}
	if output.TokensIn != 1500 {
		t.Errorf("TokensIn = %d, want 1500", output.TokensIn)
	}
	if output.TokensOut != 800 {
		t.Errorf("TokensOut = %d, want 800", output.TokensOut)
	}
	if output.DurationMS != 7500 {
		t.Errorf("DurationMS = %d, want 7500", output.DurationMS)
	}
	if output.RawOutput != content {
		t.Errorf("RawOutput = %q, want %q", output.RawOutput, content)
	}
}

func TestRestoreAnalysisFromCheckpoint_ContentHashMismatch(t *testing.T) {
	// Create a temporary file with content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "analysis.md")
	originalContent := "# Original Analysis"
	if err := os.WriteFile(tmpFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Now modify the file to simulate external modification
	modifiedContent := "# Modified Analysis"
	if err := os.WriteFile(tmpFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify temp file: %v", err)
	}

	meta := &AnalysisCheckpointMetadata{
		AgentName:   "claude",
		FilePath:    tmpFile,
		ContentHash: computeContentHash(originalContent), // Old hash from before modification
	}

	_, err := restoreAnalysisFromCheckpoint(meta)

	if err == nil {
		t.Error("restoreAnalysisFromCheckpoint() should return error when content hash doesn't match")
	}
}

func TestRestoreAnalysisFromCheckpoint_FileNotFound(t *testing.T) {
	meta := &AnalysisCheckpointMetadata{
		AgentName:   "claude",
		FilePath:    "/nonexistent/path/analysis.md",
		ContentHash: "abc123",
	}

	_, err := restoreAnalysisFromCheckpoint(meta)

	if err == nil {
		t.Error("restoreAnalysisFromCheckpoint() should return error when file doesn't exist")
	}
}
