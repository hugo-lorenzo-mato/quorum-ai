# Viability Analysis: Multi-Tenant/Multi-Project Management in Quorum AI

## Executive Summary

This document provides an exhaustive technical analysis of implementing multi-tenant/multi-project management capabilities in Quorum AI. After thorough examination of the codebase, **the implementation is technically feasible but requires significant architectural changes across all layers of the application**. The current architecture is fundamentally designed around a single-project paradigm, with the working directory (`os.Getwd()`) serving as the implicit project context throughout the entire system.

**Key Finding**: The primary challenge is not a single blocking technical barrier, but rather the pervasive assumption of a single project context that manifests across ~50+ locations in the codebase, requiring coordinated changes across CLI, TUI, WebUI, backend services, and persistence layers.

---

## 1. Architecture Analysis: Current State

### 1.1 Project Structure Overview

```
quorum-ai/
├── cmd/quorum/cmd/       # CLI commands (Cobra)
│   ├── root.go          # Config loading, working dir assumption
│   ├── serve.go         # Web server startup
│   ├── chat.go          # TUI chat mode
│   └── init.go          # Project initialization
├── frontend/            # React WebUI (Zustand state management)
│   └── src/
│       ├── stores/      # Global state stores
│       ├── hooks/       # SSE, config hooks
│       └── lib/         # API clients
├── internal/
│   ├── api/             # HTTP REST API handlers
│   ├── adapters/        # State, CLI, Git, Chat adapters
│   ├── config/          # Configuration loader
│   ├── core/            # Domain entities and interfaces
│   └── service/         # Business logic (workflow runner)
└── .quorum/             # Project-specific data directory
    ├── config.yaml      # Project configuration
    ├── state/           # Workflow state (SQLite/JSON)
    ├── runs/            # Execution reports
    ├── attachments/     # File attachments
    └── traces/          # Debug traces
```

### 1.2 Current Project Context Flow

The current system determines project context implicitly through the working directory:

```
+------------------+     +-------------------+     +------------------+
|  CLI Command     |---->|  os.Getwd()       |---->| .quorum/         |
|  (cd to project) |     |  Working Dir      |     | config.yaml      |
+------------------+     +-------------------+     +------------------+
                                |
                                v
                         +-------------------+
                         | Config Loader     |
                         | (Viper)           |
                         +-------------------+
                                |
                                v
                         +-------------------+
                         | State Manager     |
                         | (SQLite/JSON)     |
                         +-------------------+
                                |
                                v
                         +-------------------+
                         | All Services      |
                         | (Single context)  |
                         +-------------------+
```

### 1.3 Critical Code Evidence: Single-Project Assumptions

#### 1.3.1 Configuration Loading (`internal/config/loader.go:75-91`)

```go
// Try new location first: .quorum/config.yaml
newConfigPath := filepath.Join(".quorum", "config.yaml")
if _, err := os.Stat(newConfigPath); err == nil {
    l.v.SetConfigFile(newConfigPath)
} else {
    // Fall back to legacy location: .quorum.yaml
    l.v.SetConfigName(".quorum")
    l.v.SetConfigType("yaml")
    l.v.AddConfigPath(".")
}
```

**Evidence**: Config loading uses relative paths (`.quorum/`) assuming the current working directory IS the project root.

#### 1.3.2 Project Initialization (`cmd/quorum/cmd/init.go:30-85`)

```go
func runInit(_ *cobra.Command, _ []string) error {
    cwd, err := os.Getwd()
    // ...
    quorumDir := filepath.Join(cwd, ".quorum")
    configPath := filepath.Join(quorumDir, "config.yaml")
    // Creates: .quorum/, .quorum/state/, .quorum/logs/, .quorum/runs/
}
```

**Evidence**: Project initialization hardcodes `.quorum/` relative to `cwd`.

#### 1.3.3 State Manager Paths (`internal/config/loader.go:224-226`)

```go
l.v.SetDefault("state.path", ".quorum/state/state.db")
l.v.SetDefault("state.backup_path", ".quorum/state/state.db.bak")
```

**Evidence**: Default state paths are relative, assuming single project context.

#### 1.3.4 Web Server Working Directory (`cmd/quorum/cmd/serve.go:97-112`)

