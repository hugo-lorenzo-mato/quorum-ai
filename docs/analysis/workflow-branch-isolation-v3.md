> **Estado: HISTORICO -- PARCIALMENTE IMPLEMENTADO** / **Status: HISTORICAL -- PARTIALLY IMPLEMENTED**
>
> This analysis (in Spanish) informed the implementation of workflow-level Git isolation and the RunnerBuilder pattern. The implementation diverged from the proposal: `WorkflowWorktreeManager` replaced the proposed `WorkflowBranchManager`, and `WorkflowOrchestrator` was not implemented as a separate component. See `internal/adapters/git/workflow_worktree.go` and `internal/service/workflow/builder.go`. Line references are stale.

# Análisis Arquitectónico: Aislamiento Git a Nivel de Workflow

**Documento:** V3 - Análisis Definitivo
**Fecha:** 2026-01-29
**Autor:** Agente de Análisis
**Estado:** Completo y Autónomo

---

## Resumen Ejecutivo

Este documento presenta el análisis arquitectónico completo para implementar aislamiento Git a nivel de workflow en Quorum, un sistema de orquestación multi-agente para desarrollo de software. El análisis aborda la elevación del aislamiento actual de tareas (worktrees por tarea) a un modelo de ramas por workflow, habilitando la ejecución concurrente de múltiples workflows y la unificación de interfaces CLI/TUI/WebUI.

### Objetivos Principales

1. **Aislamiento por Workflow**: Crear una rama Git dedicada para cada workflow
2. **Concurrencia de Workflows**: Permitir múltiples workflows ejecutándose simultáneamente
3. **Preservación del Historial**: Mantener trazabilidad completa de cambios por workflow
4. **Unificación de Interfaces**: Convergencia del modelo de ejecución CLI/TUI/WebUI

---

## 1. Arquitectura Actual de Ejecución

### 1.1 Componentes Principales

El sistema Quorum utiliza arquitectura hexagonal con separación clara entre puertos y adaptadores:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CAPA DE PRESENTACIÓN                              │
├─────────────────────────────────────────────────────────────────────────────┤
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────────┐    │
│   │     CLI     │    │     TUI     │    │           WebUI             │    │
│   │  cmd/run.go │    │  (future)   │    │  internal/api/workflows.go  │    │
│   └──────┬──────┘    └──────┬──────┘    └──────────────┬──────────────┘    │
│          │                  │                          │                    │
└──────────┼──────────────────┼──────────────────────────┼────────────────────┘
           │                  │                          │
           ▼                  ▼                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          CAPA DE ORQUESTACIÓN                               │
├─────────────────────────────────────────────────────────────────────────────┤
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                         workflow.Runner                              │   │
│   │               internal/service/workflow/runner.go:137-158            │   │
│   │                                                                      │   │
│   │  ┌─────────┐  ┌──────────┐  ┌─────────┐  ┌──────────┐              │   │
│   │  │ Refiner │  │ Analyzer │  │ Planner │  │ Executor │              │   │
│   │  └────┬────┘  └────┬─────┘  └────┬────┘  └────┬─────┘              │   │
│   │       │            │             │            │                     │   │
│   │       ▼            ▼             ▼            ▼                     │   │
│   │  ┌──────────────────────────────────────────────────────────────┐  │   │
│   │  │                   workflow.Context                            │  │   │
│   │  │          internal/service/workflow/context.go:60-77           │  │   │
│   │  │  - State (*core.WorkflowState)                               │  │   │
│   │  │  - Agents (core.AgentRegistry)                               │  │   │
│   │  │  - Worktrees (WorktreeManager)                               │  │   │
│   │  │  - Git (core.GitClient)                                      │  │   │
│   │  │  - RateLimits (RateLimiterGetter)                           │  │   │
│   │  │  - Control (*control.ControlPlane)                          │  │   │
│   │  └──────────────────────────────────────────────────────────────┘  │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CAPA DE ADAPTADORES                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐  │
│  │   Git Adapter    │  │  State Adapter   │  │    Agent Adapters        │  │
│  │ adapters/git/    │  │ adapters/state/  │  │   adapters/agents/       │  │
│  │  client.go       │  │  sqlite.go       │  │ claude/gemini/codex/...  │  │
│  │  worktree.go     │  │                  │  │                          │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Flujo de Ejecución Actual

El workflow Runner orquesta cuatro fases secuenciales:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        WORKFLOW EXECUTION FLOW                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   1. REFINE PHASE                                                       │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  Original Prompt ──► Prompt Refiner ──► Optimized Prompt        │   │
│   │  (No Git operations - single agent)                              │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                              │                                          │
│                              ▼                                          │
│   2. ANALYZE PHASE (Multi-Agent Consensus)                             │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  ┌─────────┐  ┌─────────┐  ┌─────────┐                          │   │
│   │  │ Claude  │  │ Gemini  │  │ Codex   │  ... (parallel)          │   │
│   │  └────┬────┘  └────┬────┘  └────┬────┘                          │   │
│   │       │            │            │                                │   │
│   │       ▼            ▼            ▼                                │   │
│   │  ┌──────────────────────────────────────────────────────────┐   │   │
│   │  │         Semantic Moderator (Consensus V1..Vn)            │   │   │
│   │  │         Threshold: 0.80 (configurable)                   │   │   │
│   │  └──────────────────────────────────────────────────────────┘   │   │
│   │                              │                                   │   │
│   │                              ▼                                   │   │
│   │  ┌──────────────────────────────────────────────────────────┐   │   │
│   │  │              Synthesized Analysis                         │   │   │
│   │  └──────────────────────────────────────────────────────────┘   │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                              │                                          │
│                              ▼                                          │
│   3. PLAN PHASE                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  Analysis ──► Planner ──► Task DAG (with dependencies)          │   │
│   │  (Optional: Multi-agent plan synthesis)                          │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                              │                                          │
│                              ▼                                          │
│   4. EXECUTE PHASE                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  Task DAG ──► Executor ──► Parallel Task Execution              │   │
│   │                                                                  │   │
│   │  ┌─────────────────────────────────────────────────────────┐    │   │
│   │  │  For each ready task batch:                              │    │   │
│   │  │    1. Create worktree (git worktree add -b branch path)  │    │   │
│   │  │    2. Execute agent in worktree                          │    │   │
│   │  │    3. Finalize (commit/push/PR)                          │    │   │
│   │  │    4. Cleanup worktree (optional)                        │    │   │
│   │  └─────────────────────────────────────────────────────────┘    │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Referencia de código:**
- Runner principal: `internal/service/workflow/runner.go:137-158`
- Ejecutor de fases: `internal/service/workflow/executor.go:66-72`
- Contexto del workflow: `internal/service/workflow/context.go:60-77`

