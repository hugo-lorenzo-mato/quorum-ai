# Definitive Architectural Analysis: Workflow-Level Git Isolation for Quorum

## Document Overview

This document provides an exhaustive architectural analysis of implementing workflow-level Git isolation in the Quorum multi-agent orchestration system. The analysis covers the current execution architecture, concurrency constraints, proposed design patterns, interface unification requirements, component impact assessment, edge case handling, and performance considerations.

---

## 1. Current Architecture of Execution

### 1.1 Workflow Execution Flow

The Quorum system executes workflows through a well-defined phase progression:

```
+-------------------------------------------------------------------+
|                    WORKFLOW LIFECYCLE                             |
+-------------------------------------------------------------------+
                                |
                                v
+-------------------+    +-------------------+    +-------------------+
|   PhaseRefine     |--->|   PhaseAnalyze    |--->|    PhasePlan      |
| (Prompt Optimize) |    | (Multi-Agent      |    | (Task Generation) |
|                   |    |  Consensus)       |    |                   |
+-------------------+    +-------------------+    +-------------------+
                                                         |
                                                         v
                              +-------------------------------------------+
                              |             PhaseExecute                  |
                              |  +-------+   +-------+   +-------+       |
                              |  |Task 1 |   |Task 2 |   |Task 3 |       |
                              |  |Worktree|  |Worktree|  |Worktree|      |
                              |  +-------+   +-------+   +-------+       |
                              |        \         |          /            |
                              |         \        |         /             |
                              |          v       v        v              |
                              |       [Main Repository Branch]           |
                              +-------------------------------------------+
```

The workflow execution is initiated through three distinct entry points:

**CLI Entry Point** (`cmd/quorum/cmd/run.go`):
- Creates `workflow.Runner` with manually wired dependencies
- Uses `InitPhaseRunner()` from `common.go` for dependency construction
- Directly instantiates `workflow.NewRunner(RunnerDeps{...})`

**API/WebUI Entry Point** (`internal/api/runner_factory.go:69-181`):
- Uses `RunnerFactory.CreateRunner()` for consistent dependency injection
- Returns both `*workflow.Runner` and `*webadapters.WebOutputNotifier`
- Integrates with EventBus for real-time UI updates

**State Management Integration** (`internal/service/workflow/runner.go:246-249`):
```go
// Acquire lock
if err := r.state.AcquireLock(ctx); err != nil {
    return fmt.Errorf("acquiring lock: %w", err)
}
defer func() { _ = r.state.ReleaseLock(ctx) }()
```

### 1.2 Current Git Integration Architecture

The current system implements task-level Git worktree isolation with the following components:

```
+------------------------------------------------------------------+
|                    GIT ISOLATION ARCHITECTURE                     |
+------------------------------------------------------------------+

    Main Repository (./)
         |
         +-- .worktrees/                    [Worktree Base Directory]
         |       |
         |       +-- quorum-task1__desc/    [Task 1 Worktree]
         |       |       Branch: quorum/task1__desc
         |       |
         |       +-- quorum-task2__desc/    [Task 2 Worktree]
         |       |       Branch: quorum/task2__desc
         |       |
         |       +-- quorum-task3__desc/    [Task 3 Worktree]
         |               Branch: quorum/task3__desc
         |
         +-- .quorum/
                 +-- state/
                 |       +-- state.db       [SQLite State Database]
                 |       +-- state.db.lock  [File-based Lock]
                 |
                 +-- runs/                  [Workflow Reports]
                         +-- wf-YYYYMMDD-HHMMSS-xxxxx/
```

**WorktreeManager** (`internal/adapters/git/worktree.go:506-649`):

The `TaskWorktreeManager` implements `core.WorktreeManager` and provides:

```go
// Branch naming convention (line 143)
func resolveWorktreeBranch(name, branch string) (string, error) {
    candidate := strings.TrimSpace(branch)
    if candidate == "" {
        candidate = "quorum/" + name
    }
    // ...
}

// Worktree path convention (line 204)
worktreePath := filepath.Join(m.baseDir, m.prefix+name)  // prefix = "quorum-"
```

Branch naming pattern: `quorum/<taskID>__<normalized-label>`
Worktree path pattern: `.worktrees/quorum-<taskID>__<normalized-label>`

**GitClient Interface** (`internal/adapters/git/client.go:17-578`):

Current capabilities:
- `CreateBranch(ctx, name, base)` - Creates branch from base
- `DeleteBranch(ctx, name)` - Deletes branch
- `CheckoutBranch(ctx, name)` - Switches to branch
- `CreateWorktree(ctx, path, branch)` - Creates worktree
- `RemoveWorktree(ctx, path)` - Removes worktree
- `Commit(ctx, message)` - Creates commit
- `Push(ctx, remote, branch)` - Pushes to remote

**Missing operations required for workflow isolation**:
- `Merge(ctx, branch, strategy)` - Merge branch into current
- `Rebase(ctx, onto)` - Rebase current branch onto another
- `AbortMerge(ctx)` - Abort failed merge
- `ResetHard(ctx, ref)` - Hard reset to reference
- `CherryPick(ctx, commit)` - Cherry-pick specific commit

### 1.3 State Persistence Architecture

**SQLite Schema** (`internal/adapters/state/migrations/001_initial_schema.sql`):

```sql
-- CRITICAL: Singleton constraint on active workflow (line 12-16)
CREATE TABLE IF NOT EXISTS active_workflow (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Enforces single active workflow
    workflow_id TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Workflow table (lines 19-33)
CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    title TEXT,
    status TEXT NOT NULL,
    current_phase TEXT NOT NULL,
    prompt TEXT NOT NULL,
    -- ... no workflow_branch field
);

-- Task table with branch tracking (lines 36-60)
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT NOT NULL,
    workflow_id TEXT NOT NULL,
    -- ...
    worktree_path TEXT,
    branch TEXT,          -- Task-level branch (added in migration 002)
    -- ...
);
```

The `CHECK (id = 1)` constraint on `active_workflow` is the **primary blocker** for concurrent workflow execution. This constraint physically prevents multiple workflows from being tracked as active simultaneously.

**SQLiteStateManager** (`internal/adapters/state/sqlite.go:38-51`):

```go
type SQLiteStateManager struct {
    dbPath     string
    backupPath string
    lockPath   string           // File-based lock path
    lockTTL    time.Duration
    db         *sql.DB          // Single write connection
    readDB     *sql.DB          // Read-only connection pool
    mu         sync.RWMutex     // In-process coordination
    // ...
}
```

**Locking Mechanism** (`internal/adapters/state/sqlite.go:754-821`):

```go
func (m *SQLiteStateManager) AcquireLock(_ context.Context) error {
    // Uses file-based locking (lockPath = dbPath + ".lock")
    // Creates JSON lock file with PID, hostname, timestamp
    // Checks for stale locks using lockTTL
    // Single-process serialization via mu sync.RWMutex
}
```

The locking mechanism uses **file-based locks** rather than database-level locks, providing cross-process exclusion but not supporting per-workflow granularity.

