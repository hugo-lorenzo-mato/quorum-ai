package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// mockResilienceStateManager implements a minimal StateManager for resilience tests.
type mockResilienceStateManager struct {
	workflows       map[core.WorkflowID]*core.WorkflowState
	activeID        core.WorkflowID
	locked          bool
	deactivateCalls int
	saveCalls       int
}

func newMockResilienceStateManager() *mockResilienceStateManager {
	return &mockResilienceStateManager{
		workflows: make(map[core.WorkflowID]*core.WorkflowState),
	}
}

func (m *mockResilienceStateManager) Save(_ context.Context, state *core.WorkflowState) error {
	m.saveCalls++
	m.workflows[state.WorkflowID] = state
	return nil
}

func (m *mockResilienceStateManager) Load(_ context.Context) (*core.WorkflowState, error) {
	if m.activeID == "" {
		return nil, nil
	}
	return m.workflows[m.activeID], nil
}

func (m *mockResilienceStateManager) LoadByID(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	return m.workflows[id], nil
}

func (m *mockResilienceStateManager) AcquireLock(_ context.Context) error {
	if m.locked {
		return errors.New("already locked")
	}
	m.locked = true
	return nil
}

func (m *mockResilienceStateManager) ReleaseLock(_ context.Context) error {
	m.locked = false
	return nil
}

func (m *mockResilienceStateManager) DeactivateWorkflow(_ context.Context) error {
	m.deactivateCalls++
	m.activeID = ""
	return nil
}

func (m *mockResilienceStateManager) ArchiveWorkflows(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockResilienceStateManager) PurgeAllWorkflows(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockResilienceStateManager) DeleteWorkflow(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// mockResilienceCheckpoint implements CheckpointCreator that records calls.
type mockResilienceCheckpoint struct {
	errorCheckpointCalls int
}

func (m *mockResilienceCheckpoint) PhaseCheckpoint(_ *core.WorkflowState, _ core.Phase, _ bool) error {
	return nil
}

func (m *mockResilienceCheckpoint) TaskCheckpoint(_ *core.WorkflowState, _ *core.Task, _ bool) error {
	return nil
}

func (m *mockResilienceCheckpoint) ErrorCheckpoint(_ *core.WorkflowState, _ error) error {
	m.errorCheckpointCalls++
	return nil
}

func (m *mockResilienceCheckpoint) ErrorCheckpointWithContext(_ *core.WorkflowState, _ error, _ service.ErrorCheckpointDetails) error {
	m.errorCheckpointCalls++
	return nil
}

func (m *mockResilienceCheckpoint) CreateCheckpoint(_ *core.WorkflowState, _ string, _ map[string]interface{}) error {
	return nil
}

// TestRunner_HandleError_DeactivatesWorkflow verifies that handleError properly
// deactivates the workflow to prevent ghost workflows.
// Regression: wf-20260130-030319-atstd remained active after failing.
func TestRunner_HandleError_DeactivatesWorkflow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stateManager := newMockResilienceStateManager()
	checkpoint := &mockResilienceCheckpoint{}

	// Create a minimal runner for testing handleError
	runner := &Runner{
		config: &RunnerConfig{
			Report: report.DefaultConfig(),
		},
		state:      stateManager,
		checkpoint: checkpoint,
		logger:     logging.NewNop(),
		output:     NopOutputNotifier{},
	}

	// Create a workflow state
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test-error-001",
			Prompt:     "test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			UpdatedAt:    time.Now(),
		},
	}

	// Call handleError
	testErr := errors.New("simulated phase failure")
	_ = runner.handleError(ctx, state, testErr)

	// Verify DeactivateWorkflow was called
	if stateManager.deactivateCalls != 1 {
		t.Errorf("DeactivateWorkflow() called %d times, want 1", stateManager.deactivateCalls)
	}

	// Verify workflow status is failed
	if state.Status != core.WorkflowStatusFailed {
		t.Errorf("Status = %v, want %v", state.Status, core.WorkflowStatusFailed)
	}

	// Verify error is recorded
	if state.Error != testErr.Error() {
		t.Errorf("Error = %q, want %q", state.Error, testErr.Error())
	}
}

