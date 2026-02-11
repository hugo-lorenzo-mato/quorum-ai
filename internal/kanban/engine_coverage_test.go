package kanban

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ---------------------------------------------------------------------------
// Mock helpers specific to coverage tests
// ---------------------------------------------------------------------------

// mockProjectStateProvider implements ProjectStateProvider for testing.
type mockProjectStateProvider struct {
	activeProjects  []ProjectInfo
	loadedProjects  []ProjectInfo
	stateManagers   map[string]KanbanStateManager
	eventBuses      map[string]EventPublisher
	execContextErr  error
	listActiveErr   error
	listLoadedErr   error
	getStateErr     error
	getExecCtxError map[string]error // per-project exec context errors
}

func newMockProjectStateProvider() *mockProjectStateProvider {
	return &mockProjectStateProvider{
		stateManagers:   make(map[string]KanbanStateManager),
		eventBuses:      make(map[string]EventPublisher),
		getExecCtxError: make(map[string]error),
	}
}

func (m *mockProjectStateProvider) ListActiveProjects(_ context.Context) ([]ProjectInfo, error) {
	if m.listActiveErr != nil {
		return nil, m.listActiveErr
	}
	return m.activeProjects, nil
}

func (m *mockProjectStateProvider) ListLoadedProjects(_ context.Context) ([]ProjectInfo, error) {
	if m.listLoadedErr != nil {
		return nil, m.listLoadedErr
	}
	return m.loadedProjects, nil
}

func (m *mockProjectStateProvider) GetProjectStateManager(_ context.Context, projectID string) (KanbanStateManager, error) {
	if m.getStateErr != nil {
		return nil, m.getStateErr
	}
	return m.stateManagers[projectID], nil
}

func (m *mockProjectStateProvider) GetProjectEventBus(_ context.Context, projectID string) EventPublisher {
	return m.eventBuses[projectID]
}

func (m *mockProjectStateProvider) GetProjectExecutionContext(ctx context.Context, projectID string) (context.Context, error) {
	if err, ok := m.getExecCtxError[projectID]; ok && err != nil {
		return nil, err
	}
	if m.execContextErr != nil {
		return nil, m.execContextErr
	}
	return ctx, nil
}

// ---------------------------------------------------------------------------
// handleWorkflowEvent tests (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleWorkflowEvent_NoCurrentExecution(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// No current execution set - should ignore event
	ctx := context.Background()
	evt := events.NewWorkflowCompletedEvent("wf-1", "proj-1", time.Second)
	engine.handleWorkflowEvent(ctx, evt)
	// No panic, no side effects
}

func TestHandleWorkflowEvent_WrongWorkflow(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Set current execution to a different workflow
	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-current",
		ProjectID:  "proj-1",
	})

	ctx := context.Background()
	evt := events.NewWorkflowCompletedEvent("wf-other", "proj-1", time.Second)
	engine.handleWorkflowEvent(ctx, evt)

	// Current execution should remain unchanged
	if engine.getCurrentExecution() == nil || engine.getCurrentExecution().WorkflowID != "wf-current" {
		t.Error("current execution should remain unchanged for non-matching workflow")
	}
}

func TestHandleWorkflowEvent_CompletedEvent(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-done"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-done",
		ProjectID:  "default",
	})

	ctx := context.Background()
	evt := events.NewWorkflowCompletedEvent("wf-done", "default", time.Second)
	engine.handleWorkflowEvent(ctx, evt)

	// Workflow should be moved to to_verify
	storedWf := stateMgr.GetWorkflow("wf-done")
	if storedWf.KanbanColumn != "to_verify" {
		t.Errorf("expected to_verify, got %s", storedWf.KanbanColumn)
	}
	// Current execution should be cleared
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared after completed event")
	}
}

func TestHandleWorkflowEvent_FailedEvent(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-err"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-err",
		ProjectID:  "default",
	})

	ctx := context.Background()
	evt := events.NewWorkflowFailedEvent("wf-err", "default", "execution", fmt.Errorf("boom"))
	engine.handleWorkflowEvent(ctx, evt)

	storedWf := stateMgr.GetWorkflow("wf-err")
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement, got %s", storedWf.KanbanColumn)
	}
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared after failed event")
	}
}