```go
func runServe(_ *cobra.Command, _ []string) error {
    // Config loads from CWD
    loader := config.NewLoaderWithViper(viper.GetViper())
    quorumCfg, err := loader.Load()

    // State manager uses config paths (relative to CWD)
    statePath := quorumCfg.State.Path  // ".quorum/state/state.db"
    stateManager, err := state.NewStateManager(backend, statePath)
}
```

**Evidence**: The web server assumes it's started from the project directory and all paths are relative.

#### 1.3.5 API Server Root Directory (`internal/api/server.go:132-138`)

```go
func NewServer(stateManager core.StateManager, eventBus *events.EventBus, opts ...ServerOption) *Server {
    wd, _ := os.Getwd() // Best effort default
    s := &Server{
        // ...
        root: wd,
    }
}
```

**Evidence**: API server captures `cwd` at startup for file operations.

#### 1.3.6 Attachments Storage (`internal/attachments/store.go:39`)

```go
baseDir := filepath.Join(root, ".quorum", "attachments")
```

**Evidence**: Attachments stored in project's `.quorum/attachments/`.

#### 1.3.7 Workflow Execution (`internal/service/workflow/runner.go:1476-1477`)

```go
cwd, _ := os.Getwd()
outputDir = filepath.Join(cwd, outputDir)
```

**Evidence**: Workflow runner uses `cwd` for output paths.

#### 1.3.8 Git Worktree Creation (`cmd/quorum/cmd/chat.go:405-409`)

```go
cwd, err := os.Getwd()
if err != nil {
    return nil, fmt.Errorf("getting working directory: %w", err)
}
gitClient, err := git.NewClient(cwd)
```

**Evidence**: Git operations bound to current working directory.

### 1.4 Frontend Architecture Analysis

#### 1.4.1 State Management (Zustand Stores)

```
frontend/src/stores/
├── index.js          # Export aggregator
├── configStore.js    # Configuration state (SINGLE instance)
├── workflowStore.js  # Workflow state (SINGLE instance)
├── taskStore.js      # Task state
├── chatStore.js      # Chat state
├── agentStore.js     # Agent state
├── uiStore.js        # UI preferences (persisted to localStorage)
└── kanbanStore.js    # Kanban board state
```

**Critical Finding** (`frontend/src/stores/workflowStore.js:1-14`):

```javascript
const useWorkflowStore = create((set, get) => ({
  // State - GLOBAL, not per-project
  workflows: [],        // All workflows - assumed from SINGLE project
  activeWorkflow: null, // Currently active workflow
  selectedWorkflowId: null,
  tasks: {},
  loading: false,
  error: null,
}));
```

**Evidence**: All stores are global singletons with no project/tenant context. The `workflows` array contains workflows from the current project only (backend enforces this via file paths).

#### 1.4.2 API Client (`frontend/src/lib/api.js:1-27`)

```javascript
const API_BASE = '/api/v1';

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  // ... no project context parameter
}
```

**Evidence**: API calls have no project context; the backend determines context from its working directory.

#### 1.4.3 SSE Connection (`frontend/src/hooks/useSSE.js:9-19`)

```javascript
const SSE_URL = '/api/v1/sse/events';
// Single global connection, no project filtering
```

**Evidence**: Single SSE stream for all events; no multi-project event isolation.

#### 1.4.4 UI Store Persistence (`frontend/src/stores/uiStore.js:106-113`)

```javascript
persist(
  (set, get) => ({ /* ... */ }),
  {
    name: 'quorum-ui-store',  // SINGLE key in localStorage
    partialize: (state) => ({
      sidebarOpen: state.sidebarOpen,
      theme: state.theme,
    }),
  }
)
```

**Evidence**: UI preferences stored globally, not per-project.

### 1.5 Backend Service Architecture

#### 1.5.1 State Manager Interface (`internal/core/ports.go:20-60`)

```go
// StateManager handles workflow state persistence.
type StateManager interface {
    Save(ctx context.Context, state *WorkflowState) error
    Load(ctx context.Context) (*WorkflowState, error)
    LoadByID(ctx context.Context, id WorkflowID) (*WorkflowState, error)
    ListWorkflows(ctx context.Context) ([]WorkflowSummary, error)
    // ... no project/tenant parameter
}
```

**Evidence**: Interface assumes single project; no project ID parameter.

