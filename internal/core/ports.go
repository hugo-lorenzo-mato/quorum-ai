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
	Prompt          string
	SystemPrompt    string
	Messages        []Message // Conversation history (for API-based adapters)
	Model           string
	MaxTokens       int
	Temperature     float64
	Format          OutputFormat
	Timeout         time.Duration
	WorkDir         string
	AllowedTools    []string
	DeniedTools     []string
	Sandbox         bool
	Phase           Phase  // Current workflow phase (for phase-specific behavior)
	ReasoningEffort string // Reasoning effort level: minimal, low, medium, high, xhigh (for supporting models)
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

	// AvailableForPhaseWithConfig returns agents that pass Ping AND are enabled for the given phase,
	// using project-specific phase configuration instead of global agent config.
	// This is used in multi-project scenarios where each project may have different agent phases.
	AvailableForPhaseWithConfig(ctx context.Context, phase string, projectPhases map[string][]string) []string
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
	// Deprecated: Use ReleaseWorkflowLock for per-workflow locking.
	ReleaseLock(ctx context.Context) error

	// AcquireWorkflowLock obtains an exclusive lock for a specific workflow.
	// Returns error if lock cannot be acquired (another process holds it).
	AcquireWorkflowLock(ctx context.Context, workflowID WorkflowID) error

	// ReleaseWorkflowLock releases the lock for a specific workflow.
	ReleaseWorkflowLock(ctx context.Context, workflowID WorkflowID) error

	// RefreshWorkflowLock extends the lock expiration time for a workflow.
	// Returns error if the lock is not held by this process.
	RefreshWorkflowLock(ctx context.Context, workflowID WorkflowID) error

	// SetWorkflowRunning marks a workflow as currently executing.
	SetWorkflowRunning(ctx context.Context, workflowID WorkflowID) error

	// ClearWorkflowRunning removes a workflow from the running state.
	ClearWorkflowRunning(ctx context.Context, workflowID WorkflowID) error

	// ListRunningWorkflows returns IDs of all currently executing workflows.
	ListRunningWorkflows(ctx context.Context) ([]WorkflowID, error)

	// IsWorkflowRunning checks if a specific workflow is currently executing.
	IsWorkflowRunning(ctx context.Context, workflowID WorkflowID) (bool, error)

	// UpdateWorkflowHeartbeat updates the heartbeat timestamp for a running workflow.
	// Used to prove liveness - workflows with stale heartbeats are considered dead.
	UpdateWorkflowHeartbeat(ctx context.Context, workflowID WorkflowID) error

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

	// UpdateHeartbeat updates the heartbeat timestamp for a running workflow.
	// Used for zombie detection - workflows with stale heartbeats are considered dead.
	UpdateHeartbeat(ctx context.Context, id WorkflowID) error

	// FindZombieWorkflows returns workflows with status "running" but stale heartbeats.
	// A workflow is considered a zombie if its heartbeat is older than the threshold.
	FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*WorkflowState, error)

	// FindWorkflowsByPrompt finds workflows with the same prompt (by hash).
	// Returns matching workflows for duplicate detection.
	FindWorkflowsByPrompt(ctx context.Context, prompt string) ([]DuplicateWorkflowInfo, error)

	// ExecuteAtomically runs operations atomically within a database transaction.
	// The callback receives an AtomicStateContext that provides transactional versions
	// of state operations. If the callback returns an error, the transaction is rolled back.
	ExecuteAtomically(ctx context.Context, fn func(AtomicStateContext) error) error
}

// AtomicStateContext provides transactional access to state operations.
// Operations within this context are part of the same database transaction.
type AtomicStateContext interface {
	// LoadByID retrieves a workflow state within the transaction.
	LoadByID(id WorkflowID) (*WorkflowState, error)

	// Save persists workflow state within the transaction.
	Save(state *WorkflowState) error

	// SetWorkflowRunning marks a workflow as running within the transaction.
	SetWorkflowRunning(workflowID WorkflowID) error

	// ClearWorkflowRunning removes a workflow from running state within the transaction.
	ClearWorkflowRunning(workflowID WorkflowID) error

	// IsWorkflowRunning checks if a workflow is running within the transaction.
	IsWorkflowRunning(workflowID WorkflowID) (bool, error)
}

