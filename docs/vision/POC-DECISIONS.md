# QUORUM-AI: POC Decision Summary

## Purpose

This document summarizes key POC scope and product decisions. Architectural
decisions are recorded as ADRs in `docs/adr/`, and this summary links to them
without duplicating the full rationale. It stays short to avoid documentation
drift.

## Summary

### 1. Delivery Model

- Single CLI binary for v1 to minimize operational complexity.
- Hexagonal architecture (ports and adapters) to isolate core logic from IO.

### 2. Consensus Protocol

- V(n) iterative refinement protocol with semantic arbiter for consensus evaluation.
- Configurable thresholds for consensus, abort, and stagnation detection.
- Min/max rounds gate iteration and human review.

### 3. State and Persistence

- JSON persistence for v1 with atomic writes and explicit locking.
- SQLite deferred to v2+ based on scale and query needs.

### 4. Execution Isolation

- Git worktrees per task to prevent cross-task interference.

### 5. Constraints

- Local execution only for v1.
- Multi-language prompts supported (see ADR-0007).
- No plugin system, web dashboard, or multi-repo orchestration in v1.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/ARCHITECTURE.md
- docs/adr/0001-hexagonal-architecture.md
- docs/adr/0002-consensus-protocol.md
- docs/adr/0003-state-persistence-json.md
- docs/adr/0004-worktree-isolation.md