#### 1.5.2 SQLite State Manager (`internal/adapters/state/sqlite.go:72-133`)

```go
func NewSQLiteStateManager(dbPath string, opts ...SQLiteStateManagerOption) (*SQLiteStateManager, error) {
    // dbPath is typically ".quorum/state/state.db" - project-specific
    m := &SQLiteStateManager{
        dbPath:     dbPath,
        backupPath: dbPath + ".bak",
        lockPath:   dbPath + ".lock",
    }
}
```

**Evidence**: Database path hardcoded per instance; one DB = one project.

#### 1.5.3 Chat Session Scoping (`internal/adapters/web/chat.go:103, 223`)

```go
type activeChatState struct {
    // ...
    projectRoot string // Directory where .quorum is located, for file access scoping
}

// In CreateSession:
projectRoot, _ := os.Getwd()
```

**Evidence**: Chat sessions capture `projectRoot` at creation time.

---

## 2. Multi-Tenant Architecture: Detailed Requirements

### 2.1 Conceptual Model

For multi-tenant support, the system would need to manage multiple isolated contexts:

```
+---------------------------+
|       Quorum UI           |
|  [Project Selector: ▼]    |
+---------------------------+
            |
            v
+---------------------------+
|    Project Registry       |
| +-----------------------+ |
| | Project A             | |
| | path: /home/user/projA| |
| | config: {...}         | |
| +-----------------------+ |
| | Project B             | |
| | path: /home/user/projB| |
| | config: {...}         | |
| +-----------------------+ |
+---------------------------+
            |
            v (per-project)
+---------------------------+
|   Isolated Contexts       |
| - State Manager           |
| - Config                  |
| - Workflows               |
| - Attachments             |
| - SSE Events              |
+---------------------------+
```

### 2.2 Data Isolation Requirements

| Component | Isolation Level | Current | Required |
|-----------|----------------|---------|----------|
| Config | Per-project | Implicit (cwd) | Explicit path |
| State DB | Per-project | Implicit (cwd) | Explicit path |
| Workflows | Per-project | Implicit | Explicit project_id |
| Tasks | Per-workflow | Yes | No change |
| Attachments | Per-project | Implicit | Explicit path |
| Chat Sessions | Per-project | Implicit | Explicit project_id |
| SSE Events | Global | Yes | Per-project filtering |
| UI Preferences | Global | Yes | Optional per-project |

---

## 3. Pros and Cons Analysis

### 3.1 Benefits Matrix

| Benefit | User Type | Impact | Evidence |
|---------|-----------|--------|----------|
| Single entry point for all projects | Power users | High | Eliminates need to switch terminals/directories |
| Unified workflow overview | Team leads | High | Cross-project visibility |
| Shared agent configuration | All users | Medium | Configure once, use everywhere |
| Consistent UI state | All users | Medium | Theme/preferences persist across projects |
| Parallel project monitoring | DevOps | High | Monitor multiple codebases simultaneously |
| Reduced context switching | Developers | High | No `cd` commands to switch projects |

### 3.2 Risks and Challenges

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Data cross-contamination | Medium | Critical | Strict path isolation, project_id validation |
| Memory consumption | High | Medium | Lazy loading, LRU cache for inactive projects |
| SSE event storms | Medium | High | Project-scoped event streams |
| Configuration conflicts | Low | Medium | Per-project config isolation |
| Migration complexity | High | High | Phased rollout, backward compatibility |
| Regression in existing flows | High | High | Comprehensive integration tests |
| Security (path traversal) | Medium | Critical | Strict path validation, sandboxing |
| Performance degradation | Medium | Medium | Connection pooling, efficient state queries |

### 3.3 Implementation Complexity Assessment

| Component | Complexity | LOC Impact | Files Affected |
|-----------|------------|------------|----------------|
| Backend API | High | ~1500 | 15+ |
| State Management | High | ~800 | 8 |
| Frontend Stores | High | ~600 | 7 |
| Config System | Medium | ~400 | 5 |
| SSE System | Medium | ~300 | 4 |
| CLI/TUI | Low | ~200 | 10 |
| Tests | High | ~2000 | 20+ |
| **Total** | **High** | **~5800** | **~70** |

---

## 4. Technical Difficulties Analysis

