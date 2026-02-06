# ADR Index

This index tracks architectural decisions for quorum-ai. New ADRs should use the
template in ADR-0000.

| ADR | Title | Status | Summary |
| --- | --- | --- | --- |
| [0000](0000-template.md) | Template | N/A | Template for recording decisions |
| [0001](0001-hexagonal-architecture.md) | Adopt Hexagonal Architecture | Accepted | Use ports and adapters to isolate core logic |
| [0002](0002-consensus-protocol.md) | Consensus Protocol and Scoring | Accepted | V1/V2/V3 rounds with weighted Jaccard thresholds |
| [0003](0003-state-persistence-json.md) | JSON State Persistence for POC | Superseded | JSON state with atomic writes and locking |
| [0004](0004-worktree-isolation.md) | Worktree Isolation per Task | Accepted | Git worktrees per task to avoid interference |
| [0005](0005-workflow-decomposition.md) | WorkflowRunner Decomposition | Accepted | Split WorkflowRunner into focused phase runners |
| [0006](0006-error-handling-standardization.md) | Error Handling Standardization | Accepted | Standardize wrapping, logging, and validation |
| [0007](0007-multilingual-prompt-optimization.md) | Multilingual Prompt Optimization | Accepted | Support multi-language prompts with LLM optimization |
| [0008](0008-github-adapter-testability.md) | GitHub Adapter Testability | Accepted | Design GitHub adapter for testability |
| [0009](0009-sqlite-state-backend.md) | SQLite as Default State Backend | Accepted | SQLite-only runtime persistence |
