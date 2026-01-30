# ADR-0002: Consensus Protocol and Scoring

## Status

Accepted

## Context

The POC relies on multiple autonomous agents whose outputs vary in quality and
may conflict. We need a repeatable way to determine when outputs agree, when to
trigger additional refinement, and when to require human review.

## Decision

Adopt a semantic arbiter-based consensus protocol with iterative V(n) refinement
rounds. An AI arbiter evaluates semantic agreement between agent outputs and
determines when consensus has been reached.

### Protocol Flow

1. **V1 Analysis**: All enabled agents independently analyze the prompt
2. **V2 Refinement**: Agents review V1 outputs and refine their analysis
3. **Arbiter Evaluation**: The arbiter evaluates semantic consensus between V2 outputs
4. **Iteration**: If consensus threshold not met, repeat with V(n+1) refinement
5. **Consolidation**: Once consensus reached or max rounds exceeded, consolidate final output

### Configuration Parameters

- `threshold`: Minimum consensus score to proceed (default: 0.90)
- `min_rounds`: Minimum refinement rounds before consensus can be declared (default: 2)
- `max_rounds`: Maximum refinement rounds before aborting (default: 5)
- `warning_threshold`: Score below this logs a warning (default: 0.30)
- `stagnation_threshold`: Minimum improvement required between rounds (default: 0.02)

## Consequences

### Positive
- Provides semantic understanding of agreement rather than lexical comparison
- Adapts to context and nuance in agent outputs
- Encourages iterative refinement with feedback

### Negative
- Adds extra arbiter calls and cost when consensus is low
- Depends on arbiter model quality for accurate consensus assessment

### Neutral
- Thresholds may be tuned over time based on observed results

## References

- docs/CONFIGURATION.md#consensus
- docs/ARCHITECTURE.md
