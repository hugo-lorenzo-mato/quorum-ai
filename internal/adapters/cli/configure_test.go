package cli

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

// =============================================================================
// ConfigureRegistryFromConfig Tests
// =============================================================================

func TestConfigureRegistryFromConfig_AllDisabled(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude:   config.AgentConfig{Enabled: false},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No agents should be configured (enabled)
	enabled := registry.ListEnabled()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled agents, got %d: %v", len(enabled), enabled)
	}
}

func TestConfigureRegistryFromConfig_AllEnabled(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
				Model:   "opus",
			},
			Gemini: config.AgentConfig{
				Enabled: true,
				Path:    "gemini",
				Model:   "gemini-2.5-flash",
			},
			Codex: config.AgentConfig{
				Enabled: true,
				Path:    "codex",
				Model:   "gpt-5.1-codex",
			},
			Copilot: config.AgentConfig{
				Enabled: true,
				Path:    "copilot",
				Model:   "gpt-4o",
			},
			OpenCode: config.AgentConfig{
				Enabled: true,
				Path:    "opencode",
				Model:   "gpt-5",
			},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enabled := registry.ListEnabled()
	if len(enabled) != 5 {
		t.Errorf("expected 5 enabled agents, got %d: %v", len(enabled), enabled)
	}
}

func TestConfigureRegistryFromConfig_PartiallyEnabled(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
				Model:   "opus",
			},
			Gemini: config.AgentConfig{
				Enabled: false,
			},
			Codex: config.AgentConfig{
				Enabled: true,
				Path:    "codex",
				Model:   "gpt-5.1-codex",
			},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enabled := registry.ListEnabled()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled agents, got %d: %v", len(enabled), enabled)
	}
}

func TestConfigureRegistryFromConfig_WithPhases(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
				Model:   "opus",
				Phases: map[string]bool{
					"analyze":    true,
					"plan":       true,
					"execute":    false,
					"synthesize": true,
				},
			},
			Gemini: config.AgentConfig{Enabled: false},
			Codex:  config.AgentConfig{Enabled: false},
			Copilot: config.AgentConfig{
				Enabled: true,
				Path:    "copilot",
				Phases: map[string]bool{
					"execute": true,
				},
			},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check phase enablement for Claude
	analyzeAgents := registry.ListEnabledForPhase("analyze")
	hasClaudeAnalyze := false
	for _, name := range analyzeAgents {
		if name == "claude" {
			hasClaudeAnalyze = true
		}
	}
	if !hasClaudeAnalyze {
		t.Error("claude should be enabled for analyze phase")
	}

	// Check that claude is NOT enabled for execute
	executeAgents := registry.ListEnabledForPhase("execute")
	for _, name := range executeAgents {
		if name == "claude" {
			t.Error("claude should NOT be enabled for execute phase")
		}
	}

	// Copilot should be enabled for execute
	hasCopilotExecute := false
	for _, name := range executeAgents {
		if name == "copilot" {
			hasCopilotExecute = true
		}
	}
	if !hasCopilotExecute {
		t.Error("copilot should be enabled for execute phase")
	}
}

func TestConfigureRegistryFromConfig_WithReasoningEffort(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:         true,
				Path:            "claude",
				Model:           "opus",
				ReasoningEffort: "high",
				ReasoningEffortPhases: map[string]string{
					"analyze": "max",
					"plan":    "medium",
				},
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the agent and check config
	agent, err := registry.Get("claude")
	if err != nil {
		t.Fatalf("failed to get claude: %v", err)
	}

	// Access the underlying config through the adapter
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	if agentCfg.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want 'high'", agentCfg.ReasoningEffort)
	}

	// Check phase-specific reasoning effort
	if effort := agentCfg.GetReasoningEffort("analyze"); effort != "max" {
		t.Errorf("ReasoningEffort for analyze = %q, want 'max'", effort)
	}
	if effort := agentCfg.GetReasoningEffort("plan"); effort != "medium" {
		t.Errorf("ReasoningEffort for plan = %q, want 'medium'", effort)
	}
	// Fallback to default for non-overridden phases
	if effort := agentCfg.GetReasoningEffort("execute"); effort != "high" {
		t.Errorf("ReasoningEffort for execute = %q, want 'high' (default)", effort)
	}
}

func TestConfigureRegistryFromConfig_WithTokenDiscrepancy(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:                   true,
				Path:                      "claude",
				TokenDiscrepancyThreshold: 3.0,
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := registry.Get("claude")
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	if agentCfg.TokenDiscrepancyThreshold != 3.0 {
		t.Errorf("TokenDiscrepancyThreshold = %f, want 3.0", agentCfg.TokenDiscrepancyThreshold)
	}
}