### 1.3 Modelo de Aislamiento Actual (Task-Level Worktrees)

El aislamiento actual opera a nivel de **tarea**, no de **workflow**:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    CURRENT: TASK-LEVEL ISOLATION                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  main (branch)                                                          │
│       │                                                                 │
│       ├──► worktree: .worktrees/quorum-task1__implement-auth            │
│       │         branch: quorum/task1__implement-auth                    │
│       │                                                                 │
│       ├──► worktree: .worktrees/quorum-task2__add-database              │
│       │         branch: quorum/task2__add-database                      │
│       │                                                                 │
│       └──► worktree: .worktrees/quorum-task3__create-api                │
│                 branch: quorum/task3__create-api                        │
│                                                                         │
│  LIMITATION: All tasks from ALL workflows share the same branch space   │
│  No workflow-level isolation or grouping                                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Implementación actual:**
```go
// internal/adapters/git/worktree.go:520-555
func (m *TaskWorktreeManager) Create(ctx context.Context, task *core.Task, branch string) (*core.WorktreeInfo, error) {
    return m.CreateFromBranch(ctx, task, branch, "")
}

func (m *TaskWorktreeManager) CreateFromBranch(ctx context.Context, task *core.Task, branch, baseBranch string) (*core.WorktreeInfo, error) {
    name, usedFallback, err := buildWorktreeName(task)  // task-based naming
    // ...
    resolvedBranch, err := resolveWorktreeBranch(name, branch)  // quorum/{task-id}
    wt, err := m.manager.CreateFromBranch(ctx, name, resolvedBranch, baseBranch)
    // ...
}
```

### 1.4 Estructura de Estado Persistido

El estado del workflow se persiste en SQLite con la siguiente estructura:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        WORKFLOW STATE STRUCTURE                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  core.WorkflowState (internal/core/ports.go:218-243)                   │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  WorkflowID      WorkflowID                                        │ │
│  │  Status          WorkflowStatus (pending/running/completed/failed) │ │
│  │  CurrentPhase    Phase (analyze/plan/execute)                      │ │
│  │  Prompt          string                                            │ │
│  │  Tasks           map[TaskID]*TaskState                             │ │
│  │  TaskOrder       []TaskID                                          │ │
│  │  HeartbeatAt     *time.Time  // zombie detection                   │ │
│  │  Metrics         *StateMetrics                                     │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  core.TaskState (internal/core/ports.go:257-286)                       │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  ID              TaskID                                            │ │
│  │  Status          TaskStatus                                        │ │
│  │  WorktreePath    string      // current task isolation             │ │
│  │  LastCommit      string      // git commit SHA                     │ │
│  │  Branch          string      // git branch used                    │ │
│  │  FilesModified   []string    // recovery metadata                  │ │
│  │  Resumable       bool                                              │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  MISSING: WorkflowState.WorkflowBranch field for workflow isolation    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Análisis de Concurrencia y Bloqueos Potenciales

### 2.1 Mecanismo Actual de Prevención de Doble-Ejecución

El sistema actual previene la ejecución concurrente de un mismo workflow mediante un mapa en memoria:

```go
// internal/api/workflows.go:77-81
var runningWorkflows = struct {
    sync.Mutex
    ids map[string]bool
}{ids: make(map[string]bool)}
```

**Funciones de control:**
```go
// internal/api/workflows.go:84-99
func markRunning(id string) bool {
    runningWorkflows.Lock()
    defer runningWorkflows.Unlock()
    if runningWorkflows.ids[id] {
        return false  // Already running
    }
    runningWorkflows.ids[id] = true
    return true
}

func markFinished(id string) {
    runningWorkflows.Lock()
    defer runningWorkflows.Unlock()
    delete(runningWorkflows.ids, id)
}
```

**Limitación crítica:** Este mecanismo solo previene doble-ejecución del mismo workflow, NO soporta workflows diferentes ejecutándose concurrentemente con aislamiento Git apropiado.

### 2.2 Restricciones de Rate Limiting por CLI

Cada adaptador de agente tiene límites de tasa configurados:

```go
// internal/service/ratelimit.go:146-169
func defaultAdapterConfigs() map[string]RateLimiterConfig {
    return map[string]RateLimiterConfig{
        "claude": {
            MaxTokens:  5,
            RefillRate: 0.5,  // 1 request per 2 seconds
        },
        "gemini": {
            MaxTokens:  10,
            RefillRate: 1,    // 1 request per second
        },
        "codex": {
            MaxTokens:  3,
            RefillRate: 0.2,  // 1 request per 5 seconds
        },
        "copilot": {
            MaxTokens:  5,
            RefillRate: 0.5,
        },
        "opencode": {
            MaxTokens:  10,
            RefillRate: 1,    // 1 request per second (local Ollama)
        },
    }
}
```

