package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func TestBuildRunnerConfigFromConfig_SingleAgentConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		cfg           *config.Config
		expectEnabled bool
		expectAgent   string
		expectModel   string
	}{
		{
			name: "single_agent enabled from app config",
			cfg: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: true,
							Agent:   "claude",
							Model:   "claude-3-haiku",
						},
					},
				},
			},
			expectEnabled: true,
			expectAgent:   "claude",
			expectModel:   "claude-3-haiku",
		},
		{
			name: "single_agent disabled from app config",
			cfg: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: false,
						},
					},
				},
			},
			expectEnabled: false,
			expectAgent:   "",
			expectModel:   "",
		},
		{
			name: "single_agent enabled without model override",
			cfg: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: true,
							Agent:   "gemini",
						},
					},
				},
			},
			expectEnabled: true,
			expectAgent:   "gemini",
			expectModel:   "",
		},
		{
			name:          "empty config defaults to disabled",
			cfg:           &config.Config{},
			expectEnabled: false,
			expectAgent:   "",
			expectModel:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runnerCfg := BuildRunnerConfigFromConfig(tt.cfg)

			if runnerCfg.SingleAgent.Enabled != tt.expectEnabled {
				t.Errorf("SingleAgent.Enabled = %v, want %v", runnerCfg.SingleAgent.Enabled, tt.expectEnabled)
			}
			if runnerCfg.SingleAgent.Agent != tt.expectAgent {
				t.Errorf("SingleAgent.Agent = %q, want %q", runnerCfg.SingleAgent.Agent, tt.expectAgent)
			}
			if runnerCfg.SingleAgent.Model != tt.expectModel {
				t.Errorf("SingleAgent.Model = %q, want %q", runnerCfg.SingleAgent.Model, tt.expectModel)
			}
		})
	}
}

func TestBuildRunnerConfigFromConfig_ModeratorConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				Moderator: config.ModeratorConfig{
					Enabled:             true,
					Agent:               "claude",
					Threshold:           0.75,
					MinRounds:           1,
					MaxRounds:           5,
					WarningThreshold:    0.3,
					StagnationThreshold: 0.1,
				},
			},
		},
	}

	runnerCfg := BuildRunnerConfigFromConfig(cfg)

	if !runnerCfg.Moderator.Enabled {
		t.Error("Moderator.Enabled = false, want true")
	}
	if runnerCfg.Moderator.Agent != "claude" {
		t.Errorf("Moderator.Agent = %q, want %q", runnerCfg.Moderator.Agent, "claude")
	}
	if runnerCfg.Moderator.Threshold != 0.75 {
		t.Errorf("Moderator.Threshold = %v, want %v", runnerCfg.Moderator.Threshold, 0.75)
	}
}

func TestBuildRunnerConfigFromConfig_PhaseTimeouts(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				Timeout: "30m",
			},
			Plan: config.PlanPhaseConfig{
				Timeout: "1h",
			},
			Execute: config.ExecutePhaseConfig{
				Timeout: "2h",
			},
		},
	}

	runnerCfg := BuildRunnerConfigFromConfig(cfg)

	// 30m = 30 * 60 * 1e9 nanoseconds
	if runnerCfg.PhaseTimeouts.Analyze.Minutes() != 30 {
		t.Errorf("PhaseTimeouts.Analyze = %v, want 30m", runnerCfg.PhaseTimeouts.Analyze)
	}
	if runnerCfg.PhaseTimeouts.Plan.Hours() != 1 {
		t.Errorf("PhaseTimeouts.Plan = %v, want 1h", runnerCfg.PhaseTimeouts.Plan)
	}
	if runnerCfg.PhaseTimeouts.Execute.Hours() != 2 {
		t.Errorf("PhaseTimeouts.Execute = %v, want 2h", runnerCfg.PhaseTimeouts.Execute)
	}
}

func TestBuildRunnerConfigFromConfig_InvalidTimeoutFallsBackToDefault(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			Timeout: "invalid",
		},
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				Timeout: "not-a-duration",
			},
		},
	}

	runnerCfg := BuildRunnerConfigFromConfig(cfg)
	defaultCfg := DefaultRunnerConfig()

	// Should fall back to defaults when parsing fails
	if runnerCfg.Timeout != defaultCfg.Timeout {
		t.Errorf("Timeout = %v, want default %v", runnerCfg.Timeout, defaultCfg.Timeout)
	}
	if runnerCfg.PhaseTimeouts.Analyze != defaultCfg.PhaseTimeouts.Analyze {
		t.Errorf("PhaseTimeouts.Analyze = %v, want default %v", runnerCfg.PhaseTimeouts.Analyze, defaultCfg.PhaseTimeouts.Analyze)
	}
}

