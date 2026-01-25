package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockAgentWithCallback is a mock agent that uses a callback to determine results.
type mockAgentWithCallback struct {
	callback func(opts core.ExecuteOptions) *core.ExecuteResult
}

func (m *mockAgentWithCallback) Name() string {
	return "mock-callback"
}

func (m *mockAgentWithCallback) Execute(_ context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	if m.callback != nil {
		return m.callback(opts), nil
	}
	return &core.ExecuteResult{Output: "default"}, nil
}

func (m *mockAgentWithCallback) Ping(_ context.Context) error {
	return nil
}

func (m *mockAgentWithCallback) Capabilities() core.Capabilities {
	return core.Capabilities{
		SupportsJSON:      true,
		SupportsStreaming: false,
	}
}

func TestPlanner_Run_Success(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	// Agent returns manifest on first call, then task detail generation on subsequent calls
	callCount := 0
	agent := &mockAgentWithCallback{
		callback: func(_ core.ExecuteOptions) *core.ExecuteResult {
			callCount++
			if callCount == 1 {
				// First call: return task manifest
				return &core.ExecuteResult{
					Output: `{"tasks": [{"id": "task-1", "name": "Task 1", "dependencies": [], "complexity": "low", "cli": "claude"}]}`,
				}
			}
			// Subsequent calls: task detail generation (just return success)
			return &core.ExecuteResult{
				Output: "Task details generated successfully",
			}
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	// Create checkpoint with consolidated analysis
	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if len(wctx.State.Tasks) != 1 {
		t.Errorf("Tasks count = %d, want 1", len(wctx.State.Tasks))
	}

	if wctx.State.CurrentPhase != core.PhasePlan {
		t.Errorf("Phase = %v, want plan", wctx.State.CurrentPhase)
	}
}

func TestPlanner_Run_NoAnalysis(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints:  []core.Checkpoint{}, // No consolidated analysis
			Metrics:      &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when no analysis")
	}
}

func TestPlanner_Run_AgentNotFound(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{} // No agents registered

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when agent not found")
	}
}

func TestPlanner_Run_RateLimitFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	failingLimiter := &mockRateLimiter{acquireErr: errors.New("rate limit failed")}

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when rate limit fails")
	}
}

func TestPlanner_Run_PromptRenderFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})

	failingPrompts := &mockPromptRenderer{
		planErr: errors.New("prompt render failed"),
	}

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when prompt render fails")
	}
}

func TestPlanner_Run_AgentExecutionFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	agent := &mockAgent{
		err: errors.New("agent execution failed"),
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when agent execution fails")
	}
}

func TestPlanner_Run_InvalidPlanOutput(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output: "not valid json",
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when plan output is invalid")
	}
}

func TestPlanner_Run_DAGBuildFails(t *testing.T) {
	dag := &mockDAGBuilderWithError{buildErr: errors.New("DAG build failed")}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output: `[{"id": "task-1", "name": "Task 1", "description": "First task", "cli": "claude", "dependencies": []}]`,
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when DAG build fails")
	}
}