// WorkflowSummary provides a lightweight summary of a workflow for listing.
type WorkflowSummary struct {
	WorkflowID   WorkflowID     `json:"workflow_id"`
	Title        string         `json:"title,omitempty"`
	Status       WorkflowStatus `json:"status"`
	CurrentPhase Phase          `json:"current_phase"`
	Prompt       string         `json:"prompt"` // Truncated for display
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	IsActive     bool           `json:"is_active"`
}

// DuplicateWorkflowInfo contains information about a potential duplicate workflow.
// Used when detecting workflows with identical prompts.
type DuplicateWorkflowInfo struct {
	WorkflowID WorkflowID     `json:"workflow_id"`
	Status     WorkflowStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	Title      string         `json:"title,omitempty"`
}

// WorkflowDefinition holds the immutable definition of a workflow.
// These fields are set at creation time and do not change during execution
// (except OptimizedPrompt which is set after the refine phase).
type WorkflowDefinition struct {
	Version         int          `json:"version"`
	WorkflowID      WorkflowID   `json:"workflow_id"`
	Title           string       `json:"title,omitempty"`
	Prompt          string       `json:"prompt"`
	OptimizedPrompt string       `json:"optimized_prompt,omitempty"`
	Blueprint       *Blueprint   `json:"blueprint"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

// WorkflowRun holds the mutable execution state of a workflow.
// These fields change during workflow execution.
type WorkflowRun struct {
	ExecutionID  int                   `json:"execution_id"` // Increments on each Run/Resume to distinguish event sets
	Status       WorkflowStatus        `json:"status"`
	CurrentPhase Phase                 `json:"current_phase"`
	Error        string                `json:"error,omitempty"` // Error message if workflow failed
	Tasks        map[TaskID]*TaskState `json:"tasks"`
	TaskOrder    []TaskID              `json:"task_order"`
	AgentEvents  []AgentEvent          `json:"agent_events,omitempty"` // Persisted agent activity for UI
	Metrics      *StateMetrics         `json:"metrics"`
	Checkpoints  []Checkpoint          `json:"checkpoints"`
	UpdatedAt    time.Time             `json:"updated_at"`

	// Infrastructure
	Checksum   string     `json:"checksum,omitempty"`
	ReportPath string     `json:"report_path,omitempty"` // Persisted report directory for resume
	HeartbeatAt *time.Time `json:"heartbeat_at,omitempty"` // Last heartbeat timestamp
	ResumeCount int        `json:"resume_count,omitempty"` // Number of auto-resumes performed
	MaxResumes  int        `json:"max_resumes,omitempty"`  // Maximum allowed auto-resumes (default: 3)

	// Workflow Git isolation
	WorkflowBranch string `json:"workflow_branch,omitempty"` // Git branch for this workflow (e.g., quorum/wf-xxx)

	// Kanban board tracking
	KanbanColumn         string     `json:"kanban_column,omitempty"`          // refinement, todo, in_progress, to_verify, done
	KanbanPosition       int        `json:"kanban_position,omitempty"`        // Order within column (lower = higher in list)
	PRURL                string     `json:"pr_url,omitempty"`                 // GitHub PR URL
	PRNumber             int        `json:"pr_number,omitempty"`              // GitHub PR number
	KanbanStartedAt      *time.Time `json:"kanban_started_at,omitempty"`      // When Kanban engine started execution
	KanbanCompletedAt    *time.Time `json:"kanban_completed_at,omitempty"`    // When execution completed in Kanban context
	KanbanExecutionCount int        `json:"kanban_execution_count,omitempty"` // How many times Kanban engine executed this
	KanbanLastError      string     `json:"kanban_last_error,omitempty"`      // Last error from Kanban execution
}

// WorkflowState represents the persisted state of a workflow.
// It composes WorkflowDefinition (immutable) and WorkflowRun (mutable) via embedding.
// JSON serialization produces a flat object (no nesting) identical to the previous monolithic struct.
type WorkflowState struct {
	WorkflowDefinition
	WorkflowRun
}

// Definition returns a pointer to the workflow's definition.
func (ws *WorkflowState) Definition() *WorkflowDefinition { return &ws.WorkflowDefinition }

// Run returns a pointer to the workflow's run state.
func (ws *WorkflowState) Run() *WorkflowRun { return &ws.WorkflowRun }

// Attachment represents a user-provided file associated with a chat session or workflow.
// Attachments are stored under .quorum/attachments and are not expected to be part of the git repository.
type Attachment struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaskState represents persisted task state.
type TaskState struct {
	ID           TaskID     `json:"id"`
	Phase        Phase      `json:"phase"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
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

	// Workflow isolation merge tracking
	MergePending bool   `json:"merge_pending,omitempty"` // True if merge to workflow branch failed
	MergeCommit  string `json:"merge_commit,omitempty"`  // Commit hash of merge commit
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
		WorkflowDefinition: WorkflowDefinition{
			Version:    CurrentStateVersion,
			WorkflowID: w.ID,
			Prompt:     w.Prompt,
			Blueprint:  w.Blueprint,
			CreatedAt:  w.CreatedAt,
		},
		WorkflowRun: WorkflowRun{
			Status:       w.Status,
			CurrentPhase: w.CurrentPhase,
			Tasks:        make(map[TaskID]*TaskState),
			TaskOrder:    w.TaskOrder,
			Metrics: &StateMetrics{
				TotalCostUSD:   w.TotalCostUSD,
				TotalTokensIn:  w.TotalTokensIn,
				TotalTokensOut: w.TotalTokensOut,
				ConsensusScore: w.ConsensusScore,
			},
			Checkpoints: make([]Checkpoint, 0),
			UpdatedAt:   time.Now(),
		},
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

	// Merge operations
	Merge(ctx context.Context, branch string, opts MergeOptions) error
	AbortMerge(ctx context.Context) error
	HasMergeConflicts(ctx context.Context) (bool, error)
	GetConflictFiles(ctx context.Context) ([]string, error)

	// Rebase operations
	Rebase(ctx context.Context, onto string) error
	AbortRebase(ctx context.Context) error
	ContinueRebase(ctx context.Context) error
	HasRebaseInProgress(ctx context.Context) (bool, error)

	// Reset operations
	ResetHard(ctx context.Context, ref string) error
	ResetSoft(ctx context.Context, ref string) error

	// Cherry-pick operations
	CherryPick(ctx context.Context, commit string) error
	AbortCherryPick(ctx context.Context) error

	// Query operations
	RevParse(ctx context.Context, ref string) (string, error)
	IsAncestor(ctx context.Context, ancestor, commit string) (bool, error)

	// Status operations
	HasUncommittedChanges(ctx context.Context) (bool, error)
}

