# Configuration Reference

This document describes all configuration options available in quorum-ai. Configuration files use YAML format and are loaded from `.quorum.yaml` in the project root.

## Table of Contents

- [Overview](#overview)
- [Configuration File Location](#configuration-file-location)
- [Sections](#sections)
  - [log](#log)
  - [trace](#trace)
  - [workflow](#workflow)
  - [phases](#phases)
  - [agents](#agents)
  - [state](#state)
  - [git](#git)
  - [github](#github)
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
  include_phases: [refine, analyze, plan, execute]
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
  timeout: 12h
  max_retries: 3
  dry_run: false
  sandbox: true
  deny_tools: []
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `12h` | Maximum workflow execution time |
| `max_retries` | int | `3` | Maximum retry attempts per failed task |
| `dry_run` | bool | `false` | Simulate execution without running agents |
| `sandbox` | bool | `true` | Restrict dangerous operations (security default) |
| `deny_tools` | []string | `[]` | Tool names to deny during execution |

---

### phases

Configures per-phase settings including timeouts and phase-specific components.

```yaml
phases:
  analyze:
    timeout: 2h
    refiner:
      enabled: true
      agent: claude
    synthesizer:
      agent: claude
    moderator:
      enabled: true
      agent: claude
      threshold: 0.90
      min_rounds: 2
      max_rounds: 5
      abort_threshold: 0.30
      stagnation_threshold: 0.02
  plan:
    timeout: 1h
    synthesizer:
      enabled: false
      agent: claude
  execute:
    timeout: 2h
```

#### Analyze Phase

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `2h` | Maximum duration for analyze phase |
| `refiner.enabled` | bool | `false` | Enable prompt refinement before analysis |
| `refiner.agent` | string | - | Agent to use for refinement (model from `phase_models.refine`) |
| `synthesizer.agent` | string | - | Agent to synthesize multi-agent analyses (model from `phase_models.analyze`) |
| `moderator.enabled` | bool | `false` | Enable semantic moderator for consensus evaluation |
| `moderator.agent` | string | - | Agent to use as moderator (model from `phase_models.analyze`) |
| `moderator.threshold` | float | `0.90` | Minimum consensus score to proceed (0.0-1.0) |
| `moderator.min_rounds` | int | `2` | Minimum refinement rounds before consensus can be declared |
| `moderator.max_rounds` | int | `5` | Maximum refinement rounds before aborting |
| `moderator.abort_threshold` | float | `0.30` | Score below this aborts workflow |
| `moderator.stagnation_threshold` | float | `0.02` | Minimum improvement required between rounds |

#### Plan Phase

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `1h` | Maximum duration for plan phase |
| `synthesizer.enabled` | bool | `false` | Enable multi-agent plan synthesis |
| `synthesizer.agent` | string | - | Agent to synthesize plans (model from `phase_models.plan`) |

#### Execute Phase

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | `2h` | Maximum duration for execute phase |

#### Prompt Refiner

The refiner optimizes the user prompt before analysis for better LLM effectiveness.

**How it works:**

1. User provides a prompt to `quorum run`
2. The refiner enhances the prompt for clarity and LLM effectiveness
3. The refined prompt is used for all subsequent phases
4. Original prompt is preserved in state for reference

**Disabling refinement:**

```bash
# Via CLI flag
quorum run --skip-refine "your prompt"

# Via configuration
phases:
  analyze:
    refiner:
      enabled: false
```

**Behavior in special modes:**

- **Dry-run mode**: Refinement is skipped, original prompt is used
- **Individual phase commands** (`quorum analyze`, etc.): Refinement is skipped
- **Refinement failure**: Falls back to original prompt with a warning

#### Semantic Moderator

The moderator evaluates semantic agreement across agent outputs using weighted divergence scoring.

**Consensus flow:**

```
V1 Analysis (all agents)
    ↓
V2 Refinement (all agents review V1, ultra-critical self-review)
    ↓
Moderator Evaluation → Score >= 90%? → Proceed to consolidation
    ↓ No
V(n+1) Refinement (integrate moderator feedback)
    ↓
Moderator Evaluation → Score >= 90%? → Proceed to consolidation
    ↓ No (max rounds or stagnation)
Abort or proceed with best result
```

**Weighted Divergence Scoring:**

Not all disagreements are equal:

| Impact Level | Weight | Examples |
|--------------|--------|----------|
| **High** | Major reduction | Architectural decisions, core logic, security, breaking changes |
| **Medium** | Moderate reduction | Implementation details, edge cases, performance |
| **Low** | Minimal reduction | Naming conventions, code style, documentation, cosmetic choices |

---

### agents

Configures LLM agent backends. Each agent can define a default model and per-phase model overrides.

> **Agent Names are Aliases:** The key under `agents:` (e.g., `claude`, `copilot`) is just an alias/identifier.
> You can use any name you want. The actual CLI type is determined by built-in mappings or explicit configuration.
> This is useful for CLIs like **copilot** that support multiple models - you can define multiple entries
> using the same CLI but with different models for multi-agent analysis.

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

#### Multiple Agents with Same CLI

You can define multiple agent entries using the same CLI to run multi-agent analysis with different models:

```yaml
agents:
  # Copilot with Claude model
  copilot-claude:
    enabled: true
    path: copilot
    model: claude-sonnet-4-5
    phases:
      analyze: true
      moderate: true

  # Copilot with GPT model
  copilot-gpt:
    enabled: true
    path: copilot
    model: gpt-5
    phases:
      analyze: true

  # Both will participate in analysis, providing different perspectives
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
| `phases` | map | `{}` | Phase participation control (see below) |
| `token_discrepancy_threshold` | float | `5.0` | Token reporting validation threshold (see below) |
| `max_tokens` | int | `4096` | Maximum output tokens |
| `temperature` | float | `0.7` | Sampling temperature (0.0-2.0) |

#### Phase Participation Control

The `phases` map controls which workflow phases an agent participates in. Keys are phase names, values are boolean.

```yaml
agents:
  claude:
    enabled: true
    phases:
      refine: true      # Can refine prompts
      analyze: true     # Can perform analysis
      moderate: true    # Can evaluate consensus (fallback moderator)
      synthesize: true  # Can synthesize results
      plan: true        # Can generate plans
      execute: true     # Can execute tasks
```

If `phases` is empty or not specified, the agent participates in **all phases** (backward compatible).

**Moderator fallback chain:** When the primary moderator fails, agents with `moderate: true` are tried as fallbacks in order.

#### Token Discrepancy Detection

The `token_discrepancy_threshold` validates token counts reported by CLI tools. If reported tokens differ from estimated by more than this factor, the estimated value is used instead.

```yaml
agents:
  copilot:
    enabled: true
    token_discrepancy_threshold: 5  # Default: 5x
    # Reported tokens must be within 1/5 to 5x of estimated
    # Set to 0 to disable validation
```

| Value | Behavior |
|-------|----------|
| `5` (default) | Reported must be between 1/5 and 5x of estimated |
| `3` | More strict - between 1/3 and 3x |
| `0` | Disable discrepancy detection |

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
| `path` | string | `.quorum/state/state.json` | Base state directory (workflows stored in `workflows/` subdirectory) |
| `backup_path` | string | `.quorum/state/state.json.bak` | Legacy backup path (per-workflow backups in `workflows/<id>.json.bak`) |
| `lock_ttl` | duration | `1h` | Lock file TTL before considered stale |

**Multi-workflow storage:**

Workflows are stored in separate files under `.quorum/state/workflows/`:
- Each workflow: `.quorum/state/workflows/<workflow-id>.json`
- Active workflow tracking: `.quorum/state/workflows/active.json`
- Per-workflow backups: `.quorum/state/workflows/<workflow-id>.json.bak`

**Workflow continuity:**

The `/plan` and `/execute` commands (both CLI and TUI) automatically continue from the active workflow when no prompt is provided:

```bash
# Start a new workflow
quorum run "Implement feature X"

# Continue planning (uses active workflow)
quorum plan

# Execute tasks (uses active workflow)
quorum execute

# Resume a specific workflow
quorum plan --workflow wf-abc123
quorum execute --workflow wf-abc123

# List available workflows
quorum workflows
```

**TUI mode commands:**

```
/workflows       List all available workflows
/load [id]       Load and switch to a specific workflow
/status          Show current workflow status
/plan            Continue to planning phase (from completed analyze)
/execute         Continue to execution phase (from completed plan)
```

Example TUI workflow:
```
/workflows               # List available workflows
/load wf-1234567890-1    # Switch to a specific workflow
/status                  # Check current state
/plan                    # Continue to planning (if analyze completed)
/execute                 # Continue to execution (if plan completed)
```

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
QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD=0.95
QUORUM_PHASES_ANALYZE_REFINER_ENABLED=true
```

Nested keys use underscores: `phases.analyze.moderator.threshold` → `QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD`

---

## Example Configurations

### Minimal Configuration

```yaml
agents:
  default: claude
  claude:
    enabled: true
    path: claude
    phase_models:
      refine: claude-opus-4-5-20251101
      analyze: claude-opus-4-5-20251101
      plan: claude-opus-4-5-20251101
  gemini:
    enabled: true
    path: gemini
    phase_models:
      analyze: gemini-3-pro-preview
      plan: gemini-3-pro-preview

phases:
  analyze:
    synthesizer:
      agent: claude
    moderator:
      enabled: true
      agent: claude
      threshold: 0.90
```

### High-Quality Analysis

```yaml
agents:
  default: claude
  claude:
    enabled: true
    phase_models:
      refine: claude-opus-4-5-20251101
      analyze: claude-opus-4-5-20251101
      plan: claude-sonnet-4-5-20250929
      execute: claude-haiku-4-5-20251001
  gemini:
    enabled: true
    phase_models:
      analyze: gemini-2.5-pro
      plan: gemini-2.5-flash
      execute: gemini-2.5-flash

phases:
  analyze:
    refiner:
      enabled: true
      agent: claude
    synthesizer:
      agent: claude
    moderator:
      enabled: true
      agent: claude
      threshold: 0.95
      min_rounds: 2
      max_rounds: 5

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
    phase_models:
      analyze: gemini-2.5-flash
      plan: gemini-2.5-flash

# Disable refinement to reduce costs
phases:
  analyze:
    refiner:
      enabled: false
    synthesizer:
      agent: gemini
    moderator:
      enabled: true
      agent: gemini
      threshold: 0.85
      max_rounds: 3

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
  default: claude
  claude:
    enabled: true
    model: claude-haiku-4-5-20251001
    phase_models:
      analyze: claude-haiku-4-5-20251001

phases:
  analyze:
    synthesizer:
      agent: claude
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