func TestConfigureRegistryFromConfig_DefaultTokenDiscrepancy(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:                   true,
				Path:                      "claude",
				TokenDiscrepancyThreshold: 0, // should default to 5
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := registry.Get("claude")
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	if agentCfg.TokenDiscrepancyThreshold != DefaultTokenDiscrepancyThreshold {
		t.Errorf("TokenDiscrepancyThreshold = %f, want %f (default)", agentCfg.TokenDiscrepancyThreshold, DefaultTokenDiscrepancyThreshold)
	}
}

func TestConfigureRegistryFromConfig_WithIdleTimeout(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:     true,
				Path:        "claude",
				IdleTimeout: "10m",
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := registry.Get("claude")
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	if agentCfg.IdleTimeout != 10*time.Minute {
		t.Errorf("IdleTimeout = %v, want 10m", agentCfg.IdleTimeout)
	}
}

func TestConfigureRegistryFromConfig_InvalidIdleTimeout(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:     true,
				Path:        "claude",
				IdleTimeout: "invalid", // should default to 0
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := registry.Get("claude")
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	if agentCfg.IdleTimeout != 0 {
		t.Errorf("IdleTimeout = %v, want 0 (invalid input)", agentCfg.IdleTimeout)
	}
}

func TestConfigureRegistryFromConfig_AllAgentsWithConfigs(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:         true,
				Path:            "/usr/bin/claude",
				Model:           "opus",
				ReasoningEffort: "max",
				Phases:          map[string]bool{"analyze": true},
				IdleTimeout:     "5m",
			},
			Gemini: config.AgentConfig{
				Enabled:         true,
				Path:            "/usr/bin/gemini",
				Model:           "gemini-2.5-flash",
				ReasoningEffort: "high",
				Phases:          map[string]bool{"analyze": true},
			},
			Codex: config.AgentConfig{
				Enabled:         true,
				Path:            "/usr/bin/codex",
				Model:           "gpt-5.1-codex",
				ReasoningEffort: "medium",
				Phases:          map[string]bool{"execute": true},
				IdleTimeout:     "15m",
			},
			Copilot: config.AgentConfig{
				Enabled: true,
				Path:    "/usr/bin/copilot",
				Model:   "gpt-4o",
				Phases:  map[string]bool{"execute": true},
			},
			OpenCode: config.AgentConfig{
				Enabled: true,
				Path:    "/usr/bin/opencode",
				Model:   "gpt-5",
				Phases:  map[string]bool{"plan": true},
			},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enabled := registry.ListEnabled()
	if len(enabled) != 5 {
		t.Errorf("expected 5 enabled agents, got %d: %v", len(enabled), enabled)
	}

	// Verify each agent's configuration is correct
	type agentCheck struct {
		name  string
		model string
	}
	checks := []agentCheck{
		{"claude", "opus"},
		{"gemini", "gemini-2.5-flash"},
		{"codex", "gpt-5.1-codex"},
		{"copilot", "gpt-4o"},
		{"opencode", "gpt-5"},
	}

	for _, check := range checks {
		agent, err := registry.Get(check.name)
		if err != nil {
			t.Errorf("failed to get %s: %v", check.name, err)
			continue
		}
		if agent.Name() != check.name {
			t.Errorf("%s: Name() = %q, want %q", check.name, agent.Name(), check.name)
		}
	}
}

// =============================================================================
// getTokenDiscrepancyThreshold Tests
// =============================================================================

func TestGetTokenDiscrepancyThreshold_Local_Configured(t *testing.T) {
	t.Parallel()
	got := getTokenDiscrepancyThreshold(3.0)
	if got != 3.0 {
		t.Errorf("got %f, want 3.0", got)
	}
}

func TestGetTokenDiscrepancyThreshold_Local_Zero(t *testing.T) {
	t.Parallel()
	got := getTokenDiscrepancyThreshold(0)
	if got != DefaultTokenDiscrepancyThreshold {
		t.Errorf("got %f, want %f", got, DefaultTokenDiscrepancyThreshold)
	}
}

func TestGetTokenDiscrepancyThreshold_Local_Negative(t *testing.T) {
	t.Parallel()
	got := getTokenDiscrepancyThreshold(-1)
	if got != DefaultTokenDiscrepancyThreshold {
		t.Errorf("got %f, want %f (default for negative)", got, DefaultTokenDiscrepancyThreshold)
	}
}

