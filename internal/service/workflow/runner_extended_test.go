package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func TestNewRunner(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		deps := RunnerDeps{
			Config: nil,
			State:  &mockStateManager{},
			Agents: &mockAgentRegistry{},
		}

		runner := NewRunner(deps)
		if runner == nil {
			t.Fatal("NewRunner() returned nil")
		}
		if runner.config == nil {
			t.Error("runner.config should not be nil")
		}
		if runner.config.Timeout <= 0 {
			t.Error("default timeout should be positive")
		}
	})

	t.Run("with nil logger uses nop", func(t *testing.T) {
		deps := RunnerDeps{
			Config: DefaultRunnerConfig(),
			State:  &mockStateManager{},
			Agents: &mockAgentRegistry{},
			Logger: nil,
		}

		runner := NewRunner(deps)
		if runner.logger == nil {
			t.Error("runner.logger should not be nil")
		}
	})

	t.Run("with nil output uses nop", func(t *testing.T) {
		deps := RunnerDeps{
			Config: DefaultRunnerConfig(),
			State:  &mockStateManager{},
			Agents: &mockAgentRegistry{},
			Output: nil,
		}

		runner := NewRunner(deps)
		if runner.output == nil {
			t.Error("runner.output should not be nil")
		}
	})

	t.Run("with all dependencies", func(t *testing.T) {
		checkpoint := &mockCheckpointCreator{}
		state := &mockStateManager{}
		agents := &mockAgentRegistry{}
		logger := logging.NewNop()

		deps := RunnerDeps{
			Config:     DefaultRunnerConfig(),
			State:      state,
			Agents:     agents,
			Checkpoint: checkpoint,
			Logger:     logger,
			Output:     NopOutputNotifier{},
		}

		runner := NewRunner(deps)
		if runner.state != state {
			t.Error("state not set correctly")
		}
		if runner.agents != agents {
			t.Error("agents not set correctly")
		}
		if runner.checkpoint != checkpoint {
			t.Error("checkpoint not set correctly")
		}
		if runner.logger != logger {
			t.Error("logger not set correctly")
		}
	})
}

func TestDefaultRunnerConfig_Values(t *testing.T) {
	cfg := DefaultRunnerConfig()

	if cfg.Timeout != time.Hour {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, time.Hour)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.DryRun != false {
		t.Error("DryRun should be false by default")
	}
	if cfg.Sandbox != true {
		t.Error("Sandbox should be true by default")
	}
	// DefaultAgent has NO default - must be configured in config file
	if cfg.DefaultAgent != "" {
		t.Errorf("DefaultAgent = %q, want empty (no default)", cfg.DefaultAgent)
	}
	if cfg.WorktreeMode != "always" {
		t.Errorf("WorktreeMode = %q, want %q", cfg.WorktreeMode, "always")
	}
	if cfg.AgentPhaseModels == nil {
		t.Error("AgentPhaseModels should be initialized")
	}
}

func TestRunner_handleError(t *testing.T) {
	checkpoint := &mockCheckpointCreator{}
	state := &mockStateManager{}

	runner := &Runner{
		config:     DefaultRunnerConfig(),
		state:      state,
		checkpoint: checkpoint,
		logger:     logging.NewNop(),
	}

	workflowState := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseAnalyze,
		Status:       core.WorkflowStatusRunning,
		Metrics:      &core.StateMetrics{},
		Checkpoints:  []core.Checkpoint{},
	}

	testErr := errors.New("test error")
	err := runner.handleError(context.Background(), workflowState, testErr)

	if err != testErr {
		t.Errorf("handleError() = %v, want %v", err, testErr)
	}
	if workflowState.Status != core.WorkflowStatusFailed {
		t.Errorf("Status = %v, want %v", workflowState.Status, core.WorkflowStatusFailed)
	}
}

