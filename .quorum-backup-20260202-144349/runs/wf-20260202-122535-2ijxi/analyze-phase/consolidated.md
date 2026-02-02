# Viability Analysis: Multi-Tenant/Multi-Project Management in Quorum AI

## Executive Summary

This document provides an exhaustive technical analysis of implementing multi-tenant/multi-project management from a single Quorum AI interface. The proposed feature would allow users to manage all initialized quorums on their machine from one UI instance, with a navigation mechanism (dropdown, tabs, or sidebar) in the upper-right corner for switching between projects.

**Core Finding**: The current Quorum AI architecture is fundamentally designed around a single-project paradigm. Every major component—from state management to API endpoints to frontend stores—assumes operation within a single project context determined by the current working directory. Implementing multi-project support is technically viable but requires significant refactoring across approximately 80 files, affecting backend state management, API endpoints, SSE event broadcasting, and frontend stores. The estimated implementation complexity is HIGH, requiring approximately 6,500 lines of new or modified code.

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
|                         root = os.Getwd()                             |
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

### 1.2 Project Initialization and Directory Structure

When a user runs `quorum init`, the following structure is created **relative to the current working directory**:

```
project-root/
├── .quorum/
│   ├── config.yaml          # Project-specific configuration
│   ├── state/
│   │   └── state.db         # SQLite database (or state.json for JSON backend)
│   ├── logs/
│   ├── chat/ or chat.db     # Chat session persistence
│   ├── attachments/         # File attachments
│   └── runs/
│       └── {workflow-id}/   # Per-workflow artifacts and reports
└── ...project files...
```

**Critical Code Reference** - The initialization logic (`cmd/quorum/cmd/init.go:31-36`):
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
    // ...creates subdirectories...
}
```

**Fundamental Implication**: Project identity is implicitly tied to the filesystem path. There is **no explicit `ProjectID` field** anywhere in the system—the project is defined entirely by the current working directory from which commands are executed.

### 1.3 Configuration Loading Chain

Configuration is loaded from multiple sources with the following precedence (`internal/config/loader.go:75-91`):

```
Priority Order (highest to lowest):
1. CLI flags (--config path)
2. Environment variables (QUORUM_*)
3. .quorum/config.yaml (project-specific, in cwd)
4. .quorum.yaml (legacy location, in cwd)
5. ~/.config/quorum/config.yaml (user-level defaults)
6. Hardcoded defaults
```

**Key Code References**:
- Project config search: `internal/config/loader.go:75` - looks for `.quorum/config.yaml`
- Legacy config: `internal/config/loader.go:76` - looks for `.quorum.yaml`
- User config: `internal/config/loader.go:81` - looks in `~/.config/quorum`
- Default state path: `internal/config/loader.go:86-88` - `.quorum/state/state.json`

The loader searches for configuration relative to `cwd`, reinforcing the single-project design where the working directory determines the project context.

---

## 2. State Management Architecture

### 2.1 StateManager Interface Definition

The core state management interface is defined in `internal/core/ports.go:140-239`. This interface governs all workflow state persistence operations:

```go
type StateManager interface {
    // Workflow lifecycle
    Save(ctx context.Context, state *WorkflowState) error
    Load(ctx context.Context) (*WorkflowState, error)
    LoadByID(ctx context.Context, id WorkflowID) (*WorkflowState, error)
    ListWorkflows(ctx context.Context) ([]WorkflowSummary, error)
    DeleteWorkflow(ctx context.Context, id WorkflowID) error

    // Active workflow tracking (SINGLE ACTIVE per project)
    GetActiveWorkflowID(ctx context.Context) (WorkflowID, error)
    SetActiveWorkflowID(ctx context.Context, id WorkflowID) error
    DeactivateWorkflow(ctx context.Context) error

    // Lock management
    AcquireLock(ctx context.Context) error
    ReleaseLock(ctx context.Context) error
    AcquireWorkflowLock(ctx context.Context, workflowID WorkflowID) error
    ReleaseWorkflowLock(ctx context.Context, workflowID WorkflowID) error
    RefreshWorkflowLock(ctx context.Context, workflowID WorkflowID) error

    // Running workflow tracking
    SetWorkflowRunning(ctx context.Context, workflowID WorkflowID) error
    ClearWorkflowRunning(ctx context.Context, workflowID WorkflowID) error
    ListRunningWorkflows(ctx context.Context) ([]WorkflowID, error)
    IsWorkflowRunning(ctx context.Context, workflowID WorkflowID) (bool, error)
    UpdateWorkflowHeartbeat(ctx context.Context, workflowID WorkflowID) error

    // Zombie detection
    FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*WorkflowState, error)

    // Duplicate detection
    FindWorkflowsByPrompt(ctx context.Context, prompt string) ([]DuplicateWorkflowInfo, error)

    // Transaction support
    ExecuteAtomically(ctx context.Context, fn func(AtomicStateContext) error) error

    // Backup and restore
    Backup(ctx context.Context) error
    Restore(ctx context.Context) (*WorkflowState, error)
    Exists() bool
}
```

**Single-Project Assumptions Embedded in Interface**:
1. `GetActiveWorkflowID()` returns ONE active workflow globally—there is no project scoping
2. All methods operate on a single database at a fixed path
3. **No `ProjectID` parameter exists in any method signature**
4. Lock management assumes a single storage backend
5. Zombie and duplicate detection operate within a single project context

### 2.2 SQLite Implementation Details

The SQLite state manager (`internal/adapters/state/sqlite.go:54-66`) maintains the following structure:

```go
type SQLiteStateManager struct {
    dbPath        string        // Fixed path: .quorum/state/state.db
    backupPath    string        // dbPath + ".bak"
    lockPath      string        // dbPath + ".lock"
    lockTTL       time.Duration // Default: 1 hour
    db            *sql.DB       // Write connection (max 1 concurrent)
    readDB        *sql.DB       // Read-only connection (max 10 concurrent)
    mu            sync.RWMutex
    maxRetries    int
    baseRetryWait time.Duration
}
```

**Database Configuration** (`internal/adapters/state/sqlite.go:91-116`):
- Write connection: Uses WAL mode with `busy_timeout=5000ms`, max 1 open connection
- Read connection: Separate read-only connection pool with max 10 connections
- Foreign keys enabled, connection lifetime unlimited for writes

**Database Schema** (from `internal/adapters/state/migrations/001_initial_schema.sql`):
```sql
-- workflows table - stores WorkflowState JSON
CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- active_workflow table - singleton for "current" workflow
CREATE TABLE IF NOT EXISTS active_workflow (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    workflow_id TEXT,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id)
);

-- tasks table - stores TaskState per workflow
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    data BLOB NOT NULL,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id)
);

-- checkpoints table - stores checkpoint data
-- schema_migrations table - migration tracking
```

**Critical Finding**: **No `project_id` column exists in any table.** The `active_workflow` table is explicitly designed as a singleton (enforced by `CHECK (id = 1)`), meaning only ONE workflow can be "active" per database, and therefore per project.

### 2.3 JSON State Manager Implementation

The JSON state manager (`internal/adapters/state/json.go:41-45`) uses a simpler file-based approach:

```go
type JSONStateManager struct {
    basePath     string  // .quorum/state/
    activePath   string  // .quorum/state/active.json - single active workflow ID
    mu           sync.RWMutex
    maxBackups   int
}
```

The `active.json` file stores a single workflow ID, confirming the single-active-workflow design.

### 2.4 State Manager Instantiation

The state manager is created once during server startup (`cmd/quorum/cmd/serve.go:97-102`):

```go
statePath := cfg.State.Path
if statePath == "" {
    statePath = ".quorum/state/state.json"  // Default path
}
stateManager, err := state.NewStateManager(statePath, cfg.State.Backend)
```

The server holds exactly **one StateManager instance** that operates on one database path. This is a fundamental architectural constraint for multi-project support.

---

## 3. WebUI Architecture

### 3.1 Frontend Component Structure

```
frontend/src/
├── App.jsx                  # Main routing (NO project context)
├── components/
│   ├── Layout.jsx           # Main layout with sidebar and header
│   ├── Breadcrumbs.jsx      # Navigation breadcrumbs
│   ├── MobileBottomNav.jsx  # Mobile navigation
│   └── ...
├── pages/
│   ├── Dashboard.jsx
│   ├── Workflows.jsx
│   ├── Kanban.jsx
│   ├── Chat.jsx
│   └── Settings.jsx
├── stores/
│   ├── workflowStore.js     # Workflow state (GLOBAL singleton)
│   ├── taskStore.js         # Task state (GLOBAL singleton)
│   ├── agentStore.js        # Agent events (GLOBAL singleton)
│   ├── configStore.js       # Configuration (GLOBAL singleton)
│   ├── uiStore.js           # UI state
│   └── kanbanStore.js       # Kanban state
├── hooks/
│   └── useSSE.js            # SSE connection (SINGLE endpoint)
└── lib/
    ├── api.js               # API client (FIXED base URL)
    └── configApi.js         # Config API client
```

### 3.2 Routing Structure

Current routing in `frontend/src/App.jsx:31-38`:

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

**Critical Observation**: No `/projects/:projectId/...` prefix exists in any route. All routes assume a single-project context. For multi-project support, routes would need to be restructured as:
```
/projects/:projectId/workflows
/projects/:projectId/kanban
/projects/:projectId/chat
etc.
```

### 3.3 API Client Configuration

The API client (`frontend/src/lib/api.js:1-4`) uses a fixed base URL:

```javascript
const API_BASE = '/api/v1';

export const workflowApi = {
  list: () => fetch(`${API_BASE}/workflows`).then(r => r.json()),
  get: (id) => fetch(`${API_BASE}/workflows/${id}`).then(r => r.json()),
  // ...all endpoints use fixed API_BASE
};
```

For multi-project support, the API base would need to be dynamic:
```javascript
const getApiBase = (projectId) => `/api/v1/projects/${projectId}`;
```

### 3.4 Zustand Store Architecture

**workflowStore.js** (`frontend/src/stores/workflowStore.js:6-14`):
```javascript
const useWorkflowStore = create((set, get) => ({
  workflows: [],           // All workflows - NO project filtering
  activeWorkflow: null,    // SINGLE active workflow globally
  selectedWorkflowId: null,
  tasks: {},
  loading: false,
  error: null,
  // ... methods operate on global state
}));
```

**configStore.js** - Single configuration object for the entire application:
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

**Key Finding**: All Zustand stores are global singletons. They hold state for "the" project, not "a" project. Switching projects would require either:
1. Resetting all stores completely
2. Restructuring stores to hold per-project state maps
3. Creating separate store instances per project

### 3.5 SSE Event Handling

The SSE connection (`frontend/src/hooks/useSSE.js:9-12`) connects to a fixed endpoint:

```javascript
const SSE_URL = '/api/v1/sse/events';
const RECONNECT_DELAY = 3000;
const MAX_RECONNECT_ATTEMPTS = 10;
const POLLING_INTERVAL = 5000;
```

The event handler processes ALL events without project filtering (`useSSE.js:42-70`):

```javascript
// Workflow event handlers
const handleWorkflowStarted = useWorkflowStore(state => state.handleWorkflowStarted);
const handleWorkflowStateUpdated = useWorkflowStore(state => state.handleWorkflowStateUpdated);
const handleWorkflowCompleted = useWorkflowStore(state => state.handleWorkflowCompleted);
// ... all handlers update global state
```

For multi-project support, SSE would need to either:
- Connect with a project filter: `/api/v1/sse/events?project={projectId}`
- Filter events client-side by project ID
- Maintain multiple SSE connections (one per active project)

### 3.6 Header Layout and Project Selector Placement

The Layout component (`frontend/src/components/Layout.jsx:287-289`) has an existing slot in the header for additional actions:

```jsx
<div className="flex items-center gap-3">
  {/* Additional header actions can go here */}
