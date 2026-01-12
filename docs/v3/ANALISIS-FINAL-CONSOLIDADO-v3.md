# Final Consolidated Analysis v3

## Purpose

This document summarizes the key decisions and constraints that guide the
quorum-ai POC. It is referenced by the vision document to anchor the scope and
evaluation criteria.

## Decisions

### 1. POC Focus

- Validate whether multi-agent consensus improves reliability versus a single
  agent in software engineering workflows.
- Limit scope to CLI-based agents and local execution to reduce external
  dependencies.

### 2. Consensus Protocol

- Use a three-round dialectic process (V1/V2/V3) to reconcile disagreements.
- Apply Jaccard similarity with category weights to quantify agreement.
- Use threshold gating to determine escalation and human review.

### 3. Architecture

- Adopt hexagonal architecture to isolate core logic from adapters.
- Enforce core -> service -> adapters layer boundaries.

### 4. State and Execution

- Persist state with JSON and atomic writes for v1 simplicity.
- Use git worktree isolation per task to reduce interference and enable parallelism.

### 5. POC Constraints

- English-only prompts and outputs.
- No plugin system, web UI, or remote orchestration in v1.
- v2+ features deferred until POC exit criteria are met.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/ARCHITECTURE.md
- docs/adr/0001-hexagonal-architecture.md
