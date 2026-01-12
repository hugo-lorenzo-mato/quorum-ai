# Roadmap

This roadmap outlines planned milestones for quorum-ai. Items are grouped by
version to show progression from the current POC to long-term goals. This is a
living document and does not include specific dates.

## Legend

- Status: Complete, In Progress, Planned
- Commitment: Committed, Tentative
- Priority: P0 (high), P1 (medium), P2 (low)

## v1.0 (Current POC)

Committed items for the initial release.

| Status | Priority | Commitment | Item | Issues |
| --- | --- | --- | --- | --- |
| Complete | P0 | Committed | Core CLI with analyze/plan/execute commands | [#71](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/71) |
| Complete | P0 | Committed | JSON-based state persistence | [#47](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/47) |
| Complete | P0 | Committed | Git worktree isolation | [#64](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/64) |
| In Progress | P0 | Committed | Claude and Gemini adapters | [#57](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/57), [#58](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/58) |
| Complete | P1 | Committed | Jaccard consensus algorithm | [#49](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/49) |
| In Progress | P1 | Committed | Basic TUI with progress display | [#68](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/68), [#69](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/69) |

## v2.0 (Future)

Planned enhancements to improve scalability and extensibility.

| Status | Priority | Commitment | Item | Issues |
| --- | --- | --- | --- | --- |
| Planned | P0 | Committed | SQLite state persistence with migrations | N/A |
| Planned | P0 | Committed | Additional CLI adapters (Codex, Copilot, Aider) | [#59](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/59), [#60](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/60), [#61](https://github.com/hugo-lorenzo-mato/quorum-ai/issues/61) |
| Planned | P1 | Committed | Analytics dashboard | N/A |
| Planned | P1 | Committed | Plugin architecture foundation | N/A |
| Planned | P1 | Committed | Performance optimizations | N/A |

## v3.0 (Vision)

Long-term goals that are tentative and subject to change.

| Status | Priority | Commitment | Item | Issues |
| --- | --- | --- | --- | --- |
| Planned | P2 | Tentative | Full plugin system for custom adapters | N/A |
| Planned | P2 | Tentative | Web dashboard for workflow monitoring | N/A |
| Planned | P2 | Tentative | Multi-repository orchestration | N/A |
| Planned | P2 | Tentative | Custom consensus algorithms | N/A |
| Planned | P2 | Tentative | Team collaboration features | N/A |

## How to Contribute

- Check open issues and feature requests: https://github.com/hugo-lorenzo-mato/quorum-ai/issues
- Look for beginner-friendly work: https://github.com/hugo-lorenzo-mato/quorum-ai/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22
- Review the contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- For security-related topics, follow [SECURITY.md](SECURITY.md)