func TestHandleWorkflowEvent_FailedEventEmptyError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-empty-err"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-empty-err",
		ProjectID:  "default",
	})

	ctx := context.Background()
	// Create failed event with nil error -> empty Error field
	evt := events.NewWorkflowFailedEvent("wf-empty-err", "default", "execution", nil)
	engine.handleWorkflowEvent(ctx, evt)

	storedWf := stateMgr.GetWorkflow("wf-empty-err")
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement, got %s", storedWf.KanbanColumn)
	}
	// With nil error, the fallback message "workflow failed" should be used
	if storedWf.KanbanLastError != "workflow failed" {
		t.Errorf("expected 'workflow failed', got %q", storedWf.KanbanLastError)
	}
}

// ---------------------------------------------------------------------------
// CurrentProjectID tests (0% coverage)
// ---------------------------------------------------------------------------

func TestCurrentProjectID_NilExecution(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	if engine.CurrentProjectID() != nil {
		t.Error("expected nil project ID when no execution")
	}
}

func TestCurrentProjectID_WithExecution(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-1",
		ProjectID:  "proj-abc",
	})

	pid := engine.CurrentProjectID()
	if pid == nil {
		t.Fatal("expected non-nil project ID")
	}
	if *pid != "proj-abc" {
		t.Errorf("expected proj-abc, got %s", *pid)
	}
}

// ---------------------------------------------------------------------------
// getEngineStateManager tests (25% coverage)
// ---------------------------------------------------------------------------

func TestGetEngineStateManager_LegacyStateManager(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	// Legacy state manager should be preferred
	sm := engine.getEngineStateManager(context.Background())
	if sm == nil {
		t.Fatal("expected non-nil state manager")
	}
}

func TestGetEngineStateManager_FallbackToProjectProvider(t *testing.T) {
	t.Parallel()
	projStateMgr := newMockKanbanStateManager()
	provider := newMockProjectStateProvider()
	provider.activeProjects = []ProjectInfo{{ID: "proj-1", Name: "Project 1"}}
	provider.stateManagers["proj-1"] = projStateMgr

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	// No legacy state manager, should fallback to project provider
	sm := engine.getEngineStateManager(context.Background())
	if sm == nil {
		t.Fatal("expected state manager from project provider fallback")
	}
}

func TestGetEngineStateManager_NoStateManagerNoProvider(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		globalEventBus: events.New(100),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	sm := engine.getEngineStateManager(context.Background())
	if sm != nil {
		t.Error("expected nil when no state manager and no provider")
	}
}

func TestGetEngineStateManager_ProviderNoProjects(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.activeProjects = nil // empty

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	sm := engine.getEngineStateManager(context.Background())
	if sm != nil {
		t.Error("expected nil when provider has no projects")
	}
}

func TestGetEngineStateManager_ProviderListError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.listActiveErr = errors.New("list error")

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	sm := engine.getEngineStateManager(context.Background())
	if sm != nil {
		t.Error("expected nil when provider returns error")
	}
}

// ---------------------------------------------------------------------------
// waitForWorkflowCompletion tests (30% coverage)
// ---------------------------------------------------------------------------

func TestWaitForWorkflowCompletion_ContextCancelled(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-wait",
		ProjectID:  "default",
	})

	ctx, cancel := context.WithCancel(context.Background())
	eventCh := make(chan events.Event, 10)

	// Cancel context immediately to trigger ctx.Done() path
	cancel()

	engine.waitForWorkflowCompletion(ctx, eventCh, "wf-wait")
	// Should return without blocking
}

func TestWaitForWorkflowCompletion_MatchingCompletedEvent(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-wait-done"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-wait-done",
		ProjectID:  "default",
	})

	ctx := context.Background()
	eventCh := make(chan events.Event, 10)

	// Send completed event
	evt := events.NewWorkflowCompletedEvent("wf-wait-done", "default", time.Second)
	eventCh <- evt

	engine.waitForWorkflowCompletion(ctx, eventCh, "wf-wait-done")
	// Should process event and return
}