**Análisis de capacidad concurrente:**

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    RATE LIMITING CAPACITY ANALYSIS                           │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  CLI       Tokens   RefillRate   Sustained RPM   Burst Capacity             │
│  ────────────────────────────────────────────────────────────────────────   │
│  Claude      5        0.5/s          30              5 requests              │
│  Gemini     10        1.0/s          60             10 requests              │
│  Codex       3        0.2/s          12              3 requests              │
│  Copilot     5        0.5/s          30              5 requests              │
│  OpenCode   10        1.0/s          60             10 requests              │
│                                                                              │
│  CONCURRENT WORKFLOWS IMPACT:                                                │
│  - 2 workflows sharing Claude: Each gets ~15 RPM effective                   │
│  - 3 workflows sharing Codex:  Each gets ~4 RPM effective                    │
│  - Rate limiter is GLOBAL per adapter, not per-workflow                      │
│                                                                              │
│  RECOMMENDATION: Consider per-workflow rate limiting quotas                  │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Bloqueos de Estado SQLite

El adaptador SQLite implementa bloqueo distribuido para acceso al estado:

```go
// internal/adapters/state/sqlite.go (estructura inferida de análisis)
// - Dual connection model: write + read-only
// - WAL mode enabled for concurrency
// - Lock TTL: 1 hour
// - Max retries: 5 with exponential backoff
```

**Puntos de contención identificados:**

1. **Escritura de estado workflow**: Serialización en Save()
2. **Actualización de heartbeat**: Frecuente durante ejecución
3. **Listado de workflows**: Lectura concurrente (bajo impacto con WAL)

### 2.4 Conflictos Git Potenciales

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                       GIT CONFLICT SCENARIOS                                 │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  SCENARIO 1: Multiple workflows modifying same file                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Workflow A (branch: quorum/wf-a)  │  Workflow B (branch: quorum/wf-b) ││
│  │  - Modifies src/auth/login.ts      │  - Modifies src/auth/login.ts     ││
│  │  - Commits and pushes              │  - Commits and pushes             ││
│  │                                    │                                    ││
│  │  MERGE CONFLICT when both try to merge to main                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 2: Dependent tasks across workflows                                │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Workflow A creates: src/utils/helpers.ts                              ││
│  │  Workflow B depends on helpers.ts but B's branch doesn't have it       ││
│  │                                                                         ││
│  │  SOLUTION: Workflow branches should rebase from main periodically      ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 3: Stale branch state                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Workflow branch created 2 hours ago                                   ││
│  │  Main branch has 10 new commits from other workflows                   ││
│  │  Workflow's tasks work on outdated code                                ││
│  │                                                                         ││
│  │  SOLUTION: Pre-task branch freshness check and optional rebase         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Diseño de la Solución de Ramas por Workflow

### 3.1 Modelo Propuesto: Workflow-Level Branch Isolation

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    PROPOSED: WORKFLOW-LEVEL ISOLATION                        │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  main (default branch)                                                       │
│       │                                                                      │
│       ├──► quorum/wf-20260129-143045-k7m9p (Workflow A branch)              │
│       │         │                                                            │
│       │         ├──► worktree: .worktrees/wf-a/task1__implement-auth        │
│       │         │         (inherits from workflow branch)                    │
│       │         │                                                            │
│       │         └──► worktree: .worktrees/wf-a/task2__add-database           │
│       │                   (inherits from workflow branch)                    │
│       │                                                                      │
│       └──► quorum/wf-20260129-145200-m3p2q (Workflow B branch)              │
│                 │                                                            │
│                 ├──► worktree: .worktrees/wf-b/task1__create-api             │
│                 │                                                            │
│                 └──► worktree: .worktrees/wf-b/task2__add-tests              │
│                                                                              │
│  BENEFITS:                                                                   │
│  - Complete isolation between workflows                                      │
│  - Task changes aggregate on workflow branch                                 │
│  - Single PR per workflow to main                                            │
│  - Clear audit trail per workflow                                            │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Nuevas Interfaces Propuestas

#### 3.2.1 WorkflowBranchManager Interface

```go
// internal/core/ports.go (new interface)
// WorkflowBranchManager handles Git branch operations at workflow level
type WorkflowBranchManager interface {
    // CreateWorkflowBranch creates a dedicated branch for a workflow
    // The branch name follows: quorum/{workflow-id}
    CreateWorkflowBranch(ctx context.Context, workflowID WorkflowID, baseBranch string) (*WorkflowBranchInfo, error)

    // GetWorkflowBranch retrieves info about a workflow's branch
    GetWorkflowBranch(ctx context.Context, workflowID WorkflowID) (*WorkflowBranchInfo, error)

    // SyncWorkflowBranch rebases the workflow branch from base if needed
    // Returns true if sync was performed
    SyncWorkflowBranch(ctx context.Context, workflowID WorkflowID) (bool, error)

    // MergeWorkflowBranch merges workflow changes to base branch
    MergeWorkflowBranch(ctx context.Context, workflowID WorkflowID, strategy MergeStrategy) (*MergeResult, error)

    // CleanupWorkflowBranch removes branch and associated worktrees
    CleanupWorkflowBranch(ctx context.Context, workflowID WorkflowID, force bool) error

    // ListWorkflowBranches returns all workflow branches
    ListWorkflowBranches(ctx context.Context) ([]*WorkflowBranchInfo, error)
}

// WorkflowBranchInfo contains workflow branch metadata
type WorkflowBranchInfo struct {
    WorkflowID    WorkflowID
    BranchName    string
    BaseBranch    string
    CreatedAt     time.Time
    LastSyncAt    *time.Time
    CommitCount   int
    AheadOfBase   int
    BehindBase    int
    Status        WorkflowBranchStatus
}

type WorkflowBranchStatus string

const (
    WorkflowBranchStatusActive   WorkflowBranchStatus = "active"
    WorkflowBranchStatusMerged   WorkflowBranchStatus = "merged"
    WorkflowBranchStatusStale    WorkflowBranchStatus = "stale"
    WorkflowBranchStatusConflict WorkflowBranchStatus = "conflict"
)

type MergeStrategy string

const (
    MergeStrategyMerge  MergeStrategy = "merge"
    MergeStrategySquash MergeStrategy = "squash"
    MergeStrategyRebase MergeStrategy = "rebase"
)

type MergeResult struct {
    CommitSHA   string
    Merged      bool
    ConflictFiles []string
}
```

