// Package workflow provides adapters to bridge service layer types
// to workflow interface requirements.
package workflow

import (
	"context"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// ConsensusAdapter wraps service.ConsensusChecker to satisfy ConsensusEvaluator.
type ConsensusAdapter struct {
	checker *service.ConsensusChecker
}

// NewConsensusAdapter creates a new consensus adapter.
func NewConsensusAdapter(checker *service.ConsensusChecker) *ConsensusAdapter {
	return &ConsensusAdapter{checker: checker}
}

// Evaluate evaluates consensus between analysis outputs.
func (a *ConsensusAdapter) Evaluate(outputs []AnalysisOutput) ConsensusResult {
	// Convert workflow.AnalysisOutput to service.AnalysisOutput
	serviceOutputs := make([]service.AnalysisOutput, len(outputs))
	for i, o := range outputs {
		serviceOutputs[i] = service.AnalysisOutput{
			AgentName:       o.AgentName,
			RawOutput:       o.RawOutput,
			Claims:          o.Claims,
			Risks:           o.Risks,
			Recommendations: o.Recommendations,
		}
	}

	result := a.checker.Evaluate(serviceOutputs)

	// Convert divergences to strings
	divergenceStrings := make([]string, len(result.Divergences))
	for i, d := range result.Divergences {
		divergenceStrings[i] = d.Category + ": " + d.Agent1 + " vs " + d.Agent2
	}

	return ConsensusResult{
		Score:            result.Score,
		NeedsV3:          result.NeedsV3,
		NeedsHumanReview: result.NeedsHumanReview,
		Divergences:      divergenceStrings,
	}
}

// Threshold returns the consensus threshold value.
func (a *ConsensusAdapter) Threshold() float64 {
	return a.checker.GetThreshold()
}

// V2Threshold returns the V2 escalation threshold.
func (a *ConsensusAdapter) V2Threshold() float64 {
	return a.checker.GetV2Threshold()
}

// HumanThreshold returns the human review threshold.
func (a *ConsensusAdapter) HumanThreshold() float64 {
	return a.checker.GetHumanThreshold()
}

// CheckpointAdapter wraps service.CheckpointManager to satisfy CheckpointCreator.
type CheckpointAdapter struct {
	manager *service.CheckpointManager
	ctx     context.Context
}

// NewCheckpointAdapter creates a new checkpoint adapter.
func NewCheckpointAdapter(manager *service.CheckpointManager, ctx context.Context) *CheckpointAdapter {
	return &CheckpointAdapter{manager: manager, ctx: ctx}
}

// PhaseCheckpoint creates a checkpoint for phase transitions.
func (a *CheckpointAdapter) PhaseCheckpoint(state *core.WorkflowState, phase core.Phase, completed bool) error {
	return a.manager.PhaseCheckpoint(a.ctx, state, phase, completed)
}

// TaskCheckpoint creates a checkpoint for task transitions.
func (a *CheckpointAdapter) TaskCheckpoint(state *core.WorkflowState, task *core.Task, completed bool) error {
	return a.manager.TaskCheckpoint(a.ctx, state, task, completed)
}

// ConsensusCheckpoint creates a checkpoint after consensus evaluation.
func (a *CheckpointAdapter) ConsensusCheckpoint(state *core.WorkflowState, result ConsensusResult) error {
	// Convert workflow.ConsensusResult to service.ConsensusResult
	serviceResult := service.ConsensusResult{
		Score:            result.Score,
		NeedsV3:          result.NeedsV3,
		NeedsHumanReview: result.NeedsHumanReview,
	}
	return a.manager.ConsensusCheckpoint(a.ctx, state, serviceResult)
}

// ErrorCheckpoint creates a checkpoint on error.
func (a *CheckpointAdapter) ErrorCheckpoint(state *core.WorkflowState, err error) error {
	return a.manager.ErrorCheckpoint(a.ctx, state, err)
}

// CreateCheckpoint creates a checkpoint with custom type and metadata.
func (a *CheckpointAdapter) CreateCheckpoint(state *core.WorkflowState, checkpointType string, metadata map[string]interface{}) error {
	return a.manager.CreateCheckpoint(a.ctx, state, service.CheckpointType(checkpointType), metadata)
}

// RetryAdapter wraps service.RetryPolicy to satisfy RetryExecutor.
type RetryAdapter struct {
	policy *service.RetryPolicy
	ctx    context.Context
}

// NewRetryAdapter creates a new retry adapter.
func NewRetryAdapter(policy *service.RetryPolicy, ctx context.Context) *RetryAdapter {
	return &RetryAdapter{policy: policy, ctx: ctx}
}

// Execute runs the function with retry logic.
func (a *RetryAdapter) Execute(fn func() error) error {
	return a.policy.Execute(a.ctx, func(_ context.Context) error {
		return fn()
	})
}

// ExecuteWithNotify runs with retry and notifications.
func (a *RetryAdapter) ExecuteWithNotify(fn func() error, notify func(attempt int, err error)) error {
	return a.policy.ExecuteWithNotify(a.ctx, func(_ context.Context) error {
		return fn()
	}, func(attempt int, err error, _ time.Duration) {
		notify(attempt, err)
	})
}

// RateLimiterAdapter wraps service.RateLimiter to satisfy workflow.RateLimiter.
type RateLimiterAdapter struct {
	limiter *service.RateLimiter
	ctx     context.Context
}

// Acquire blocks until a token is available.
func (a *RateLimiterAdapter) Acquire() error {
	return a.limiter.Acquire(a.ctx)
}

// RateLimiterRegistryAdapter wraps service.RateLimiterRegistry to satisfy RateLimiterGetter.
type RateLimiterRegistryAdapter struct {
	registry *service.RateLimiterRegistry
	ctx      context.Context
}

// NewRateLimiterRegistryAdapter creates a new rate limiter registry adapter.
func NewRateLimiterRegistryAdapter(registry *service.RateLimiterRegistry, ctx context.Context) *RateLimiterRegistryAdapter {
	return &RateLimiterRegistryAdapter{registry: registry, ctx: ctx}
}

// Get returns the rate limiter for an agent.
func (a *RateLimiterRegistryAdapter) Get(agentName string) RateLimiter {
	return &RateLimiterAdapter{
		limiter: a.registry.Get(agentName),
		ctx:     a.ctx,
	}
}

// PromptRendererAdapter wraps service.PromptRenderer to satisfy workflow.PromptRenderer.
type PromptRendererAdapter struct {
	renderer *service.PromptRenderer
}

// NewPromptRendererAdapter creates a new prompt renderer adapter.
func NewPromptRendererAdapter(renderer *service.PromptRenderer) *PromptRendererAdapter {
	return &PromptRendererAdapter{renderer: renderer}
}

// RenderOptimizePrompt renders the prompt optimization template.
func (a *PromptRendererAdapter) RenderOptimizePrompt(params OptimizePromptParams) (string, error) {
	return a.renderer.RenderOptimizePrompt(service.OptimizePromptParams{
		OriginalPrompt: params.OriginalPrompt,
	})
}

// RenderAnalyzeV1 renders the initial analysis prompt.
func (a *PromptRendererAdapter) RenderAnalyzeV1(params AnalyzeV1Params) (string, error) {
	return a.renderer.RenderAnalyzeV1(service.AnalyzeV1Params{
		Prompt:  params.Prompt,
		Context: params.Context,
	})
}

// RenderAnalyzeV2 renders the critique analysis prompt.
func (a *PromptRendererAdapter) RenderAnalyzeV2(params AnalyzeV2Params) (string, error) {
	return a.renderer.RenderAnalyzeV2(service.AnalyzeV2Params{
		Prompt:     params.Prompt,
		V1Analysis: params.V1Analysis,
		AgentName:  params.AgentName,
	})
}

// RenderAnalyzeV3 renders the reconciliation prompt.
func (a *PromptRendererAdapter) RenderAnalyzeV3(params AnalyzeV3Params) (string, error) {
	// Convert []string divergences to []service.Divergence
	divergences := make([]service.Divergence, len(params.Divergences))
	for i, d := range params.Divergences {
		divergences[i] = service.Divergence{Category: d}
	}
	return a.renderer.RenderAnalyzeV3(service.AnalyzeV3Params{
		Prompt:      params.Prompt,
		V1Analysis:  params.V1Analysis,
		V2Analysis:  params.V2Analysis,
		Divergences: divergences,
	})
}

// RenderConsolidateAnalysis renders the analysis consolidation prompt.
func (a *PromptRendererAdapter) RenderConsolidateAnalysis(params ConsolidateAnalysisParams) (string, error) {
	// Convert workflow.AnalysisOutput to service.AnalysisOutput
	serviceAnalyses := make([]service.AnalysisOutput, len(params.Analyses))
	for i, ao := range params.Analyses {
		serviceAnalyses[i] = service.AnalysisOutput{
			AgentName:       ao.AgentName,
			RawOutput:       ao.RawOutput,
			Claims:          ao.Claims,
			Risks:           ao.Risks,
			Recommendations: ao.Recommendations,
		}
	}
	return a.renderer.RenderConsolidateAnalysis(service.ConsolidateAnalysisParams{
		Prompt:   params.Prompt,
		Analyses: serviceAnalyses,
	})
}

// RenderPlanGenerate renders the plan generation prompt.
func (a *PromptRendererAdapter) RenderPlanGenerate(params PlanParams) (string, error) {
	return a.renderer.RenderPlanGenerate(service.PlanParams{
		Prompt:               params.Prompt,
		ConsolidatedAnalysis: params.ConsolidatedAnalysis,
		MaxTasks:             params.MaxTasks,
	})
}

// RenderTaskExecute renders the task execution prompt.
func (a *PromptRendererAdapter) RenderTaskExecute(params TaskExecuteParams) (string, error) {
	return a.renderer.RenderTaskExecute(service.TaskExecuteParams{
		Task:    params.Task,
		Context: params.Context,
	})
}

// ResumePointAdapter wraps service.CheckpointManager to satisfy ResumePointProvider.
type ResumePointAdapter struct {
	manager *service.CheckpointManager
}

// NewResumePointAdapter creates a new resume point adapter.
func NewResumePointAdapter(manager *service.CheckpointManager) *ResumePointAdapter {
	return &ResumePointAdapter{manager: manager}
}

// GetResumePoint determines where to resume a workflow.
func (a *ResumePointAdapter) GetResumePoint(state *core.WorkflowState) (*ResumePoint, error) {
	rp, err := a.manager.GetResumePoint(state)
	if err != nil {
		return nil, err
	}
	return &ResumePoint{
		Phase:     rp.Phase,
		TaskID:    rp.TaskID,
		FromStart: rp.FromStart,
	}, nil
}

// DAGAdapter wraps service.DAGBuilder to satisfy both DAGBuilder and TaskDAG interfaces.
type DAGAdapter struct {
	dag *service.DAGBuilder
}

// NewDAGAdapter creates a new DAG adapter.
func NewDAGAdapter(dag *service.DAGBuilder) *DAGAdapter {
	return &DAGAdapter{dag: dag}
}

// AddTask adds a task to the DAG.
func (a *DAGAdapter) AddTask(task *core.Task) error {
	return a.dag.AddTask(task)
}

// AddDependency adds a dependency: from depends on to.
func (a *DAGAdapter) AddDependency(from, to core.TaskID) error {
	return a.dag.AddDependency(from, to)
}

// Build validates the DAG and returns the state.
func (a *DAGAdapter) Build() (interface{}, error) {
	return a.dag.Build()
}

// GetReadyTasks returns tasks ready for execution.
func (a *DAGAdapter) GetReadyTasks(completed map[core.TaskID]bool) []*core.Task {
	return a.dag.GetReadyTasks(completed)
}
