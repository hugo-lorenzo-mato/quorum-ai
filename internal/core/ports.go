package core

import (
	"context"
	"time"
)

// =============================================================================
// Agent Port (T027)
// =============================================================================

// Agent defines the contract for AI agent CLI adapters.
type Agent interface {
	// Name returns the adapter identifier (e.g., "claude", "gemini").
	Name() string

	// Capabilities returns what the agent can do.
	Capabilities() Capabilities

	// Ping checks if the agent CLI is available and authenticated.
	Ping(ctx context.Context) error

	// Execute runs a prompt through the agent and returns the result.
	Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error)
}

// Capabilities describes what an agent can do.
type Capabilities struct {
	SupportsStreaming bool
	SupportsTools     bool
	SupportsImages    bool
	SupportsJSON      bool
	SupportedModels   []string
	DefaultModel      string
	MaxContextTokens  int
	MaxOutputTokens   int
	RateLimitRPM      int // Requests per minute
	RateLimitTPM      int // Tokens per minute
}

// OutputFormat specifies the expected output format.
type OutputFormat string

const (
	OutputFormatText     OutputFormat = "text"
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatMarkdown OutputFormat = "markdown"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// ExecuteOptions configures an agent execution.
type ExecuteOptions struct {
	Prompt       string
	SystemPrompt string
	Messages     []Message // Conversation history (for API-based adapters)
	Model        string
	MaxTokens    int
	Temperature  float64
	Format       OutputFormat
	Timeout      time.Duration
	WorkDir      string
	AllowedTools []string
	DeniedTools  []string
	Sandbox      bool
	Phase        Phase // Current workflow phase (for phase-specific behavior)
}

// DefaultExecuteOptions returns sensible defaults.
func DefaultExecuteOptions() ExecuteOptions {
	return ExecuteOptions{
		MaxTokens:   4096,
		Temperature: 0.7,
		Format:      OutputFormatText,
		Timeout:     10 * time.Minute,
	}
}

// ExecuteResult contains the output of an agent execution.
type ExecuteResult struct {
	Output       string
	Parsed       map[string]interface{} // For JSON output
	TokensIn     int
	TokensOut    int
	CostUSD      float64
	Duration     time.Duration
	Model        string
	FinishReason string
	ToolCalls    []ToolCall
}

// ToolCall represents a tool invocation by the agent.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
	Result    string
}

// TotalTokens returns the sum of input and output tokens.
func (r *ExecuteResult) TotalTokens() int {
	return r.TokensIn + r.TokensOut
}

// AgentRegistry manages registered agents.
type AgentRegistry interface {
	// Register adds an agent to the registry.
	Register(name string, agent Agent) error

	// Get retrieves an agent by name.
	Get(name string) (Agent, error)

	// List returns all registered agent names.
	List() []string

	// ListEnabled returns names of configured and enabled agents.
	ListEnabled() []string

	// Available returns agents that pass Ping.
	Available(ctx context.Context) []string

	// AvailableForPhase returns agents that pass Ping AND are enabled for the given phase.
	// Phase should be one of: "optimize", "analyze", "plan", "execute"
	AvailableForPhase(ctx context.Context, phase string) []string

	// ListEnabledForPhase returns agent names that are configured and enabled for the given phase.
	// Unlike AvailableForPhase, this does not ping agents - it only checks configuration.
	ListEnabledForPhase(phase string) []string
}

// =============================================================================
// StateManager Port (T028)
// =============================================================================