### 1.4 Workflow State Model

**WorkflowState** (`internal/core/ports.go` - from context summary):

The current `WorkflowState` structure contains:
- `WorkflowID`, `Status`, `CurrentPhase`
- `Prompt`, `OptimizedPrompt`
- `Tasks map[TaskID]*TaskState`
- `Checkpoints`, `Metrics`
- **Missing**: `WorkflowBranch`, `BaseBranch`, `MergeStrategy` fields

**TaskState** (`internal/core/ports.go` - line 283):
- Includes `Branch` field for task-level branch tracking
- Includes `WorktreePath`, `LastCommit`, `FilesModified`
- `Resumable` flag for recovery support

---

## 2. Concurrency Analysis and Potential Deadlocks

### 2.1 Current Concurrency Model

```
+------------------------------------------------------------------------+
|                    CURRENT CONCURRENCY MODEL                            |
+------------------------------------------------------------------------+

    CLI Process 1                     WebUI Process
         |                                  |
         v                                  v
    [AcquireLock]                    [AcquireLock]
         |                                  |
         v                                  X (BLOCKED - lock held)
    [Run Workflow]                          |
         |                                  |
    [ReleaseLock]                           |
         |                                  v
         X                           [AcquireLock] (succeeds)
                                           |
                                           v
                                     [Run Workflow]
```

**Global Lock Acquisition** (`internal/service/workflow/runner.go:246-249`):

The Runner acquires a global lock at the start of every workflow operation:
- `Run()` - New workflow execution
- `Resume()` - Resume existing workflow
- `Analyze()` - Analyze-only mode
- `Plan()` - Plan-only mode
- `Replan()` - Re-planning

This global lock means **only one workflow operation can execute at a time across all entry points**.

### 2.2 Identified Concurrency Bottlenecks

**Bottleneck 1: Single Active Workflow Constraint**

Location: `internal/adapters/state/migrations/001_initial_schema.sql:12-16`

```sql
CREATE TABLE IF NOT EXISTS active_workflow (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    workflow_id TEXT NOT NULL,
    -- ...
);
```

Impact: The database physically cannot track multiple active workflows. Any attempt to insert a second row fails with a constraint violation.

**Bottleneck 2: File-Based Global Lock**

Location: `internal/adapters/state/sqlite.go:754-821`

The `AcquireLock()` method uses a single lock file (`dbPath + ".lock"`) with exclusive creation:

```go
// Line 804
f, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
```

Impact: Cross-process serialization prevents any parallel workflow execution.

**Bottleneck 3: Single Write Connection**

Location: `internal/adapters/state/sqlite.go:83-86`

```go
// SQLite only supports one writer at a time
db.SetMaxOpenConns(1)
db.SetMaxIdleConns(1)
db.SetConnMaxLifetime(0)
```

Impact: Write operations are serialized at the connection level, though this is inherent to SQLite.

**Bottleneck 4: Rate Limiter Registry Scope**

Location: `internal/api/runner_factory.go:96`

```go
rateLimiterRegistry := service.NewRateLimiterRegistry()
```

The rate limiter registry is created **per runner instance**, not globally. Multiple concurrent workflows would create separate registries, potentially overwhelming API rate limits.

**Bottleneck 5: In-Memory Workflow Tracking**

Location: `internal/api/workflows.go` (from context summary):

```go
var runningWorkflows = struct {
    sync.Mutex
    ids map[string]bool
}{ids: make(map[string]bool)}
```

This tracking is **in-memory only** and does not persist across process restarts. It provides double-execution prevention within a single process but not across distributed instances.

### 2.3 Deadlock Scenarios

**Scenario 1: Lock-State Inconsistency**

```
Time  Process A                      Process B
----  ---------                      ---------
T1    AcquireLock() succeeds
T2    Save(workflowA) begins
T3                                   AcquireLock() blocks
T4    Process A crashes
T5                                   Lock TTL expires
T6                                   AcquireLock() succeeds
T7                                   Load() returns workflowA (partial state)
```

The stale lock cleanup mechanism (checking `lockTTL`) creates a window where inconsistent state can be observed.

**Scenario 2: Worktree Branch Collision**

```
Time  Workflow A                     Workflow B
----  ----------                     ----------
T1    Create worktree task-1
      Branch: quorum/task-1__impl
T2                                   Create worktree task-1
                                     Branch: quorum/task-1__impl
                                     ERROR: Branch exists!
```

Current worktree naming does not include workflow ID, causing collisions when different workflows have tasks with the same ID and similar names.

**Scenario 3: Git Lock Contention**

```
Time  Task A (Worktree 1)            Task B (Worktree 2)
----  ------------------             ------------------
T1    git commit (acquires .git/index.lock)
T2                                   git commit (waits for lock)
T3    commit completes
T4                                   git commit (acquires lock)
T5    git push origin branch-a
T6                                   git push origin branch-b
T7    CONFLICT: remote ref updated
```

Parallel git operations on the same repository can conflict, especially during push operations.

### 2.4 Current Protections

**SQLite WAL Mode** (`internal/adapters/state/sqlite.go:77-78`):

```go
db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
```

WAL mode allows concurrent reads during writes, reducing lock contention.

**Retry with Backoff** (`internal/adapters/state/sqlite.go:146-171`):

```go
func (m *SQLiteStateManager) retryWrite(ctx context.Context, operation string, fn func() error) error {
    // Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
    wait := m.baseRetryWait * time.Duration(1<<attempt)
    // ...
}
```

Write operations retry on `SQLITE_BUSY` errors with exponential backoff.

**Process Existence Check** (`internal/adapters/state/sqlite.go:779`):

```go
if processExists(info.PID) {
    return core.ErrState("LOCK_ACQUIRE_FAILED", ...)
}
```

Stale lock detection verifies if the lock-holding process is still alive.

---

## 3. Design of the Branch-per-Workflow Solution

### 3.1 Proposed Architecture

```
+------------------------------------------------------------------------+
|                 WORKFLOW-LEVEL GIT ISOLATION ARCHITECTURE              |
+------------------------------------------------------------------------+

    Main Repository
         |
         +-- Branch: main (or master)
         |
         +-- Branch: quorum/wf-20250129-143000-abc12    [Workflow A Branch]
         |       |
         |       +-- Worktree: .worktrees/wf-abc12/
         |       |       |
         |       |       +-- task-1/ (branch: quorum/wf-abc12/task-1)
         |       |       +-- task-2/ (branch: quorum/wf-abc12/task-2)
         |       |
         |       +-- [Tasks merge to workflow branch]
         |
         +-- Branch: quorum/wf-20250129-143500-def34    [Workflow B Branch]
                 |
                 +-- Worktree: .worktrees/wf-def34/
                         |
                         +-- task-1/ (branch: quorum/wf-def34/task-1)
                         +-- task-2/ (branch: quorum/wf-def34/task-2)


    MERGE FLOW:

    Task Branches ----merge----> Workflow Branch ----merge----> Main Branch
                                                                    ^
                                                               (Optional:
                                                                Auto-PR)
```

