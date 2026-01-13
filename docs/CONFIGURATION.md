# Configuration Reference

This document describes all configuration options available in quorum-ai. Configuration files use YAML format and are loaded from `.quorum.yaml` in the project root.

## Table of Contents

- [Overview](#overview)
- [Configuration File Location](#configuration-file-location)
- [Sections](#sections)
  - [log](#log)
  - [trace](#trace)
  - [workflow](#workflow)
  - [agents](#agents)
  - [prompt_optimizer](#prompt_optimizer)
  - [state](#state)
  - [git](#git)
  - [github](#github)
  - [consensus](#consensus)
  - [costs](#costs)

---

## Overview

quorum-ai uses a layered configuration system:

1. **Built-in defaults** - Sensible defaults for all options
2. **Global config** - `~/.config/quorum/config.yaml` (user-level)
3. **Project config** - `.quorum.yaml` in project root (project-level)
4. **Environment variables** - `QUORUM_*` prefix overrides
5. **CLI flags** - Highest priority overrides

Later sources override earlier ones. Generate a starter configuration with:

```bash
quorum init
```

---

## Configuration File Location

| Location | Purpose |
|----------|---------|
| `~/.config/quorum/config.yaml` | User-level defaults |
| `.quorum.yaml` | Project-specific settings |
| `configs/default.yaml` | Reference template (do not edit) |

---

## Sections

### log

Controls logging output format and verbosity.

```yaml
log:
  level: info
  format: auto
  file: ""
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `format` | string | `auto` | Output format: `auto` (detect TTY), `text`, `json` |
| `file` | string | `""` | Optional log file path. Empty writes to stdout only |

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
  include_phases: [optimize, analyze, consensus, plan, execute]
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `off` | Trace mode: `off`, `summary`, `full` |
| `dir` | string | `.quorum/traces` | Directory for trace artifacts |
| `schema_version` | int | `1` | Trace schema version for compatibility |
| `redact` | bool | `true` | Redact sensitive values in traces |
| `redact_patterns` | []string | `[]` | Additional regex patterns to redact |
| `redact_allowlist` | []string | `[]` | Regex patterns to exclude from redaction |
| `max_bytes` | int | `262144` | Maximum bytes per trace file (256KB) |
| `total_max_bytes` | int | `10485760` | Maximum total bytes per run (10MB) |
| `max_files` | int | `500` | Maximum number of files per run |
| `include_phases` | []string | all | Phases to include in tracing |

**Trace modes:**

- `off` - No tracing
- `summary` - Only `run.json` manifest and `trace.jsonl` events
- `full` - Includes prompt/response payload files (subject to size limits)

---

### workflow

Controls workflow execution behavior.

```yaml
workflow:
  timeout: 2h
  max_retries: 3
  dry_run: false
  sandbox: false
  deny_tools: []
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `2h` | Maximum workflow execution time |
| `max_retries` | int | `3` | Maximum retry attempts per failed task |
| `dry_run` | bool | `false` | Simulate execution without running agents |
| `sandbox` | bool | `false` | Restrict dangerous operations |
| `deny_tools` | []string | `[]` | Tool names to deny during execution |

---

### agents

Configures LLM agent backends. Each agent can define a default model and per-phase model overrides.

```yaml
agents:
  default: claude

  claude:
    enabled: true
    path: claude
    model: claude-sonnet-4-5-20250929
    phase_models:
      optimize: claude-opus-4-5-20251101
      analyze: claude-opus-4-5-20251101
      plan: claude-sonnet-4-5-20250929
      execute: claude-haiku-4-5-20251001
    max_tokens: 4096
    temperature: 0.7
```

#### Agent Selection

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default` | string | `claude` | Default agent for single-agent operations |

#### Common Agent Fields

Each agent configuration supports these fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | varies | Enable/disable this agent |
| `path` | string | agent name | Path to CLI executable |
| `model` | string | varies | Default model (fallback when phase not specified) |
| `phase_models` | map | `{}` | Per-phase model overrides |
| `max_tokens` | int | `4096` | Maximum output tokens |
| `temperature` | float | `0.7` | Sampling temperature (0.0-2.0) |

#### Model Resolution Order

Models are resolved in this priority order:

1. **Task-specific model** - Passed via CLI or task definition
2. **Phase model** - From `phase_models.<phase>` (optimize/analyze/plan/execute)
3. **Default model** - From `model` field (fallback)

If `phase_models` defines all four phases, the `model` field serves only as documentation or for non-workflow operations.

#### Claude Configuration

```yaml
claude:
  enabled: true
  path: claude
  model: claude-sonnet-4-5-20250929
  phase_models:
    optimize: claude-opus-4-5-20251101
    analyze: claude-opus-4-5-20251101
    plan: claude-sonnet-4-5-20250929
    execute: claude-haiku-4-5-20251001
```

**Available models (as of January 2025):**

| Model ID | Description | Best For |
|----------|-------------|----------|
| `claude-opus-4-5-20251101` | Maximum intelligence | Complex analysis, research |
| `claude-sonnet-4-5-20250929` | Balanced performance | General tasks, planning |
| `claude-haiku-4-5-20251001` | Fastest, near-frontier | Execution, high-volume |
| `claude-opus-4-1-20250805` | Legacy Opus | Migration only |
| `claude-sonnet-4-20250514` | Legacy Sonnet | Migration only |

#### Gemini Configuration

```yaml
gemini:
  enabled: true
  path: gemini
  model: gemini-2.5-flash
  phase_models:
    optimize: gemini-2.5-pro
    analyze: gemini-2.5-pro
    plan: gemini-2.5-flash
    execute: gemini-2.5-flash
```

**Available models (as of January 2025):**

| Model ID | Description | Best For |
|----------|-------------|----------|
| `gemini-3-pro` | Preview - Most capable | Complex analysis |
| `gemini-3-flash` | Preview - Fast and capable | General tasks |
| `gemini-2.5-pro` | GA - High capability | Production analysis |
| `gemini-2.5-flash` | GA - Balanced | Production general |
| `gemini-2.5-flash-lite` | GA - Lightweight | High-volume tasks |

#### Codex Configuration

```yaml
codex:
  enabled: false
  path: codex
  model: o4-mini
  phase_models:
    optimize: o3
    analyze: o3
    plan: o4-mini
    execute: o4-mini
```

**Available models (as of January 2025):**

| Model ID | Description | Best For |
|----------|-------------|----------|
| `o3` | Reasoning model | Complex analysis |
| `o4-mini` | Fast reasoning | General tasks |
| `gpt-4.1` | GPT-4 Turbo | Legacy compatibility |
| `codex` | Code-specialized | Code generation |

> **Note:** Model availability depends on your OpenAI account tier. Some models (e.g., `o3`) require API access.

#### Copilot Configuration

```yaml
copilot:
  enabled: false
  path: copilot
  model: claude-sonnet-4-5
  phase_models:
    optimize: claude-sonnet-4-5
    analyze: claude-sonnet-4-5
    plan: claude-sonnet-4-5
    execute: claude-sonnet-4-5
```

GitHub Copilot CLI is a standalone coding agent (`npm install -g @github/copilot`) that replaced the deprecated `gh copilot` extension.

**Installation:**
```bash
npm install -g @github/copilot
copilot /login  # Authenticate with GitHub
```

**Available models:**

| Model ID | Description | Best For |
|----------|-------------|----------|
| `claude-sonnet-4-5` | Claude Sonnet 4.5 (default) | General tasks |
| `claude-sonnet-4` | Claude Sonnet 4 | Balanced |
| `gpt-5` | GPT-5 | Alternative |

**YOLO mode flags (auto-enabled by quorum-ai):**
- `--allow-all-tools` - Auto-approve all tool usage
- `--allow-all-paths` - Disable path verification
- `--allow-all-urls` - Disable URL verification

> **Note:** Requires GitHub Copilot Pro, Pro+, Business, or Enterprise subscription.

---

### prompt_optimizer

Configures the prompt optimization phase that runs before analysis.

```yaml
prompt_optimizer:
  enabled: true
  agent: claude
  model: ""
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable/disable prompt optimization |
| `agent` | string | `claude` | Agent to use for optimization |
| `model` | string | `""` | Model override (uses agent's `phase_models.optimize` if empty) |

**How it works:**

1. User provides a prompt to `quorum run`
2. The optimizer enhances the prompt for clarity and LLM effectiveness
3. The optimized prompt is used for all subsequent phases
4. Original prompt is preserved in state for reference

**Model resolution for optimization:**

1. `prompt_optimizer.model` if specified
2. Agent's `phase_models.optimize` if defined
3. Agent's default `model` as fallback

**Disabling optimization:**

```bash
# Via CLI flag
quorum run --skip-optimize "your prompt"

# Via configuration
prompt_optimizer:
  enabled: false
```

**Behavior in special modes:**

- **Dry-run mode**: Optimization is skipped, original prompt is used
- **Individual phase commands** (`quorum analyze`, etc.): Optimization is skipped
- **Optimization failure**: Falls back to original prompt with a warning

---

### state

Configures workflow state persistence for resume capability.

```yaml
state:
  path: .quorum/state/state.json
  backup_path: .quorum/state/state.json.bak
  lock_ttl: 1h
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `.quorum/state/state.json` | Primary state file path |
| `backup_path` | string | `.quorum/state/state.json.bak` | Backup state file path |
| `lock_ttl` | duration | `1h` | Lock file TTL before considered stale |

---

### git

Configures git integration and worktree isolation.

```yaml
git:
  worktree_dir: .worktrees
  auto_clean: true
  worktree_mode: parallel
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `worktree_dir` | string | `.worktrees` | Directory for git worktrees |
| `auto_clean` | bool | `true` | Auto-cleanup completed worktrees |
| `worktree_mode` | string | `parallel` | Worktree creation: `always`, `parallel`, `disabled` |

**Worktree modes:**

- `always` - Create worktree for every task
- `parallel` - Create worktrees only for parallel task execution
- `disabled` - Execute all tasks in main working directory

---

### github

Configures GitHub integration for PR creation and issue tracking.

```yaml
github:
  token: ""
  remote: origin
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `token` | string | `""` | GitHub token (prefer `GITHUB_TOKEN` env var) |
| `remote` | string | `origin` | Git remote name for GitHub operations |

> **Security:** Never commit tokens in configuration files. Use environment variables or credential helpers.

---

### consensus

Configures the multi-agent consensus validation system.

```yaml
consensus:
  threshold: 0.80
  v2_threshold: 0.60
  human_threshold: 0.50
  weights:
    claims: 0.40
    risks: 0.30
    recommendations: 0.30
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `threshold` | float | `0.80` | Minimum consensus score to proceed (0.0-1.0) |
| `v2_threshold` | float | `0.60` | Score below this triggers V2 critique phase |
| `human_threshold` | float | `0.50` | Score below this aborts for human review |
| `weights.claims` | float | `0.40` | Weight for claims/findings similarity |
| `weights.risks` | float | `0.30` | Weight for risks/concerns similarity |
| `weights.recommendations` | float | `0.30` | Weight for recommendations similarity |

**Escalation policy (80/60/50):**

```
Score >= 80%  →  Proceed to next phase
Score 60-79%  →  Trigger V2 critique (cross-agent review)
Score 50-59%  →  Trigger V3 synthesis (consolidation attempt)
Score < 50%   →  Abort workflow, require human review
```

---

### costs

Configures cost tracking and limits.

```yaml
costs:
  max_per_workflow: 10.0
  max_per_task: 2.0
  alert_threshold: 0.80
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_per_workflow` | float | `10.0` | Maximum USD per workflow (0 = unlimited) |
| `max_per_task` | float | `2.0` | Maximum USD per task (0 = unlimited) |
| `alert_threshold` | float | `0.80` | Warn when cost reaches this fraction of limit |

---

## Environment Variables

All configuration options can be overridden via environment variables using the `QUORUM_` prefix:

```bash
QUORUM_LOG_LEVEL=debug
QUORUM_WORKFLOW_TIMEOUT=4h
QUORUM_AGENTS_CLAUDE_MODEL=claude-opus-4-5-20251101
QUORUM_CONSENSUS_THRESHOLD=0.85
```

Nested keys use underscores: `agents.claude.model` → `QUORUM_AGENTS_CLAUDE_MODEL`

---

## Example Configurations

### Minimal Configuration

```yaml
agents:
  claude:
    enabled: true
  gemini:
    enabled: true

consensus:
  threshold: 0.80
```

### High-Quality Analysis

```yaml
agents:
  default: claude
  claude:
    enabled: true
    phase_models:
      optimize: claude-opus-4-5-20251101
      analyze: claude-opus-4-5-20251101
      plan: claude-sonnet-4-5-20250929
      execute: claude-haiku-4-5-20251001
  gemini:
    enabled: true
    phase_models:
      optimize: gemini-2.5-pro
      analyze: gemini-2.5-pro
      plan: gemini-2.5-flash
      execute: gemini-2.5-flash

prompt_optimizer:
  enabled: true
  agent: claude

consensus:
  threshold: 0.85
  v2_threshold: 0.70

costs:
  max_per_workflow: 25.0
```

### Cost-Conscious Configuration

```yaml
agents:
  default: gemini
  claude:
    enabled: false
  gemini:
    enabled: true
    model: gemini-2.5-flash-lite

# Disable optimization to reduce costs
prompt_optimizer:
  enabled: false

consensus:
  threshold: 0.75

costs:
  max_per_workflow: 5.0
  max_per_task: 1.0
```

### Development/Debug Configuration

```yaml
log:
  level: debug
  format: text

trace:
  mode: full
  redact: false

workflow:
  dry_run: true

agents:
  claude:
    enabled: true
    model: claude-haiku-4-5-20251001
```

---

## Validation

Validate your configuration with:

```bash
quorum doctor
```

This checks:
- YAML syntax validity
- Required fields presence
- Agent CLI availability
- Model identifier format
- Threshold value ranges
