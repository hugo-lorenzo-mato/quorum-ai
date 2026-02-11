package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
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

func (n NopOutputNotifier) PhaseStarted(_ core.Phase)                           {}
func (n NopOutputNotifier) TaskStarted(_ *core.Task)                            {}
func (n NopOutputNotifier) TaskCompleted(_ *core.Task, _ time.Duration)         {}
func (n NopOutputNotifier) TaskFailed(_ *core.Task, _ error)                    {}
func (n NopOutputNotifier) TaskSkipped(_ *core.Task, _ string)                  {}
func (n NopOutputNotifier) WorkflowStateUpdated(_ *core.WorkflowState)          {}
func (n NopOutputNotifier) Log(_, _, _ string)                                  {}
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
	Git          core.GitClient    // Git operations for commit/push
	GitHub       core.GitHubClient // GitHub operations for PR creation
	Logger       *logging.Logger
	Config       *Config
	Output       OutputNotifier
	ModeEnforcer ModeEnforcerInterface
	Control      *control.ControlPlane
	Report       *report.WorkflowReportWriter // Writes analysis/plan/execute reports to markdown

	// Workflow-level Git isolation
	WorkflowWorktrees core.WorkflowWorktreeManager // Workflow-scoped worktree manager
	GitIsolation      *GitIsolationConfig          // Git isolation configuration

	// Project root directory for multi-project support.
	// Used as fallback working directory when worktrees are not enabled.
	ProjectRoot string
}

// ModeEnforcerInterface provides mode enforcement capabilities.
// This interface wraps service.ModeEnforcer to avoid circular imports.
type ModeEnforcerInterface interface {
	// CanExecute checks if an operation can be executed.
	CanExecute(ctx context.Context, op ModeOperation) error
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
	InWorkspace          bool
	IsDestructive        bool
}

// Config holds workflow configuration.
type Config struct {
	DryRun       bool
	DenyTools    []string
	DefaultAgent string
	// AgentPhaseModels allows per-agent, per-phase model overrides.
	AgentPhaseModels map[string]map[string]string
	// WorktreeAutoClean controls automatic worktree cleanup after task execution.
	WorktreeAutoClean bool
	// WorktreeMode controls when worktrees are created for tasks.
	WorktreeMode string
	// SynthesizerAgent specifies which agent to use for analysis synthesis.
	// The model is resolved from AgentPhaseModels[agent][analyze].
	SynthesizerAgent string
	// PlanSynthesizerEnabled controls whether multi-agent planning is used.
	PlanSynthesizerEnabled bool
	// PlanSynthesizerAgent specifies which agent to use for plan synthesis.
	// The model is resolved from AgentPhaseModels[agent][plan].
	PlanSynthesizerAgent string
	// PhaseTimeouts holds per-phase timeout durations.
	PhaseTimeouts PhaseTimeouts
	// Moderator configures semantic consensus evaluation via a moderator LLM.
	Moderator ModeratorConfig
	// SingleAgent configures single-agent execution mode (bypasses multi-agent consensus).
	SingleAgent SingleAgentConfig
	// Finalization configures post-task git operations.
	Finalization FinalizationConfig
	// ProjectAgentPhases holds project-specific phase configuration per agent.
	// Used in multi-project scenarios where each project may have different agent phases.
	ProjectAgentPhases map[string][]string
}

// ModeratorConfig configures the semantic moderator LLM for consensus evaluation.
type ModeratorConfig struct {
	// Enabled activates semantic consensus evaluation via a moderator LLM.
	Enabled bool
	// Agent specifies which agent to use as moderator (claude, gemini, codex, copilot).
	// The model is resolved from AgentPhaseModels[agent][analyze].
	Agent string
	// Threshold is the semantic consensus score required to pass (0.0-1.0, default: 0.90).
	Threshold float64
	// Thresholds provides adaptive thresholds based on task type.
	// Keys: "analysis", "design", "bugfix", "refactor". If a task type matches,
	// its threshold is used instead of the default Threshold.
	Thresholds map[string]float64
	// MinSuccessfulAgents is the minimum number of agents that must succeed
	// in a given analysis/refinement round before continuing (default: 2).
	MinSuccessfulAgents int
	// MinRounds is the minimum number of rounds before accepting consensus (default: 2).
	MinRounds int
	// MaxRounds limits the number of V(n) refinement rounds (default: 5).
	MaxRounds int
	// WarningThreshold logs a warning if consensus score drops below this (default: 0.30).
	WarningThreshold float64
	// StagnationThreshold triggers early exit if score improvement is below this (default: 0.02).
	StagnationThreshold float64
}

