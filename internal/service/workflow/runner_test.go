package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestDefaultRunnerConfig(t *testing.T) {
	cfg := DefaultRunnerConfig()

	if cfg == nil {
		t.Fatal("DefaultRunnerConfig() returned nil")
	}
	if cfg.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
	if cfg.MaxRetries <= 0 {
		t.Error("MaxRetries should be positive")
	}
	if cfg.DefaultAgent == "" {
		t.Error("DefaultAgent should not be empty")
	}
}

func TestDefaultRunnerConfig_SandboxEnabled(t *testing.T) {
	config := DefaultRunnerConfig()
	if !config.Sandbox {
		t.Error("Expected DefaultRunnerConfig().Sandbox to be true")
	}
}

func TestRunner_validateRunInput(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		config  *RunnerConfig
		agents  []string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid prompt",
			prompt:  "Analyze this code",
			config:  DefaultRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: false,
		},
		{
			name:    "empty prompt",
			prompt:  "",
			config:  DefaultRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeEmptyPrompt,
		},
		{
			name:    "whitespace only prompt",
			prompt:  "   \t\n  ",
			config:  DefaultRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeEmptyPrompt,
		},
		{
			name:    "prompt too long",
			prompt:  strings.Repeat("a", core.MaxPromptLength+1),
			config:  DefaultRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodePromptTooLong,
		},
		{
			name:   "zero timeout",
			prompt: "test",
			config: &RunnerConfig{
				Timeout:      0,
				DefaultAgent: "claude",
			},
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeInvalidTimeout,
		},
		{
			name:    "no agents",
			prompt:  "test",
			config:  DefaultRunnerConfig(),
			agents:  []string{},
			wantErr: true,
			errCode: core.CodeNoAgents,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockAgentRegistry{}
			for _, name := range tt.agents {
				registry.Register(name, &mockAgent{})
			}

			runner := &Runner{
				config: tt.config,
				agents: registry,
			}

			err := runner.validateRunInput(tt.prompt)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRunInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCode != "" {
				domErr, ok := err.(*core.DomainError)
				if !ok {
					t.Errorf("expected DomainError, got %T", err)
					return
				}
				if domErr.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", domErr.Code, tt.errCode)
				}
			}
		})
	}
}

func TestRunner_initializeState(t *testing.T) {
	runner := &Runner{
		config: DefaultRunnerConfig(),
	}

	prompt := "Test prompt for analysis"
	state := runner.initializeState(prompt)

	if state == nil {
		t.Fatal("initializeState() returned nil")
	}
	if state.Prompt != prompt {
		t.Errorf("Prompt = %q, want %q", state.Prompt, prompt)
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("Status = %v, want %v", state.Status, core.WorkflowStatusRunning)
	}
	if state.CurrentPhase != core.PhaseOptimize {
		t.Errorf("CurrentPhase = %v, want %v", state.CurrentPhase, core.PhaseOptimize)
	}
	if state.WorkflowID == "" {
		t.Error("WorkflowID should not be empty")
	}
	if state.Tasks == nil {
		t.Error("Tasks map should be initialized")
	}
	if state.Checkpoints == nil {
		t.Error("Checkpoints slice should be initialized")
	}
	if state.Metrics == nil {
		t.Error("Metrics should be initialized")
	}
}

func TestRunner_createContext(t *testing.T) {
	config := &RunnerConfig{
		DryRun:       true,
		Sandbox:      false,
		DenyTools:    []string{"rm", "sudo"},
		DefaultAgent: "gemini",
		V3Agent:      "claude",
	}

	runner := &Runner{
		config: config,
	}

	state := &core.WorkflowState{
		WorkflowID: "wf-test",
	}

	ctx := runner.createContext(state)

	if ctx == nil {
		t.Fatal("createContext() returned nil")
	}
	if ctx.State != state {
		t.Error("State not set correctly")
	}
	if ctx.Config == nil {
		t.Fatal("Config not set")
	}
	if ctx.Config.DryRun != config.DryRun {
		t.Errorf("DryRun = %v, want %v", ctx.Config.DryRun, config.DryRun)
	}
	if ctx.Config.Sandbox != config.Sandbox {
		t.Errorf("Sandbox = %v, want %v", ctx.Config.Sandbox, config.Sandbox)
	}
	if ctx.Config.DefaultAgent != config.DefaultAgent {
		t.Errorf("DefaultAgent = %q, want %q", ctx.Config.DefaultAgent, config.DefaultAgent)
	}
	if ctx.Config.V3Agent != config.V3Agent {
		t.Errorf("V3Agent = %q, want %q", ctx.Config.V3Agent, config.V3Agent)
	}
}

func TestGenerateWorkflowID(t *testing.T) {
	id1 := generateWorkflowID()
	id2 := generateWorkflowID()

	if id1 == "" {
		t.Error("generateWorkflowID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateWorkflowID() should return unique IDs")
	}
	if !strings.HasPrefix(id1, "wf-") {
		t.Errorf("generateWorkflowID() = %q, should start with 'wf-'", id1)
	}
}

// mockStateManager implements StateManager for testing.
type mockStateManager struct {
	state   *core.WorkflowState
	saveErr error
	loadErr error
	lockErr error
}

func (m *mockStateManager) Save(_ context.Context, state *core.WorkflowState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.state = state
	return nil
}

func (m *mockStateManager) Load(_ context.Context) (*core.WorkflowState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.state, nil
}

func (m *mockStateManager) AcquireLock(_ context.Context) error {
	return m.lockErr
}

func (m *mockStateManager) ReleaseLock(_ context.Context) error {
	return nil
}

func TestRunner_SetDryRun(t *testing.T) {
	runner := &Runner{
		config: DefaultRunnerConfig(),
	}

	if runner.config.DryRun {
		t.Error("DryRun should be false initially")
	}

	runner.SetDryRun(true)
	if !runner.config.DryRun {
		t.Error("SetDryRun(true) should set DryRun to true")
	}

	runner.SetDryRun(false)
	if runner.config.DryRun {
		t.Error("SetDryRun(false) should set DryRun to false")
	}
}

func TestRunner_GetState(t *testing.T) {
	expectedState := &core.WorkflowState{
		WorkflowID: "wf-test",
		Status:     core.WorkflowStatusCompleted,
	}

	stateManager := &mockStateManager{state: expectedState}
	runner := &Runner{
		config: DefaultRunnerConfig(),
		state:  stateManager,
	}

	state, err := runner.GetState(context.Background())
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state != expectedState {
		t.Error("GetState() did not return expected state")
	}
}
