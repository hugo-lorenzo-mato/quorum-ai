//go:build go1.18

package config_test

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"gopkg.in/yaml.v3"
)

func FuzzConfigParse(f *testing.F) {
	// Valid config seeds
	f.Add(`log:
  level: info
  format: auto
agents:
  default: claude
  claude:
    enabled: true
workflow:
  timeout: 30m
  max_retries: 3
`)
	f.Add(`log:
  level: debug
  format: json
consensus:
  threshold: 0.75
`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`log:
  level: info
  format: text
agents:
  default: claude
  claude:
    enabled: true
    model: claude-3-opus
    path: /usr/bin/claude
  gemini:
    enabled: true
    model: gemini-pro
    path: /usr/bin/gemini
workflow:
  timeout: 1h
  max_retries: 5
state:
  path: .quorum/state
  lock_ttl: 5m
git:
  worktree_dir: .quorum/worktrees
github:
  remote: origin
consensus:
  threshold: 0.8
  weights:
    claims: 0.35
    risks: 0.35
    recommendations: 0.30
costs:
  max_per_workflow: 10.0
  max_per_task: 1.0
  alert_threshold: 0.8
`)

	f.Fuzz(func(t *testing.T, data string) {
		var cfg config.Config

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic parsing config: %v", r)
			}
		}()

		err := yaml.Unmarshal([]byte(data), &cfg)
		if err != nil {
			return // Invalid YAML is expected
		}

		// If parsed, validation should not panic
		_ = config.ValidateConfig(&cfg)
	})
}

func FuzzConsensusThreshold(f *testing.F) {
	f.Add(0.0)
	f.Add(0.5)
	f.Add(0.75)
	f.Add(1.0)
	f.Add(-0.1)
	f.Add(1.5)
	f.Add(0.999999)
	f.Add(-1000.0)
	f.Add(1000.0)

	f.Fuzz(func(t *testing.T, threshold float64) {
		cfg := config.Config{
			Log: config.LogConfig{
				Level:  "info",
				Format: "auto",
			},
			Agents: config.AgentsConfig{
				Default: "claude",
			},
			Workflow: config.WorkflowConfig{
				Timeout:    "30m",
				MaxRetries: 3,
			},
			State: config.StateConfig{
				Path:    ".quorum/state",
				LockTTL: "5m",
			},
			Git: config.GitConfig{
				WorktreeDir: ".quorum/worktrees",
			},
			GitHub: config.GitHubConfig{
				Remote: "origin",
			},
			Consensus: config.ConsensusConfig{
				Threshold: threshold,
				Weights: config.ConsensusWeight{
					Claims:          0.35,
					Risks:           0.35,
					Recommendations: 0.30,
				},
			},
		}

		err := config.ValidateConfig(&cfg)

		// Should return error for invalid thresholds
		if threshold < 0 || threshold > 1 {
			if err == nil {
				t.Errorf("expected error for threshold %f", threshold)
			}
		}
	})
}

func FuzzConfigMaxRetries(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(3)
	f.Add(5)
	f.Add(10)
	f.Add(-1)
	f.Add(-100)
	f.Add(1000)

	f.Fuzz(func(t *testing.T, maxRetries int) {
		cfg := config.Config{
			Log: config.LogConfig{
				Level:  "info",
				Format: "auto",
			},
			Agents: config.AgentsConfig{
				Default: "claude",
			},
			Workflow: config.WorkflowConfig{
				Timeout:    "30m",
				MaxRetries: maxRetries,
			},
			State: config.StateConfig{
				Path:    ".quorum/state",
				LockTTL: "5m",
			},
			Git: config.GitConfig{
				WorktreeDir: ".quorum/worktrees",
			},
			GitHub: config.GitHubConfig{
				Remote: "origin",
			},
			Consensus: config.ConsensusConfig{
				Threshold: 0.75,
				Weights: config.ConsensusWeight{
					Claims:          0.35,
					Risks:           0.35,
					Recommendations: 0.30,
				},
			},
		}

		err := config.ValidateConfig(&cfg)

		// Negative retries or >10 should be invalid
		if (maxRetries < 0 || maxRetries > 10) && err == nil {
			t.Errorf("expected error for max_retries %d", maxRetries)
		}
	})
}

func FuzzConfigAgentModel(f *testing.F) {
	f.Add("claude-3-opus")
	f.Add("gemini-pro")
	f.Add("")
	f.Add("unknown-model")
	f.Add("claude-3-sonnet-20240229")
	f.Add("gpt-4")
	f.Add("very-long-model-name-that-might-cause-issues")

	f.Fuzz(func(t *testing.T, model string) {
		cfg := config.Config{
			Log: config.LogConfig{
				Level:  "info",
				Format: "auto",
			},
			Agents: config.AgentsConfig{
				Default: "claude",
				Claude: config.AgentConfig{
					Enabled: true,
					Model:   model,
					Path:    "/usr/bin/claude",
				},
			},
			Workflow: config.WorkflowConfig{
				Timeout:    "30m",
				MaxRetries: 3,
			},
			State: config.StateConfig{
				Path:    ".quorum/state",
				LockTTL: "5m",
			},
			Git: config.GitConfig{
				WorktreeDir: ".quorum/worktrees",
			},
			GitHub: config.GitHubConfig{
				Remote: "origin",
			},
			Consensus: config.ConsensusConfig{
				Threshold: 0.75,
				Weights: config.ConsensusWeight{
					Claims:          0.35,
					Risks:           0.35,
					Recommendations: 0.30,
				},
			},
		}

		// Should not panic during validation
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic validating config with model %q: %v", model, r)
			}
		}()

		_ = config.ValidateConfig(&cfg)
	})
}