### 4.1 Difficulty #1: State Manager Refactoring

**Description**: The `StateManager` interface and all implementations assume a single project context. Database paths are determined at construction time, not per-operation.

**Files Affected**:
- `internal/core/ports.go:20-60` - Interface definition
- `internal/adapters/state/sqlite.go` - SQLite implementation
- `internal/adapters/state/json.go` - JSON file implementation
- `internal/adapters/state/factory.go` - Factory methods

**Current Code** (`internal/core/ports.go:20-35`):
```go
type StateManager interface {
    Save(ctx context.Context, state *WorkflowState) error
    Load(ctx context.Context) (*WorkflowState, error)
    // No project context
}
```

**Solutions**:

| Solution | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| A) Add projectID parameter to all methods | Simple, explicit | Breaking change, verbose | Not recommended |
| B) Create StateManagerFactory per request | Isolation, clean | Connection overhead | For JSON backend |
| C) Connection pool with project routing | Efficient, scalable | Complex implementation | Recommended for SQLite |
| D) Single DB with project_id column | Simple schema | Data leak risk | Not recommended |

**Recommended Approach (C)**: Implement a `ProjectStateManagerPool` that maintains connections per project:

```go
type ProjectStateManagerPool struct {
    managers sync.Map // map[projectPath]*SQLiteStateManager
    maxIdle  int
    ttl      time.Duration
}

func (p *ProjectStateManagerPool) ForProject(projectPath string) (StateManager, error) {
    // Return cached or create new manager for project
}
```

### 4.2 Difficulty #2: SSE Event Isolation

**Description**: SSE events are broadcast globally. With multiple projects, users would receive events from all active projects, causing confusion and performance issues.

**Files Affected**:
- `internal/events/eventbus.go` - Event publishing
- `internal/api/sse.go` - SSE handler
- `frontend/src/hooks/useSSE.js` - Client subscription

**Current Flow**:
```
Backend Event → EventBus.Publish() → All SSE Clients
```

**Required Flow**:
```
Backend Event (with projectID) → EventBus.Publish() → Filter by client's active project → SSE Client
```

**Solutions**:

| Solution | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| A) Project-specific SSE endpoints | Clean isolation | Multiple connections | For few projects |
| B) Client-side filtering | Simple backend | Wasted bandwidth | Not recommended |
| C) Server-side filtering with topics | Efficient, scalable | Complex subscription | Recommended |

**Recommended Approach (C)**: Topic-based event routing:

```go
// Event with project context
type Event struct {
    Type      EventType
    ProjectID string    // New field
    Data      interface{}
}

// Client subscription with filter
type SSEClient struct {
    Projects []string // Active project IDs
}
```

### 4.3 Difficulty #3: Frontend State Architecture

**Description**: All Zustand stores are global singletons. Multi-project support requires either multiple store instances or project-scoped state partitions.

**Files Affected**:
- `frontend/src/stores/workflowStore.js`
- `frontend/src/stores/configStore.js`
- `frontend/src/stores/taskStore.js`
- `frontend/src/stores/chatStore.js`
- All components consuming stores

**Solutions**:

| Solution | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| A) Store per project (dynamic) | Complete isolation | Complex lifecycle | Not recommended |
| B) Partitioned state by projectID | Single store, clear structure | Migration effort | Recommended |
| C) Context-based store selection | React-friendly | Breaking change | Alternative |

**Recommended Approach (B)**: Partition state by project ID:

```javascript
const useWorkflowStore = create((set, get) => ({
  // Keyed by projectID
  projectStates: {}, // { [projectID]: { workflows, activeWorkflow, ... } }
  activeProjectId: null,

  // Actions scoped to active project
  fetchWorkflows: async () => {
    const projectId = get().activeProjectId;
    const workflows = await workflowApi.list(projectId);
    set(state => ({
      projectStates: {
        ...state.projectStates,
        [projectId]: { ...state.projectStates[projectId], workflows }
      }
    }));
  },
}));
```

### 4.4 Difficulty #4: Configuration Isolation

**Description**: Each project has its own `.quorum/config.yaml` with potentially different agent configurations, timeouts, and settings.

**Files Affected**:
- `internal/config/loader.go` - Config loading
- `cmd/quorum/cmd/serve.go` - Server startup
- `frontend/src/stores/configStore.js` - Config state

