> **Status: HISTORICAL -- PARTIALLY IMPLEMENTED**
>
> This analysis informed the implementation of workflow-level Git isolation. The actual implementation uses `WorkflowWorktreeManager` (in `internal/adapters/git/workflow_worktree.go`) with a different interface than proposed here. Superseded by V3 analysis. Line references are stale.

# Workflow-Level Git Branch Isolation: Architectural Analysis V2

**Document Version**: 2.0
**Date**: January 2026
**Scope**: Complete architectural analysis for enabling concurrent workflow execution via Git branch isolation

---

## Executive Summary

This document presents a comprehensive analysis for implementing **workflow-level Git branch isolation** in Quorum, enabling true concurrent workflow execution. Currently, Quorum uses task-level worktrees for filesystem isolation but lacks workflow-level branch coordination, preventing multiple workflows from running simultaneously on the same repository.

The proposed solution introduces a `WorkflowBranchManager` component that:
1. Creates isolated branches for each workflow execution
2. Manages the lifecycle from creation through merge/cleanup
3. Handles conflict detection and resolution
4. Integrates seamlessly with existing CLI/TUI/WebUI interfaces

---

## Table of Contents

1. [Current Architecture Analysis](#1-current-architecture-analysis)
2. [AI CLI Concurrency Constraints](#2-ai-cli-concurrency-constraints)
3. [Workflow Branch Design](#3-workflow-branch-design)
4. [Interface Unification Strategy](#4-interface-unification-strategy)
5. [Component Impact Analysis](#5-component-impact-analysis)
6. [Edge Cases and Failure Scenarios](#6-edge-cases-and-failure-scenarios)
7. [Performance Implications](#7-performance-implications)
8. [Implementation Plan](#8-implementation-plan)

---

## 1. Current Architecture Analysis

### 1.1 Execution Entry Points

Quorum exposes three entry points for workflow execution, each with distinct state management patterns:

```
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW ENTRY POINTS                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐       │
│  │    CLI      │     │     API     │     │    TUI      │       │
│  │ cmd/run.go  │     │workflows.go │     │ (embedded)  │       │
│  └──────┬──────┘     └──────┬──────┘     └──────┬──────┘       │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌─────────────────────────────────────────────────────┐       │
│  │              Runner.RunWithState()                   │       │
│  │           internal/service/workflow/runner.go        │       │
│  └─────────────────────────────────────────────────────┘       │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────┐       │
│  │                State Lock Acquisition                │       │
│  │            state.AcquireLock() (lines 246-249)       │       │
│  └─────────────────────────────────────────────────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**CLI Entry** (`cmd/quorum/cmd/run.go`):
- Direct invocation via `quorum run "prompt"`
- Synchronous execution with terminal output
- Uses `RunnerFactory` to construct dependencies

**API Entry** (`internal/api/workflows.go:516-682`):
- HTTP handler `HandleRunWorkflow` starts async execution
- Maintains `runningWorkflows` singleton (lines 78-81) to prevent double-execution
- Returns `202 Accepted` immediately while workflow runs in background

**TUI Entry**:
- Embedded within the Bubble Tea TUI
- Shares the same `Runner` infrastructure

### 1.2 State Management Architecture

The SQLite-based state manager implements a dual-connection model for optimized concurrent access:

```go
// internal/adapters/state/sqlite.go:38-51
type SQLiteStateManager struct {
    dbPath     string
    backupPath string
    lockPath   string
    lockTTL    time.Duration
    db         *sql.DB // Write connection
    readDB     *sql.DB // Read-only connection for non-blocking reads
    mu         sync.RWMutex
    maxRetries    int
    baseRetryWait time.Duration
}
```

**Connection Configuration**:
- **Write connection** (lines 76-87): Single connection with WAL mode, 5-second busy timeout
- **Read connection** (lines 89-101): Up to 10 concurrent readers, 1-second busy timeout

```
┌────────────────────────────────────────────────────────────────┐
│                   SQLite State Architecture                     │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────┐           ┌──────────────────┐           │
│  │   Write Conn     │           │   Read Pool      │           │
│  │  MaxConns: 1     │           │  MaxConns: 10    │           │
│  │  WAL Mode        │           │  Read-Only       │           │
│  │  5s busy_timeout │           │  1s busy_timeout │           │
│  └────────┬─────────┘           └────────┬─────────┘           │
│           │                              │                      │
│           ▼                              ▼                      │
│  ┌─────────────────────────────────────────────────────┐       │
│  │                   quorum.db (WAL)                    │       │
│  │  Tables: workflows, tasks, checkpoints,              │       │
│  │          active_workflow, schema_migrations          │       │
│  └─────────────────────────────────────────────────────┘       │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### 1.3 Current Workflow Execution Model

The `Runner` orchestrates four sequential phases with a single workflow lock:

```go
// internal/service/workflow/runner.go:229-313
func (r *Runner) Run(ctx context.Context, prompt string) error {
    // ...validation...

    // Lock acquisition prevents concurrent workflows
    if err := r.state.AcquireLock(ctx); err != nil {
        return fmt.Errorf("acquiring lock: %w", err)
    }
    defer func() { _ = r.state.ReleaseLock(ctx) }()

    // Phase execution
    if err := r.refiner.Run(ctx, wctx); err != nil { ... }
    if err := r.analyzer.Run(ctx, wctx); err != nil { ... }
    if err := r.planner.Run(ctx, wctx); err != nil { ... }
    if err := r.executor.Run(ctx, wctx); err != nil { ... }

    return r.state.Save(ctx, workflowState)
}
```

**Critical Limitation**: The file-based lock (`lockPath` at line 42) combined with `active_workflow` table (lines 345-355) enforces exactly one running workflow per repository.

### 1.4 Current Task-Level Isolation

Tasks are isolated via Git worktrees, but this operates at task level, not workflow level:

```
┌────────────────────────────────────────────────────────────────┐
│              Current Task Worktree Architecture                 │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Main Repository (HEAD)                                         │
│  └── .worktrees/                                               │
│       ├── quorum-task-1__api-endpoints/                        │
│       │   └── (isolated workspace for task-1)                  │
│       ├── quorum-task-2__frontend-components/                  │
│       │   └── (isolated workspace for task-2)                  │
│       └── quorum-task-3__database-schema/                      │
│           └── (isolated workspace for task-3)                  │
│                                                                 │
│  Branch Naming: quorum/{taskID}__{normalized-label}            │
│  (internal/adapters/git/worktree.go:143)                       │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

**Worktree Creation** (`internal/adapters/git/worktree.go:187-265`):
```go
func (m *WorktreeManager) CreateFromBranch(ctx context.Context, path, branch, base string) error {
    // 1. Create branch from base
    // 2. Create worktree pointing to branch
    // 3. Checkout branch in worktree
}
```

**Worktree Naming Convention** (line 143):
```go
branchName := fmt.Sprintf("quorum/%s__%s", taskID, normalizeLabel(label))
```

### 1.5 Core Data Structures

**WorkflowState** (`internal/core/ports.go:217-243`):
```go
type WorkflowState struct {
    Version         int
    WorkflowID      WorkflowID
    Title           string
    Status          WorkflowStatus
    CurrentPhase    Phase
    Prompt          string
    OptimizedPrompt string
    Tasks           map[TaskID]*TaskState
    TaskOrder       []TaskID
    Config          *WorkflowConfig
    Metrics         *StateMetrics
    Checkpoints     []Checkpoint
    HeartbeatAt     *time.Time
    ResumeCount     int
    MaxResumes      int
}
```

**TaskState** (`internal/core/ports.go:256-286`):
```go
type TaskState struct {
    ID           TaskID
    Phase        Phase
    Name         string
    Status       TaskStatus
    CLI          string
    Model        string
    Dependencies []TaskID
    WorktreePath string        // Current isolation mechanism
    Branch       string        // Git branch for this task
    LastCommit   string        // Recovery checkpoint
    FilesModified []string
    Resumable    bool
    ResumeHint   string
}
```

---

## 2. AI CLI Concurrency Constraints

### 2.1 Rate Limiting Configuration

Each AI CLI adapter has distinct rate limits managed via token bucket algorithm:

```go
// internal/service/ratelimit.go:146-169
func defaultAdapterConfigs() map[string]RateLimiterConfig {
    return map[string]RateLimiterConfig{
        "claude": {
            MaxTokens:  5,
            RefillRate: 0.5, // 1 request per 2 seconds
        },
        "gemini": {
            MaxTokens:  10,
            RefillRate: 1, // 1 request per second
        },
        "codex": {
            MaxTokens:  3,
            RefillRate: 0.2, // 1 request per 5 seconds (MOST CONSERVATIVE)
        },
        "copilot": {
            MaxTokens:  5,
            RefillRate: 0.5, // 1 request per 2 seconds
        },
        "opencode": {
            MaxTokens:  10,
            RefillRate: 1, // 1 request per second (local Ollama)
        },
    }
}
```

### 2.2 Concurrency Summary Table

| CLI      | Max Tokens | Refill Rate | Effective RPS | Notes                          |
|----------|------------|-------------|---------------|--------------------------------|
| Gemini   | 10         | 1.0/sec     | 1.0           | Highest throughput             |
| OpenCode | 10         | 1.0/sec     | 1.0           | Local Ollama, configurable     |
| Claude   | 5          | 0.5/sec     | 0.5           | Anthropic rate limits          |
| Copilot  | 5          | 0.5/sec     | 0.5           | GitHub API limits              |
| Codex    | 3          | 0.2/sec     | 0.2           | **Most conservative**          |

### 2.3 Rate Limit Acquisition Flow

```
┌────────────────────────────────────────────────────────────────┐
│                  Rate Limit Acquisition Flow                    │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Task Execution                                                 │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────────────────────────────┐                   │
│  │ executor.acquireRateLimit(wctx, agent)  │                   │
│  │ (executor.go:488-491)                   │                   │
│  └──────────────────┬──────────────────────┘                   │
│                     │                                           │
│                     ▼                                           │
│  ┌─────────────────────────────────────────┐                   │
│  │ limiter := wctx.RateLimits.Get(agent)   │                   │
│  │ return limiter.Acquire()                │                   │
│  └──────────────────┬──────────────────────┘                   │
│                     │                                           │
│                     ▼                                           │
│  ┌─────────────────────────────────────────┐                   │
│  │ Token Bucket Algorithm                   │                   │
│  │ - Check available tokens                │                   │
│  │ - Wait if needed (refill rate)          │                   │
│  │ - Acquire and decrement                 │                   │
│  └─────────────────────────────────────────┘                   │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### 2.4 Critical Finding: Adapter-Level Independence

Rate limiting operates at the **adapter level**, not workflow level. This means:
- Multiple workflows can share the same rate limiter
- Concurrent workflows using the same CLI will compete for tokens
- No per-workflow token allocation exists

**Implication for Branch Isolation**: Rate limits must remain global across workflows to respect CLI provider quotas. Workflow isolation should focus on Git operations, not API calls.

---

## 3. Workflow Branch Design

### 3.1 Branch Naming Convention

Extend the existing task branch pattern to workflow level:

```
Current (task-level):  quorum/{taskID}__{label}
Proposed (workflow):   quorum/workflow/{workflowID}
                       quorum/workflow/{workflowID}/task/{taskID}__{label}
```

**Examples**:
```
quorum/workflow/wf-20260129-143052-k7m9p                    # Workflow branch
quorum/workflow/wf-20260129-143052-k7m9p/task/task-1__api   # Task worktree
quorum/workflow/wf-20260129-143052-k7m9p/task/task-2__ui    # Task worktree
```

### 3.2 WorkflowBranchManager Interface

```go
// Proposed: internal/core/ports.go (new interface)

// WorkflowBranchManager handles workflow-level Git branch isolation.
type WorkflowBranchManager interface {
    // CreateWorkflowBranch creates an isolated branch for a workflow.
    // base specifies the source branch (typically main/master).
    CreateWorkflowBranch(ctx context.Context, workflowID WorkflowID, base string) (*WorkflowBranchInfo, error)

    // GetWorkflowBranch retrieves branch info for an existing workflow.
    GetWorkflowBranch(ctx context.Context, workflowID WorkflowID) (*WorkflowBranchInfo, error)

    // MergeWorkflowBranch merges the workflow branch back to base.
    // Returns conflict info if merge cannot be completed automatically.
    MergeWorkflowBranch(ctx context.Context, workflowID WorkflowID, opts MergeOptions) (*MergeResult, error)

    // DeleteWorkflowBranch removes the workflow branch and associated task branches.
    DeleteWorkflowBranch(ctx context.Context, workflowID WorkflowID) error

    // ListWorkflowBranches returns all active workflow branches.
    ListWorkflowBranches(ctx context.Context) ([]*WorkflowBranchInfo, error)

    // DetectConflicts checks for merge conflicts without merging.
    DetectConflicts(ctx context.Context, workflowID WorkflowID) (*ConflictInfo, error)

    // ResolveConflicts applies automatic or manual conflict resolution.
    ResolveConflicts(ctx context.Context, workflowID WorkflowID, resolution ConflictResolution) error
}

type WorkflowBranchInfo struct {
    WorkflowID    WorkflowID
    BranchName    string
    BaseBranch    string
    CreatedAt     time.Time
    HeadCommit    string
    TaskBranches  []string
    Status        BranchStatus  // active, merged, stale, conflicted
}

type MergeOptions struct {
    Strategy      MergeStrategy // fast-forward, merge-commit, squash
    CommitMessage string
    AutoResolve   bool          // Attempt automatic conflict resolution
}

type MergeResult struct {
    Success    bool
    CommitSHA  string
    Conflicts  []ConflictFile
    Strategy   MergeStrategy
}

type ConflictInfo struct {
    HasConflicts  bool
    ConflictFiles []ConflictFile
    CanAutoResolve bool
}

type ConflictFile struct {
    Path        string
    OurChanges  string
    TheirChanges string
    Ancestor    string
}

type ConflictResolution struct {
    Strategy    ResolutionStrategy // ours, theirs, manual
    Manual      map[string]string  // path -> resolved content
}
```

### 3.3 Workflow Branch Lifecycle

```
┌────────────────────────────────────────────────────────────────────┐
│                  Workflow Branch Lifecycle                          │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. CREATION                                                        │
│  ┌─────────────────┐                                               │
│  │ Workflow Start  │                                               │
│  └────────┬────────┘                                               │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────────────────────────────┐                       │
│  │ CreateWorkflowBranch(wfID, "main")      │                       │
│  │ - git checkout -b quorum/workflow/{id}  │                       │
│  │ - Record base commit SHA                │                       │
│  │ - Update workflow state                 │                       │
│  └────────┬────────────────────────────────┘                       │
│           │                                                         │
│           ▼                                                         │
│  2. TASK EXECUTION (isolated worktrees on workflow branch)         │
│  ┌─────────────────────────────────────────┐                       │
│  │ For each task:                          │                       │
│  │ - Create worktree from workflow branch  │                       │
│  │ - Execute task in worktree              │                       │
│  │ - Commit changes to task branch         │                       │
│  │ - Merge task branch → workflow branch   │                       │
│  └────────┬────────────────────────────────┘                       │
│           │                                                         │
│           ▼                                                         │
│  3. COMPLETION                                                      │
│  ┌─────────────────────────────────────────┐                       │
│  │ Workflow Complete                       │                       │
│  │ - Detect conflicts with main           │                       │
│  │ - Auto-resolve if possible             │                       │
│  │ - Merge workflow branch → main         │                       │
│  │ - Delete workflow branch               │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
│  4. FAILURE/CLEANUP                                                 │
│  ┌─────────────────────────────────────────┐                       │
│  │ On failure:                             │                       │
│  │ - Preserve branch for recovery          │                       │
│  │ - Mark status as "stale"                │                       │
│  │ - Cleanup after configurable TTL        │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

### 3.4 Concurrent Workflow Isolation Model

```
┌──────────────────────────────────────────────────────────────────────┐
│                   Concurrent Workflow Execution                       │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│                        main branch                                    │
│  ─────────────○───────────────────────────────────────────────────── │
│               │                                                       │
│               ├────────────────────────────────────────┐              │
│               │                                        │              │
│               ▼                                        ▼              │
│  ┌────────────────────────┐            ┌────────────────────────┐    │
│  │ quorum/workflow/wf-A   │            │ quorum/workflow/wf-B   │    │
│  │                        │            │                        │    │
│  │  ├─ task-1 (worktree)  │            │  ├─ task-1 (worktree)  │    │
│  │  ├─ task-2 (worktree)  │            │  ├─ task-2 (worktree)  │    │
│  │  └─ task-3 (worktree)  │            │  └─ task-3 (worktree)  │    │
│  │                        │            │                        │    │
│  │  [No conflicts within] │            │  [No conflicts within] │    │
│  └────────────┬───────────┘            └────────────┬───────────┘    │
│               │                                     │                 │
│               │    Merge (check conflicts)          │                 │
│               ▼                                     ▼                 │
│  ─────────────○─────────────────────────────────────○──────────────  │
│                        main branch                                    │
│                                                                       │
│  KEY INSIGHT: Workflows operate on separate branches. Conflicts only │
│  possible at merge time, not during execution.                        │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.5 Automatic Conflict Resolution Strategy

```go
// Proposed: internal/adapters/git/conflict_resolver.go

type AutoConflictResolver struct {
    git GitClient
}

func (r *AutoConflictResolver) ResolveAutomatically(ctx context.Context, conflicts []ConflictFile) ([]ResolvedFile, error) {
    var resolved []ResolvedFile

    for _, conflict := range conflicts {
        resolution, err := r.attemptAutoResolve(conflict)
        if err != nil {
            return nil, fmt.Errorf("cannot auto-resolve %s: %w", conflict.Path, err)
        }
        resolved = append(resolved, resolution)
    }

    return resolved, nil
}

func (r *AutoConflictResolver) attemptAutoResolve(conflict ConflictFile) (ResolvedFile, error) {
    // Strategy 1: Non-overlapping changes
    if canMergeNonOverlapping(conflict) {
        return mergeNonOverlapping(conflict)
    }

    // Strategy 2: Package.json/go.mod dependency merging
    if isManifestFile(conflict.Path) {
        return mergeManifest(conflict)
    }

    // Strategy 3: Imports section in code files
    if canMergeImports(conflict) {
        return mergeImports(conflict)
    }

    // Cannot auto-resolve
    return ResolvedFile{}, ErrManualResolutionRequired
}
```

---

## 4. Interface Unification Strategy

### 4.1 Current Interface Disparity

| Capability              | CLI        | API           | TUI        |
|------------------------|------------|---------------|------------|
| Start workflow         | `run`      | POST /run     | Button     |
| Resume workflow        | `resume`   | POST /resume  | Button     |
| Cancel workflow        | Ctrl+C     | POST /cancel  | Button     |
| Pause workflow         | -          | POST /pause   | Button     |
| View status            | `status`   | GET /status   | Live view  |
| List workflows         | `list`     | GET /list     | List view  |
| **Concurrent support** | **No**     | **Partial**   | **No**     |

### 4.2 Unified Workflow Manager

```go
// Proposed: internal/service/workflow/manager.go

// WorkflowManager provides unified workflow operations across all interfaces.
type WorkflowManager struct {
    runners      map[WorkflowID]*Runner  // Active runners
    branchMgr    WorkflowBranchManager
    state        StateManager
    rateLimits   *RateLimiterRegistry
    mu           sync.RWMutex
    maxConcurrent int
}

// StartWorkflow begins a new workflow with branch isolation.
func (m *WorkflowManager) StartWorkflow(ctx context.Context, opts StartOptions) (*WorkflowHandle, error) {
    m.mu.Lock()

    // Check concurrent workflow limit
    if len(m.runners) >= m.maxConcurrent {
        m.mu.Unlock()
        return nil, ErrMaxConcurrentWorkflows
    }

    // Create workflow branch
    branch, err := m.branchMgr.CreateWorkflowBranch(ctx, opts.WorkflowID, opts.BaseBranch)
    if err != nil {
        m.mu.Unlock()
        return nil, fmt.Errorf("creating workflow branch: %w", err)
    }

    // Create runner with branch context
    runner := NewRunner(RunnerDeps{
        // ... existing deps ...
        WorkflowBranch: branch,
    })

    m.runners[opts.WorkflowID] = runner
    m.mu.Unlock()

    // Start execution in background
    handle := &WorkflowHandle{
        WorkflowID: opts.WorkflowID,
        Branch:     branch,
        done:       make(chan struct{}),
    }

    go m.executeWorkflow(ctx, runner, opts, handle)

    return handle, nil
}

// GetWorkflow retrieves status for any workflow (active or historical).
func (m *WorkflowManager) GetWorkflow(ctx context.Context, id WorkflowID) (*WorkflowStatus, error) {
    m.mu.RLock()
    runner, active := m.runners[id]
    m.mu.RUnlock()

    if active {
        return runner.GetStatus(), nil
    }

    // Load from state
    state, err := m.state.LoadByID(ctx, id)
    if err != nil {
        return nil, err
    }

    return &WorkflowStatus{
        WorkflowID: id,
        State:      state.Status,
        Phase:      state.CurrentPhase,
        // ...
    }, nil
}

// ListActiveWorkflows returns all currently executing workflows.
func (m *WorkflowManager) ListActiveWorkflows() []WorkflowID {
    m.mu.RLock()
    defer m.mu.RUnlock()

    ids := make([]WorkflowID, 0, len(m.runners))
    for id := range m.runners {
        ids = append(ids, id)
    }
    return ids
}
```

### 4.3 API Handler Updates

```go
// Updated: internal/api/workflows.go

type WorkflowsHandler struct {
    manager *WorkflowManager  // Replace runningWorkflows singleton
    // ...
}

// HandleRunWorkflow now supports concurrent execution.
func (h *WorkflowsHandler) HandleRunWorkflow(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...

    handle, err := h.manager.StartWorkflow(r.Context(), StartOptions{
        WorkflowID: generateWorkflowID(),
        Prompt:     req.Prompt,
        BaseBranch: req.BaseBranch, // Optional: defaults to main
    })

    if errors.Is(err, ErrMaxConcurrentWorkflows) {
        http.Error(w, "maximum concurrent workflows reached", http.StatusTooManyRequests)
        return
    }

    // Return handle for tracking
    json.NewEncoder(w).Encode(RunWorkflowResponse{
        WorkflowID: handle.WorkflowID,
        Branch:     handle.Branch.BranchName,
        Status:     "running",
    })
}
```

### 4.4 CLI Command Updates

```go
// Updated: cmd/quorum/cmd/run.go

func runWorkflow(cmd *cobra.Command, args []string) error {
    manager := getWorkflowManager()

    handle, err := manager.StartWorkflow(cmd.Context(), StartOptions{
        Prompt:       strings.Join(args, " "),
        BaseBranch:   flagBaseBranch,
        Foreground:   flagForeground,  // Wait for completion
    })

    if flagForeground {
        return waitForWorkflow(handle)
    }

    fmt.Printf("Workflow %s started on branch %s\n", handle.WorkflowID, handle.Branch.BranchName)
    return nil
}
```

---

## 5. Component Impact Analysis

### 5.1 Affected Components Matrix

| Component                  | File(s)                        | Impact | Changes Required                              |
|---------------------------|--------------------------------|--------|-----------------------------------------------|
| Runner                    | `runner.go`                    | High   | Branch context, remove global lock            |
| State Manager             | `sqlite.go`                    | Medium | Per-workflow locking, concurrent writes       |
| Worktree Manager          | `worktree.go`                  | High   | Workflow branch awareness                     |
| Executor                  | `executor.go`                  | Medium | Branch-scoped worktree creation               |
| API Handlers              | `workflows.go`                 | High   | Replace singleton, support concurrency        |
| Task Finalizer            | `task_finalizer.go`            | Low    | Merge to workflow branch instead of main      |
| Git Client                | `client.go`                    | Low    | Add conflict detection methods                |
| Rate Limiter              | `ratelimit.go`                 | None   | Already global (correct behavior)             |
| Heartbeat Manager         | `heartbeat.go`                 | Low    | Support multiple workflow heartbeats          |

### 5.2 State Manager Changes

```go
// Modified: internal/adapters/state/sqlite.go

// AcquireWorkflowLock obtains a lock for a specific workflow.
// Multiple workflows can hold locks simultaneously.
func (m *SQLiteStateManager) AcquireWorkflowLock(ctx context.Context, workflowID WorkflowID) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    lockPath := filepath.Join(filepath.Dir(m.dbPath), fmt.Sprintf("workflow-%s.lock", workflowID))

    // Create workflow-specific lock file
    info := lockInfo{
        PID:        os.Getpid(),
        WorkflowID: string(workflowID),
        AcquiredAt: time.Now(),
    }

    return m.createLockFile(lockPath, info)
}

// ReleaseWorkflowLock releases a workflow-specific lock.
func (m *SQLiteStateManager) ReleaseWorkflowLock(ctx context.Context, workflowID WorkflowID) error {
    lockPath := filepath.Join(filepath.Dir(m.dbPath), fmt.Sprintf("workflow-%s.lock", workflowID))
    return os.Remove(lockPath)
}
```

### 5.3 Worktree Manager Integration

```go
// Modified: internal/adapters/git/worktree.go

type TaskWorktreeManager struct {
    mgr           *WorktreeManager
    workflowID    WorkflowID
    workflowBranch string  // NEW: parent workflow branch
}

// Create now creates worktrees from the workflow branch, not main.
func (m *TaskWorktreeManager) Create(ctx context.Context, task *core.Task, label string) (*core.WorktreeInfo, error) {
    // Use workflow branch as base instead of HEAD
    baseBranch := m.workflowBranch
    if baseBranch == "" {
        baseBranch = "HEAD"  // Fallback for legacy single-workflow mode
    }

    return m.mgr.CreateFromBranch(ctx, wtPath, branchName, baseBranch)
}
```

### 5.4 Runner Modifications

```go
// Modified: internal/service/workflow/runner.go

type Runner struct {
    // ... existing fields ...
    workflowBranch *WorkflowBranchInfo  // NEW: isolated branch context
    branchMgr      WorkflowBranchManager // NEW: branch lifecycle manager
}

func (r *Runner) RunWithState(ctx context.Context, state *core.WorkflowState) error {
    // Replace global lock with workflow-specific lock
    if err := r.state.AcquireWorkflowLock(ctx, state.WorkflowID); err != nil {
        return fmt.Errorf("acquiring workflow lock: %w", err)
    }
    defer func() { _ = r.state.ReleaseWorkflowLock(ctx, state.WorkflowID) }()

    // Create workflow branch if not resuming
    if r.workflowBranch == nil {
        branch, err := r.branchMgr.CreateWorkflowBranch(ctx, state.WorkflowID, r.getBaseBranch())
        if err != nil {
            return fmt.Errorf("creating workflow branch: %w", err)
        }
        r.workflowBranch = branch
        state.WorkflowBranch = branch.BranchName  // Persist for resume
    }

    // ... continue with phases ...
}
```

---

## 6. Edge Cases and Failure Scenarios

### 6.1 Failure Scenario Matrix

| Scenario                           | Detection                    | Recovery Strategy                              |
|-----------------------------------|------------------------------|------------------------------------------------|
| Process crash mid-task            | Heartbeat timeout (30s)      | Resume from last checkpoint on workflow branch |
| Git conflict at merge             | Merge command failure        | Auto-resolve or flag for manual intervention   |
| Disk full during worktree create  | OS error on mkdir            | Cleanup partial worktrees, report error        |
| Network failure during agent call | Timeout/connection error     | Retry with backoff, fall back to next agent    |
| Concurrent branch deletion        | Git ref error                | Recreate branch from last known commit         |
| SQLite busy during save           | SQLITE_BUSY error            | Exponential backoff retry (5 attempts)         |
| Two workflows editing same file   | No conflict during execution | Detected at merge time, auto-resolve if possible |

### 6.2 Orphaned Branch Cleanup

```go
// internal/adapters/git/branch_cleanup.go

type BranchCleanupManager struct {
    git           GitClient
    branchMgr     WorkflowBranchManager
    state         StateManager
    staleTTL      time.Duration  // Default: 24 hours
}

func (c *BranchCleanupManager) CleanupOrphanedBranches(ctx context.Context) error {
    // 1. List all workflow branches
    branches, err := c.branchMgr.ListWorkflowBranches(ctx)
    if err != nil {
        return err
    }

    // 2. Check each branch against workflow state
    for _, branch := range branches {
        state, err := c.state.LoadByID(ctx, branch.WorkflowID)
        if err != nil {
            continue
        }

        shouldCleanup := false

        // Orphaned: workflow doesn't exist
        if state == nil {
            shouldCleanup = time.Since(branch.CreatedAt) > c.staleTTL
        }

        // Stale: workflow completed but branch not merged
        if state != nil && state.Status == core.WorkflowStatusCompleted {
            shouldCleanup = time.Since(state.UpdatedAt) > c.staleTTL
        }

        // Failed: workflow failed and exceeded retry window
        if state != nil && state.Status == core.WorkflowStatusFailed {
            shouldCleanup = time.Since(state.UpdatedAt) > c.staleTTL
        }

        if shouldCleanup {
            _ = c.branchMgr.DeleteWorkflowBranch(ctx, branch.WorkflowID)
        }
    }

    return nil
}
```

### 6.3 Conflict Resolution Workflow

```
┌────────────────────────────────────────────────────────────────────┐
│                   Merge Conflict Resolution Flow                    │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Workflow Complete                                                  │
│       │                                                             │
│       ▼                                                             │
│  ┌─────────────────────────────────────┐                           │
│  │ branchMgr.DetectConflicts(wfID)     │                           │
│  └──────────────┬──────────────────────┘                           │
│                 │                                                   │
│       ┌─────────┴─────────┐                                        │
│       │                   │                                         │
│       ▼                   ▼                                         │
│  No Conflicts        Has Conflicts                                  │
│       │                   │                                         │
│       ▼                   ▼                                         │
│  ┌─────────────┐    ┌─────────────────────────────┐                │
│  │ Fast-forward│    │ Can auto-resolve?           │                │
│  │ merge       │    └──────────┬──────────────────┘                │
│  └─────────────┘               │                                    │
│                      ┌─────────┴─────────┐                         │
│                      │                   │                          │
│                      ▼                   ▼                          │
│                 Yes                   No                            │
│                      │                   │                          │
│                      ▼                   ▼                          │
│  ┌─────────────────────────┐    ┌─────────────────────────┐       │
│  │ Apply auto-resolution   │    │ Notify user             │       │
│  │ - Non-overlapping merge │    │ - List conflicting files│       │
│  │ - Manifest merging      │    │ - Provide resolution UI │       │
│  │ - Import consolidation  │    │ - Manual intervention   │       │
│  └───────────┬─────────────┘    └───────────┬─────────────┘       │
│              │                              │                       │
│              ▼                              ▼                       │
│  ┌─────────────────────────────────────────────────────┐           │
│  │                  Complete Merge                      │           │
│  │  - Commit to main                                   │           │
│  │  - Delete workflow branch                           │           │
│  │  - Update workflow state                            │           │
│  └─────────────────────────────────────────────────────┘           │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

### 6.4 Zombie Workflow Detection

The existing heartbeat mechanism extends to support concurrent workflows:

```go
// internal/service/workflow/heartbeat.go

type HeartbeatManager struct {
    state    StateManager
    interval time.Duration
    active   map[WorkflowID]context.CancelFunc  // Track all active workflows
    mu       sync.Mutex
}

// Start begins heartbeat updates for a workflow.
func (h *HeartbeatManager) Start(workflowID WorkflowID) {
    h.mu.Lock()
    defer h.mu.Unlock()

    ctx, cancel := context.WithCancel(context.Background())
    h.active[workflowID] = cancel

    go h.heartbeatLoop(ctx, workflowID)
}

// FindZombies returns workflows with stale heartbeats.
func (h *HeartbeatManager) FindZombies(ctx context.Context, threshold time.Duration) ([]*core.WorkflowState, error) {
    return h.state.FindZombieWorkflows(ctx, threshold)
}
```

---

## 7. Performance Implications

### 7.1 Resource Usage Analysis

| Resource              | Current (Single)    | Proposed (Concurrent)       | Mitigation                   |
|----------------------|---------------------|----------------------------|------------------------------|
| Disk Space           | N worktrees         | N × M worktrees            | Cleanup policy, sparse checkout |
| Memory               | 1 Runner instance   | M Runner instances         | Limit max concurrent         |
| Git Objects          | Linear growth       | Parallel branch objects    | Regular gc, prune            |
| SQLite Connections   | 1W + 10R            | 1W + 10R (shared)          | Connection pooling           |
| File Descriptors     | ~100/workflow       | ~100 × M                   | ulimit configuration         |
| CPU (Git ops)        | Sequential          | Parallel (lock contention) | Per-workflow mutex           |

### 7.2 Recommended Limits

```go
// Proposed: internal/config/limits.go

const (
    // MaxConcurrentWorkflows limits simultaneous workflow executions.
    // Based on: 5 agents × 3 tasks/agent × 100MB/worktree = ~1.5GB at max
    MaxConcurrentWorkflows = 5

    // MaxTasksPerWorkflow limits tasks to prevent resource exhaustion.
    MaxTasksPerWorkflow = 50

    // WorktreeCleanupInterval defines stale worktree check frequency.
    WorktreeCleanupInterval = 1 * time.Hour

    // StaleBranchTTL defines how long to keep completed workflow branches.
    StaleBranchTTL = 24 * time.Hour
)
```

### 7.3 Benchmarks to Establish

Before implementation, establish baseline metrics:

1. **Worktree Creation Time**: Time to create N worktrees in parallel
2. **Branch Merge Time**: Time to merge M-commit branch with conflicts
3. **SQLite Contention**: Write latency under concurrent workflow load
4. **Memory per Workflow**: Baseline Runner + agent subprocess memory

---

## 8. Implementation Plan

### 8.1 Phase 1: Core Infrastructure (Priority: High)

**Deliverables**:
1. `WorkflowBranchManager` interface and implementation
2. Per-workflow locking in `SQLiteStateManager`
3. Updated `WorkflowState` with branch tracking fields

**Code Locations**:
- New: `internal/core/ports.go` (interface)
- New: `internal/adapters/git/workflow_branch.go` (implementation)
- Modified: `internal/adapters/state/sqlite.go` (locking)
- Modified: `internal/core/ports.go` (WorkflowState fields)

**Tests**:
- Unit: Branch creation/deletion
- Unit: Concurrent lock acquisition
- Integration: Two workflows on separate branches

### 8.2 Phase 2: Conflict Detection & Resolution (Priority: High)

**Deliverables**:
1. Conflict detection at merge time
2. Auto-resolution for non-overlapping changes
3. Manual resolution API/CLI commands

**Code Locations**:
- New: `internal/adapters/git/conflict_resolver.go`
- Modified: `internal/adapters/git/client.go` (merge methods)
- New: `cmd/quorum/cmd/resolve.go` (CLI command)

**Tests**:
- Unit: Various conflict scenarios
- Integration: Multi-workflow merge with conflicts

### 8.3 Phase 3: Unified Interface (Priority: Medium)

**Deliverables**:
1. `WorkflowManager` unified coordinator
2. Updated API handlers for concurrency
3. CLI concurrent workflow commands
4. TUI concurrent workflow view

**Code Locations**:
- New: `internal/service/workflow/manager.go`
- Modified: `internal/api/workflows.go`
- Modified: `cmd/quorum/cmd/run.go`
- Modified: TUI components

**Tests**:
- Integration: Start multiple workflows via API
- E2E: CLI start, API status, TUI view

### 8.4 Phase 4: Cleanup & Observability (Priority: Low)

**Deliverables**:
1. Orphaned branch cleanup daemon
2. Metrics for concurrent workflow monitoring
3. Documentation updates

**Code Locations**:
- New: `internal/adapters/git/branch_cleanup.go`
- New: `internal/metrics/workflow_metrics.go`
- Modified: `docs/` documentation

### 8.5 Implementation Sequence Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Implementation Phases                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Week 1-2: Phase 1 - Core Infrastructure                            │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━                          │
│  ├─ WorkflowBranchManager interface                                 │
│  ├─ Implementation with GitClient                                   │
│  ├─ Per-workflow locking                                            │
│  └─ WorkflowState extensions                                        │
│                                                                      │
│  Week 3-4: Phase 2 - Conflict Handling                              │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━                            │
│  ├─ Conflict detection                                              │
│  ├─ Auto-resolution strategies                                      │
│  ├─ Manual resolution API                                           │
│  └─ CLI resolve command                                             │
│                                                                      │
│  Week 5-6: Phase 3 - Interface Unification                          │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━                         │
│  ├─ WorkflowManager coordinator                                     │
│  ├─ API handler updates                                             │
│  ├─ CLI concurrent support                                          │
│  └─ TUI multi-workflow view                                         │
│                                                                      │
│  Week 7: Phase 4 - Cleanup & Polish                                 │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━                              │
│  ├─ Branch cleanup daemon                                           │
│  ├─ Metrics & observability                                         │
│  └─ Documentation                                                   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Appendix A: Configuration Schema

```yaml
# .quorum/config.yaml additions

workflow:
  # Maximum concurrent workflows (default: 5)
  max_concurrent: 5

  # Branch naming prefix
  branch_prefix: "quorum/workflow"

  # Auto-cleanup stale branches after this duration
  stale_branch_ttl: "24h"

  # Merge strategy: fast-forward, merge-commit, squash
  merge_strategy: "merge-commit"

  # Auto-resolve conflicts when possible
  auto_resolve_conflicts: true

  # Preserve branch after completion for this duration
  preserve_completed_branches: "1h"
```

---

## Appendix B: Database Schema Updates

```sql
-- Migration: 006_add_workflow_branch.sql

-- Add workflow branch tracking
ALTER TABLE workflows ADD COLUMN workflow_branch TEXT;
ALTER TABLE workflows ADD COLUMN base_branch TEXT DEFAULT 'main';
ALTER TABLE workflows ADD COLUMN branch_status TEXT DEFAULT 'active';
-- branch_status: active, merged, stale, conflicted

-- Track merge status
ALTER TABLE workflows ADD COLUMN merged_at TIMESTAMP;
ALTER TABLE workflows ADD COLUMN merged_commit TEXT;

-- Index for branch queries
CREATE INDEX idx_workflows_branch ON workflows(workflow_branch);
CREATE INDEX idx_workflows_branch_status ON workflows(branch_status);

-- Update schema version
INSERT INTO schema_migrations (version) VALUES (6);
```

---

## Appendix C: Glossary

| Term                    | Definition                                                |
|------------------------|-----------------------------------------------------------|
| Workflow Branch        | Isolated Git branch for a single workflow execution        |
| Task Worktree          | Git worktree for parallel task execution within workflow   |
| Branch Isolation       | Technique to prevent concurrent workflows from conflicting |
| Heartbeat              | Periodic update proving workflow is still alive            |
| Zombie Workflow        | Workflow with stale heartbeat (process likely crashed)     |
| Auto-Resolution        | Automatic merge conflict resolution for safe patterns      |
| Token Bucket           | Rate limiting algorithm used for CLI API calls             |

---

*Document generated for Quorum multi-agent orchestration system. This analysis is self-contained and requires no external references.*
