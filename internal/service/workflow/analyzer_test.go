package workflow

import (
	"context"
	"sync"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockConsensusEvaluator implements ConsensusEvaluator for testing.
type mockConsensusEvaluator struct {
	score            float64
	needsV3          bool
	needsHumanReview bool
	threshold        float64
	v2Threshold      float64
	humanThreshold   float64
}

func (m *mockConsensusEvaluator) Evaluate(_ []AnalysisOutput) ConsensusResult {
	return ConsensusResult{
		Score:            m.score,
		NeedsV3:          m.needsV3,
		NeedsHumanReview: m.needsHumanReview,
	}
}

func (m *mockConsensusEvaluator) Threshold() float64 {
	return m.threshold
}

func (m *mockConsensusEvaluator) V2Threshold() float64 {
	if m.v2Threshold == 0 {
		return 0.60
	}
	return m.v2Threshold
}

func (m *mockConsensusEvaluator) HumanThreshold() float64 {
	if m.humanThreshold == 0 {
		return 0.50
	}
	return m.humanThreshold
}

// mockAgentRegistry implements core.AgentRegistry for testing.
type mockAgentRegistry struct {
	agents map[string]core.Agent
}

func (m *mockAgentRegistry) Register(name string, agent core.Agent) error {
	if m.agents == nil {
		m.agents = make(map[string]core.Agent)
	}
	m.agents[name] = agent
	return nil
}

func (m *mockAgentRegistry) Get(name string) (core.Agent, error) {
	if agent, ok := m.agents[name]; ok {
		return agent, nil
	}
	return nil, core.ErrNotFound("agent", name)
}

func (m *mockAgentRegistry) List() []string {
	names := make([]string, 0, len(m.agents))
	for name := range m.agents {
		names = append(names, name)
	}
	return names
}

func (m *mockAgentRegistry) Available(_ context.Context) []string {
	return m.List()
}

// mockAgent implements core.Agent for testing.
type mockAgent struct {
	name   string
	result *core.ExecuteResult
	err    error
}

func (m *mockAgent) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

func (m *mockAgent) Capabilities() core.Capabilities {
	return core.Capabilities{
		SupportsJSON:      true,
		SupportsStreaming: false,
	}
}

func (m *mockAgent) Ping(_ context.Context) error {
	return nil
}

func (m *mockAgent) Execute(_ context.Context, _ core.ExecuteOptions) (*core.ExecuteResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockRateLimiterGetter implements RateLimiterGetter for testing.
type mockRateLimiterGetter struct {
	limiter RateLimiter
}

func (m *mockRateLimiterGetter) Get(_ string) RateLimiter {
	if m.limiter != nil {
		return m.limiter
	}
	return &mockRateLimiter{}
}

// mockRateLimiter implements RateLimiter for testing.
type mockRateLimiter struct {
	acquireErr error
}

func (m *mockRateLimiter) Acquire() error {
	return m.acquireErr
}

// mockRetryExecutor implements RetryExecutor for testing.
type mockRetryExecutor struct{}

func (m *mockRetryExecutor) Execute(fn func() error) error {
	return fn()
}

func (m *mockRetryExecutor) ExecuteWithNotify(fn func() error, _ func(int, error)) error {
	return fn()
}

// mockPromptRenderer implements PromptRenderer for testing.
type mockPromptRenderer struct {
	optimizeErr error
	v1Err       error
	v2Err       error
	v3Err       error
	planErr     error
	taskErr     error
}

func (m *mockPromptRenderer) RenderOptimizePrompt(_ OptimizePromptParams) (string, error) {
	if m.optimizeErr != nil {
		return "", m.optimizeErr
	}
	return "optimized prompt", nil
}

func (m *mockPromptRenderer) RenderAnalyzeV1(_ AnalyzeV1Params) (string, error) {
	if m.v1Err != nil {
		return "", m.v1Err
	}
	return "analyze v1 prompt", nil
}

func (m *mockPromptRenderer) RenderAnalyzeV2(_ AnalyzeV2Params) (string, error) {
	if m.v2Err != nil {
		return "", m.v2Err
	}
	return "analyze v2 prompt", nil
}

func (m *mockPromptRenderer) RenderAnalyzeV3(_ AnalyzeV3Params) (string, error) {
	if m.v3Err != nil {
		return "", m.v3Err
	}
	return "analyze v3 prompt", nil
}

func (m *mockPromptRenderer) RenderPlanGenerate(_ PlanParams) (string, error) {
	if m.planErr != nil {
		return "", m.planErr
	}
	return "plan prompt", nil
}

func (m *mockPromptRenderer) RenderTaskExecute(_ TaskExecuteParams) (string, error) {
	if m.taskErr != nil {
		return "", m.taskErr
	}
	return "task prompt", nil
}

// mockCheckpointCreator implements CheckpointCreator for testing.
type mockCheckpointCreator struct {
	mu          sync.Mutex
	checkpoints []string
}

func (m *mockCheckpointCreator) PhaseCheckpoint(_ *core.WorkflowState, phase core.Phase, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = append(m.checkpoints, string(phase))
	return nil
}

func (m *mockCheckpointCreator) TaskCheckpoint(_ *core.WorkflowState, task *core.Task, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = append(m.checkpoints, string(task.ID))
	return nil
}

func (m *mockCheckpointCreator) ConsensusCheckpoint(_ *core.WorkflowState, _ ConsensusResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = append(m.checkpoints, "consensus")
	return nil
}

func (m *mockCheckpointCreator) ErrorCheckpoint(_ *core.WorkflowState, _ error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = append(m.checkpoints, "error")
	return nil
}

func (m *mockCheckpointCreator) CreateCheckpoint(_ *core.WorkflowState, checkpointType string, _ map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = append(m.checkpoints, checkpointType)
	return nil
}

func TestNewAnalyzer(t *testing.T) {
	consensus := &mockConsensusEvaluator{score: 0.8, threshold: 0.75}
	analyzer := NewAnalyzer(consensus)

	if analyzer == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
	if analyzer.consensus != consensus {
		t.Error("NewAnalyzer() did not set consensus evaluator")
	}
}

func TestAnalyzer_Run_WithHighConsensus(t *testing.T) {
	// Setup mocks
	consensus := &mockConsensusEvaluator{score: 0.9, needsV3: false, threshold: 0.75}
	analyzer := NewAnalyzer(consensus)

	registry := &mockAgentRegistry{}
	agent := &mockAgent{
		result: &core.ExecuteResult{
			Output:    `{"claims":["claim1"],"risks":["risk1"],"recommendations":["rec1"]}`,
			TokensIn:  100,
			TokensOut: 50,
		},
	}
	registry.Register("claude", agent)
	registry.Register("gemini", agent)

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID:   "wf-test",
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "test prompt",
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints:  []core.Checkpoint{},
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
			Sandbox:      true,
			DefaultAgent: "claude",
			V3Agent:      "claude",
		},
	}

	err := analyzer.Run(context.Background(), wctx)
	if err != nil {
		t.Fatalf("Analyzer.Run() error = %v", err)
	}
}

