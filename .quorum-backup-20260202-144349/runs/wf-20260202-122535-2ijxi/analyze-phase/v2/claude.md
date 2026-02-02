# Viability Analysis: Multi-Tenant/Multi-Project Management in Quorum AI

## Executive Summary

This analysis evaluates the technical feasibility of implementing multi-tenant/multi-project management from a single Quorum AI interface. The feature would allow users to manage all initialized quorums on their machine from one UI instance, with a navigation mechanism (dropdown, tabs, or sidebar) in the upper-right corner for switching between projects.

**Key Finding**: The current architecture is fundamentally designed around a single-project paradigm. Implementing multi-project support is technically viable but requires significant refactoring across all layers: backend state management, API endpoints, SSE event broadcasting, and frontend stores. The estimated complexity is HIGH, affecting approximately 40-50 files across the codebase.

---

## 1. Current Architecture Analysis

### 1.1 High-Level System Architecture

```
+-------------------+     +-------------------+     +-------------------+
|                   |     |                   |     |                   |
|      WebUI        |     |       CLI         |     |       TUI         |
|   (React+Zustand) |     |    (Cobra)        |     |   (Bubbletea)     |
|                   |     |                   |     |                   |
+--------+----------+     +--------+----------+     +--------+----------+
         |                         |                         |
         |  HTTP/SSE               |  Direct                 |  Direct
         |                         |                         |
         v                         v                         v
+--------+-------------------------+-------------------------+----------+
|                                                                       |
|                         API Server (Chi Router)                       |
|                                                                       |
|  +---------------+  +----------------+  +------------------+          |
|  | Workflow API  |  | Config API     |  | SSE Handler      |          |
|  | /api/v1/wf/*  |  | /api/v1/config |  | /api/v1/sse/*    |          |
|  +-------+-------+  +-------+--------+  +--------+---------+          |
|          |                  |                    |                    |
+----------+------------------+--------------------+--------------------+
           |                  |                    |
           v                  v                    v
+----------+------------------+--------------------+--------------------+
|                                                                       |
|                    Core Services Layer                                |
|                                                                       |
|  +---------------+  +----------------+  +------------------+          |
|  | StateManager  |  | EventBus       |  | WorkflowExecutor |          |
|  | (SQLite/JSON) |  | (Pub/Sub)      |  | (UnifiedTracker) |          |
|  +-------+-------+  +-------+--------+  +--------+---------+          |
|          |                  |                    |                    |
+----------+------------------+--------------------+--------------------+
           |                                       |
           v                                       v
+----------+-------------------+    +--------------+-------------------+
|                              |    |                                  |
|  .quorum/ Directory          |    |  AI Agent Adapters               |
|  (Per-Project State)         |    |  (Claude, Gemini, Codex, etc.)   |
|                              |    |                                  |
+------------------------------+    +----------------------------------+
```

### 1.2 Project Initialization Flow

When a user runs `quorum init`, the following structure is created **relative to the current working directory**:

```
project-root/
├── .quorum/
│   ├── config.yaml          # Project-specific configuration
│   ├── state/
│   │   └── state.db         # SQLite database (or state.json)
│   ├── logs/
│   └── runs/
│       └── {workflow-id}/   # Per-workflow artifacts
└── ...project files...
```

**Critical Code Reference**: `cmd/quorum/cmd/init.go:30-40`
```go
func runInit(_ *cobra.Command, _ []string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("getting current directory: %w", err)
    }

    quorumDir := filepath.Join(cwd, ".quorum")
    if err := os.MkdirAll(quorumDir, 0o750); err != nil {
        return fmt.Errorf("creating .quorum directory: %w", err)
    }
    // ...
}
```

**Implication**: Project identity is implicitly tied to the filesystem path. There is no explicit `ProjectID` field anywhere in the system.

---

## 2. State Management Architecture

### 2.1 StateManager Interface

The core state management interface is defined in `internal/core/ports.go:140-235`:

```go
type StateManager interface {
    // Workflow lifecycle
    Save(ctx context.Context, state *WorkflowState) error
    Load(ctx context.Context) (*WorkflowState, error)
    LoadByID(ctx context.Context, id WorkflowID) (*WorkflowState, error)
    ListWorkflows(ctx context.Context) ([]*WorkflowSummary, error)
    DeleteWorkflow(ctx context.Context, id WorkflowID) error

    // Active workflow tracking (SINGLE ACTIVE per project)
    GetActiveWorkflowID(ctx context.Context) (WorkflowID, error)
    SetActiveWorkflowID(ctx context.Context, id WorkflowID) error
    DeactivateWorkflow(ctx context.Context) error

    // Checkpoint management
    CreateCheckpoint(ctx context.Context, state *WorkflowState) (string, error)
    ListCheckpoints(ctx context.Context, workflowID WorkflowID) ([]string, error)
    LoadCheckpoint(ctx context.Context, checkpointID string) (*WorkflowState, error)

    // Additional methods...
}
```

