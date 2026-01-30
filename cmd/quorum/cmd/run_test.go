package cmd

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

func TestParseTraceConfig_DefaultsToOff(t *testing.T) {
	cfg := &config.Config{}

	trace, err := parseTraceConfig(cfg, "")
	if err != nil {
		t.Fatalf("parseTraceConfig error: %v", err)
	}
	if trace.Mode != "off" {
		t.Fatalf("expected mode off, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_Override(t *testing.T) {
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}

	trace, err := parseTraceConfig(cfg, "full")
	if err != nil {
		t.Fatalf("parseTraceConfig error: %v", err)
	}
	if trace.Mode != "full" {
		t.Fatalf("expected mode full, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_InvalidMode(t *testing.T) {
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}

	if _, err := parseTraceConfig(cfg, "invalid"); err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}

func TestValidateSingleAgentFlags(t *testing.T) {
	tests := []struct {
		name          string
		singleAgent   bool
		agent         string
		model         string
		expectedError string
	}{
		{
			name:          "multi-agent mode (default)",
			singleAgent:   false,
			agent:         "",
			model:         "",
			expectedError: "",
		},
		{
			name:          "single-agent with agent",
			singleAgent:   true,
			agent:         "claude",
			model:         "",
			expectedError: "",
		},
		{
			name:          "single-agent with agent and model",
			singleAgent:   true,
			agent:         "claude",
			model:         "claude-3-haiku",
			expectedError: "",
		},
		{
			name:          "single-agent without agent",
			singleAgent:   true,
			agent:         "",
			model:         "",
			expectedError: "--agent is required",
		},
		{
			name:          "agent without single-agent flag",
			singleAgent:   false,
			agent:         "claude",
			model:         "",
			expectedError: "--agent requires --single-agent",
		},
		{
			name:          "model without single-agent flag",
			singleAgent:   false,
			agent:         "",
			model:         "claude-3-haiku",
			expectedError: "--model requires --single-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origSingleAgent := singleAgent
			origAgentName := agentName
			origAgentModel := agentModel
			defer func() {
				singleAgent = origSingleAgent
				agentName = origAgentName
				agentModel = origAgentModel
			}()

			// Set test values
			singleAgent = tt.singleAgent
			agentName = tt.agent
			agentModel = tt.model

			err := validateSingleAgentFlags()

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestBuildSingleAgentConfig(t *testing.T) {
	tests := []struct {
		name        string
		singleAgent bool
		agent       string
		model       string
		configAgent string
		configModel string
		expected    workflow.SingleAgentConfig
	}{
		{
			name:        "multi-agent mode returns config values",
			singleAgent: false,
			agent:       "",
			model:       "",
			configAgent: "gemini",
			configModel: "gemini-pro",
			expected: workflow.SingleAgentConfig{
				Enabled: false,
				Agent:   "gemini",
				Model:   "gemini-pro",
			},
		},
		{
			name:        "single-agent flag overrides config",
			singleAgent: true,
			agent:       "claude",
			model:       "",
			configAgent: "gemini",
			configModel: "gemini-pro",
			expected: workflow.SingleAgentConfig{
				Enabled: true,
				Agent:   "claude",
				Model:   "",
			},
		},
		{
			name:        "single-agent with model override",
			singleAgent: true,
			agent:       "claude",
			model:       "claude-3-haiku",
			configAgent: "gemini",
			configModel: "gemini-pro",
			expected: workflow.SingleAgentConfig{
				Enabled: true,
				Agent:   "claude",
				Model:   "claude-3-haiku",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origSingleAgent := singleAgent
			origAgentName := agentName
			origAgentModel := agentModel
			defer func() {
				singleAgent = origSingleAgent
				agentName = origAgentName
				agentModel = origAgentModel
			}()

			// Set test values
			singleAgent = tt.singleAgent
			agentName = tt.agent
			agentModel = tt.model

			cfg := &config.Config{
				Phases: config.PhasesConfig{
					Analyze: config.AnalyzePhaseConfig{
						SingleAgent: config.SingleAgentConfig{
							Enabled: false,
							Agent:   tt.configAgent,
							Model:   tt.configModel,
						},
					},
				},
			}

			result := buildSingleAgentConfig(cfg)

			if result.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled: expected %v, got %v", tt.expected.Enabled, result.Enabled)
			}
			if result.Agent != tt.expected.Agent {
				t.Errorf("Agent: expected %q, got %q", tt.expected.Agent, result.Agent)
			}
			if result.Model != tt.expected.Model {
				t.Errorf("Model: expected %q, got %q", tt.expected.Model, result.Model)
			}
		})
	}
}