**Current Code** (`internal/config/loader.go:62-98`):
```go
func (l *Loader) Load() (*Config, error) {
    // Uses .quorum/config.yaml from CWD
    newConfigPath := filepath.Join(".quorum", "config.yaml")
}
```

**Solutions**:

| Solution | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| A) Absolute path parameter | Simple, explicit | Breaking change | Recommended |
| B) Config cache per project | Efficient | Staleness issues | Combine with A |
| C) Merged global + project config | Flexibility | Complexity | Future enhancement |

**Recommended Approach (A+B)**: Accept explicit path with caching:

```go
type MultiProjectConfigLoader struct {
    cache map[string]*Config
    mu    sync.RWMutex
}

func (l *MultiProjectConfigLoader) LoadProject(projectPath string) (*Config, error) {
    configPath := filepath.Join(projectPath, ".quorum", "config.yaml")
    // ... load and cache
}
```

### 4.5 Difficulty #5: Project Discovery and Registry

**Description**: The system needs a mechanism to discover and track initialized quorum projects.

**Options**:
1. **Manual Registration**: Users explicitly add projects
2. **Directory Scanning**: Scan common paths for `.quorum/` directories
3. **Recent Projects**: Track recently accessed projects
4. **Filesystem Watch**: Monitor for new project initializations

**Recommended Approach**: Hybrid (1 + 3):
- Store registered projects in `~/.config/quorum/projects.json`
- Auto-add projects on `quorum serve` or `quorum init`
- Allow manual addition/removal via UI

```json
// ~/.config/quorum/projects.json
{
  "projects": [
    {
      "id": "proj-abc123",
      "name": "my-web-app",
      "path": "/home/user/projects/my-web-app",
      "lastAccessed": "2026-02-02T12:00:00Z"
    }
  ],
  "defaultProject": "proj-abc123"
}
```

### 4.6 Difficulty #6: Git Worktree Isolation

**Description**: Git operations (worktrees, commits) are tied to the repository in the working directory.

**Files Affected**:
- `internal/adapters/git/client.go`
- `internal/adapters/git/worktree.go`
- `internal/service/workflow/builder.go:642-645`

**Current Code**:
```go
cwd, err := os.Getwd()
gitClient, err := git.NewClient(cwd)
```

**Solution**: Pass explicit project path:
```go
func (r *Runner) createGitClient(projectPath string) (core.GitClient, error) {
    return git.NewClient(projectPath)
}
```

---

## 5. Alternative Architectures

### 5.1 Alternative A: In-Process Multi-Project (Recommended)

**Description**: Extend the existing single-server architecture to handle multiple projects within the same process.

```
+--------------------------------------------------+
|                  Quorum Server                    |
|  +--------------------------------------------+  |
|  |        Project Context Manager             |  |
|  |  +---------+  +---------+  +---------+     |  |
|  |  |Proj A   |  |Proj B   |  |Proj C   |     |  |
|  |  |StateMgr |  |StateMgr |  |StateMgr |     |  |
|  |  |Config   |  |Config   |  |Config   |     |  |
|  |  |GitClient|  |GitClient|  |GitClient|     |  |
|  |  +---------+  +---------+  +---------+     |  |
|  +--------------------------------------------+  |
|                       |                          |
|  +--------------------------------------------+  |
|  |              API Router                    |  |
|  |  /api/v1/projects/{projectID}/workflows    |  |
|  |  /api/v1/projects/{projectID}/config       |  |
|  +--------------------------------------------+  |
|                       |                          |
|  +--------------------------------------------+  |
|  |            SSE Event Hub                   |  |
|  |  (project-scoped event streams)            |  |
|  +--------------------------------------------+  |
+--------------------------------------------------+
```

**API Changes**:
```
Current:  GET /api/v1/workflows
Proposed: GET /api/v1/projects/{projectID}/workflows
          GET /api/v1/workflows?projectId={projectID}
```

**Frontend Changes**:
- Add ProjectSelector component to header
- Modify all stores to be project-scoped
- Update SSE hook to filter by project

**Pros**:
- Single process, efficient resource sharing
- Existing code largely reusable
- Gradual migration possible

**Cons**:
- Memory growth with many projects
- Single point of failure
- Must handle project lifecycle carefully