func TestRunner_Run_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		timeout time.Duration
		agents  []string
	}{
		{"empty prompt", "", time.Hour, []string{"claude"}},
		{"zero timeout", "valid", 0, []string{"claude"}},
		{"no agents", "valid", time.Hour, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockAgentRegistry{}
			for _, name := range tt.agents {
				registry.Register(name, &mockAgent{})
			}

			config := DefaultRunnerConfig()
			config.Timeout = tt.timeout

			runner := &Runner{
				config: config,
				agents: registry,
				state:  &mockStateManager{},
				logger: logging.NewNop(),
			}

			err := runner.Run(context.Background(), tt.prompt)
			if err == nil {
				t.Error("Run() should return error")
			}
		})
	}
}

func TestRunner_Run_LockError(t *testing.T) {
	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	state := &mockStateManager{
		lockErr: errors.New("lock error"),
	}

	runner := &Runner{
		config: DefaultRunnerConfig(),
		agents: registry,
		state:  state,
		logger: logging.NewNop(),
	}

	err := runner.Run(context.Background(), "test prompt")
	if err == nil {
		t.Error("Run() should return lock error")
	}
}

func TestGenerateWorkflowID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 100

	for i := 0; i < count; i++ {
		id := generateWorkflowID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestRunner_Resume_NoState(t *testing.T) {
	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	state := &mockStateManager{
		state: nil, // No state to resume
	}

	resumeProvider := &mockResumePointProvider{
		resumePoint: &ResumePoint{Phase: core.PhaseAnalyze},
	}

	runner := &Runner{
		config:         DefaultRunnerConfig(),
		agents:         registry,
		state:          state,
		resumeProvider: resumeProvider,
		logger:         logging.NewNop(),
	}

	err := runner.Resume(context.Background())
	if err == nil {
		t.Error("Resume() should return error when no state")
	}
}

// mockResumePointProvider implements ResumePointProvider for testing.
type mockResumePointProvider struct {
	resumePoint *ResumePoint
	err         error
}

func (m *mockResumePointProvider) GetResumePoint(_ *core.WorkflowState) (*ResumePoint, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resumePoint, nil
}

func TestResumePoint_Fields(t *testing.T) {
	rp := ResumePoint{
		Phase:     core.PhaseExecute,
		TaskID:    "task-1",
		FromStart: true,
	}

	if rp.Phase != core.PhaseExecute {
		t.Errorf("Phase = %v, want %v", rp.Phase, core.PhaseExecute)
	}
	if rp.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", rp.TaskID, "task-1")
	}
	if !rp.FromStart {
		t.Error("FromStart should be true")
	}
}

func TestRunnerConfig_Fields(t *testing.T) {
	cfg := &RunnerConfig{
		Timeout:            2 * time.Hour,
		MaxRetries:         5,
		DryRun:            true,
		Sandbox:           false,
		DenyTools:         []string{"rm", "sudo", "mkfs"},
		DefaultAgent:      "gemini",
		AgentPhaseModels:  map[string]map[string]string{"claude": {"analyze": "opus"}},
		WorktreeAutoClean: true,
		WorktreeMode:      "parallel",
	}

	if cfg.Timeout != 2*time.Hour {
		t.Errorf("Timeout = %v, want 2h", cfg.Timeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if !cfg.DryRun {
		t.Error("DryRun should be true")
	}
	if cfg.Sandbox {
		t.Error("Sandbox should be false")
	}
	if len(cfg.DenyTools) != 3 {
		t.Errorf("len(DenyTools) = %d, want 3", len(cfg.DenyTools))
	}
	if cfg.DefaultAgent != "gemini" {
		t.Errorf("DefaultAgent = %q, want %q", cfg.DefaultAgent, "gemini")
	}
	if !cfg.WorktreeAutoClean {
		t.Error("WorktreeAutoClean should be true")
	}
	if cfg.WorktreeMode != "parallel" {
		t.Errorf("WorktreeMode = %q, want %q", cfg.WorktreeMode, "parallel")
	}
}
