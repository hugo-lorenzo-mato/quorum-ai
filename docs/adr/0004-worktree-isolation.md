# ADR-0004: Worktree Isolation per Task

## Status

Accepted

## Context

Task execution may run in parallel and generate code changes. Sharing a single
working tree across tasks risks file conflicts, cross-task interference, and
hard-to-reproduce outcomes.

## Decision

Execute each task in its own Git worktree. The workflow runner creates and
cleans up worktrees to isolate changes and avoid interference between tasks.

## Consequences

### Positive
- Prevents cross-task file conflicts and accidental overwrites.
- Makes task results reproducible and easier to review.

### Negative
- Adds overhead for worktree creation and cleanup.
- Requires disk space proportional to concurrent tasks.

### Neutral
- Worktree lifecycle management becomes part of workflow orchestration.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/vision/POC-DECISIONS.md