**Estimated Complexity**: Medium-High

### 5.2 Alternative B: Proxy Architecture

**Description**: A lightweight central proxy that routes requests to project-specific backend instances.

```
+----------------------+     +----------------------+
|   Quorum Hub (UI)    |     |   Project Registry   |
|  [Project Selector]  |<--->|   (JSON/SQLite)      |
+----------------------+     +----------------------+
           |
           v
+----------------------+
|      API Proxy       |
| (routes by projectID)|
+----------------------+
     |         |         |
     v         v         v
+--------+ +--------+ +--------+
|Server A| |Server B| |Server C|
|Proj A  | |Proj B  | |Proj C  |
|(port X)| |(port Y)| |(port Z)|
+--------+ +--------+ +--------+
```

**Pros**:
- Complete isolation between projects
- Each project has independent resources
- Failure isolation

**Cons**:
- Resource overhead (multiple processes)
- Complex orchestration
- Port management complexity
- Harder to implement unified UI

**Estimated Complexity**: High

### 5.3 Alternative C: Browser-Based Multi-Tab

**Description**: Keep the current single-project architecture but open multiple browser tabs/windows for different projects.

```
+---------------+  +---------------+  +---------------+
| Browser Tab 1 |  | Browser Tab 2 |  | Browser Tab 3 |
| Project A     |  | Project B     |  | Project C     |
| :8080         |  | :8081         |  | :8082         |
+---------------+  +---------------+  +---------------+
       |                 |                 |
       v                 v                 v
+---------------+  +---------------+  +---------------+
| quorum serve  |  | quorum serve  |  | quorum serve  |
| (Project A)   |  | (Project B)   |  | (Project C)   |
+---------------+  +---------------+  +---------------+
```

**Pros**:
- Zero changes to existing code
- Users can already do this today
- Simple implementation (document feature)

**Cons**:
- Poor user experience
- Multiple processes running
- No unified view
- Manual port management

**Estimated Complexity**: None (documentation only)

### 5.4 Comparison Matrix

| Criteria | Alt A (In-Process) | Alt B (Proxy) | Alt C (Multi-Tab) |
|----------|-------------------|---------------|-------------------|
| Implementation Effort | High | Very High | None |
| User Experience | Excellent | Good | Poor |
| Resource Efficiency | Good | Poor | Poor |
| Isolation | Good | Excellent | Excellent |
| Unified UI | Yes | Yes | No |
| Backward Compatible | Yes | Yes | Yes |
| Scalability | Medium | High | Low |
| **Recommendation** | **Primary** | Future option | Document existing |

---

## 6. UI/UX Considerations

### 6.1 Project Selector Component Options

#### Option 1: Dropdown in Header (Recommended)

```
+------------------------------------------------------------------+
| [Quorum Logo]    [Home] [Workflows] [Chat]    [Project: ▼ My App] |
+------------------------------------------------------------------+
                                                     |
                                                     v
                                              +----------------+
                                              | > My App       |
                                              |   Backend API  |
                                              |   Frontend     |
                                              |----------------+
                                              | + Add Project  |
                                              +----------------+
```

**Pros**: Minimal UI change, always visible
**Cons**: Limited space for project names

#### Option 2: Sidebar Section

```
+------------------+--------------------------------------------+
| PROJECTS         |                                            |
| > My App         |           [Page Content]                   |
|   Backend API    |                                            |
|   Frontend       |                                            |
|                  |                                            |
| + Add Project    |                                            |
+------------------+--------------------------------------------+
```

**Pros**: More room for project list
**Cons**: Takes sidebar space

#### Option 3: Command Palette (Cmd+K)

```
+------------------------------------------------------------------+
|                    Switch Project (Cmd+K)                        |
|  +------------------------------------------------------------+  |
|  | Search projects...                                         |  |
|  +------------------------------------------------------------+  |
|  |  My App         ~/projects/my-app                          |  |
|  |  Backend API    ~/projects/backend                         |  |
|  |  Frontend       ~/projects/frontend                        |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
```

**Pros**: Power-user friendly, doesn't take space
**Cons**: Discovery issue for new users

**Recommendation**: Implement Option 1 as primary, with Option 3 as enhancement.

### 6.2 Context Switch Behavior