func TestAnalyzer_Run_NoAgents(t *testing.T) {
	consensus := &mockConsensusEvaluator{score: 0.9, threshold: 0.75}
	analyzer := NewAnalyzer(consensus)

	registry := &mockAgentRegistry{} // Empty registry

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID: "wf-test",
			Prompt:     "test prompt",
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config:     &Config{},
	}

	err := analyzer.Run(context.Background(), wctx)
	if err == nil {
		t.Fatal("Analyzer.Run() with no agents should return error")
	}
}

func TestParseAnalysisOutput(t *testing.T) {
	tests := []struct {
		name       string
		agentName  string
		output     string
		wantClaims int
		wantRisks  int
		wantRecs   int
	}{
		{
			name:       "valid JSON",
			agentName:  "claude",
			output:     `{"claims":["c1","c2"],"risks":["r1"],"recommendations":["rec1","rec2","rec3"]}`,
			wantClaims: 2,
			wantRisks:  1,
			wantRecs:   3,
		},
		{
			name:       "invalid JSON",
			agentName:  "gemini",
			output:     "This is not JSON",
			wantClaims: 0,
			wantRisks:  0,
			wantRecs:   0,
		},
		{
			name:       "empty JSON",
			agentName:  "copilot",
			output:     `{}`,
			wantClaims: 0,
			wantRisks:  0,
			wantRecs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &core.ExecuteResult{Output: tt.output}
			output := parseAnalysisOutput(tt.agentName, result)

			if output.AgentName != tt.agentName {
				t.Errorf("AgentName = %q, want %q", output.AgentName, tt.agentName)
			}
			if len(output.Claims) != tt.wantClaims {
				t.Errorf("len(Claims) = %d, want %d", len(output.Claims), tt.wantClaims)
			}
			if len(output.Risks) != tt.wantRisks {
				t.Errorf("len(Risks) = %d, want %d", len(output.Risks), tt.wantRisks)
			}
			if len(output.Recommendations) != tt.wantRecs {
				t.Errorf("len(Recommendations) = %d, want %d", len(output.Recommendations), tt.wantRecs)
			}
		})
	}
}

func TestGetConsolidatedAnalysis(t *testing.T) {
	tests := []struct {
		name        string
		state       *core.WorkflowState
		wantContent string
	}{
		{
			name: "no checkpoints",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{},
			},
			wantContent: "",
		},
		{
			name: "with consolidated analysis checkpoint",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{
						Type: "consolidated_analysis",
						Data: []byte(`{"content":"analysis content","agent_count":2}`),
					},
				},
			},
			wantContent: "analysis content",
		},
		{
			name: "multiple checkpoints, returns latest",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{
						Type: "consolidated_analysis",
						Data: []byte(`{"content":"old analysis"}`),
					},
					{
						Type: "other",
						Data: []byte(`{"foo":"bar"}`),
					},
					{
						Type: "consolidated_analysis",
						Data: []byte(`{"content":"new analysis"}`),
					},
				},
			},
			wantContent: "new analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetConsolidatedAnalysis(tt.state)
			if got != tt.wantContent {
				t.Errorf("GetConsolidatedAnalysis() = %q, want %q", got, tt.wantContent)
			}
		})
	}
}
