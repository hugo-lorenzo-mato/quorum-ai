package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// OutputNotifier provides real-time updates to the UI/output layer.
// This interface mirrors tui.Output but is defined here to avoid circular imports.
type OutputNotifier interface {
	// PhaseStarted is called when a phase begins.
	PhaseStarted(phase core.Phase)
	// TaskStarted is called when a task begins.
	TaskStarted(task *core.Task)
	// TaskCompleted is called when a task finishes successfully.
	TaskCompleted(task *core.Task, duration time.Duration)
	// TaskFailed is called when a task fails.
	TaskFailed(task *core.Task, err error)
	// WorkflowStateUpdated is called when the workflow state changes (e.g., tasks created).
	WorkflowStateUpdated(state *core.WorkflowState)
}

// NopOutputNotifier is a no-op implementation of OutputNotifier.
type NopOutputNotifier struct{}

func (n NopOutputNotifier) PhaseStarted(_ core.Phase)                   {}
func (n NopOutputNotifier) TaskStarted(_ *core.Task)                    {}
func (n NopOutputNotifier) TaskCompleted(_ *core.Task, _ time.Duration) {}
func (n NopOutputNotifier) TaskFailed(_ *core.Task, _ error)            {}
func (n NopOutputNotifier) WorkflowStateUpdated(_ *core.WorkflowState)  {}

// Context provides shared resources for workflow phases.
// It encapsulates the runtime state and dependencies needed
// by all phase runners.
type Context struct {
	State      *core.WorkflowState
	Agents     core.AgentRegistry
	Prompts    PromptRenderer
	Checkpoint CheckpointCreator
	Retry      RetryExecutor
	RateLimits RateLimiterGetter
	Worktrees  WorktreeManager
	Logger     *logging.Logger
	Config     *Config
	Output     OutputNotifier
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
}

// PromptRenderer renders prompts for different phases.
type PromptRenderer interface {
	RenderAnalyzeV1(params AnalyzeV1Params) (string, error)
	RenderAnalyzeV2(params AnalyzeV2Params) (string, error)
	RenderAnalyzeV3(params AnalyzeV3Params) (string, error)
	RenderPlanGenerate(params PlanParams) (string, error)
	RenderTaskExecute(params TaskExecuteParams) (string, error)
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
	Divergences      []string
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