</div>
```

This is the **ideal location** for a project selector dropdown, as specified in the requirements.

---

## 4. Backend API Architecture

### 4.1 Server Structure

The API server (`internal/api/server.go:27-55`) holds a single instance of all critical components:

```go
type Server struct {
    router          chi.Router
    stateManager    core.StateManager    // SINGLE state manager
    chatStore       core.ChatStore
    eventBus        *events.EventBus     // GLOBAL event bus
    agentRegistry   core.AgentRegistry
    logger          *slog.Logger
    chatHandler     *webadapters.ChatHandler
    attachments     *attachments.Store
    resourceMonitor *diagnostics.ResourceMonitor
    configLoader    *config.Loader
    root            string               // SINGLE root directory

    unifiedTracker *UnifiedTracker      // SINGLE workflow tracker
    executor       *WorkflowExecutor    // SINGLE executor
    heartbeat      *workflow.HeartbeatManager
    kanbanEngine   *kanban.Engine
    configMu       sync.RWMutex
}
```

**Server Initialization** (`internal/api/server.go:131-150`):
```go
func NewServer(stateManager core.StateManager, eventBus *events.EventBus, opts ...ServerOption) *Server {
    wd, _ := os.Getwd() // Root defaults to current working directory
    s := &Server{
        stateManager: stateManager,
        eventBus:     eventBus,
        logger:       slog.Default(),
        root:         wd,  // FIXED root
    }
    // ...
    s.attachments = attachments.NewStore(s.root)
    s.chatHandler = webadapters.NewChatHandler(s.agentRegistry, eventBus, s.attachments, s.chatStore)
    s.router = s.setupRouter()
    return s
}
```

**Critical Issue**: The server has ONE `StateManager`, ONE `EventBus`, and ONE `root` directory. Multi-project would require either:
- A pool of StateManagers (one per project)
- A modified StateManager that accepts ProjectID on all operations
- Multiple server instances (one per project) behind a proxy

### 4.2 Current API Endpoints

The routing structure (`internal/api/server.go:185-200`):

```
GET    /api/v1/workflows/           List all workflows
GET    /api/v1/workflows/{id}       Get specific workflow
GET    /api/v1/workflows/active     Get active workflow
POST   /api/v1/workflows/           Create workflow
PUT    /api/v1/workflows/{id}       Update workflow
DELETE /api/v1/workflows/{id}       Delete workflow
POST   /api/v1/workflows/{id}/run   Start workflow
POST   /api/v1/workflows/{id}/pause Pause workflow
POST   /api/v1/workflows/{id}/cancel Cancel workflow
// ... additional workflow endpoints

GET    /api/v1/config               Get configuration
PUT    /api/v1/config               Update configuration

GET    /api/v1/sse/events           SSE event stream

// Chat, files, diagnostics endpoints...
```

**No project scoping exists**. For multi-project support, the API would need restructuring:
```
GET /api/v1/projects                           List registered projects
POST /api/v1/projects                          Register new project
DELETE /api/v1/projects/{projectId}            Remove project

GET /api/v1/projects/{projectId}/workflows     Project-scoped workflows
POST /api/v1/projects/{projectId}/workflows    Create in project
GET /api/v1/projects/{projectId}/config        Project configuration
GET /api/v1/projects/{projectId}/sse/events    Project-filtered SSE
```

### 4.3 SSE Handler Implementation

The SSE handler (`internal/api/sse.go:18-64`) broadcasts **all events** to **all connected clients**:

```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    flusher, ok := w.(http.Flusher)
    if !ok {
        respondError(w, http.StatusInternalServerError, "streaming not supported")
        return
    }

    // Subscribe to ALL events - NO PROJECT FILTERING
    eventCh := s.eventBus.Subscribe()

    ctx := r.Context()
    s.logger.Info("SSE client connected", "remote_addr", r.RemoteAddr)

    // Stream ALL events to client
    for {
        select {
        case <-ctx.Done():
            s.logger.Info("SSE client disconnected", "remote_addr", r.RemoteAddr)
            return
        case event, ok := <-eventCh:
            if !ok {
                return
            }
            s.sendEventToClient(w, flusher, event)  // NO FILTERING
        }
    }
}
```

**Problem**: In a multi-project scenario, every connected client would receive events from ALL projects. This is both a performance issue (unnecessary event traffic) and a potential security/privacy concern.

### 4.4 EventBus Architecture

The EventBus (`internal/events/bus.go:11-65`) implements simple pub/sub without project awareness:

```go
// Event is the base interface for all events.
type Event interface {
    EventType() string
    Timestamp() time.Time
    WorkflowID() string  // Has WorkflowID, but NO ProjectID
}

// BaseEvent provides common fields for all events.
type BaseEvent struct {
    Type     string    `json:"type"`
    Time     time.Time `json:"timestamp"`
    Workflow string    `json:"workflow_id"`
    // NO project_id field
}

type EventBus struct {
    mu           sync.RWMutex
    subscribers  []*Subscriber
    prioritySubs []*Subscriber
    bufferSize   int
    droppedCount int64
    closed       bool
}