func TestWaitForWorkflowCompletion_MatchingFailedEvent(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-wait-fail"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-wait-fail",
		ProjectID:  "default",
	})

	ctx := context.Background()
	eventCh := make(chan events.Event, 10)

	// Send failed event
	evt := events.NewWorkflowFailedEvent("wf-wait-fail", "default", "execution", fmt.Errorf("fail"))
	eventCh <- evt

	engine.waitForWorkflowCompletion(ctx, eventCh, "wf-wait-fail")
	// Should process event and return
}

func TestWaitForWorkflowCompletion_NonMatchingEvent(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-target"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-target",
		ProjectID:  "default",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Event, 10)

	// Send event for different workflow, then cancel
	nonMatchEvt := events.NewWorkflowCompletedEvent("wf-other", "default", time.Second)
	eventCh <- nonMatchEvt

	// After non-matching event, context will timeout
	engine.waitForWorkflowCompletion(ctx, eventCh, "wf-target")
	// Should ignore non-matching event and return on context cancel
}

// ---------------------------------------------------------------------------
// startExecutionForProject tests (61.5% coverage)
// ---------------------------------------------------------------------------

func TestStartExecutionForProject_MoveError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.moveErr = errors.New("move failed")
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-move-err",
			Title:      "Test",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			KanbanColumn: "todo",
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	ctx := context.Background()
	engine.startExecutionForProject(ctx, wf, "default", stateMgr)

	// Should not set current execution when move fails
	time.Sleep(20 * time.Millisecond)
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should not be set when move fails")
	}
}

func TestStartExecutionForProject_EmptyKanbanColumn(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)
	exec := &mockWorkflowExecutor{}

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-no-col",
			Title:      "No Column",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			KanbanColumn: "", // empty - should default to "todo"
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	ctx := context.Background()
	engine.startExecutionForProject(ctx, wf, "default", stateMgr)

	// Give goroutine time to run
	time.Sleep(50 * time.Millisecond)

	calls := exec.RunCalls()
	if len(calls) == 0 {
		t.Error("expected executor to be called")
	}
}

func TestStartExecutionForProject_ExecutorError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)
	exec := &mockWorkflowExecutor{
		runResult: errors.New("exec failed"),
	}

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-exec-err",
			Title:      "Exec Error",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			KanbanColumn: "todo",
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     exec,
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	ctx := context.Background()
	engine.startExecutionForProject(ctx, wf, "default", stateMgr)

	// Give goroutine time to run and handle failure
	time.Sleep(100 * time.Millisecond)

	// Should have moved to refinement after failure
	storedWf := stateMgr.GetWorkflow("wf-exec-err")
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement after exec error, got %s", storedWf.KanbanColumn)
	}
}

func TestStartExecutionForProject_ExecContextError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	projStateMgr := newMockKanbanStateManager()
	provider.stateManagers["proj-1"] = projStateMgr
	provider.loadedProjects = []ProjectInfo{{ID: "proj-1"}}
	provider.activeProjects = []ProjectInfo{{ID: "proj-1"}}
	provider.eventBuses["proj-1"] = events.New(10)
	provider.getExecCtxError["proj-1"] = errors.New("context creation failed")

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-ctx-err",
			Title:      "Ctx Error",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			KanbanColumn: "todo",
		},
	}
	projStateMgr.AddWorkflow(wf)

	eventBus := events.New(100)
	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  eventBus,
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	ctx := context.Background()
	engine.startExecutionForProject(ctx, wf, "proj-1", projStateMgr)

	// Give goroutine time to fail on context creation
	time.Sleep(100 * time.Millisecond)

	// Should have handled failure due to context creation error
	storedWf := projStateMgr.GetWorkflow("wf-ctx-err")
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement after context error, got %s", storedWf.KanbanColumn)
	}
}

// ---------------------------------------------------------------------------
// tick edge cases
// ---------------------------------------------------------------------------

func TestTick_ListLoadedProjectsError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.listLoadedErr = errors.New("list failed")

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx) // Should not panic, just log error
}

func TestTick_GetProjectStateManagerError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.loadedProjects = []ProjectInfo{{ID: "proj-err"}}
	provider.getStateErr = errors.New("state manager error")

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx) // Should continue without panic
}