// SingleAgentConfig configures single-agent execution mode for the analyze phase.
// When Enabled is true, the Analyzer bypasses multi-agent consensus.
type SingleAgentConfig struct {
	// Enabled activates single-agent mode, bypassing multi-agent consensus.
	Enabled bool
	// Agent is the name of the agent to use for single-agent analysis.
	Agent string
	// Model is an optional override for the agent's default model.
	// If empty, the agent's configured default model is used.
	Model string
	// ReasoningEffort is an optional override for the agent's reasoning effort.
	// Only supported by certain agents/models (e.g., Codex). Empty means use agent defaults.
	ReasoningEffort string
}

// PhaseTimeouts holds timeout durations for each workflow phase.
type PhaseTimeouts struct {
	Analyze            time.Duration
	Plan               time.Duration
	Execute            time.Duration
	ProcessGracePeriod time.Duration // Time to wait after logical completion before killing (default: 30s)
}

// FinalizationConfig configures post-task git operations.
type FinalizationConfig struct {
	// AutoCommit commits changes after each task completes.
	AutoCommit bool
	// AutoPush pushes the task branch to remote after commit.
	AutoPush bool
	// AutoPR creates a pull request for each task branch.
	AutoPR bool
	// AutoMerge merges the PR automatically after creation.
	AutoMerge bool
	// PRBaseBranch is the target branch for PRs (empty = use current branch).
	PRBaseBranch string
	// MergeStrategy for auto-merge: merge, squash, rebase (default: squash).
	MergeStrategy string
	// Remote is the git remote name (default: origin).
	Remote string
}

// PromptRenderer renders prompts for different phases.
type PromptRenderer interface {
	RenderRefinePrompt(params RefinePromptParams) (string, error)
	RenderAnalyzeV1(params AnalyzeV1Params) (string, error)
	RenderSynthesizeAnalysis(params SynthesizeAnalysisParams) (string, error)
	RenderPlanGenerate(params PlanParams) (string, error)
	RenderPlanManifest(params PlanParams) (string, error)
	RenderPlanComprehensive(params ComprehensivePlanParams) (string, error)
	RenderSynthesizePlans(params SynthesizePlansParams) (string, error)
	RenderTaskExecute(params TaskExecuteParams) (string, error)
	RenderTaskDetailGenerate(params TaskDetailGenerateParams) (string, error)
	RenderModeratorEvaluate(params ModeratorEvaluateParams) (string, error)
	RenderVnRefine(params VnRefineParams) (string, error)
}

// TaskDetailGenerateParams holds parameters for CLI-driven task detail generation.
type TaskDetailGenerateParams struct {
	TaskID               string
	TaskName             string
	Dependencies         []string
	OutputPath           string
	ConsolidatedAnalysis string
}

// SynthesizeAnalysisParams holds parameters for analysis synthesis prompt.
type SynthesizeAnalysisParams struct {
	Prompt         string
	Analyses       []AnalysisOutput
	OutputFilePath string // Path where LLM should write output
}

// RefinePromptParams holds parameters for prompt refinement.
type RefinePromptParams struct {
	OriginalPrompt string
	Template       string
}

// AnalyzeV1Params holds parameters for V1 analysis prompt.
type AnalyzeV1Params struct {
	Prompt         string
	Context        string
	OutputFilePath string // Path where LLM should write output
}

// PlanParams holds parameters for plan generation prompt.
type PlanParams struct {
	Prompt               string
	ConsolidatedAnalysis string
	MaxTasks             int
}

// AgentInfo contains information about an available agent for task assignment.
type AgentInfo struct {
	Name         string // Agent identifier (e.g., "claude", "codex")
	Model        string // Model being used
	Strengths    string // Human-readable description of agent strengths
	Capabilities string // List of capabilities (e.g., "JSON, streaming, tools")
}

// ComprehensivePlanParams holds parameters for single-call comprehensive planning.
// This is used when the CLI generates both the task breakdown AND all task files.
type ComprehensivePlanParams struct {
	Prompt               string      // Original user request
	ConsolidatedAnalysis string      // Complete consolidated analysis
	AvailableAgents      []AgentInfo // Agents available for task execution
	TasksDir             string      // Directory where task files should be written
	NamingConvention     string      // File naming convention (e.g., "{id}-{name}.md")
}

// TaskExecuteParams holds parameters for task execution prompt.
type TaskExecuteParams struct {
	Task    *core.Task
	Context string
	WorkDir string
	// Constraints are optional additional rules for the agent (e.g., policy limits).
	Constraints []string
}

