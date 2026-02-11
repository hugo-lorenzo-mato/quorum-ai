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

## Pre-Commit Validation Checklist

**IMPORTANT:** Before pushing any changes, especially test-related changes, run this complete validation to ensure CI will pass:

### 1. Format Code

```bash
# Format Go code (REQUIRED - CI checks this)
gofmt -w internal/ cmd/

# Verify formatting
gofmt -l internal/ cmd/  # Should return empty
```

### 2. Lint Checks

```bash
# Backend lint (REQUIRED)
make build-frontend  # golangci-lint needs embedded frontend
golangci-lint run --timeout=10m

# Frontend lint (REQUIRED)
cd frontend
npm ci --ignore-scripts  # If dependencies changed
npm run lint
cd ..
```

### 3. Run All Tests Locally

```bash
# Backend tests with race detector (REQUIRED - CI uses -race)
make build-frontend
go test -race -v -timeout 10m ./...

# Frontend tests with coverage
cd frontend
npm run test:coverage
cd ..

# Integration tests (REQUIRED for main branch)
go test -race -v -tags=integration -timeout 20m ./...
```

### 4. Platform-Specific Validation

**If you modified path handling, file I/O, or platform-dependent code:**

```bash
# Test on actual platforms if possible:
# - Windows: WSL won't catch Windows-specific issues
# - macOS: Test symlink resolution (/tmp -> /private/tmp)
# - Linux: Test as baseline

# Check for common cross-platform issues:
# - Use filepath.ToSlash() for path comparisons in tests
# - Use runtime.GOOS for platform-specific behavior
# - Don't rely on HOME env var on Windows (use explicit dirs in tests)
# - Close file handles before deleting in tests (Windows requirement)
```

### 5. Build Verification

```bash
# Verify cross-compilation (CI builds for 5 platforms)
GOOS=linux GOARCH=amd64 go build ./cmd/quorum
GOOS=darwin GOARCH=arm64 go build ./cmd/quorum
GOOS=windows GOARCH=amd64 go build ./cmd/quorum
```

### 6. Security Checks

```bash
# gosec (optional locally, but CI runs it)
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -exclude=G104,G301,G302,G306 -exclude-dir=.git -exclude-dir=frontend ./...

# govulncheck (REQUIRED)
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### 7. Final Verification Script

Save this as `scripts/validate-ci.sh`:

```bash
#!/bin/bash
set -e

echo "=== Building frontend ==="
make build-frontend

echo "=== Formatting Go code ==="
gofmt -w internal/ cmd/

echo "=== Backend lint ==="
golangci-lint run --timeout=10m

echo "=== Frontend lint ==="
(cd frontend && npm run lint)

echo "=== Backend tests with race detector ==="
go test -race -v -timeout 10m ./...

echo "=== Frontend tests ==="
(cd frontend && npm run test:coverage)

echo "=== Integration tests ==="
go test -race -v -tags=integration -timeout 20m ./...

echo "=== Security checks ==="
govulncheck ./...

echo "=== Cross-compilation test ==="
GOOS=linux GOARCH=amd64 go build -o /tmp/quorum-test ./cmd/quorum
GOOS=darwin GOARCH=arm64 go build -o /tmp/quorum-test ./cmd/quorum
GOOS=windows GOARCH=amd64 go build -o /tmp/quorum-test.exe ./cmd/quorum

echo ""
echo "✓ All validations passed! Safe to push."
```

Then run before every push:

```bash
chmod +x scripts/validate-ci.sh
./scripts/validate-ci.sh
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

### What CI Validates

The CI pipeline consists of 5 workflows that run on every push to main and every PR:

**1. Lint Workflow**
- Backend: golangci-lint with Go 1.25 compatibility
- Frontend: ESLint with React rules
- **Runs on:** Ubuntu
- **Can run locally:** YES (100%)

**2. Tests Workflow**
- Backend: `go test -race -coverprofile=coverage.out ./...`
- Frontend: `npm run test:coverage`
- Coverage upload to CodeCov
- **Runs on:** Ubuntu
- **Can run locally:** YES (95% - upload requires token)

**3. Build Workflow**
- Cross-compilation for 5 platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- **Runs on:** Ubuntu (cross-compiles all platforms)
- **Can run locally:** YES (100%)