func (eb *EventBus) Subscribe(types ...string) <-chan Event {
    // Creates subscription that receives ALL events (optionally filtered by type)
    // NO project filtering capability
}
```

**Current Event Flow**:
```
Workflow Event → EventBus.Publish() → ALL Subscribers → ALL SSE Clients
```

**Required Flow for Multi-Project**:
```
Workflow Event (with ProjectID) → EventBus.Publish() → Project Filter → Matching Clients Only
```

### 4.5 File Operations and Path Validation

The file API (`internal/api/files.go:265-285`) validates paths against a single root:

```go
func (s *Server) resolvePath(requestedPath string) (string, error) {
    // Resolve to absolute path
    absPath := filepath.Join(s.root, requestedPath)

    // Ensure resolved path is under root to prevent path traversal
    if !strings.HasPrefix(absPath, s.root) {
        return "", fmt.Errorf("path outside project root")
    }

    return absPath, nil
}
```

The chat handler (`internal/adapters/web/chat.go:786-813`) similarly restricts file access to `projectRoot`.

**Implication**: Multi-project would require dynamic root resolution per request, with strict validation to prevent cross-project file access.

---

## 5. CLI and TUI Analysis

### 5.1 CLI Single-Project Assumptions

The CLI extensively uses `os.Getwd()` to determine project context. Files using `os.Getwd()` include:

| File | Line | Purpose |
|------|------|---------|
| `cmd/quorum/cmd/init.go` | 31 | Project initialization |
| `cmd/quorum/cmd/common.go` | 238 | Worktree manager creation |
| `cmd/quorum/cmd/run.go` | varies | Workflow execution |
| `cmd/quorum/cmd/chat.go` | varies | Chat session |
| `cmd/quorum/cmd/serve.go` | 97-102 | Server startup |
| `internal/service/workflow/runner.go` | varies | Workflow runner |
| `internal/api/server.go` | 132 | API server root |
| Plus 10+ additional locations | | |

**CLI Global Flags** (`cmd/quorum/cmd/root.go:56-81`):
```go
rootCmd.PersistentFlags().String("config", "", "config file (default .quorum/config.yaml)")
rootCmd.PersistentFlags().String("log-level", "info", "log level")
rootCmd.PersistentFlags().String("log-format", "text", "log format (text or json)")
```

**Missing**: A `--project` or `--project-root` flag to specify an alternative project directory.

### 5.2 TUI Constraints

The TUI file explorer (`internal/tui/chat/explorer.go:61-66`) is anchored to the current working directory:

```go
func NewExplorer(root string) *Explorer {
    if root == "" {
        root, _ = os.Getwd()
    }
    return &Explorer{
        root:    root,  // Navigation cannot go above this
        current: root,
    }
}
```

The file viewer (`internal/tui/chat/file_viewer.go:47-56`) rejects paths outside the root:
```go
if !strings.HasPrefix(absPath, e.root) {
    return nil, fmt.Errorf("path outside project root")
}
```

### 5.3 Multi-Project Viability for CLI/TUI

**CLI**: Could support multi-project via a `--project-root` flag, but this would require:
- Modifying all commands to accept and propagate project context
- Updating config loading to use specified root
- Adjusting state manager creation to use project-specific paths

**TUI**: More challenging due to the interactive, stateful nature:
- Would need a project selection screen at startup
- Or a command to switch projects mid-session (requiring state reset)
- File navigation would need to be scoped to selected project

**Recommendation**: For initial implementation, limit multi-project to WebUI only, maintaining CLI/TUI as single-project interfaces that operate on the current directory.

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

The `WorkflowState` struct (`internal/core/state.go`) similarly lacks project context:

```go
type WorkflowState struct {
    Version       int
    WorkflowID    WorkflowID
    Title         string
    Status        WorkflowStatus
    CurrentPhase  Phase
    Prompt        string
    // ... many fields for execution state
    // NO ProjectID field
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
    // ... metrics, timestamps, agent events, tasks
    // NO project_id field in response
}
```

**Impact**: Adding `ProjectID` would require:
1. Modifying core domain models
2. Updating all persistence code
3. Adding to API response models
4. Frontend changes to handle project context

---

## 7. Identified Single-Project Coupling Points

### 7.1 Comprehensive Coupling Summary

| Component | File:Line | Coupling Type | Impact Level |
|-----------|-----------|---------------|--------------|
| Project Init | `cmd/quorum/cmd/init.go:31` | `os.Getwd()` determines project | Critical |
| State Path | `cmd/quorum/cmd/serve.go:97-102` | Hardcoded `.quorum/state/` | Critical |
| SQLite Manager | `internal/adapters/state/sqlite.go:72-133` | Fixed `dbPath` | Critical |
| Active Workflow | `internal/core/ports.go:157-162` | Single `GetActiveWorkflowID()` | High |
| Config Loader | `internal/config/loader.go:75-91` | CWD-relative search | High |
| API Server | `internal/api/server.go:132-137` | Single StateManager, root | Critical |
| SSE Handler | `internal/api/sse.go:33-34` | Broadcasts all events | High |
| EventBus | `internal/events/bus.go:11-16` | No ProjectID in Event | High |
| Frontend Routes | `frontend/src/App.jsx:31-38` | No `/projects/` prefix | High |
| Workflow Store | `frontend/src/stores/workflowStore.js:6-14` | Global `workflows[]` | High |
| Config Store | `frontend/src/stores/configStore.js` | Single `config` object | Medium |
| SSE URL | `frontend/src/hooks/useSSE.js:9` | Fixed `/api/v1/sse/events` | High |
| API Base | `frontend/src/lib/api.js:1` | Fixed `/api/v1` | High |
| File API | `internal/api/files.go:265` | Single root validation | High |
| Chat Handler | `internal/adapters/web/chat.go:222-223` | projectRoot = cwd | High |
| Report Writer | `internal/service/report/writer.go:24` | `.quorum/runs` path | Medium |
| Attachments | `internal/attachments/store.go:38-39` | `<root>/.quorum/attachments` | Medium |
| Chat Store | `cmd/quorum/cmd/serve.go:117-122` | `.quorum/chat` path | Medium |
| TUI Explorer | `internal/tui/chat/explorer.go:61-66` | Root = cwd | Medium |
| Worktree Manager | `cmd/quorum/cmd/common.go:238-243` | Git client uses cwd | Medium |

### 7.2 Impact Analysis by Layer

```
+------------------------+--------+------------------+------------------+
| Affected Layer         | Files  | Estimated Effort | Risk Level       |
+------------------------+--------+------------------+------------------+
| Core Domain Models     |   5    |   Medium         | Low              |
| State Management       |   8    |   High           | High             |
| API Layer              |  12    |   High           | Medium           |
| EventBus/SSE           |   4    |   Medium         | High             |
| Frontend Stores        |   6    |   High           | High             |
| Frontend Components    |  15    |   Medium         | Medium           |
| CLI Commands           |  10    |   Medium         | Low              |
| Tests                  |  20+   |   High           | Medium           |
+------------------------+--------+------------------+------------------+
| TOTAL                  | ~80    |   Very High      | High             |
+------------------------+--------+------------------+------------------+
```

---

## 8. Pros and Cons Matrix

### 8.1 Benefits Analysis

| Benefit | User Type | Value | Technical Rationale |
|---------|-----------|-------|---------------------|
| Unified dashboard | All users | HIGH | View and manage all projects from one interface; no need to run separate servers |
| Cross-project metrics | Power users | MEDIUM | Aggregate cost tracking, token usage, workflow completion rates across projects |
| Context switching | Developers | HIGH | Instantly switch between projects without restarting servers or changing directories |
| Reduced server instances | Operations | MEDIUM | Single `quorum serve` process for all projects; lower memory footprint |
| Workflow comparison | Teams | MEDIUM | Compare approaches, prompts, and results across different projects |
| Centralized configuration | All users | MEDIUM | Global settings with project-specific overrides from one location |

### 8.2 Risks Analysis

| Risk | Probability | Impact | Evidence/Location | Mitigation Strategy |
|------|-------------|--------|-------------------|---------------------|
| Data leakage between projects | MEDIUM | HIGH | No auth middleware (`server.go:163`), global EventBus (`bus.go:45`) | Strict isolation at API layer, project-scoped SSE |
| Increased memory usage | HIGH | MEDIUM | Each StateManager holds DB connections (`sqlite.go:59-62`) | Lazy-load project state, LRU eviction |
| SSE event floods | MEDIUM | MEDIUM | Subscribe to ALL events (`sse.go:34`) | Server-side project filtering |
| Config confusion | LOW | MEDIUM | Single config store (`configStore.js`) | Clear project indicators in UI |
| Git worktree conflicts | MEDIUM | HIGH | Worktrees under `.quorum/worktrees/` | Project-prefixed worktree paths |
| Migration complexity | HIGH | MEDIUM | Schema lacks project_id (`001_initial_schema.sql`) | Use separate DB per project (no schema migration) |
| Performance degradation | MEDIUM | MEDIUM | Single event loop for all projects | Benchmark testing, connection pooling |
| State corruption on crash | LOW | HIGH | Multiple DB connections per server | Transaction boundaries, graceful shutdown |

### 8.3 Implementation Complexity Analysis

| Aspect | Complexity | Lines of Code | Risk | Dependencies |
|--------|------------|---------------|------|--------------|
| Add ProjectID to core models | Low | ~500 | Low | None |
| Create ProjectRegistry | Medium | ~600 | Low | Config system |
| StateManager pool | High | ~1,500 | Medium | StateManager interface |
| API project scoping | Medium | ~1,000 | Medium | Router, middleware |
| SSE event filtering | Medium | ~400 | Low | EventBus |
| Frontend stores restructure | High | ~2,000 | High | All components |
| Project discovery mechanism | Medium | ~800 | Medium | Filesystem access |
| CLI --project flag | Low | ~300 | Low | Config loading |
| Project selector UI component | Medium | ~400 | Low | Layout component |
| **TOTAL** | **Very High** | **~7,500** | **High** | - |

---

## 9. Main Technical Difficulties

### 9.1 Difficulty 1: State Manager Architecture Refactoring

**Technical Description**: The `StateManager` interface and all implementations assume a single database path. Multi-project requires either modifying the interface to accept project context or implementing a manager pool.

**Affected Files**:
- `internal/core/ports.go:140-239` - Interface definition
- `internal/adapters/state/sqlite.go` - SQLite implementation
- `internal/adapters/state/json.go` - JSON implementation
- `internal/adapters/state/factory.go` - Factory function
- All callers of StateManager methods (~30 locations)

**Solution A: Interface Modification**
```go
// Modified interface - all methods receive projectID
type StateManager interface {
    Save(ctx context.Context, projectID string, state *WorkflowState) error
    Load(ctx context.Context, projectID string) (*WorkflowState, error)
    LoadByID(ctx context.Context, projectID string, id WorkflowID) (*WorkflowState, error)
    // ... all methods get projectID parameter
}
```

| Aspect | Assessment |
|--------|------------|
| Pros | Clean API, explicit project context everywhere |
| Cons | Breaking change to every caller (~30 locations), extensive refactoring |
| Effort | High |
| Risk | Medium (requires careful migration) |

**Solution B: Manager Pool (Recommended)**
```go
type ProjectStatePool struct {
    managers    map[string]StateManager  // projectID -> manager
    mu          sync.RWMutex
    maxManagers int                      // LRU eviction threshold
    factory     StateManagerFactory
}

func (p *ProjectStatePool) ForProject(ctx context.Context, projectID string) (StateManager, error) {
    p.mu.RLock()
    if m, ok := p.managers[projectID]; ok {
        p.mu.RUnlock()
        return m, nil
    }
    p.mu.RUnlock()

    p.mu.Lock()
    defer p.mu.Unlock()

    // Double-check after acquiring write lock
    if m, ok := p.managers[projectID]; ok {
        return m, nil
    }

    // Create new manager for project
    projectPath := p.resolveProjectPath(projectID)
    m, err := p.factory.Create(filepath.Join(projectPath, ".quorum", "state", "state.db"))
    if err != nil {
        return nil, fmt.Errorf("creating state manager for project %s: %w", projectID, err)
    }

    p.managers[projectID] = m
    p.evictIfNeeded()  // LRU eviction
    return m, nil
}
```

| Aspect | Assessment |
|--------|------------|
| Pros | Maintains interface compatibility, lazy loading, memory management |
| Cons | Higher runtime complexity, connection pooling overhead |
| Effort | Medium |
| Risk | Low (additive change) |

**Recommendation**: Solution B (Manager Pool) - maintains backward compatibility, allows lazy loading, and provides memory management through LRU eviction.

### 9.2 Difficulty 2: SSE Event Isolation

**Technical Description**: The EventBus broadcasts all events to all subscribers. Connected SSE clients receive events from all projects, causing unnecessary traffic and potential information leakage.

**Affected Files**:
- `internal/events/bus.go:11-65` - EventBus implementation
- `internal/events/workflow.go` - Event definitions
- `internal/api/sse.go:18-64` - SSE handler
- `frontend/src/hooks/useSSE.js:9-100` - Frontend SSE handling

**Current Event Flow**:
```
+------------------+     +------------------+     +------------------+
| Workflow A Event |---->|    EventBus      |---->| ALL SSE Clients  |
+------------------+     | (No filtering)   |     +------------------+
                         +------------------+
+------------------+           |
| Workflow B Event |---------->|
+------------------+
```

**Required Event Flow**:
```
+------------------+     +------------------+     +------------------+
| Workflow A Event |---->|    EventBus      |---->| Client A (Proj A)|
| ProjectID: A     |     | Filter by        |     +------------------+
+------------------+     | ProjectID        |
                         +------------------+
+------------------+           |              +------------------+
| Workflow B Event |---------->+------------->| Client B (Proj B)|
| ProjectID: B     |                          +------------------+
+------------------+
```

**Solution**:

1. **Extend Event Interface** (`internal/events/bus.go`):
```go
type Event interface {
    EventType() string
    Timestamp() time.Time
    WorkflowID() string
    ProjectID() string  // NEW: Required for filtering
}

type BaseEvent struct {
    Type     string    `json:"type"`
    Time     time.Time `json:"timestamp"`
    Workflow string    `json:"workflow_id"`
    Project  string    `json:"project_id"`  // NEW
}

func (e BaseEvent) ProjectID() string { return e.Project }
```

2. **Add Project-Filtered Subscription** (`internal/events/bus.go`):
```go
func (eb *EventBus) SubscribeForProject(projectID string, types ...string) <-chan Event {
    eb.mu.Lock()
    defer eb.mu.Unlock()

    sub := &Subscriber{
        ch:        make(chan Event, eb.bufferSize),
        types:     make(map[string]bool),
        projectID: projectID,  // NEW: Filter criteria
        priority:  false,
    }
    for _, t := range types {
        sub.types[t] = true
    }
    eb.subscribers = append(eb.subscribers, sub)
    return sub.ch
}

func (eb *EventBus) Publish(event Event) {
    eb.mu.RLock()
    defer eb.mu.RUnlock()

    for _, sub := range eb.subscribers {
        // Filter by project if subscription has projectID
        if sub.projectID != "" && event.ProjectID() != sub.projectID {
            continue
        }
        // Type filtering...
        select {
        case sub.ch <- event:
        default:
            atomic.AddInt64(&eb.droppedCount, 1)
        }
    }
}
```

3. **Modify SSE Handler** (`internal/api/sse.go`):
```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    // ... header setup ...

    projectID := r.URL.Query().Get("project")

    var eventCh <-chan Event
    if projectID != "" {
        eventCh = s.eventBus.SubscribeForProject(projectID)
    } else {
        eventCh = s.eventBus.Subscribe()  // Legacy: all events
    }

    // ... stream events ...
}
```

4. **Update Frontend SSE Connection** (`frontend/src/hooks/useSSE.js`):
```javascript
const getSSEUrl = (projectId) => {
    if (projectId) {
        return `/api/v1/sse/events?project=${projectId}`;
    }
    return '/api/v1/sse/events';
};

// In connect function:
const url = getSSEUrl(currentProjectId);
eventSourceRef.current = new EventSource(url);
```

**Estimated Changes**: ~400 lines across 4 files.

### 9.3 Difficulty 3: Frontend Store Architecture Transformation

**Technical Description**: All Zustand stores are global singletons holding state for "the" project. Switching projects requires either complete state reset or restructuring to hold per-project state.

**Affected Files**:
- `frontend/src/stores/workflowStore.js`
- `frontend/src/stores/taskStore.js`
- `frontend/src/stores/agentStore.js`
- `frontend/src/stores/configStore.js`
- `frontend/src/stores/kanbanStore.js`
- All components consuming these stores

**Current Architecture**:
```
+-------------------+
|  Global Store     |
|  workflows: []    |  <-- Single array for all workflows
|  activeWorkflow   |  <-- Single active workflow
|  config: {}       |  <-- Single config
+-------------------+
        |
        v