// ModeratorAnalysisSummary represents an analysis for moderator evaluation.
type ModeratorAnalysisSummary struct {
	AgentName string
	FilePath  string // Path to the analysis file for the moderator to read
}

// ModeratorEvaluateParams holds parameters for moderator semantic evaluation prompt.
type ModeratorEvaluateParams struct {
	Prompt         string
	Round          int
	NextRound      int // Round + 1, for recommendations to agents
	Analyses       []ModeratorAnalysisSummary
	BelowThreshold bool
	OutputFilePath string // Path where LLM should write moderator report
}

// VnDivergenceInfo contains divergence information for V(n) refinement.
type VnDivergenceInfo struct {
	Category       string
	YourPosition   string
	OtherPositions string
	Guidance       string
}

// VnRefineParams holds parameters for V(n) refinement prompt.
type VnRefineParams struct {
	Prompt               string
	Context              string
	Round                int
	PreviousRound        int
	PreviousAnalysis     string
	HasArbiterEvaluation bool    // True if arbiter has evaluated (V3+), false for V2
	ConsensusScore       float64 // Only meaningful if HasArbiterEvaluation is true
	Threshold            float64
	Agreements           []string
	Divergences          []VnDivergenceInfo
	MissingPerspectives  []string
	Constraints          []string
	OutputFilePath       string // Path where LLM should write output
}

// CheckpointCreator creates checkpoints during workflow execution.
type CheckpointCreator interface {
	PhaseCheckpoint(state *core.WorkflowState, phase core.Phase, completed bool) error
	TaskCheckpoint(state *core.WorkflowState, task *core.Task, completed bool) error
	ErrorCheckpoint(state *core.WorkflowState, err error) error
	ErrorCheckpointWithContext(state *core.WorkflowState, err error, details service.ErrorCheckpointDetails) error
	CreateCheckpoint(state *core.WorkflowState, checkpointType string, metadata map[string]interface{}) error
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
	// CreateFromBranch creates a new worktree for a task from a specified base branch.
	// This is useful for dependent tasks that need to start from another task's branch.
	CreateFromBranch(ctx context.Context, task *core.Task, branch, baseBranch string) (*core.WorktreeInfo, error)
	// Get retrieves worktree info for a task.
	Get(ctx context.Context, task *core.Task) (*core.WorktreeInfo, error)
	// Remove cleans up a task's worktree.
	Remove(ctx context.Context, task *core.Task) error
	// CleanupStale removes worktrees for completed/failed tasks.
	CleanupStale(ctx context.Context) error
	// List returns all managed worktrees.
	List(ctx context.Context) ([]*core.WorktreeInfo, error)
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

// ResolveFilePath returns an absolute path for filesystem operations (os.Stat, os.ReadFile).
// If the path is already absolute, it's returned as-is.
// If the path is relative and ProjectRoot is set, it's resolved against ProjectRoot.
// This ensures file checks work correctly in multi-project mode where the server's
// CWD may differ from the project root.
func (c *Context) ResolveFilePath(path string) string {
	if path == "" {
		return path
	}
	// Check for absolute paths (including Unix-style paths on Windows)
	if filepath.IsAbs(path) {
		return path
	}
	// On Windows, filepath.IsAbs("/unix/path") returns false
	// But such paths should be treated as absolute
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return path
	}
	if c != nil && c.ProjectRoot != "" {
		return filepath.Join(c.ProjectRoot, path)
	}
	return path
}

// CheckControl applies workflow-level control checks (cancel + pause).
// It returns immediately when no ControlPlane is configured.
func (c *Context) CheckControl(ctx context.Context) error {
	if c == nil || c.Control == nil {
		return nil
	}
	if err := c.Control.CheckCancelled(); err != nil {
		return err
	}
	return c.Control.WaitIfPaused(ctx)
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

// UseWorkflowIsolation returns true if workflow-level Git isolation is enabled.
// This requires GitIsolation to be configured with Enabled=true, WorkflowWorktrees
// to be set, and a WorkflowBranch to be established in the workflow state.
func (c *Context) UseWorkflowIsolation() bool {
	if c.GitIsolation == nil || !c.GitIsolation.Enabled {
		return false
	}
	if c.WorkflowWorktrees == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.State != nil && c.State.WorkflowBranch != ""
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

// SynthesizePlansParams holds parameters for multi-agent plan synthesis prompt.
type SynthesizePlansParams struct {
	Prompt   string
	Analysis string
	Plans    []PlanOutput
	MaxTasks int
}