#### 3.2.2 Updated WorkflowState

```go
// internal/core/ports.go (updated struct)
type WorkflowState struct {
    // ... existing fields ...

    // NEW: Workflow-level Git isolation
    WorkflowBranch     string     `json:"workflow_branch,omitempty"`     // quorum/{workflow-id}
    WorkflowBaseBranch string     `json:"workflow_base_branch,omitempty"` // Branch workflow was created from
    BranchCreatedAt    *time.Time `json:"branch_created_at,omitempty"`
    BranchSyncedAt     *time.Time `json:"branch_synced_at,omitempty"`

    // Merge status tracking
    MergeStatus        string     `json:"merge_status,omitempty"`   // pending, merged, conflict
    MergeCommit        string     `json:"merge_commit,omitempty"`   // Merge commit SHA
    MergedAt           *time.Time `json:"merged_at,omitempty"`
}
```

### 3.3 Implementación del WorkflowBranchManager

```go
// internal/adapters/git/workflow_branch.go (new file)
package git

import (
    "context"
    "fmt"
    "time"

    "github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

type WorkflowBranchManagerImpl struct {
    git         *Client
    worktrees   *WorktreeManager
    branchPrefix string
}

func NewWorkflowBranchManager(git *Client, worktrees *WorktreeManager) *WorkflowBranchManagerImpl {
    return &WorkflowBranchManagerImpl{
        git:          git,
        worktrees:    worktrees,
        branchPrefix: "quorum/",
    }
}

func (m *WorkflowBranchManagerImpl) CreateWorkflowBranch(
    ctx context.Context,
    workflowID core.WorkflowID,
    baseBranch string,
) (*core.WorkflowBranchInfo, error) {
    branchName := m.branchPrefix + string(workflowID)

    // Validate base branch
    if baseBranch == "" {
        var err error
        baseBranch, err = m.git.DefaultBranch(ctx)
        if err != nil {
            return nil, fmt.Errorf("getting default branch: %w", err)
        }
    }

    // Check if branch already exists
    exists, err := m.git.BranchExists(ctx, branchName)
    if err != nil {
        return nil, fmt.Errorf("checking branch existence: %w", err)
    }
    if exists {
        return nil, core.ErrValidation("WORKFLOW_BRANCH_EXISTS",
            fmt.Sprintf("branch %s already exists", branchName))
    }

    // Create workflow branch from base
    if err := m.git.CreateBranch(ctx, branchName, baseBranch); err != nil {
        return nil, fmt.Errorf("creating workflow branch: %w", err)
    }

    now := time.Now()
    return &core.WorkflowBranchInfo{
        WorkflowID:  workflowID,
        BranchName:  branchName,
        BaseBranch:  baseBranch,
        CreatedAt:   now,
        Status:      core.WorkflowBranchStatusActive,
    }, nil
}

func (m *WorkflowBranchManagerImpl) SyncWorkflowBranch(
    ctx context.Context,
    workflowID core.WorkflowID,
) (bool, error) {
    info, err := m.GetWorkflowBranch(ctx, workflowID)
    if err != nil {
        return false, err
    }

    // Check if behind base
    if info.BehindBase == 0 {
        return false, nil // Already up to date
    }

    // Attempt rebase (non-destructive check first)
    branchName := m.branchPrefix + string(workflowID)

    // Fetch latest from remote
    if err := m.git.Fetch(ctx, "origin"); err != nil {
        return false, fmt.Errorf("fetching origin: %w", err)
    }

    // Check for conflicts before rebase
    // This would need implementation in GitClient
    // For now, we log and return

    return true, nil
}
```

### 3.4 Integración con el Runner

El flujo de ejecución del workflow se modifica para incluir gestión de rama:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                   UPDATED WORKFLOW EXECUTION FLOW                            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   1. WORKFLOW INITIALIZATION (NEW)                                           │
│   ┌──────────────────────────────────────────────────────────────────────┐   │
│   │  a. Load/Create WorkflowState                                        │   │
│   │  b. >>> CREATE WORKFLOW BRANCH <<<                                   │   │
│   │     - branchManager.CreateWorkflowBranch(ctx, workflowID, "main")    │   │
│   │     - Store branch info in WorkflowState                             │   │
│   │  c. Persist state with branch metadata                               │   │
│   └──────────────────────────────────────────────────────────────────────┘   │
│                              │                                               │
│                              ▼                                               │
│   2. REFINE PHASE (unchanged)                                               │
│   3. ANALYZE PHASE (unchanged)                                              │
│   4. PLAN PHASE (unchanged)                                                 │
│                              │                                               │
│                              ▼                                               │
│   5. EXECUTE PHASE (MODIFIED)                                               │
│   ┌──────────────────────────────────────────────────────────────────────┐   │
│   │  For each task batch:                                                │   │
│   │    a. >>> SYNC WORKFLOW BRANCH (optional) <<<                        │   │
│   │       - Check if workflow branch is behind main                      │   │
│   │       - If behind and no conflicts, rebase                          │   │
│   │    b. Create task worktree FROM WORKFLOW BRANCH                      │   │
│   │       - worktrees.CreateFromBranch(task, "", workflowBranch)        │   │
│   │    c. Execute agent in worktree                                      │   │
│   │    d. Merge task changes TO WORKFLOW BRANCH                          │   │
│   │    e. Cleanup task worktree                                          │   │
│   └──────────────────────────────────────────────────────────────────────┘   │
│                              │                                               │
│                              ▼                                               │
│   6. WORKFLOW FINALIZATION (NEW)                                            │
│   ┌──────────────────────────────────────────────────────────────────────┐   │
│   │  a. All tasks completed successfully?                                │   │
│   │  b. >>> MERGE WORKFLOW BRANCH TO MAIN <<<                            │   │
│   │     - branchManager.MergeWorkflowBranch(ctx, wfID, "squash")         │   │
│   │  c. Create PR if configured                                          │   │
│   │  d. Cleanup workflow branch (optional)                               │   │
│   └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.5 Modificaciones al Executor

