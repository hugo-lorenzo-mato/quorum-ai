package config

// DefaultConfigYAML contains the default configuration YAML content.
// This is used by both `quorum init` and the API reset endpoint to ensure consistency.
const DefaultConfigYAML = `# Quorum AI Configuration
# Documentation: https://github.com/hugo-lorenzo-mato/quorum-ai/blob/main/docs/CONFIGURATION.md
#
# Values not specified here use sensible defaults. See docs for all options.

# Phase configuration
phases:
  analyze:
    # Prompt refiner - enhances user prompt before analysis
    refiner:
      enabled: true
      agent: claude
      template: refine-prompt-v2
    # Analysis synthesizer - consolidates multi-agent analyses
    synthesizer:
      agent: claude
    # Semantic moderator for multi-agent consensus evaluation
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.80
  plan:
    # Plan synthesizer - when enabled, agents propose plans in parallel,
    # then synthesizer consolidates. When disabled, single-agent planning.
    synthesizer:
      enabled: false
      agent: claude

# Agent configuration
# Phases use opt-in model: only phases set to true are enabled.
# If phases is empty or omitted, agent is enabled for NO phases (strict allowlist).
# Available phases: refine, analyze, moderate, synthesize, plan, execute
agents:
  default: claude

  claude:
    enabled: true
    path: claude
    model: claude-opus-4-6
    reasoning_effort: high
    idle_timeout: "15m"
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true

  gemini:
    enabled: true
    path: gemini
    model: gemini-3-pro-preview
    idle_timeout: "15m"
    # Use faster model for execution
    phase_models:
      execute: gemini-3-flash-preview
    phases:
      analyze: true
      execute: true

  codex:
    enabled: true
    path: codex
    model: gpt-5.3-codex
    reasoning_effort: high
    idle_timeout: "15m"
    reasoning_effort_phases:
      refine: xhigh
      analyze: xhigh
      plan: xhigh
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true

  copilot:
    enabled: true
    path: copilot
    model: claude-sonnet-4-5
    idle_timeout: "15m"
    phases:
      moderate: true

  # OpenCode - Local LLM agent via Ollama (MCP-compatible)
  # Requires: Ollama running at localhost:11434 with compatible models
  # Profiles: coder (qwen2.5-coder, deepseek-coder-v2), architect (llama3.1, deepseek-r1)
  opencode:
    enabled: false
    path: opencode
    model: qwen2.5-coder
    idle_timeout: "15m"
    # Phase-specific models: use coder models for execution, architect for analysis/planning
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

# Git configuration
# Organized by lifecycle: worktree (during execution), task (progress), finalization (delivery)
git:
  # Worktree management - temporary environment during execution
  worktree:
    dir: .worktrees
    mode: parallel    # always | parallel | disabled
    auto_clean: true  # Remove worktrees after completion

  # Task progress - incremental saving
  task:
    auto_commit: true  # Commit after each task completes

  # Finalization - workflow result delivery
  finalization:
    auto_push: true       # Push workflow branch to remote
    auto_pr: true         # Create single PR for workflow
    auto_merge: true      # Merge PR automatically (disable for manual review)
    pr_base_branch: ""    # Target branch (empty = repository default)
    merge_strategy: squash  # merge | squash | rebase

# Workflow execution settings
workflow:
  timeout: 16h
  max_retries: 3
  heartbeat:
    # Heartbeat monitoring is always active (cannot be disabled).
    # Intervals can be tuned below.
    interval: 30s
    stale_threshold: 2m
    check_interval: 60s
    auto_resume: true
    max_resumes: 1

# Issue generation
issues:
  enabled: true
  provider: github
  auto_generate: false
  timeout: 5m
  mode: direct
  draft_directory: ""
  repository: ""
  parent_prompt: ""
  prompt:
    language: english
    tone: professional
    include_diagrams: true
    title_format: "[quorum] {task_name}"
    body_prompt_file: ""
    convention: ""
    custom_instructions: ""
  labels:
    - quorum-generated
  assignees: []
  generator:
    enabled: false
    agent: claude
    model: haiku
    summarize: true
    max_body_length: 8000
    reasoning_effort: ""
    instructions: ""
    title_instructions: ""
`
