package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockWorktreeManager implements WorktreeManager for tests.
type mockWorktreeManager struct {
	createInfo *core.WorktreeInfo
	createErr  error
	removeErr  error
}

func (m *mockWorktreeManager) Create(_ context.Context, _ *core.Task, _ string) (*core.WorktreeInfo, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createInfo, nil
}

func (m *mockWorktreeManager) CreateFromBranch(_ context.Context, _ *core.Task, _, _ string) (*core.WorktreeInfo, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createInfo, nil
}

func (m *mockWorktreeManager) Get(_ context.Context, _ *core.Task) (*core.WorktreeInfo, error) {
	return m.createInfo, nil
}

func (m *mockWorktreeManager) Remove(_ context.Context, _ *core.Task) error {
	return m.removeErr
}

func (m *mockWorktreeManager) CleanupStale(_ context.Context) error {
	return nil
}

func (m *mockWorktreeManager) List(_ context.Context) ([]*core.WorktreeInfo, error) {
	return nil, nil
}

// mockOutputNotifier tracks calls to output methods.
type mockOutputNotifier struct {
	phaseStarted  []core.Phase
	taskStarted   []*core.Task
	taskCompleted []*core.Task
	taskFailed    []*core.Task
	taskSkipped   []*core.Task
	stateUpdated  int
}

func (m *mockOutputNotifier) PhaseStarted(phase core.Phase) {
	m.phaseStarted = append(m.phaseStarted, phase)
}

func (m *mockOutputNotifier) TaskStarted(task *core.Task) {
	m.taskStarted = append(m.taskStarted, task)
}

func (m *mockOutputNotifier) TaskCompleted(task *core.Task, _ time.Duration) {
	m.taskCompleted = append(m.taskCompleted, task)
}

func (m *mockOutputNotifier) TaskFailed(task *core.Task, _ error) {
	m.taskFailed = append(m.taskFailed, task)
}

func (m *mockOutputNotifier) TaskSkipped(task *core.Task, _ string) {
	m.taskSkipped = append(m.taskSkipped, task)
}

func (m *mockOutputNotifier) WorkflowStateUpdated(_ *core.WorkflowState) {
	m.stateUpdated++
}

func (m *mockOutputNotifier) Log(_, _, _ string) {}

func (m *mockOutputNotifier) AgentEvent(_, _, _ string, _ map[string]interface{}) {}

func TestExecutor_Run_AllTasksCompleted(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    "success",
			TokensIn:  100,
			TokensOut: 200,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	output := &mockOutputNotifier{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Output:     output,
		Config: &Config{
			DryRun:       false,
			Sandbox:      true,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if wctx.State.Tasks["task-1"].Status != core.TaskStatusCompleted {
		t.Errorf("task status = %v, want completed", wctx.State.Tasks["task-1"].Status)
	}

	if len(output.taskStarted) != 1 {
		t.Errorf("TaskStarted called %d times, want 1", len(output.taskStarted))
	}
	if len(output.taskCompleted) != 1 {
		t.Errorf("TaskCompleted called %d times, want 1", len(output.taskCompleted))
	}
}

func TestExecutor_Run_DryRunMode(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       true, // Enable dry-run mode
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if wctx.State.Tasks["task-1"].Status != core.TaskStatusCompleted {
		t.Errorf("task status = %v, want completed", wctx.State.Tasks["task-1"].Status)
	}
}

func TestExecutor_Run_NoReadyTasks(t *testing.T) {
	// Create a DAG with no tasks registered but tasks in state
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	dag.Build()

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when no ready tasks")
	}
}

func TestExecutor_Run_AgentExecutionFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		err: errors.New("agent execution failed"),
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when agent execution fails")
	}

	if wctx.State.Tasks["task-1"].Status != core.TaskStatusFailed {
		t.Errorf("task status = %v, want failed", wctx.State.Tasks["task-1"].Status)
	}
}

func TestExecutor_Run_WithWorktrees(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    "success",
			TokensIn:  100,
			TokensOut: 200,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	worktreeMgr := &mockWorktreeManager{
		createInfo: &core.WorktreeInfo{Path: "/tmp/worktree"},
	}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Worktrees:  worktreeMgr,
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:            false,
			DefaultAgent:      "claude",
			WorktreeMode:      "always",
			WorktreeAutoClean: true,
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if wctx.State.Tasks["task-1"].WorktreePath != "/tmp/worktree" {
		t.Errorf("task worktree path = %q, want /tmp/worktree", wctx.State.Tasks["task-1"].WorktreePath)
	}
}