```go
// internal/service/workflow/executor.go (modified)

// setupWorktree creates a worktree for task isolation if enabled.
// MODIFIED: Now creates from workflow branch instead of HEAD
func (e *Executor) setupWorktree(
    ctx context.Context,
    wctx *Context,
    task *core.Task,
    taskState *core.TaskState,
    useWorktrees bool,
) (workDir string, created bool) {
    if !useWorktrees || wctx.Worktrees == nil {
        return "", false
    }

    // NEW: Use workflow branch as base instead of HEAD
    baseBranch := wctx.State.WorkflowBranch
    if baseBranch == "" {
        // Fallback to dependency branch or HEAD (backward compatibility)
        baseBranch = e.findDependencyBranch(wctx, task)
    }

    var wtInfo *core.WorktreeInfo
    var err error

    if baseBranch != "" {
        wctx.Logger.Info("creating worktree from workflow branch",
            "task_id", task.ID,
            "workflow_branch", baseBranch,
        )
        wtInfo, err = wctx.Worktrees.CreateFromBranch(ctx, task, "", baseBranch)
    } else {
        wtInfo, err = wctx.Worktrees.Create(ctx, task, "")
    }

    if err != nil {
        wctx.Logger.Warn("failed to create worktree, executing in main repo",
            "task_id", task.ID,
            "error", err,
        )
        return "", false
    }

    wctx.Lock()
    taskState.WorktreePath = wtInfo.Path
    wctx.Unlock()

    return wtInfo.Path, true
}
```

---

## 4. Unificación de Interfaces (CLI/TUI/WebUI)

### 4.1 Estado Actual de las Interfaces

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    CURRENT INTERFACE ARCHITECTURE                            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  CLI (cmd/quorum/cmd/run.go)                                                │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Direct workflow.Runner instantiation                                 ││
│  │  - Synchronous execution (blocks until complete)                        ││
│  │  - File-based state (~/.quorum/state.db or local .quorum/)              ││
│  │  - OutputNotifier: tui.Output (terminal progress)                       ││
│  │  - Control: Local ControlPlane instance                                 ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  WebUI (internal/api/workflows.go)                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Factory-based Runner creation                                        ││
│  │  - Asynchronous execution (returns 202 Accepted)                        ││
│  │  - SQLite state (shared .quorum/state.db)                               ││
│  │  - OutputNotifier: WebOutputNotifier (persists events)                  ││
│  │  - Control: Per-workflow ControlPlane (registered in map)               ││
│  │  - Heartbeat: HeartbeatManager for zombie detection                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  DIVERGENCES:                                                               │
│  1. State initialization (CLI creates new, WebUI loads existing)            │
│  2. Execution model (blocking vs async)                                     │
│  3. Progress reporting (terminal vs persisted events)                       │
│  4. Error handling (direct return vs HTTP status codes)                     │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Propuesta de Unificación: WorkflowOrchestrator

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    UNIFIED INTERFACE ARCHITECTURE                            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐                                 │
│   │   CLI   │    │   TUI   │    │  WebUI  │                                 │
│   └────┬────┘    └────┬────┘    └────┬────┘                                 │
│        │              │              │                                       │
│        ▼              ▼              ▼                                       │
│   ┌──────────────────────────────────────────────────────────────────────┐  │
│   │                      WorkflowOrchestrator                             │  │
│   │  ┌────────────────────────────────────────────────────────────────┐  │  │
│   │  │  - Unified workflow lifecycle management                       │  │  │
│   │  │  - Handles both sync and async execution                       │  │  │
│   │  │  - Manages ControlPlane registry                               │  │  │
│   │  │  - Coordinates heartbeat management                            │  │  │
│   │  │  - >>> NEW: WorkflowBranchManager integration <<<              │  │  │
│   │  └────────────────────────────────────────────────────────────────┘  │  │
│   │                              │                                        │  │
│   │                              ▼                                        │  │
│   │  ┌────────────────────────────────────────────────────────────────┐  │  │
│   │  │  Methods:                                                      │  │  │
│   │  │  - CreateWorkflow(ctx, prompt, config) -> WorkflowID           │  │  │
│   │  │  - StartWorkflow(ctx, id, async bool) -> error                 │  │  │
│   │  │  - ResumeWorkflow(ctx, id) -> error                            │  │  │
│   │  │  - PauseWorkflow(ctx, id) -> error                             │  │  │
│   │  │  - CancelWorkflow(ctx, id) -> error                            │  │  │
│   │  │  - GetWorkflowStatus(ctx, id) -> *WorkflowStatus               │  │  │
│   │  │  - SubscribeEvents(ctx, id, ch chan Event) -> error            │  │  │
│   │  │  - MergeWorkflow(ctx, id, strategy) -> *MergeResult            │  │  │
│   │  └────────────────────────────────────────────────────────────────┘  │  │
│   └──────────────────────────────────────────────────────────────────────┘  │
│                              │                                               │
│                              ▼                                               │
│   ┌──────────────────────────────────────────────────────────────────────┐  │
│   │                        workflow.Runner                                │  │
│   └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 WorkflowOrchestrator Interface