### 3.2 Branch Naming Convention

**Workflow Branch Pattern**:
```
quorum/<workflow-id>
Example: quorum/wf-20250129-143000-abc12
```

**Task Branch Pattern** (within workflow namespace):
```
quorum/<workflow-id>/<task-id>__<label>
Example: quorum/wf-20250129-143000-abc12/task-001__implement-auth
```

**Worktree Directory Pattern**:
```
.worktrees/<workflow-id>/
    <task-id>__<label>/
Example: .worktrees/wf-abc12/task-001__implement-auth/
```

### 3.3 Lifecycle Phases

**Phase 1: Workflow Initialization**

```
1. Generate workflow ID (wf-YYYYMMDD-HHMMSS-xxxxx)
2. Create workflow branch from base (main/master)
   git checkout -b quorum/<workflow-id> <base-branch>
3. Create workflow worktree directory
   mkdir -p .worktrees/<workflow-id>
4. Register workflow in state database
   INSERT INTO workflows (id, workflow_branch, base_branch, ...)
```

**Phase 2: Task Execution**

```
For each task:
1. Create task branch from workflow branch
   git checkout -b quorum/<workflow-id>/<task-id> quorum/<workflow-id>
2. Create task worktree
   git worktree add .worktrees/<workflow-id>/<task-id> quorum/<workflow-id>/<task-id>
3. Execute agent in worktree
4. Commit changes to task branch
5. Merge task branch to workflow branch (configurable strategy)
```

**Phase 3: Workflow Completion**

```
1. All task branches merged to workflow branch
2. (Optional) Create PR from workflow branch to base branch
3. (Optional) Auto-merge PR if configured
4. Cleanup task worktrees
5. (Optional) Delete task branches
6. Update workflow status to completed
```

### 3.4 Merge Strategies

**Sequential Merge (Default)**:
```
Task 1 ----merge----> Workflow Branch
                           |
Task 2 ----------------merge----> Workflow Branch
                                       |
Task 3 ----------------------------merge----> Workflow Branch
```

Pros: Simple conflict resolution, clear history
Cons: Slower, cannot parallelize merge operations

**Parallel Merge with Conflict Detection**:
```
Task 1 ----+
           |
Task 2 ----+----merge attempt----> Workflow Branch
           |                            |
Task 3 ----+                    [Conflict Resolution]
```

Requires: Conflict detection, resolution strategy, or fallback to sequential

**Rebase Strategy**:
```
Task 1: commits A, B
Task 2: commits C, D

After rebase:
Workflow Branch: A -> B -> C' -> D' (linear history)
```

Pros: Clean linear history
Cons: Rewrites history, more complex recovery

### 3.5 State Model Extensions

**WorkflowState Additions**:
```go
type WorkflowState struct {
    // Existing fields...

    // NEW: Git isolation fields
    WorkflowBranch string        // e.g., "quorum/wf-20250129-143000-abc12"
    BaseBranch     string        // e.g., "main"
    MergeStrategy  string        // "sequential", "parallel", "rebase"
    WorktreeRoot   string        // e.g., ".worktrees/wf-abc12"
}
```

**Database Schema Additions**:
```sql
-- Add to workflows table
ALTER TABLE workflows ADD COLUMN workflow_branch TEXT;
ALTER TABLE workflows ADD COLUMN base_branch TEXT DEFAULT 'main';
ALTER TABLE workflows ADD COLUMN merge_strategy TEXT DEFAULT 'sequential';
ALTER TABLE workflows ADD COLUMN worktree_root TEXT;

-- Update active_workflow to support multiple
-- OPTION A: Remove CHECK constraint
ALTER TABLE active_workflow DROP CONSTRAINT active_workflow_id_check;
-- Add index for efficient lookups
CREATE INDEX idx_active_workflow_id ON active_workflow(workflow_id);

-- OPTION B: Replace with workflow status
-- Already tracked in workflows.status = 'running'
-- Remove active_workflow table entirely
```

### 3.6 WorkflowWorktreeManager Interface

```go
// New interface for workflow-level isolation
type WorkflowWorktreeManager interface {
    // Workflow lifecycle
    InitializeWorkflow(ctx context.Context, workflowID string, baseBranch string) (*WorkflowGitInfo, error)
    FinalizeWorkflow(ctx context.Context, workflowID string, merge bool) error
    CleanupWorkflow(ctx context.Context, workflowID string) error

    // Task operations (within workflow context)
    CreateTaskWorktree(ctx context.Context, workflowID string, task *Task) (*WorktreeInfo, error)
    MergeTaskToWorkflow(ctx context.Context, workflowID string, taskID TaskID, strategy string) error

    // Query operations
    GetWorkflowStatus(ctx context.Context, workflowID string) (*WorkflowGitStatus, error)
    ListActiveWorkflows(ctx context.Context) ([]*WorkflowGitInfo, error)
}

type WorkflowGitInfo struct {
    WorkflowID     string
    WorkflowBranch string
    BaseBranch     string
    WorktreeRoot   string
    CreatedAt      time.Time
    TaskCount      int
    PendingMerges  int
}

type WorkflowGitStatus struct {
    HasConflicts     bool
    AheadOfBase      int
    BehindBase       int
    UnmergedTasks    []TaskID
    LastMergeCommit  string
}
```

---

## 4. Unification of Interfaces (CLI/TUI/WebUI)

### 4.1 Current Interface Divergence

The codebase exhibits significant divergence in how different interfaces construct and manage workflow execution:

**CLI Path** (`cmd/quorum/cmd/common.go:54-293`):

```go
func InitPhaseRunner(ctx context.Context, phase core.Phase, maxRetries int, dryRun, sandbox bool) (*PhaseRunnerDeps, error) {
    // 1. Load configuration via viper
    loader := config.NewLoaderWithViper(viper.GetViper())
    cfg, err := loader.Load()

    // 2. Create state manager
    stateManager, err := state.NewStateManagerWithOptions(backend, statePath, stateOpts)

    // 3. Create agent registry
    registry := cli.NewRegistry()
    configureAgentsFromConfig(registry, cfg, loader)

    // 4. Create all adapters manually
    checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
    retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
    // ... 7+ more adapters

    // 5. Create RunnerConfig manually
    runnerConfig := &workflow.RunnerConfig{...}

    return &PhaseRunnerDeps{...}, nil
}
```

**WebUI Path** (`internal/api/runner_factory.go:69-181`):

```go
func (f *RunnerFactory) CreateRunner(ctx context.Context, workflowID string, cp *control.ControlPlane) (*workflow.Runner, *webadapters.WebOutputNotifier, error) {
    // 1. Validate prerequisites
    if f.stateManager == nil { return nil, nil, fmt.Errorf("...") }

    // 2. Load configuration
    cfg, err := f.configLoader.Load()

    // 3. Build runner configuration
    runnerConfig := buildRunnerConfig(cfg)

    // 4. Create service components (different order than CLI)
    checkpointManager := service.NewCheckpointManager(f.stateManager, f.logger)
    retryPolicy := service.NewRetryPolicy(...)
    rateLimiterRegistry := service.NewRateLimiterRegistry()  // NEW instance!

    // 5. Create adapters (subset of CLI adapters)
    // ... similar but not identical

    // 6. Create runner
    runner := workflow.NewRunner(workflow.RunnerDeps{...})

    return runner, outputNotifier, nil
}
```