+-------------------+
|  All Components   |  <-- All access same global state
+-------------------+
```

**Solution A: Per-Project Store Instances**
```javascript
// stores/projectStoreManager.js
const workflowStores = new Map();
const taskStores = new Map();
const configStores = new Map();

export function getWorkflowStore(projectId) {
    if (!workflowStores.has(projectId)) {
        workflowStores.set(projectId, createWorkflowStore(projectId));
    }
    return workflowStores.get(projectId);
}

function createWorkflowStore(projectId) {
    return create((set, get) => ({
        projectId,
        workflows: [],
        activeWorkflow: null,
        // ... all existing methods, but scoped to projectId
    }));
}
```

| Aspect | Assessment |
|--------|------------|
| Pros | True isolation between projects, familiar store pattern |
| Cons | Complex store management, subscription handling per project |
| Effort | High |

**Solution B: Nested State by Project (Recommended)**
```javascript
// stores/workflowStore.js
const useWorkflowStore = create((set, get) => ({
    projects: {},  // { [projectId]: { workflows: [], activeWorkflow: null, ... } }
    currentProjectId: null,

    setCurrentProject: (projectId) => {
        set({ currentProjectId: projectId });
        // Initialize project state if needed
        if (!get().projects[projectId]) {
            set(state => ({
                projects: {
                    ...state.projects,
                    [projectId]: { workflows: [], activeWorkflow: null, loading: false, error: null }
                }
            }));
        }
    },

    // Selectors for current project
    getCurrentProjectState: () => {
        const { projects, currentProjectId } = get();
        return projects[currentProjectId] || { workflows: [], activeWorkflow: null };
    },

    // Actions operate on current project
    fetchWorkflows: async () => {
        const { currentProjectId } = get();
        if (!currentProjectId) return;

        set(state => ({
            projects: {
                ...state.projects,
                [currentProjectId]: { ...state.projects[currentProjectId], loading: true }
            }
        }));

        try {
            const workflows = await workflowApi.listForProject(currentProjectId);
            set(state => ({
                projects: {
                    ...state.projects,
                    [currentProjectId]: {
                        ...state.projects[currentProjectId],
                        workflows,
                        loading: false
                    }
                }
            }));
        } catch (error) {
            set(state => ({
                projects: {
                    ...state.projects,
                    [currentProjectId]: {
                        ...state.projects[currentProjectId],
                        error: error.message,
                        loading: false
                    }
                }
            }));
        }
    },

    // Event handlers update correct project
    handleWorkflowStarted: (data) => {
        const projectId = data.project_id;
        set(state => ({
            projects: {
                ...state.projects,
                [projectId]: {
                    ...state.projects[projectId],
                    workflows: [...(state.projects[projectId]?.workflows || []), data.workflow]
                }
            }
        }));
    },
}));

// Custom hooks for easier component access
export const useCurrentWorkflows = () => useWorkflowStore(state =>
    state.projects[state.currentProjectId]?.workflows || []
);

export const useActiveWorkflow = () => useWorkflowStore(state =>
    state.projects[state.currentProjectId]?.activeWorkflow
);
```

| Aspect | Assessment |
|--------|------------|
| Pros | Single store, familiar Zustand patterns, automatic state persistence |
| Cons | Slightly more complex selectors, potential memory growth |
| Effort | Medium |

**Recommendation**: Solution B (Nested State) - simpler implementation, maintains single store subscription pattern, easier debugging.

### 9.4 Difficulty 4: Project Discovery and Registration

**Technical Description**: No mechanism exists to discover initialized Quorum projects on the system. Users need a way to add projects to the multi-project interface.

**Options Analysis**:

| Approach | Description | Pros | Cons |
|----------|-------------|------|------|
| **Directory Scanning** | Scan filesystem for `.quorum/` directories | Automatic discovery | Slow, permission issues, network mounts problematic |
| **Registry File** | Maintain `~/.config/quorum/projects.yaml` | Fast lookup, explicit control | Manual management required |
| **User Configuration** | UI for adding/removing projects | Full user control | Requires manual entry |
| **Hybrid (Recommended)** | Registry + optional scan + UI management | Best of all approaches | Most complex to implement |

**Recommended Implementation - Hybrid Approach**:

1. **Project Registry Schema** (`~/.config/quorum/projects.yaml`):
```yaml
version: 1
projects:
  - id: "proj-abc123"
    path: "/home/user/projects/alpha"
    name: "Project Alpha"
    last_accessed: "2025-01-15T10:30:00Z"
    status: "healthy"
  - id: "proj-def456"
    path: "/home/user/projects/beta"
    name: "Beta Service"
    last_accessed: "2025-01-14T15:45:00Z"
    status: "healthy"
  - id: "proj-ghi789"
    path: "/home/user/projects/gamma"
    name: "Gamma API"
    last_accessed: "2025-01-10T09:00:00Z"
    status: "degraded"  # .quorum exists but config invalid
```

2. **Project Registry Service** (`internal/project/registry.go`):
```go
type ProjectRegistry struct {
    configPath string
    projects   map[string]*Project
    mu         sync.RWMutex
}

type Project struct {
    ID           string    `yaml:"id"`
    Path         string    `yaml:"path"`
    Name         string    `yaml:"name"`
    LastAccessed time.Time `yaml:"last_accessed"`
    Status       string    `yaml:"status"`
}

func (r *ProjectRegistry) AddProject(path string) (*Project, error) {
    // Validate .quorum directory exists
    quorumDir := filepath.Join(path, ".quorum")
    if _, err := os.Stat(quorumDir); os.IsNotExist(err) {
        return nil, fmt.Errorf("not a quorum project: %s", path)
    }

    // Validate config is readable
    configPath := filepath.Join(quorumDir, "config.yaml")
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        // Create with "degraded" status
        return r.createProject(path, "degraded")
    }

    return r.createProject(path, "healthy")
}

func (r *ProjectRegistry) ListProjects() []*Project {
    r.mu.RLock()
    defer r.mu.RUnlock()

    projects := make([]*Project, 0, len(r.projects))
    for _, p := range r.projects {
        projects = append(projects, p)
    }
    return projects
}

func (r *ProjectRegistry) ValidateProject(id string) error {
    r.mu.RLock()
    project, ok := r.projects[id]
    r.mu.RUnlock()

    if !ok {
        return fmt.Errorf("project not found: %s", id)
    }

    // Check directory still exists
    if _, err := os.Stat(project.Path); os.IsNotExist(err) {
        return fmt.Errorf("project directory missing: %s", project.Path)
    }

    // Check .quorum still valid
    quorumDir := filepath.Join(project.Path, ".quorum")
    if _, err := os.Stat(quorumDir); os.IsNotExist(err) {
        return fmt.Errorf("project not initialized: %s", project.Path)
    }

    return nil
}
```

3. **UI Flow for Adding Projects**:
```
User clicks "Add Project" in selector dropdown
        |
        v
File/folder picker dialog opens
        |
        v
User selects project directory
        |
        v
Backend validates .quorum/ exists
        |
        +-- No .quorum/ --> Error: "Not a Quorum project. Run 'quorum init' first"
        |
        +-- .quorum/ exists --> Project added to registry
        |
        v
Project appears in selector dropdown
```

**Edge Cases to Handle**:
- Project directory deleted after registration
- `.quorum/` corrupted or incomplete
- Permissions changed (no read access)
- Network mount unavailable
- Path with special characters
- Duplicate registration attempts

### 9.5 Difficulty 5: Git Worktree Isolation

**Technical Description**: Git worktrees are created relative to the repository root for workflow execution. With multi-project support, parallel workflows from different projects could have worktree conflicts, especially in monorepo scenarios.

**Current Implementation** (`internal/adapters/git/worktree.go`):
- Worktrees created under `{repo-root}/.quorum/worktrees/{workflow-id}/`
- Each workflow gets a dedicated worktree for isolated execution
- Worktrees are cleaned up after workflow completion

**Risk Scenario**:
```
Monorepo/
├── project-a/
│   └── .quorum/
│       └── worktrees/
│           └── wf-123/  <-- Workflow from Project A
└── project-b/
    └── .quorum/
        └── worktrees/
            └── wf-456/  <-- Workflow from Project B (COULD CONFLICT)
```

If both projects share the same parent Git repository, worktree management could conflict when both try to create worktrees with overlapping refs.

**Solution**:

1. **Include Project ID in Worktree Path**:
```go
func (w *WorktreeManager) CreateWorktree(ctx context.Context, workflowID, projectID string) (string, error) {
    // Path includes project ID to prevent collisions
    worktreePath := filepath.Join(
        w.repoRoot,
        ".quorum",
        "worktrees",
        projectID,      // NEW: Project-scoped
        workflowID,
    )

    if err := os.MkdirAll(filepath.Dir(worktreePath), 0o750); err != nil {
        return "", fmt.Errorf("creating worktree directory: %w", err)
    }

    // Create worktree with unique branch name
    branchName := fmt.Sprintf("quorum-%s-%s", projectID[:8], workflowID[:8])
    // ...
}
```

2. **Add Worktree Validation**:
```go
func (w *WorktreeManager) ValidateNoConflict(projectID, workflowID string) error {
    // Check no other project is using overlapping worktree
    pattern := filepath.Join(w.repoRoot, ".quorum", "worktrees", "*", workflowID)
    matches, _ := filepath.Glob(pattern)

    for _, match := range matches {
        existingProject := filepath.Base(filepath.Dir(match))
        if existingProject != projectID {
            return fmt.Errorf("worktree conflict: workflow %s already has worktree in project %s",
                workflowID, existingProject)
        }
    }
    return nil
}
```

3. **Cleanup Mechanism**:
```go
func (w *WorktreeManager) CleanupOrphanedWorktrees(ctx context.Context) error {
    // Find all worktrees
    pattern := filepath.Join(w.repoRoot, ".quorum", "worktrees", "*", "*")
    matches, _ := filepath.Glob(pattern)

    for _, worktreePath := range matches {
        workflowID := filepath.Base(worktreePath)
        projectID := filepath.Base(filepath.Dir(worktreePath))

        // Check if workflow still exists
        exists, _ := w.workflowExists(ctx, projectID, workflowID)
        if !exists {
            w.logger.Info("removing orphaned worktree",
                "project", projectID,
                "workflow", workflowID)
            _ = os.RemoveAll(worktreePath)
        }
    }
    return nil
}
```

---

## 10. Architectural Alternatives

### 10.1 Alternative A: Multi-Server with WebUI Endpoint Selection

**Concept**: Maintain the current "one server per project" architecture. Enhance the WebUI to connect to multiple server endpoints, allowing users to switch between projects by changing the active backend connection.

**Architecture Diagram**:
```
                    +-------------------------+
                    |        WebUI            |
                    |  (Single Page App)      |
                    +------------+------------+
                                 |
              +------------------+------------------+
              |                  |                  |
              v                  v                  v
    +--------------------+ +--------------------+ +--------------------+
    | Server Instance A  | | Server Instance B  | | Server Instance C  |
    | localhost:8081     | | localhost:8082     | | localhost:8083     |
    | Project: Alpha     | | Project: Beta      | | Project: Gamma     |
    +--------------------+ +--------------------+ +--------------------+
              |                  |                  |
              v                  v                  v
    +--------------------+ +--------------------+ +--------------------+
    | /path/to/alpha/    | | /path/to/beta/     | | /path/to/gamma/    |
    | .quorum/           | | .quorum/           | | .quorum/           |
    +--------------------+ +--------------------+ +--------------------+