**Single-Project Assumptions**:
1. `GetActiveWorkflowID()` returns ONE active workflow - no project scoping
2. All methods operate on the single database at the configured path
3. No `ProjectID` parameter in any method signature

### 2.2 SQLite Implementation

The SQLite state manager (`internal/adapters/state/sqlite.go:53-66`) maintains:

```go
type SQLiteStateManager struct {
    dbPath     string        // Fixed path: .quorum/state/state.db
    backupPath string
    lockPath   string
    lockTTL    time.Duration
    db         *sql.DB       // Write connection
    readDB     *sql.DB       // Read-only connection
    mu         sync.RWMutex
    // ...
}
```

**Database Schema** (from migrations):
- `workflows` table - stores WorkflowState JSON
- `tasks` table - stores TaskState per workflow
- `checkpoints` table - stores checkpoint data
- `schema_migrations` table - migration tracking

**No `project_id` column exists in any table.**

### 2.3 Configuration Loading

Configuration is loaded from multiple sources with precedence (`internal/config/loader.go:54-91`):

```
Priority Order (highest to lowest):
1. CLI flags (--config path)
2. Environment variables (QUORUM_*)
3. .quorum/config.yaml (project-specific)
4. .quorum.yaml (legacy location)
5. ~/.config/quorum/config.yaml (user-level)
6. Hardcoded defaults
```

**Code Reference**: `internal/config/loader.go:54-91`

The loader searches for config relative to `cwd`, reinforcing the single-project design.

---

## 3. WebUI Architecture

### 3.1 Frontend Component Structure

```
frontend/src/
├── App.jsx                  # Main routing (NO project context)
├── components/
│   ├── Layout.jsx           # Main layout with sidebar
│   ├── MobileBottomNav.jsx  # Mobile navigation
│   └── ...
├── pages/
│   ├── Dashboard.jsx
│   ├── Workflows.jsx
│   ├── Kanban.jsx
│   ├── Chat.jsx
│   └── Settings.jsx
├── stores/
│   ├── workflowStore.js     # Workflow state (GLOBAL)
│   ├── taskStore.js         # Task state (GLOBAL)
│   ├── agentStore.js        # Agent events (GLOBAL)
│   ├── configStore.js       # Configuration (GLOBAL)
│   ├── uiStore.js           # UI state
│   └── kanbanStore.js       # Kanban state
├── hooks/
│   └── useSSE.js            # SSE connection (SINGLE endpoint)
└── lib/
    ├── api.js               # API client
    └── configApi.js         # Config API client
```

### 3.2 Routing Structure

Current routing in `frontend/src/App.jsx:29-38`:

```jsx
<Routes>
  <Route path="/" element={<Dashboard />} />
  <Route path="/workflows" element={<Workflows />} />
  <Route path="/workflows/:id" element={<Workflows />} />
  <Route path="/kanban" element={<Kanban />} />
  <Route path="/chat" element={<Chat />} />
  <Route path="/settings" element={<Settings />} />
</Routes>
```

**Observation**: No `/projects/:projectId/...` prefix exists. Routes assume single-project context.

### 3.3 Zustand Store Architecture

**workflowStore.js** (lines 6-14):
```javascript
const useWorkflowStore = create((set, get) => ({
  workflows: [],           // All workflows (no project filtering)
  activeWorkflow: null,    // SINGLE active workflow
  selectedWorkflowId: null,
  tasks: {},
  loading: false,
  error: null,
  // ...
}));
```

**configStore.js** (lines 27-54):
```javascript
const initialState = {
  config: null,           // Single config object
  etag: null,             // For conflict detection
  lastModified: null,
  source: null,
  localChanges: {},
  isDirty: false,
  // ... conflict state
};
```

**Key Finding**: All stores are global singletons. There is no mechanism for project-scoped state.

### 3.4 SSE Event Handling

The SSE connection (`frontend/src/hooks/useSSE.js:9`) connects to:
```javascript
const SSE_URL = '/api/v1/sse/events';
```

Events are broadcast globally. The handler (`useSSE.js:104-226`) processes all events without project filtering:

```javascript
const handleEvent = useCallback((eventType, data) => {
  switch (eventType) {
    case 'workflow_started':
      handleWorkflowStarted(data);  // No project check
      break;
    case 'workflow_completed':
      handleWorkflowCompleted(data);
      // ...
  }
}, [...]);
```

---

## 4. Backend API Architecture

### 4.1 Server Structure

The API server (`internal/api/server.go:27-55`) holds:

```go
type Server struct {
    stateManager    core.StateManager    // SINGLE state manager
    chatStore       core.ChatStore
    eventBus        *events.EventBus     // GLOBAL event bus
    agentRegistry   core.AgentRegistry
    unifiedTracker  *UnifiedTracker      // Workflow execution tracker
    executor        *WorkflowExecutor
    // ...
}
```

**Critical**: One `StateManager` instance per server. Multi-project would require either:
- A pool of StateManagers (one per project)
- A modified StateManager that accepts ProjectID on all operations

### 4.2 API Endpoints

Current endpoint structure (`internal/api/workflows.go`):

```
GET    /api/v1/workflows/           List all workflows
GET    /api/v1/workflows/{id}       Get specific workflow
GET    /api/v1/workflows/active     Get active workflow
POST   /api/v1/workflows/           Create workflow
PUT    /api/v1/workflows/{id}       Update workflow
DELETE /api/v1/workflows/{id}       Delete workflow
POST   /api/v1/workflows/{id}/run   Start workflow
POST   /api/v1/workflows/{id}/pause Pause workflow
// ... etc
```

**No project scoping**. For multi-project support, endpoints would need:
```
GET /api/v1/projects/{projectId}/workflows/
POST /api/v1/projects/{projectId}/workflows/
// etc.
```

### 4.3 SSE Handler

The SSE handler (`internal/api/sse.go:18-64`) broadcasts all events:

```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    // Subscribe to ALL events
    eventCh := s.eventBus.Subscribe()

    // Stream ALL events to client
    for {
        select {
        case event, ok := <-eventCh:
            s.sendEventToClient(w, flusher, event)
        }
    }
}
```

**Problem**: Every connected client receives every event. Multi-project requires filtering by project ID.

### 4.4 EventBus Architecture

The EventBus (`internal/events/bus.go:45-65`) is a simple pub/sub:

```go
type EventBus struct {
    mu           sync.RWMutex
    subscribers  []*Subscriber
    prioritySubs []*Subscriber
    bufferSize   int
    droppedCount int64
    closed       bool
}
```

Events include `WorkflowID()` but not `ProjectID()`:

```go
type Event interface {
    EventType() string
    Timestamp() time.Time
    WorkflowID() string  // No ProjectID()
}
```

---

## 5. CLI and TUI Analysis

### 5.1 CLI Single-Project Assumptions

The CLI uses `os.Getwd()` extensively to determine project context:

**Files using `os.Getwd()`** (18 locations found):
- `cmd/quorum/cmd/init.go:31` - Project initialization
- `cmd/quorum/cmd/common.go:238` - Worktree manager creation
- `cmd/quorum/cmd/run.go` - Workflow execution
- `cmd/quorum/cmd/chat.go` - Chat session
- `internal/service/workflow/runner.go` - Workflow runner
- `internal/api/server.go` - API server
- And 12 more files...

**Example from common.go:238-243**:
```go
cwd, err := os.Getwd()
if err != nil {
    return nil, fmt.Errorf("getting working directory: %w", err)
}

gitClient, err := git.NewClient(cwd)
```

### 5.2 CLI Flag Structure

Current global flags (`cmd/quorum/cmd/root.go`):
- `--config` - Specify config file path
- `--log-level` - Logging level
- `--log-format` - Logging format

**Missing**: `--project` flag to specify project directory.

### 5.3 TUI Considerations

The TUI (using Bubbletea) operates similarly to CLI - it runs from the current directory and assumes that directory is the project root.

---

## 6. Core Data Model Analysis

### 6.1 Workflow Model

The `Workflow` struct (`internal/core/workflow.go:24-40`) has no project awareness:

```go
type Workflow struct {
    ID             WorkflowID
    Status         WorkflowStatus
    CurrentPhase   Phase
    Prompt         string
    Tasks          map[TaskID]*Task
    TaskOrder      []TaskID
    Config         *WorkflowConfig
    ConsensusScore float64
    TotalCostUSD   float64
    TotalTokensIn  int
    TotalTokensOut int
    CreatedAt      time.Time
    StartedAt      *time.Time
    CompletedAt    *time.Time
    Error          string
    // NO ProjectID field
}
```

### 6.2 WorkflowState (Persisted Form)

Similarly, `WorkflowState` (`internal/core/state.go`) lacks project context:

```go
type WorkflowState struct {
    Version       int
    WorkflowID    WorkflowID
    Title         string
    Status        WorkflowStatus
    CurrentPhase  Phase
    Prompt        string
    // ... many fields
    // NO ProjectID
}
```

### 6.3 API Response Models

`WorkflowResponse` (`internal/api/workflows.go:25-47`):

```go
type WorkflowResponse struct {
    ID              string
    ExecutionID     int
    Title           string
    Status          string
    CurrentPhase    string
    Prompt          string
    // ... metrics, timestamps, etc.
    // NO project_id field
}
```

