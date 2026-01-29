package cli

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenCodeAdapter(t *testing.T) {
	t.Run("creates adapter with defaults", func(t *testing.T) {
		cfg := AgentConfig{}
		agent, err := NewOpenCodeAdapter(cfg)

		require.NoError(t, err)
		require.NotNil(t, agent)

		adapter, ok := agent.(*OpenCodeAdapter)
		require.True(t, ok, "should return *OpenCodeAdapter")

		assert.Equal(t, "opencode", adapter.Name())
		assert.Equal(t, "opencode", adapter.config.Path)
	})

	t.Run("respects custom path", func(t *testing.T) {
		cfg := AgentConfig{Path: "/custom/opencode"}
		agent, err := NewOpenCodeAdapter(cfg)

		require.NoError(t, err)
		adapter := agent.(*OpenCodeAdapter)
		assert.Equal(t, "/custom/opencode", adapter.config.Path)
	})

	t.Run("sets default ollama URL", func(t *testing.T) {
		cfg := AgentConfig{}
		agent, err := NewOpenCodeAdapter(cfg)

		require.NoError(t, err)
		adapter := agent.(*OpenCodeAdapter)
		assert.Equal(t, "http://localhost:11434/v1", adapter.ollamaURL)
	})
}

func TestOpenCodeAdapter_Name(t *testing.T) {
	adapter := createTestOpenCodeAdapter(t)
	assert.Equal(t, "opencode", adapter.Name())
}

func TestOpenCodeAdapter_Capabilities(t *testing.T) {
	adapter := createTestOpenCodeAdapter(t)
	caps := adapter.Capabilities()

	assert.True(t, caps.SupportsJSON)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.Contains(t, caps.SupportedModels, "qwen2.5-coder:32b")
	assert.Contains(t, caps.SupportedModels, "deepseek-r1:32b")
	assert.Equal(t, "qwen2.5-coder:32b", caps.DefaultModel)
}

func TestOpenCodeAdapter_DetectProfile(t *testing.T) {
	adapter := createTestOpenCodeAdapter(t)

	tests := []struct {
		name     string
		prompt   string
		expected Profile
	}{
		// Coder profile tests
		{
			name:     "coder_create_keyword",
			prompt:   "Create a new function to handle user authentication",
			expected: ProfileCoder,
		},
		{
			name:     "coder_implement_keyword",
			prompt:   "Implement the sorting algorithm",
			expected: ProfileCoder,
		},
		{
			name:     "coder_fix_keyword",
			prompt:   "Fix the bug in the login handler",
			expected: ProfileCoder,
		},
		{
			name:     "coder_debug_keyword",
			prompt:   "Debug the failing test",
			expected: ProfileCoder,
		},
		{
			name:     "coder_refactor_keyword",
			prompt:   "Refactor this function for better readability",
			expected: ProfileCoder,
		},
		{
			name:     "coder_code_keyword",
			prompt:   "Write code to parse JSON",
			expected: ProfileCoder,
		},
		{
			name:     "coder_function_keyword",
			prompt:   "Add a function for data validation",
			expected: ProfileCoder,
		},
		{
			name:     "coder_script_keyword",
			prompt:   "Create a script for backup",
			expected: ProfileCoder,
		},

		// Architect profile tests
		{
			name:     "architect_analyze_keyword",
			prompt:   "Analyze the codebase structure",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_plan_keyword",
			prompt:   "Plan the migration strategy",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_design_keyword",
			prompt:   "Design the API architecture",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_audit_keyword",
			prompt:   "Audit the security configuration",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_review_keyword",
			prompt:   "Review the pull request",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_strategy_keyword",
			prompt:   "Develop a strategy for scaling",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_compare_keyword",
			prompt:   "Compare the two approaches",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_evaluate_keyword",
			prompt:   "Evaluate the performance impact",
			expected: ProfileArchitect,
		},
		{
			name:     "architect_pros_cons",
			prompt:   "List the pros and cons of this approach",
			expected: ProfileArchitect,
		},

		// Edge cases
		{
			name:     "empty_prompt_defaults_to_coder",
			prompt:   "",
			expected: ProfileCoder,
		},
		{
			name:     "no_keywords_defaults_to_coder",
			prompt:   "Hello world",
			expected: ProfileCoder,
		},
		{
			name:     "mixed_keywords_higher_architect_score",
			prompt:   "Analyze and design the new feature architecture, then plan implementation",
			expected: ProfileArchitect,
		},
		{
			name:     "mixed_keywords_higher_coder_score",
			prompt:   "Create, implement, and write the code for the new feature",
			expected: ProfileCoder,
		},
		{
			name:     "case_insensitive",
			prompt:   "ANALYZE THE CODE AND DESIGN THE SYSTEM",
			expected: ProfileArchitect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.detectProfile(tt.prompt)
			assert.Equal(t, tt.expected, result, "prompt: %s", tt.prompt)
		})
	}
}