**Key Differences Identified**:

| Aspect | CLI | WebUI |
|--------|-----|-------|
| Config loading | `viper.GetViper()` + `WithConfigFile()` | `configLoader.Load()` |
| State manager | Created fresh | Pre-injected |
| Agent registry | `cli.NewRegistry()` | Pre-injected `core.AgentRegistry` |
| Rate limiter | Per-runner instance | Per-runner instance |
| Output handling | `logging.Logger` | `WebOutputNotifier` + `EventBus` |
| Control plane | None (synchronous) | `*control.ControlPlane` |
| Heartbeat | None | `*HeartbeatManager` |

### 4.2 Proposed Unified Architecture

```
+------------------------------------------------------------------------+
|                    UNIFIED RUNNER ARCHITECTURE                          |
+------------------------------------------------------------------------+

                        +------------------+
                        | RunnerBuilder    |
                        | (Fluent API)     |
                        +------------------+
                               |
            +------------------+------------------+
            |                  |                  |
            v                  v                  v
    +-------------+    +-------------+    +---------------+
    | CLI Entry   |    | TUI Entry   |    | WebUI Entry   |
    | Point       |    | Point       |    | Point         |
    +-------------+    +-------------+    +---------------+
            |                  |                  |
            v                  v                  v
    +--------+----------+----------+----------+--------+
    |              RunnerBuilder.New()                  |
    |                                                   |
    |  .WithConfig(cfg)                                |
    |  .WithStateManager(sm)                           |
    |  .WithAgentRegistry(ar)                          |
    |  .WithRateLimiter(rl)        <- Shared instance  |
    |  .WithOutputNotifier(on)     <- Interface        |
    |  .WithControlPlane(cp)       <- Optional         |
    |  .WithHeartbeat(hb)          <- Optional         |
    |  .WithWorkflowIsolation(wi)  <- NEW: Git config  |
    |  .Build()                                        |
    +--------------------------------------------------+
                        |
                        v
                +------------------+
                | workflow.Runner  |
                +------------------+
```

**Proposed RunnerBuilder Interface**:

```go
type RunnerBuilder struct {
    config           *RunnerConfig
    stateManager     StateManager
    agentRegistry    core.AgentRegistry
    rateLimiter      *RateLimiterRegistry  // Shared across workflows
    outputNotifier   OutputNotifier
    controlPlane     *control.ControlPlane
    heartbeat        *HeartbeatManager
    gitIsolation     *GitIsolationConfig
    // ...
}

func NewRunnerBuilder() *RunnerBuilder
func (b *RunnerBuilder) WithConfig(cfg *config.Config) *RunnerBuilder
func (b *RunnerBuilder) WithStateManager(sm StateManager) *RunnerBuilder
func (b *RunnerBuilder) WithAgentRegistry(ar core.AgentRegistry) *RunnerBuilder
func (b *RunnerBuilder) WithSharedRateLimiter(rl *RateLimiterRegistry) *RunnerBuilder
func (b *RunnerBuilder) WithOutputNotifier(on OutputNotifier) *RunnerBuilder
func (b *RunnerBuilder) WithControlPlane(cp *control.ControlPlane) *RunnerBuilder
func (b *RunnerBuilder) WithHeartbeat(hb *HeartbeatManager) *RunnerBuilder
func (b *RunnerBuilder) WithGitIsolation(gi *GitIsolationConfig) *RunnerBuilder
func (b *RunnerBuilder) Build() (*Runner, error)
```

### 4.3 Shared Rate Limiter Architecture

Current problem: Each runner creates its own `RateLimiterRegistry`, allowing parallel workflows to exceed API limits.

**Proposed Solution**:

```go
// Global rate limiter registry (singleton)
type GlobalRateLimiterRegistry struct {
    limiters map[string]*RateLimiter  // agent name -> limiter
    mu       sync.RWMutex
}

var globalRateLimiter = &GlobalRateLimiterRegistry{
    limiters: make(map[string]*RateLimiter),
}

func GetGlobalRateLimiter() *GlobalRateLimiterRegistry {
    return globalRateLimiter
}

func (r *GlobalRateLimiterRegistry) Get(agentName string) *RateLimiter {
    r.mu.RLock()
    limiter, exists := r.limiters[agentName]
    r.mu.RUnlock()

    if exists {
        return limiter
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Double-check after acquiring write lock
    if limiter, exists = r.limiters[agentName]; exists {
        return limiter
    }

    // Create new limiter with agent-specific limits
    limiter = NewRateLimiter(getAgentLimits(agentName))
    r.limiters[agentName] = limiter
    return limiter
}
```

### 4.4 Unified Output Interface

```go
// OutputNotifier interface (already exists, needs enforcement)
type OutputNotifier interface {
    // Workflow lifecycle
    WorkflowStarted(state *core.WorkflowState)
    WorkflowStateUpdated(state *core.WorkflowState)
    WorkflowCompleted(state *core.WorkflowState)
    WorkflowFailed(state *core.WorkflowState, err error)

    // Phase lifecycle
    PhaseStarted(phase core.Phase)
    PhaseCompleted(phase core.Phase)

    // Task lifecycle
    TaskStarted(task *core.Task)
    TaskCompleted(task *core.Task, duration time.Duration)
    TaskFailed(task *core.Task, err error)

    // Agent events
    AgentEvent(eventType, agentName, message string, metadata map[string]interface{})

    // Logging
    Log(level, source, message string)
}

// CLI implementation
type CLIOutputNotifier struct {
    logger *logging.Logger
    writer io.Writer  // os.Stdout
}

// WebUI implementation (existing)
type WebOutputNotifier struct {
    eventBus   *events.EventBus
    workflowID string
}

// TUI implementation
type TUIOutputNotifier struct {
    app      *tview.Application
    logView  *tview.TextView
    eventCh  chan OutputEvent
}

// Null implementation for testing/silent mode
type NopOutputNotifier struct{}
```

---

## 5. Impact on Existing Components

### 5.1 Component Impact Matrix

| Component | File Location | Impact Level | Changes Required |
|-----------|---------------|--------------|------------------|
| StateManager Interface | `internal/core/ports.go` | HIGH | Add multi-workflow methods |
| SQLiteStateManager | `internal/adapters/state/sqlite.go` | HIGH | Remove singleton constraint, add workflow branch fields |
| WorktreeManager | `internal/adapters/git/worktree.go` | HIGH | Add workflow namespace, new naming |
| GitClient | `internal/adapters/git/client.go` | MEDIUM | Add merge/rebase operations |
| Runner | `internal/service/workflow/runner.go` | MEDIUM | Per-workflow locking, branch init |
| Executor | `internal/service/workflow/executor.go` | MEDIUM | Workflow-scoped worktrees |
| RunnerFactory | `internal/api/runner_factory.go` | MEDIUM | Git isolation config |
| CLI Commands | `cmd/quorum/cmd/*.go` | MEDIUM | Workflow branch options |
| Planner | `internal/service/workflow/planner.go` | LOW | No direct changes |
| Analyzer | `internal/service/workflow/analyzer.go` | LOW | No direct changes |
| EventBus | `internal/events/bus.go` | LOW | Add workflow scope |

