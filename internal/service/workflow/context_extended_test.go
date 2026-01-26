package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestBuildContextString_EmptyState(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowID:   "",
		CurrentPhase: "",
		Tasks:        nil,
		TaskOrder:    nil,
	}

	result := BuildContextString(state)
	if result == "" {
		t.Error("BuildContextString should return something even for empty state")
	}
}

func TestBuildContextString_NilTask(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": nil, // nil task should be handled
		},
		TaskOrder: []core.TaskID{"task-1"},
	}

	// Should not panic
	result := BuildContextString(state)
	if result == "" {
		t.Error("BuildContextString should return something")
	}
}

func TestBuildContextString_MixedTaskStatuses(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Completed Task", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Name: "Running Task", Status: core.TaskStatusRunning},
			"task-3": {ID: "task-3", Name: "Pending Task", Status: core.TaskStatusPending},
			"task-4": {ID: "task-4", Name: "Failed Task", Status: core.TaskStatusFailed},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2", "task-3", "task-4"},
	}

	result := BuildContextString(state)

	// Should contain completed task
	if !containsSubstring(result, "Completed Task") {
		t.Error("should contain completed task name")
	}

	// Should not contain running task (only completed are shown)
	if containsSubstring(result, "Running Task") {
		t.Error("should not contain running task name")
	}
}

func TestResolvePhaseModel_NilConfig(t *testing.T) {
	result := ResolvePhaseModel(nil, "claude", core.PhaseAnalyze, "")
	if result != "" {
		t.Errorf("ResolvePhaseModel(nil) = %q, want empty", result)
	}
}

func TestResolvePhaseModel_NilAgentPhaseModels(t *testing.T) {
	cfg := &Config{
		AgentPhaseModels: nil,
	}

	result := ResolvePhaseModel(cfg, "claude", core.PhaseAnalyze, "")
	if result != "" {
		t.Errorf("ResolvePhaseModel with nil AgentPhaseModels = %q, want empty", result)
	}
}

func TestResolvePhaseModel_UnknownAgent(t *testing.T) {
	cfg := &Config{
		AgentPhaseModels: map[string]map[string]string{
			"claude": {"analyze": "opus"},
		},
	}

	result := ResolvePhaseModel(cfg, "unknown", core.PhaseAnalyze, "")
	if result != "" {
		t.Errorf("ResolvePhaseModel for unknown agent = %q, want empty", result)
	}
}

func TestResolvePhaseModel_WhitespaceTaskModel(t *testing.T) {
	cfg := &Config{
		AgentPhaseModels: map[string]map[string]string{
			"claude": {"analyze": "opus"},
		},
	}

	// Whitespace-only task model should be treated as empty
	result := ResolvePhaseModel(cfg, "claude", core.PhaseAnalyze, "   ")
	if result != "opus" {
		t.Errorf("ResolvePhaseModel with whitespace task model = %q, want 'opus'", result)
	}
}

func TestResolvePhaseModel_WhitespacePhaseModel(t *testing.T) {
	cfg := &Config{
		AgentPhaseModels: map[string]map[string]string{
			"claude": {"analyze": "   "}, // whitespace-only model
		},
	}

	result := ResolvePhaseModel(cfg, "claude", core.PhaseAnalyze, "")
	if result != "" {
		t.Errorf("ResolvePhaseModel with whitespace phase model = %q, want empty", result)
	}
}