// TestRunner_HandleError_WritesErrorFile verifies that handleError writes
// error details to the report directory.
func TestRunner_HandleError_WritesErrorFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()
	stateManager := newMockResilienceStateManager()
	checkpoint := &mockResilienceCheckpoint{}

	runner := &Runner{
		config: &RunnerConfig{
			Report: report.Config{
				BaseDir: tmpDir,
				Enabled: true,
			},
		},
		state:      stateManager,
		checkpoint: checkpoint,
		logger:     logging.NewNop(),
		output:     NopOutputNotifier{},
	}

	// Create state with report path
	reportPath := filepath.Join(tmpDir, "wf-test-error-file")
	if err := os.MkdirAll(reportPath, 0755); err != nil {
		t.Fatalf("Failed to create report dir: %v", err)
	}

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test-error-file",
			Prompt:     "test prompt for error file",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			ReportPath:   reportPath,
			UpdatedAt:    time.Now(),
		},
	}

	// Call handleError
	testErr := errors.New("test error message")
	_ = runner.handleError(ctx, state, testErr)

	// Verify error.md was created
	errorFile := filepath.Join(reportPath, "error.md")
	if _, err := os.Stat(errorFile); os.IsNotExist(err) {
		t.Error("error.md file was not created")
	}

	// Verify error.md contains expected content
	content, err := os.ReadFile(errorFile)
	if err != nil {
		t.Fatalf("Failed to read error.md: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Workflow Error") {
		t.Error("error.md should contain 'Workflow Error' header")
	}
	if !strings.Contains(contentStr, string(state.WorkflowID)) {
		t.Errorf("error.md should contain workflow ID %s", state.WorkflowID)
	}
	if !strings.Contains(contentStr, testErr.Error()) {
		t.Errorf("error.md should contain error message %q", testErr.Error())
	}
}

// TestRunner_CreateContext_InitializesReportDirectory verifies that createContext
// initializes the report directory immediately, before any validation can fail.
// Regression: wf-20260130-030319-atstd had no report directory because it failed
// before Initialize() was called.
func TestRunner_CreateContext_InitializesReportDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	runner := &Runner{
		config: &RunnerConfig{
			Report: report.Config{
				BaseDir: tmpDir,
				Enabled: true,
			},
			Moderator: ModeratorConfig{
				Threshold: 0.85,
			},
		},
		logger: logging.NewNop(),
		output: NopOutputNotifier{},
	}

	// Create a new workflow state (simulating new workflow)
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test-eager-init",
			Prompt:     "test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseRefine,
			UpdatedAt:    time.Now(),
			// ReportPath is empty - should be set by createContext
		},
	}

	// Call createContext
	wctx := runner.createContext(state)

	// Verify ReportPath was set
	if state.ReportPath == "" {
		t.Error("ReportPath should be set after createContext")
	}

	// Verify the directory exists
	if _, err := os.Stat(state.ReportPath); os.IsNotExist(err) {
		t.Errorf("Report directory %s should exist after createContext", state.ReportPath)
	}

	// Verify Report writer is not nil
	if wctx.Report == nil {
		t.Error("Context.Report should not be nil")
	}
}

// TestRunner_CreateContext_ReusesExistingReportPath verifies that createContext
// reuses the existing report path when resuming a workflow.
func TestRunner_CreateContext_ReusesExistingReportPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "wf-existing-path")

	// Create the existing report directory
	if err := os.MkdirAll(existingPath, 0755); err != nil {
		t.Fatalf("Failed to create existing path: %v", err)
	}

	runner := &Runner{
		config: &RunnerConfig{
			Report: report.Config{
				BaseDir: tmpDir,
				Enabled: true,
			},
			Moderator: ModeratorConfig{
				Threshold: 0.85,
			},
		},
		logger: logging.NewNop(),
		output: NopOutputNotifier{},
	}

	// Create a workflow state with existing ReportPath (resuming scenario)
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-resume-test",
			Prompt:     "test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseExecute,
			ReportPath:   existingPath, // Already set from previous run
			UpdatedAt:    time.Now(),
		},
	}

	// Call createContext
	_ = runner.createContext(state)

	// Verify ReportPath was NOT changed
	if state.ReportPath != existingPath {
		t.Errorf("ReportPath = %s, want %s (should not change on resume)", state.ReportPath, existingPath)
	}
}