// Worktree represents a git worktree.
type Worktree struct {
	Path     string
	Branch   string
	Commit   string
	IsMain   bool
	IsLocked bool
}

// MergeOptions configures merge behavior.
type MergeOptions struct {
	Strategy       string // "recursive", "ours", "theirs", "resolve"
	StrategyOption string // e.g., "ours", "theirs" for recursive strategy
	NoCommit       bool   // Stage changes but don't commit
	NoFastForward  bool   // Always create merge commit
	Squash         bool   // Squash all commits into one
	Message        string // Custom commit message
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

// =============================================================================
// ChatStore Port (Chat Session Persistence)
// =============================================================================

// ChatStore defines the contract for chat session persistence.
// Implementations can use JSON files or SQLite for storage.
type ChatStore interface {
	// SaveSession persists a chat session.
	SaveSession(ctx context.Context, session *ChatSessionState) error

	// LoadSession retrieves a chat session by ID.
	// Returns nil and no error if session doesn't exist.
	LoadSession(ctx context.Context, id string) (*ChatSessionState, error)

	// ListSessions returns all chat sessions (without messages for efficiency).
	ListSessions(ctx context.Context) ([]*ChatSessionState, error)

	// DeleteSession removes a chat session and all its messages.
	DeleteSession(ctx context.Context, id string) error

	// SaveMessage adds a message to a session.
	SaveMessage(ctx context.Context, msg *ChatMessageState) error

	// LoadMessages retrieves all messages for a session.
	LoadMessages(ctx context.Context, sessionID string) ([]*ChatMessageState, error)
}

// ChatSessionState represents the persisted state of a chat session.
type ChatSessionState struct {
	ID          string    `json:"id"`
	Title       string    `json:"title,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Agent       string    `json:"agent"`
	Model       string    `json:"model,omitempty"`
	ProjectRoot string    `json:"project_root,omitempty"`
}

// ChatMessageState represents a persisted chat message.
type ChatMessageState struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // "user", "agent", "system"
	Agent     string    `json:"agent,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	TokensIn  int       `json:"tokens_in,omitempty"`
	TokensOut int       `json:"tokens_out,omitempty"`
	CostUSD   float64   `json:"cost_usd,omitempty"`
}

// =============================================================================
// WorkflowWorktreeManager Port (T004)
// =============================================================================

// WorkflowWorktreeManager manages Git isolation at the workflow level.
type WorkflowWorktreeManager interface {
	// InitializeWorkflow creates a workflow branch and worktree root directory.
	// Returns WorkflowGitInfo with branch and path information.
	// The workflow branch is created from baseBranch (e.g., "main").
	InitializeWorkflow(ctx context.Context, workflowID string, baseBranch string) (*WorkflowGitInfo, error)

	// FinalizeWorkflow completes the workflow Git operations.
	// If merge is true, merges the workflow branch to the base branch.
	// Cleans up task branches and worktrees.
	FinalizeWorkflow(ctx context.Context, workflowID string, merge bool) error

	// CleanupWorkflow removes all Git artifacts for a workflow.
	// Removes worktrees, task branches, and optionally the workflow branch.
	CleanupWorkflow(ctx context.Context, workflowID string, removeWorkflowBranch bool) error

	// CreateTaskWorktree creates a worktree for a task within the workflow.
	// The task worktree is created in the workflow's worktree root.
	// The task branch is created from the workflow branch.
	CreateTaskWorktree(ctx context.Context, workflowID string, task *Task) (*WorktreeInfo, error)

	// RemoveTaskWorktree removes a task's worktree.
	// Optionally removes the task branch as well.
	RemoveTaskWorktree(ctx context.Context, workflowID string, taskID TaskID, removeBranch bool) error

	// MergeTaskToWorkflow merges a task branch into the workflow branch.
	// Uses the specified strategy: "sequential", "parallel", "rebase"
	MergeTaskToWorkflow(ctx context.Context, workflowID string, taskID TaskID, strategy string) error

	// MergeAllTasksToWorkflow merges all completed task branches to workflow branch.
	MergeAllTasksToWorkflow(ctx context.Context, workflowID string, taskIDs []TaskID, strategy string) error

	// GetWorkflowStatus returns the current Git status of the workflow.
	GetWorkflowStatus(ctx context.Context, workflowID string) (*WorkflowGitStatus, error)

	// ListActiveWorkflows returns information about all active workflow worktrees.
	ListActiveWorkflows(ctx context.Context) ([]*WorkflowGitInfo, error)

	// GetWorkflowBranch returns the workflow branch name for a workflow ID.
	GetWorkflowBranch(workflowID string) string

	// GetTaskBranch returns the task branch name for a workflow and task.
	GetTaskBranch(workflowID string, taskID TaskID) string
}

// WorkflowGitInfo contains information about a workflow's Git state.
type WorkflowGitInfo struct {
	WorkflowID     string
	WorkflowBranch string
	BaseBranch     string
	WorktreeRoot   string
	CreatedAt      time.Time
	TaskCount      int
	PendingMerges  int
}

// WorkflowGitStatus contains the current status of a workflow's Git state.
type WorkflowGitStatus struct {
	HasConflicts    bool
	AheadOfBase     int
	BehindBase      int
	UnmergedTasks   []TaskID
	LastMergeCommit string
}