---

## 7. Identified Single-Project Coupling Points

### 7.1 Summary Table

| Component | File:Line | Coupling Type | Impact |
|-----------|-----------|--------------|--------|
| Project Init | init.go:30-40 | `os.Getwd()` | Project path is CWD |
| State Path | common.go:89-92 | Hardcoded `.quorum/state/state.json` | No project namespace |
| SQLite Manager | sqlite.go:72-133 | Fixed `dbPath` | One DB per instance |
| Active Workflow | ports.go:204-214 | Single `GetActiveWorkflowID()` | No per-project active |
| Config Loader | loader.go:54-91 | CWD-relative search | One config per instance |
| API Server | server.go:27-55 | Single StateManager | All workflows in one pool |
| SSE Handler | sse.go:18-64 | Broadcasts all events | No project filtering |
| EventBus | bus.go:18-28 | No ProjectID in Event | Events not scoped |
| Frontend Routes | App.jsx:31-37 | No `/projects/` prefix | UI assumes single project |
| Workflow Store | workflowStore.js:6-14 | Global `workflows[]` | No project segregation |
| Config Store | configStore.js:27-54 | Single `config` object | One config in memory |

### 7.2 Impact Analysis

```
+------------------------+--------+------------------+
| Affected Layer         | Files  | Estimated Effort |
+------------------------+--------+------------------+
| Core Domain Models     |   5    |   Medium         |
| State Management       |   8    |   High           |
| API Layer              |  12    |   High           |
| EventBus/SSE           |   4    |   Medium         |
| Frontend Stores        |   6    |   High           |
| Frontend Components    |  15    |   Medium         |
| CLI Commands           |  10    |   Medium         |
| Tests                  |  20+   |   High           |
+------------------------+--------+------------------+
| TOTAL                  | ~80    |   Very High      |
+------------------------+--------+------------------+
```

---

## 8. Pros and Cons Matrix

### 8.1 Benefits

| Benefit | User Type | Value | Rationale |
|---------|-----------|-------|-----------|
| Unified dashboard | All | HIGH | View all projects from one interface |
| Cross-project metrics | Power users | MEDIUM | Aggregate cost/token tracking |
| Context switching | Developers | HIGH | Faster project navigation |
| Reduced server instances | Ops | MEDIUM | Single `quorum serve` for all projects |
| Workflow comparison | Teams | MEDIUM | Compare approaches across projects |

### 8.2 Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Data leakage between projects | MEDIUM | HIGH | Strict isolation at API layer |
| Increased memory usage | HIGH | MEDIUM | Lazy-load project state |
| SSE event floods | MEDIUM | MEDIUM | Project-scoped subscriptions |
| Config confusion | LOW | MEDIUM | Clear project indicators in UI |
| Git worktree conflicts | MEDIUM | HIGH | Per-project worktree isolation |
| Migration complexity | HIGH | MEDIUM | Versioned migration scripts |
| Performance degradation | MEDIUM | MEDIUM | Benchmark before/after |

### 8.3 Implementation Costs

| Aspect | Complexity | Code Changes | Risk |
|--------|------------|--------------|------|
| Add ProjectID to models | Low | ~500 lines | Low |
| StateManager pool | High | ~1,500 lines | Medium |
| API project scoping | Medium | ~1,000 lines | Medium |
| SSE event filtering | Medium | ~400 lines | Low |
| Frontend multi-store | High | ~2,000 lines | High |
| Project discovery | Medium | ~800 lines | Medium |
| CLI --project flag | Low | ~300 lines | Low |
| **TOTAL** | **Very High** | **~6,500 lines** | **High** |

---

## 9. Main Technical Difficulties

### 9.1 Difficulty 1: State Manager Refactoring

**Description**: The `StateManager` interface and all implementations assume a single database path. Multi-project requires either interface changes or a manager pool.

**Affected Files**:
- `internal/core/ports.go` (interface definition)
- `internal/adapters/state/sqlite.go` (SQLite implementation)
- `internal/adapters/state/json.go` (JSON implementation)
- `internal/adapters/state/factory.go` (factory function)

**Solution A: Interface Modification**
```go
// Modified interface
type StateManager interface {
    Save(ctx context.Context, projectID string, state *WorkflowState) error
    Load(ctx context.Context, projectID string) (*WorkflowState, error)
    // ... all methods get projectID
}
```
- Pros: Clean API
- Cons: Breaking change to every caller (~30 locations)

**Solution B: Manager Pool**
```go
type ProjectStatePool struct {
    managers map[string]StateManager
    mu       sync.RWMutex
}

func (p *ProjectStatePool) ForProject(projectID string) (StateManager, error) {
    // Return or create manager for project
}
```
- Pros: Maintains interface compatibility
- Cons: Higher memory usage, complexity

