# Architecture Documentation

## Overview

quorum-ai follows a **hexagonal architecture** (also known as Ports and Adapters) to achieve clear separation between business logic and external systems. This architecture enables:

- Independent testing of core business logic
- Easy replacement of external dependencies
- Clear boundaries between system layers
- Simplified reasoning about data flow

---

## Architecture Diagram

```mermaid
graph TB
    subgraph "Entry Points"
        CLI[CLI Commands<br/>Cobra]
        TUI[TUI<br/>Bubbletea]
        PLAIN[Plain Logger<br/>CI/CD fallback]
    end

    subgraph "Service Layer"
        WF[Workflow Runner]
        DAG[DAG Builder]
        CONS[Consensus Checker]
        PROMPT[Prompt Renderer]
        RETRY[Retry Policy]
        RATE[Rate Limiter]
        CKPT[Checkpoint Manager]
    end

    subgraph "Core Domain"
        ENTITIES[Entities<br/>Task, Workflow, Phase, Artifact]
        PORTS[Ports<br/>Agent, StateManager, GitClient]
        ERRORS[Domain Errors]
    end

    subgraph "Adapters"
        subgraph "CLI Adapters"
            CLAUDE[Claude Adapter]
            GEMINI[Gemini Adapter]
            CODEX[Codex Adapter]
        end
        subgraph "State Adapter"
            JSON[JSON State Manager]
            LOCK[Process Lock]
        end
        subgraph "Git Adapters"
            GIT[Git Client]
            WT[Worktree Manager]
            GH[GitHub Client]
        end
    end

    subgraph "External Systems"
        CLAUDE_CLI[claude CLI]
        GEMINI_CLI[gemini CLI]
        CODEX_CLI[codex CLI]
        FS[File System]
        GIT_BIN[git binary]
        GH_BIN[gh CLI]
    end

    CLI --> WF
    TUI --> WF
    PLAIN --> WF

    WF --> DAG
    WF --> CONS
    WF --> PROMPT
    WF --> RETRY
    WF --> RATE
    WF --> CKPT

    DAG --> ENTITIES
    CONS --> ENTITIES
    CKPT --> PORTS

    WF --> PORTS
    PORTS --> CLAUDE
    PORTS --> GEMINI
    PORTS --> CODEX
    PORTS --> JSON
    PORTS --> GIT

    CLAUDE --> CLAUDE_CLI
    GEMINI --> GEMINI_CLI
    CODEX --> CODEX_CLI
    JSON --> FS
    LOCK --> FS
    GIT --> GIT_BIN
    WT --> GIT_BIN
    GH --> GH_BIN
```

---

## Layer Responsibilities

### 1. Core Domain (`internal/core/`)

The innermost layer contains pure business logic with **zero external dependencies**.

Core responsibilities:

- **Entities**: Task, Workflow, Phase, Artifact
- **Ports**: Agent, StateManager, GitClient, GitHubClient
- **Errors**: domain-specific error categories and classifications

### 2. Service Layer (`internal/service/`)

Orchestrates business operations using core entities and ports.

Core responsibilities:

- **Workflow orchestration** across analyze, plan, and execute phases
- **Dependency management** with DAG construction and ready-task selection
- **Consensus evaluation** using Jaccard similarity and category weights
- **Prompt rendering** for phase-specific tasks
- **Resilience controls** (retry and rate limiting)
- **Checkpointing** for resume and recovery

**Workflow Runner Flow:**

```
1. Load or create workflow state
2. Acquire process lock
3. For each phase (Analyze -> Plan -> Execute):
   a. Build DAG for phase tasks
   b. Get ready tasks (no pending dependencies)
   c. Execute tasks in parallel (respecting rate limits)
   d. Evaluate consensus (if applicable)
   e. Handle divergences (V2/V3 if needed)
   f. Save checkpoint
4. Release lock
5. Generate report
```

**Independent Phase Execution:**

Phases can also run independently via dedicated commands (`quorum analyze`,
`quorum plan`, `quorum execute`). Each phase validates prerequisites:
- **analyze**: Creates new workflow state
- **plan**: Requires completed analysis (consolidated output)
- **execute**: Requires completed plan (task list in state)

This enables debugging, cost control, and recovery between phases.

### 3. Adapters (`internal/adapters/`)

Implement ports by wrapping external systems.

#### CLI Adapters (`internal/adapters/cli/`)

| Adapter | CLI Tool | Capabilities |
|---------|----------|-------------|
| `claude` | `claude` | Full analysis, planning, code generation |
| `gemini` | `gemini` | Analysis, validation |
| `codex` | `codex` | Code-focused tasks (optional) |
| `copilot` | `gh copilot` | GitHub-integrated tasks (optional, PTY required) |
| `aider` | `aider` | AI pair programming (optional) |

#### State Adapter (`internal/adapters/state/`)

Responsibilities:

- JSON-based persistence with atomic writes
- Process lock management with stale detection

#### Git Adapters (`internal/adapters/git/`)

Responsibilities:

- Git CLI wrapper for status, commit, and push
- Worktree lifecycle management (create, remove, cleanup)

#### GitHub Adapter (`internal/adapters/github/`)

