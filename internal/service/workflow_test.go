package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockAgent implements core.Agent for testing.
type mockAgent struct {
	name        string
	result      *core.ExecuteResult
	err         error
	callCount   int
	mu          sync.Mutex
	executeFunc func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error)
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Capabilities() core.Capabilities {
	return core.Capabilities{
		SupportsJSON: true,
	}
}

func (m *mockAgent) Ping(ctx context.Context) error {
	return nil
}

func (m *mockAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.executeFunc != nil {
		return m.executeFunc(ctx, opts)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockAgentRegistry implements core.AgentRegistry for testing.
type mockAgentRegistry struct {
	agents map[string]core.Agent
}

func newMockAgentRegistry() *mockAgentRegistry {
	return &mockAgentRegistry{
		agents: make(map[string]core.Agent),
	}
}

func (r *mockAgentRegistry) Register(name string, agent core.Agent) error {
	r.agents[name] = agent
	return nil
}

func (r *mockAgentRegistry) Get(name string) (core.Agent, error) {
	agent, ok := r.agents[name]
	if !ok {
		return nil, errors.New("agent not found")
	}
	return agent, nil
}

func (r *mockAgentRegistry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

func (r *mockAgentRegistry) Available(ctx context.Context) []string {
	return r.List()
}

// mockWorkflowStateManager implements core.StateManager for workflow tests.
type mockWorkflowStateManager struct {
	state     *core.WorkflowState
	saveError error
	loadError error
	mu        sync.Mutex
}

func newMockWorkflowStateManager() *mockWorkflowStateManager {
	return &mockWorkflowStateManager{}
}

func (m *mockWorkflowStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveError != nil {
		return m.saveError
	}
	// Deep copy to avoid mutation issues
	data, _ := json.Marshal(state)
	var copy core.WorkflowState
	json.Unmarshal(data, &copy)
	m.state = &copy
	return nil
}

func (m *mockWorkflowStateManager) Load(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadError != nil {
		return nil, m.loadError
	}
	return m.state, nil
}

func (m *mockWorkflowStateManager) AcquireLock(ctx context.Context) error { return nil }
func (m *mockWorkflowStateManager) ReleaseLock(ctx context.Context) error { return nil }
func (m *mockWorkflowStateManager) Exists() bool                          { return m.state != nil }
func (m *mockWorkflowStateManager) Backup(ctx context.Context) error      { return nil }
func (m *mockWorkflowStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
	return m.state, nil
}

func createTestWorkflowRunner(agents ...core.Agent) (*WorkflowRunner, *mockWorkflowStateManager, *mockAgentRegistry) {
	stateManager := newMockWorkflowStateManager()
	registry := newMockAgentRegistry()
	logger := logging.NewNop()

	for _, agent := range agents {
		registry.Register(agent.Name(), agent)
	}

	prompts, _ := NewPromptRenderer()
	consensus := NewConsensusChecker(0.75, DefaultWeights())

	config := &WorkflowRunnerConfig{
		Timeout:      5 * time.Minute,
		MaxRetries:   3,
		DryRun:       false,
		DefaultAgent: "claude",
		V3Agent:      "claude",
	}

	runner := NewWorkflowRunner(config, stateManager, registry, consensus, prompts, logger)
	return runner, stateManager, registry
}

func TestWorkflowRunner_Run_NoAgents(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()
	ctx := context.Background()

	err := runner.Run(ctx, "Test prompt")
	if err == nil {
		t.Error("Run() should fail with no agents")
	}
}

func TestWorkflowRunner_Run_DryRun(t *testing.T) {
	analysisResult := &core.ExecuteResult{
		Output: `{"claims":["claim1"],"risks":["risk1"],"recommendations":["rec1"]}`,
	}
	planResult := &core.ExecuteResult{
		Output: `[{"id":"task-1","name":"Task 1","description":"Do something"}]`,
	}

	callCount := 0
	agent := &mockAgent{
		name: "claude",
		executeFunc: func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			callCount++
			if callCount <= 1 {
				return analysisResult, nil
			}
			return planResult, nil
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	runner.config.DryRun = true
	ctx := context.Background()

	err := runner.Run(ctx, "Test prompt")
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}

	state, _ := stateManager.Load(ctx)
	if state == nil {
		t.Fatal("state should not be nil")
	}
	if state.Status != core.WorkflowStatusCompleted {
		t.Errorf("Status = %v, want %v", state.Status, core.WorkflowStatusCompleted)
	}
}

func TestWorkflowRunner_Resume_NoState(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: `{"claims":[],"risks":[],"recommendations":[]}`,
		},
	}

	runner, _, _ := createTestWorkflowRunner(agent)
	ctx := context.Background()

	err := runner.Resume(ctx)
	if err == nil {
		t.Error("Resume() should fail with no state")
	}
}

func TestWorkflowRunner_AnalyzePhase_SingleAgent(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: `{"claims":["claim1"],"risks":["risk1"],"recommendations":["rec1"]}`,
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
	}
	stateManager.state = state

	err := runner.runAnalyzePhase(ctx, state)
	if err != nil {
		t.Errorf("runAnalyzePhase() error = %v", err)
	}

	if len(state.Checkpoints) == 0 {
		t.Error("should have created checkpoints")
	}
}