func TestBuildRunnerConfigFromConfig_WorkflowSettings(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			Timeout:    "6h",
			MaxRetries: 5,
			DryRun:     true,
			DenyTools:  []string{"rm", "delete"},
		},
		Agents: config.AgentsConfig{
			Default: "claude",
		},
	}

	runnerCfg := BuildRunnerConfigFromConfig(cfg)

	if runnerCfg.Timeout.Hours() != 6 {
		t.Errorf("Timeout = %v, want 6h", runnerCfg.Timeout)
	}
	if runnerCfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", runnerCfg.MaxRetries)
	}
	if !runnerCfg.DryRun {
		t.Error("DryRun = false, want true")
	}
	if len(runnerCfg.DenyTools) != 2 {
		t.Errorf("DenyTools length = %d, want 2", len(runnerCfg.DenyTools))
	}
	if runnerCfg.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want %q", runnerCfg.DefaultAgent, "claude")
	}
}

func TestBuildAgentPhaseModels(t *testing.T) {
	t.Parallel()
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{
			Enabled: true,
			PhaseModels: map[string]string{
				"analyze": "claude-3-sonnet",
				"plan":    "claude-3-haiku",
			},
		},
		Gemini: config.AgentConfig{
			Enabled: false, // Disabled agent should not be included
			PhaseModels: map[string]string{
				"analyze": "gemini-pro",
			},
		},
		Codex: config.AgentConfig{
			Enabled: true,
			// No phase models - should not be included
		},
	}

	result := buildAgentPhaseModels(agents)

	// Claude is enabled with phase models
	if claudeModels, ok := result["claude"]; !ok {
		t.Error("expected claude in result")
	} else {
		if claudeModels["analyze"] != "claude-3-sonnet" {
			t.Errorf("claude analyze = %q, want %q", claudeModels["analyze"], "claude-3-sonnet")
		}
		if claudeModels["plan"] != "claude-3-haiku" {
			t.Errorf("claude plan = %q, want %q", claudeModels["plan"], "claude-3-haiku")
		}
	}

	// Gemini is disabled, should not be included
	if _, ok := result["gemini"]; ok {
		t.Error("gemini should not be in result (disabled)")
	}

	// Codex has no phase models, should not be included
	if _, ok := result["codex"]; ok {
		t.Error("codex should not be in result (no phase models)")
	}
}

func TestNewRunnerBuilder(t *testing.T) {
	t.Parallel()
	b := NewRunnerBuilder()

	if b == nil {
		t.Fatal("NewRunnerBuilder() returned nil")
	}
	if b.errors == nil {
		t.Error("errors slice should be initialized")
	}
	if len(b.errors) != 0 {
		t.Errorf("errors slice should be empty, got %d", len(b.errors))
	}
}

func TestRunnerBuilder_WithConfig(t *testing.T) {
	t.Parallel()
	t.Run("valid config", func(t *testing.T) {
		b := NewRunnerBuilder()
		cfg := &config.Config{}

		result := b.WithConfig(cfg)

		if result != b {
			t.Error("WithConfig should return same builder for chaining")
		}
		if b.config != cfg {
			t.Error("config should be stored")
		}
		if len(b.errors) != 0 {
			t.Error("no errors expected for valid config")
		}
	})

	t.Run("nil config adds error", func(t *testing.T) {
		b := NewRunnerBuilder()

		result := b.WithConfig(nil)

		if result != b {
			t.Error("WithConfig should return same builder for chaining")
		}
		if len(b.errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(b.errors))
		}
	})
}

func TestRunnerBuilder_WithDryRun(t *testing.T) {
	t.Parallel()
	b := NewRunnerBuilder()

	result := b.WithDryRun(true)

	if result != b {
		t.Error("WithDryRun should return same builder for chaining")
	}
	if b.dryRun == nil || !*b.dryRun {
		t.Error("dryRun should be set to true")
	}
}

