package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// OutputNotifier provides real-time updates to the UI/output layer.
// This interface is a subset of tui.Output, defined here to avoid circular imports.
// NOTE: This intentionally has fewer methods than tui.Output.
// Missing methods (WorkflowStarted, WorkflowCompleted, WorkflowFailed) are
// handled directly by the CLI layer, not by the workflow runner.
type OutputNotifier interface {
	// PhaseStarted is called when a phase begins.
	PhaseStarted(phase core.Phase)
	// TaskStarted is called when a task begins.
	TaskStarted(task *core.Task)
	// TaskCompleted is called when a task finishes successfully.
	TaskCompleted(task *core.Task, duration time.Duration)
	// TaskFailed is called when a task fails.
	TaskFailed(task *core.Task, err error)
	// TaskSkipped is called when a task is skipped.
	TaskSkipped(task *core.Task, reason string)
	// WorkflowStateUpdated is called when the workflow state changes (e.g., tasks created).
	// NOTE: This is semantically different from WorkflowCompleted.
	WorkflowStateUpdated(state *core.WorkflowState)
	// Log sends a log message to the UI.
	// level can be "info", "warn", "error", "success", "debug".
	// source identifies the origin (e.g., "workflow", "optimizer", "analyzer", "executor").
	Log(level, source, message string)
	// AgentEvent is called when an agent emits a streaming event.
	// kind can be "started", "tool_use", "thinking", "chunk", "progress", "completed", "error".
	AgentEvent(kind, agent, message string, data map[string]interface{})
}

// NopOutputNotifier is a no-op implementation of OutputNotifier.
type NopOutputNotifier struct{}

func (n NopOutputNotifier) PhaseStarted(_ core.Phase)                   {}
func (n NopOutputNotifier) TaskStarted(_ *core.Task)                    {}
func (n NopOutputNotifier) TaskCompleted(_ *core.Task, _ time.Duration) {}
func (n NopOutputNotifier) TaskFailed(_ *core.Task, _ error)            {}
func (n NopOutputNotifier) TaskSkipped(_ *core.Task, _ string)          {}
func (n NopOutputNotifier) WorkflowStateUpdated(_ *core.WorkflowState)       {}
func (n NopOutputNotifier) Log(_, _, _ string)                               {}
func (n NopOutputNotifier) AgentEvent(_, _, _ string, _ map[string]interface{}) {}

// Context provides shared resources for workflow phases.
// It encapsulates the runtime state and dependencies needed
// by all phase runners.
type Context struct {
	mu           sync.RWMutex // protects State during concurrent access
	State        *core.WorkflowState
	Agents       core.AgentRegistry
	Prompts      PromptRenderer
	Checkpoint   CheckpointCreator
	Retry        RetryExecutor
	RateLimits   RateLimiterGetter
	Worktrees    WorktreeManager
	Logger       *logging.Logger
	Config       *Config
	Output       OutputNotifier
	ModeEnforcer ModeEnforcerInterface
	Control      *control.ControlPlane
	Report       *report.WorkflowReportWriter // Writes analysis/plan/execute reports to markdown
}

// ModeEnforcerInterface provides mode enforcement capabilities.
// This interface wraps service.ModeEnforcer to avoid circular imports.
type ModeEnforcerInterface interface {
	// CanExecute checks if an operation can be executed.
	CanExecute(ctx context.Context, op ModeOperation) error
	// RecordCost records cost for an operation.
	RecordCost(cost float64)
	// IsSandboxed returns whether sandbox mode is enabled.
	IsSandboxed() bool
	// IsDryRun returns whether dry-run mode is enabled.
	IsDryRun() bool
}

// ModeOperation represents an operation to validate.
type ModeOperation struct {
	Name                 string
	Type                 string // llm, file_read, file_write, git, network, shell
	Tool                 string
	HasSideEffects       bool
	RequiresConfirmation bool
	EstimatedCost        float64
	InWorkspace          bool
	AllowedInSandbox     bool
	IsDestructive        bool
}

// Config holds workflow configuration.
type Config struct {
	DryRun       bool
	Sandbox      bool
	DenyTools    []string
	DefaultAgent string
	V3Agent      string
	// AgentPhaseModels allows per-agent, per-phase model overrides.
	AgentPhaseModels map[string]map[string]string
	// WorktreeAutoClean controls automatic worktree cleanup after task execution.
	WorktreeAutoClean bool
	// WorktreeMode controls when worktrees are created for tasks.
	WorktreeMode string
	// MaxCostPerWorkflow is the maximum total cost for the workflow in USD (0 = unlimited).
	MaxCostPerWorkflow float64
	// MaxCostPerTask is the maximum cost per task in USD (0 = unlimited).
	MaxCostPerTask float64
	// ConsolidatorAgent specifies which agent to use for analysis consolidation.
	ConsolidatorAgent string
	// ConsolidatorModel specifies the model to use for consolidation (optional).
	ConsolidatorModel string
	// PhaseTimeouts holds per-phase timeout durations.
	PhaseTimeouts PhaseTimeouts
}

// PhaseTimeouts holds timeout durations for each workflow phase.
type PhaseTimeouts struct {
	Analyze time.Duration
	Plan    time.Duration
	Execute time.Duration
}

// PromptRenderer renders prompts for different phases.
type PromptRenderer interface {
	RenderOptimizePrompt(params OptimizePromptParams) (string, error)
	RenderAnalyzeV1(params AnalyzeV1Params) (string, error)
	RenderAnalyzeV2(params AnalyzeV2Params) (string, error)
	RenderAnalyzeV3(params AnalyzeV3Params) (string, error)
	RenderConsolidateAnalysis(params ConsolidateAnalysisParams) (string, error)
	RenderPlanGenerate(params PlanParams) (string, error)
	RenderTaskExecute(params TaskExecuteParams) (string, error)
}