func TestWorkflowRunner_AnalyzePhase_MultipleAgents(t *testing.T) {
	agent1 := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: `{"claims":["claim1","claim2"],"risks":["risk1"],"recommendations":["rec1"]}`,
		},
	}
	agent2 := &mockAgent{
		name: "gemini",
		result: &core.ExecuteResult{
			Output: `{"claims":["claim1","claim3"],"risks":["risk2"],"recommendations":["rec1"]}`,
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent1, agent2)
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
	}
	stateManager.state = state

	err := runner.runAnalyzePhase(ctx, state)
	if err != nil {
		t.Errorf("runAnalyzePhase() error = %v", err)
	}

	// Both agents should have been called
	if agent1.callCount == 0 {
		t.Error("agent1 should have been called")
	}
	if agent2.callCount == 0 {
		t.Error("agent2 should have been called")
	}
}

func TestWorkflowRunner_V3Escalation(t *testing.T) {
	// Create agents with divergent outputs to trigger V3
	agent1 := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: `{"claims":["unique claim A"],"risks":["unique risk A"],"recommendations":["unique rec A"]}`,
		},
	}
	agent2 := &mockAgent{
		name: "gemini",
		result: &core.ExecuteResult{
			Output: `{"claims":["unique claim B"],"risks":["unique risk B"],"recommendations":["unique rec B"]}`,
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent1, agent2)
	// Set low threshold to ensure V3 is triggered
	runner.consensus.Threshold = 0.9
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
	}
	stateManager.state = state

	err := runner.runAnalyzePhase(ctx, state)
	if err != nil {
		t.Errorf("runAnalyzePhase() error = %v", err)
	}

	// Should have multiple checkpoints including consensus
	hasConsensusCheckpoint := false
	for _, cp := range state.Checkpoints {
		if cp.Type == string(CheckpointConsensus) {
			hasConsensusCheckpoint = true
			break
		}
	}
	if !hasConsensusCheckpoint {
		t.Error("should have consensus checkpoint")
	}
}

func TestWorkflowRunner_PlanPhase(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: `[{"id":"task-1","name":"Task 1","description":"Do task 1"},{"id":"task-2","name":"Task 2","description":"Do task 2","dependencies":["task-1"]}]`,
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	ctx := context.Background()

	// Create state with consolidated analysis
	analysisData, _ := json.Marshal(map[string]interface{}{
		"content":     "Consolidated analysis content",
		"agent_count": 1,
	})
	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhasePlan,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    make([]core.TaskID, 0),
		Checkpoints: []core.Checkpoint{
			{
				Type: "consolidated_analysis",
				Data: analysisData,
			},
		},
		Metrics: &core.StateMetrics{},
	}
	stateManager.state = state

	err := runner.runPlanPhase(ctx, state)
	if err != nil {
		t.Errorf("runPlanPhase() error = %v", err)
	}

	if len(state.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(state.Tasks))
	}

	if len(state.TaskOrder) != 2 {
		t.Errorf("len(TaskOrder) = %d, want 2", len(state.TaskOrder))
	}
}

