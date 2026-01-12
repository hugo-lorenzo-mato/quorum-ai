# ADR-0002: Consensus Protocol and Scoring

## Status

Accepted

## Context

The POC relies on multiple autonomous agents whose outputs vary in quality and
may conflict. We need a repeatable way to determine when outputs agree, when to
escalate to critique, and when to require human review.

## Decision

Adopt a three-round dialectic protocol (V1/V2/V3) and a weighted Jaccard
similarity score across categorized content (claims, risks, recommendations).
Use explicit thresholds to decide whether to proceed, run critique rounds, or
require human review.

## Consequences

### Positive
- Provides a measurable, repeatable consensus signal across agents.
- Encourages structured disagreement and refinement before execution.

### Negative
- Adds extra runs and cost when consensus is low.
- Requires consistent categorization of agent outputs to compute scores.

### Neutral
- Thresholds may be tuned over time based on observed results.

## References

- docs/vision/QUORUM-POC-VISION-v1.md
- docs/vision/POC-DECISIONS.md
