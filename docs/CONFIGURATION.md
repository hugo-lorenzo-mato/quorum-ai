# Configuration Reference

Complete reference for all quorum-ai configuration options.

## Table of Contents

- [Overview](#overview)
- [Configuration File Location](#configuration-file-location)
- [Configuration Sections](#configuration-sections)
  - [log](#log)
  - [trace](#trace)
  - [workflow](#workflow)
  - [phases](#phases)
  - [agents](#agents)
  - [state](#state)
  - [git](#git)
  - [github](#github)
  - [chat](#chat)
  - [report](#report)
  - [diagnostics](#diagnostics)
- [Environment Variables](#environment-variables)
- [Example Configurations](#example-configurations)
- [Validation](#validation)

---

## Overview

quorum-ai uses a layered configuration system with the following precedence (highest to lowest):

1. **CLI flags** - Command-line arguments
2. **Environment variables** - `QUORUM_*` prefix
3. **Project config** - `.quorum/config.yaml` in project root
4. **Legacy project config** - `.quorum.yaml` (backward compatibility)
5. **Global config** - `~/.config/quorum/config.yaml`
6. **Built-in defaults** - Sensible defaults for all options

Generate a starter configuration:

```bash
quorum init
```

---

## Configuration File Location

| Location | Purpose |
|----------|---------|
| `.quorum/config.yaml` | Project-specific settings (recommended) |
| `.quorum.yaml` | Legacy project settings |
| `~/.config/quorum/config.yaml` | User-level defaults |
| `configs/default.yaml` | Reference template (do not edit) |

---

## Configuration Sections

### log

Controls logging output.

```yaml
log:
  level: info
  format: auto
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `format` | string | `auto` | Output format: `auto` (detect TTY), `text`, `json` |

---

### trace

Configures execution tracing for debugging and auditing.

```yaml
trace:
  mode: off
  dir: .quorum/traces
  schema_version: 1
  redact: true
  redact_patterns: []
  redact_allowlist: []
  max_bytes: 262144
  total_max_bytes: 10485760
  max_files: 500
  include_phases: [refine, analyze, plan, execute]
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `off` | Trace mode: `off`, `summary`, `full` |
| `dir` | string | `.quorum/traces` | Directory for trace artifacts |
| `schema_version` | int | `1` | Trace schema version |
| `redact` | bool | `true` | Redact sensitive values |
| `redact_patterns` | []string | `[]` | Additional regex patterns to redact |
| `redact_allowlist` | []string | `[]` | Patterns to exclude from redaction |
| `max_bytes` | int | `262144` | Max bytes per trace file (256KB) |
| `total_max_bytes` | int | `10485760` | Max total bytes per run (10MB) |
| `max_files` | int | `500` | Max files per run |
| `include_phases` | []string | all | Phases to trace |

**Trace modes:**

| Mode | Description |
|------|-------------|
| `off` | No tracing |
| `summary` | Only manifest and event log |
| `full` | Includes prompt/response payloads |

---

### workflow

Controls workflow execution behavior.

```yaml
workflow:
  timeout: 12h
  max_retries: 3
  dry_run: false
  sandbox: true
  deny_tools: []
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `12h` | Maximum workflow execution time |
| `max_retries` | int | `3` | Retry attempts per failed task (0-10) |
| `dry_run` | bool | `false` | Simulate without running agents |
| `sandbox` | bool | `true` | Restrict dangerous operations |
| `deny_tools` | []string | `[]` | Tool names to block during execution |

---

### phases

Configures per-phase settings.

```yaml
phases:
  analyze:
    timeout: 2h
    refiner:
      enabled: true
      agent: codex
    synthesizer:
      agent: claude
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.90
      min_rounds: 2
      max_rounds: 5
      abort_threshold: 0.30
      stagnation_threshold: 0.02
    single_agent:
      enabled: false
      agent: ""
      model: ""
  plan:
    timeout: 1h
    synthesizer:
      enabled: false
      agent: claude
  execute:
    timeout: 2h
```

#### phases.analyze

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `2h` | Maximum duration |
| `refiner.enabled` | bool | `false` | Enable prompt refinement |
| `refiner.agent` | string | - | Agent for refinement |
| `synthesizer.agent` | string | - | Agent to synthesize analyses |
| `moderator.enabled` | bool | `false` | Enable consensus evaluation |
| `moderator.agent` | string | - | Agent for moderation |
| `moderator.threshold` | float | `0.85` | Consensus score to proceed (0.0-1.0) |
| `moderator.min_rounds` | int | `2` | Minimum refinement rounds |
| `moderator.max_rounds` | int | `5` | Maximum refinement rounds |
| `moderator.abort_threshold` | float | `0.30` | Score below this aborts workflow |
| `moderator.stagnation_threshold` | float | `0.02` | Minimum improvement between rounds |
| `single_agent.enabled` | bool | `false` | Bypass multi-agent consensus |
| `single_agent.agent` | string | - | Agent for single-agent mode |
| `single_agent.model` | string | - | Optional model override |

#### phases.plan

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `1h` | Maximum duration |
| `synthesizer.enabled` | bool | `false` | Enable multi-agent planning |
| `synthesizer.agent` | string | - | Agent to synthesize plans |

#### phases.execute

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `2h` | Maximum duration |

#### Prompt Refiner

Enhances user prompts before analysis for better LLM effectiveness.

**Behavior:**
- Refines prompt for clarity and LLM effectiveness
- Original prompt preserved in state
- Skipped in dry-run mode and individual phase commands
- Falls back to original on failure

```bash
# Skip refinement via CLI
quorum run --skip-refine "your prompt"
```

#### Semantic Moderator

Evaluates consensus across agent outputs using weighted divergence scoring.

**Consensus flow:**

```
Analysis (all agents) → Refinement → Moderator → Score ≥ threshold? → Synthesize
                                         ↓ No
                                    Refine again (up to max_rounds)
```

**Divergence weights:**

| Impact | Weight | Examples |
|--------|--------|----------|
| High | Major | Architecture, security, breaking changes |
| Medium | Moderate | Implementation details, edge cases |
| Low | Minimal | Naming, style, documentation |

#### Single-Agent Mode

Bypasses multi-agent consensus. Mutually exclusive with `moderator.enabled`.

```yaml
phases:
  analyze:
    single_agent:
      enabled: true
      agent: claude
    moderator:
      enabled: false  # Required when single_agent is enabled
```

---

### agents

Configures LLM agent backends.

```yaml
agents:
  default: claude

  claude:
    enabled: true
    path: claude
    model: claude-opus-4-5-20251101
    phase_models:
      execute: claude-sonnet-4-5-20250929
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true
    token_discrepancy_threshold: 5.0

  codex:
    enabled: true
    path: codex
    model: gpt-5.2-codex
    reasoning_effort: high
    reasoning_effort_phases:
      refine: xhigh
      analyze: xhigh
```

#### Agent Selection

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default` | string | `claude` | Default agent for single-agent operations |

#### Common Agent Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | varies | Enable/disable agent |
| `path` | string | agent name | Path to CLI executable |
| `model` | string | varies | Default model |
| `phase_models` | map | `{}` | Per-phase model overrides |
| `phases` | map | `{}` | Phase participation (opt-in) |
| `token_discrepancy_threshold` | float | `5.0` | Token validation threshold |

#### Codex-Specific Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `reasoning_effort` | string | - | Default reasoning effort: `minimal`, `low`, `medium`, `high`, `xhigh` |
| `reasoning_effort_phases` | map | `{}` | Per-phase reasoning effort overrides |

#### OpenCode Agent

OpenCode is an MCP-compatible software engineering agent that connects to local LLMs via Ollama. It supports intelligent model selection based on task type.

```yaml
agents:
  opencode:
    enabled: false
    path: opencode
    model: qwen2.5-coder
    phase_models:
      refine: llama3.1
      analyze: llama3.1
      moderate: llama3.1
      synthesize: llama3.1
      plan: llama3.1
      execute: qwen2.5-coder
    phases:
      analyze: true
      plan: true
      execute: true
```

**Requirements:**
- OpenCode CLI installed ([https://opencode.ai/docs/cli/](https://opencode.ai/docs/cli/))
- Ollama running at `localhost:11434` with compatible models

**Model Profiles:**

| Profile | Models | Use Case |
|---------|--------|----------|
| Coder | `qwen2.5-coder`, `deepseek-coder-v2` | Code generation, editing, execution |
| Architect | `llama3.1`, `deepseek-r1` | Analysis, planning, architecture review |

**Environment Setup:**
```bash
export OPENAI_BASE_URL=http://localhost:11434/v1
```

#### Phase Participation (opt-in model)

The `phases` map controls which phases an agent participates in:

- **Only phases set to `true` are enabled**
- **Omitted phases are disabled**
- **Empty or missing `phases` = enabled for all** (backward compatible)

Available phases: `refine`, `analyze`, `moderate`, `synthesize`, `plan`, `execute`

```yaml
# Agent only as moderator
copilot:
  enabled: true
  phases:
    moderate: true
    # All others omitted = disabled

# Agent for analysis and execution
gemini:
  enabled: true
  phases:
    analyze: true
    execute: true
```

#### Model Resolution Order

1. Task-specific model (CLI/task definition)
2. Phase model (`phase_models.<phase>`)
3. Default model (`model`)

#### Token Discrepancy Detection

Validates reported token counts against estimates.

| Value | Behavior |
|-------|----------|
| `5` (default) | Reported must be 1/5 to 5x of estimated |
| `0` | Disable validation |

#### Multiple Agents with Same CLI

Define multiple entries using the same CLI for multi-agent analysis:

```yaml
agents:
  copilot-claude:
    enabled: true
    path: copilot
    model: claude-sonnet-4-5
    phases:
      analyze: true

  copilot-gpt:
    enabled: true
    path: copilot
    model: gpt-5
    phases:
      analyze: true
```

---

### state

Configures workflow state persistence.

```yaml
state:
  backend: sqlite
  path: .quorum/state/state.db
  backup_path: .quorum/state/state.db.bak
  lock_ttl: 1h
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `backend` | string | `sqlite` | Storage: `sqlite` (database) or `json` (file) |
| `path` | string | `.quorum/state/state.db` | State database path |
| `backup_path` | string | `.quorum/state/state.db.bak` | Backup file path |
| `lock_ttl` | duration | `1h` | Lock TTL before stale |

**Backend comparison:**

| Backend | Best For |
|---------|----------|
| `sqlite` | Large workflows, concurrent access, performance (default) |
| `json` | Simple setups, debugging, human-readable |

**Path extension handling:**

Extensions are automatically adjusted when switching backends:

| Configured | Backend | Actual |
|------------|---------|--------|
| `state.db` | `json` | `state.json` |
| `state.json` | `sqlite` | `state.db` |

---

### git

Configures git integration and task finalization.

```yaml
git:
  worktree_dir: .worktrees
  auto_clean: true
  worktree_mode: parallel
  auto_commit: true
  auto_push: true
  auto_pr: true
  pr_base_branch: ""
  auto_merge: false
  merge_strategy: squash
```

#### Worktree Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `worktree_dir` | string | `.worktrees` | Worktree directory |
| `auto_clean` | bool | `true` | Remove worktrees after completion |
| `worktree_mode` | string | `parallel` | When to create worktrees |

**Worktree modes:**

| Mode | Description |
|------|-------------|
| `always` | Every task gets its own worktree |
| `parallel` | Only when 2+ tasks run concurrently (recommended) |
| `disabled` | All tasks share main working directory |

#### Post-Task Finalization

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `auto_commit` | bool | `true` | Commit changes after task |
| `auto_push` | bool | `true` | Push branch to remote |
| `auto_pr` | bool | `true` | Create pull request |
| `pr_base_branch` | string | `""` | PR target branch (empty = repo default) |
| `auto_merge` | bool | `false` | Merge PR immediately |
| `merge_strategy` | string | `squash` | Merge method: `merge`, `squash`, `rebase` |

**Finalization flow:**

1. Task completes on branch `quorum/<task-id>`
2. `auto_commit` → commit changes
3. `auto_push` → push to remote
4. `auto_pr` → create PR targeting `pr_base_branch`
5. `auto_merge` → merge using `merge_strategy`

> **Warning:** `auto_merge` is disabled by default. Enable only for automated pipelines.

---

### github

Configures GitHub integration.

```yaml
github:
  remote: origin
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `remote` | string | `origin` | Git remote name |

> **Authentication:** Provide token via `GITHUB_TOKEN` or `GH_TOKEN` environment variable.

---

### chat

Configures TUI chat behavior.

```yaml
chat:
  timeout: 3m
  progress_interval: 15s
  editor: vim
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `3m` | Chat message timeout |
| `progress_interval` | duration | `15s` | Progress log interval |
| `editor` | string | `vim` | Editor for file editing (`vim`, `nvim`, `code`) |

---

### report

Configures markdown report generation.

```yaml
report:
  enabled: true
  base_dir: .quorum/runs
  use_utc: true
  include_raw: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable report generation |
| `base_dir` | string | `.quorum/runs` | Output directory |
| `use_utc` | bool | `true` | Use UTC timestamps |
| `include_raw` | bool | `true` | Include raw agent outputs |

---

### diagnostics

Configures system diagnostics for process resilience.

```yaml
diagnostics:
  enabled: true

  resource_monitoring:
    interval: 30s
    fd_threshold_percent: 80
    goroutine_threshold: 10000
    memory_threshold_mb: 4096
    history_size: 120

  crash_dump:
    dir: .quorum/crashdumps
    max_files: 10
    include_stack: true
    include_env: false

  preflight_checks:
    enabled: true
    min_free_fd_percent: 20
    min_free_memory_mb: 256
```

#### diagnostics (root)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable diagnostics subsystem |

#### diagnostics.resource_monitoring

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interval` | duration | `30s` | Snapshot interval |
| `fd_threshold_percent` | int | `80` | FD usage warning threshold (0-100) |
| `goroutine_threshold` | int | `10000` | Goroutine count warning threshold |
| `memory_threshold_mb` | int | `4096` | Heap memory warning threshold (MB) |
| `history_size` | int | `120` | Snapshots to retain |

#### diagnostics.crash_dump

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dir` | string | `.quorum/crashdumps` | Crash dump directory |
| `max_files` | int | `10` | Max dumps to retain |
| `include_stack` | bool | `true` | Include stack traces |
| `include_env` | bool | `false` | Include environment (redacted) |

#### diagnostics.preflight_checks

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable preflight checks |
| `min_free_fd_percent` | int | `20` | Minimum free FD percentage |
| `min_free_memory_mb` | int | `256` | Minimum free memory (MB) |

---

## Environment Variables

Override any configuration via `QUORUM_` prefix:

```bash
QUORUM_LOG_LEVEL=debug
QUORUM_WORKFLOW_TIMEOUT=4h
QUORUM_AGENTS_CLAUDE_MODEL=claude-opus-4-5-20251101
QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD=0.95
```

Nested keys use underscores: `phases.analyze.moderator.threshold` → `QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD`

---

## Example Configurations

### Multi-Agent Analysis (Default)

```yaml
phases:
  analyze:
    refiner:
      enabled: true
      agent: codex
    synthesizer:
      agent: claude
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.90

agents:
  default: claude
  claude:
    enabled: true
    phases:
      analyze: true
      synthesize: true
      plan: true
      execute: true
  gemini:
    enabled: true
    phases:
      analyze: true
      execute: true
  codex:
    enabled: true
    phases:
      refine: true
      analyze: true
  copilot:
    enabled: true
    phases:
      moderate: true
```

### Single-Agent Mode

```yaml
phases:
  analyze:
    single_agent:
      enabled: true
      agent: claude
    moderator:
      enabled: false

agents:
  default: claude
  claude:
    enabled: true
```

### Debug Configuration

```yaml
log:
  level: debug

trace:
  mode: full
  redact: false

workflow:
  dry_run: true
```

---

## Validation

Validate configuration:

```bash
quorum doctor
```

Checks:
- YAML syntax
- Required fields
- Agent CLI availability
- Model identifier format
- Threshold value ranges