func TestWorkflowRunner_ExecutePhase(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output:    "Task completed successfully",
			TokensIn:  100,
			TokensOut: 200,
			CostUSD:   0.01,
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Task 1",
				Status: core.TaskStatusPending,
				Phase:  core.PhaseExecute,
			},
		},
		TaskOrder:   []core.TaskID{"task-1"},
		Checkpoints: make([]core.Checkpoint, 0),
		Metrics:     &core.StateMetrics{},
	}
	stateManager.state = state

	// Add task to DAG
	runner.dag.AddTask(&core.Task{
		ID:     "task-1",
		Name:   "Task 1",
		Status: core.TaskStatusPending,
		Phase:  core.PhaseExecute,
	})

	err := runner.runExecutePhase(ctx, state)
	if err != nil {
		t.Errorf("runExecutePhase() error = %v", err)
	}

	task := state.Tasks["task-1"]
	if task.Status != core.TaskStatusCompleted {
		t.Errorf("task status = %v, want %v", task.Status, core.TaskStatusCompleted)
	}

	if state.Metrics.TotalTokensIn != 100 {
		t.Errorf("TotalTokensIn = %d, want 100", state.Metrics.TotalTokensIn)
	}
}

func TestWorkflowRunner_ExecutePhase_DryRun(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: "Should not be called",
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	runner.config.DryRun = true
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Task 1",
				Status: core.TaskStatusPending,
				Phase:  core.PhaseExecute,
			},
		},
		TaskOrder:   []core.TaskID{"task-1"},
		Checkpoints: make([]core.Checkpoint, 0),
		Metrics:     &core.StateMetrics{},
	}
	stateManager.state = state

	runner.dag.AddTask(&core.Task{
		ID:     "task-1",
		Name:   "Task 1",
		Status: core.TaskStatusPending,
		Phase:  core.PhaseExecute,
	})

	err := runner.runExecutePhase(ctx, state)
	if err != nil {
		t.Errorf("runExecutePhase() error = %v", err)
	}

	// Agent should not be called in dry-run mode
	if agent.callCount > 0 {
		t.Error("agent should not be called in dry-run mode")
	}

	task := state.Tasks["task-1"]
	if task.Status != core.TaskStatusCompleted {
		t.Errorf("task status = %v, want %v", task.Status, core.TaskStatusCompleted)
	}
}

func TestWorkflowRunner_ExecutePhase_WithDependencies(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		result: &core.ExecuteResult{
			Output: "Task completed",
		},
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Task 1",
				Status: core.TaskStatusPending,
				Phase:  core.PhaseExecute,
			},
			"task-2": {
				ID:           "task-2",
				Name:         "Task 2",
				Status:       core.TaskStatusPending,
				Phase:        core.PhaseExecute,
				Dependencies: []core.TaskID{"task-1"},
			},
		},
		TaskOrder:   []core.TaskID{"task-1", "task-2"},
		Checkpoints: make([]core.Checkpoint, 0),
		Metrics:     &core.StateMetrics{},
	}
	stateManager.state = state

	// Add tasks to DAG
	runner.dag.AddTask(&core.Task{
		ID:     "task-1",
		Name:   "Task 1",
		Status: core.TaskStatusPending,
		Phase:  core.PhaseExecute,
	})
	runner.dag.AddTask(&core.Task{
		ID:           "task-2",
		Name:         "Task 2",
		Status:       core.TaskStatusPending,
		Phase:        core.PhaseExecute,
		Dependencies: []core.TaskID{"task-1"},
	})
	runner.dag.AddDependency("task-2", "task-1")

	err := runner.runExecutePhase(ctx, state)
	if err != nil {
		t.Errorf("runExecutePhase() error = %v", err)
	}

	// Both tasks should be completed
	for id, task := range state.Tasks {
		if task.Status != core.TaskStatusCompleted {
			t.Errorf("task %s status = %v, want %v", id, task.Status, core.TaskStatusCompleted)
		}
	}
}

