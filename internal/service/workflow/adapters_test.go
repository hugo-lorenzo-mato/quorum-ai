package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func TestNewConsensusAdapter(t *testing.T) {
	checker := service.NewConsensusChecker(0.7, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	if adapter == nil {
		t.Fatal("NewConsensusAdapter() returned nil")
	}
	if adapter.checker != checker {
		t.Error("ConsensusAdapter.checker not set correctly")
	}
}

func TestConsensusAdapter_Threshold(t *testing.T) {
	checker := service.NewConsensusCheckerWithThresholds(0.8, 0.5, 0.3, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	if got := adapter.Threshold(); got != 0.8 {
		t.Errorf("Threshold() = %v, want 0.8", got)
	}
}

func TestConsensusAdapter_V2Threshold(t *testing.T) {
	checker := service.NewConsensusCheckerWithThresholds(0.8, 0.5, 0.3, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	if got := adapter.V2Threshold(); got != 0.5 {
		t.Errorf("V2Threshold() = %v, want 0.5", got)
	}
}

func TestConsensusAdapter_HumanThreshold(t *testing.T) {
	checker := service.NewConsensusCheckerWithThresholds(0.8, 0.5, 0.3, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	if got := adapter.HumanThreshold(); got != 0.3 {
		t.Errorf("HumanThreshold() = %v, want 0.3", got)
	}
}

func TestConsensusAdapter_Evaluate(t *testing.T) {
	checker := service.NewConsensusChecker(0.7, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	outputs := []AnalysisOutput{
		{
			AgentName:       "claude",
			RawOutput:       "Analysis 1",
			Claims:          []string{"claim1", "claim2"},
			Risks:           []string{"risk1"},
			Recommendations: []string{"rec1"},
		},
		{
			AgentName:       "gemini",
			RawOutput:       "Analysis 2",
			Claims:          []string{"claim1", "claim3"},
			Risks:           []string{"risk1", "risk2"},
			Recommendations: []string{"rec1", "rec2"},
		},
	}

	result := adapter.Evaluate(outputs)

	// Score should be between 0 and 1
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score = %v, want between 0 and 1", result.Score)
	}
}

func TestConsensusAdapter_PreservesFullStructure(t *testing.T) {
	checker := service.NewConsensusChecker(0.8, service.DefaultWeights())
	adapter := NewConsensusAdapter(checker)

	outputs := []AnalysisOutput{
		{
			AgentName: "claude",
			Claims:    []string{"claim1", "claim2"},
			Risks:     []string{"risk1"},
		},
		{
			AgentName: "gemini",
			Claims:    []string{"claim1", "claim3"},
			Risks:     []string{"risk2"},
		},
	}

	result := adapter.Evaluate(outputs)

	// Check CategoryScores are preserved
	if result.CategoryScores == nil {
		t.Error("CategoryScores should not be nil")
	}
	if _, ok := result.CategoryScores["claims"]; !ok {
		t.Error("CategoryScores should include 'claims'")
	}

	// Check Divergences have full detail
	for _, d := range result.Divergences {
		if d.Agent1Items == nil && d.Agent2Items == nil {
			t.Error("Divergence should have agent items")
		}
		if d.JaccardScore < 0 || d.JaccardScore > 1 {
			t.Errorf("JaccardScore should be 0-1, got %f", d.JaccardScore)
		}
	}
}

func TestConsensusResult_DivergenceStrings(t *testing.T) {
	result := ConsensusResult{
		Divergences: []Divergence{
			{
				Category: "claims",
				Agent1:   "claude",
				Agent2:   "gemini",
			},
		},
	}

	strings := result.DivergenceStrings()

	if len(strings) != 1 {
		t.Errorf("Expected 1 string, got %d", len(strings))
	}
	if strings[0] != "claims: claude vs gemini" {
		t.Errorf("Unexpected string: %s", strings[0])
	}
}

func TestConsensusAdapter_Evaluate_EmptyOutputs(t *testing.T) {
	checker := service.NewConsensusChecker(0.7, service.CategoryWeights{})
	adapter := NewConsensusAdapter(checker)

	result := adapter.Evaluate([]AnalysisOutput{})

	// Empty outputs should still work
	if result.Score < 0 {
		t.Errorf("Score = %v, want >= 0", result.Score)
	}
}

func TestNewCheckpointAdapter(t *testing.T) {
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

func TestPromptRendererAdapter_RenderAnalyzeV2(t *testing.T) {
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	params := AnalyzeV2Params{
		Prompt:     "Analyze this codebase",
		V1Analysis: "Initial analysis result",
		AgentName:  "claude",
	}

	result, err := adapter.RenderAnalyzeV2(params)
	if err != nil {
		t.Errorf("RenderAnalyzeV2() error = %v", err)
	}
	if result == "" {
		t.Error("RenderAnalyzeV2() returned empty string")
	}
}

func TestPromptRendererAdapter_RenderAnalyzeV3(t *testing.T) {
	renderer, err := service.NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	adapter := NewPromptRendererAdapter(renderer)
	params := AnalyzeV3Params{
		Prompt:      "Analyze this codebase",
		V1Analysis:  "V1 analysis",
		V2Analysis:  "V2 analysis",
		Divergences: []string{"security: agent1 vs agent2"},
	}

	result, err := adapter.RenderAnalyzeV3(params)
	if err != nil {
		t.Errorf("RenderAnalyzeV3() error = %v", err)
	}
	if result == "" {
		t.Error("RenderAnalyzeV3() returned empty string")
	}
}

func TestPromptRendererAdapter_RenderPlanGenerate(t *testing.T) {
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
	notifier := NopOutputNotifier{}

	// These should all be no-ops and not panic
	notifier.PhaseStarted(core.PhaseAnalyze)
	notifier.TaskStarted(&core.Task{})
	notifier.TaskCompleted(&core.Task{}, time.Second)
	notifier.TaskFailed(&core.Task{}, errors.New("test"))
	notifier.TaskSkipped(&core.Task{}, "skip")
	notifier.WorkflowStateUpdated(&core.WorkflowState{})
}