### 5.2 StateManager Interface Changes

**Current Interface** (`internal/core/ports.go` - StateManager):

```go
type StateManager interface {
    Save(ctx context.Context, state *WorkflowState) error
    Load(ctx context.Context) (*WorkflowState, error)  // Loads "active" workflow
    LoadByID(ctx context.Context, id WorkflowID) (*WorkflowState, error)
    AcquireLock(ctx context.Context) error  // Global lock
    ReleaseLock(ctx context.Context) error
    DeactivateWorkflow(ctx context.Context) error
    ArchiveWorkflows(ctx context.Context) (int, error)
    PurgeAllWorkflows(ctx context.Context) (int, error)
    DeleteWorkflow(ctx context.Context, id WorkflowID) error
}
```

**Proposed Interface Extensions**:

```go
type StateManager interface {
    // Existing methods...

    // NEW: Per-workflow locking
    AcquireWorkflowLock(ctx context.Context, workflowID WorkflowID) error
    ReleaseWorkflowLock(ctx context.Context, workflowID WorkflowID) error

    // NEW: Multi-workflow tracking
    ListRunningWorkflows(ctx context.Context) ([]WorkflowID, error)
    SetWorkflowRunning(ctx context.Context, workflowID WorkflowID) error
    ClearWorkflowRunning(ctx context.Context, workflowID WorkflowID) error

    // NEW: Git branch tracking
    GetWorkflowBranch(ctx context.Context, workflowID WorkflowID) (string, error)
    SetWorkflowBranch(ctx context.Context, workflowID WorkflowID, branch string) error
}
```

### 5.3 SQLiteStateManager Migration

**Migration SQL** (new file: `migrations/006_workflow_isolation.sql`):

```sql
-- Migration 006: Workflow-level Git isolation

-- Add git isolation columns to workflows table
ALTER TABLE workflows ADD COLUMN workflow_branch TEXT;
ALTER TABLE workflows ADD COLUMN base_branch TEXT DEFAULT 'main';
ALTER TABLE workflows ADD COLUMN merge_strategy TEXT DEFAULT 'sequential';
ALTER TABLE workflows ADD COLUMN worktree_root TEXT;

-- Create multi-workflow running tracking table
-- Replaces the singleton active_workflow pattern
CREATE TABLE IF NOT EXISTS running_workflows (
    workflow_id TEXT PRIMARY KEY,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lock_holder_pid INTEGER,
    lock_holder_host TEXT,
    heartbeat_at DATETIME
);

-- Create per-workflow lock table
CREATE TABLE IF NOT EXISTS workflow_locks (
    workflow_id TEXT PRIMARY KEY,
    acquired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    holder_pid INTEGER NOT NULL,
    holder_host TEXT NOT NULL,
    expires_at DATETIME
);

-- Update tasks table for workflow-scoped branches
-- Branch naming will include workflow ID
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_branch ON tasks(workflow_id, branch);

-- Insert migration record
INSERT INTO schema_migrations (version, description)
VALUES (6, 'Workflow-level Git isolation');
```

### 5.4 Executor Changes

**Current Task Execution** (`internal/service/workflow/executor.go:186-404`):

```go
func (e *Executor) executeTask(ctx context.Context, wctx *Context, task *core.Task, useWorktrees bool) error {
    // ...
    workDir, worktreeCreated := e.setupWorktree(ctx, wctx, task, taskState, useWorktrees)
    defer e.cleanupWorktree(ctx, wctx, task, worktreeCreated)
    // ...
}
```

**Proposed Changes**:

```go
func (e *Executor) executeTask(ctx context.Context, wctx *Context, task *core.Task, useWorktrees bool) error {
    // ...

    // NEW: Setup worktree within workflow namespace
    workDir, worktreeCreated := e.setupWorkflowScopedWorktree(ctx, wctx, task, taskState, useWorktrees)
    defer e.cleanupWorkflowScopedWorktree(ctx, wctx, task, worktreeCreated)

    // ... execute task ...

    // NEW: Merge task changes to workflow branch
    if worktreeCreated && taskState.Status == core.TaskStatusCompleted {
        if err := e.mergeTaskToWorkflow(ctx, wctx, task); err != nil {
            wctx.Logger.Warn("failed to merge task to workflow branch",
                "task_id", task.ID,
                "error", err,
            )
        }
    }
}

func (e *Executor) setupWorkflowScopedWorktree(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, useWorktrees bool) (string, bool) {
    if !useWorktrees || wctx.Worktrees == nil {
        return "", false
    }

    // Get workflow branch as base
    workflowBranch := wctx.State.WorkflowBranch
    if workflowBranch == "" {
        // Fall back to current behavior
        return e.setupWorktree(ctx, wctx, task, taskState, useWorktrees)
    }

    // Create task worktree from workflow branch
    taskBranch := fmt.Sprintf("%s/%s", workflowBranch, task.ID)
    wtInfo, err := wctx.Worktrees.CreateFromBranch(ctx, task, taskBranch, workflowBranch)
    // ...
}
```

### 5.5 Runner Initialization Changes

**Current Run Method** (`internal/service/workflow/runner.go:230-313`):

```go
func (r *Runner) Run(ctx context.Context, prompt string) error {
    // ...
    if err := r.state.AcquireLock(ctx); err != nil {
        return fmt.Errorf("acquiring lock: %w", err)
    }
    defer func() { _ = r.state.ReleaseLock(ctx) }()

    workflowState := r.initializeState(prompt)
    // ...
}
```

**Proposed Changes**:

```go
func (r *Runner) Run(ctx context.Context, prompt string) error {
    // ...

    // Initialize state first to get workflow ID
    workflowState := r.initializeState(prompt)

    // NEW: Acquire per-workflow lock
    if err := r.state.AcquireWorkflowLock(ctx, workflowState.WorkflowID); err != nil {
        return fmt.Errorf("acquiring workflow lock: %w", err)
    }
    defer func() { _ = r.state.ReleaseWorkflowLock(ctx, workflowState.WorkflowID) }()

    // NEW: Initialize workflow branch
    if r.gitIsolation != nil && r.gitIsolation.Enabled {
        if err := r.initializeWorkflowBranch(ctx, workflowState); err != nil {
            return fmt.Errorf("initializing workflow branch: %w", err)
        }
    }

    // ...
}

func (r *Runner) initializeWorkflowBranch(ctx context.Context, state *core.WorkflowState) error {
    if r.git == nil {
        return nil
    }

    baseBranch, err := r.git.DefaultBranch(ctx)
    if err != nil {
        baseBranch = "main"
    }

    workflowBranch := fmt.Sprintf("quorum/%s", state.WorkflowID)

    if err := r.git.CreateBranch(ctx, workflowBranch, baseBranch); err != nil {
        return fmt.Errorf("creating workflow branch: %w", err)
    }

    state.WorkflowBranch = workflowBranch
    state.BaseBranch = baseBranch

    return nil
}
```