func TestWorkflowRunner_ExecutePhase_TaskFailure(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		err:  errors.New("execution failed"),
	}

	runner, stateManager, _ := createTestWorkflowRunner(agent)
	// Reduce retries for faster test
	runner.retry = NewRetryPolicy(WithMaxAttempts(1))
	ctx := context.Background()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Task 1",
				Status: core.TaskStatusPending,
				Phase:  core.PhaseExecute,
			},
		},
		TaskOrder:   []core.TaskID{"task-1"},
		Checkpoints: make([]core.Checkpoint, 0),
		Metrics:     &core.StateMetrics{},
	}
	stateManager.state = state

	runner.dag.AddTask(&core.Task{
		ID:     "task-1",
		Name:   "Task 1",
		Status: core.TaskStatusPending,
		Phase:  core.PhaseExecute,
	})

	err := runner.runExecutePhase(ctx, state)
	if err == nil {
		t.Error("runExecutePhase() should fail when task fails")
	}

	task := state.Tasks["task-1"]
	if task.Status != core.TaskStatusFailed {
		t.Errorf("task status = %v, want %v", task.Status, core.TaskStatusFailed)
	}
	if task.Error == "" {
		t.Error("task error should not be empty")
	}
}

func TestWorkflowRunner_ParsePlan_JSON(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	output := `[{"id":"task-1","name":"Task 1","description":"Do something","cli":"claude"},{"id":"task-2","name":"Task 2","description":"Do something else","dependencies":["task-1"]}]`

	tasks, err := runner.parsePlan(output)
	if err != nil {
		t.Fatalf("parsePlan() error = %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(tasks))
	}

	if tasks[0].ID != "task-1" {
		t.Errorf("tasks[0].ID = %s, want task-1", tasks[0].ID)
	}

	if tasks[0].CLI != "claude" {
		t.Errorf("tasks[0].CLI = %s, want claude", tasks[0].CLI)
	}

	if len(tasks[1].Dependencies) != 1 || tasks[1].Dependencies[0] != "task-1" {
		t.Errorf("tasks[1].Dependencies = %v, want [task-1]", tasks[1].Dependencies)
	}
}

func TestWorkflowRunner_ParsePlan_WrappedJSON(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	output := `{"tasks":[{"id":"task-1","name":"Task 1","description":"Do something"}]}`

	tasks, err := runner.parsePlan(output)
	if err != nil {
		t.Fatalf("parsePlan() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("len(tasks) = %d, want 1", len(tasks))
	}
}

func TestWorkflowRunner_ParsePlan_InvalidJSON(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	output := `not valid json`

	_, err := runner.parsePlan(output)
	if err == nil {
		t.Fatal("parsePlan() error = nil, want error for invalid JSON")
	}

	// Should return an error explaining the parse failure
	if !strings.Contains(err.Error(), "failed to parse plan output") {
		t.Errorf("error = %v, want error containing 'failed to parse plan output'", err)
	}
}

func TestWorkflowRunner_SelectCritiqueAgent(t *testing.T) {
	agent1 := &mockAgent{name: "claude"}
	agent2 := &mockAgent{name: "gemini"}

	runner, _, _ := createTestWorkflowRunner(agent1, agent2)

	// Should select different agent
	critique := runner.selectCritiqueAgent(context.Background(), "claude")
	if critique == "claude" {
		t.Error("should select different agent for critique")
	}

	critique = runner.selectCritiqueAgent(context.Background(), "gemini")
	if critique == "gemini" {
		t.Error("should select different agent for critique")
	}
}

func TestWorkflowRunner_SelectCritiqueAgent_SingleAgent(t *testing.T) {
	agent := &mockAgent{name: "claude"}

	runner, _, _ := createTestWorkflowRunner(agent)

	// Should return same agent if only one available
	critique := runner.selectCritiqueAgent(context.Background(), "claude")
	if critique != "claude" {
		t.Errorf("critique = %s, want claude (only available agent)", critique)
	}
}

func TestWorkflowRunner_ParseAnalysisOutput(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	result := &core.ExecuteResult{
		Output: `{"claims":["claim1","claim2"],"risks":["risk1"],"recommendations":["rec1","rec2"]}`,
	}

	output := runner.parseAnalysisOutput("claude", result)

	if output.AgentName != "claude" {
		t.Errorf("AgentName = %s, want claude", output.AgentName)
	}

	if len(output.Claims) != 2 {
		t.Errorf("len(Claims) = %d, want 2", len(output.Claims))
	}

	if len(output.Risks) != 1 {
		t.Errorf("len(Risks) = %d, want 1", len(output.Risks))
	}

	if len(output.Recommendations) != 2 {
		t.Errorf("len(Recommendations) = %d, want 2", len(output.Recommendations))
	}
}

func TestWorkflowRunner_ParseAnalysisOutput_InvalidJSON(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	result := &core.ExecuteResult{
		Output: "not valid json but still output",
	}

	output := runner.parseAnalysisOutput("claude", result)

	if output.AgentName != "claude" {
		t.Errorf("AgentName = %s, want claude", output.AgentName)
	}

	if output.RawOutput != result.Output {
		t.Error("RawOutput should contain the original output")
	}

	// Claims/Risks/Recommendations should be nil or empty
	if len(output.Claims) != 0 || len(output.Risks) != 0 || len(output.Recommendations) != 0 {
		t.Error("structured fields should be empty for invalid JSON")
	}
}

func TestWorkflowRunner_BuildContext(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task 1", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Name: "Task 2", Status: core.TaskStatusPending},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2"},
	}

	ctx := runner.buildContext(state)

	if ctx == "" {
		t.Error("context should not be empty")
	}

	if !strings.Contains(ctx, "wf-test") {
		t.Error("context should contain workflow ID")
	}

	if !strings.Contains(ctx, "execute") {
		t.Error("context should contain current phase")
	}
}

