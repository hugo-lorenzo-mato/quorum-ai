# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project structure
- Core domain entities and ports
- Service layer orchestration
- CLI and TUI foundations
- State persistence and git worktree support
- **Single-Agent Execution Mode** for analyze phase:
  - New `--single-agent <agent>` CLI flag to bypass multi-agent consensus
  - New `--single-agent-model <model>` CLI flag for optional model override
  - Configuration via `phases.analyze.single_agent.enabled/agent/model`
  - Produces compatible `consolidated_analysis` checkpoint for downstream phases
  - Lower cost and latency for simpler, well-defined tasks

### Changed
- None

### Deprecated
- None

### Removed
- None

### Fixed
- `ValidateModeratorConfig` now correctly counts enabled agents using `ListEnabled()` instead of all registered agent factories

### Security
- None

[Unreleased]: https://github.com/hugo-lorenzo-mato/quorum-ai/commits/main
