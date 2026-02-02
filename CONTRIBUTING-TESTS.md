# Testing Requirements for Contributors

This document outlines the mandatory testing requirements for all contributions to Quorum AI.

## Quick Reference

```bash
# Run all checks before submitting
make check            # Go lint + tests + frontend checks
make test             # Go unit tests only
make frontend-test    # Frontend tests only
make test-coverage    # Go coverage report
make frontend-coverage # Frontend coverage report
```

## Golden Rule

**All tests must pass before any PR can be merged.**

If tests fail:
1. **Expected change in behavior**: Update the test to match the new behavior
2. **Unexpected failure**: Investigate and fix the root cause

## Test Requirements by Area

### Backend (Go)

| Area Changed | Required Tests |
|--------------|----------------|
| API endpoints | Unit tests in `internal/api/*_test.go` |
| Config parsing | Unit tests + fuzz tests in `internal/config/` |
| Core domain (Workflow, Task) | Unit tests + fuzz tests in `internal/core/` |
| Service layer | Unit tests in `internal/service/*_test.go` |
| Agent adapters | Integration tests with `//go:build integration` |
| CLI commands | E2E tests in `tests/e2e/cli_test.go` |
| New DTOs | Contract tests if API-facing |

### Frontend (React)

| Area Changed | Required Tests |
|--------------|----------------|
| Components | Unit tests in `__tests__/*.test.jsx` |
| Stores (Zustand) | Unit tests for actions and selectors |
| API layer | Contract tests against schemas |
| User flows | Playwright E2E tests in `frontend/e2e/` |

## Test Patterns

### Go Unit Tests

```go
func TestFeatureName(t *testing.T) {
    // Arrange
    input := "test data"

    // Act
    result, err := MyFunction(input)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Table-Driven Tests

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Config
        wantErr bool
    }{
        {"valid config", "...", Config{}, false},
        {"invalid yaml", "{{{", Config{}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Frontend Tests

```jsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import MyComponent from './MyComponent';

describe('MyComponent', () => {
    it('renders correctly', () => {
        render(<MyComponent />);
        expect(screen.getByText('Expected Text')).toBeInTheDocument();
    });

    it('handles user interaction', async () => {
        const user = userEvent.setup();
        render(<MyComponent />);
        await user.click(screen.getByRole('button'));
        expect(screen.getByText('Result')).toBeInTheDocument();
    });
});
```

### Contract Tests

When modifying API DTOs, update the contract schemas:

```javascript
// frontend/src/contracts/schemas.js
export const myNewSchema = {
    type: 'object',
    required: ['id', 'status'],
    properties: {
        id: { type: 'string' },
        status: { type: 'string', enum: ['pending', 'completed'] },
    },
};
```

## Handling Test Failures

### Case 1: Intentional Behavior Change

If you intentionally changed behavior:

1. **Update the test** to match the new expected behavior
2. **Update golden files** if output format changed:
   ```bash
   # Update golden file with new output
   go test ./tests/e2e/... -update-golden
   ```
3. **Update contract schemas** if API response changed
4. **Document the change** in your PR description

### Case 2: Unexpected Failure

If the test failure was unexpected:

1. **Read the failure message carefully**
2. **Check recent changes** that might have caused it
3. **Run in isolation** to reproduce:
   ```bash
   go test -v -run TestSpecificTest ./path/to/package/...
   ```
4. **Debug if needed**:
   ```bash
   go test -v -run TestSpecificTest ./... 2>&1 | head -100
   ```
5. **Fix the root cause**, not the symptom

### Case 3: Flaky Test

If a test passes sometimes and fails other times:

1. **Check for race conditions**:
   ```bash
   go test -race ./...
   ```
2. **Look for timing dependencies** (sleep, timeouts)
3. **Check for shared state** between tests
4. **File an issue** if you can't fix it immediately

## Race Detection

All Go tests run with `-race` flag in CI. To catch issues locally:

```bash
go test -race ./...
```

## Coverage Requirements

While there's no strict coverage threshold, aim for:
- **New code**: 80%+ coverage
- **Critical paths**: 100% coverage (authentication, payments, state machines)

Check coverage:
```bash
make test-coverage    # Go
make frontend-coverage # Frontend
```

## Fuzz Testing

For input parsing and validation code, add fuzz tests:

```go
func FuzzParseInput(f *testing.F) {
    f.Add("valid input")
    f.Add("")
    f.Add("special\x00chars")

    f.Fuzz(func(t *testing.T, input string) {
        // Should not panic
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("panic: %v", r)
            }
        }()

        _, _ = ParseInput(input)
    })
}
```

Run fuzz tests:
```bash
make fuzz
```

## Integration Tests

For tests that need real resources (database, external services):

```go
//go:build integration

func TestDatabaseIntegration(t *testing.T) {
    // Test with real database
}
```

Run integration tests:
```bash
make test-integration
```

## E2E Tests

For CLI and full-system tests:

```go
//go:build e2e

func TestCLI(t *testing.T) {
    binary := buildBinary(t)
    // Test CLI commands
}
```

Run E2E tests:
```bash
make test-e2e
```

## Pre-Submit Checklist

Before submitting a PR:

- [ ] `make check` passes
- [ ] All existing tests pass
- [ ] New tests added for new functionality
- [ ] Tests updated for changed functionality
- [ ] No flaky tests introduced
- [ ] Coverage didn't significantly decrease

## Getting Help

- Check existing tests for patterns
- Read `TESTING.md` for detailed testing guide
- Ask in PR comments if unsure about test approach
