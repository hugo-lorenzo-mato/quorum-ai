# ADR-0008: GitHub Adapter Testability via CommandRunner Interface

## Status

Accepted

## Context

The GitHub adapter (`internal/adapters/github`) wraps the `gh` CLI to interact with GitHub's API. The original implementation had direct `exec.Command` calls embedded throughout the code, making it impossible to unit test without actually having `gh` installed and authenticated.

This resulted in:
1. **Low test coverage**: Only 15.4% coverage, limited to parsing functions and struct field tests
2. **CI/CD complexity**: Integration tests required `gh` CLI setup and authentication
3. **Slow feedback**: Tests that required `gh` were slow and flaky
4. **Untestable error paths**: Timeout handling, rate limiting, and error scenarios couldn't be tested

### Alternatives Considered

1. **HTTP Client with httptest**: Replace `gh` CLI with direct GitHub REST API calls
   - Pros: Full control, no external dependencies, httptest is idiomatic
   - Cons: Significant rewrite, must handle auth/pagination/rate-limiting that `gh` provides

2. **Interface for entire Client**: Create interface for `Client` and use mock in consumers
   - Pros: Simple, already done via `core.GitHubClient`
   - Cons: Doesn't test the adapter itself, just its consumers

3. **CommandRunner interface injection** (selected): Abstract command execution behind interface
   - Pros: Minimal changes, keeps `gh` CLI benefits, enables comprehensive testing
   - Cons: Slight indirection in command execution

## Decision

Introduce a `CommandRunner` interface that abstracts command execution:

```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) (string, error)
}
```

The adapter uses dependency injection to receive a `CommandRunner`:
- Production code uses `ExecRunner` (wraps `os/exec`)
- Test code uses `MockRunner` (returns configured responses)

### Key Changes

1. **New files**:
   - `runner.go`: Defines `CommandRunner` interface and `ExecRunner` implementation
   - `runner_mock.go`: Provides `MockRunner` for tests

2. **Client modification**:
   - Added `runner` field to `Client` struct
   - `NewClient()` uses `ExecRunner` by default
   - `NewClientWithRunner()` accepts custom runner for testing
   - `NewClientSkipAuth()` creates client without auth check (test-only)

3. **Test pattern**:
```go
func TestClient_GetPR(t *testing.T) {
    runner := NewMockRunner()
    runner.OnCommand("gh pr view 42").ReturnJSON(`{"number": 42, ...}`)

    client := NewClientSkipAuth("owner", "repo", runner)

    pr, err := client.GetPR(context.Background(), 42)
    // assertions...
}
```

## Consequences

### Positive

- **Coverage increased from 15.4% to 63.5%** with room for more
- **Fast unit tests**: No external process execution
- **Testable error paths**: Timeouts, failures, edge cases can be tested
- **CI/CD simplified**: No `gh` CLI required for unit tests
- **Preserves `gh` benefits**: Authentication, pagination, rate limiting handled by `gh`
- **Minimal refactoring**: Core logic unchanged, only execution abstracted

### Negative

- **Slight indirection**: One extra layer between Client and exec
- **Mock maintenance**: Mock responses must match `gh` output format
- **Integration tests still needed**: Real `gh` interaction tested separately

### Neutral

- **Backward compatible**: Public API unchanged, existing code works
- **Pattern familiar**: CommandRunner is a common Go testing pattern

## References

- [Go Testing Techniques](https://go.dev/blog/table-driven-tests)
- [Dependency Injection in Go](https://blog.drewolson.org/dependency-injection-in-go)
- [GitHub CLI Documentation](https://cli.github.com/manual/)
