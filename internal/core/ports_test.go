package core

import (
	"testing"
	"time"
)

func TestDefaultExecuteOptions(t *testing.T) {
	opts := DefaultExecuteOptions()

	if opts.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens 4096, got %d", opts.MaxTokens)
	}
	if opts.Temperature != 0.7 {
		t.Errorf("expected Temperature 0.7, got %f", opts.Temperature)
	}
	if opts.Format != OutputFormatText {
		t.Errorf("expected Format text, got %s", opts.Format)
	}
	if opts.Timeout != 10*time.Minute {
		t.Errorf("expected Timeout 10m, got %v", opts.Timeout)
	}
}

func TestExecuteResult_TotalTokens(t *testing.T) {
	tests := []struct {
		in, out, total int
	}{
		{0, 0, 0},
		{100, 50, 150},
		{1000, 500, 1500},
	}

	for _, tt := range tests {
		r := &ExecuteResult{TokensIn: tt.in, TokensOut: tt.out}
		if got := r.TotalTokens(); got != tt.total {
			t.Errorf("TotalTokens() = %d, want %d", got, tt.total)
		}
	}
}

func TestCheckStatus_IsSuccess(t *testing.T) {
	tests := []struct {
		name    string
		status  CheckStatus
		success bool
	}{
		{
			name:    "success with no failures",
			status:  CheckStatus{State: "success", Failed: 0},
			success: true,
		},
		{
			name:    "success state but with failures",
			status:  CheckStatus{State: "success", Failed: 1},
			success: false,
		},
		{
			name:    "failure state",
			status:  CheckStatus{State: "failure", Failed: 1},
			success: false,
		},
		{
			name:    "pending state",
			status:  CheckStatus{State: "pending", Failed: 0},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsSuccess(); got != tt.success {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.success)
			}
		})
	}
}

func TestCheckStatus_IsPending(t *testing.T) {
	tests := []struct {
		name    string
		status  CheckStatus
		pending bool
	}{
		{
			name:    "pending state",
			status:  CheckStatus{State: "pending", Pending: 0},
			pending: true,
		},
		{
			name:    "has pending checks",
			status:  CheckStatus{State: "success", Pending: 1},
			pending: true,
		},
		{
			name:    "success with no pending",
			status:  CheckStatus{State: "success", Pending: 0},
			pending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsPending(); got != tt.pending {
				t.Errorf("IsPending() = %v, want %v", got, tt.pending)
			}
		})
	}
}

func TestNewWorkflowState(t *testing.T) {
	bp := &Blueprint{
		Consensus: BlueprintConsensus{Threshold: 0.8},
		MaxRetries: 5,
	}
	wf := NewWorkflow("test-wf", "test prompt", bp)
	task := NewTask("t1", "test task", PhaseAnalyze)
	task.TokensIn = 100
	task.TokensOut = 50
	task.CostUSD = 0.01
	_ = wf.AddTask(task)
	wf.ConsensusScore = 0.9
	wf.TotalCostUSD = 0.01
	wf.TotalTokensIn = 100
	wf.TotalTokensOut = 50

	state := NewWorkflowState(wf)

	if state.Version != CurrentStateVersion {
		t.Errorf("expected version %d, got %d", CurrentStateVersion, state.Version)
	}
	if state.WorkflowID != "test-wf" {
		t.Errorf("expected workflow ID 'test-wf', got %s", state.WorkflowID)
	}
	if state.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %s", state.Prompt)
	}
	if len(state.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.Tasks))
	}
	if state.Metrics.TotalCostUSD != 0.01 {
		t.Errorf("expected cost 0.01, got %f", state.Metrics.TotalCostUSD)
	}
	if state.Metrics.ConsensusScore != 0.9 {
		t.Errorf("expected consensus score 0.9, got %f", state.Metrics.ConsensusScore)
	}

	// Verify task state
	taskState, ok := state.Tasks["t1"]
	if !ok {
		t.Fatal("expected task t1 in state")
	}
	if taskState.TokensIn != 100 {
		t.Errorf("expected TokensIn 100, got %d", taskState.TokensIn)
	}
}

func TestOutputFormatConstants(t *testing.T) {
	if OutputFormatText != "text" {
		t.Errorf("expected 'text', got %s", OutputFormatText)
	}
	if OutputFormatJSON != "json" {
		t.Errorf("expected 'json', got %s", OutputFormatJSON)
	}
	if OutputFormatMarkdown != "markdown" {
		t.Errorf("expected 'markdown', got %s", OutputFormatMarkdown)
	}
}

func TestWorktreeStatusConstants(t *testing.T) {
	if WorktreeStatusActive != "active" {
		t.Errorf("expected 'active', got %s", WorktreeStatusActive)
	}
	if WorktreeStatusStale != "stale" {
		t.Errorf("expected 'stale', got %s", WorktreeStatusStale)
	}
	if WorktreeStatusCleaned != "cleaned" {
		t.Errorf("expected 'cleaned', got %s", WorktreeStatusCleaned)
	}
}

func TestCapabilities(t *testing.T) {
	caps := Capabilities{
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsImages:    false,
		SupportsJSON:      true,
		SupportedModels:   []string{"claude-3", "claude-3.5"},
		DefaultModel:      "claude-3.5",
		MaxContextTokens:  200000,
		MaxOutputTokens:   4096,
		RateLimitRPM:      60,
		RateLimitTPM:      100000,
	}

	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming to be true")
	}
	if len(caps.SupportedModels) != 2 {
		t.Errorf("expected 2 models, got %d", len(caps.SupportedModels))
	}
}

func TestToolCall(t *testing.T) {
	tc := ToolCall{
		ID:   "call-123",
		Name: "read_file",
		Arguments: map[string]interface{}{
			"path": "/test/file.txt",
		},
		Result: "file contents here",
	}

	if tc.ID != "call-123" {
		t.Errorf("expected ID 'call-123', got %s", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected Name 'read_file', got %s", tc.Name)
	}
	if tc.Arguments["path"] != "/test/file.txt" {
		t.Errorf("expected path '/test/file.txt', got %v", tc.Arguments["path"])
	}
}