func TestTick_NilStateManager(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.loadedProjects = []ProjectInfo{{ID: "proj-nil"}}
	// stateManagers["proj-nil"] not set -> returns nil

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx) // Should skip nil stateManager
}

func TestTick_GetNextKanbanWorkflowError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	stateMgr := newMockKanbanStateManager()
	stateMgr.getNextErr = errors.New("get next error")
	provider.loadedProjects = []ProjectInfo{{ID: "proj-1"}}
	provider.stateManagers["proj-1"] = stateMgr

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx) // Should log warning and continue
}

func TestTick_NilWorkflow(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	stateMgr := newMockKanbanStateManager()
	// nextWorkflow is nil by default
	provider.loadedProjects = []ProjectInfo{{ID: "proj-1"}}
	provider.stateManagers["proj-1"] = stateMgr

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx) // Should skip empty queue
}

func TestTick_AlreadyExecuting(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	provider := newMockProjectStateProvider()
	stateMgr := newMockKanbanStateManager()
	stateMgr.SetNextWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-queued"},
	})
	provider.loadedProjects = []ProjectInfo{{ID: "proj-1"}}
	provider.stateManagers["proj-1"] = stateMgr

	engine := &Engine{
		executor:        exec,
		projectProvider: provider,
		globalEventBus:  events.New(100),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store(&currentExecution{WorkflowID: "wf-running", ProjectID: "proj-1"})
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx)

	// Should not pick any new workflow
	if len(exec.RunCalls()) > 0 {
		t.Error("should not pick new workflow when already executing")
	}
}

func TestTick_MultipleProjects_FirstHasWorkflow(t *testing.T) {
	t.Parallel()
	exec := &mockWorkflowExecutor{}
	provider := newMockProjectStateProvider()

	sm1 := newMockKanbanStateManager()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-from-proj1",
			Title:      "From Project 1",
		},
		WorkflowRun: core.WorkflowRun{
			KanbanColumn: "todo",
		},
	}
	sm1.AddWorkflow(wf)
	sm1.SetNextWorkflow(wf)

	sm2 := newMockKanbanStateManager()
	sm2.SetNextWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-from-proj2"},
		WorkflowRun:        core.WorkflowRun{KanbanColumn: "todo"},
	})

	provider.loadedProjects = []ProjectInfo{
		{ID: "proj-1"},
		{ID: "proj-2"},
	}
	provider.stateManagers["proj-1"] = sm1
	provider.stateManagers["proj-2"] = sm2
	provider.eventBuses["proj-1"] = events.New(10)

	eventBus := events.New(100)
	engine := &Engine{
		executor:        exec,
		projectProvider: provider,
		globalEventBus:  eventBus,
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.tick(ctx)

	// Give goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	// Should execute from first project only
	calls := exec.RunCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 execution call, got %d", len(calls))
	}
	if calls[0] != "wf-from-proj1" {
		t.Errorf("expected wf-from-proj1, got %s", calls[0])
	}
}

// ---------------------------------------------------------------------------
// ListActiveProjects with nil stateManager (SingleProjectProvider)
// ---------------------------------------------------------------------------

func TestSingleProjectProvider_NilStateManager(t *testing.T) {
	t.Parallel()
	provider := NewSingleProjectProvider(nil, nil)

	projects, err := provider.ListActiveProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects != nil {
		t.Errorf("expected nil when stateManager is nil, got %v", projects)
	}
}

func TestSingleProjectProvider_ListLoadedProjects(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	provider := NewSingleProjectProvider(sm, nil)

	projects, err := provider.ListLoadedProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != "default" {
		t.Errorf("expected default project, got %s", projects[0].ID)
	}
}

func TestSingleProjectProvider_GetProjectExecutionContext(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	provider := NewSingleProjectProvider(sm, nil)

	ctx := context.Background()
	execCtx, err := provider.GetProjectExecutionContext(ctx, "any-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execCtx != ctx {
		t.Error("expected same context back in single-project mode")
	}
}

func TestSingleProjectProvider_GetProjectEventBus(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(10)
	provider := NewSingleProjectProvider(sm, eb)

	result := provider.GetProjectEventBus(context.Background(), "any")
	if result != eb {
		t.Error("expected the same event bus back")
	}
}

