package kanban

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// mockWorkflowExecutor is a test double for WorkflowExecutor.
type mockWorkflowExecutor struct {
	mu        sync.Mutex
	runCalls  []core.WorkflowID
	runResult error
	runDelay  time.Duration
	onRun     func(wfID core.WorkflowID)
}

func (m *mockWorkflowExecutor) Run(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	m.runCalls = append(m.runCalls, workflowID)
	delay := m.runDelay
	result := m.runResult
	onRun := m.onRun
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if onRun != nil {
		onRun(workflowID)
	}
	return result
}

func (m *mockWorkflowExecutor) RunCalls() []core.WorkflowID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]core.WorkflowID{}, m.runCalls...)
}

// mockKanbanStateManager is a test double for KanbanStateManager.
type mockKanbanStateManager struct {
	mu              sync.Mutex
	workflows       map[string]*core.WorkflowState
	nextWorkflow    *core.WorkflowState
	engineState     *KanbanEngineState
	moveErr         error
	updateErr       error
	loadErr         error
	getNextErr      error
	getEngineErr    error
	saveEngineErr   error
	moveCallCount   int
	updateCallCount int
}

func newMockKanbanStateManager() *mockKanbanStateManager {
	return &mockKanbanStateManager{
		workflows: make(map[string]*core.WorkflowState),
	}
}

func (m *mockKanbanStateManager) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.workflows[string(id)], nil
}

func (m *mockKanbanStateManager) GetNextKanbanWorkflow(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getNextErr != nil {
		return nil, m.getNextErr
	}
	wf := m.nextWorkflow
	m.nextWorkflow = nil // Consume it
	return wf, nil
}

func (m *mockKanbanStateManager) MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.moveCallCount++
	if m.moveErr != nil {
		return m.moveErr
	}
	if wf, ok := m.workflows[workflowID]; ok {
		wf.KanbanColumn = toColumn
		wf.KanbanPosition = position
	}
	return nil
}

func (m *mockKanbanStateManager) UpdateKanbanStatus(ctx context.Context, workflowID, column, prURL string, prNumber int, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCallCount++
	if m.updateErr != nil {
		return m.updateErr
	}
	if wf, ok := m.workflows[workflowID]; ok {
		wf.KanbanColumn = column
		wf.PRURL = prURL
		wf.PRNumber = prNumber
		wf.KanbanLastError = lastError
	}
	return nil
}

func (m *mockKanbanStateManager) GetKanbanEngineState(ctx context.Context) (*KanbanEngineState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getEngineErr != nil {
		return nil, m.getEngineErr
	}
	return m.engineState, nil
}

func (m *mockKanbanStateManager) SaveKanbanEngineState(ctx context.Context, state *KanbanEngineState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveEngineErr != nil {
		return m.saveEngineErr
	}
	m.engineState = state
	return nil
}

func (m *mockKanbanStateManager) AddWorkflow(wf *core.WorkflowState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflows[string(wf.WorkflowID)] = wf
}

func (m *mockKanbanStateManager) SetNextWorkflow(wf *core.WorkflowState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextWorkflow = wf
}

func (m *mockKanbanStateManager) MoveCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.moveCallCount
}

func (m *mockKanbanStateManager) UpdateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCallCount
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewEngine(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	cfg := EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	}

	engine := NewEngine(cfg)

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.IsEnabled() {
		t.Error("new engine should be disabled")
	}
	if engine.CurrentWorkflowID() != nil {
		t.Error("new engine should have no current workflow")
	}
}

func TestNewEngine_CustomTickInterval(t *testing.T) {
	t.Parallel()
	cfg := EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
		TickInterval: 1 * time.Second,
	}

	engine := NewEngine(cfg)
	if engine.tickInterval != 1*time.Second {
		t.Errorf("expected tick interval 1s, got %v", engine.tickInterval)
	}
}

func TestNewEngine_DefaultTickInterval(t *testing.T) {
	t.Parallel()
	cfg := EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
		TickInterval: 0, // Should use default
	}

	engine := NewEngine(cfg)
	if engine.tickInterval != DefaultTickInterval {
		t.Errorf("expected default tick interval %v, got %v", DefaultTickInterval, engine.tickInterval)
	}
}

