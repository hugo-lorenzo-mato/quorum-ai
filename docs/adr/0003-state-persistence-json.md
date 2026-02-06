# ADR-0003: JSON State Persistence for POC

## Status

Superseded by [ADR-0009](0009-sqlite-state-backend.md)

> **Note:** Runtime persistence is now SQLite-only. This ADR documents the
> original POC decision to persist state in JSON.

## Context

The POC requires durable workflow state and checkpoints, but is scoped to a
single user running locally. Introducing a database would add operational
complexity and dependencies that are not justified for v1.

## Decision

Persist workflow state in JSON with atomic writes and explicit locking. Defer
SQLite or other database backends to a later version when scale or query needs
justify it.

## Consequences

### Positive
- Minimal operational complexity and dependencies for v1.
- Straightforward to inspect and debug state locally.

### Negative
- Limited query capabilities and potential performance issues at scale.
- Requires careful locking to avoid concurrent write corruption.

### Neutral
- Migration to SQLite remains an option for v2+ without changing domain logic.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/vision/POC-DECISIONS.md
