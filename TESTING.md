# Testing Guide

This document explains how to run and write tests for Quorum-AI.

## Quick Start

```bash
# Run all Go tests
go test ./...

# Run all Go tests with race detection
go test -race ./...

# Run frontend tests
cd frontend && npm test

# Run Playwright E2E tests
cd frontend && npx playwright test
```

## Test Structure

```
quorum-ai/
├── internal/
│   ├── testutil/           # Shared test utilities
│   │   ├── mocks.go        # MockAgent, MockStateManager, MockRegistry
│   │   ├── golden.go       # Golden file testing utilities
│   │   └── helpers.go      # TempDir, GitRepo helpers
│   └── */*_test.go         # Unit tests alongside code
├── tests/
│   └── e2e/
│       └── cli_test.go     # CLI end-to-end tests
└── frontend/
    ├── src/**/*.test.{ts,tsx}  # Vitest unit tests
    └── e2e/*.spec.ts           # Playwright E2E tests
```

## Go Tests

### Unit Tests

Run unit tests (fast, no external dependencies):

```bash
go test ./...
go test -race ./...          # With race detection
go test -short ./...         # Skip slow tests
go test -v ./internal/core/  # Verbose, specific package
```

### Integration Tests

Integration tests use the `integration` build tag and may require database/filesystem:

```bash
go test -tags=integration ./...
go test -tags=integration -race ./...
```

### E2E Tests

End-to-end tests use the `e2e` build tag:

```bash
go test -tags=e2e ./tests/e2e/...
```

### Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out        # View in browser
go tool cover -func=coverage.out        # Summary by function
```

## Frontend Tests

### Unit Tests (Vitest)

```bash
cd frontend
npm test                    # Run all tests
npm test -- --run           # Run once (no watch)
npm test -- --coverage      # With coverage (requires @vitest/coverage-v8)
npm test -- MyComponent     # Filter by name
```

### E2E Tests (Playwright)

```bash
cd frontend
npx playwright test                      # Run all
npx playwright test --ui                 # Interactive UI mode
npx playwright test workflows.spec.ts    # Specific file
npx playwright test --project=chromium   # Specific browser
```

## Writing Tests

### Using testutil Mocks

The `internal/testutil` package provides reusable mocks:

```go
import "github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"

func TestMyFeature(t *testing.T) {
    // Create a mock agent with custom behavior
    agent := testutil.NewMockAgent().
        WithResponse("test response").
        WithExecuteFunc(func(ctx context.Context, req adapters.Request) (adapters.Response, error) {
            // Custom logic
            return adapters.Response{Content: "custom"}, nil
        })

    // Create a mock state manager
    state := testutil.NewMockStateManager()

    // Create a mock registry
    registry := testutil.NewMockRegistry().
        WithAgent("test-agent", agent)

    // Use mocks in your test...
}
```

### Table-Driven Tests

Prefer table-driven tests for multiple cases:

```go
func TestParser(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Result
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "hello",
            want:  Result{Value: "hello"},
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Golden File Tests

Use golden files to test CLI/TUI output:

```go
import "github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"

func TestCLIOutput(t *testing.T) {
    golden := testutil.NewGolden(t, "testdata/golden")

    // Run command and capture output
    output := runCommand(t, "help")

    // Scrub non-deterministic values
    scrubbed := testutil.ScrubAll(output)

    // Compare against golden file (auto-creates if missing)
    golden.Assert(t, "help.golden", scrubbed)
}
```

To update golden files when output intentionally changes:

```bash
UPDATE_GOLDEN=1 go test -tags=e2e ./tests/e2e/...
```

### Test Helpers

```go
import "github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"

func TestWithTempDir(t *testing.T) {
    // Creates temp directory, cleaned up automatically
    dir := testutil.TempDir(t)

    // Create a temp git repo for testing
    repo := testutil.NewGitRepo(t)
    repo.Commit(t, "initial commit")
    repo.CreateBranch(t, "feature")
}
```

## Build Tags

| Tag | Purpose | Example |
|-----|---------|---------|
| (none) | Unit tests, fast, no dependencies | `go test ./...` |
| `integration` | DB, filesystem, network | `go test -tags=integration ./...` |
| `e2e` | Full CLI/system tests | `go test -tags=e2e ./tests/e2e/...` |
| `smoke` | Tests with real LLM (slow, costs money) | `go test -tags=smoke ./...` |

To add a build tag to a test file:

```go
//go:build integration

package mypackage

// Tests in this file only run with -tags=integration
```

## CI/CD

On every PR, CI runs:

1. **Lint** - `golangci-lint` and `npm run lint`
2. **Unit Tests** - `go test -race ./...` and `npm test`
3. **Integration Tests** - `go test -tags=integration ./...`
4. **E2E Tests** - CLI golden tests and Playwright

Coverage is uploaded to Codecov automatically.

### Running CI Checks Locally

```bash
# Lint
golangci-lint run
cd frontend && npm run lint

# Full test suite (what CI runs)
go test -race ./...
go test -race -tags=integration ./...
go test -tags=e2e ./tests/e2e/...
cd frontend && npm test -- --run
cd frontend && npx playwright test
```

## Troubleshooting

### Tests Fail with Race Condition

Run with `-race` flag to detect:

```bash
go test -race ./...
```

### Golden Test Fails After Intentional Change

Update the golden files:

```bash
UPDATE_GOLDEN=1 go test -tags=e2e ./tests/e2e/...
```

### Playwright Tests Fail Locally

Install browsers:

```bash
cd frontend && npx playwright install
```

### Coverage Not Working in Frontend

Install the coverage dependency:

```bash
cd frontend && npm install -D @vitest/coverage-v8
```

## Current Metrics

As of 2026-02-02:

| Metric | Count |
|--------|-------|
| Go test files | 108 |
| Go Test* functions | ~1222 |
| Go Fuzz* functions | 4 |
| Frontend test files | 20 |
| Playwright E2E specs | 5 |
| Go coverage | 33.2% (partial) |

For detailed analysis, see `claude-analisis-testing-vfinal.md`.