func TestSingleProjectProvider_GetProjectStateManager(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	provider := NewSingleProjectProvider(sm, nil)

	result, err := provider.GetProjectStateManager(context.Background(), "any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sm {
		t.Error("expected the same state manager back")
	}
}

// ---------------------------------------------------------------------------
// publishEvent tests
// ---------------------------------------------------------------------------

func TestPublishEvent_NilProjectEventBus(t *testing.T) {
	t.Parallel()
	eventBus := events.New(100)
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Subscribe to global bus to verify event is published there
	ch := eventBus.Subscribe(events.TypeWorkflowCompleted)
	defer eventBus.Unsubscribe(ch)

	evt := events.NewWorkflowCompletedEvent("wf-1", "proj-1", time.Second)
	engine.publishEvent(nil, evt) // nil project bus

	select {
	case received := <-ch:
		if received.WorkflowID() != "wf-1" {
			t.Errorf("expected wf-1, got %s", received.WorkflowID())
		}
	case <-time.After(time.Second):
		t.Error("expected event on global bus")
	}
}

func TestPublishEvent_NilGlobalEventBus(t *testing.T) {
	t.Parallel()
	localBus := events.New(10)
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
		globalEventBus: nil, // nil global bus
	}
	engine.currentExe.Store((*currentExecution)(nil))

	evt := events.NewWorkflowCompletedEvent("wf-1", "proj-1", time.Second)
	// Should not panic with nil global event bus
	engine.publishEvent(localBus, evt)
}

// ---------------------------------------------------------------------------
// handleWorkflowCompletedForProject edge cases
// ---------------------------------------------------------------------------

func TestHandleWorkflowCompletedForProject_StateManagerError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.getStateErr = errors.New("state manager unavailable")
	eventBus := events.New(100)

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  eventBus,
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-1",
		ProjectID:  "proj-1",
	})

	ctx := context.Background()
	engine.handleWorkflowCompletedForProject(ctx, "wf-1", "proj-1")

	// Should clear execution even when state manager is unavailable
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared even on state manager error")
	}
}

func TestHandleWorkflowCompletedForProject_WithPRInfo(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-with-pr"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			KanbanColumn: "in_progress",
			PRURL:        "https://github.com/org/repo/pull/42",
			PRNumber:     42,
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-with-pr",
		ProjectID:  "default",
	})

	ctx := context.Background()
	engine.handleWorkflowCompletedForProject(ctx, "wf-with-pr", "default")

	storedWf := stateMgr.GetWorkflow("wf-with-pr")
	if storedWf.PRURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected PR URL to be preserved, got %s", storedWf.PRURL)
	}
	if storedWf.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", storedWf.PRNumber)
	}
}

func TestHandleWorkflowCompletedForProject_WithBranchNoPR(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-branch"},
		WorkflowRun: core.WorkflowRun{
			Status:         core.WorkflowStatusCompleted,
			KanbanColumn:   "in_progress",
			WorkflowBranch: "quorum/wf-branch",
			PRURL:          "", // No PR
		},
	}
	stateMgr.AddWorkflow(wf)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-branch",
		ProjectID:  "default",
	})

	ctx := context.Background()
	engine.handleWorkflowCompletedForProject(ctx, "wf-branch", "default")

	storedWf := stateMgr.GetWorkflow("wf-branch")
	if storedWf.KanbanColumn != "to_verify" {
		t.Errorf("expected to_verify, got %s", storedWf.KanbanColumn)
	}
}

func TestHandleWorkflowCompletedForProject_LoadByIDError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.loadErr = errors.New("load error")
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-load-err",
		ProjectID:  "default",
	})

	ctx := context.Background()
	engine.handleWorkflowCompletedForProject(ctx, "wf-load-err", "default")

	// Should still complete (with empty PR info) and clear execution
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared even on load error")
	}
}

// ---------------------------------------------------------------------------
// handleWorkflowFailedForProject edge cases
// ---------------------------------------------------------------------------