### 5.6 CLI Command Changes

**New CLI Flags** (`cmd/quorum/cmd/run.go`):

```go
func init() {
    // Existing flags...

    // NEW: Git isolation flags
    runCmd.Flags().Bool("workflow-branch", true, "Create isolated branch for workflow")
    runCmd.Flags().String("base-branch", "", "Base branch for workflow (default: main/master)")
    runCmd.Flags().String("merge-strategy", "sequential", "Merge strategy: sequential, parallel, rebase")
    runCmd.Flags().Bool("auto-merge", false, "Auto-merge workflow branch to base on completion")
}
```

**New Subcommands**:

```go
// quorum workflow list-running
var workflowListRunningCmd = &cobra.Command{
    Use:   "list-running",
    Short: "List currently running workflows",
    RunE: func(cmd *cobra.Command, args []string) error {
        // ...
    },
}

// quorum workflow merge <workflow-id>
var workflowMergeCmd = &cobra.Command{
    Use:   "merge <workflow-id>",
    Short: "Merge completed workflow branch to base",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // ...
    },
}
```

---

## 6. Edge Cases and Failure Scenarios

### 6.1 Git Operation Failures

**Scenario: Branch Creation Fails**

```
Trigger: Branch already exists (from previous incomplete workflow)
Location: initializeWorkflowBranch()

Detection:
- git branch creation returns error containing "already exists"

Recovery Strategy:
1. Check if existing branch belongs to current workflow (by commit message/tag)
2. If yes: resume from existing branch
3. If no: create branch with unique suffix (wf-xxx-retry1)
4. Log warning for manual cleanup

Code Pattern:
if err := r.git.CreateBranch(ctx, workflowBranch, baseBranch); err != nil {
    if strings.Contains(err.Error(), "already exists") {
        // Check branch ownership
        if r.isOwnBranch(ctx, workflowBranch, state.WorkflowID) {
            r.logger.Info("resuming with existing workflow branch", "branch", workflowBranch)
            return nil
        }
        // Create alternative branch
        workflowBranch = fmt.Sprintf("%s-retry%d", workflowBranch, time.Now().Unix())
        return r.git.CreateBranch(ctx, workflowBranch, baseBranch)
    }
    return err
}
```

**Scenario: Worktree Creation Fails**

```
Trigger: Disk full, permission denied, path already exists
Location: setupWorkflowScopedWorktree()

Detection:
- os.MkdirAll fails
- git worktree add fails

Recovery Strategy:
1. Check available disk space
2. Attempt cleanup of orphaned worktrees
3. Fall back to main repository execution (no isolation)
4. Mark workflow with degraded isolation mode

Code Pattern:
wtInfo, err := wctx.Worktrees.CreateFromBranch(ctx, task, taskBranch, workflowBranch)
if err != nil {
    if isQuotaExceeded(err) {
        // Cleanup and retry
        cleaned, _ := wctx.Worktrees.CleanupStale(ctx)
        r.logger.Info("cleaned stale worktrees", "count", cleaned)
        wtInfo, err = wctx.Worktrees.CreateFromBranch(ctx, task, taskBranch, workflowBranch)
    }
    if err != nil {
        // Fall back to non-isolated execution
        r.logger.Warn("worktree creation failed, executing without isolation", "error", err)
        state.IsolationMode = "degraded"
        return "", false
    }
}
```

**Scenario: Merge Conflict During Task Integration**

```
Trigger: Two parallel tasks modify same file
Location: mergeTaskToWorkflow()

Detection:
- git merge returns non-zero exit code
- Output contains "CONFLICT"

Recovery Strategy:
1. Abort the merge
2. Mark task as requiring manual resolution
3. Create conflict report file
4. Pause workflow execution
5. Notify user via output interface

Code Pattern:
if err := r.git.Merge(ctx, taskBranch, strategy); err != nil {
    if isConflict(err) {
        _ = r.git.AbortMerge(ctx)

        conflictInfo := &ConflictInfo{
            TaskID:       task.ID,
            TaskBranch:   taskBranch,
            TargetBranch: workflowBranch,
            ConflictFiles: parseConflictFiles(err.Error()),
        }

        state.PendingConflicts = append(state.PendingConflicts, conflictInfo)
        state.Status = core.WorkflowStatusPausedConflict

        if wctx.Output != nil {
            wctx.Output.Log("error", "executor",
                fmt.Sprintf("Merge conflict in task %s. Manual resolution required.", task.ID))
        }

        return fmt.Errorf("merge conflict: %w", err)
    }
    return err
}
```

### 6.2 Process Crash Scenarios

**Scenario: Process Crash During Task Execution**

```
Trigger: SIGKILL, OOM, panic, power loss
Location: Any point during executeTask()

State at Crash:
- Task status: "running"
- Worktree exists but may have uncommitted changes
- Workflow lock held (file-based)
- Heartbeat stops updating

Detection (on restart):
1. FindZombieWorkflows() detects stale heartbeat
2. Lock file exists but process is dead

Recovery Strategy:
1. Identify zombie workflows via heartbeat check
2. Check worktree status for uncommitted work
3. Offer recovery options:
   a. Resume from last checkpoint
   b. Recover uncommitted changes to new branch
   c. Discard and restart task
4. Clean up stale locks

Code Pattern (existing zombie detection in sqlite.go:1244-1284):
func (m *SQLiteStateManager) FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
    cutoff := time.Now().UTC().Add(-staleThreshold)
    rows, err := m.readDB.QueryContext(ctx, `
        SELECT id FROM workflows
        WHERE status = 'running'
        AND (heartbeat_at IS NULL OR heartbeat_at < ?)
    `, cutoff)
    // ...
}

// NEW: Recovery workflow
func (r *Runner) RecoverZombieWorkflow(ctx context.Context, workflowID WorkflowID) error {
    state, err := r.state.LoadByID(ctx, workflowID)
    if err != nil {
        return err
    }

    // Check for uncommitted worktree changes
    for _, taskState := range state.Tasks {
        if taskState.Status == core.TaskStatusRunning && taskState.WorktreePath != "" {
            if hasUncommittedChanges(taskState.WorktreePath) {
                // Create recovery branch with uncommitted work
                recoveryBranch := fmt.Sprintf("%s-recovery-%d", taskState.Branch, time.Now().Unix())
                if err := commitToRecoveryBranch(taskState.WorktreePath, recoveryBranch); err != nil {
                    r.logger.Warn("failed to recover uncommitted changes", "task", taskState.ID, "error", err)
                }
            }
        }
    }

    // Reset task statuses for retry
    for _, taskState := range state.Tasks {
        if taskState.Status == core.TaskStatusRunning {
            taskState.Status = core.TaskStatusPending
            taskState.Retries++
        }
    }

    state.Status = core.WorkflowStatusPaused
    return r.state.Save(ctx, state)
}
```