**Recommendation**: Solution B (Manager Pool) - maintains backward compatibility, allows lazy loading.

### 9.2 Difficulty 2: SSE Event Isolation

**Description**: The EventBus broadcasts all events to all subscribers. Connected clients receive events from all projects.

**Affected Files**:
- `internal/events/bus.go` - EventBus implementation
- `internal/events/workflow.go` - Event definitions
- `internal/api/sse.go` - SSE handler

**Current Flow**:
```
Workflow Event → EventBus.Publish() → ALL Subscribers → ALL SSE Clients
```

**Required Flow**:
```
Workflow Event → EventBus.Publish() → Project Filter → Matching Clients
```

**Solution**:
1. Add `ProjectID() string` to Event interface:
```go
type Event interface {
    EventType() string
    Timestamp() time.Time
    WorkflowID() string
    ProjectID() string  // NEW
}
```

2. Modify SSE handler to filter:
```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    projectID := r.URL.Query().Get("project")

    for event := range eventCh {
        if projectID == "" || event.ProjectID() == projectID {
            s.sendEventToClient(w, flusher, event)
        }
    }
}
```

**Estimated Changes**: ~400 lines across 4 files.

### 9.3 Difficulty 3: Frontend Store Architecture

**Description**: All Zustand stores are global singletons. They hold state for "the" project, not "a" project.

**Affected Files**:
- `frontend/src/stores/workflowStore.js`
- `frontend/src/stores/taskStore.js`
- `frontend/src/stores/agentStore.js`
- `frontend/src/stores/configStore.js`
- `frontend/src/stores/kanbanStore.js`

**Solution A: Per-Project Store Instances**
```javascript
// stores/projectStoreManager.js
const projectStores = new Map();

export function getWorkflowStore(projectId) {
    if (!projectStores.has(projectId)) {
        projectStores.set(projectId, createWorkflowStore(projectId));
    }
    return projectStores.get(projectId);
}
```

**Solution B: Nested State by Project**
```javascript
const useWorkflowStore = create((set, get) => ({
    projects: {},  // { [projectId]: { workflows: [], activeWorkflow: null, ... } }
    currentProjectId: null,

    setCurrentProject: (projectId) => set({ currentProjectId: projectId }),

    getProjectState: () => {
        const { projects, currentProjectId } = get();
        return projects[currentProjectId] || { workflows: [], activeWorkflow: null };
    },
}));
```

**Recommendation**: Solution B - simpler implementation, single store subscription pattern maintained.

### 9.4 Difficulty 4: Project Discovery

**Description**: No mechanism exists to discover initialized Quorum projects on the system.

**Options**:

| Approach | Pros | Cons |
|----------|------|------|
| Directory scanning | Automatic discovery | Slow, permission issues |
| Registry file | Fast lookup | Manual management |
| User configuration | Explicit control | User effort |

**Recommended Approach**: Hybrid
1. User-configured project list in `~/.config/quorum/projects.yaml`
2. "Add Project" action that validates `.quorum/` exists
3. Optional directory scan with user confirmation

**Configuration Schema**:
```yaml
# ~/.config/quorum/projects.yaml
projects:
  - id: "project-alpha"
    path: "/home/user/projects/alpha"
    name: "Project Alpha"
    last_accessed: "2025-01-15T10:30:00Z"
  - id: "project-beta"
    path: "/home/user/projects/beta"
    name: "Beta Service"
```

### 9.5 Difficulty 5: Git Worktree Isolation

**Description**: Git worktrees are created relative to the repository root. With multi-project support, parallel workflows from different projects could have worktree conflicts.

**Current Implementation** (`internal/adapters/git/worktree.go`):
- Worktrees created under `{repo-root}/.quorum/worktrees/`
- Each workflow gets a dedicated worktree

**Risk**: If two projects share a parent git repository (monorepo), their worktrees could collide.

**Solution**:
1. Include project ID in worktree path: `.quorum/worktrees/{project-id}/{workflow-id}/`
2. Validate worktree paths don't overlap across projects
3. Add cleanup mechanism for orphaned worktrees

---

## 10. Architectural Alternatives

### 10.1 Alternative A: In-Process Multi-Project (Recommended)