**4. CI Workflow (comprehensive)**
- Check: golangci-lint on all code
- Tests: Go tests on Ubuntu, macOS, Windows (3 platforms)
- Integration tests with -race
- CodeQL security analysis (Go + JavaScript)
- SonarCloud quality analysis
- Dependency review (PRs only)
- **Runs on:** Ubuntu, macOS, Windows
- **Can run locally:** PARTIAL (85%)
  - Tests: YES
  - CodeQL: NO (requires GitHub infrastructure)
  - SonarCloud: PARTIAL (requires token)
  - Dependency Review: NO (requires GitHub API)

**5. Security Workflow**
- gosec (Go security scanner)
- govulncheck (vulnerability scanner)
- npm audit (frontend dependencies)
- CodeQL analysis
- **Runs on:** Ubuntu
- **Can run locally:** PARTIAL (70%)
  - gosec, govulncheck, npm audit: YES
  - CodeQL: NO

### Running Full CI Validation Locally

**What you CAN validate (90% of CI):**

```bash
# 1. Lint (exact match with CI)
make build-frontend
golangci-lint run --timeout=10m
cd frontend && npm run lint && cd ..

# 2. Tests (exact match with CI)
go test -race -v -timeout 10m ./...
go test -race -tags=integration -timeout 20m ./...
cd frontend && npm run test:coverage && cd ..

# 3. Build (exact match with CI)
GOOS=linux GOARCH=amd64 go build ./cmd/quorum
GOOS=darwin GOARCH=arm64 go build ./cmd/quorum
GOOS=windows GOARCH=amd64 go build ./cmd/quorum

# 4. Security (partial match)
govulncheck ./...
gosec -exclude=G104,G301,G302,G306 -exclude-dir=.git -exclude-dir=frontend ./...
cd frontend && npm audit --audit-level=high && cd ..
```

**What you CANNOT validate locally:**

```bash
# CodeQL Analysis - requires GitHub Actions infrastructure
# SonarCloud - requires SONAR_TOKEN and sonarcloud.io account
# Dependency Review - requires GitHub API (PR context)
# CodeCov upload - requires CODECOV_TOKEN
```

**Recommendation:**

Use the pre-commit validation script (`scripts/validate-ci.sh`) to catch 90% of issues before pushing. The remaining 10% (CodeQL, SonarCloud) will run in CI and rarely fail if local validation passes.

## Cross-Platform Testing Guidelines

### Writing Platform-Independent Tests

**Path Handling:**

```go
// GOOD: Platform-independent path comparison
func TestPathComparison(t *testing.T) {
    got := someFunc()
    want := "/expected/path"

    // Normalize both paths for comparison
    if filepath.ToSlash(got) != filepath.ToSlash(want) {
        t.Errorf("got %q, want %q", got, want)
    }
}

// BAD: Direct string comparison fails on Windows
func TestPathComparison(t *testing.T) {
    got := someFunc()  // Returns "C:\path\to\file" on Windows
    want := "/path/to/file"
    if got != want {  // FAILS on Windows
        t.Errorf("got %q, want %q", got, want)
    }
}
```

**Detecting Absolute Paths:**

```go
// GOOD: Cross-platform absolute path detection
func isAbsolutePath(path string) bool {
    if filepath.IsAbs(path) {
        return true
    }
    // Windows: filepath.IsAbs("/unix/path") = false
    // But these should still be treated as absolute
    if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
        return true
    }
    return false
}
```

**File Handle Management (Critical for Windows):**

```go
// GOOD: Close handles before deleting
func TestWithTempDir(t *testing.T) {
    tmpDir := t.TempDir()

    db, _ := sql.Open("sqlite3", filepath.Join(tmpDir, "test.db"))
    // ... use db ...
    db.Close()  // REQUIRED on Windows before tmpDir cleanup

    // Now tmpDir can be cleaned up by Go's test framework
}

// GOOD: Return to original directory before cleanup
func TestChangingWorkDir(t *testing.T) {
    tmpDir := t.TempDir()
    oldDir, _ := os.Getwd()
    defer os.Chdir(oldDir)  // REQUIRED on Windows

    os.Chdir(tmpDir)
    // ... do work ...
}
```

**Symlink Resolution (macOS):**

```go
// GOOD: Resolve symlinks before path comparison
func comparePaths(path1, path2 string) bool {
    // macOS: /tmp -> /private/tmp, /var -> /private/var
    resolved1, _ := filepath.EvalSymlinks(path1)
    resolved2, _ := filepath.EvalSymlinks(path2)
    return resolved1 == resolved2
}

// Handle non-existent paths
func resolvePathSafely(path string) string {
    if resolved, err := filepath.EvalSymlinks(path); err == nil {
        return resolved
    }
    // Path doesn't exist - try parent directory
    parent := filepath.Dir(path)
    if resolved, err := filepath.EvalSymlinks(parent); err == nil {
        return filepath.Join(resolved, filepath.Base(path))
    }
    return filepath.Clean(path)
}
```