// StateManager defines the contract for workflow state persistence.
type StateManager interface {
	// Save persists the current workflow state atomically.
	// Also sets the saved workflow as the active workflow.
	Save(ctx context.Context, state *WorkflowState) error

	// Load retrieves the active workflow state from storage.
	// Returns nil state and no error if state doesn't exist.
	Load(ctx context.Context) (*WorkflowState, error)

	// LoadByID retrieves a specific workflow state by its ID.
	// Returns nil state and no error if workflow doesn't exist.
	LoadByID(ctx context.Context, id WorkflowID) (*WorkflowState, error)

	// ListWorkflows returns summaries of all available workflows.
	ListWorkflows(ctx context.Context) ([]WorkflowSummary, error)

	// GetActiveWorkflowID returns the ID of the currently active workflow.
	// Returns empty string if no active workflow.
	GetActiveWorkflowID(ctx context.Context) (WorkflowID, error)

	// SetActiveWorkflowID sets the active workflow ID.
	SetActiveWorkflowID(ctx context.Context, id WorkflowID) error

	// AcquireLock obtains an exclusive lock on the state file.
	// Returns error if lock cannot be acquired (another process holds it).
	AcquireLock(ctx context.Context) error

	// ReleaseLock releases the exclusive lock.
	ReleaseLock(ctx context.Context) error

	// Exists checks if state file exists.
	Exists() bool

	// Backup creates a backup of the current state.
	Backup(ctx context.Context) error

	// Restore restores from the most recent backup.
	Restore(ctx context.Context) (*WorkflowState, error)

	// DeactivateWorkflow clears the active workflow without deleting any data.
	// After this call, GetActiveWorkflowID returns empty and Load returns nil.
	DeactivateWorkflow(ctx context.Context) error

	// ArchiveWorkflows moves completed workflows to an archive location.
	// Returns the number of workflows archived.
	ArchiveWorkflows(ctx context.Context) (int, error)

	// PurgeAllWorkflows deletes all workflow data permanently.
	// Returns the number of workflows deleted.
	PurgeAllWorkflows(ctx context.Context) (int, error)

	// DeleteWorkflow deletes a single workflow by ID.
	// Returns error if workflow does not exist.
	DeleteWorkflow(ctx context.Context, id WorkflowID) error
}

// WorkflowSummary provides a lightweight summary of a workflow for listing.
type WorkflowSummary struct {
	WorkflowID   WorkflowID     `json:"workflow_id"`
	Status       WorkflowStatus `json:"status"`
	CurrentPhase Phase          `json:"current_phase"`
	Prompt       string         `json:"prompt"` // Truncated for display
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	IsActive     bool           `json:"is_active"`
}

// WorkflowState represents the persisted state of a workflow.
type WorkflowState struct {
	Version         int                   `json:"version"`
	WorkflowID      WorkflowID            `json:"workflow_id"`
	Title           string                `json:"title,omitempty"`
	Status          WorkflowStatus        `json:"status"`
	CurrentPhase    Phase                 `json:"current_phase"`
	Prompt          string                `json:"prompt"`
	OptimizedPrompt string                `json:"optimized_prompt,omitempty"`
	Error           string                `json:"error,omitempty"` // Error message if workflow failed
	Tasks           map[TaskID]*TaskState `json:"tasks"`
	TaskOrder       []TaskID              `json:"task_order"`
	Config          *WorkflowConfig       `json:"config"`
	Metrics         *StateMetrics         `json:"metrics"`
	Checkpoints     []Checkpoint          `json:"checkpoints"`
	AgentEvents     []AgentEvent          `json:"agent_events,omitempty"` // Persisted agent activity for UI
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Checksum        string                `json:"checksum,omitempty"`
	ReportPath      string                `json:"report_path,omitempty"` // Persisted report directory for resume
}

// TaskState represents persisted task state.
type TaskState struct {
	ID           TaskID     `json:"id"`
	Phase        Phase      `json:"phase"`
	Name         string     `json:"name"`
	Status       TaskStatus `json:"status"`
	CLI          string     `json:"cli"`
	Model        string     `json:"model"`
	Dependencies []TaskID   `json:"dependencies"`
	TokensIn     int        `json:"tokens_in"`
	TokensOut    int        `json:"tokens_out"`
	CostUSD      float64    `json:"cost_usd"`
	Retries      int        `json:"retries"`
	Error        string     `json:"error,omitempty"`
	WorktreePath string     `json:"worktree_path,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	// Task execution artifacts
	Output       string     `json:"output,omitempty"`        // Agent output (truncated if large)
	OutputFile   string     `json:"output_file,omitempty"`   // Path to full output if truncated
	ModelUsed    string     `json:"model_used,omitempty"`    // Actual model used
	FinishReason string     `json:"finish_reason,omitempty"` // Why agent stopped
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`    // Tools invoked

	// Recovery metadata - enables resume from partial execution
	LastCommit    string   `json:"last_commit,omitempty"`    // Git commit SHA after task completion
	FilesModified []string `json:"files_modified,omitempty"` // Files modified by this task
	Branch        string   `json:"branch,omitempty"`         // Git branch used for this task
	Resumable     bool     `json:"resumable,omitempty"`      // Whether task can be resumed
	ResumeHint    string   `json:"resume_hint,omitempty"`    // Hint for resuming execution
}