When switching projects, the UI should:

1. **Save current state**: Preserve unsaved form data with warning
2. **Show loading indicator**: During new project context load
3. **Preserve UI preferences**: Theme, sidebar state (global)
4. **Clear project state**: Workflows, tasks, chat (project-specific)
5. **Reconnect SSE**: With new project filter

```javascript
async function switchProject(newProjectId) {
  const { isDirty } = useConfigStore.getState();

  if (isDirty) {
    const confirmed = await showConfirmDialog(
      'You have unsaved changes. Switch anyway?'
    );
    if (!confirmed) return;
  }

  // Clear project-specific state
  useWorkflowStore.getState().clearForProject();
  useTaskStore.getState().clearForProject();

  // Set new project
  useProjectStore.getState().setActiveProject(newProjectId);

  // Reconnect SSE with new project filter
  reconnectSSE(newProjectId);

  // Load new project data
  await Promise.all([
    useWorkflowStore.getState().fetchWorkflows(),
    useConfigStore.getState().loadConfig(),
  ]);
}
```

---

## 7. Security Considerations

### 7.1 Path Traversal Prevention

**Risk**: Malicious project paths could access unauthorized files.

**Current Mitigation** (`internal/adapters/web/chat.go:793-818`):
```go
// Uses os.OpenRoot for sandboxing
root, err := os.OpenRoot(absProjectRoot)
// ...
relPath, err := filepath.Rel(absProjectRoot, absPath)
if strings.HasPrefix(relPath, "..") {
    return "", fmt.Errorf("path escapes project root")
}
```

**Additional Mitigations Required**:
1. Validate all project paths against allowlist
2. Use `filepath.Clean()` on all paths
3. Reject symlinks that escape project root
4. Sandbox file operations per project

### 7.2 Cross-Project Data Access

**Risk**: API requests could access data from unauthorized projects.

**Mitigation**:
```go
func (s *Server) validateProjectAccess(r *http.Request, projectID string) error {
    // Get user's allowed projects from session
    allowedProjects := getUserProjects(r.Context())

    if !contains(allowedProjects, projectID) {
        return ErrUnauthorized
    }
    return nil
}
```

### 7.3 SSE Event Leakage

**Risk**: Events from one project leaking to another.

**Mitigation**:
- Server-side filtering based on project subscription
- Never include sensitive data in events
- Project ID validation on every event

---

## 8. Implementation Plan (Conceptual)

### Phase 1: Foundation (Backend Core)

**Objective**: Create project-aware backend infrastructure without breaking existing functionality.

**Components**:
1. **Project Registry Service** (`internal/projects/registry.go`)
   - Store project metadata in `~/.config/quorum/projects.json`
   - CRUD operations for projects
   - Project validation (check `.quorum/` exists)

2. **Multi-Project Config Loader** (`internal/config/multiloader.go`)
   - Load config for specific project path
   - Cache configs with TTL

3. **State Manager Pool** (`internal/adapters/state/pool.go`)
   - Pool of state managers per project
   - Connection lifecycle management

**Files to Create**:
- `internal/projects/registry.go` (new)
- `internal/projects/discovery.go` (new)
- `internal/config/multiloader.go` (new)
- `internal/adapters/state/pool.go` (new)

**Files to Modify**:
- `cmd/quorum/cmd/init.go` - Register project on init
- `cmd/quorum/cmd/serve.go` - Initialize project pool

### Phase 2: API Layer

**Objective**: Add project context to API endpoints.

**API Routes**:
```
GET    /api/v1/projects                    # List registered projects
POST   /api/v1/projects                    # Register new project
DELETE /api/v1/projects/{id}               # Unregister project
GET    /api/v1/projects/{id}/workflows     # List workflows for project
# ... mirror all existing endpoints under /projects/{id}/
```

**Backward Compatibility**:
- Keep existing `/api/v1/workflows` working
- Route to default project (from config or first registered)

**Files to Create**:
- `internal/api/projects.go` (new)

**Files to Modify**:
- `internal/api/server.go` - Add project routes
- `internal/api/workflows.go` - Add project context
- `internal/api/config.go` - Add project context
- `internal/api/sse.go` - Add project filtering

### Phase 3: SSE Event Isolation

**Objective**: Enable project-scoped event streams.