```
+-------------------------------------------------------------------+
|                        Single quorum serve                         |
|                                                                    |
|  +------------------+  +------------------+  +-----------------+   |
|  | Project A        |  | Project B        |  | Project C       |   |
|  | StateManager     |  | StateManager     |  | StateManager    |   |
|  | /path/a/.quorum  |  | /path/b/.quorum  |  | /path/c/.quorum |   |
|  +------------------+  +------------------+  +-----------------+   |
|                                                                    |
|  +-------------------------------------------------------------+  |
|  |                    ProjectStatePool                          |  |
|  |  - Lazy loads StateManagers per project                      |  |
|  |  - Caches active managers (LRU eviction)                     |  |
|  |  - Thread-safe access                                        |  |
|  +-------------------------------------------------------------+  |
|                                                                    |
|  +-------------------------------------------------------------+  |
|  |                    EventBus (Project-Aware)                  |  |
|  |  - Events include ProjectID                                  |  |
|  |  - Subscriptions can filter by project                       |  |
|  +-------------------------------------------------------------+  |
|                                                                    |
|  +-------------------------------------------------------------+  |
|  |                    API Layer                                 |  |
|  |  GET /api/v1/projects                                        |  |
|  |  GET /api/v1/projects/{id}/workflows                         |  |
|  |  GET /api/v1/projects/{id}/config                            |  |
|  |  GET /api/v1/sse/events?project={id}                         |  |
|  +-------------------------------------------------------------+  |
|                                                                    |
+-------------------------------------------------------------------+
```

**Pros**:
- Single process, simpler deployment
- Shared resources (memory for common libs)
- Unified API
- Real-time project switching

**Cons**:
- Memory usage scales with active projects
- All projects share fate (process crash affects all)
- More complex codebase

**Estimated Changes**: ~6,500 lines

### 10.2 Alternative B: Multi-Instance with Proxy

```
+------------------+
|   Proxy Server   |  (nginx/caddy)
| :8080            |
+--------+---------+
         |
         | Routes by /projects/{id}
         |
   +-----+-----+-----+
   |           |           |
   v           v           v
+------+  +------+  +------+
| :8081|  | :8082|  | :8083|
| Proj |  | Proj |  | Proj |
|  A   |  |  B   |  |  C   |
+------+  +------+  +------+
```

**Pros**:
- Strong isolation between projects
- No changes to core Quorum code
- Independent scaling per project

**Cons**:
- Multiple processes to manage
- Resource duplication (N instances of Go runtime)
- Complex frontend (must track which backend to query)
- No unified SSE stream

**Estimated Changes**: ~1,000 lines (mostly frontend routing)

### 10.3 Alternative C: Database-Level Multi-Tenancy

```
Single SQLite Database with Project Partitioning

+----------------------------------+
|           state.db               |
+----------------------------------+
| projects                         |
|   id | name | path | created_at  |
+----------------------------------+
| workflows                        |
|   id | project_id | status | ... |
+----------------------------------+
| tasks                            |
|   id | workflow_id | project_id  |
+----------------------------------+
```

**Pros**:
- Centralized data management
- Efficient queries across projects
- Simpler backup/restore

**Cons**:
- Requires significant schema migration
- All projects in one file (corruption risk)
- Potential lock contention
- Doesn't match current per-project structure

**Estimated Changes**: ~4,000 lines

### 10.4 Recommendation

**Recommended: Alternative A (In-Process Multi-Project)**

Rationale:
1. **Maintains existing project structure** - Each project keeps its `.quorum/` directory
2. **Backward compatible** - Existing projects work without migration
3. **Unified UX** - Single interface, real-time updates across projects
4. **Reasonable complexity** - More changes than B, but cleaner architecture than C
5. **Efficient resource use** - Shared Go runtime, lazy-loaded project state

---

## 11. Implementation Plan

### Phase 1: Backend Foundation

**Objective**: Add project awareness to core backend components.

**Changes Required**:

1. **Create ProjectRegistry** (`internal/project/registry.go`)
   - Load/save `~/.config/quorum/projects.yaml`
   - Methods: `ListProjects()`, `AddProject()`, `RemoveProject()`, `ValidateProject()`

2. **Create ProjectStatePool** (`internal/adapters/state/pool.go`)
   - Map of `projectID → StateManager`
   - Lazy initialization
   - LRU eviction for memory management

3. **Extend Event Interface** (`internal/events/bus.go`)
   - Add `ProjectID() string` to Event interface
   - Update all event constructors
   - Modify EventBus to support filtered subscriptions

4. **Add Project Endpoints** (`internal/api/projects.go`)
   - `GET /api/v1/projects` - List registered projects
   - `POST /api/v1/projects` - Register a project
   - `DELETE /api/v1/projects/{id}` - Remove project registration

### Phase 2: API Layer Modifications

**Objective**: Scope all workflow operations by project.

**Changes Required**:

1. **Modify Server struct** (`internal/api/server.go`)
   - Replace single `stateManager` with `ProjectStatePool`
   - Add `projectRegistry` field

2. **Add Project Middleware** (`internal/api/middleware.go`)
   - Extract project ID from URL path or header
   - Attach to request context

3. **Update All Handlers** (`internal/api/workflows.go`, etc.)
   - Extract project context
   - Use `pool.ForProject(projectID)` to get StateManager