// ConsolidateAnalysisParams holds parameters for analysis consolidation prompt.
type ConsolidateAnalysisParams struct {
	Prompt   string
	Analyses []AnalysisOutput
}

// OptimizePromptParams holds parameters for prompt optimization.
type OptimizePromptParams struct {
	OriginalPrompt string
}

// AnalyzeV1Params holds parameters for V1 analysis prompt.
type AnalyzeV1Params struct {
	Prompt  string
	Context string
}

// AnalyzeV2Params holds parameters for V2 critique prompt.
type AnalyzeV2Params struct {
	Prompt     string
	V1Analysis string
	AgentName  string
}

// AnalyzeV3Params holds parameters for V3 reconciliation prompt.
type AnalyzeV3Params struct {
	Prompt      string
	V1Analysis  string
	V2Analysis  string
	Divergences []string
}

// PlanParams holds parameters for plan generation prompt.
type PlanParams struct {
	Prompt               string
	ConsolidatedAnalysis string
	MaxTasks             int
}

// TaskExecuteParams holds parameters for task execution prompt.
type TaskExecuteParams struct {
	Task    *core.Task
	Context string
}

// CheckpointCreator creates checkpoints during workflow execution.
type CheckpointCreator interface {
	PhaseCheckpoint(state *core.WorkflowState, phase core.Phase, completed bool) error
	TaskCheckpoint(state *core.WorkflowState, task *core.Task, completed bool) error
	ConsensusCheckpoint(state *core.WorkflowState, result ConsensusResult) error
	ErrorCheckpoint(state *core.WorkflowState, err error) error
	CreateCheckpoint(state *core.WorkflowState, checkpointType string, metadata map[string]interface{}) error
}

// ConsensusResult represents the result of consensus evaluation.
type ConsensusResult struct {
	Score            float64
	NeedsV3          bool
	NeedsHumanReview bool
	CategoryScores   map[string]float64
	Divergences      []Divergence
	Agreement        map[string][]string
}

// Divergence represents a disagreement between agents.
type Divergence struct {
	Category     string
	Agent1       string
	Agent1Items  []string
	Agent2       string
	Agent2Items  []string
	JaccardScore float64
}

// DivergenceStrings returns a simplified string representation for logging.
func (r ConsensusResult) DivergenceStrings() []string {
	result := make([]string, len(r.Divergences))
	for i, d := range r.Divergences {
		result[i] = d.Category + ": " + d.Agent1 + " vs " + d.Agent2
	}
	return result
}

// RetryExecutor provides retry capabilities.
type RetryExecutor interface {
	Execute(fn func() error) error
	ExecuteWithNotify(fn func() error, notify func(attempt int, err error)) error
}

// RateLimiterGetter provides rate limiters for agents.
type RateLimiterGetter interface {
	Get(agentName string) RateLimiter
}

// RateLimiter controls request rate.
type RateLimiter interface {
	Acquire() error
}

// WorktreeManager manages git worktrees for task isolation.
type WorktreeManager interface {
	// Create creates a new worktree for a task.
	Create(ctx context.Context, task *core.Task, branch string) (*core.WorktreeInfo, error)
	// Get retrieves worktree info for a task.
	Get(ctx context.Context, task *core.Task) (*core.WorktreeInfo, error)
	// Remove cleans up a task's worktree.
	Remove(ctx context.Context, task *core.Task) error
	// CleanupStale removes worktrees for completed/failed tasks.
	CleanupStale(ctx context.Context) error
}

// BuildContextString constructs a context string from workflow state.
// Note: This function is not thread-safe. Use Context.GetContextString() for concurrent access.
func BuildContextString(state *core.WorkflowState) string {
	var ctx strings.Builder
	ctx.WriteString(fmt.Sprintf("Workflow: %s\n", state.WorkflowID))
	ctx.WriteString(fmt.Sprintf("Phase: %s\n", state.CurrentPhase))

	for _, id := range state.TaskOrder {
		task := state.Tasks[id]
		if task != nil && task.Status == core.TaskStatusCompleted {
			ctx.WriteString(fmt.Sprintf("- Completed: %s\n", task.Name))
		}
	}

	return ctx.String()
}

// GetContextString returns a context string with thread-safe access to state.
func (c *Context) GetContextString() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return BuildContextString(c.State)
}

// Lock acquires a write lock on the context state.
func (c *Context) Lock() {
	c.mu.Lock()
}

// Unlock releases the write lock on the context state.
func (c *Context) Unlock() {
	c.mu.Unlock()
}

// RLock acquires a read lock on the context state.
func (c *Context) RLock() {
	c.mu.RLock()
}

// RUnlock releases the read lock on the context state.
func (c *Context) RUnlock() {
	c.mu.RUnlock()
}

// UpdateMetrics safely updates workflow metrics.
func (c *Context) UpdateMetrics(fn func(m *core.StateMetrics)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State.Metrics == nil {
		c.State.Metrics = &core.StateMetrics{}
	}
	fn(c.State.Metrics)
}

// ResolvePhaseModel returns the model override for a given agent/phase.
// Priority: phase override > task model > empty (use agent default).
func ResolvePhaseModel(cfg *Config, agentName string, phase core.Phase, taskModel string) string {
	if strings.TrimSpace(taskModel) != "" {
		return taskModel
	}
	if cfg != nil && cfg.AgentPhaseModels != nil {
		if phaseModels, ok := cfg.AgentPhaseModels[agentName]; ok {
			if model, ok := phaseModels[phase.String()]; ok && strings.TrimSpace(model) != "" {
				return model
			}
		}
	}
	return ""
}
