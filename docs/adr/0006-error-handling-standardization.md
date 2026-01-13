# ADR-0006: Error Handling Standardization

## Status
Accepted

## Context

Current error handling in the codebase is inconsistent:

1. Some code uses `fmt.Errorf("...: %w", err)` for wrapping
2. Some code uses domain errors (`core.ErrValidation`, etc.)
3. Some error paths silently log and continue
4. Error context is sometimes lost in the chain

Based on research from:
- [Go 1.13 Error Wrapping](https://go.dev/blog/go1.13-errors)
- [JetBrains Go Error Handling Best Practices](https://www.jetbrains.com/guide/go/tutorials/handle_errors_in_go/best_practices/)
- [Datadog Go Error Handling Guide](https://www.datadoghq.com/blog/go-error-handling/)

Key principles identified:
- Use `%w` for wrapping, never `%v`
- Wrap at abstraction boundaries to translate errors
- Log once at the boundary, add context everywhere else
- Keep 3-5 sentinel errors per domain

## Decision

### Error Wrapping Rules

1. **At adapter boundaries**: Translate to domain errors
   ```go
   // In adapters/cli/claude.go
   result, err := c.ExecuteCommand(ctx, args, "")
   if err != nil {
       return nil, core.ErrExecution("CLI_FAILED", "claude execution failed").WithCause(err)
   }
   ```

2. **Within service layer**: Wrap with context using `%w`
   ```go
   // In service/workflow.go
   if err := w.analyzer.Run(ctx, wctx); err != nil {
       return fmt.Errorf("analysis phase: %w", err)
   }
   ```

3. **At domain boundaries**: Use domain error constructors
   ```go
   // In core/errors.go - existing pattern is good
   return core.ErrValidation("EMPTY_PROMPT", "prompt cannot be empty")
   ```

### Error Logging

- Log **once** at the top-level handler (cmd layer)
- Lower layers add context via wrapping
- Never log and return the same error

### Input Validation Pattern

Add validation at service entry points:

```go
// Validate at public method entry
func (w *WorkflowRunner) Run(ctx context.Context, prompt string) error {
    if err := w.validateRunInput(prompt); err != nil {
        return err
    }
    // ... rest of implementation
}

func (w *WorkflowRunner) validateRunInput(prompt string) error {
    if strings.TrimSpace(prompt) == "" {
        return core.ErrValidation("EMPTY_PROMPT", "prompt cannot be empty")
    }
    if len(prompt) > MaxPromptLength {
        return core.ErrValidation("PROMPT_TOO_LONG",
            fmt.Sprintf("prompt exceeds maximum length of %d", MaxPromptLength))
    }
    return nil
}
```

### New Domain Error Codes

Add to `core/errors.go`:

```go
const (
    // Validation errors
    CodeEmptyPrompt      = "EMPTY_PROMPT"
    CodePromptTooLong    = "PROMPT_TOO_LONG"
    CodeInvalidConfig    = "INVALID_CONFIG"

    // Execution errors
    CodeNoAgents         = "NO_AGENTS"
    CodeAgentFailed      = "AGENT_FAILED"
    CodeExecutionStuck   = "EXECUTION_STUCK"
)
```

## Consequences

### Positive
- Consistent error handling across codebase
- Better error context for debugging
- Type-safe error checking with `errors.Is/As`
- Clear separation between validation and runtime errors

### Negative
- More verbose error construction
- Need to update existing error sites
- Slight learning curve for contributors

## Implementation

1. Add new error codes to `core/errors.go`
2. Add input validation to `WorkflowRunner.Run` and `Resume`
3. Standardize error wrapping in service layer
4. Ensure adapters translate to domain errors
5. Review and update logging to avoid log-and-return pattern
