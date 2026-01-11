# quorum-ai

[![CI](https://github.com/hugo-lorenzo-mato/quorum-ai/actions/workflows/test.yml/badge.svg)](https://github.com/hugo-lorenzo-mato/quorum-ai/actions/workflows/test.yml)
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

---

## Quick Start

### Prerequisites

- Go 1.22 or later
- Git 2.20 or later
- At least one LLM CLI installed:
  - [Claude Code](https://github.com/anthropics/claude-code) (recommended)
  - [Gemini CLI](https://github.com/google/gemini-cli)

### Installation

```bash
# Using go install
go install github.com/hugolma/quorum-ai/cmd/quorum@latest

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
  - name: claude
    enabled: true
  - name: gemini
    enabled: true

consensus:
  threshold: 0.80

logging:
  level: info
```

### Usage

```bash
# Verify prerequisites
quorum doctor

# Run full workflow (analyze -> plan -> execute)
quorum run "Implement user authentication with JWT tokens"

# Run individual phases
quorum analyze "Review the authentication module for security issues"
quorum plan "Add rate limiting to the API endpoints"
quorum execute

# Check workflow status
quorum status
```

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