func TestOpenCodeAdapter_ResolveModel(t *testing.T) {
	t.Run("explicit_model_takes_priority", func(t *testing.T) {
		adapter := createTestOpenCodeAdapter(t)

		opts := core.ExecuteOptions{
			Prompt: "Analyze the code", // Would normally be architect
			Model:  "explicit-model",
		}

		model := adapter.resolveModel(opts)
		assert.Equal(t, "explicit-model", model)
	})

	t.Run("configured_model_takes_priority_over_profile", func(t *testing.T) {
		cfg := AgentConfig{
			Name:  "opencode",
			Model: "configured-model",
		}
		agent, _ := NewOpenCodeAdapter(cfg)
		adapter := agent.(*OpenCodeAdapter)

		opts := core.ExecuteOptions{
			Prompt: "Analyze the code",
		}

		model := adapter.resolveModel(opts)
		assert.Equal(t, "configured-model", model)
	})

	t.Run("profile_based_selection_coder", func(t *testing.T) {
		adapter := createTestOpenCodeAdapter(t)

		opts := core.ExecuteOptions{
			Prompt: "Create a new function",
		}

		model := adapter.resolveModel(opts)
		assert.Equal(t, "qwen2.5-coder:32b", model)
	})

	t.Run("profile_based_selection_architect", func(t *testing.T) {
		adapter := createTestOpenCodeAdapter(t)

		opts := core.ExecuteOptions{
			Prompt: "Analyze and design the architecture",
		}

		model := adapter.resolveModel(opts)
		assert.Equal(t, "deepseek-r1:32b", model)
	})
}

func TestOpenCodeAdapter_BuildArgs(t *testing.T) {
	adapter := createTestOpenCodeAdapter(t)

	t.Run("includes_run_command", func(t *testing.T) {
		opts := core.ExecuteOptions{}
		args := adapter.buildArgs(opts, "")

		require.NotEmpty(t, args)
		assert.Equal(t, "run", args[0])
	})

	t.Run("includes_model_flag", func(t *testing.T) {
		opts := core.ExecuteOptions{}
		args := adapter.buildArgs(opts, "test-model")

		assert.Contains(t, args, "--model")

		// Find model value after --model flag
		for i, arg := range args {
			if arg == "--model" && i+1 < len(args) {
				assert.Equal(t, "test-model", args[i+1])
				return
			}
		}
		t.Fatal("--model flag not followed by model name")
	})

	t.Run("no_model_flag_when_empty", func(t *testing.T) {
		opts := core.ExecuteOptions{}
		args := adapter.buildArgs(opts, "")

		assert.NotContains(t, args, "--model")
	})
}

func TestOpenCodeAdapter_ParseOutput(t *testing.T) {
	adapter := createTestOpenCodeAdapter(t)

	t.Run("successful_output_sets_model", func(t *testing.T) {
		result := &CommandResult{
			Stdout:   "Hello world",
			ExitCode: 0,
			Duration: time.Second,
		}

		execResult, err := adapter.parseOutput(result, "test-model")

		require.NoError(t, err)
		assert.Equal(t, "test-model", execResult.Model)
		assert.Equal(t, "Hello world", execResult.Output)
	})

	t.Run("parses_json_output", func(t *testing.T) {
		result := &CommandResult{
			Stdout:   `{"content": "Generated code", "tokens": 100}`,
			ExitCode: 0,
			Duration: time.Second,
		}

		execResult, err := adapter.parseOutput(result, "test-model")

		require.NoError(t, err)
		assert.NotNil(t, execResult.Parsed)
	})

	t.Run("non_zero_exit_returns_error", func(t *testing.T) {
		result := &CommandResult{
			Stderr:   "Command failed",
			ExitCode: 1,
			Duration: time.Second,
		}

		_, err := adapter.parseOutput(result, "test-model")
		assert.Error(t, err)
	})

	t.Run("estimates_tokens", func(t *testing.T) {
		result := &CommandResult{
			Stdout:   "This is a test output with multiple words",
			ExitCode: 0,
			Duration: time.Second,
		}

		execResult, err := adapter.parseOutput(result, "test-model")

		require.NoError(t, err)
		assert.Greater(t, execResult.TokensOut, 0)
	})
}

// Helper functions
func createTestOpenCodeAdapter(t *testing.T) *OpenCodeAdapter {
	cfg := AgentConfig{Name: "opencode"}
	agent, err := NewOpenCodeAdapter(cfg)
	require.NoError(t, err)
	adapter := agent.(*OpenCodeAdapter)
	// Use an unreachable URL to force isModelAvailable to return true (assume available)
	adapter.ollamaURL = "http://unreachable.test"
	return adapter
}
