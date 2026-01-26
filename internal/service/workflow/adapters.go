// Package workflow provides adapters to bridge service layer types
// to workflow interface requirements.
package workflow

import (
	"context"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

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

// ErrorCheckpoint creates a checkpoint on error.
func (a *CheckpointAdapter) ErrorCheckpoint(state *core.WorkflowState, err error) error {
	return a.manager.ErrorCheckpoint(a.ctx, state, err)
}

// ErrorCheckpointWithContext creates a detailed error checkpoint with full context.
func (a *CheckpointAdapter) ErrorCheckpointWithContext(state *core.WorkflowState, err error, details service.ErrorCheckpointDetails) error {
	return a.manager.ErrorCheckpointWithContext(a.ctx, state, err, details)
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

// RenderRefinePrompt renders the prompt refinement template.
func (a *PromptRendererAdapter) RenderRefinePrompt(params RefinePromptParams) (string, error) {
	return a.renderer.RenderRefinePrompt(service.RefinePromptParams{
		OriginalPrompt: params.OriginalPrompt,
	})
}

// RenderAnalyzeV1 renders the initial analysis prompt.
func (a *PromptRendererAdapter) RenderAnalyzeV1(params AnalyzeV1Params) (string, error) {
	return a.renderer.RenderAnalyzeV1(service.AnalyzeV1Params{
		Prompt:         params.Prompt,
		Context:        params.Context,
		OutputFilePath: params.OutputFilePath,
	})
}

// RenderSynthesizeAnalysis renders the analysis synthesis prompt.
func (a *PromptRendererAdapter) RenderSynthesizeAnalysis(params SynthesizeAnalysisParams) (string, error) {
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
	return a.renderer.RenderSynthesizeAnalysis(service.SynthesizeAnalysisParams{
		Prompt:         params.Prompt,
		Analyses:       serviceAnalyses,
		OutputFilePath: params.OutputFilePath,
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

// RenderPlanManifest renders the plan manifest prompt.
func (a *PromptRendererAdapter) RenderPlanManifest(params PlanParams) (string, error) {
	return a.renderer.RenderPlanManifest(service.PlanParams{
		Prompt:               params.Prompt,
		ConsolidatedAnalysis: params.ConsolidatedAnalysis,
		MaxTasks:             params.MaxTasks,
	})
}

// RenderPlanComprehensive renders the comprehensive single-call planning prompt.
func (a *PromptRendererAdapter) RenderPlanComprehensive(params ComprehensivePlanParams) (string, error) {
	// Convert AgentInfo to service layer types
	agents := make([]service.AgentInfo, len(params.AvailableAgents))
	for i, ag := range params.AvailableAgents {
		agents[i] = service.AgentInfo{
			Name:         ag.Name,
			Model:        ag.Model,
			Strengths:    ag.Strengths,
			Capabilities: ag.Capabilities,
		}
	}

	return a.renderer.RenderPlanComprehensive(service.ComprehensivePlanParams{
		Prompt:               params.Prompt,
		ConsolidatedAnalysis: params.ConsolidatedAnalysis,
		AvailableAgents:      agents,
		TasksDir:             params.TasksDir,
		NamingConvention:     params.NamingConvention,
	})
}

// RenderTaskDetailGenerate renders the task detail generation prompt.
func (a *PromptRendererAdapter) RenderTaskDetailGenerate(params TaskDetailGenerateParams) (string, error) {
	return a.renderer.RenderTaskDetailGenerate(service.TaskDetailGenerateParams{
		TaskID:               params.TaskID,
		TaskName:             params.TaskName,
		Dependencies:         params.Dependencies,
		OutputPath:           params.OutputPath,
		ConsolidatedAnalysis: params.ConsolidatedAnalysis,
	})
}

// RenderSynthesizePlans renders the multi-agent plan synthesis prompt.
func (a *PromptRendererAdapter) RenderSynthesizePlans(params SynthesizePlansParams) (string, error) {
	// Convert PlanOutput to service layer types
	plans := make([]service.PlanProposal, len(params.Plans))
	for i, p := range params.Plans {
		plans[i] = service.PlanProposal{
			AgentName: p.AgentName,
			Model:     p.Model,
			Content:   p.RawOutput,
		}
	}

	return a.renderer.RenderSynthesizePlans(service.SynthesizePlansParams{
		Prompt:   params.Prompt,
		Analysis: params.Analysis,
		Plans:    plans,
		MaxTasks: params.MaxTasks,
	})
}

// RenderTaskExecute renders the task execution prompt.
func (a *PromptRendererAdapter) RenderTaskExecute(params TaskExecuteParams) (string, error) {
	return a.renderer.RenderTaskExecute(service.TaskExecuteParams{
		Task:    params.Task,
		Context: params.Context,
	})
}

// RenderModeratorEvaluate renders the semantic moderator evaluation prompt.
func (a *PromptRendererAdapter) RenderModeratorEvaluate(params ModeratorEvaluateParams) (string, error) {
	// Convert workflow.ModeratorAnalysisSummary to service.ModeratorAnalysisSummary
	serviceAnalyses := make([]service.ModeratorAnalysisSummary, len(params.Analyses))
	for i, analysis := range params.Analyses {
		serviceAnalyses[i] = service.ModeratorAnalysisSummary{
			AgentName: analysis.AgentName,
			FilePath:  analysis.FilePath,
		}
	}
	return a.renderer.RenderModeratorEvaluate(service.ModeratorEvaluateParams{
		Prompt:         params.Prompt,
		Round:          params.Round,
		NextRound:      params.Round + 1,
		Analyses:       serviceAnalyses,
		BelowThreshold: params.BelowThreshold,
		OutputFilePath: params.OutputFilePath,
	})
}

// RenderVnRefine renders the V(n) refinement prompt.
func (a *PromptRendererAdapter) RenderVnRefine(params VnRefineParams) (string, error) {
	// Convert workflow.VnDivergenceInfo to service.VnDivergenceInfo
	serviceDivergences := make([]service.VnDivergenceInfo, len(params.Divergences))
	for i, div := range params.Divergences {
		serviceDivergences[i] = service.VnDivergenceInfo{
			Category:       div.Category,
			YourPosition:   div.YourPosition,
			OtherPositions: div.OtherPositions,
			Guidance:       div.Guidance,
		}
	}
	return a.renderer.RenderVnRefine(service.VnRefineParams{
		Prompt:               params.Prompt,
		Context:              params.Context,
		Round:                params.Round,
		PreviousRound:        params.PreviousRound,
		PreviousAnalysis:     params.PreviousAnalysis,
		HasArbiterEvaluation: params.HasArbiterEvaluation,
		ConsensusScore:       params.ConsensusScore,
		Threshold:            params.Threshold,
		Agreements:           params.Agreements,
		Divergences:          serviceDivergences,
		MissingPerspectives:  params.MissingPerspectives,
		Constraints:          params.Constraints,
		OutputFilePath:       params.OutputFilePath,
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

// Clear removes all tasks from the DAG.
func (a *DAGAdapter) Clear() {
	a.dag.Clear()
}

// GetReadyTasks returns tasks ready for execution.
func (a *DAGAdapter) GetReadyTasks(completed map[core.TaskID]bool) []*core.Task {
	return a.dag.GetReadyTasks(completed)
}

// ModeEnforcerAdapter wraps service.ModeEnforcer to satisfy ModeEnforcerInterface.
type ModeEnforcerAdapter struct {
	enforcer *service.ModeEnforcer
}

// NewModeEnforcerAdapter creates a new mode enforcer adapter.
func NewModeEnforcerAdapter(enforcer *service.ModeEnforcer) *ModeEnforcerAdapter {
	if enforcer == nil {
		return nil
	}
	return &ModeEnforcerAdapter{enforcer: enforcer}
}

// CanExecute implements ModeEnforcerInterface.
func (a *ModeEnforcerAdapter) CanExecute(ctx context.Context, op ModeOperation) error {
	if a.enforcer == nil {
		return nil
	}

	serviceOp := service.Operation{
		Name:                 op.Name,
		Type:                 service.OperationType(op.Type),
		Tool:                 op.Tool,
		HasSideEffects:       op.HasSideEffects,
		RequiresConfirmation: op.RequiresConfirmation,
		InWorkspace:          op.InWorkspace,
		AllowedInSandbox:     op.AllowedInSandbox,
		IsDestructive:        op.IsDestructive,
	}

	return a.enforcer.CanExecute(ctx, serviceOp)
}

// IsSandboxed implements ModeEnforcerInterface.
func (a *ModeEnforcerAdapter) IsSandboxed() bool {
	if a.enforcer == nil {
		return false
	}
	return a.enforcer.Mode().Sandbox
}

// IsDryRun implements ModeEnforcerInterface.
func (a *ModeEnforcerAdapter) IsDryRun() bool {
	if a.enforcer == nil {
		return false
	}
	return a.enforcer.Mode().DryRun
}