**Scenario: Process Crash During Merge**

```
Trigger: Crash while merging task branch to workflow branch
Location: mergeTaskToWorkflow()

State at Crash:
- Merge in progress (MERGE_HEAD exists)
- Partial merge state in index
- Task marked as completed but not merged

Detection:
- .git/MERGE_HEAD exists
- Task status is completed but no merge commit

Recovery Strategy:
1. Detect incomplete merge state
2. Abort merge
3. Re-attempt merge
4. If still fails, mark as conflict

Code Pattern:
func (r *Runner) detectAndRecoverIncompleteGitState(ctx context.Context, state *WorkflowState) error {
    // Check for incomplete merge
    mergeHeadPath := filepath.Join(r.repoPath, ".git", "MERGE_HEAD")
    if _, err := os.Stat(mergeHeadPath); err == nil {
        r.logger.Warn("detected incomplete merge, aborting")
        if err := r.git.AbortMerge(ctx); err != nil {
            return fmt.Errorf("aborting incomplete merge: %w", err)
        }
    }

    // Check for incomplete rebase
    rebaseApplyPath := filepath.Join(r.repoPath, ".git", "rebase-apply")
    if _, err := os.Stat(rebaseApplyPath); err == nil {
        r.logger.Warn("detected incomplete rebase, aborting")
        if err := r.git.AbortRebase(ctx); err != nil {
            return fmt.Errorf("aborting incomplete rebase: %w", err)
        }
    }

    return nil
}
```

### 6.3 Concurrent Workflow Conflicts

**Scenario: Two Workflows Modify Same File**

```
Trigger: Workflow A and B both modify "config.yaml"
Location: When either workflow merges to main

Detection:
- Merge to main fails with conflict
- PR review shows conflicts

Prevention Strategy:
1. File-level locking (optional, high overhead)
2. Pre-merge conflict detection
3. Workflow queue for same-file modifications

Mitigation Strategy:
1. Create PR instead of direct merge
2. Require manual conflict resolution
3. Notify user of potential conflicts at workflow start

Code Pattern:
func (r *Runner) checkPotentialConflicts(ctx context.Context, workflowID WorkflowID) ([]string, error) {
    // Get files this workflow will likely modify (from analysis)
    modifiedFiles := r.predictModifiedFiles(ctx, workflowID)

    // Check other running workflows
    running, err := r.state.ListRunningWorkflows(ctx)
    if err != nil {
        return nil, err
    }

    var conflicts []string
    for _, otherID := range running {
        if otherID == workflowID {
            continue
        }
        otherFiles := r.predictModifiedFiles(ctx, otherID)
        overlap := intersect(modifiedFiles, otherFiles)
        conflicts = append(conflicts, overlap...)
    }

    return conflicts, nil
}
```

**Scenario: Resource Exhaustion with Many Concurrent Workflows**

```
Trigger: 10+ workflows running simultaneously
Resources affected: File descriptors, memory, disk space, API rate limits

Detection:
- Open file descriptor count near limit
- Memory pressure (OOM killer triggered)
- Disk usage threshold exceeded
- API rate limit errors

Prevention Strategy:
1. Global workflow concurrency limit
2. Resource quotas per workflow
3. Backpressure mechanism

Code Pattern:
const MaxConcurrentWorkflows = 5

func (r *Runner) Run(ctx context.Context, prompt string) error {
    // Check concurrent workflow count
    running, err := r.state.ListRunningWorkflows(ctx)
    if err != nil {
        return err
    }

    if len(running) >= MaxConcurrentWorkflows {
        return core.ErrResourceExhausted("MAX_WORKFLOWS",
            fmt.Sprintf("maximum concurrent workflows (%d) reached", MaxConcurrentWorkflows))
    }

    // ... continue with workflow execution
}
```

### 6.4 State Corruption Scenarios

**Scenario: Database Corruption**

```
Trigger: Disk error, incomplete write, SQLite bug
Detection: Query returns SQLITE_CORRUPT

Recovery Strategy:
1. Attempt to load from backup
2. Rebuild state from git history
3. Mark affected workflows for manual review

Code Pattern (existing in sqlite.go:926-975):
func (m *SQLiteStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
    if _, err := os.Stat(m.backupPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("no backup file found at %s", m.backupPath)
    }

    // ... restore from backup
}
```

**Scenario: Git History Divergence**

```
Trigger: Force push on shared branch, external modifications
Detection:
- Rebase/merge fails unexpectedly
- Commit parents don't match expected

Recovery Strategy:
1. Detect divergence via fetch + compare
2. Create backup of local changes
3. Reset to remote state
4. Re-apply local changes as new commits

Code Pattern:
func (r *Runner) detectGitDivergence(ctx context.Context, branch string) (bool, error) {
    // Fetch latest
    if err := r.git.Fetch(ctx, "origin"); err != nil {
        return false, err
    }

    localCommit, err := r.git.RevParse(ctx, branch)
    if err != nil {
        return false, err
    }

    remoteCommit, err := r.git.RevParse(ctx, "origin/"+branch)
    if err != nil {
        // Remote branch doesn't exist
        return false, nil
    }

    // Check if local is ancestor of remote
    isAncestor, err := r.git.IsAncestor(ctx, localCommit, remoteCommit)
    if err != nil {
        return false, err
    }

    return !isAncestor && localCommit != remoteCommit, nil
}
```

---

## 7. Performance and Resource Considerations

### 7.1 Resource Consumption Analysis

**Disk Space Requirements**:

| Component | Per Workflow | Per Task | Notes |
|-----------|--------------|----------|-------|
| Worktree checkout | ~Size of repo | Same | Full working copy |
| Git objects | Minimal | ~delta | Shared with main repo via hardlinks |
| State database | ~10KB | ~1KB per task | Grows with checkpoints |
| Report files | ~50KB | ~5KB | Markdown + logs |

**Calculation for 5 concurrent workflows with 10 tasks each**:
```
Disk: 5 repos × 100MB = 500MB worktrees
      50 tasks × 5MB delta = 250MB git objects
      5 workflows × 100KB state = 500KB database
      5 workflows × 500KB reports = 2.5MB
Total: ~750MB peak disk usage
```

**Memory Requirements**:

| Component | Per Process | Per Workflow | Notes |
|-----------|-------------|--------------|-------|
| Runner instance | ~5MB | ~5MB | Includes all adapters |
| Agent process | ~50MB | ~50MB per concurrent task | External CLI processes |
| SQLite connections | ~1MB | Shared | WAL mode, 10 read + 1 write |
| Git operations | ~10MB | ~10MB during merge | Peaks during diff/merge |

**Calculation for 5 concurrent workflows**:
```
Memory: 5 runners × 5MB = 25MB
        25 concurrent tasks × 50MB = 1.25GB agents
        SQLite connections = ~5MB
        Git operations = ~50MB peak
Total: ~1.3GB peak memory usage
```

**File Descriptor Usage**:

| Component | FDs per Workflow | Notes |
|-----------|------------------|-------|
| SQLite database | 3-5 | WAL mode uses multiple files |
| Log files | 1-2 | Report writer |
| Git worktree | 1-3 | Lock files |
| Agent processes | 3+ | stdin/stdout/stderr |

**Calculation**:
```
Default limit: 1024 FDs
5 workflows × 15 FDs = 75 base
25 tasks × 5 FDs = 125 during execution
Total: ~200 FDs peak (well within limits)
```

### 7.2 Git Operation Performance

**Worktree Creation**:
```
Operation: git worktree add
Time: O(file_count) - typically 1-5 seconds for medium repos
I/O: Full checkout of files (may be optimized with sparse-checkout)
```

**Branch Operations**:
```
Operation: git branch creation
Time: O(1) - milliseconds
I/O: Single ref file creation

Operation: git checkout
Time: O(changed_files)
I/O: Only different files updated
```

**Merge Operations**:
```
Operation: git merge (fast-forward)
Time: O(1) - just ref update
I/O: Minimal

Operation: git merge (3-way)
Time: O(changed_files + conflicts)
I/O: Reads common ancestor, both tips
```

### 7.3 Database Performance

**Current Query Patterns** (`internal/adapters/state/sqlite.go`):

Read operations use dedicated read connection pool:
```go
readDB.SetMaxOpenConns(10)  // Line 98
readDB.SetMaxIdleConns(5)
```

Write operations serialize through single connection:
```go
db.SetMaxOpenConns(1)  // Line 84
```

**Scaling Considerations**:

For 5 concurrent workflows with frequent state saves:
- Writes: ~1 per task completion + periodic heartbeats
- Reads: ~10 per workflow (status checks, listings)

Write bottleneck analysis:
```
Assume: 5 workflows, 10 tasks each, 1 minute per task average
Write frequency: 50 task completions / 10 minutes = 5 writes/minute
Heartbeat frequency: 5 workflows × 6/minute = 30 writes/minute
Total: ~35 writes/minute = ~0.6 writes/second

SQLite capacity: ~1000 writes/second (WAL mode)
Utilization: <0.1% - no bottleneck
```

### 7.4 Optimization Strategies

**Strategy 1: Sparse Checkout for Large Repos**

```go
func (m *WorkflowWorktreeManager) CreateSparseWorktree(ctx context.Context, workflowID string, patterns []string) error {
    // Create worktree with sparse-checkout
    wtPath := filepath.Join(m.baseDir, workflowID)

    // Initialize sparse-checkout
    if _, err := m.git.run(ctx, "sparse-checkout", "init", "--cone"); err != nil {
        return err
    }

    // Set sparse-checkout patterns
    if _, err := m.git.run(ctx, "sparse-checkout", "set", strings.Join(patterns, " ")); err != nil {
        return err
    }

    return nil
}
```

**Strategy 2: Batch State Saves**

```go
type BatchingStateManager struct {
    underlying StateManager
    pending    []*core.WorkflowState
    mu         sync.Mutex
    flushCh    chan struct{}
}

func (b *BatchingStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
    b.mu.Lock()
    b.pending = append(b.pending, state)
    b.mu.Unlock()

    select {
    case b.flushCh <- struct{}{}:
    default:
    }

    return nil
}

func (b *BatchingStateManager) flushLoop() {
    ticker := time.NewTicker(500 * time.Millisecond)
    for {
        select {
        case <-b.flushCh:
        case <-ticker.C:
        }
        b.flush()
    }
}
```

**Strategy 3: Lazy Worktree Creation**

```go
func shouldDeferWorktree(wctx *Context, task *core.Task) bool {
    // Don't create worktree until task actually starts executing
    // Reduces disk usage for workflows with many pending tasks

    // Always create if parallel mode
    if wctx.Config.WorktreeMode == "parallel" {
        return false
    }

    // Defer if task has unmet dependencies
    for _, depID := range task.Dependencies {
        depState := wctx.State.Tasks[depID]
        if depState == nil || depState.Status != core.TaskStatusCompleted {
            return true
        }
    }

    return false
}
```

**Strategy 4: Connection Pooling for Git Operations**

```go
type GitClientPool struct {
    clients map[string]*Client  // path -> client
    mu      sync.RWMutex
    maxAge  time.Duration
}

func (p *GitClientPool) Get(repoPath string) (*Client, error) {
    p.mu.RLock()
    if client, ok := p.clients[repoPath]; ok {
        p.mu.RUnlock()
        return client, nil
    }
    p.mu.RUnlock()

    p.mu.Lock()
    defer p.mu.Unlock()

    // Double-check
    if client, ok := p.clients[repoPath]; ok {
        return client, nil
    }

    client, err := NewClient(repoPath)
    if err != nil {
        return nil, err
    }

    p.clients[repoPath] = client
    return client, nil
}
```

### 7.5 Monitoring and Alerting

**Metrics to Track**:

```go
type WorkflowMetrics struct {
    // Counters
    WorkflowsStarted      int64
    WorkflowsCompleted    int64
    WorkflowsFailed       int64
    TasksExecuted         int64
    MergeConflicts        int64

    // Gauges
    ActiveWorkflows       int64
    PendingTasks          int64
    WorktreeCount         int64
    DiskUsageBytes        int64

    // Histograms
    WorkflowDuration      []time.Duration
    TaskDuration          []time.Duration
    MergeDuration         []time.Duration

    // Errors
    GitErrors             int64
    StateErrors           int64
    AgentTimeouts         int64
}
```

**Alert Thresholds**:

| Metric | Warning | Critical |
|--------|---------|----------|
| Active workflows | >3 | >5 |
| Disk usage | >80% | >95% |
| Merge conflicts/hour | >5 | >10 |
| Average task duration | >10min | >30min |
| Git errors/hour | >10 | >50 |
| Zombie workflows | >0 | >2 |

---

## Summary

This analysis provides a comprehensive examination of implementing workflow-level Git isolation in the Quorum system. The key findings are:

1. **Current Architecture Limitations**:
   - Single active workflow constraint in database schema
   - Global file-based locking prevents concurrent execution
   - Task-level worktrees lack workflow namespace
   - No merge/rebase operations in GitClient

2. **Required Changes**:
   - Database migration to remove singleton constraint
   - Per-workflow locking mechanism
   - Workflow-scoped branch and worktree naming
   - Extended GitClient interface for merge operations
   - Unified RunnerBuilder for all entry points
   - Shared global rate limiter

3. **Risk Areas**:
   - Merge conflicts between concurrent workflows
   - Resource exhaustion with many workflows
   - Process crash recovery complexity
   - Git history divergence detection

4. **Performance Considerations**:
   - Disk usage scales linearly with concurrent workflows
   - SQLite write capacity is adequate
   - Git operations may benefit from sparse checkout
   - Connection pooling can reduce overhead

The proposed architecture maintains backward compatibility while enabling true concurrent workflow execution with proper Git isolation. Implementation should proceed incrementally, starting with database schema changes and progressing through the component stack.
