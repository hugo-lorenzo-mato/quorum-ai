# quorum-ai

[![CI](https://github.com/hugo-lorenzo-mato/quorum-ai/actions/workflows/test.yml/badge.svg)](https://github.com/hugo-lorenzo-mato/quorum-ai/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/hugo-lorenzo-mato/quorum-ai/branch/main/graph/badge.svg)](https://codecov.io/gh/hugo-lorenzo-mato/quorum-ai)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Multi-agent LLM orchestrator with consensus-based validation for reliable software engineering workflows.**

quorum-ai reduces LLM hallucinations and increases output reliability by running multiple autonomous agents in parallel and validating their outputs through a dialectic consensus protocol.

---

## Features

- **Multi-Agent Execution**: Orchestrate Claude, Gemini, and other CLI-based LLM agents in parallel
- **Consensus Validation**: Jaccard similarity algorithm measures agreement across agent outputs
- **Dialectic Protocol**: V1/V2/V3 (Thesis-Antithesis-Synthesis) process refines divergent outputs
- **Git Worktree Isolation**: Each task executes in isolated worktrees to prevent conflicts
- **Resume from Checkpoint**: Recover from failures without re-running completed work
- **Cost Tracking**: Monitor token usage and costs across all agents
- **Secret Sanitization**: Multi-pattern regex ensures API keys never appear in logs
- **Trace Mode**: Optional file-based traces for prompts, outputs, and consensus decisions

---

## Quick Start

### Prerequisites

- Go 1.22 or later
- Git 2.20 or later
- At least one LLM CLI installed:
  - [Claude Code](https://github.com/anthropics/claude-code) (recommended)
  - [Gemini CLI](https://github.com/google/gemini-cli)

### Installation

Download a prebuilt binary from the releases page and place it in your PATH:

- https://github.com/hugo-lorenzo-mato/quorum-ai/releases

Or build from source:

```bash
# Using go install
go install github.com/hugo-lorenzo-mato/quorum-ai/cmd/quorum@latest

# Or clone and build
git clone https://github.com/hugo-lorenzo-mato/quorum-ai.git
cd quorum-ai
make build
```

### Configuration

Create a configuration file at `.quorum.yaml` in your project root:

```yaml
# Minimal configuration
agents:
  claude:
    enabled: true
  gemini:
    enabled: true

consensus:
  threshold: 0.80

log:
  level: info
```

### Usage

```bash
# Verify prerequisites
quorum doctor

# Run full workflow (analyze -> plan -> execute)
quorum run "Implement user authentication with JWT tokens"

# Check workflow status
quorum status

# Enable trace output (summary or full)
quorum run --trace "Add a CLI flag to validate configs"
quorum run --trace=full "Refactor the payment processing module"

# Inspect trace runs
quorum trace --list
quorum trace --run-id wf-1234-1700000000
```

### Trace artifacts

When trace mode is enabled, artifacts are written to `.quorum/traces/<run_id>/`:

- `run.json`: run manifest (config snapshot, git/app metadata, summary).
- `trace.jsonl`: ordered trace events (phase, model, tokens, hashes).
- `*.txt` / `*.json`: prompt/response payloads (full mode only).

Trace modes:
- `summary`: only `run.json` and `trace.jsonl` (no prompt/response files).
- `full`: includes prompt/response files for each step (subject to size limits).

Example `trace.jsonl` entry:
```json
{"seq":1,"ts":"2026-01-13T00:00:00Z","event_type":"prompt","phase":"analyze","step":"v1","agent":"claude","model":"claude-sonnet-4-20250514","tokens_in":120,"tokens_out":0,"cost_usd":0.0023,"hash_raw":"...","hash_stored":"..."}
```

Example `run.json` (trimmed):
```json
{
  "run_id": "wf-1234-1700000000-1700000000",
  "workflow_id": "wf-1234-1700000000",
  "prompt_length": 120,
  "started_at": "2026-01-13T00:00:00Z",
  "ended_at": "2026-01-13T00:02:10Z",
  "app_version": "0.4.0",
  "app_commit": "abc1234",
  "git_commit": "def5678",
  "git_dirty": false,
  "config": {
    "mode": "summary",
    "dir": ".quorum/traces",
    "schema_version": 1,
    "redact": true
  },
  "summary": {
    "total_prompts": 6,
    "total_tokens_in": 1234,
    "total_tokens_out": 987,
    "total_cost_usd": 0.0421,
    "total_files": 0,
    "total_bytes": 0
  }
}
```

Trace configuration (optional):
```yaml
trace:
  mode: summary        # off | summary | full
  dir: .quorum/traces
  redact: true
  max_bytes: 262144
  total_max_bytes: 10485760
  max_files: 500
```

Notes:
- `summary` never stores prompt/response payloads on disk.
- `full` payloads are redacted and truncated based on limits; hashes remain for integrity checks.
- `quorum trace --json` outputs the raw manifest for automation.

Troubleshooting:
- No traces listed: ensure `trace.mode` is not `off` and the run finished without errors.
- Missing prompt/response files: you are likely in `summary` mode or size limits dropped content.
- Unexpected empty output: confirm `trace.dir` points to the correct workspace.

---

## Architecture

quorum-ai uses hexagonal architecture for clean separation between business logic and external systems:

```mermaid
graph LR
    subgraph "CLI"
        A[quorum run]
    end

    subgraph "Service Layer"
        B[Workflow Runner]
        C[Consensus Checker]
        D[DAG Builder]
    end

    subgraph "Adapters"
        E[Claude]
        F[Gemini]
        G[Git/Worktrees]
        H[State/JSON]
    end

    A --> B
    B --> C
    B --> D
    B --> E
    B --> F
    B --> G
    B --> H
```

For detailed architecture documentation, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## How It Works

### 1. Analyze Phase

Multiple agents independently analyze the task:

```
Claude Agent ----+
                 |---> Consensus Check ---> 80%+ Agreement? ---> Proceed
Gemini Agent ----+                     |
                                       +---> <80%? ---> V2 Critique
                                       +---> <60%? ---> V3 Synthesis
```

### 2. Plan Phase

Consolidated analysis informs plan generation:

- Parse plan into executable tasks
- Build dependency graph (DAG)
- Identify parallelizable tasks

### 3. Execute Phase

Tasks run in isolated environments:

- Create git worktree per task
- Execute with selected agent
- Validate results (tests, lint)
- Merge or report issues

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/ARCHITECTURE.md) | System design and layer responsibilities |
| [Vision](docs/vision/QUORUM-POC-VISION-v1.md) | POC goals, metrics, and success criteria |
| [Roadmap](ROADMAP.md) | Planned features for future versions |
| [Changelog](CHANGELOG.md) | Version history and changes |
| [Contributing](CONTRIBUTING.md) | How to contribute to the project |

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Setting up the development environment
- Code style and linting
- Commit message format
- Pull request process

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- TUI powered by [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss)
- Inspired by ensemble methods in machine learning and dialectic reasoning
