# QUORUM-AI: Consolidated Analysis v3

## Purpose

This document consolidates architectural and product decisions used to scope the
POC. It serves as a stable reference for the vision document and is intentionally
short to avoid documentation drift.

## Decision Summary

### 1. Delivery Model

- Single CLI binary for v1 to minimize operational complexity.
- Hexagonal architecture (ports and adapters) to isolate core logic from IO.

### 2. Consensus Protocol

- V1/V2/V3 dialectic protocol to resolve divergent agent outputs.
- Jaccard similarity with category weights for consensus scoring.
- Thresholds gate escalation and human review.

### 3. State and Persistence

- JSON persistence for v1 with atomic writes and explicit locking.
- SQLite deferred to v2+ based on scale and query needs.

### 4. Execution Isolation

- Git worktrees per task to prevent cross-task interference.

### 5. Constraints

- Local execution only for v1.
- English-only prompts and outputs.
- No plugin system, web dashboard, or multi-repo orchestration in v1.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/ARCHITECTURE.md
- docs/adr/0001-hexagonal-architecture.md