func TestRunnerBuilder_WithMaxRetries(t *testing.T) {
	t.Parallel()
	b := NewRunnerBuilder()

	result := b.WithMaxRetries(10)

	if result != b {
		t.Error("WithMaxRetries should return same builder for chaining")
	}
	if b.maxRetries == nil || *b.maxRetries != 10 {
		t.Error("maxRetries should be set to 10")
	}
}

func TestRunnerBuilder_WithRunnerConfig(t *testing.T) {
	t.Parallel()
	t.Run("valid runner config", func(t *testing.T) {
		b := NewRunnerBuilder()
		rc := &RunnerConfig{MaxRetries: 5}

		result := b.WithRunnerConfig(rc)

		if result != b {
			t.Error("WithRunnerConfig should return same builder for chaining")
		}
		if b.runnerConfig != rc {
			t.Error("runnerConfig should be stored")
		}
		if !b.runnerConfigExplicit {
			t.Error("runnerConfigExplicit should be true")
		}
	})

	t.Run("nil runner config adds error", func(t *testing.T) {
		b := NewRunnerBuilder()

		result := b.WithRunnerConfig(nil)

		if result != b {
			t.Error("WithRunnerConfig should return same builder for chaining")
		}
		if len(b.errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(b.errors))
		}
	})
}

func TestRunnerBuilder_buildRunnerConfig_Precedence(t *testing.T) {
	t.Parallel()
	t.Run("explicit runner config takes precedence", func(t *testing.T) {
		explicitConfig := &RunnerConfig{
			MaxRetries: 99,
			DryRun:     true,
		}

		b := &RunnerBuilder{
			config: &config.Config{
				Workflow: config.WorkflowConfig{
					MaxRetries: 5,
					DryRun:     false,
				},
			},
			runnerConfig:         explicitConfig,
			runnerConfigExplicit: true,
		}

		result := b.buildRunnerConfig()

		if result != explicitConfig {
			t.Error("should return explicit runner config")
		}
		if result.MaxRetries != 99 {
			t.Errorf("MaxRetries = %d, want 99", result.MaxRetries)
		}
	})

	t.Run("builds from app config when no explicit config", func(t *testing.T) {
		b := &RunnerBuilder{
			config: &config.Config{
				Workflow: config.WorkflowConfig{
					MaxRetries: 7,
					DryRun:     true,
				},
			},
		}

		result := b.buildRunnerConfig()

		if result.MaxRetries != 7 {
			t.Errorf("MaxRetries = %d, want 7", result.MaxRetries)
		}
		if !result.DryRun {
			t.Error("DryRun = false, want true")
		}
	})
}

func TestRunnerBuilder_buildSingleAgentConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		appConfig      *config.Config
		workflowConfig *WorkflowConfigOverride
		expected       SingleAgentConfig
	}{
		{
			name: "workflow single-agent overrides app multi-agent",
			appConfig: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: false,
						},
					},
				},
			},
			workflowConfig: &WorkflowConfigOverride{
				ExecutionMode:   "single_agent",
				SingleAgentName: "claude",
			},
			expected: SingleAgentConfig{
				Enabled: true,
				Agent:   "claude",
				Model:   "",
			},
		},
		{
			name: "workflow multi-agent overrides app single-agent",
			appConfig: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: true,
							Agent:   "gemini",
						},
					},
				},
			},
			workflowConfig: &WorkflowConfigOverride{
				ExecutionMode: "multi_agent",
			},
			expected: SingleAgentConfig{
				Enabled: false,
			},
		},
		{
			name: "no workflow config falls back to app config",
			appConfig: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: true,
							Agent:   "codex",
							Model:   "gpt-4",
						},
					},
				},
			},
			workflowConfig: nil,
			expected: SingleAgentConfig{
				Enabled: true,
				Agent:   "codex",
				Model:   "gpt-4",
			},
		},
		{
			name: "empty workflow execution_mode falls back to app config",
			appConfig: &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: true,
							Agent:   "claude",
						},
					},
				},
			},
			workflowConfig: &WorkflowConfigOverride{
				ExecutionMode: "", // Empty means not specified
			},
			expected: SingleAgentConfig{
				Enabled: true,
				Agent:   "claude",
			},
		},
		{
			name:      "workflow single-agent with model override",
			appConfig: &config.Config{},
			workflowConfig: &WorkflowConfigOverride{
				ExecutionMode:    "single_agent",
				SingleAgentName:  "claude",
				SingleAgentModel: "claude-3-haiku",
			},
			expected: SingleAgentConfig{
				Enabled: true,
				Agent:   "claude",
				Model:   "claude-3-haiku",
			},
		},
		{
			name:           "both nil defaults to disabled",
			appConfig:      &config.Config{},
			workflowConfig: nil,
			expected: SingleAgentConfig{
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &RunnerBuilder{
				config:         tt.appConfig,
				workflowConfig: tt.workflowConfig,
			}

			got := b.buildSingleAgentConfig(tt.appConfig)

			if got.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.expected.Enabled)
			}
			if got.Agent != tt.expected.Agent {
				t.Errorf("Agent = %v, want %v", got.Agent, tt.expected.Agent)
			}
			if got.Model != tt.expected.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.expected.Model)
			}
		})
	}
}