func TestEngine_EnableDisable(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Enable
	ctx := context.Background()
	if err := engine.Enable(ctx); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}
	if !engine.IsEnabled() {
		t.Error("engine should be enabled")
	}

	// Check state persisted
	if stateMgr.engineState == nil || !stateMgr.engineState.Enabled {
		t.Error("enabled state should be persisted")
	}

	// Disable
	if err := engine.Disable(ctx); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	if engine.IsEnabled() {
		t.Error("engine should be disabled")
	}
}

func TestEngine_EnableWithOpenCircuitBreaker(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	// Trip the circuit breaker
	engine.circuitBreaker.RecordFailure()
	engine.circuitBreaker.RecordFailure()

	// Try to enable
	ctx := context.Background()
	err := engine.Enable(ctx)
	if err == nil {
		t.Error("expected error when enabling with open circuit breaker")
	}
	if engine.IsEnabled() {
		t.Error("engine should still be disabled")
	}
}

func TestEngine_ResetCircuitBreaker(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Trip the breaker
	engine.circuitBreaker.RecordFailure()
	engine.circuitBreaker.RecordFailure()
	if !engine.circuitBreaker.IsOpen() {
		t.Fatal("circuit breaker should be open")
	}

	// Reset
	ctx := context.Background()
	if err := engine.ResetCircuitBreaker(ctx); err != nil {
		t.Fatalf("ResetCircuitBreaker failed: %v", err)
	}

	if engine.circuitBreaker.IsOpen() {
		t.Error("circuit breaker should be closed after reset")
	}
}

func TestEngine_GetState(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	state := engine.GetState()

	if state.Enabled {
		t.Error("new engine should be disabled")
	}
	if state.CurrentWorkflowID != nil {
		t.Error("new engine should have no current workflow")
	}
	if state.ConsecutiveFailures != 0 {
		t.Error("new engine should have 0 consecutive failures")
	}
	if state.CircuitBreakerOpen {
		t.Error("new engine should have closed circuit breaker")
	}
}

func TestEngine_LoadPersistedState(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-123"
	lastFailure := time.Now().Add(-time.Hour)
	stateMgr.engineState = &KanbanEngineState{
		Enabled:             true,
		CurrentWorkflowID:   &wfID,
		ConsecutiveFailures: 1,
		CircuitBreakerOpen:  false,
		LastFailureAt:       &lastFailure,
	}

	// Also add the workflow so recovery doesn't fail
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun:        core.WorkflowRun{Status: core.WorkflowStatusRunning},
	})

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	// Check state was loaded
	if !engine.IsEnabled() {
		t.Error("persisted enabled state should be loaded")
	}
	if engine.circuitBreaker.ConsecutiveFailures() != 1 {
		t.Error("persisted failure count should be loaded")
	}
}

func TestEngine_Tick_PicksWorkflow(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	// Add workflow to queue
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-todo-1",
			Title:      "Test Workflow",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			KanbanColumn: "todo",
		},
	}
	stateMgr.AddWorkflow(wf)
	stateMgr.SetNextWorkflow(wf)

	// Create engine with short tick interval
	engine := NewEngine(EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
		TickInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()

	// Enable and start
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	if err := engine.Enable(ctx); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	// Wait for tick to pick up workflow
	time.Sleep(50 * time.Millisecond)

	// Check workflow was picked
	calls := exec.RunCalls()
	if len(calls) == 0 {
		t.Error("expected workflow to be executed")
	} else if calls[0] != "wf-todo-1" {
		t.Errorf("expected wf-todo-1, got %s", calls[0])
	}

	// Check workflow moved to in_progress
	if stateMgr.MoveCallCount() == 0 {
		t.Error("workflow should have been moved")
	}
}

func TestEngine_Tick_DoesNotPickWhenDisabled(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	stateMgr := newMockKanbanStateManager()

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-should-not-run"},
		WorkflowRun:        core.WorkflowRun{KanbanColumn: "todo"},
	}
	stateMgr.AddWorkflow(wf)
	stateMgr.SetNextWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
		TickInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	// Don't enable engine
	time.Sleep(50 * time.Millisecond)

	calls := exec.RunCalls()
	if len(calls) > 0 {
		t.Error("workflow should not be executed when engine is disabled")
	}
}

func TestEngine_Tick_DoesNotPickWithOpenCircuitBreaker(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	stateMgr := newMockKanbanStateManager()

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-blocked"},
		WorkflowRun:        core.WorkflowRun{KanbanColumn: "todo"},
	}
	stateMgr.AddWorkflow(wf)
	stateMgr.SetNextWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
		TickInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	// Enable but trip circuit breaker
	engine.enabled.Store(true)
	engine.circuitBreaker.RecordFailure()
	engine.circuitBreaker.RecordFailure()

	time.Sleep(50 * time.Millisecond)

	calls := exec.RunCalls()
	if len(calls) > 0 {
		t.Error("workflow should not be executed when circuit breaker is open")
	}
}