```go
// internal/service/orchestrator.go (new file)
package service

import (
    "context"
    "time"

    "github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
    "github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
    "github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// WorkflowOrchestrator provides unified workflow lifecycle management.
// It bridges CLI, TUI, and WebUI interfaces with consistent behavior.
type WorkflowOrchestrator interface {
    // Lifecycle management
    CreateWorkflow(ctx context.Context, opts CreateWorkflowOptions) (core.WorkflowID, error)
    StartWorkflow(ctx context.Context, id core.WorkflowID, async bool) error
    ResumeWorkflow(ctx context.Context, id core.WorkflowID) error
    PauseWorkflow(ctx context.Context, id core.WorkflowID) error
    CancelWorkflow(ctx context.Context, id core.WorkflowID) error

    // Status and state
    GetWorkflow(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)
    ListWorkflows(ctx context.Context, filter WorkflowFilter) ([]core.WorkflowSummary, error)

    // Event subscription
    SubscribeEvents(ctx context.Context, id core.WorkflowID) (<-chan WorkflowEvent, error)

    // Git operations (NEW)
    GetWorkflowBranch(ctx context.Context, id core.WorkflowID) (*core.WorkflowBranchInfo, error)
    MergeWorkflow(ctx context.Context, id core.WorkflowID, strategy core.MergeStrategy) (*core.MergeResult, error)

    // Cleanup
    DeleteWorkflow(ctx context.Context, id core.WorkflowID) error
    CleanupStaleWorkflows(ctx context.Context, age time.Duration) (int, error)
}

type CreateWorkflowOptions struct {
    Prompt      string
    Title       string
    Config      *workflow.Config
    BaseBranch  string  // NEW: Specify base branch for workflow
}

type WorkflowFilter struct {
    Status       []core.WorkflowStatus
    Phase        []core.Phase
    CreatedAfter *time.Time
    Limit        int
}

type WorkflowEvent struct {
    Type      WorkflowEventType
    WorkflowID core.WorkflowID
    Phase     core.Phase
    TaskID    core.TaskID
    Message   string
    Data      map[string]interface{}
    Timestamp time.Time
}

type WorkflowEventType string

const (
    EventWorkflowStarted    WorkflowEventType = "workflow_started"
    EventWorkflowCompleted  WorkflowEventType = "workflow_completed"
    EventWorkflowFailed     WorkflowEventType = "workflow_failed"
    EventPhaseStarted       WorkflowEventType = "phase_started"
    EventPhaseCompleted     WorkflowEventType = "phase_completed"
    EventTaskStarted        WorkflowEventType = "task_started"
    EventTaskCompleted      WorkflowEventType = "task_completed"
    EventTaskFailed         WorkflowEventType = "task_failed"
    EventBranchCreated      WorkflowEventType = "branch_created"
    EventBranchMerged       WorkflowEventType = "branch_merged"
    EventBranchConflict     WorkflowEventType = "branch_conflict"
)
```

---

## 5. Impacto en Componentes Existentes

### 5.1 Matriz de Cambios

| Componente | Archivo | Tipo de Cambio | Complejidad | Riesgo |
|------------|---------|----------------|-------------|--------|
| WorkflowState | `internal/core/ports.go:218-243` | Añadir campos | Baja | Bajo |
| TaskWorktreeManager | `internal/adapters/git/worktree.go:506-650` | Modificar lógica | Media | Medio |
| Executor | `internal/service/workflow/executor.go:702-747` | Modificar setupWorktree | Media | Medio |
| Runner | `internal/service/workflow/runner.go:137-158` | Añadir branchManager | Media | Medio |
| API Server | `internal/api/workflows.go:508-682` | Añadir endpoints | Media | Bajo |
| SQLite Adapter | `internal/adapters/state/sqlite.go` | Nueva migración | Baja | Bajo |
| CLI Run | `cmd/quorum/cmd/run.go:78+` | Usar orchestrator | Alta | Medio |

### 5.2 Cambios en WorkflowState (core/ports.go)

```go
// BEFORE (lines 218-243)
type WorkflowState struct {
    Version         int
    WorkflowID      WorkflowID
    // ... existing fields
}

// AFTER (new fields)
type WorkflowState struct {
    // ... existing fields ...

    // Workflow-level Git isolation (NEW)
    WorkflowBranch     string     `json:"workflow_branch,omitempty"`
    WorkflowBaseBranch string     `json:"workflow_base_branch,omitempty"`
    BranchCreatedAt    *time.Time `json:"branch_created_at,omitempty"`
    BranchSyncedAt     *time.Time `json:"branch_synced_at,omitempty"`
    MergeStatus        string     `json:"merge_status,omitempty"`
    MergeCommit        string     `json:"merge_commit,omitempty"`
    MergedAt           *time.Time `json:"merged_at,omitempty"`
}
```

### 5.3 Migración de Base de Datos

```sql
-- migrations/006_add_workflow_branch.sql

-- Add workflow branch columns to workflow state
ALTER TABLE workflow_states ADD COLUMN workflow_branch TEXT;
ALTER TABLE workflow_states ADD COLUMN workflow_base_branch TEXT;
ALTER TABLE workflow_states ADD COLUMN branch_created_at DATETIME;
ALTER TABLE workflow_states ADD COLUMN branch_synced_at DATETIME;
ALTER TABLE workflow_states ADD COLUMN merge_status TEXT;
ALTER TABLE workflow_states ADD COLUMN merge_commit TEXT;
ALTER TABLE workflow_states ADD COLUMN merged_at DATETIME;

-- Index for branch lookups
CREATE INDEX idx_workflow_branch ON workflow_states(workflow_branch);
```

