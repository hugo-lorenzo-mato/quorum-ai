package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestDefaultRunnerConfig(t *testing.T) {
	t.Parallel()
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
	// DefaultAgent has NO default - must be configured in config file
	if cfg.DefaultAgent != "" {
		t.Errorf("DefaultAgent = %q, want empty (no default)", cfg.DefaultAgent)
	}
}

// testRunnerConfig returns a config suitable for testing with required fields set.
func testRunnerConfig() *RunnerConfig {
	cfg := DefaultRunnerConfig()
	cfg.DefaultAgent = "claude" // Required for tests
	return cfg
}

func TestRunner_validateRunInput(t *testing.T) {
	t.Parallel()
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
			config:  testRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: false,
		},
		{
			name:    "empty prompt",
			prompt:  "",
			config:  testRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeEmptyPrompt,
		},
		{
			name:    "whitespace only prompt",
			prompt:  "   \t\n  ",
			config:  testRunnerConfig(),
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeEmptyPrompt,
		},
		{
			name:    "prompt too long",
			prompt:  strings.Repeat("a", core.MaxPromptLength+1),
			config:  testRunnerConfig(),
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
			config:  testRunnerConfig(),
			agents:  []string{},
			wantErr: true,
			errCode: core.CodeNoAgents,
		},
		{
			name:    "missing default agent",
			prompt:  "test",
			config:  DefaultRunnerConfig(), // No DefaultAgent set
			agents:  []string{"claude"},
			wantErr: true,
			errCode: core.CodeInvalidConfig,
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
	t.Parallel()
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
	if state.CurrentPhase != core.PhaseRefine {
		t.Errorf("CurrentPhase = %v, want %v", state.CurrentPhase, core.PhaseRefine)
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
	t.Parallel()
	config := &RunnerConfig{
		DryRun:       true,
		DenyTools:    []string{"rm", "sudo"},
		DefaultAgent: "gemini",
	}

	runner := &Runner{
		config: config,
	}

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test",
		},
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
	if ctx.Config.DefaultAgent != config.DefaultAgent {
		t.Errorf("DefaultAgent = %q, want %q", ctx.Config.DefaultAgent, config.DefaultAgent)
	}
}

func TestGenerateWorkflowID(t *testing.T) {
	t.Parallel()
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

func (m *mockStateManager) LoadByID(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.state != nil && m.state.WorkflowID == id {
		return m.state, nil
	}
	return nil, nil
}

func (m *mockStateManager) DeactivateWorkflow(_ context.Context) error {
	return nil
}

func (m *mockStateManager) ArchiveWorkflows(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockStateManager) PurgeAllWorkflows(_ context.Context) (int, error) {
	if m.state != nil {
		m.state = nil
		return 1, nil
	}
	return 0, nil
}

func (m *mockStateManager) DeleteWorkflow(_ context.Context, id core.WorkflowID) error {
	if m.state != nil && m.state.WorkflowID == id {
		m.state = nil
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) Exists() bool {
	return m.state != nil
}

func (m *mockStateManager) Backup(_ context.Context) error {
	return nil
}

func (m *mockStateManager) Restore(_ context.Context) (*core.WorkflowState, error) {
	return m.state, nil
}

func (m *mockStateManager) ListWorkflows(_ context.Context) ([]core.WorkflowSummary, error) {
	if m.state != nil {
		return []core.WorkflowSummary{{WorkflowID: m.state.WorkflowID}}, nil
	}
	return nil, nil
}

func (m *mockStateManager) GetActiveWorkflowID(_ context.Context) (core.WorkflowID, error) {
	if m.state != nil {
		return m.state.WorkflowID, nil
	}
	return "", nil
}

func (m *mockStateManager) SetActiveWorkflowID(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) UpdateHeartbeat(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) FindZombieWorkflows(_ context.Context, _ time.Duration) ([]*core.WorkflowState, error) {
	return nil, nil
}

func (m *mockStateManager) AcquireWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ReleaseWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	if m.state != nil && m.state.Status == core.WorkflowStatusRunning {
		return []core.WorkflowID{m.state.WorkflowID}, nil
	}
	return nil, nil
}

func (m *mockStateManager) IsWorkflowRunning(_ context.Context, id core.WorkflowID) (bool, error) {
	if m.state != nil && m.state.WorkflowID == id && m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
}

func (m *mockStateManager) UpdateWorkflowHeartbeat(_ context.Context, id core.WorkflowID) error {
	if m.state != nil && m.state.WorkflowID == id {
		now := time.Now().UTC()
		m.state.HeartbeatAt = &now
		return nil
	}
	return nil
}

func (m *mockStateManager) ExecuteAtomically(_ context.Context, fn func(core.AtomicStateContext) error) error {
	atomicCtx := &mockAtomicCtx{m: m}
	return fn(atomicCtx)
}

type mockAtomicCtx struct {
	m *mockStateManager
}

func (a *mockAtomicCtx) LoadByID(id core.WorkflowID) (*core.WorkflowState, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id {
		return a.m.state, nil
	}
	return nil, nil
}

func (a *mockAtomicCtx) Save(state *core.WorkflowState) error {
	a.m.state = state
	return nil
}

func (a *mockAtomicCtx) SetWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicCtx) ClearWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicCtx) IsWorkflowRunning(id core.WorkflowID) (bool, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id && a.m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
}

func TestRunner_SetDryRun(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	expectedState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusCompleted,
		},
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

func TestFinalizeMetrics_CalculatesDuration(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			CreatedAt: time.Now().Add(-5 * time.Minute),
		},
		WorkflowRun: core.WorkflowRun{
			Metrics: &core.StateMetrics{},
		},
	}

	runner := &Runner{}
	runner.finalizeMetrics(state)

	// Duration should be approximately 5 minutes
	if state.Metrics.Duration < 4*time.Minute || state.Metrics.Duration > 6*time.Minute {
		t.Errorf("Expected ~5m duration, got %v", state.Metrics.Duration)
	}
}

func TestFinalizeMetrics_InitializesNilMetrics(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			CreatedAt: time.Now(),
		},
	}

	runner := &Runner{}
	runner.finalizeMetrics(state)

	if state.Metrics == nil {
		t.Error("Expected Metrics to be initialized")
	}
}