```

**Required Changes**:

| Component | Change | Complexity |
|-----------|--------|------------|
| Frontend API client | Dynamic base URL per project | Low |
| Frontend SSE hook | Reconnect to different endpoint on switch | Low |
| Frontend stores | Reset state on project switch | Medium |
| Project selector UI | Store list of endpoints, current selection | Low |
| Backend | **None** | None |

**Implementation Details**:

1. **API Client Modification** (`frontend/src/lib/api.js`):
```javascript
let currentEndpoint = 'http://localhost:8080';

export function setProjectEndpoint(endpoint) {
    currentEndpoint = endpoint;
}

export const workflowApi = {
    list: () => fetch(`${currentEndpoint}/api/v1/workflows`).then(r => r.json()),
    // ... all methods use currentEndpoint
};
```

2. **Project Registry in LocalStorage**:
```javascript
// frontend/src/lib/projectRegistry.js
const PROJECTS_KEY = 'quorum-projects';

export function getProjects() {
    return JSON.parse(localStorage.getItem(PROJECTS_KEY) || '[]');
}

export function addProject(name, endpoint) {
    const projects = getProjects();
    projects.push({ id: crypto.randomUUID(), name, endpoint, addedAt: new Date().toISOString() });
    localStorage.setItem(PROJECTS_KEY, JSON.stringify(projects));
}

export function removeProject(id) {
    const projects = getProjects().filter(p => p.id !== id);
    localStorage.setItem(PROJECTS_KEY, JSON.stringify(projects));
}
```

**Pros**:
- **Minimal backend changes** - Zero modifications to existing backend code
- **Strong isolation** - Each project runs in separate process
- **Low risk** - No complex refactoring
- **Independent scaling** - Can run servers on different machines
- **Gradual rollout** - Can implement incrementally

**Cons**:
- **Multiple processes** - User must start server for each project
- **Resource duplication** - N instances of Go runtime, memory overhead
- **No centralized discovery** - User must manually configure endpoints
- **SSE reconnection** - Must reconnect on project switch (brief disruption)
- **Port management** - Need to track which port each project uses

**Compatibility**:
- CLI/TUI: No changes (continue single-project operation)
- Backend: No changes
- Frontend: Medium changes (~800 lines)

**Recommended For**: Teams wanting quick multi-project access without backend complexity.

### 10.2 Alternative B: In-Process Multi-Project with ProjectContext (Recommended)

**Concept**: Modify the backend to manage multiple projects within a single server process. Introduce explicit `ProjectID` scoping throughout the API, state management, and event system.

**Architecture Diagram**:
```
+-------------------------------------------------------------------------+
|                        Single quorum serve Process                       |
|                                                                          |
|  +------------------------+  +------------------------+                 |
|  | Project A Context      |  | Project B Context      |                 |
|  | - StateManager A       |  | - StateManager B       |                 |
|  | - ConfigLoader A       |  | - ConfigLoader B       |                 |
|  | - EventBus A           |  | - EventBus B           |                 |
|  | - ChatStore A          |  | - ChatStore B          |                 |
|  | root: /path/to/alpha   |  | root: /path/to/beta    |                 |
|  +------------------------+  +------------------------+                 |
|              |                         |                                 |
|              +-----------+-------------+                                 |
|                          |                                               |
|  +------------------------v-----------------------+                     |
|  |              ProjectStatePool                  |                     |
|  |  - Lazy loads ProjectContext per project       |                     |
|  |  - LRU eviction for memory management          |                     |
|  |  - Thread-safe concurrent access               |                     |
|  +------------------------------------------------+                     |
|                          |                                               |
|  +------------------------v-----------------------+                     |
|  |              Project-Aware API Layer           |                     |
|  |  GET  /api/v1/projects                         |                     |
|  |  GET  /api/v1/projects/{id}/workflows          |                     |
|  |  POST /api/v1/projects/{id}/workflows          |                     |
|  |  GET  /api/v1/projects/{id}/config             |                     |
|  |  GET  /api/v1/projects/{id}/sse/events         |                     |
|  +------------------------------------------------+                     |
|                          |                                               |
+-------------------------------------------------------------------------+
                           |
                           v
+-------------------------------------------------------------------------+
|                         WebUI (React)                                    |
|  +-------------------+   +------------------+   +-------------------+    |
|  | Project Selector  |   | Project Store    |   | Per-Project       |    |
|  | (Header Dropdown) |   | currentProjectId |   | Nested Stores     |    |
|  +-------------------+   +------------------+   +-------------------+    |
+-------------------------------------------------------------------------+
```

**Required Changes**:

| Component | Change | Files | Complexity |
|-----------|--------|-------|------------|
| ProjectRegistry | New service for project management | 2 | Medium |
| ProjectContext | Encapsulates per-project resources | 3 | Medium |
| ProjectStatePool | Manages multiple StateManagers | 2 | High |
| API Routes | Add `/projects/{id}` prefix | 8 | Medium |
| API Middleware | Extract project from path | 1 | Low |
| EventBus | Add ProjectID to events, filtering | 2 | Medium |
| SSE Handler | Project-scoped event streams | 1 | Medium |
| Frontend Stores | Nest state by project | 5 | High |
| Frontend Router | Add project routes | 2 | Medium |
| Frontend API | Dynamic project base | 2 | Low |
| Project Selector UI | New component | 1 | Low |

**Core Implementation - ProjectContext**:
```go
// internal/project/context.go
type ProjectContext struct {
    ID             string
    Root           string
    StateManager   core.StateManager
    ConfigLoader   *config.Loader
    EventBus       *events.EventBus
    ChatStore      core.ChatStore
    Attachments    *attachments.Store
}

func NewProjectContext(id, root string) (*ProjectContext, error) {
    // Validate project directory
    quorumDir := filepath.Join(root, ".quorum")
    if _, err := os.Stat(quorumDir); os.IsNotExist(err) {
        return nil, fmt.Errorf("not a valid quorum project: %s", root)
    }

    // Create state manager for this project
    statePath := filepath.Join(quorumDir, "state", "state.db")
    stateManager, err := state.NewSQLiteStateManager(statePath)
    if err != nil {
        return nil, fmt.Errorf("creating state manager: %w", err)
    }

    // Create config loader for this project
    configLoader := config.NewLoader(config.WithRoot(root))

    // Create event bus for this project
    eventBus := events.New(100)

    // Create chat store for this project
    chatPath := filepath.Join(quorumDir, "chat.db")
    chatStore, err := chat.NewSQLiteStore(chatPath)
    if err != nil {
        return nil, fmt.Errorf("creating chat store: %w", err)
    }

    // Create attachments store for this project
    attachStore := attachments.NewStore(root)

    return &ProjectContext{
        ID:           id,
        Root:         root,
        StateManager: stateManager,
        ConfigLoader: configLoader,
        EventBus:     eventBus,
        ChatStore:    chatStore,
        Attachments:  attachStore,
    }, nil
}

func (pc *ProjectContext) Close() error {
    // Close all resources
    if closer, ok := pc.StateManager.(io.Closer); ok {
        closer.Close()
    }
    if closer, ok := pc.ChatStore.(io.Closer); ok {
        closer.Close()
    }
    pc.EventBus.Close()
    return nil
}
```

**Core Implementation - ProjectStatePool**:
```go
// internal/project/pool.go
type ProjectStatePool struct {
    registry    *ProjectRegistry
    contexts    map[string]*ProjectContext
    accessOrder []string  // For LRU tracking
    mu          sync.RWMutex
    maxActive   int
    logger      *slog.Logger
}

func NewProjectStatePool(registry *ProjectRegistry, maxActive int) *ProjectStatePool {
    return &ProjectStatePool{
        registry:  registry,
        contexts:  make(map[string]*ProjectContext),
        maxActive: maxActive,
        logger:    slog.Default(),
    }
}

func (p *ProjectStatePool) GetContext(ctx context.Context, projectID string) (*ProjectContext, error) {
    p.mu.RLock()
    if pc, ok := p.contexts[projectID]; ok {
        p.mu.RUnlock()
        p.updateAccessOrder(projectID)
        return pc, nil
    }
    p.mu.RUnlock()

    p.mu.Lock()
    defer p.mu.Unlock()

    // Double-check after lock
    if pc, ok := p.contexts[projectID]; ok {
        return pc, nil
    }

    // Get project info from registry
    project, err := p.registry.GetProject(projectID)
    if err != nil {
        return nil, fmt.Errorf("project not found: %w", err)
    }

    // Create new context
    pc, err := NewProjectContext(projectID, project.Path)
    if err != nil {
        return nil, fmt.Errorf("creating project context: %w", err)
    }

    // LRU eviction if needed
    if len(p.contexts) >= p.maxActive {
        p.evictLRU()
    }

    p.contexts[projectID] = pc
    p.accessOrder = append(p.accessOrder, projectID)

    p.logger.Info("loaded project context", "project_id", projectID, "path", project.Path)
    return pc, nil
}

func (p *ProjectStatePool) evictLRU() {
    if len(p.accessOrder) == 0 {
        return
    }

    // Find LRU project (not currently active)
    for i := 0; i < len(p.accessOrder); i++ {
        projectID := p.accessOrder[i]
        pc := p.contexts[projectID]

        // Don't evict if has running workflows
        running, _ := pc.StateManager.ListRunningWorkflows(context.Background())
        if len(running) > 0 {
            continue
        }

        // Evict this project
        pc.Close()
        delete(p.contexts, projectID)
        p.accessOrder = append(p.accessOrder[:i], p.accessOrder[i+1:]...)
        p.logger.Info("evicted project context (LRU)", "project_id", projectID)
        return
    }
}
```

**API Routing Modification**:
```go
// internal/api/server.go
func (s *Server) setupRouter() chi.Router {
    r := chi.NewRouter()

    // ... middleware ...

    r.Route("/api/v1", func(r chi.Router) {
        // Project listing (no project context needed)
        r.Get("/projects", s.handleListProjects)
        r.Post("/projects", s.handleAddProject)

        // Project-scoped endpoints
        r.Route("/projects/{projectID}", func(r chi.Router) {
            r.Use(s.projectContextMiddleware)  // Injects ProjectContext

            r.Get("/", s.handleGetProject)
            r.Delete("/", s.handleRemoveProject)

            r.Route("/workflows", func(r chi.Router) {
                r.Get("/", s.handleListWorkflows)
                r.Post("/", s.handleCreateWorkflow)
                r.Get("/active", s.handleGetActiveWorkflow)
                // ... existing workflow endpoints
            })

            r.Route("/config", func(r chi.Router) {
                r.Get("/", s.handleGetConfig)
                r.Put("/", s.handleUpdateConfig)
            })

            r.Get("/sse/events", s.handleSSE)  // Project-filtered SSE
        })

        // Legacy endpoints (backward compatibility with default project)
        r.Route("/workflows", func(r chi.Router) {
            r.Use(s.defaultProjectMiddleware)
            // ... existing endpoints work with default project
        })
    })

    return r
}