func TestEngine_HandleWorkflowCompleted(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-complete"},
		WorkflowRun: core.WorkflowRun{
			Status:         core.WorkflowStatusRunning,
			KanbanColumn:   "in_progress",
			WorkflowBranch: "quorum/wf-complete",
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Set as current workflow with project context
	wfID := "wf-complete"
	projectID := "default"
	engine.currentExe.Store(&currentExecution{
		WorkflowID: wfID,
		ProjectID:  projectID,
	})

	// Handle completion
	ctx := context.Background()
	engine.handleWorkflowCompletedForProject(ctx, wfID, projectID)

	// Verify workflow moved to to_verify
	if stateMgr.UpdateCallCount() == 0 {
		t.Error("expected UpdateKanbanStatus to be called")
	}
	storedWf := stateMgr.workflows[wfID]
	if storedWf.KanbanColumn != "to_verify" {
		t.Errorf("expected to_verify column, got %s", storedWf.KanbanColumn)
	}

	// Current workflow should be cleared
	if engine.CurrentWorkflowID() != nil {
		t.Error("current workflow should be cleared after completion")
	}
}

func TestEngine_HandleWorkflowFailed(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-fail"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			KanbanColumn: "in_progress",
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	wfID := "wf-fail"
	projectID := "default"
	engine.currentExe.Store(&currentExecution{
		WorkflowID: wfID,
		ProjectID:  projectID,
	})

	ctx := context.Background()
	engine.handleWorkflowFailedForProject(ctx, wfID, projectID, "test error")

	// Verify workflow moved to refinement
	storedWf := stateMgr.workflows[wfID]
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement column, got %s", storedWf.KanbanColumn)
	}
	if storedWf.KanbanLastError != "test error" {
		t.Errorf("expected error to be stored, got %s", storedWf.KanbanLastError)
	}

	// Failure should be recorded
	if engine.circuitBreaker.ConsecutiveFailures() != 1 {
		t.Error("failure should be recorded in circuit breaker")
	}
}

func TestEngine_CircuitBreakerTrips(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-trip"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			KanbanColumn: "in_progress",
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Pre-record one failure
	engine.circuitBreaker.RecordFailure()

	// Enable engine
	ctx := context.Background()
	engine.enabled.Store(true)

	wfID := "wf-trip"
	projectID := "default"
	engine.currentExe.Store(&currentExecution{
		WorkflowID: wfID,
		ProjectID:  projectID,
	})

	// This failure should trip the breaker
	engine.handleWorkflowFailedForProject(ctx, wfID, projectID, "second failure")

	if !engine.circuitBreaker.IsOpen() {
		t.Error("circuit breaker should be open")
	}
	if engine.IsEnabled() {
		t.Error("engine should be disabled after circuit breaker trips")
	}
}

func TestEngine_RecoverInterrupted_Completed(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	wfID := "wf-recovered"
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			KanbanColumn: "in_progress",
		},
	}
	stateMgr.AddWorkflow(wf)
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	// Wait for recovery
	time.Sleep(10 * time.Millisecond)

	// Workflow should have been moved to to_verify
	storedWf := stateMgr.workflows[wfID]
	if storedWf.KanbanColumn != "to_verify" {
		t.Errorf("expected to_verify after recovery, got %s", storedWf.KanbanColumn)
	}
}

func TestEngine_RecoverInterrupted_Failed(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	wfID := "wf-failed-recovery"
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusFailed,
			Error:        "previous error",
			KanbanColumn: "in_progress",
		},
	}
	stateMgr.AddWorkflow(wf)
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	time.Sleep(10 * time.Millisecond)

	storedWf := stateMgr.workflows[wfID]
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement after failed recovery, got %s", storedWf.KanbanColumn)
	}
}

func TestEngine_RecoverInterrupted_Deleted(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	wfID := "wf-deleted"
	// Workflow NOT added - simulates deletion
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		engine.Stop(stopCtx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Current workflow should be cleared
	if engine.CurrentWorkflowID() != nil {
		t.Error("current workflow should be cleared for deleted workflow")
	}
}

func TestEngine_GracefulStop(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
		TickInterval: 10 * time.Millisecond,
	})

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := engine.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
