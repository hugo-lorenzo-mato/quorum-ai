# ADR-0005: WorkflowRunner Decomposition

## Status
Accepted

## Context

The `WorkflowRunner` in `internal/service/workflow.go` has grown to ~950 lines and violates the Single Responsibility Principle (SRP). It currently handles:

1. Workflow orchestration (Run, Resume)
2. Analysis phase execution (V(n) iterative refinement with semantic arbiter)
3. Plan phase generation
4. Task execution and DAG traversal
5. Error handling and checkpointing
6. State management coordination

Based on research from:
- [Go Project Structure Patterns 2025](https://www.glukhov.org/post/2025/12/go-project-structure/)
- [Clean/Hexagonal Architecture in Go](https://medium.com/@kemaltf_/clean-architecture-hexagonal-architecture-in-go-a-practical-guide-aca2593b7223)
- [Three Dots Labs Clean Architecture](https://threedots.tech/post/introducing-clean-architecture/)

The hexagonal architecture principle states: "Each hexagon should be a single working unit with a single responsibility."

## Decision

Decompose `WorkflowRunner` into focused components following the hexagonal pattern:

### New Structure

```
internal/service/
├── workflow/
│   ├── runner.go          # WorkflowRunner - orchestrator only (~150 lines)
│   ├── runner_test.go
│   ├── analyzer.go         # AnalysisPhaseRunner - V(n) iterative refinement (~200 lines)
│   ├── analyzer_test.go
│   ├── arbiter.go          # SemanticArbiter - consensus evaluation (~150 lines)
│   ├── arbiter_test.go
│   ├── planner.go          # PlanPhaseRunner - plan generation (~150 lines)
│   ├── planner_test.go
│   ├── executor.go         # ExecutePhaseRunner - task execution (~200 lines)
│   ├── executor_test.go
│   └── context.go          # WorkflowContext - shared execution context
├── checkpoint.go           # (existing)
├── retry.go                # (existing)
└── ratelimit.go            # (existing)
```

### Component Responsibilities

1. **WorkflowRunner** (orchestrator)
   - Initialize workflow state
   - Coordinate phase transitions
   - Handle workflow-level errors
   - Manage lock acquisition/release

2. **AnalysisPhaseRunner**
   - Run V1 parallel analysis
   - Run V(n) iterative refinement rounds
   - Coordinate with semantic arbiter for consensus evaluation
   - Consolidate analysis outputs

3. **SemanticArbiter**
   - Evaluate semantic consensus between agent outputs
   - Generate consensus scores and divergence reports
   - Determine when to continue or stop refinement

4. **PlanPhaseRunner**
   - Generate execution plan from analysis
   - Parse and validate task structure
   - Build dependency graph

5. **ExecutePhaseRunner**
   - Execute tasks according to DAG
   - Handle task-level retries
   - Track task metrics

6. **WorkflowContext**
   - Shared state accessor
   - Logger and metrics
   - Configuration

### Interface Design

```go
// PhaseRunner executes a workflow phase
type PhaseRunner interface {
    Run(ctx context.Context, wctx *WorkflowContext) error
}

// WorkflowContext provides shared resources
type WorkflowContext struct {
    State      *core.WorkflowState
    Agents     core.AgentRegistry
    Prompts    *PromptRenderer
    Checkpoint *CheckpointManager
    Retry      *RetryPolicy
    RateLimits *RateLimiterRegistry
    Logger     *logging.Logger
    Config     *WorkflowConfig
}
```

## Consequences

### Positive
- Each component has a single responsibility
- Easier to test individual phases
- Clearer code organization
- Follows established Go project patterns
- Enables phase-specific configuration

### Negative
- More files to navigate
- Slight increase in abstraction
- Migration effort required

## Implementation Plan

1. Create `internal/service/workflow/` package
2. Extract `WorkflowContext` type
3. Extract `AnalysisPhaseRunner` with V(n) iterative refinement methods
4. Extract `SemanticArbiter` for consensus evaluation
5. Extract `PlanPhaseRunner`
6. Extract `ExecutePhaseRunner`
7. Refactor `WorkflowRunner` to orchestrator role
8. Update tests and imports
9. Verify all tests pass

## References

- [Go Project Structure 2025](https://www.glukhov.org/post/2025/12/go-project-structure/)
- [Hexagonal Architecture in Go](https://medium.com/@matiasvarela/hexagonal-architecture-in-go-cfd4e436faa3)
- [Three Dots Labs Blog](https://threedots.tech/post/introducing-clean-architecture/)