func (s *Server) projectContextMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        projectID := chi.URLParam(r, "projectID")
        if projectID == "" {
            respondError(w, http.StatusBadRequest, "project ID required")
            return
        }

        pc, err := s.pool.GetContext(r.Context(), projectID)
        if err != nil {
            respondError(w, http.StatusNotFound, err.Error())
            return
        }

        // Add to request context
        ctx := context.WithValue(r.Context(), projectContextKey, pc)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Pros**:
- **True multi-project support** - Single process manages all projects
- **Unified UX** - Seamless project switching, no reconnection delays
- **Resource efficient** - Shared Go runtime, lazy loading, LRU eviction
- **Strong isolation** - Each project has separate state, events, config
- **Backward compatible** - Legacy endpoints continue to work
- **Centralized management** - Project registry in one location

**Cons**:
- **Significant refactoring** - ~6,500 lines of changes
- **Higher complexity** - Pool management, context switching
- **Shared fate** - Server crash affects all projects
- **Memory growth** - Active projects consume memory
- **Testing complexity** - Multi-project scenarios need thorough testing

**Compatibility**:
- CLI/TUI: Optional (can add `--project` flag later)
- Backend: Major changes (but backward compatible)
- Frontend: Major changes

**Recommended For**: Production deployments requiring proper multi-tenant support.

### 10.3 Alternative C: Global Active Project Switching

**Concept**: Add a server-level "active project" that can be switched via API. All existing endpoints operate on the current active project.

**Architecture Diagram**:
```
+---------------------------+
|       WebUI               |
+------------+--------------+
             |
             | POST /api/v1/projects/active
             | body: { "project_id": "proj-xyz" }
             |
             v
+---------------------------+
|    API Server             |
|    activeProject: "xyz"   | <-- Global state
+------------+--------------+
             |
             | All endpoints use activeProject
             |
             v
+---------------------------+
|    /path/to/xyz/.quorum/  |
+---------------------------+
```

**Required Changes**:

| Component | Change | Complexity |
|-----------|--------|------------|
| Server struct | Add `activeProjectID string` | Low |
| Project switch endpoint | `POST /projects/active` | Low |
| Middleware | Resolve paths using active project | Low |
| State reloading | Reinitialize StateManager on switch | Medium |
| Frontend | Add project selector, call switch endpoint | Low |

**Implementation**:
```go
type Server struct {
    // ... existing fields
    activeProjectID string
    projectRegistry *ProjectRegistry
    mu              sync.RWMutex
}

func (s *Server) handleSetActiveProject(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ProjectID string `json:"project_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request")
        return
    }

    project, err := s.projectRegistry.GetProject(req.ProjectID)
    if err != nil {
        respondError(w, http.StatusNotFound, "project not found")
        return
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    // Close current state manager
    if closer, ok := s.stateManager.(io.Closer); ok {
        closer.Close()
    }

    // Reinitialize for new project
    statePath := filepath.Join(project.Path, ".quorum", "state", "state.db")
    s.stateManager, err = state.NewSQLiteStateManager(statePath)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to switch project")
        return
    }

    s.root = project.Path
    s.activeProjectID = req.ProjectID

    respondJSON(w, http.StatusOK, map[string]string{"active_project": req.ProjectID})
}
```

**Pros**:
- **Minimal code changes** - ~500 lines
- **Simple model** - One active project at a time
- **Quick implementation** - Low risk, fast delivery

**Cons**:
- **Race conditions** - If multiple users switch projects simultaneously
- **Not truly multi-tenant** - Can only view one project at a time
- **State loss** - Switching loses any in-memory state
- **SSE disruption** - Events from old project may leak during switch
- **Poor UX** - Must switch to view other project data

**Compatibility**:
- CLI/TUI: No changes
- Backend: Minor changes
- Frontend: Minor changes

**Not Recommended**: This approach has fundamental flaws for multi-user scenarios and provides a degraded user experience compared to alternatives A or B.

### 10.4 Alternative Comparison Matrix

```
+------------------+-------------+--------------+-------------+
| Criterion        | Alt A       | Alt B        | Alt C       |
|                  | Multi-Server| In-Process   | Global      |
+------------------+-------------+--------------+-------------+
| Backend changes  | None        | Major        | Minor       |
| Frontend changes | Medium      | Major        | Minor       |
| Isolation        | Strong      | Strong       | Weak        |
| UX quality       | Good        | Excellent    | Poor        |
| Resource usage   | Higher      | Medium       | Lowest      |
| Implementation   | 2-3 weeks   | 6-8 weeks    | 1 week      |
| Multi-user safe  | Yes         | Yes          | No          |
| Real-time switch | No*         | Yes          | Yes         |
| Discovery        | Manual      | Automatic    | Manual      |
+------------------+-------------+--------------+-------------+
* Requires SSE reconnection on project switch
```

### 10.5 Recommendation

**Recommended: Alternative B (In-Process Multi-Project with ProjectContext)**

**Rationale**:

1. **Proper Architectural Foundation**: Creates a clean separation between projects with explicit `ProjectContext` boundaries, preventing data leakage and enabling future enhancements like per-project permissions.

2. **Maintains Existing Project Structure**: Each project keeps its `.quorum/` directory unchanged. No migration of existing data required.

3. **Backward Compatible**: Legacy endpoints continue to work with a "default" project, allowing gradual migration.

4. **Unified User Experience**: Single interface, real-time project switching, no reconnection delays, consistent behavior across all views.

5. **Efficient Resource Use**: Shared Go runtime with lazy-loaded project contexts and LRU eviction prevents memory bloat while maintaining responsiveness.

6. **Scalable Design**: The `ProjectStatePool` pattern can be extended to support remote project contexts, distributed deployments, or cloud-hosted project storage in the future.

**Implementation Approach**: Phased delivery (see Section 11) to minimize risk and allow iterative validation.

---

## 11. Implementation Plan (For Alternative B)

### Phase 1: Backend Foundation (~2 weeks effort)

**Objective**: Add project awareness to core backend components without breaking existing functionality.

**Step 1.1: Create ProjectRegistry** (`internal/project/registry.go`)
- Load/save `~/.config/quorum/projects.yaml`
- Methods: `ListProjects()`, `AddProject(path)`, `RemoveProject(id)`, `GetProject(id)`, `ValidateProject(id)`
- Edge case handling: missing directories, permission errors, corrupted configs

**Step 1.2: Create ProjectContext** (`internal/project/context.go`)
- Encapsulates: StateManager, ConfigLoader, EventBus, ChatStore, AttachmentStore
- Lifecycle: `NewProjectContext(id, path)`, `Close()`
- Validation: Ensure `.quorum/` exists and is properly initialized

**Step 1.3: Create ProjectStatePool** (`internal/project/pool.go`)
- Map of `projectID → ProjectContext`
- Lazy initialization on first access
- LRU eviction when exceeding `maxActive`
- Thread-safe access with read-write locking

**Step 1.4: Extend Event Interface** (`internal/events/bus.go`)
- Add `ProjectID() string` to Event interface
- Add `Project string` field to BaseEvent
- Update `NewBaseEvent()` to accept projectID
- Add `SubscribeForProject(projectID)` method
- Modify `Publish()` to support project filtering

**Step 1.5: Add Project API Endpoints** (`internal/api/projects.go`)
- `GET /api/v1/projects` - List registered projects
- `POST /api/v1/projects` - Register new project (accepts path)
- `DELETE /api/v1/projects/{id}` - Remove project from registry
- `GET /api/v1/projects/{id}` - Get project details and health status

**Deliverables**:
- ProjectRegistry service with persistence
- ProjectContext with full resource encapsulation
- ProjectStatePool with lazy loading and LRU
- Extended EventBus with project filtering
- New project management endpoints
- Unit tests for all new components

### Phase 2: API Layer Modifications (~1.5 weeks effort)

**Objective**: Scope all workflow and configuration operations by project.

**Step 2.1: Modify Server struct** (`internal/api/server.go`)
- Replace single `stateManager` with `ProjectStatePool`
- Add `projectRegistry` field
- Update `NewServer()` to initialize pool

**Step 2.2: Add Project Middleware** (`internal/api/middleware.go`)
- Extract project ID from URL path parameter
- Retrieve `ProjectContext` from pool
- Attach context to request for downstream handlers
- Handle errors: invalid project ID, project not found

**Step 2.3: Update All Handlers** (`internal/api/workflows.go`, `internal/api/config.go`, etc.)
- Extract `ProjectContext` from request context
- Use project-specific StateManager, ConfigLoader
- Update all response models to include `project_id`

**Step 2.4: Modify SSE Handler** (`internal/api/sse.go`)
- Accept `?project=` query parameter
- Use `SubscribeForProject()` for filtered subscription
- Include `project_id` in all event payloads

**Step 2.5: Add Legacy Compatibility Layer**
- Existing `/api/v1/workflows` endpoints continue to work
- Use "default" project (first registered or cwd-based)
- Log deprecation warnings for legacy endpoint usage

**Deliverables**:
- Modified Server with ProjectStatePool
- Project context middleware
- Updated handlers using project context
- Project-filtered SSE
- Backward-compatible legacy endpoints
- Integration tests for multi-project scenarios

### Phase 3: Frontend Integration (~2 weeks effort)

**Objective**: Add project selection and navigation to WebUI.

**Step 3.1: Create projectStore** (`frontend/src/stores/projectStore.js`)
```javascript
{
  projects: [],           // List of registered projects
  currentProjectId: null, // Active project
  loading: false,
  error: null,
  // Actions
  loadProjects: async () => {},
  selectProject: (id) => {},
  addProject: async (path) => {},
  removeProject: async (id) => {},
}
```

**Step 3.2: Create ProjectSelector Component** (`frontend/src/components/ProjectSelector.jsx`)
- Dropdown in header (upper-right per requirements)
- Shows current project name with color indicator
- Lists all registered projects
- "Add Project" option with folder picker
- "Manage Projects" option for removal/editing

**Step 3.3: Modify Existing Stores**
- Restructure workflowStore with nested `projects` state
- Add `currentProjectId` dependency
- Create selectors for current project data
- Update event handlers to route to correct project

**Step 3.4: Update API Client** (`frontend/src/lib/api.js`)
- Create `getProjectApiBase(projectId)` helper
- Update all API methods to include project in path
- Handle project switching gracefully

**Step 3.5: Update SSE Hook** (`frontend/src/hooks/useSSE.js`)
- Include project ID in SSE URL
- Reconnect when project changes
- Route events to correct project state

**Step 3.6: Update Layout Component** (`frontend/src/components/Layout.jsx`)
- Integrate ProjectSelector in header
- Show project indicator/badge
- Handle loading states during project switch

**Step 3.7: Update Routing** (Optional)
- Add `/projects/:projectId/...` route structure
- Or use store-based project context (simpler)

**Deliverables**:
- projectStore with full project management
- ProjectSelector component
- Restructured workflow/task/config stores
- Updated API client with project scoping
- Project-aware SSE handling
- Updated Layout with project selector
- E2E tests for project switching

### Phase 4: CLI Enhancement (~1 week effort)

**Objective**: Add project awareness to CLI commands (optional, can be deferred).

**Step 4.1: Add Global --project Flag** (`cmd/quorum/cmd/root.go`)
```go
rootCmd.PersistentFlags().String("project", "", "Project ID or path (default: current directory)")
```

**Step 4.2: Resolve Project Context** (`cmd/quorum/cmd/common.go`)
- If `--project` provided, look up in registry or use as path
- Otherwise, use current directory
- Validate `.quorum/` exists at resolved path

**Step 4.3: Add Project Commands**
```
quorum project list              - List registered projects
quorum project add [path]        - Register project (default: current dir)
quorum project remove <id>       - Unregister a project
quorum project info [id]         - Show project details
quorum project switch <id>       - Set default project in registry
```

**Deliverables**:
- Global `--project` flag
- Project context resolution
- Project management commands
- Updated command documentation

### Phase 5: Testing and Validation (~1.5 weeks effort)

**Objective**: Ensure stability, performance, and backward compatibility.

**Step 5.1: Unit Tests**
- ProjectRegistry CRUD operations
- ProjectStatePool concurrency and LRU eviction
- EventBus project filtering
- API middleware project resolution

**Step 5.2: Integration Tests**
- Multi-project SSE streams
- Concurrent access to different projects
- Project switching with active workflows
- State isolation verification

**Step 5.3: Performance Testing**
- Memory usage with 5, 10, 20 active projects
- SSE event throughput with multiple projects
- API latency with project context resolution

**Step 5.4: Migration Testing**
- Existing single-project setups continue to work
- Legacy endpoint compatibility
- Config file format compatibility

**Step 5.5: Documentation**
- Updated API documentation
- Migration guide from single-project
- Project registry format specification

**Deliverables**:
- Comprehensive test suite
- Performance benchmarks
- Migration documentation
- Updated user guide

---

## 12. UX/UI Recommendations

### 12.1 Project Selector Component Design

**Location**: Upper-right corner of header (as specified in requirements)

```
+-----------------------------------------------------------------------+
|  [☰] Quorum                                      [● Alpha ▾] [⚙]     |
+-----------------------------------------------------------------------+
|  Dashboard │ Workflows │ Kanban │ Chat │ Settings                     |
+-----------------------------------------------------------------------+
```

**Dropdown Behavior**:
```
+----------------------------------+
| ● Alpha                    ✓     |  <-- Current project (checkmark)
| ○ Beta                           |
| ○ Gamma (offline)          ⚠     |  <-- Status indicator
|----------------------------------|
| + Add Project...                 |
| ⚙ Manage Projects                |
+----------------------------------+
```

**Color Coding System**:
- Each project gets a consistent accent color (generated from project ID hash)
- Color appears as a small dot/badge next to project name
- Active workflows show pulsing indicator

### 12.2 Visual Indicators

1. **Project Badge in Header**: Small colored dot indicating current project
2. **Breadcrumb Enhancement**: Shows `Project: Alpha > Workflows > wf-abc123`
3. **Activity Indicator**: Pulsing dot when project has running workflows
4. **Offline/Degraded Status**: Warning icon for projects with issues

### 12.3 Project Switching Flow

```
User clicks Project Selector
        │
        ▼
