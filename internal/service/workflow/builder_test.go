package workflow

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// testBuilderConfig returns a minimal valid config for testing.
func testBuilderConfig() *config.Config {
	return &config.Config{
		Workflow: config.WorkflowConfig{
			Timeout:    "1h",
			MaxRetries: 3,
		},
		Agents: config.AgentsConfig{
			Default: "claude",
			Claude: config.AgentConfig{
				Enabled: true,
			},
		},
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				Timeout: "30m",
				Moderator: config.ModeratorConfig{
					Enabled: false,
				},
			},
			Plan: config.PlanPhaseConfig{
				Timeout: "30m",
			},
			Execute: config.ExecutePhaseConfig{
				Timeout: "30m",
			},
		},
		Git: config.GitConfig{
			WorktreeDir: ".worktrees",
		},
	}
}

func TestNewRunnerBuilder(t *testing.T) {
	builder := NewRunnerBuilder()
	if builder == nil {
		t.Fatal("NewRunnerBuilder() returned nil")
	}
	if builder.runnerConfig == nil {
		t.Error("runnerConfig should not be nil")
	}
	if builder.errors == nil {
		t.Error("errors slice should be initialized")
	}
}

func TestRunnerBuilder_FluentChaining(t *testing.T) {
	// Test that all methods return *RunnerBuilder for proper chaining
	builder := NewRunnerBuilder()

	// Each method should return the same builder
	b1 := builder.WithConfig(testBuilderConfig())
	if b1 != builder {
		t.Error("WithConfig should return same builder")
	}

	b2 := builder.WithLogger(logging.NewNop())
	if b2 != builder {
		t.Error("WithLogger should return same builder")
	}

	b3 := builder.WithDryRun(true)
	if b3 != builder {
		t.Error("WithDryRun should return same builder")
	}

	b4 := builder.WithSandbox(false)
	if b4 != builder {
		t.Error("WithSandbox should return same builder")
	}

	b5 := builder.WithMaxRetries(5)
	if b5 != builder {
		t.Error("WithMaxRetries should return same builder")
	}

	b6 := builder.WithOutputNotifier(NopOutputNotifier{})
	if b6 != builder {
		t.Error("WithOutputNotifier should return same builder")
	}
}

func TestRunnerBuilder_ErrorAccumulation(t *testing.T) {
	builder := NewRunnerBuilder()

	// Setting nil config should accumulate an error
	builder.WithConfig(nil)

	if len(builder.errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(builder.errors))
	}
}

func TestRunnerBuilder_NilRunnerConfig(t *testing.T) {
	builder := NewRunnerBuilder()

	// Setting nil runner config should accumulate an error
	builder.WithRunnerConfig(nil)

	if len(builder.errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(builder.errors))
	}
}

func TestDefaultGitIsolationConfig(t *testing.T) {
	cfg := DefaultGitIsolationConfig()

	if cfg == nil {
		t.Fatal("DefaultGitIsolationConfig() returned nil")
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.BaseBranch != "" {
		t.Errorf("BaseBranch = %q, want empty (auto-detect)", cfg.BaseBranch)
	}
	if cfg.MergeStrategy != "sequential" {
		t.Errorf("MergeStrategy = %q, want 'sequential'", cfg.MergeStrategy)
	}
	if cfg.AutoMerge {
		t.Error("AutoMerge should be false by default")
	}
}

func TestBuildRunnerConfigFromConfig(t *testing.T) {
	cfg := testBuilderConfig()
	rc := BuildRunnerConfigFromConfig(cfg)

	if rc == nil {
		t.Fatal("BuildRunnerConfigFromConfig() returned nil")
	}
	if rc.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want 'claude'", rc.DefaultAgent)
	}
	if rc.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rc.MaxRetries)
	}
	if rc.PhaseTimeouts.Analyze != 30*time.Minute {
		t.Errorf("PhaseTimeouts.Analyze = %v, want 30m", rc.PhaseTimeouts.Analyze)
	}
	if rc.PhaseTimeouts.Plan != 30*time.Minute {
		t.Errorf("PhaseTimeouts.Plan = %v, want 30m", rc.PhaseTimeouts.Plan)
	}
	if rc.PhaseTimeouts.Execute != 30*time.Minute {
		t.Errorf("PhaseTimeouts.Execute = %v, want 30m", rc.PhaseTimeouts.Execute)
	}
}

func TestBuildRunnerConfigFromConfig_Defaults(t *testing.T) {
	// Empty config should use defaults
	cfg := &config.Config{}
	rc := BuildRunnerConfigFromConfig(cfg)

	if rc == nil {
		t.Fatal("BuildRunnerConfigFromConfig() returned nil")
	}
	// Should use default timeout
	if rc.Timeout != DefaultRunnerConfig().Timeout {
		t.Errorf("Timeout = %v, want default %v", rc.Timeout, DefaultRunnerConfig().Timeout)
	}
}

func TestBuildRunnerConfigFromConfig_PhaseModels(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Default: "claude",
			Claude: config.AgentConfig{
				Enabled: true,
				PhaseModels: map[string]string{
					"analyze": "claude-3-opus",
					"plan":    "claude-3-sonnet",
				},
			},
		},
	}
	rc := BuildRunnerConfigFromConfig(cfg)

	if rc == nil {
		t.Fatal("BuildRunnerConfigFromConfig() returned nil")
	}
	if rc.AgentPhaseModels == nil {
		t.Fatal("AgentPhaseModels should not be nil")
	}
	if pm, ok := rc.AgentPhaseModels["claude"]; !ok {
		t.Error("AgentPhaseModels should include 'claude'")
	} else {
		if pm["analyze"] != "claude-3-opus" {
			t.Errorf("claude.analyze = %q, want 'claude-3-opus'", pm["analyze"])
		}
	}
}

func TestGitIsolationConfig_Struct(t *testing.T) {
	gi := &GitIsolationConfig{
		Enabled:       true,
		BaseBranch:    "develop",
		MergeStrategy: "rebase",
		AutoMerge:     true,
	}

	if !gi.Enabled {
		t.Error("Enabled should be true")
	}
	if gi.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want 'develop'", gi.BaseBranch)
	}
	if gi.MergeStrategy != "rebase" {
		t.Errorf("MergeStrategy = %q, want 'rebase'", gi.MergeStrategy)
	}
	if !gi.AutoMerge {
		t.Error("AutoMerge should be true")
	}
}