**Status**: Implemented but not yet integrated into workflow runner.

Responsibilities:

- PR creation and issue management via `gh` CLI
- CI status polling and wait

Note: The adapter is complete and tested but requires manual instantiation.
Future work may integrate it into the workflow runner for automated PR creation.

### 4. Configuration (`internal/config/`)

Responsibilities:

- Configuration loading with defined precedence
- Validation rules and error messages
- Default values for all settings

**Configuration Precedence (highest to lowest):**

1. CLI flags
2. Environment variables (`QUORUM_*`)
3. Project config (`.quorum.yaml`)
4. User config (`~/.config/quorum/config.yaml`)
5. Built-in defaults

### 5. TUI (`internal/tui/`)

Responsibilities:

- Bubbletea model and update loop
- Rendering and styling
- Plain text fallback for non-TTY environments

### 6. Logging (`internal/logging/`)

Responsibilities:

- slog wrapper with context propagation
- Secret pattern matching and redaction

---

## Data Flow

### Complete Workflow Execution

```
User Input (prompt)
       |
       v
+------+-------+
|  CLI Parser  |  <- Cobra parses flags and args
+------+-------+
       |
       v
+------+-------+
| Config Load  |  <- Viper merges configs
+------+-------+
       |
       v
+------+-------+
| State Load   |  <- JSON state manager
+------+-------+
       |
       v
+------+-------+
| Lock Acquire |  <- Process lock (PID file)
+------+-------+
       |
       v
+------+-------+
| ANALYZE      |
| Phase        |
|  - V1: All   |  <- Parallel agent execution
|  - Consensus |  <- Jaccard calculation
|  - V2/V3?    |  <- Dialectic if needed
+------+-------+
       |
       v
+------+-------+
| PLAN Phase   |
|  - Generate  |  <- Agent creates plan
|  - Parse     |  <- Markdown to tasks
|  - DAG Build |  <- Dependency graph
+------+-------+
       |
       v
+------+-------+
| EXECUTE      |
| Phase        |
|  - Worktree  |  <- Isolated execution
|  - Tasks     |  <- Parallel where possible
|  - Validate  |  <- Test/lint checks
+------+-------+
       |
       v
+------+-------+
| State Save   |  <- Atomic write
+------+-------+
       |
       v
+------+-------+
| Lock Release |
+------+-------+
       |
       v
+------+-------+
| Report Gen   |  <- Metrics, summary
+------+-------+
```

---

## System Invariants

### State Invariants

1. **Atomic Persistence**: State is never partially written
2. **Lock Exclusivity**: Only one process can execute at a time
3. **Checkpoint Consistency**: Workflow can resume from any saved state
4. **Idempotent Tasks**: Re-running a completed task produces same result

### Execution Invariants

1. **Dependency Order**: Tasks execute only after dependencies complete
2. **Isolation**: Each task executes in its own git worktree
3. **Rollback Safety**: Failed tasks do not affect main branch
4. **Secret Protection**: Logs never contain API keys or tokens

### Consensus Invariants

1. **Score Range**: Consensus score is always in [0.0, 1.0]
2. **Threshold Gating**: Low consensus triggers escalation
3. **Human Override**: System never proceeds without human approval below 50%
4. **Deterministic**: Same inputs produce same consensus score

---

## Directory Structure

```
quorum-ai/
├── cmd/quorum/              # Entry point
│   ├── main.go              # Minimal main
│   └── cmd/                  # Cobra commands
│       ├── root.go          # Root command and global flags
│       ├── run.go           # Main workflow execution
│       ├── analyze.go       # Analysis phase only
│       ├── plan.go          # Planning phase only
│       ├── execute.go       # Execution phase only
│       ├── common.go        # Shared phase utilities
│       ├── status.go        # Workflow status inspection
│       ├── doctor.go        # Prerequisites validation
│       ├── init.go          # Configuration scaffolding
│       ├── trace.go         # Trace inspection
│       └── version.go       # Version information
│
├── internal/                 # Private packages
│   ├── core/                 # Domain layer
│   ├── service/              # Application layer
│   │   └── prompts/         # Prompt templates
│   ├── adapters/             # Infrastructure
│   │   ├── cli/
│   │   ├── state/
│   │   ├── git/
│   │   └── github/
│   ├── config/
│   ├── tui/
│   ├── logging/
│   └── testutil/
│
├── pkg/parser/               # Public packages
│
├── configs/                  # Config examples
├── testdata/                 # Test fixtures
└── docs/                     # Documentation
```

---

## Design Decisions

For detailed rationale behind architectural choices, see:

- [ADR-0001: Hexagonal Architecture](adr/0001-hexagonal-architecture.md)
- [ADR-0002: Consensus Protocol and Scoring](adr/0002-consensus-protocol.md)
- [ADR-0003: JSON State Persistence for POC](adr/0003-state-persistence-json.md)
- [ADR-0004: Worktree Isolation per Task](adr/0004-worktree-isolation.md)
- [ADR-0007: Multilingual Prompt Support](adr/0007-multilingual-prompt-optimization.md)

---

## References

- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/) - Alistair Cockburn
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Effective Go](https://go.dev/doc/effective_go)