func TestHandleWorkflowFailedForProject_StateManagerError(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	provider.getStateErr = errors.New("state manager unavailable")
	eventBus := events.New(100)

	engine := &Engine{
		executor:        &mockWorkflowExecutor{},
		projectProvider: provider,
		globalEventBus:  eventBus,
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:          testLogger(),
		tickInterval:    DefaultTickInterval,
		tickerFactory:   time.NewTicker,
	}
	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-fail",
		ProjectID:  "proj-1",
	})
	engine.enabled.Store(true)

	ctx := context.Background()
	engine.handleWorkflowFailedForProject(ctx, "wf-fail", "proj-1", "some error")

	// Should clear execution even when state manager is unavailable
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared even on state manager error")
	}
	// Failure should still be recorded in circuit breaker
	if engine.circuitBreaker.ConsecutiveFailures() < 1 {
		t.Error("failure should be recorded in circuit breaker")
	}
}

func TestHandleWorkflowFailedForProject_UpdateKanbanStatusError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.updateErr = errors.New("update error")
	eventBus := events.New(100)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-update-err"},
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

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-update-err",
		ProjectID:  "default",
	})

	ctx := context.Background()
	engine.handleWorkflowFailedForProject(ctx, "wf-update-err", "default", "fail msg")

	// Should still clear execution
	if engine.getCurrentExecution() != nil {
		t.Error("current execution should be cleared even on update error")
	}
}

// ---------------------------------------------------------------------------
// loadState edge cases
// ---------------------------------------------------------------------------