**Environment Variables:**

```go
// GOOD: Use testable functions instead of os.UserHomeDir()
func checkConfig() error {
    homeDir, _ := os.UserHomeDir()
    return checkConfigInDir(homeDir)
}

func checkConfigInDir(homeDir string) error {
    // Testable - can pass any directory
    configPath := filepath.Join(homeDir, ".config", "app.yaml")
    // ...
}

// In tests:
func TestCheckConfig(t *testing.T) {
    tmpDir := t.TempDir()
    err := checkConfigInDir(tmpDir)  // Works on all platforms
    // ...
}

// BAD: Relying on environment variables
func TestCheckConfig(t *testing.T) {
    tmpDir := t.TempDir()
    t.Setenv("HOME", tmpDir)  // Doesn't work on Windows
    err := checkConfig()  // Uses os.UserHomeDir() which ignores HOME on Windows
}
```

**Temp Directory Variables:**

```go
// GOOD: Platform-appropriate temp dir env var
func TestTempDir(t *testing.T) {
    tmpEnvVar := "TMPDIR"
    if runtime.GOOS == "windows" {
        tmpEnvVar = "TMP"  // Windows uses TMP or TEMP
    }

    orig := os.Getenv(tmpEnvVar)
    os.Setenv(tmpEnvVar, "/invalid")
    defer os.Setenv(tmpEnvVar, orig)
    // ...
}
```

**File Permissions (Windows):**

```go
// GOOD: Skip permission checks on Windows
func TestFilePermissions(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("File permissions work differently on Windows")
    }

    // Test Unix permissions
    info, _ := os.Stat(path)
    if info.Mode().Perm() != 0o600 {
        t.Errorf("wrong permissions")
    }
}
```

**Platform-Incompatible Commands:**

```go
// GOOD: Skip platform-incompatible tests
func TestPwdCommand(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("pwd command not available on Windows")
    }

    // Test Unix-specific behavior
    output := exec.Command("pwd").Output()
    // ...
}
```

### Common Cross-Platform Pitfalls

**AVOID:**
- Direct string comparison of paths
- Relying on HOME environment variable in tests
- Using Unix-specific commands (pwd, ls, grep) without platform checks
- Leaving files/directories open before cleanup
- Hardcoding path separators (/ or \)
- Assuming symlink behavior (limited on Windows)
- Expecting specific file permission values on Windows

**DO:**
- Use filepath.ToSlash() for path comparisons
- Pass directories explicitly to testable functions
- Use runtime.GOOS for platform-specific logic
- Close all handles before test cleanup
- Use filepath.Join() for path construction
- Resolve symlinks with EvalSymlinks() on macOS
- Skip permission tests on Windows or accept different values

### Testing Your Cross-Platform Changes

**Before pushing changes that touch:**
- File I/O
- Path resolution
- Directory operations
- Environment variables
- Process execution

**Run the full test suite:**

```bash
# This catches most platform issues
go test -race -v ./...

# Check for race conditions (critical)
go test -race ./...

# Look for platform-specific failures in output
# Tests that fail on one platform but not others indicate platform issues
```

**If you don't have access to macOS/Windows:**
- Review your code against the guidelines above
- Pay special attention to path handling and file operations
- The CI will catch platform issues, but it's better to prevent them

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

As of 2026-02-11:

| Metric | Count |
|--------|-------|
| Go test files | 108+ |
| Go Test* functions | ~1222+ |
| Go Fuzz* functions | 4 |
| Frontend test files | 20+ |
| Playwright E2E specs | 5+ |
| Go coverage | 33.2% (partial) |
| CI Status | All workflows passing |
| Platforms tested | Ubuntu, macOS, Windows |

### Recent CI Improvements (2026-02-11)

Fixed 40+ test failures across platforms:
- Data races: 4 tests (root cause of CI failures)
- Frontend lint: 2 critical errors + 15 warnings
- Backend format: 75+ files
- Windows-specific: 25 tests
- macOS-specific: 4 tests
- Logic tests: 1 test

All workflows now passing on all platforms.

For detailed analysis, see `TEST_COVERAGE_REPORT.md`.