// TestRunner_RunWithState_ValidationFailure_DeactivatesWorkflow verifies that
// RunWithState's markFailed helper properly deactivates the workflow.
func TestRunner_RunWithState_ValidationFailure_DeactivatesWorkflow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stateManager := newMockResilienceStateManager()

	// Create a minimal runner
	registry := &mockResilienceRegistry{agents: []string{"claude"}}

	runner := &Runner{
		config: &RunnerConfig{
			Timeout:      time.Hour,
			DefaultAgent: "claude",
			Report:       report.DefaultConfig(),
		},
		state:  stateManager,
		agents: registry,
		logger: logging.NewNop(),
		output: NopOutputNotifier{},
	}

	// Create state with empty prompt (will fail validation)
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-validation-fail",
			Prompt:     "", // Empty - will fail validation
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			CurrentPhase: core.PhaseRefine,
			UpdatedAt:    time.Now(),
		},
	}

	// Call RunWithState - should fail validation
	err := runner.RunWithState(ctx, state)
	if err == nil {
		t.Fatal("RunWithState() should return error for empty prompt")
	}

	// Verify DeactivateWorkflow was called
	if stateManager.deactivateCalls != 1 {
		t.Errorf("DeactivateWorkflow() called %d times, want 1", stateManager.deactivateCalls)
	}

	// Verify status is failed
	if state.Status != core.WorkflowStatusFailed {
		t.Errorf("Status = %v, want %v", state.Status, core.WorkflowStatusFailed)
	}
}

// mockResilienceRegistry implements a minimal AgentRegistry for tests.
type mockResilienceRegistry struct {
	agents []string
}

func (m *mockResilienceRegistry) Register(_ string, _ core.Agent) error {
	return nil
}

func (m *mockResilienceRegistry) Get(name string) (core.Agent, error) {
	for _, a := range m.agents {
		if a == name {
			return &mockResilienceAgent{name: name}, nil
		}
	}
	return nil, core.ErrNotFound("agent", name)
}

func (m *mockResilienceRegistry) List() []string {
	return m.agents
}

func (m *mockResilienceRegistry) ListEnabled() []string {
	return m.agents
}

func (m *mockResilienceRegistry) Available(_ context.Context) []string {
	return m.agents
}

func (m *mockResilienceRegistry) AvailableForPhase(_ context.Context, _ string) []string {
	return m.agents
}

func (m *mockResilienceRegistry) ListEnabledForPhase(_ string) []string {
	return m.agents
}

func (m *mockResilienceRegistry) AvailableForPhaseWithConfig(_ context.Context, _ string, _ map[string][]string) []string {
	return m.agents
}

// mockResilienceAgent implements a minimal Agent for tests.
type mockResilienceAgent struct {
	name string
}

func (m *mockResilienceAgent) Name() string {
	return m.name
}

func (m *mockResilienceAgent) Capabilities() core.Capabilities {
	return core.Capabilities{
		SupportsJSON:      true,
		SupportsStreaming: false,
		MaxContextTokens:  100000,
		MaxOutputTokens:   8192,
	}
}

func (m *mockResilienceAgent) Ping(_ context.Context) error {
	return nil
}

func (m *mockResilienceAgent) Execute(_ context.Context, _ core.ExecuteOptions) (*core.ExecuteResult, error) {
	return &core.ExecuteResult{
		Output:    "mock response",
		TokensIn:  100,
		TokensOut: 50,
		Duration:  time.Millisecond * 100,
	}, nil
}