func TestRunnerBuilder_WithWorkflowConfig(t *testing.T) {
	t.Parallel()
	b := &RunnerBuilder{}
	wfConfig := &WorkflowConfigOverride{
		ExecutionMode:   "single_agent",
		SingleAgentName: "claude",
	}

	result := b.WithWorkflowConfig(wfConfig)

	// Verify method returns builder for chaining
	if result != b {
		t.Error("WithWorkflowConfig should return the builder for chaining")
	}

	// Verify config is stored
	if b.workflowConfig != wfConfig {
		t.Error("WithWorkflowConfig should store the config")
	}
}

func TestRunnerBuilder_WithWorkflowConfig_Nil(t *testing.T) {
	t.Parallel()
	b := &RunnerBuilder{}

	result := b.WithWorkflowConfig(nil)

	// Verify method returns builder for chaining even with nil
	if result != b {
		t.Error("WithWorkflowConfig should return the builder for chaining")
	}

	// Verify nil is stored
	if b.workflowConfig != nil {
		t.Error("WithWorkflowConfig should store nil")
	}
}

func TestRunnerBuilder_buildRunnerConfig_Overrides(t *testing.T) {
	t.Parallel()
	appConfig := &config.Config{
		Workflow: config.WorkflowConfig{
			MaxRetries: 1,
			DryRun:     true,
		},
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				Moderator: config.ModeratorConfig{
					Threshold: 0.80,
				},
			},
		},
	}

	tests := []struct {
		name           string
		workflowConfig *WorkflowConfigOverride
		expected       func(*testing.T, *RunnerConfig)
	}{
		{
			name: "override consensus threshold",
			workflowConfig: &WorkflowConfigOverride{
				ConsensusThreshold: 0.95,
			},
			expected: func(t *testing.T, rc *RunnerConfig) {
				if rc.Moderator.Threshold != 0.95 {
					t.Errorf("Threshold = %v, want 0.95", rc.Moderator.Threshold)
				}
			},
		},
		{
			name: "override max retries",
			workflowConfig: &WorkflowConfigOverride{
				MaxRetries: 5,
			},
			expected: func(t *testing.T, rc *RunnerConfig) {
				if rc.MaxRetries != 5 {
					t.Errorf("MaxRetries = %v, want 5", rc.MaxRetries)
				}
			},
		},
		{
			name: "override dry run",
			workflowConfig: &WorkflowConfigOverride{
				DryRun:    false,
				HasDryRun: true,
			},
			expected: func(t *testing.T, rc *RunnerConfig) {
				if rc.DryRun != false {
					t.Errorf("DryRun = %v, want false", rc.DryRun)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewRunnerBuilder().
				WithConfig(appConfig).
				WithWorkflowConfig(tt.workflowConfig)

			rc := b.buildRunnerConfig()
			tt.expected(t, rc)
		})
	}
}

func TestWorkflowConfigOverride_IsSingleAgentMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		config   *WorkflowConfigOverride
		expected bool
	}{
		{
			name:     "nil config returns false",
			config:   nil,
			expected: false,
		},
		{
			name:     "empty execution mode returns false",
			config:   &WorkflowConfigOverride{ExecutionMode: ""},
			expected: false,
		},
		{
			name:     "multi_agent returns false",
			config:   &WorkflowConfigOverride{ExecutionMode: "multi_agent"},
			expected: false,
		},
		{
			name:     "single_agent returns true",
			config:   &WorkflowConfigOverride{ExecutionMode: "single_agent"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsSingleAgentMode(); got != tt.expected {
				t.Errorf("IsSingleAgentMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}