func TestLoadState_NilStateManager(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		globalEventBus: events.New(100),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	// No state manager, should return nil (no error, use defaults)
	err := engine.loadState(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestLoadState_NilPersistedState(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	// engineState is nil by default

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.loadState(context.Background())
	if err != nil {
		t.Errorf("expected nil error for nil state, got %v", err)
	}
}

func TestLoadState_GetKanbanEngineStateError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.getEngineErr = errors.New("get state error")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.loadState(context.Background())
	if err == nil {
		t.Error("expected error from GetKanbanEngineState")
	}
}

func TestLoadState_WithCurrentWorkflowID(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-in-progress"
	lastFailure := time.Now().Add(-30 * time.Minute)
	stateMgr.engineState = &KanbanEngineState{
		Enabled:             true,
		CurrentWorkflowID:   &wfID,
		ConsecutiveFailures: 1,
		CircuitBreakerOpen:  false,
		LastFailureAt:       &lastFailure,
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.loadState(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !engine.IsEnabled() {
		t.Error("engine should be enabled from persisted state")
	}
	if engine.circuitBreaker.ConsecutiveFailures() != 1 {
		t.Error("circuit breaker failures should be restored")
	}
}

func TestLoadState_WithoutLastFailure(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.engineState = &KanbanEngineState{
		Enabled:             false,
		ConsecutiveFailures: 0,
		CircuitBreakerOpen:  false,
		LastFailureAt:       nil, // no last failure
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.loadState(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// persistState edge cases
// ---------------------------------------------------------------------------

func TestPersistState_NilStateManager(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		globalEventBus: events.New(100),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	err := engine.persistState(context.Background())
	if err != nil {
		t.Errorf("expected nil error for nil state manager, got %v", err)
	}
}

func TestPersistState_SaveError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.saveEngineErr = errors.New("save failed")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.persistState(context.Background())
	if err == nil {
		t.Error("expected error from SaveKanbanEngineState")
	}
}

func TestPersistState_WithLastFailure(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	// Record a failure to set lastFailureAt
	engine.circuitBreaker.RecordFailure()

	err := engine.persistState(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if stateMgr.engineState == nil {
		t.Fatal("expected engine state to be saved")
	}
	if stateMgr.engineState.LastFailureAt == nil {
		t.Error("expected LastFailureAt to be set")
	}
}

// ---------------------------------------------------------------------------
// recoverInterrupted edge cases
// ---------------------------------------------------------------------------

func TestRecoverInterrupted_NilStateManager(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		globalEventBus: events.New(100),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestRecoverInterrupted_GetEngineStateError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.getEngineErr = errors.New("state error")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err == nil {
		t.Error("expected error from GetKanbanEngineState")
	}
}

func TestRecoverInterrupted_NilState(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	// engineState is nil

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoverInterrupted_NilCurrentWorkflowID(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: nil,
	}

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoverInterrupted_LoadByIDError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-load-err"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.loadErr = errors.New("load error")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	// Should not return an error - just warn and clear
	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoverInterrupted_RunningWorkflow(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-running"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			KanbanColumn: "in_progress",
		},
	})

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	storedWf := stateMgr.GetWorkflow(wfID)
	if storedWf.KanbanColumn != "refinement" {
		t.Errorf("expected refinement for interrupted running workflow, got %s", storedWf.KanbanColumn)
	}
}

// ---------------------------------------------------------------------------
// Start error path
// ---------------------------------------------------------------------------

func TestStart_NoProjectProvider(t *testing.T) {
	t.Parallel()
	engine := &Engine{
		executor:       &mockWorkflowExecutor{},
		globalEventBus: events.New(100),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         testLogger(),
		tickInterval:   DefaultTickInterval,
		tickerFactory:  time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	err := engine.Start(context.Background())
	if err == nil {
		t.Error("expected error when no project provider configured")
	}
}

// ---------------------------------------------------------------------------
// Disable with persisted state error
// ---------------------------------------------------------------------------

func TestDisable_PersistError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.saveEngineErr = errors.New("save error")
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.enabled.Store(true)
	err := engine.Disable(context.Background())
	if err == nil {
		t.Error("expected error from Disable when persist fails")
	}
}

func TestEnable_PersistError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.saveEngineErr = errors.New("save error")
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	err := engine.Enable(context.Background())
	if err == nil {
		t.Error("expected error from Enable when persist fails")
	}
}

func TestResetCircuitBreaker_PersistError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.saveEngineErr = errors.New("save error")
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Trip and reset
	engine.circuitBreaker.RecordFailure()
	engine.circuitBreaker.RecordFailure()

	err := engine.ResetCircuitBreaker(context.Background())
	if err == nil {
		t.Error("expected error when persist fails")
	}
}

// ---------------------------------------------------------------------------
// GetState with last failure
// ---------------------------------------------------------------------------

func TestGetState_WithLastFailure(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	engine.circuitBreaker.RecordFailure()
	state := engine.GetState()
	if state.LastFailureAt == nil {
		t.Error("expected LastFailureAt to be set after failure")
	}
	if state.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 failure, got %d", state.ConsecutiveFailures)
	}
}

func TestGetState_WithCurrentExecution(t *testing.T) {
	t.Parallel()
	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: newMockKanbanStateManager(),
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-active",
		ProjectID:  "proj-1",
	})
	engine.enabled.Store(true)

	state := engine.GetState()
	if state.CurrentWorkflowID == nil || *state.CurrentWorkflowID != "wf-active" {
		t.Error("expected current workflow ID in state")
	}
	if !state.Enabled {
		t.Error("expected enabled in state")
	}
}

// ---------------------------------------------------------------------------
// NewEngine with ProjectProvider (no legacy state manager)
// ---------------------------------------------------------------------------

func TestNewEngine_WithProjectProvider(t *testing.T) {
	t.Parallel()
	provider := newMockProjectStateProvider()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:        &mockWorkflowExecutor{},
		ProjectProvider: provider,
		EventBus:        eventBus,
		Logger:          testLogger(),
	})

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.projectProvider != provider {
		t.Error("expected project provider to be set directly")
	}
}

func TestNewEngine_LegacySingleProjectFallback(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	// Should create SingleProjectProvider when only StateManager is set
	if engine.projectProvider == nil {
		t.Error("expected project provider to be created from legacy state manager")
	}
}

// ---------------------------------------------------------------------------
// clearCurrentExecution with persistState error
// ---------------------------------------------------------------------------

func TestClearCurrentExecution_PersistError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	stateMgr.saveEngineErr = errors.New("persist error")
	eventBus := events.New(100)

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     eventBus,
		Logger:       testLogger(),
	})

	engine.currentExe.Store(&currentExecution{
		WorkflowID: "wf-1",
		ProjectID:  "proj-1",
	})

	engine.clearCurrentExecution(context.Background())

	// Should still clear execution even when persist fails
	if engine.getCurrentExecution() != nil {
		t.Error("execution should be cleared even on persist error")
	}
}