4. **Modify SSE Handler** (`internal/api/sse.go`)
   - Accept `?project=` query parameter
   - Filter events by project ID

### Phase 3: Frontend Integration

**Objective**: Add project selection and navigation to WebUI.

**Changes Required**:

1. **Create ProjectSelector Component**
   ```
   frontend/src/components/ProjectSelector.jsx
   - Dropdown in header (upper-right)
   - Shows current project name
   - Lists all registered projects
   - "Add Project" option
   ```

2. **Create projectStore**
   ```
   frontend/src/stores/projectStore.js
   - projects: []
   - currentProjectId: string
   - loadProjects()
   - selectProject(id)
   ```

3. **Modify Existing Stores**
   - Nest state by project ID
   - Add selectors for current project
   - Update all consumers

4. **Update API Client**
   - Include project ID in all requests
   - Modify SSE URL to include project filter

5. **Update Layout Component**
   - Integrate ProjectSelector in header
   - Show project indicator

### Phase 4: CLI Enhancement

**Objective**: Add project awareness to CLI commands.

**Changes Required**:

1. **Add Global --project Flag** (`cmd/quorum/cmd/root.go`)
   ```go
   rootCmd.PersistentFlags().String("project", "", "Project ID or path")
   ```

2. **Resolve Project Context** (`cmd/quorum/cmd/common.go`)
   - If `--project` provided, use specified path
   - Otherwise, use current directory
   - Validate `.quorum/` exists

3. **Add Project Commands**
   ```
   quorum project list    - List registered projects
   quorum project add     - Register current directory
   quorum project remove  - Unregister a project
   ```

### Phase 5: Testing and Migration

**Objective**: Ensure stability and backward compatibility.

**Changes Required**:

1. **Unit Tests**
   - ProjectStatePool behavior
   - Event filtering
   - API endpoint scoping

2. **Integration Tests**
   - Multi-project SSE streams
   - Concurrent project access
   - Git worktree isolation

3. **Migration Guide**
   - Document upgrading from single-project
   - Automatic project registration on first access

---

## 12. UX/UI Recommendations

### 12.1 Project Selector Component

**Location**: Upper-right corner of header (as specified in requirements)

```
+---------------------------------------------------------------+
|  [Logo] Quorum                          [Project: Alpha ▼]    |
+---------------------------------------------------------------+
|                                                               |
|  Workflows | Kanban | Chat | Settings                         |
|                                                               |
```

**Dropdown Behavior**:
```
+---------------------------+
| ✓ Project Alpha           |
|   Project Beta            |
|   Project Gamma           |
|---------------------------|
|   + Add Project...        |
|   ⚙ Manage Projects       |
+---------------------------+
```

### 12.2 Visual Indicators

1. **Color coding** - Each project gets a subtle accent color
2. **Breadcrumb** - Shows `Project > Workflows > wf-abc123`
3. **Active indicator** - Pulsing dot when project has running workflows

### 12.3 Project Switching Flow

```
User clicks Project Selector
        |
        v
Dropdown shows available projects
        |
        v
User selects different project
        |
        v
Frontend clears current project state
        |
        v
Frontend loads new project data
        |
        v
URL updates to /projects/{id}/...
        |
        v
SSE reconnects with new project filter
```

---

## 13. Security Considerations

### 13.1 Access Control Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Cross-project data access | HIGH | API middleware validates project ownership |
| Path traversal in project paths | HIGH | Validate paths, reject `..` segments |
| SSE event leakage | MEDIUM | Server-side project filtering |
| Sensitive paths exposed | MEDIUM | Hash or encrypt project paths in UI |

### 13.2 Recommended Security Measures

1. **Project Path Validation**
   ```go
   func validateProjectPath(path string) error {
       // Must be absolute
       if !filepath.IsAbs(path) {
           return errors.New("path must be absolute")
       }
       // No path traversal
       if strings.Contains(path, "..") {
           return errors.New("path traversal not allowed")
       }
       // .quorum directory must exist
       if _, err := os.Stat(filepath.Join(path, ".quorum")); err != nil {
           return errors.New(".quorum directory not found")
       }
       return nil
   }
   ```

2. **Project ID Generation**
   - Use cryptographically random IDs
   - Don't expose filesystem paths in API responses

3. **Request Scoping**
   - Every API request must include valid project context
   - Middleware rejects requests without project ID

---

## 14. Performance Considerations

### 14.1 Memory Impact

**Current (Single Project)**:
- ~50MB base memory
- +10MB per active workflow

**Multi-Project (Lazy Loading)**:
- ~50MB base memory
- +5MB per loaded project StateManager
- +10MB per active workflow across all projects

**Mitigation**:
- LRU eviction for inactive project StateManagers
- Configurable maximum concurrent projects
- Unload project state after inactivity timeout

