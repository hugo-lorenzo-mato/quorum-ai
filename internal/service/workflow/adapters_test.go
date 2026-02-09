package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func TestNewCheckpointAdapter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	manager := &service.CheckpointManager{}
	adapter := NewCheckpointAdapter(manager, ctx)

	if adapter == nil {
		t.Fatal("NewCheckpointAdapter() returned nil")
	}
	if adapter.manager != manager {
		t.Error("CheckpointAdapter.manager not set correctly")
	}
	if adapter.ctx != ctx {
		t.Error("CheckpointAdapter.ctx not set correctly")
	}
}

func TestNewRetryAdapter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := service.NewRetryPolicy()
	adapter := NewRetryAdapter(policy, ctx)

	if adapter == nil {
		t.Fatal("NewRetryAdapter() returned nil")
	}
	if adapter.policy != policy {
		t.Error("RetryAdapter.policy not set correctly")
	}
}

func TestRetryAdapter_Execute_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := service.NewRetryPolicy(service.WithMaxAttempts(3))
	adapter := NewRetryAdapter(policy, ctx)

	callCount := 0
	err := adapter.Execute(func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestRetryAdapter_Execute_Retry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := service.NewRetryPolicy(
		service.WithMaxAttempts(3),
		service.WithBaseDelay(1*time.Millisecond),
	)
	adapter := NewRetryAdapter(policy, ctx)

	callCount := 0
	// Use a retryable error (ErrTimeout creates a DomainError with Retryable=true)
	retryableErr := core.ErrTimeout("temporary timeout")

	err := adapter.Execute(func() error {
		callCount++
		if callCount < 3 {
			return retryableErr
		}
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestRetryAdapter_ExecuteWithNotify(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := service.NewRetryPolicy(
		service.WithMaxAttempts(2),
		service.WithBaseDelay(1*time.Millisecond),
	)
	adapter := NewRetryAdapter(policy, ctx)

	callCount := 0
	notifyCount := 0
	// Use a retryable error
	retryableErr := core.ErrTimeout("retry timeout")

	err := adapter.ExecuteWithNotify(
		func() error {
			callCount++
			if callCount < 2 {
				return retryableErr
			}
			return nil
		},
		func(attempt int, err error) {
			notifyCount++
		},
	)

	if err != nil {
		t.Errorf("ExecuteWithNotify() error = %v, want nil", err)
	}
	if notifyCount == 0 {
		t.Error("Notify was never called")
	}
}

func TestNewRateLimiterRegistryAdapter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := service.NewRateLimiterRegistry()
	adapter := NewRateLimiterRegistryAdapter(registry, ctx)

	if adapter == nil {
		t.Fatal("NewRateLimiterRegistryAdapter() returned nil")
	}
	if adapter.registry != registry {
		t.Error("RateLimiterRegistryAdapter.registry not set correctly")
	}
}

func TestRateLimiterRegistryAdapter_Get(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := service.NewRateLimiterRegistry()
	adapter := NewRateLimiterRegistryAdapter(registry, ctx)

	limiter := adapter.Get("claude")
	if limiter == nil {
		t.Fatal("Get() returned nil")
	}

	// Should be able to acquire
	err := limiter.Acquire()
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}
}

func TestNewPromptRendererAdapter(t *testing.T) {
	t.Parallel()
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	if adapter == nil {
		t.Fatal("NewPromptRendererAdapter() returned nil")
	}
	if adapter.renderer != renderer {
		t.Error("PromptRendererAdapter.renderer not set correctly")
	}
}

func TestPromptRendererAdapter_RenderAnalyzeV1(t *testing.T) {
	t.Parallel()
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	params := AnalyzeV1Params{
		Prompt:  "Analyze this codebase",
		Context: "Some context",
	}

	result, err := adapter.RenderAnalyzeV1(params)
	if err != nil {
		t.Errorf("RenderAnalyzeV1() error = %v", err)
	}
	if result == "" {
		t.Error("RenderAnalyzeV1() returned empty string")
	}
}

func TestPromptRendererAdapter_RenderPlanGenerate(t *testing.T) {
	t.Parallel()
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	params := PlanParams{
		Prompt:               "Implement new feature",
		ConsolidatedAnalysis: "Analysis results",
		MaxTasks:             10,
	}

	result, err := adapter.RenderPlanGenerate(params)
	if err != nil {
		t.Errorf("RenderPlanGenerate() error = %v", err)
	}
	if result == "" {
		t.Error("RenderPlanGenerate() returned empty string")
	}
}

func TestPromptRendererAdapter_RenderTaskExecute(t *testing.T) {
	t.Parallel()
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	task := &core.Task{
		ID:          "task-1",
		Name:        "Implement feature",
		Description: "Add new functionality",
	}
	params := TaskExecuteParams{
		Task:    task,
		Context: "Execution context",
	}

	result, err := adapter.RenderTaskExecute(params)
	if err != nil {
		t.Errorf("RenderTaskExecute() error = %v", err)
	}
	if result == "" {
		t.Error("RenderTaskExecute() returned empty string")
	}
}

func TestNewResumePointAdapter(t *testing.T) {
	t.Parallel()
	manager := &service.CheckpointManager{}
	adapter := NewResumePointAdapter(manager)

	if adapter == nil {
		t.Fatal("NewResumePointAdapter() returned nil")
	}
	if adapter.manager != manager {
		t.Error("ResumePointAdapter.manager not set correctly")
	}
}

func TestNewDAGAdapter(t *testing.T) {
	t.Parallel()
	dag := service.NewDAGBuilder()
	adapter := NewDAGAdapter(dag)

	if adapter == nil {
		t.Fatal("NewDAGAdapter() returned nil")
	}
	if adapter.dag != dag {
		t.Error("DAGAdapter.dag not set correctly")
	}
}

func TestDAGAdapter_AddTask(t *testing.T) {
	t.Parallel()
	dag := service.NewDAGBuilder()
	adapter := NewDAGAdapter(dag)

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	err := adapter.AddTask(task)
	if err != nil {
		t.Errorf("AddTask() error = %v", err)
	}
}

func TestDAGAdapter_AddDependency(t *testing.T) {
	t.Parallel()
	dag := service.NewDAGBuilder()
	adapter := NewDAGAdapter(dag)

	task1 := &core.Task{ID: "task-1", Name: "Task 1"}
	task2 := &core.Task{ID: "task-2", Name: "Task 2"}

	_ = adapter.AddTask(task1)
	_ = adapter.AddTask(task2)

	err := adapter.AddDependency("task-2", "task-1")
	if err != nil {
		t.Errorf("AddDependency() error = %v", err)
	}
}

func TestDAGAdapter_Build(t *testing.T) {
	t.Parallel()
	dag := service.NewDAGBuilder()
	adapter := NewDAGAdapter(dag)

	task := &core.Task{ID: "task-1", Name: "Task 1"}
	_ = adapter.AddTask(task)

	state, err := adapter.Build()
	if err != nil {
		t.Errorf("Build() error = %v", err)
	}
	if state == nil {
		t.Error("Build() returned nil state")
	}
}

func TestDAGAdapter_GetReadyTasks(t *testing.T) {
	t.Parallel()
	dag := service.NewDAGBuilder()
	adapter := NewDAGAdapter(dag)

	task1 := &core.Task{ID: "task-1", Name: "Task 1"}
	task2 := &core.Task{ID: "task-2", Name: "Task 2"}

	_ = adapter.AddTask(task1)
	_ = adapter.AddTask(task2)
	_ = adapter.AddDependency("task-2", "task-1")
	_, _ = adapter.Build()

	// With no tasks completed, only task-1 should be ready
	ready := adapter.GetReadyTasks(map[core.TaskID]bool{})
	if len(ready) != 1 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 1", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != "task-1" {
		t.Errorf("GetReadyTasks()[0].ID = %q, want task-1", ready[0].ID)
	}

	// With task-1 completed, task-2 should be ready
	ready = adapter.GetReadyTasks(map[core.TaskID]bool{"task-1": true})
	if len(ready) != 1 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 1", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != "task-2" {
		t.Errorf("GetReadyTasks()[0].ID = %q, want task-2", ready[0].ID)
	}
}

func TestNopOutputNotifier(t *testing.T) {
	t.Parallel()
	notifier := NopOutputNotifier{}

	// These should all be no-ops and not panic
	notifier.PhaseStarted(core.PhaseAnalyze)
	notifier.TaskStarted(&core.Task{})
	notifier.TaskCompleted(&core.Task{}, time.Second)
	notifier.TaskFailed(&core.Task{}, errors.New("test"))
	notifier.TaskSkipped(&core.Task{}, "skip")
	notifier.WorkflowStateUpdated(&core.WorkflowState{})
	notifier.Log("info", "test", "message")
	notifier.AgentEvent("started", "claude", "message", nil)
}