// ---------------------------------------------------------------------------
// RecoverInterrupted with UpdateKanbanStatus error paths
// ---------------------------------------------------------------------------

func TestRecoverInterrupted_CompletedUpdateError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-completed-err"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			KanbanColumn: "in_progress",
		},
	})
	stateMgr.updateErr = errors.New("update error")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoverInterrupted_FailedWithCustomError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-failed-custom"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusFailed,
			Error:        "custom error message",
			KanbanColumn: "in_progress",
		},
	})

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	storedWf := stateMgr.GetWorkflow(wfID)
	if storedWf.KanbanLastError != "custom error message" {
		t.Errorf("expected custom error, got %q", storedWf.KanbanLastError)
	}
}

func TestRecoverInterrupted_FailedWithEmptyError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-failed-empty"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusFailed,
			Error:        "", // empty
			KanbanColumn: "in_progress",
		},
	})

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	storedWf := stateMgr.GetWorkflow(wfID)
	if storedWf.KanbanLastError != "interrupted during execution" {
		t.Errorf("expected default error, got %q", storedWf.KanbanLastError)
	}
}

func TestRecoverInterrupted_RunningUpdateError(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()
	wfID := "wf-running-err"
	stateMgr.engineState = &KanbanEngineState{
		CurrentWorkflowID: &wfID,
	}
	stateMgr.AddWorkflow(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: core.WorkflowID(wfID)},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			KanbanColumn: "in_progress",
		},
	})
	stateMgr.updateErr = errors.New("update error")

	engine := NewEngine(EngineConfig{
		Executor:     &mockWorkflowExecutor{},
		StateManager: stateMgr,
		EventBus:     events.New(100),
		Logger:       testLogger(),
	})

	err := engine.recoverInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Disable and Enable without global event bus (nil)
// ---------------------------------------------------------------------------

func TestEnable_NilGlobalEventBus(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	engine := &Engine{
		executor:           &mockWorkflowExecutor{},
		legacyStateManager: stateMgr,
		projectProvider:    NewSingleProjectProvider(stateMgr, nil),
		circuitBreaker:     NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:             testLogger(),
		tickInterval:       DefaultTickInterval,
		tickerFactory:      time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	err := engine.Enable(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !engine.IsEnabled() {
		t.Error("engine should be enabled")
	}
}

func TestDisable_NilGlobalEventBus(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	engine := &Engine{
		executor:           &mockWorkflowExecutor{},
		legacyStateManager: stateMgr,
		projectProvider:    NewSingleProjectProvider(stateMgr, nil),
		circuitBreaker:     NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:             testLogger(),
		tickInterval:       DefaultTickInterval,
		tickerFactory:      time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))
	engine.enabled.Store(true)

	err := engine.Disable(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if engine.IsEnabled() {
		t.Error("engine should be disabled")
	}
}

func TestResetCircuitBreaker_NilGlobalEventBus(t *testing.T) {
	t.Parallel()
	stateMgr := newMockKanbanStateManager()

	engine := &Engine{
		executor:           &mockWorkflowExecutor{},
		legacyStateManager: stateMgr,
		projectProvider:    NewSingleProjectProvider(stateMgr, nil),
		circuitBreaker:     NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:             testLogger(),
		tickInterval:       DefaultTickInterval,
		tickerFactory:      time.NewTicker,
	}
	engine.currentExe.Store((*currentExecution)(nil))

	engine.circuitBreaker.RecordFailure()
	engine.circuitBreaker.RecordFailure()

	err := engine.ResetCircuitBreaker(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if engine.circuitBreaker.IsOpen() {
		t.Error("circuit breaker should be closed after reset")
	}
}

// ---------------------------------------------------------------------------
// Stop with context timeout
// ---------------------------------------------------------------------------

func TestStop_ContextTimeout(t *testing.T) {
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

	// Use an already-cancelled context to simulate timeout
	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Close stopCh but don't wait for doneCh
	// The expired context should cause Stop to return ctx.Err()
	err := engine.Stop(stopCtx)
	// The error might or might not be nil depending on timing, but it should not panic
	_ = err
}