**Changes**:
1. Add `ProjectID` to `Event` struct
2. Modify SSE handler to accept project filter
3. Update event publishers to include project context

**Files to Modify**:
- `internal/events/eventbus.go`
- `internal/api/sse.go`

### Phase 4: Frontend Integration

**Objective**: Add project management UI.

**New Components**:
- `ProjectSelector.jsx` - Dropdown component
- `ProjectSettings.jsx` - Project management page
- `AddProjectModal.jsx` - Register new project

**Store Changes**:
- Create `projectStore.js` - Active project, project list
- Modify all stores to use active project context
- Update API client to include project ID

**Files to Create**:
- `frontend/src/stores/projectStore.js`
- `frontend/src/components/ProjectSelector.jsx`
- `frontend/src/pages/ProjectSettings.jsx`

**Files to Modify**:
- `frontend/src/components/Layout.jsx` - Add ProjectSelector
- `frontend/src/stores/workflowStore.js` - Project scoping
- `frontend/src/stores/configStore.js` - Project scoping
- `frontend/src/hooks/useSSE.js` - Project filtering
- `frontend/src/lib/api.js` - Add project parameter

### Phase 5: CLI/TUI Enhancements (Optional)

**Objective**: Add multi-project awareness to CLI/TUI.

**New Commands**:
```bash
quorum projects list        # List registered projects
quorum projects add <path>  # Register project
quorum projects remove <id> # Unregister project
quorum projects switch <id> # Set default project
```

**TUI Changes**:
- Add project switcher to TUI header
- Project indicator in status bar

---

## 9. Conclusion and Recommendation

### 9.1 Viability Assessment

| Aspect | Assessment |
|--------|------------|
| **Technical Feasibility** | Yes, with significant effort |
| **Architectural Compatibility** | Requires refactoring, not rewrite |
| **Risk Level** | Medium-High (many touch points) |
| **Value Proposition** | High for power users and teams |
| **Maintenance Impact** | Moderate (adds complexity) |

### 9.2 Recommended Approach

**Implement Alternative A (In-Process Multi-Project)** using the phased approach:

1. **Start with backend foundation** (Phase 1) - Low risk, high value
2. **Add API support** (Phase 2) - Enables frontend work
3. **Isolate SSE events** (Phase 3) - Critical for UX
4. **Build frontend UI** (Phase 4) - User-facing value
5. **Enhance CLI/TUI** (Phase 5) - Optional, low priority

### 9.3 Key Success Criteria

1. **Zero data cross-contamination** - Verified by integration tests
2. **Backward compatibility** - Existing single-project users unaffected
3. **Responsive project switching** - < 500ms context switch
4. **Memory efficiency** - < 50MB overhead per inactive project
5. **SSE isolation** - No event leakage between projects

### 9.4 Risk Mitigation

1. **Comprehensive test suite** before starting
2. **Feature flag** to enable/disable multi-project
3. **Gradual rollout** - opt-in initially
4. **Monitoring** for memory and performance
5. **Clear migration path** documentation

---

## Appendix A: File Reference Summary

| File | Purpose | Multi-Tenant Impact |
|------|---------|---------------------|
| `internal/config/loader.go` | Config loading | Add project path parameter |
| `internal/adapters/state/sqlite.go` | State persistence | Pool per project |
| `internal/api/server.go` | HTTP API | Add project routes |
| `internal/api/sse.go` | SSE streaming | Add project filtering |
| `internal/events/eventbus.go` | Event system | Add project ID |
| `frontend/src/stores/workflowStore.js` | Workflow state | Partition by project |
| `frontend/src/stores/configStore.js` | Config state | Partition by project |
| `frontend/src/hooks/useSSE.js` | SSE client | Project subscription |
| `frontend/src/lib/api.js` | API client | Add project parameter |
| `cmd/quorum/cmd/serve.go` | Server startup | Initialize project pool |
| `cmd/quorum/cmd/init.go` | Project init | Register project |

---

## Appendix B: Glossary

- **Project**: A directory containing `.quorum/` configuration and state
- **Tenant**: In this context, synonymous with Project
- **Context**: The currently active project's configuration and state
- **State Manager**: Interface for persisting workflow state
- **SSE**: Server-Sent Events for real-time updates
- **Zustand**: Frontend state management library