func TestExecutor_Run_WorktreeCreateFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    "success",
			TokensIn:  100,
			TokensOut: 200,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	worktreeMgr := &mockWorktreeManager{
		createErr: errors.New("worktree create failed"),
	}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Worktrees:  worktreeMgr,
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "always",
		},
	}

	// Should still succeed, just log warning
	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil (should continue despite worktree error)", err)
	}
}

func TestExecutor_Run_RateLimitFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	failingLimiter := &mockRateLimiter{acquireErr: errors.New("rate limit failed")}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{limiter: failingLimiter},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when rate limit fails")
	}
}

func TestExecutor_Run_PromptRenderFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	failingPrompts := &mockPromptRenderer{
		taskErr: errors.New("prompt render failed"),
	}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    failingPrompts,
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when prompt render fails")
	}
}

func TestExecutor_Run_AgentNotFound(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "unknown-agent"}
	dag.AddTask(task)
	dag.Build()

	registry := &mockAgentRegistry{}
	// Not registering any agent

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending, CLI: "unknown-agent"},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when agent not found")
	}
}

func TestExecutor_Run_ParallelTasks(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	// Add two independent tasks
	task1 := &core.Task{ID: "task-1", Name: "Test 1", CLI: "claude"}
	task2 := &core.Task{ID: "task-2", Name: "Test 2", CLI: "claude"}
	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    "success",
			TokensIn:  100,
			TokensOut: 200,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test 1", Status: core.TaskStatusPending},
					"task-2": {ID: "task-2", Name: "Test 2", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1", "task-2"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "parallel", // Enable parallel execution
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Both tasks should be completed
	if wctx.State.Tasks["task-1"].Status != core.TaskStatusCompleted {
		t.Errorf("task-1 status = %v, want completed", wctx.State.Tasks["task-1"].Status)
	}
	if wctx.State.Tasks["task-2"].Status != core.TaskStatusCompleted {
		t.Errorf("task-2 status = %v, want completed", wctx.State.Tasks["task-2"].Status)
	}
}

func TestExecutor_Run_SkipsAlreadyCompletedTasks(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	// Task already completed
	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusCompleted}, // Already done
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
}

func TestExecutor_Run_UsesDefaultAgentWhenNoCLI(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: ""} // No CLI specified
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{Output: "success", TokensOut: 200},
	}
	registry := &mockAgentRegistry{}
	registry.Register("gemini", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending, CLI: ""},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "gemini", // Default agent
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
}

func TestExecutor_Run_UpdatesMetrics(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    "success",
			TokensIn:  100,
			TokensOut: 200,
			CostUSD:   1.5,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if wctx.State.Metrics.TotalTokensIn != 100 {
		t.Errorf("TotalTokensIn = %d, want 100", wctx.State.Metrics.TotalTokensIn)
	}
	if wctx.State.Metrics.TotalTokensOut != 200 {
		t.Errorf("TotalTokensOut = %d, want 200", wctx.State.Metrics.TotalTokensOut)
	}
	if wctx.State.Metrics.TotalCostUSD != 1.5 {
		t.Errorf("TotalCostUSD = %v, want 1.5", wctx.State.Metrics.TotalCostUSD)
	}

	// Check task metrics
	taskState := wctx.State.Tasks["task-1"]
	if taskState.TokensIn != 100 {
		t.Errorf("task TokensIn = %d, want 100", taskState.TokensIn)
	}
	if taskState.TokensOut != 200 {
		t.Errorf("task TokensOut = %d, want 200", taskState.TokensOut)
	}
	if taskState.CostUSD != 1.5 {
		t.Errorf("task CostUSD = %v, want 1.5", taskState.CostUSD)
	}
}

func TestExecutor_Run_SaveStateFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{err: errors.New("save failed")}
	executor := NewExecutor(dag, saver, nil)

	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	dag.AddTask(task)
	dag.Build()

	agent := &mockAgent{
		result: &core.ExecuteResult{Output: "success", TokensOut: 200},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
				TaskOrder:   []core.TaskID{"task-1"},
				Checkpoints: []core.Checkpoint{},
				Metrics:     &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when state save fails")
	}
}

func TestExecutor_Run_NoTasks(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	// No tasks added to DAG

	registry := &mockAgentRegistry{}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhasePlan, // Previous phase
				Tasks:        map[core.TaskID]*core.TaskState{},
				TaskOrder:    []core.TaskID{},
				Checkpoints:  []core.Checkpoint{},
				Metrics:      &core.StateMetrics{},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	err := executor.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when no tasks to execute")
	}

	// Verify it's a validation error with the correct code
	var qErr *core.DomainError
	if errors.As(err, &qErr) {
		if qErr.Code != core.CodeMissingTasks {
			t.Errorf("error code = %q, want %q", qErr.Code, core.CodeMissingTasks)
		}
	} else {
		t.Error("expected *core.DomainError")
	}
}