### 14.2 Network Impact

**SSE Connections**:
- Current: 1 SSE connection per client
- Multi-project: Still 1 SSE connection, but filtered server-side

**API Calls**:
- Additional calls for project listing
- Same call patterns within a project

### 14.3 Database Impact

**Current**: One SQLite database per project
**Multi-project**: Still one database per project (unchanged)

This maintains existing performance characteristics per project while allowing the server to manage multiple databases.

---

## 15. Conclusion and Recommendation

### 15.1 Viability Assessment

| Criterion | Assessment |
|-----------|------------|
| **Technical Feasibility** | YES - Achievable with significant effort |
| **Architectural Fit** | MODERATE - Requires refactoring, not rewrite |
| **Backward Compatibility** | YES - Existing projects work unchanged |
| **Maintenance Impact** | HIGH - Increases codebase complexity |
| **User Value** | HIGH - Significant UX improvement for multi-project users |

### 15.2 Final Recommendation

**Proceed with In-Process Multi-Project (Alternative A)**, implemented in 5 phases as described.

**Critical Success Factors**:
1. Strong isolation between projects at API layer
2. Efficient lazy-loading of project state
3. Clear UX indicators for current project context
4. Comprehensive testing of multi-project scenarios

**Estimated Total Effort**:
- Backend: ~3,500 lines of code
- Frontend: ~2,500 lines of code
- Tests: ~2,000 lines of code
- Documentation: ~500 lines

**Risks to Monitor**:
1. Memory usage with many projects
2. SSE event throughput under load
3. Git worktree conflicts in monorepos
4. Migration issues for existing users

---

## Appendix A: Files Requiring Modification

### Backend (Go)

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/core/ports.go` | Extend | Add ProjectID to methods or create pool interface |
| `internal/core/workflow.go` | Extend | Add ProjectID field |
| `internal/core/state.go` | Extend | Add ProjectID to WorkflowState |
| `internal/adapters/state/sqlite.go` | Modify | Support project-scoped paths |
| `internal/adapters/state/json.go` | Modify | Support project-scoped paths |
| `internal/adapters/state/factory.go` | Extend | Create pool factory |
| `internal/events/bus.go` | Extend | Add project filtering |
| `internal/events/workflow.go` | Extend | Add ProjectID to events |
| `internal/api/server.go` | Modify | Use ProjectStatePool |
| `internal/api/sse.go` | Modify | Filter by project |
| `internal/api/workflows.go` | Modify | Scope by project |
| `cmd/quorum/cmd/root.go` | Extend | Add --project flag |
| `cmd/quorum/cmd/common.go` | Modify | Resolve project context |

### Frontend (JavaScript/React)

| File | Change Type | Description |
|------|-------------|-------------|
| `frontend/src/App.jsx` | Modify | Add project routes |
| `frontend/src/components/Layout.jsx` | Modify | Add ProjectSelector |
| `frontend/src/components/ProjectSelector.jsx` | NEW | Project dropdown |
| `frontend/src/stores/projectStore.js` | NEW | Project state management |
| `frontend/src/stores/workflowStore.js` | Modify | Nest by project |
| `frontend/src/stores/taskStore.js` | Modify | Nest by project |
| `frontend/src/stores/configStore.js` | Modify | Per-project config |
| `frontend/src/hooks/useSSE.js` | Modify | Include project filter |
| `frontend/src/lib/api.js` | Modify | Include project in requests |

---

## Appendix B: Database Schema Changes

### New Table: projects (if using central registry)

```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_accessed TIMESTAMP
);

CREATE INDEX idx_projects_path ON projects(path);
```

### Modified Table: workflows

```sql
-- No change to per-project database
-- Each project maintains its own state.db

-- For central tracking (optional):
ALTER TABLE workflows ADD COLUMN project_id TEXT;
CREATE INDEX idx_workflows_project ON workflows(project_id);
```

---

## Appendix C: API Endpoint Changes

### New Endpoints

```
GET    /api/v1/projects                    List registered projects
POST   /api/v1/projects                    Register new project
DELETE /api/v1/projects/{projectId}        Remove project registration
GET    /api/v1/projects/{projectId}        Get project details
```

### Modified Endpoints

```
# Existing endpoints gain project scoping:
GET    /api/v1/projects/{projectId}/workflows
POST   /api/v1/projects/{projectId}/workflows
GET    /api/v1/projects/{projectId}/workflows/{workflowId}
GET    /api/v1/projects/{projectId}/config
PUT    /api/v1/projects/{projectId}/config
GET    /api/v1/projects/{projectId}/sse/events
```

### Backward Compatibility

For backward compatibility, the original endpoints remain functional when only one project is registered or when a default project is configured:

```
GET /api/v1/workflows  →  Uses default/only project
```