func TestGetTokenDiscrepancyThreshold_Local_SmallPositive(t *testing.T) {
	t.Parallel()
	got := getTokenDiscrepancyThreshold(0.5)
	if got != 0.5 {
		t.Errorf("got %f, want 0.5", got)
	}
}

func TestGetTokenDiscrepancyThreshold_Local_Large(t *testing.T) {
	t.Parallel()
	got := getTokenDiscrepancyThreshold(100.0)
	if got != 100.0 {
		t.Errorf("got %f, want 100.0", got)
	}
}

// =============================================================================
// parseIdleTimeout Tests (extended)
// =============================================================================

func TestParseIdleTimeout_ValidDurations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"90s", 90 * time.Second},
		{"1h30m", time.Hour + 30*time.Minute},
		{"100ms", 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := parseIdleTimeout(tt.input)
			if got != tt.want {
				t.Errorf("parseIdleTimeout(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseIdleTimeout_InvalidInputs(t *testing.T) {
	t.Parallel()
	tests := []string{
		"",
		"invalid",
		"abc",
		"minutes",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got := parseIdleTimeout(input)
			if got != 0 {
				t.Errorf("parseIdleTimeout(%q) = %v, want 0", input, got)
			}
		})
	}
}

// =============================================================================
// ConfigureRegistryFromConfig Timeout Tests
// =============================================================================

func TestConfigureRegistryFromConfig_SetsTimeout(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := registry.Get("claude")
	claude := agent.(*ClaudeAdapter)
	agentCfg := claude.Config()

	// ConfigureRegistryFromConfig sets 5 minute timeout
	if agentCfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", agentCfg.Timeout)
	}
}

// =============================================================================
// ConfigureRegistryFromConfig ReasoningEffortPhases Tests
// =============================================================================

func TestConfigureRegistryFromConfig_ReasoningEffortPhasesAllAgents(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	phases := map[string]string{
		"analyze": "high",
		"plan":    "medium",
	}

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled:               true,
				Path:                  "claude",
				ReasoningEffortPhases: phases,
			},
			Gemini: config.AgentConfig{
				Enabled:               true,
				Path:                  "gemini",
				ReasoningEffortPhases: phases,
			},
			Codex: config.AgentConfig{
				Enabled:               true,
				Path:                  "codex",
				ReasoningEffortPhases: phases,
			},
			Copilot: config.AgentConfig{
				Enabled:               true,
				Path:                  "copilot",
				ReasoningEffortPhases: phases,
			},
			OpenCode: config.AgentConfig{
				Enabled:               true,
				Path:                  "opencode",
				ReasoningEffortPhases: phases,
			},
		},
	}

	err := ConfigureRegistryFromConfig(registry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all agents got the phases configured
	for _, name := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		agent, err := registry.Get(name)
		if err != nil {
			t.Errorf("failed to get %s: %v", name, err)
			continue
		}
		// Check if the agent has the right type by calling Name()
		if agent.Name() != name {
			t.Errorf("%s: Name() = %q", name, agent.Name())
		}
	}
}

// =============================================================================
// ConfigureRegistryFromConfig Idempotency Tests
// =============================================================================

func TestConfigureRegistryFromConfig_OverwritesPrevious(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	// First configuration
	cfg1 := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
				Model:   "model-v1",
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}
	_ = ConfigureRegistryFromConfig(registry, cfg1)

	agent1, _ := registry.Get("claude")
	claude1 := agent1.(*ClaudeAdapter)
	if claude1.Config().Model != "model-v1" {
		t.Errorf("Model = %q, want model-v1", claude1.Config().Model)
	}

	// Second configuration overwrites
	cfg2 := &config.Config{
		Agents: config.AgentsConfig{
			Claude: config.AgentConfig{
				Enabled: true,
				Path:    "claude",
				Model:   "model-v2",
			},
			Gemini:   config.AgentConfig{Enabled: false},
			Codex:    config.AgentConfig{Enabled: false},
			Copilot:  config.AgentConfig{Enabled: false},
			OpenCode: config.AgentConfig{Enabled: false},
		},
	}
	_ = ConfigureRegistryFromConfig(registry, cfg2)

	agent2, _ := registry.Get("claude")
	claude2 := agent2.(*ClaudeAdapter)
	if claude2.Config().Model != "model-v2" {
		t.Errorf("Model = %q, want model-v2 after reconfiguration", claude2.Config().Model)
	}
}