┌─────────────────────────────────┐
│  Dropdown shows projects        │
│  - Current: checkmark           │
│  - Status indicators            │
└─────────────────┬───────────────┘
                  │
                  ▼
┌─────────────────────────────────┐
│  User selects different project │
└─────────────────┬───────────────┘
                  │
                  ▼
┌─────────────────────────────────┐
│  Frontend updates:              │
│  1. Sets currentProjectId       │
│  2. Shows loading indicator     │
│  3. Reconnects SSE with project │
│  4. Fetches project data        │
└─────────────────┬───────────────┘
                  │
                  ▼
┌─────────────────────────────────┐
│  UI reflects new project:       │
│  - Dashboard shows new metrics  │
│  - Workflows list updates       │
│  - Config reflects project      │
└─────────────────────────────────┘
```

### 12.4 Add Project Flow

```
User clicks "Add Project..."
        │
        ▼
┌─────────────────────────────────┐
│  File/folder picker dialog      │
│  "Select Quorum Project Folder" │
└─────────────────┬───────────────┘
                  │
                  ▼
┌─────────────────────────────────┐
│  User selects directory         │
└─────────────────┬───────────────┘
                  │
                  ▼
┌─────────────────────────────────┐
│  Backend validates:             │
│  - .quorum/ exists              │
│  - config.yaml readable         │
│  - Not already registered       │
└─────────────────┬───────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ▼                   ▼
   [Valid]            [Invalid]
        │                   │
        ▼                   ▼