func TestConfig_AllFields(t *testing.T) {
	cfg := &Config{
		DryRun:       true,
		Sandbox:      false,
		DenyTools:    []string{"rm", "sudo"},
		DefaultAgent: "gemini",
		AgentPhaseModels: map[string]map[string]string{
			"claude": {"analyze": "opus", "plan": "sonnet"},
			"gemini": {"analyze": "pro"},
		},
		WorktreeAutoClean: true,
		WorktreeMode:      "parallel",
		Moderator: ModeratorConfig{
			Enabled:   true,
			Agent:     "claude",
			Threshold: 0.90,
			MinRounds: 2,
			MaxRounds: 3,
		},
	}

	if !cfg.DryRun {
		t.Error("DryRun should be true")
	}
	if cfg.Sandbox {
		t.Error("Sandbox should be false")
	}
	if len(cfg.DenyTools) != 2 {
		t.Errorf("len(DenyTools) = %d, want 2", len(cfg.DenyTools))
	}
	if cfg.DefaultAgent != "gemini" {
		t.Errorf("DefaultAgent = %q, want gemini", cfg.DefaultAgent)
	}
	if len(cfg.AgentPhaseModels) != 2 {
		t.Errorf("len(AgentPhaseModels) = %d, want 2", len(cfg.AgentPhaseModels))
	}
	if len(cfg.AgentPhaseModels["claude"]) != 2 {
		t.Errorf("len(AgentPhaseModels[claude]) = %d, want 2", len(cfg.AgentPhaseModels["claude"]))
	}
	if !cfg.WorktreeAutoClean {
		t.Error("WorktreeAutoClean should be true")
	}
	if cfg.WorktreeMode != "parallel" {
		t.Errorf("WorktreeMode = %q, want parallel", cfg.WorktreeMode)
	}
	if !cfg.Moderator.Enabled {
		t.Error("Moderator.Enabled should be true")
	}
	if cfg.Moderator.Agent != "claude" {
		t.Errorf("Moderator.Agent = %q, want claude", cfg.Moderator.Agent)
	}
}

func TestAnalyzeV1Params_Fields(t *testing.T) {
	params := AnalyzeV1Params{
		Prompt:  "Test prompt",
		Context: "Test context",
	}

	if params.Prompt != "Test prompt" {
		t.Errorf("Prompt = %q, want %q", params.Prompt, "Test prompt")
	}
	if params.Context != "Test context" {
		t.Errorf("Context = %q, want %q", params.Context, "Test context")
	}
}

func TestPlanParams_Fields(t *testing.T) {
	params := PlanParams{
		Prompt:               "Test prompt",
		ConsolidatedAnalysis: "Consolidated analysis",
		MaxTasks:             10,
	}

	if params.Prompt != "Test prompt" {
		t.Errorf("Prompt = %q, want %q", params.Prompt, "Test prompt")
	}
	if params.ConsolidatedAnalysis != "Consolidated analysis" {
		t.Errorf("ConsolidatedAnalysis = %q, want %q", params.ConsolidatedAnalysis, "Consolidated analysis")
	}
	if params.MaxTasks != 10 {
		t.Errorf("MaxTasks = %d, want 10", params.MaxTasks)
	}
}

func TestTaskExecuteParams_Fields(t *testing.T) {
	task := &core.Task{
		ID:          "task-1",
		Name:        "Test Task",
		Description: "A test task",
	}
	params := TaskExecuteParams{
		Task:    task,
		Context: "Execution context",
	}

	if params.Task != task {
		t.Error("Task not set correctly")
	}
	if params.Context != "Execution context" {
		t.Errorf("Context = %q, want %q", params.Context, "Execution context")
	}
}

func TestNopOutputNotifier_AllMethods(t *testing.T) {
	notifier := NopOutputNotifier{}
	task := &core.Task{ID: "test"}
	state := &core.WorkflowState{}

	// These should all be no-ops and not panic
	notifier.PhaseStarted(core.PhaseAnalyze)
	notifier.PhaseStarted(core.PhasePlan)
	notifier.PhaseStarted(core.PhaseExecute)
	notifier.TaskStarted(task)
	notifier.TaskStarted(nil)
	notifier.TaskCompleted(task, 0)
	notifier.TaskCompleted(nil, 0)
	notifier.TaskFailed(task, nil)
	notifier.TaskFailed(nil, nil)
	notifier.TaskSkipped(task, "skip")
	notifier.TaskSkipped(nil, "")
	notifier.WorkflowStateUpdated(state)
	notifier.WorkflowStateUpdated(nil)
	notifier.Log("info", "source", "message")
	notifier.Log("error", "", "")
	notifier.AgentEvent("started", "claude", "message", nil)
	notifier.AgentEvent("tool_use", "gemini", "Using tool", map[string]interface{}{"tool": "test"})

	// Test passes if no panic occurred
}
