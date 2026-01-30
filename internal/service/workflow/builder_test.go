package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func TestRunnerBuilder_buildSingleAgentConfig(t *testing.T) {
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
	appConfig := &config.Config{
		Workflow: config.WorkflowConfig{
			MaxRetries: 1,
			DryRun:     true,
			Sandbox:    false,
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
			name: "override dry run and sandbox",
			workflowConfig: &WorkflowConfigOverride{
				DryRun:     false,
				Sandbox:    true,
				HasDryRun:  true,
				HasSandbox: true,
			},
			expected: func(t *testing.T, rc *RunnerConfig) {
				if rc.DryRun != false {
					t.Errorf("DryRun = %v, want false", rc.DryRun)
				}
				if rc.Sandbox != true {
					t.Errorf("Sandbox = %v, want true", rc.Sandbox)
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