// MaxInlineOutputSize is the maximum size of output to store inline.
const MaxInlineOutputSize = 10000 // 10KB

// StateMetrics holds aggregated workflow metrics.
type StateMetrics struct {
	TotalCostUSD   float64       `json:"total_cost_usd"`
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	ConsensusScore float64       `json:"consensus_score"`
	Duration       time.Duration `json:"duration"`
}

// Checkpoint represents a resumable point in execution.
type Checkpoint struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Phase     Phase     `json:"phase"`
	TaskID    TaskID    `json:"task_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Data      []byte    `json:"data,omitempty"`
}

// CurrentStateVersion is the schema version for state files.
const CurrentStateVersion = 1

// NewWorkflowState creates a new state from a workflow.
func NewWorkflowState(w *Workflow) *WorkflowState {
	state := &WorkflowState{
		Version:      CurrentStateVersion,
		WorkflowID:   w.ID,
		Status:       w.Status,
		CurrentPhase: w.CurrentPhase,
		Prompt:       w.Prompt,
		Tasks:        make(map[TaskID]*TaskState),
		TaskOrder:    w.TaskOrder,
		Config:       w.Config,
		Metrics: &StateMetrics{
			TotalCostUSD:   w.TotalCostUSD,
			TotalTokensIn:  w.TotalTokensIn,
			TotalTokensOut: w.TotalTokensOut,
			ConsensusScore: w.ConsensusScore,
		},
		Checkpoints: make([]Checkpoint, 0),
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   time.Now(),
	}

	for id, task := range w.Tasks {
		state.Tasks[id] = &TaskState{
			ID:           task.ID,
			Phase:        task.Phase,
			Name:         task.Name,
			Status:       task.Status,
			CLI:          task.CLI,
			Model:        task.Model,
			Dependencies: task.Dependencies,
			TokensIn:     task.TokensIn,
			TokensOut:    task.TokensOut,
			CostUSD:      task.CostUSD,
			Retries:      task.Retries,
			Error:        task.Error,
			StartedAt:    task.StartedAt,
			CompletedAt:  task.CompletedAt,
		}
	}

	return state
}

// =============================================================================
// GitClient Port (T029)
// =============================================================================

// GitClient defines the contract for git operations.
type GitClient interface {
	// Repository information
	RepoRoot(ctx context.Context) (string, error)
	CurrentBranch(ctx context.Context) (string, error)
	DefaultBranch(ctx context.Context) (string, error)
	RemoteURL(ctx context.Context) (string, error)

	// Branch operations
	BranchExists(ctx context.Context, name string) (bool, error)
	CreateBranch(ctx context.Context, name, base string) error
	DeleteBranch(ctx context.Context, name string) error
	CheckoutBranch(ctx context.Context, name string) error

	// Worktree operations
	CreateWorktree(ctx context.Context, path, branch string) error
	RemoveWorktree(ctx context.Context, path string) error
	ListWorktrees(ctx context.Context) ([]Worktree, error)

	// Commit operations
	Status(ctx context.Context) (*GitStatus, error)
	Add(ctx context.Context, paths ...string) error
	Commit(ctx context.Context, message string) (string, error)
	Push(ctx context.Context, remote, branch string) error

	// Diff operations
	Diff(ctx context.Context, base, head string) (string, error)
	DiffFiles(ctx context.Context, base, head string) ([]string, error)

	// Utility
	IsClean(ctx context.Context) (bool, error)
	Fetch(ctx context.Context, remote string) error
}

// Worktree represents a git worktree.
type Worktree struct {
	Path     string
	Branch   string
	Commit   string
	IsMain   bool
	IsLocked bool
}

// GitStatus represents the status of a git repository.
type GitStatus struct {
	Branch       string
	Ahead        int
	Behind       int
	Staged       []FileStatus
	Unstaged     []FileStatus
	Untracked    []string
	HasConflicts bool
}

// FileStatus represents a file's git status.
type FileStatus struct {
	Path   string
	Status string // M, A, D, R, C, U
}

// WorktreeManager provides higher-level worktree management.
type WorktreeManager interface {
	// Create creates a new worktree for a task.
	Create(ctx context.Context, task *Task, branch string) (*WorktreeInfo, error)

	// Get retrieves worktree info for a task.
	Get(ctx context.Context, task *Task) (*WorktreeInfo, error)

	// Remove cleans up a task's worktree.
	Remove(ctx context.Context, task *Task) error

	// CleanupStale removes worktrees for completed/failed tasks.
	CleanupStale(ctx context.Context) error

	// List returns all managed worktrees.
	List(ctx context.Context) ([]*WorktreeInfo, error)
}

// WorktreeInfo contains information about a task's worktree.
type WorktreeInfo struct {
	TaskID    TaskID
	Path      string
	Branch    string
	CreatedAt time.Time
	Status    WorktreeStatus
}

// WorktreeStatus represents the state of a worktree.
type WorktreeStatus string

const (
	WorktreeStatusActive  WorktreeStatus = "active"
	WorktreeStatusStale   WorktreeStatus = "stale"
	WorktreeStatusCleaned WorktreeStatus = "cleaned"
)

// =============================================================================
// GitHubClient Port (T030)
// =============================================================================

// GitHubClient defines the contract for GitHub API operations.
type GitHubClient interface {
	// Repository operations
	GetRepo(ctx context.Context) (*RepoInfo, error)
	GetDefaultBranch(ctx context.Context) (string, error)

	// Pull request operations
	CreatePR(ctx context.Context, opts CreatePROptions) (*PullRequest, error)
	GetPR(ctx context.Context, number int) (*PullRequest, error)
	ListPRs(ctx context.Context, opts ListPROptions) ([]*PullRequest, error)
	UpdatePR(ctx context.Context, number int, opts UpdatePROptions) error
	MergePR(ctx context.Context, number int, opts MergePROptions) error
	ClosePR(ctx context.Context, number int) error

	// Review operations
	RequestReview(ctx context.Context, number int, reviewers []string) error
	AddComment(ctx context.Context, number int, body string) error

	// Check operations
	GetCheckStatus(ctx context.Context, ref string) (*CheckStatus, error)
	WaitForChecks(ctx context.Context, ref string, timeout time.Duration) (*CheckStatus, error)

	// Authentication
	ValidateToken(ctx context.Context) error
	GetAuthenticatedUser(ctx context.Context) (string, error)
}

// RepoInfo contains repository information.
type RepoInfo struct {
	Owner         string
	Name          string
	FullName      string
	DefaultBranch string
	IsPrivate     bool
	HTMLURL       string
}

// CreatePROptions configures pull request creation.
type CreatePROptions struct {
	Title     string
	Body      string
	Head      string // Source branch
	Base      string // Target branch
	Draft     bool
	Labels    []string
	Assignees []string
}

// ListPROptions configures pull request listing.
type ListPROptions struct {
	State     string // open, closed, all
	Head      string
	Base      string
	Sort      string
	Direction string
	Limit     int
}

// UpdatePROptions configures pull request updates.
type UpdatePROptions struct {
	Title     *string
	Body      *string
	State     *string
	Base      *string
	Labels    []string
	Assignees []string
}

// MergePROptions configures pull request merging.
type MergePROptions struct {
	Method        string // merge, squash, rebase
	CommitTitle   string
	CommitMessage string
	SHA           string // Optional: require specific SHA
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number    int
	Title     string
	Body      string
	State     string
	Head      PRBranch
	Base      PRBranch
	HTMLURL   string
	Draft     bool
	Merged    bool
	Mergeable *bool
	Labels    []string
	Assignees []string
	CreatedAt time.Time
	UpdatedAt time.Time
	MergedAt  *time.Time
}

// PRBranch represents a PR branch reference.
type PRBranch struct {
	Ref  string
	SHA  string
	Repo string
}

// CheckStatus represents the combined status of all checks.
type CheckStatus struct {
	State      string // pending, success, failure, error
	TotalCount int
	Passed     int
	Failed     int
	Pending    int
	Checks     []Check
	UpdatedAt  time.Time
}

// Check represents a single CI check.
type Check struct {
	Name        string
	Status      string // queued, in_progress, completed
	Conclusion  string // success, failure, neutral, cancelled, skipped, timed_out
	HTMLURL     string
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// IsSuccess returns true if all checks passed.
func (cs *CheckStatus) IsSuccess() bool {
	return cs.State == "success" && cs.Failed == 0
}

// IsPending returns true if any checks are still running.
func (cs *CheckStatus) IsPending() bool {
	return cs.Pending > 0 || cs.State == "pending"
}
