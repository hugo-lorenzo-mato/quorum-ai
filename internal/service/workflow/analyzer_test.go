package workflow

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

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

func (m *mockAgentRegistry) AvailableForPhase(_ context.Context, _ string) []string {
	// Mock returns all agents for any phase (can be extended for more specific tests)
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
	planErr     error
	taskErr     error
}

func (m *mockPromptRenderer) RenderRefinePrompt(_ RefinePromptParams) (string, error) {
	if m.optimizeErr != nil {
		return "", m.optimizeErr
	}
	return "refined prompt", nil
}

func (m *mockPromptRenderer) RenderAnalyzeV1(_ AnalyzeV1Params) (string, error) {
	if m.v1Err != nil {
		return "", m.v1Err
	}
	return "analyze v1 prompt", nil
}

func (m *mockPromptRenderer) RenderSynthesizeAnalysis(_ SynthesizeAnalysisParams) (string, error) {
	return "synthesize analysis prompt", nil
}

func (m *mockPromptRenderer) RenderPlanGenerate(_ PlanParams) (string, error) {
	if m.planErr != nil {
		return "", m.planErr
	}
	return "plan prompt", nil
}

func (m *mockPromptRenderer) RenderSynthesizePlans(_ SynthesizePlansParams) (string, error) {
	return "synthesize plans prompt", nil
}

func (m *mockPromptRenderer) RenderTaskExecute(_ TaskExecuteParams) (string, error) {
	if m.taskErr != nil {
		return "", m.taskErr
	}
	return "task prompt", nil
}

func (m *mockPromptRenderer) RenderModeratorEvaluate(_ ModeratorEvaluateParams) (string, error) {
	return "moderator evaluate prompt", nil
}

func (m *mockPromptRenderer) RenderVnRefine(_ VnRefineParams) (string, error) {
	return "vn refine prompt", nil
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
	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "claude",
		Threshold: 0.90,
		MinRounds: 2,
		MaxRounds: 3,
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	if analyzer == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
	if analyzer.moderator == nil {
		t.Error("NewAnalyzer() did not set moderator")
	}
}

func TestNewAnalyzer_Disabled(t *testing.T) {
	config := ModeratorConfig{
		Enabled: false,
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	if analyzer == nil {
		t.Fatal("NewAnalyzer() returned nil")
	}
	// When disabled, moderator should still be created but will be inactive
}

func TestAnalyzer_Run_WithModeratorDisabled_ReturnsError(t *testing.T) {
	// When moderator is disabled, analyzer.Run should return an error
	// because the new design requires semantic moderator for consensus
	config := ModeratorConfig{
		Enabled: false,
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

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
		},
	}

	err = analyzer.Run(context.Background(), wctx)
	if err == nil {
		t.Fatal("Analyzer.Run() should return error when arbiter is disabled")
	}
	if !strings.Contains(err.Error(), "semantic moderator is required") {
		t.Errorf("error message should mention moderator required, got: %v", err)
	}
}

