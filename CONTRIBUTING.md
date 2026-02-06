# Contributing to quorum-ai

Thank you for your interest in contributing to quorum-ai. This document provides guidelines and instructions for contributing.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Commit Message Format](#commit-message-format)
- [Pull Request Process](#pull-request-process)
- [Testing Requirements](#testing-requirements)
- [Documentation Requirements](#documentation-requirements)

## Prerequisites

Before contributing, ensure you have the following installed:

- **Go 1.24 or later** - [Installation guide](https://go.dev/doc/install)
- **Git 2.20 or later** - For worktree support
- **golangci-lint** - [Installation guide](https://golangci-lint.run/usage/install/)
- **Make** - For running build commands

Optional but recommended:

- [pre-commit](https://pre-commit.com/) - For automatic pre-commit hooks
- [goreleaser](https://goreleaser.com/) - For testing release builds locally

## Development Setup

1. **Fork and clone the repository**

   ```bash
   git clone https://github.com/YOUR_USERNAME/quorum-ai.git
   cd quorum-ai
   ```

2. **Install dependencies**

   ```bash
   go mod download
   ```

3. **Verify setup**

   ```bash
   make all
   ```

   This runs build, test, and lint to ensure everything is working.

4. **Create a feature branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

## Code Style

This project follows standard Go conventions with additional requirements:

### General Guidelines

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` for formatting (or `goimports` for import organization)
- Keep functions focused and small (< 50 lines recommended)
- Write self-documenting code with clear variable and function names
- Comments should explain "why", not "what"

### Linting

All code must pass golangci-lint:

```bash
make lint
```

To auto-fix issues where possible:

```bash
make lint-fix
```

The linter configuration is in `.golangci.yml`. Do not disable linters without team discussion.

### Package Structure

Follow hexagonal architecture:

- `internal/core/` - Domain entities and port interfaces (no external dependencies)
- `internal/service/` - Business logic and orchestration
- `internal/adapters/` - External integrations (CLI, state, git, github)
- `internal/config/` - Configuration loading and validation
- `internal/tui/` - Terminal UI components
- `cmd/quorum/` - CLI entry point and commands

## Commit Message Format

This project follows [Conventional Commits](https://www.conventionalcommits.org/) v1.0.0.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature (MINOR version bump) |
| `fix` | Bug fix (PATCH version bump) |
| `docs` | Documentation only |
| `style` | Formatting, no logic change |
| `refactor` | Code refactor, no behavior change |
| `perf` | Performance improvement |
| `test` | Add or modify tests |
| `build` | Build system or dependencies |
| `ci` | CI/CD configuration |
| `chore` | Maintenance tasks |

### Rules

- Subject line: max 72 characters, imperative mood, no period
- Breaking changes: add `!` after type/scope and `BREAKING CHANGE:` in footer
- Reference issues: `Refs: #123` or `Closes: #456`

### Examples

```
feat(consensus): add weighted Jaccard similarity

Implement category-weighted consensus scoring with configurable
weights for claims, risks, and recommendations.

Refs: #42
```

```
fix(state): prevent data loss on concurrent writes

Add file locking with stale lock detection to prevent race
conditions during checkpoint saves.

Closes: #87
```

## Pull Request Process

1. **Before submitting**

   - Ensure all tests pass: `make test`
   - Ensure linting passes: `make lint`
   - Update documentation if needed
   - Add tests for new functionality

2. **Create the PR**

   - Use a descriptive title following commit conventions
   - Fill out the PR template completely
   - Link related issues
   - Request review from maintainers

3. **Review process**

   - Address all review comments
   - Keep commits clean (squash fixups if requested)
   - Ensure CI passes

4. **After merge**

   - Delete your feature branch
   - Verify the change in the main branch

### PR Size Guidelines

- Prefer small, focused PRs (< 400 lines changed)
- Split large features into incremental PRs
- Each PR should be independently reviewable and testable

## Testing Requirements

### Coverage

- Target: 80% code coverage for new code
- Check coverage: `make test-coverage`

### Test Types

| Type | Location | Purpose |
|------|----------|---------|
| Unit tests | `*_test.go` next to source | Test individual functions |
| Integration tests | `*_test.go` with build tags | Test component interactions |
| E2E tests | `testdata/` | Test full workflows |

### Test Conventions

- Use table-driven tests for multiple cases
- Name tests descriptively: `TestFunctionName_Scenario_ExpectedResult`
- Use `testify` assertions sparingly; prefer standard library
- Mock external dependencies using interfaces

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test ./internal/service/...

# With verbose output
go test -v ./...
```

## Documentation Requirements

### Code Documentation

- All exported functions, types, and constants must have doc comments
- Doc comments should start with the element name
- Include examples for complex APIs

### Project Documentation

- Update README.md for user-facing changes
- Update CHANGELOG.md following [Keep a Changelog](https://keepachangelog.com/) format
- Add ADRs for significant architectural decisions in `docs/adr/`

## Adding Agents, Models and Reasoning Levels

If you need to add a new AI agent, a new model to an existing agent, or a new reasoning effort level, see [docs/ADDING_AGENTS.md](docs/ADDING_AGENTS.md). It lists every file you need to touch and the order to follow.

## Questions and Support

- Open a [GitHub Issue](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/new/choose) for bugs or features
- Check existing issues before creating new ones
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

---

Thank you for contributing to quorum-ai!
