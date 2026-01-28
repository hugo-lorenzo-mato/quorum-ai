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
    # Analysis synthesizer - consolidates multi-agent analyses
    synthesizer:
      agent: claude
    # Semantic moderator for multi-agent consensus evaluation
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.90
  plan:
    # Plan synthesizer - when enabled, agents propose plans in parallel,
    # then synthesizer consolidates. When disabled, single-agent planning.
    synthesizer:
      enabled: true
      agent: claude

# Agent configuration
# Phases use opt-in model: only phases set to true are enabled.
# If phases is empty or omitted, agent is enabled for all phases.
# Available phases: refine, analyze, moderate, synthesize, plan, execute
agents:
  default: claude

  claude:
    enabled: true
    path: claude
    model: claude-opus-4-5-20251101
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
    # Use faster model for execution
    phase_models:
      execute: gemini-3-flash-preview
    phases:
      analyze: true
      execute: true

  codex:
    enabled: true
    path: codex
    model: gpt-5.2-codex
    reasoning_effort: high
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
    phases:
      moderate: true

  # OpenCode - Local LLM agent via Ollama (MCP-compatible)
  # Requires: Ollama running at localhost:11434 with compatible models
  # Profiles: coder (qwen2.5-coder, deepseek-coder-v2), architect (llama3.1, deepseek-r1)
  opencode:
    enabled: false
    path: opencode
    model: qwen2.5-coder
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
# Tasks run in isolated worktrees on branch quorum/<task-id>.
# After completion: commit -> push -> PR (configurable).
git:
  # When to create worktrees: always | parallel | disabled
  worktree_mode: parallel
  # Post-task finalization
  auto_commit: true
  auto_push: true
  auto_pr: true
  # PR target branch (empty = repository default)
  pr_base_branch: ""
  # Auto-merge disabled by default for safety
  auto_merge: false
  merge_strategy: squash
`