### 5.4 Nuevos Archivos Requeridos

```
internal/
├── adapters/
│   └── git/
│       └── workflow_branch.go     (NEW - WorkflowBranchManager impl)
├── core/
│   └── ports.go                   (MODIFY - add interfaces)
└── service/
    ├── orchestrator.go            (NEW - WorkflowOrchestrator interface)
    ├── orchestrator_impl.go       (NEW - Implementation)
    └── workflow/
        └── executor.go            (MODIFY - use workflow branch)

internal/adapters/state/
└── migrations/
    └── 006_add_workflow_branch.sql (NEW)
```

---

## 6. Casos Edge y Escenarios de Fallo

### 6.1 Escenarios de Fallo Identificados

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           FAILURE SCENARIOS                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  SCENARIO 1: Branch Creation Fails                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: Git permissions, disk full, branch name conflict                ││
│  │  Impact: Workflow cannot start                                          ││
│  │  Recovery:                                                              ││
│  │    1. Log error with full context                                       ││
│  │    2. Set WorkflowState.Error with details                              ││
│  │    3. Set Status = failed                                               ││
│  │    4. Emit WorkflowFailed event                                         ││
│  │    5. Allow retry after issue resolved                                  ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 2: Workflow Branch Merge Conflict                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: Concurrent workflows modify same files                          ││
│  │  Detection: MergeWorkflowBranch returns ConflictFiles non-empty         ││
│  │  Recovery:                                                              ││
│  │    1. Set MergeStatus = "conflict"                                      ││
│  │    2. Store conflicting files in state                                  ││
│  │    3. Emit BranchConflict event                                         ││
│  │    4. Keep workflow branch intact                                       ││
│  │    5. Require manual resolution or rebase                               ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 3: Process Crash During Execution                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: OOM, SIGKILL, power failure                                     ││
│  │  Detection: Stale heartbeat (existing mechanism)                        ││
│  │  Recovery:                                                              ││
│  │    1. FindZombieWorkflows detects stale workflow                        ││
│  │    2. Workflow branch remains intact (no cleanup on crash)              ││
│  │    3. Resume loads state with branch metadata                           ││
│  │    4. Task worktrees may be orphaned -> cleanup at start                ││
│  │    5. Continue from last checkpoint                                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 4: Concurrent Workflow Exhausts Rate Limits                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: Multiple workflows requesting same CLI simultaneously           ││
│  │  Impact: Slow execution, potential timeouts                             ││
│  │  Mitigation:                                                            ││
│  │    1. Per-workflow rate limit quotas                                    ││
│  │    2. Fair scheduling across workflows                                  ││
│  │    3. Priority queuing for smaller workflows                            ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 5: Base Branch Changes During Workflow                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: Other PRs merged to main while workflow running                 ││
│  │  Impact: Workflow branch becomes outdated                               ││
│  │  Options:                                                               ││
│  │    a. SyncWorkflowBranch before each phase (proactive)                  ││
│  │    b. Sync only when merge attempted (reactive)                         ││
│  │    c. Never sync, require manual rebase (conservative)                  ││
│  │  Recommendation: Option (a) with configurable behavior                  ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  SCENARIO 6: Task Worktree Creation Fails                                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Cause: Disk space, git lock, corrupted repo                            ││
│  │  Current behavior: Execute in main repo (fallback)                      ││
│  │  New behavior:                                                          ││
│  │    1. Try creating from workflow branch                                 ││
│  │    2. If fails, execute in workflow branch directly (no isolation)      ││
│  │    3. Log warning and continue                                          ││
│  │    4. Mark task as "executed_without_isolation"                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Manejo de Errores Propuesto

```go
// Error handling patterns for workflow branch operations

// internal/core/errors.go (additions)
const (
    ErrCodeBranchExists      = "WORKFLOW_BRANCH_EXISTS"
    ErrCodeBranchNotFound    = "WORKFLOW_BRANCH_NOT_FOUND"
    ErrCodeBranchConflict    = "WORKFLOW_BRANCH_CONFLICT"
    ErrCodeBranchStale       = "WORKFLOW_BRANCH_STALE"
    ErrCodeMergeFailed       = "WORKFLOW_MERGE_FAILED"
)

// IsRetryable determines if an error can be retried
func IsRetryable(err error) bool {
    var domainErr *DomainError
    if errors.As(err, &domainErr) {
        switch domainErr.Code {
        case ErrCodeBranchStale:
            return true  // Can sync and retry
        case ErrCodeBranchConflict:
            return false // Requires manual intervention
        case ErrCodeBranchExists:
            return false // Logic error
        }
    }
    return false
}
```

---

## 7. Consideraciones de Rendimiento y Recursos