func TestAnalyzer_Run_NoAgents(t *testing.T) {
	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "claude",
		Threshold: 0.90,
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

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

	err = analyzer.Run(context.Background(), wctx)
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

func TestParseAnalysisOutput_Markdown(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantClaims int
		wantRisks  int
		wantRecs   int
	}{
		{
			name: "standard markdown sections",
			output: `## Claims
- The codebase uses Go modules
- Tests are present

## Risks
- No documentation
- Missing error handling

## Recommendations
- Add documentation
- Improve error handling
- Add more tests`,
			wantClaims: 2,
			wantRisks:  2,
			wantRecs:   3,
		},
		{
			name: "mixed header levels",
			output: `### Claims
- Claim one
- Claim two

### Risks
- Risk one

### Recommendations
- Rec one`,
			wantClaims: 2,
			wantRisks:  1,
			wantRecs:   1,
		},
		{
			name: "numbered lists",
			output: `## Claims
1. First claim
2. Second claim

## Risks
1) Risk one

## Recommendations
1. Recommendation one`,
			wantClaims: 2,
			wantRisks:  1,
			wantRecs:   1,
		},
		{
			name: "with extra text between sections",
			output: `Here is my analysis:

## Claims
- The code is well structured

Some additional thoughts about claims.

## Risks
- Performance could be improved

## Recommendations
- Consider caching`,
			wantClaims: 1,
			wantRisks:  1,
			wantRecs:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &core.ExecuteResult{Output: tt.output}
			output := parseAnalysisOutput("test-agent", result)

			if len(output.Claims) != tt.wantClaims {
				t.Errorf("len(Claims) = %d, want %d. Claims: %v", len(output.Claims), tt.wantClaims, output.Claims)
			}
			if len(output.Risks) != tt.wantRisks {
				t.Errorf("len(Risks) = %d, want %d. Risks: %v", len(output.Risks), tt.wantRisks, output.Risks)
			}
			if len(output.Recommendations) != tt.wantRecs {
				t.Errorf("len(Recommendations) = %d, want %d. Recs: %v", len(output.Recommendations), tt.wantRecs, output.Recommendations)
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

func TestAnalyzer_Run_SkipsWhenPhaseCompleted(t *testing.T) {
	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "claude",
		Threshold: 0.90,
	}
	analyzer, err := NewAnalyzer(config)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}

	checkpoint := &mockCheckpointCreator{}

	// Create a state that has a phase_complete checkpoint for analyze
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowID: "wf-test",
			Prompt:     "test prompt",
			Checkpoints: []core.Checkpoint{
				{
					Type:  "phase_complete",
					Phase: core.PhaseAnalyze,
				},
			},
		},
		Agents:     &mockAgentRegistry{agents: map[string]core.Agent{"claude": &mockAgent{}}},
		Prompts:    &mockPromptRenderer{},
		Checkpoint: checkpoint,
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Config:     &Config{},
	}

	err = analyzer.Run(context.Background(), wctx)
	if err != nil {
		t.Fatalf("Analyzer.Run() error = %v, want nil (should skip)", err)
	}

	// Verify that no new checkpoints were created (analyzer was skipped)
	checkpoint.mu.Lock()
	checkpointCount := len(checkpoint.checkpoints)
	checkpoint.mu.Unlock()
	if checkpointCount > 0 {
		t.Errorf("Analyzer.Run() created %d checkpoint(s), want 0 when phase is already completed", checkpointCount)
	}
}

func TestIsPhaseCompleted(t *testing.T) {
	tests := []struct {
		name       string
		state      *core.WorkflowState
		phase      core.Phase
		wantResult bool
	}{
		{
			name: "no checkpoints",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{},
			},
			phase:      core.PhaseAnalyze,
			wantResult: false,
		},
		{
			name: "has phase_complete for analyze",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{Type: "phase_start", Phase: core.PhaseAnalyze},
					{Type: "phase_complete", Phase: core.PhaseAnalyze},
				},
			},
			phase:      core.PhaseAnalyze,
			wantResult: true,
		},
		{
			name: "has phase_complete for different phase",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{Type: "phase_complete", Phase: core.PhaseRefine},
				},
			},
			phase:      core.PhaseAnalyze,
			wantResult: false,
		},
		{
			name: "only has phase_start",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{Type: "phase_start", Phase: core.PhaseAnalyze},
				},
			},
			phase:      core.PhaseAnalyze,
			wantResult: false,
		},
		{
			name: "phase_complete exists among multiple checkpoints",
			state: &core.WorkflowState{
				Checkpoints: []core.Checkpoint{
					{Type: "phase_start", Phase: core.PhaseRefine},
					{Type: "phase_complete", Phase: core.PhaseRefine},
					{Type: "phase_start", Phase: core.PhaseAnalyze},
					{Type: "consolidated_analysis", Phase: core.PhaseAnalyze},
					{Type: "phase_complete", Phase: core.PhaseAnalyze},
					{Type: "phase_start", Phase: core.PhaseAnalyze}, // Extra (should still find phase_complete)
				},
			},
			phase:      core.PhaseAnalyze,
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPhaseCompleted(tt.state, tt.phase)
			if got != tt.wantResult {
				t.Errorf("isPhaseCompleted() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}