func TestPlanner_Run_WithOutput(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	callCount := 0
	agent := &mockAgentWithCallback{
		callback: func(_ core.ExecuteOptions) *core.ExecuteResult {
			callCount++
			if callCount == 1 {
				return &core.ExecuteResult{
					Output: `{"tasks": [{"id": "task-1", "name": "Task 1", "dependencies": [], "complexity": "low", "cli": "claude"}]}`,
				}
			}
			return &core.ExecuteResult{Output: "Task details generated"}
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	output := &mockOutputNotifier{}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
			DefaultAgent: "claude",
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if len(output.phaseStarted) != 1 || output.phaseStarted[0] != core.PhasePlan {
		t.Errorf("PhaseStarted not called correctly")
	}
	if output.stateUpdated != 1 {
		t.Errorf("WorkflowStateUpdated called %d times, want 1", output.stateUpdated)
	}
}

func TestPlanner_Run_WithDependencies(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	callCount := 0
	agent := &mockAgentWithCallback{
		callback: func(_ core.ExecuteOptions) *core.ExecuteResult {
			callCount++
			if callCount == 1 {
				return &core.ExecuteResult{
					Output: `{"tasks": [
						{"id": "task-1", "name": "Task 1", "dependencies": [], "complexity": "low", "cli": "claude"},
						{"id": "task-2", "name": "Task 2", "dependencies": ["task-1"], "complexity": "medium", "cli": "claude"}
					]}`,
				}
			}
			return &core.ExecuteResult{Output: "Task details generated"}
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	if len(wctx.State.Tasks) != 2 {
		t.Errorf("Tasks count = %d, want 2", len(wctx.State.Tasks))
	}

	// Verify dependencies were added to DAG
	if len(dag.deps) == 0 {
		t.Error("Dependencies not added to DAG")
	}
}

func TestPlanner_Run_SaveStateFails(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{err: errors.New("save failed")}
	planner := NewPlanner(dag, saver)

	callCount := 0
	agent := &mockAgentWithCallback{
		callback: func(_ core.ExecuteOptions) *core.ExecuteResult {
			callCount++
			if callCount == 1 {
				return &core.ExecuteResult{
					Output: `{"tasks": [{"id": "task-1", "name": "Task 1", "dependencies": [], "complexity": "low", "cli": "claude"}]}`,
				}
			}
			return &core.ExecuteResult{Output: "Task details generated"}
		},
	}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	analysisData := map[string]interface{}{"content": "Test analysis"}
	analysisJSON, _ := json.Marshal(analysisData)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints: []core.Checkpoint{
				{Type: "consolidated_analysis", Data: analysisJSON},
			},
			Metrics: &core.StateMetrics{},
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
		},
	}

	err := planner.Run(context.Background(), wctx)
	if err == nil {
		t.Error("Run() should return error when save fails")
	}
}

// mockDAGBuilderWithError is a DAG builder that returns an error on Build.
type mockDAGBuilderWithError struct {
	mockDAGBuilder
	buildErr error
}

func (m *mockDAGBuilderWithError) Build() (interface{}, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return m, nil
}

func TestParsePlanItems_GeminiWithMultipleParts(t *testing.T) {
	input := `{
		"candidates": [{
			"content": {
				"parts": [
					{"text": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"},
					{"text": ""}
				]
			}
		}]
	}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_GeminiEmptyCandidate(t *testing.T) {
	input := `{
		"candidates": []
	}`

	_, err := parsePlanItems(input)
	if err == nil {
		t.Error("parsePlanItems() should return error for empty candidates")
	}
}

func TestParsePlanItems_GeminiEmptyParts(t *testing.T) {
	input := `{
		"candidates": [{
			"content": {
				"parts": []
			}
		}]
	}`

	_, err := parsePlanItems(input)
	if err == nil {
		t.Error("parsePlanItems() should return error for empty parts")
	}
}

func TestExtractJSON_WithEscapedBackslash(t *testing.T) {
	input := `{"key": "value with \\\\ backslash"}`
	result := extractJSON(input)

	if result != input {
		t.Errorf("extractJSON() = %q, want %q", result, input)
	}
}

func TestExtractJSON_NestedArrays(t *testing.T) {
	input := `Prefix [[1, 2], [3, 4]] suffix`
	result := extractJSON(input)

	if result != `[[1, 2], [3, 4]]` {
		t.Errorf("extractJSON() = %q, want [[1, 2], [3, 4]]", result)
	}
}

func TestRawToText_Number(t *testing.T) {
	raw := json.RawMessage(`123`)
	result := rawToText(raw)

	// Numbers should not be extracted as text
	if result != "" {
		t.Errorf("rawToText() = %q, want empty for number", result)
	}
}

func TestRawToText_Boolean(t *testing.T) {
	raw := json.RawMessage(`true`)
	result := rawToText(raw)

	// Booleans should not be extracted as text
	if result != "" {
		t.Errorf("rawToText() = %q, want empty for boolean", result)
	}
}

func TestRawToText_Null(t *testing.T) {
	raw := json.RawMessage(`null`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty for null", result)
	}
}

func TestRawToText_ArrayWithEmptyText(t *testing.T) {
	raw := json.RawMessage(`[{"text": ""}, {"text": "   "}]`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty for array with empty texts", result)
	}
}

func TestGetConsolidatedAnalysis_MultipleCheckpoints(t *testing.T) {
	data1, _ := json.Marshal(map[string]interface{}{"content": "First analysis"})
	data2, _ := json.Marshal(map[string]interface{}{"content": "Second analysis"})

	state := &core.WorkflowState{
		Checkpoints: []core.Checkpoint{
			{Type: "other_type", Data: []byte(`{}`)},
			{Type: "consolidated_analysis", Data: data1},
			{Type: "consolidated_analysis", Data: data2}, // Later one should be returned
		},
	}

	result := GetConsolidatedAnalysis(state)
	if result != "Second analysis" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want %q", result, "Second analysis")
	}
}

func TestGetConsolidatedAnalysis_NoCheckpoints(t *testing.T) {
	state := &core.WorkflowState{
		Checkpoints: []core.Checkpoint{},
	}

	result := GetConsolidatedAnalysis(state)
	if result != "" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want empty", result)
	}
}

func TestGetConsolidatedAnalysis_NilCheckpoints(t *testing.T) {
	state := &core.WorkflowState{
		Checkpoints: nil,
	}

	result := GetConsolidatedAnalysis(state)
	if result != "" {
		t.Errorf("GetConsolidatedAnalysis() = %q, want empty", result)
	}
}