┌───────────────┐   ┌───────────────┐
│ Project added │   │ Error message │
│ to registry   │   │ with guidance │
└───────────────┘   └───────────────┘
```

### 12.5 Keyboard Shortcuts

- `Ctrl/Cmd + K`: Open quick project switcher (command palette style)
- `Ctrl/Cmd + [1-9]`: Switch to project by position
- `Ctrl/Cmd + Shift + P`: Add new project

---

## 13. Security Considerations

### 13.1 Access Control Risks

| Risk | Severity | Description | Evidence Location |
|------|----------|-------------|-------------------|
| Cross-project data access | HIGH | API could return data from wrong project | No project validation in handlers |
| Path traversal | HIGH | Malicious project paths could escape sandbox | `internal/api/files.go:265` |
| SSE event leakage | MEDIUM | Events from one project visible to another | `internal/api/sse.go:33-34` |
| Sensitive path exposure | MEDIUM | Filesystem paths visible in API responses | Project registry stores full paths |
| Unauthorized project access | MEDIUM | No auth mechanism for project-level access | `internal/api/server.go:163` |

### 13.2 Required Security Measures

**1. Project Path Validation** (Critical):
```go
func validateProjectPath(path string) error {
    // Must be absolute path
    if !filepath.IsAbs(path) {
        return errors.New("project path must be absolute")
    }

    // Clean path to resolve any . or ..
    cleanPath := filepath.Clean(path)
    if cleanPath != path {
        return errors.New("project path contains invalid sequences")
    }

    // Verify .quorum directory exists
    quorumDir := filepath.Join(cleanPath, ".quorum")
    info, err := os.Stat(quorumDir)
    if err != nil {
        return fmt.Errorf(".quorum directory not accessible: %w", err)
    }
    if !info.IsDir() {
        return errors.New(".quorum is not a directory")
    }

    // Verify we have read access
    configPath := filepath.Join(quorumDir, "config.yaml")
    if _, err := os.ReadFile(configPath); err != nil {
        return fmt.Errorf("cannot read project config: %w", err)
    }

    return nil
}
```

**2. Project ID Generation**:
```go
func generateProjectID() string {
    // Use cryptographically random ID
    // Don't expose filesystem paths in IDs
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("proj-%s", hex.EncodeToString(b)[:12])
}
```

**3. Request Scoping Middleware**:
```go
func (s *Server) requireProjectContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        pc := getProjectContext(r.Context())
        if pc == nil {
            respondError(w, http.StatusUnauthorized, "project context required")
            return
        }

        // Verify project is still valid
        if err := s.registry.ValidateProject(pc.ID); err != nil {
            respondError(w, http.StatusNotFound, "project no longer valid")
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**4. File Path Validation per Project**:
```go
func (s *Server) resolveFilePath(pc *ProjectContext, requestedPath string) (string, error) {
    // Resolve against project root
    absPath := filepath.Join(pc.Root, filepath.Clean(requestedPath))

    // Ensure still under project root
    if !strings.HasPrefix(absPath, pc.Root) {
        return "", fmt.Errorf("path outside project: %s", requestedPath)
    }

    return absPath, nil
}
```

**5. SSE Event Isolation**:
```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    projectID := chi.URLParam(r, "projectID")
    if projectID == "" {
        respondError(w, http.StatusBadRequest, "project ID required for SSE")
        return
    }

    pc, err := s.pool.GetContext(r.Context(), projectID)
    if err != nil {
        respondError(w, http.StatusNotFound, "project not found")
        return
    }

    // Subscribe to THIS project's EventBus only
    eventCh := pc.EventBus.Subscribe()
    // ...
}
```

---

## 14. Performance Considerations

### 14.1 Memory Impact Analysis

**Current Single-Project Baseline**:
- ~50MB base memory (Go runtime + loaded libraries)
- +10MB per active workflow (state, events, execution context)
- +5MB for SQLite connections and caches

**Multi-Project with Lazy Loading**:
- ~50MB base memory (unchanged)
- +8MB per loaded ProjectContext:
  - StateManager: ~3MB (SQLite connections, WAL buffers)
  - EventBus: ~1MB (subscriber channels)
  - ConfigLoader: ~0.5MB
  - ChatStore: ~2MB
  - Attachments: ~1MB
  - Context overhead: ~0.5MB
- +10MB per active workflow (unchanged)

**Memory Management Strategy**:
```
+--------------------+---------------------+
| Active Projects    | Estimated Memory    |
+--------------------+---------------------+
| 1 (current)        | ~60MB               |
| 3 (typical use)    | ~75MB               |
| 5 (power user)     | ~90MB               |
| 10 (edge case)     | ~130MB              |
| 20 (extreme)       | ~210MB              |
+--------------------+---------------------+
```

**LRU Eviction Configuration**:
```go
// Suggested defaults
const (
    DefaultMaxActiveProjects = 5   // Keep 5 projects hot
    MinActiveProjects        = 2   // Never evict below this
    EvictionGracePeriod      = 5 * time.Minute  // Don't evict recently used
)
```

### 14.2 Network Impact

**SSE Connections**:
- Current: 1 SSE connection per client, receives ALL events
- Multi-project: 1 SSE connection per client, filtered by project (IMPROVED)

**API Call Patterns**:
- New: Project listing call on page load (~100 bytes per project)
- Unchanged: Workflow listing, creation, management calls
- Improved: Events contain only relevant project data

### 14.3 Database Impact

**Per-Project SQLite**:
- Each project maintains its own `state.db`
- No schema changes required
- Existing WAL configuration provides good concurrency

**Connection Pooling**:
- Write connection: 1 per project (SQLite limitation)
- Read connections: Up to 10 per project
- Total with 5 projects: 5 write + 50 read = 55 connections

### 14.4 Recommended Performance Optimizations

1. **Lazy State Loading**: Don't load full workflow history on project switch; load on-demand
2. **Event Batching**: Batch SSE events when multiple occur in rapid succession
3. **Connection Pooling**: Share read connection pool across projects if memory constrained
4. **State Caching**: Cache frequently accessed data (workflow summaries, config) in memory
5. **Background Cleanup**: Periodically clean up orphaned worktrees, old checkpoints

---

## 15. Edge Cases and Special Scenarios

### 15.1 Project Directory Issues

| Scenario | Detection | Handling |
|----------|-----------|----------|
| Directory deleted | `os.Stat()` fails on access | Mark as "offline", prompt for removal |
| Permissions changed | Read/write errors | Show error, offer to re-add with correct perms |
| `.quorum/` corrupted | Config parse fails, DB errors | Mark as "degraded", show diagnostic |
| Network mount unavailable | Timeout on access | Mark as "offline", retry with backoff |
| Disk full | Write errors | Show storage warning, prevent new operations |

### 15.2 Concurrent Access

| Scenario | Risk | Mitigation |
|----------|------|------------|
| Two users switch projects | Race in global active (Alt C) | Use Alt B with per-user context |
| Parallel workflows in same project | Lock contention | Per-workflow locking already exists |
| Same workflow from different clients | State inconsistency | Optimistic locking, conflict detection |
| Project removed while in use | Orphaned references | Validate project on each request |

### 15.3 Data Integrity

| Scenario | Risk | Mitigation |
|----------|------|------------|
| Crash during project switch | Partial state | Transaction boundaries, atomic operations |
| Concurrent state modifications | Lost updates | SQLite WAL, per-workflow locks |
| Event loss during SSE reconnect | Missing updates | Full state refresh on reconnect |
| Registry file corruption | Lost project list | Automatic backup, recovery from `.quorum/` dirs |

### 15.4 Monorepo Considerations

When multiple Quorum projects exist within a single Git repository:

```
monorepo/
├── .git/
├── service-a/
│   └── .quorum/
├── service-b/
│   └── .quorum/
└── shared/
```

**Risks**:
- Git worktree conflicts between services
- Unclear which project "owns" shared code
- Branch naming collisions

**Mitigations**:
- Project-prefixed worktree paths: `.quorum/worktrees/{project-id}/{workflow-id}/`
- Clear documentation about monorepo usage
- Warning when adding project that shares Git root with existing project

---

## 16. Conclusion

### 16.1 Viability Assessment Summary

| Criterion | Assessment | Confidence |
|-----------|------------|------------|
| **Technical Feasibility** | YES - Achievable with significant effort | High |
| **Architectural Fit** | MODERATE - Requires refactoring, not rewrite | High |
| **Backward Compatibility** | YES - Existing projects work unchanged | High |
| **Maintenance Impact** | HIGH - Increases codebase complexity 20-30% | Medium |
| **User Value** | HIGH - Major UX improvement for multi-project users | High |
| **Risk Level** | MEDIUM - Manageable with phased approach | Medium |

### 16.2 Final Recommendation

**Proceed with Alternative B (In-Process Multi-Project with ProjectContext)**, implemented in 5 phases as described in Section 11.

**Critical Success Factors**:

1. **Strong Isolation**: Enforce project boundaries at API layer through middleware
2. **Efficient Resource Use**: Implement lazy loading and LRU eviction from day one
3. **Clear UX**: Project selector must be intuitive with clear visual indicators
4. **Comprehensive Testing**: Multi-project scenarios must have thorough test coverage
5. **Phased Delivery**: Backend first, then frontend, with working state at each phase

### 16.3 Implementation Summary

**Total Estimated Effort**: 8-10 weeks for full implementation

**Code Changes**:
- Backend: ~3,500 lines of new/modified code
- Frontend: ~2,500 lines of new/modified code
- Tests: ~2,000 lines of new tests
- Documentation: ~500 lines

**Files Affected**: ~80 files across backend and frontend

### 16.4 Risks to Monitor

1. **Memory Usage**: Monitor with 5+ active projects, implement LRU aggressively
2. **SSE Throughput**: Benchmark event delivery under multi-project load
3. **Git Worktree Conflicts**: Test thoroughly in monorepo scenarios
4. **Migration Issues**: Ensure existing single-project users have smooth upgrade path
5. **Complexity Creep**: Resist adding features beyond core multi-project support in v1

### 16.5 Alternative Path

If time or resources are constrained, **Alternative A (Multi-Server with WebUI Endpoint Selection)** provides 80% of the user value with 20% of the implementation effort. This can serve as an interim solution while the full Alternative B implementation is developed.

---

## Appendix A: Files Requiring Modification

### Backend (Go) - Priority Order

| File | Change Type | Priority | Description |
|------|-------------|----------|-------------|
| `internal/project/registry.go` | NEW | P0 | Project registry service |
| `internal/project/context.go` | NEW | P0 | ProjectContext encapsulation |
| `internal/project/pool.go` | NEW | P0 | ProjectStatePool management |
| `internal/events/bus.go` | EXTEND | P0 | Add ProjectID to events |
| `internal/events/workflow.go` | EXTEND | P0 | Update event constructors |
| `internal/api/server.go` | MODIFY | P1 | Use ProjectStatePool |
| `internal/api/projects.go` | NEW | P1 | Project management endpoints |
| `internal/api/middleware.go` | NEW | P1 | Project context middleware |
| `internal/api/sse.go` | MODIFY | P1 | Project-filtered SSE |
| `internal/api/workflows.go` | MODIFY | P1 | Use project context |
| `internal/api/config.go` | MODIFY | P1 | Use project context |
| `internal/api/files.go` | MODIFY | P2 | Project-scoped path validation |
| `internal/core/ports.go` | EXTEND | P2 | Optional: add ProjectID to interface |
| `internal/core/workflow.go` | EXTEND | P2 | Optional: add ProjectID field |
| `cmd/quorum/cmd/root.go` | EXTEND | P3 | Add --project flag |
| `cmd/quorum/cmd/project.go` | NEW | P3 | Project management commands |

### Frontend (JavaScript/React) - Priority Order

| File | Change Type | Priority | Description |
|------|-------------|----------|-------------|
| `frontend/src/stores/projectStore.js` | NEW | P0 | Project state management |
| `frontend/src/components/ProjectSelector.jsx` | NEW | P0 | Project dropdown component |
| `frontend/src/stores/workflowStore.js` | MODIFY | P1 | Nest state by project |
| `frontend/src/stores/taskStore.js` | MODIFY | P1 | Nest state by project |
| `frontend/src/stores/agentStore.js` | MODIFY | P1 | Nest state by project |
| `frontend/src/stores/configStore.js` | MODIFY | P1 | Per-project config |
| `frontend/src/hooks/useSSE.js` | MODIFY | P1 | Project-scoped SSE |
| `frontend/src/lib/api.js` | MODIFY | P1 | Dynamic API base |
| `frontend/src/components/Layout.jsx` | MODIFY | P2 | Integrate ProjectSelector |
| `frontend/src/App.jsx` | MODIFY | P2 | Optional: project routes |

---

## Appendix B: API Endpoint Reference

### New Endpoints

```
GET    /api/v1/projects                              List registered projects
POST   /api/v1/projects                              Register new project
       Request: { "path": "/absolute/path/to/project" }
       Response: { "id": "proj-abc123", "name": "Project Name", "status": "healthy" }

DELETE /api/v1/projects/{projectId}                  Remove project from registry

GET    /api/v1/projects/{projectId}                  Get project details
       Response: { "id": "proj-abc123", "path": "...", "status": "healthy", "last_accessed": "..." }
```

### Modified Endpoints (Project-Scoped)

```
GET    /api/v1/projects/{projectId}/workflows        List workflows in project
POST   /api/v1/projects/{projectId}/workflows        Create workflow in project
GET    /api/v1/projects/{projectId}/workflows/{id}   Get specific workflow
PUT    /api/v1/projects/{projectId}/workflows/{id}   Update workflow
DELETE /api/v1/projects/{projectId}/workflows/{id}   Delete workflow
POST   /api/v1/projects/{projectId}/workflows/{id}/run    Start workflow
GET    /api/v1/projects/{projectId}/workflows/active      Get active workflow

GET    /api/v1/projects/{projectId}/config           Get project configuration
PUT    /api/v1/projects/{projectId}/config           Update project configuration

GET    /api/v1/projects/{projectId}/sse/events       Project-scoped SSE stream
```

### Backward Compatible Endpoints (Legacy)

```
# These continue to work using default/first project
GET    /api/v1/workflows                             → Redirects to default project
POST   /api/v1/workflows                             → Creates in default project
GET    /api/v1/config                                → Returns default project config
GET    /api/v1/sse/events                            → Returns all events (deprecated)
```

---

## Appendix C: Configuration Schema

### Project Registry File

**Location**: `~/.config/quorum/projects.yaml`

```yaml
version: 1
default_project: "proj-abc123"  # Optional: default for legacy endpoints
projects:
  - id: "proj-abc123"
    path: "/home/user/projects/alpha"
    name: "Project Alpha"
    last_accessed: "2025-01-15T10:30:00Z"
    status: "healthy"
    color: "#4A90D9"  # Optional: UI accent color

  - id: "proj-def456"
    path: "/home/user/projects/beta"
    name: "Beta Service"
    last_accessed: "2025-01-14T15:45:00Z"
    status: "healthy"

  - id: "proj-ghi789"
    path: "/home/user/projects/gamma"
    name: "Gamma API"
    last_accessed: "2025-01-10T09:00:00Z"
    status: "degraded"
    status_message: "Config file has validation errors"
```

### Project Status Values

| Status | Meaning | UI Indicator |
|--------|---------|--------------|
| `healthy` | Project fully operational | Green dot |
| `degraded` | Partial functionality (config issues) | Yellow warning |
| `offline` | Directory not accessible | Gray/red offline |
| `initializing` | First-time setup in progress | Spinner |

---

## Appendix D: Decision Tree for Implementation Approach

```
START: Multi-project requirement identified
    │
    ▼
┌─────────────────────────────────────────┐
│ Q1: Is timeline critical (<2 weeks)?    │
└─────────────────┬───────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
      [YES]               [NO]
        │                   │
        ▼                   ▼
┌───────────────┐   ┌─────────────────────────────┐
│ Alt A:        │   │ Q2: Is strong isolation     │
│ Multi-Server  │   │ required (multi-user)?      │
│ + WebUI       │   └─────────────┬───────────────┘
│ Endpoint      │                 │
│ Selection     │       ┌─────────┴─────────┐
└───────────────┘       │                   │
                      [YES]               [NO]
                        │                   │
                        ▼                   ▼
              ┌───────────────┐   ┌───────────────┐
              │ Alt B:        │   │ Q3: Is memory │
              │ In-Process    │   │ constrained?  │
              │ Multi-Project │   └───────┬───────┘
              │ (Recommended) │           │
              └───────────────┘   ┌───────┴───────┐
                                  │               │
                                [YES]           [NO]
                                  │               │
                                  ▼               ▼
                        ┌───────────────┐ ┌───────────────┐
                        │ Alt A:        │ │ Alt B:        │
                        │ Multi-Server  │ │ In-Process    │
                        └───────────────┘ └───────────────┘
```

**Default Recommendation**: Alternative B (In-Process Multi-Project) unless timeline or resource constraints dictate otherwise.