func TestWorkflowRunner_SetDryRun(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	if runner.config.DryRun {
		t.Error("DryRun should be false by default")
	}

	runner.SetDryRun(true)

	if !runner.config.DryRun {
		t.Error("DryRun should be true after SetDryRun(true)")
	}
}

func TestDefaultWorkflowRunnerConfig(t *testing.T) {
	cfg := DefaultWorkflowRunnerConfig()

	if cfg.Timeout != time.Hour {
		t.Errorf("Timeout = %v, want 1h", cfg.Timeout)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %s, want claude", cfg.DefaultAgent)
	}

	if cfg.V3Agent != "claude" {
		t.Errorf("V3Agent = %s, want claude", cfg.V3Agent)
	}
}

func TestGenerateWorkflowID(t *testing.T) {
	id1 := generateWorkflowID()
	id2 := generateWorkflowID()

	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}

	if !strings.HasPrefix(id1, "wf-") {
		t.Errorf("ID should start with 'wf-', got %s", id1)
	}
}

func TestWorkflowRunner_RebuildDAG(t *testing.T) {
	runner, _, _ := createTestWorkflowRunner()

	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Task 1",
				Status: core.TaskStatusPending,
				Phase:  core.PhaseExecute,
			},
			"task-2": {
				ID:           "task-2",
				Name:         "Task 2",
				Status:       core.TaskStatusPending,
				Phase:        core.PhaseExecute,
				Dependencies: []core.TaskID{"task-1"},
			},
		},
	}

	err := runner.rebuildDAG(state)
	if err != nil {
		t.Fatalf("rebuildDAG() error = %v", err)
	}

	// Verify DAG was rebuilt correctly
	completed := make(map[core.TaskID]bool)
	ready := runner.dag.GetReadyTasks(completed)

	if len(ready) != 1 || ready[0].ID != "task-1" {
		t.Error("only task-1 should be ready initially")
	}

	completed["task-1"] = true
	ready = runner.dag.GetReadyTasks(completed)

	if len(ready) != 1 || ready[0].ID != "task-2" {
		t.Error("task-2 should be ready after task-1 completes")
	}
}