### 7.1 Impacto de Operaciones Git

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                      GIT OPERATION PERFORMANCE                               │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Operation                  Frequency          Latency (typical)             │
│  ───────────────────────────────────────────────────────────────────────    │
│  Branch Creation            1 per workflow     50-200ms                      │
│  Worktree Creation          1 per task         100-500ms                     │
│  Branch Sync (rebase)       0-N per workflow   200ms-2s (repo size)          │
│  Commit                     1 per task         50-100ms                      │
│  Push                       1 per task         500ms-5s (network)            │
│  Merge                      1 per workflow     100-500ms                     │
│  Branch Deletion            1 per workflow     50-100ms                      │
│                                                                              │
│  TOTAL OVERHEAD PER WORKFLOW (10 tasks):                                    │
│  - Without sync: ~5-15 seconds                                              │
│  - With sync every phase: ~10-25 seconds                                    │
│                                                                              │
│  RECOMMENDATIONS:                                                           │
│  1. Parallelize worktree creation where possible                            │
│  2. Batch push operations (end of phase, not per-task)                      │
│  3. Lazy sync (only when merge attempted)                                   │
│  4. Background cleanup of old branches                                      │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Uso de Recursos con Múltiples Workflows

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    RESOURCE USAGE PROJECTIONS                                │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Concurrent Workflows: 3                                                     │
│  Tasks per Workflow: 5                                                       │
│  Parallel Tasks per Workflow: 2                                              │
│                                                                              │
│  MEMORY:                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Per Workflow:                                                          ││
│  │    - Runner instance:        ~50MB                                      ││
│  │    - State in memory:        ~5MB                                       ││
│  │    - Agent CLI processes:    ~100-500MB per active agent               ││
│  │                                                                         ││
│  │  Total (3 workflows, 6 concurrent agents):                              ││
│  │    Base: 3 * 55MB = 165MB                                              ││
│  │    Agents: 6 * 200MB = 1.2GB                                           ││
│  │    Peak: ~1.5GB                                                        ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  DISK:                                                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Per Worktree:               ~Size of repo (shared object store)        ││
│  │  SQLite DB:                  ~10KB per workflow + 5KB per task          ││
│  │  Log files:                  ~100KB per workflow                        ││
│  │                                                                         ││
│  │  Total (3 workflows, 15 tasks):                                        ││
│  │    Worktrees: Minimal (git uses hardlinks)                             ││
│  │    State DB: ~100KB                                                    ││
│  │    Logs: ~300KB                                                        ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  FILE DESCRIPTORS:                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  Per Workflow:               ~50 (DB, logs, pipes)                     ││
│  │  Per Agent Process:          ~20                                        ││
│  │                                                                         ││
│  │  Total (3 workflows, 6 agents): ~300 FDs                               ││
│  │  Recommended ulimit: 4096+                                             ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 7.3 Optimizaciones Recomendadas

1. **Pooling de Worktrees**: Reutilizar worktrees entre tareas del mismo workflow
2. **Git Object Cache**: Compartir object store entre worktrees (ya implementado por git)
3. **Lazy Branch Sync**: Solo sincronizar cuando hay conflictos potenciales
4. **Background Cleanup**: Limpiar ramas y worktrees obsoletos en goroutine separada
5. **Rate Limit Fairness**: Implementar colas por workflow para distribución equitativa

---

## 8. Plan de Implementación

### 8.1 Fases de Desarrollo

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                      IMPLEMENTATION ROADMAP                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  PHASE 1: Foundation                                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Add WorkflowBranch fields to WorkflowState                           ││
│  │  - Create SQL migration                                                  ││
│  │  - Define WorkflowBranchManager interface                                ││
│  │  - Implement basic WorkflowBranchManagerImpl                            ││
│  │  - Unit tests for branch operations                                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  PHASE 2: Runner Integration                                                │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Modify Runner to use WorkflowBranchManager                           ││
│  │  - Update Executor.setupWorktree to use workflow branch                 ││
│  │  - Add branch creation at workflow start                                ││
│  │  - Add branch merge at workflow completion                              ││
│  │  - Integration tests                                                    ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  PHASE 3: API & CLI Updates                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Add branch endpoints to API                                          ││
│  │  - Update CLI to display branch info                                    ││
│  │  - Add merge command to CLI                                             ││
│  │  - Update WebUI to show branch status                                   ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  PHASE 4: Orchestrator Unification                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Implement WorkflowOrchestrator                                       ││
│  │  - Migrate CLI to use orchestrator                                      ││
│  │  - Migrate WebUI to use orchestrator                                    ││
│  │  - Remove duplicate logic                                               ││
│  │  - End-to-end tests                                                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  PHASE 5: Concurrency & Polish                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  - Test concurrent workflow execution                                   ││
│  │  - Implement rate limit fairness                                        ││
│  │  - Add conflict detection and reporting                                 ││
│  │  - Performance optimization                                             ││
│  │  - Documentation                                                        ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 8.2 Archivos a Crear/Modificar

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `internal/core/ports.go` | MODIFY | Add WorkflowBranchManager interface, update WorkflowState |
| `internal/adapters/git/workflow_branch.go` | CREATE | WorkflowBranchManagerImpl |
| `internal/adapters/state/migrations/006_add_workflow_branch.sql` | CREATE | Database migration |
| `internal/service/workflow/runner.go` | MODIFY | Add branchManager dependency |
| `internal/service/workflow/executor.go` | MODIFY | Use workflow branch in setupWorktree |
| `internal/service/orchestrator.go` | CREATE | WorkflowOrchestrator interface |
| `internal/service/orchestrator_impl.go` | CREATE | Orchestrator implementation |
| `internal/api/workflows.go` | MODIFY | Add branch endpoints |
| `cmd/quorum/cmd/run.go` | MODIFY | Use WorkflowOrchestrator |

---

## 9. Conclusiones

### 9.1 Resumen de Beneficios

1. **Aislamiento Completo**: Cada workflow tiene su propia rama Git
2. **Concurrencia Habilitada**: Múltiples workflows pueden ejecutarse sin interferencia
3. **Trazabilidad Mejorada**: Historial claro de cambios por workflow
4. **Unificación de Interfaces**: Un único modelo de ejecución para CLI/TUI/WebUI
5. **Recuperación Robusta**: El estado de la rama persiste ante fallos

### 9.2 Riesgos Mitigados

- **Conflictos de merge**: Detectados antes de aplicar cambios a main
- **Pérdida de trabajo**: Ramas preservan cambios ante crashes
- **Inconsistencia de estado**: Metadatos de rama sincronizados con estado persistido

### 9.3 Próximos Pasos

1. Revisar y aprobar este análisis
2. Crear issues en el tracker para cada fase
3. Implementar Phase 1 (Foundation) como PR inicial
4. Iterar con feedback del equipo

---

*Documento generado automáticamente por el sistema de análisis Quorum.*
