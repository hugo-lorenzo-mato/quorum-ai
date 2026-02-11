package workflow

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// Test builder method coverage

func TestRunnerBuilder_WithStateManager_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("nil state manager adds error", func(t *testing.T) {
		b := NewRunnerBuilder()
		result := b.WithStateManager(nil)

		if result != b {
			t.Error("WithStateManager should return same builder for chaining")
		}
		if len(b.errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(b.errors))
		}
	})
}

func TestRunnerBuilder_WithAgentRegistry_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("nil agent registry adds error", func(t *testing.T) {
		b := NewRunnerBuilder()
		result := b.WithAgentRegistry(nil)

		if result != b {
			t.Error("WithAgentRegistry should return same builder for chaining")
		}
		if len(b.errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(b.errors))
		}
	})
}

func TestRunnerBuilder_WithSharedRateLimiter_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	rl := service.NewRateLimiterRegistry()

	result := b.WithSharedRateLimiter(rl)

	if result != b {
		t.Error("WithSharedRateLimiter should return same builder for chaining")
	}
	if b.rateLimiter != rl {
		t.Error("rate limiter should be stored")
	}
}

func TestRunnerBuilder_WithOutputNotifier_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	on := NopOutputNotifier{}

	result := b.WithOutputNotifier(on)

	if result != b {
		t.Error("WithOutputNotifier should return same builder for chaining")
	}
	if b.outputNotifier == nil {
		t.Error("output notifier should be stored")
	}
}

func TestRunnerBuilder_WithControlPlane_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	// cp := control.NewControlPlane() - avoid using control package
	// Just test that the method stores whatever is passed

	result := b.WithControlPlane(nil) // Test with nil

	if result != b {
		t.Error("WithControlPlane should return same builder for chaining")
	}
}

func TestRunnerBuilder_WithHeartbeat_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	hb := &HeartbeatManager{}

	result := b.WithHeartbeat(hb)

	if result != b {
		t.Error("WithHeartbeat should return same builder for chaining")
	}
	if b.heartbeat != hb {
		t.Error("heartbeat should be stored")
	}
}

func TestRunnerBuilder_WithLogger_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	logger := logging.NewNop()

	result := b.WithLogger(logger)

	if result != b {
		t.Error("WithLogger should return same builder for chaining")
	}
	if b.logger != logger {
		t.Error("logger should be stored")
	}
}

func TestRunnerBuilder_WithSlogLogger_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	slogger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	result := b.WithSlogLogger(slogger)

	if result != b {
		t.Error("WithSlogLogger should return same builder for chaining")
	}
	if b.slogger != slogger {
		t.Error("slog logger should be stored")
	}
}

func TestRunnerBuilder_WithGitIsolation_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	gi := &GitIsolationConfig{
		Enabled:    true,
		BaseBranch: "main",
	}

	result := b.WithGitIsolation(gi)

	if result != b {
		t.Error("WithGitIsolation should return same builder for chaining")
	}
	if b.gitIsolation != gi {
		t.Error("git isolation should be stored")
	}
}

func TestRunnerBuilder_WithPhase_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	phase := core.PhasePlan

	result := b.WithPhase(phase)

	if result != b {
		t.Error("WithPhase should return same builder for chaining")
	}
	if b.phase == nil || *b.phase != phase {
		t.Error("phase should be stored")
	}
}

func TestRunnerBuilder_WithModeEnforcer_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	me := NewModeEnforcerAdapter(service.NewModeEnforcer(service.ExecutionMode{}))

	result := b.WithModeEnforcer(me)

	if result != b {
		t.Error("WithModeEnforcer should return same builder for chaining")
	}
	if b.modeEnforcer != me {
		t.Error("mode enforcer should be stored")
	}
}

func TestRunnerBuilder_WithGitClient_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	// Use nil to test method without full interface implementation
	result := b.WithGitClient(nil)

	if result != b {
		t.Error("WithGitClient should return same builder for chaining")
	}
	if !b.skipGitAutoCreate {
		t.Error("skipGitAutoCreate should be true")
	}
}

func TestRunnerBuilder_WithGitHubClient_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	result := b.WithGitHubClient(nil)

	if result != b {
		t.Error("WithGitHubClient should return same builder for chaining")
	}
}

func TestRunnerBuilder_WithGitClientFactory_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	result := b.WithGitClientFactory(nil)

	if result != b {
		t.Error("WithGitClientFactory should return same builder for chaining")
	}
}

func TestRunnerBuilder_WithWorktreeManager_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	result := b.WithWorktreeManager(nil)

	if result != b {
		t.Error("WithWorktreeManager should return same builder for chaining")
	}
	if !b.skipGitAutoCreate {
		t.Error("skipGitAutoCreate should be true")
	}
}

func TestRunnerBuilder_WithProjectRoot_Coverage(t *testing.T) {
	t.Parallel()

	b := NewRunnerBuilder()
	root := "/test/project"

	result := b.WithProjectRoot(root)

	if result != b {
		t.Error("WithProjectRoot should return same builder for chaining")
	}
	if b.projectRoot != root {
		t.Error("project root should be stored")
	}
}

func TestRunnerBuilder_Build_MissingRequiredDeps_Coverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupBuilder func() *RunnerBuilder
		expectError string
	}{
		{
			name: "missing config",
			setupBuilder: func() *RunnerBuilder {
				return NewRunnerBuilder()
			},
			expectError: "config is required",
		},
		{
			name: "accumulated errors",
			setupBuilder: func() *RunnerBuilder {
				return NewRunnerBuilder().
					WithConfig(nil).
					WithStateManager(nil)
			},
			expectError: "builder errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setupBuilder()
			_, err := b.Build(context.Background())

			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestDefaultGitIsolationConfig_Coverage(t *testing.T) {
	t.Parallel()

	cfg := DefaultGitIsolationConfig()

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.MergeStrategy != "sequential" {
		t.Errorf("MergeStrategy = %q, want %q", cfg.MergeStrategy, "sequential")
	}
	if cfg.AutoMerge {
		t.Error("AutoMerge should be false by default")
	}
}

func TestRunnerBuilder_createGitComponents_Coverage(t *testing.T) {
	// Cannot run in parallel due to global factory mutation
	// Save original factories
	origCreateGitClient := createGitClient
	origCreateWorktreeManager := createWorktreeManager
	origCreateGitHubClient := createGitHubClient
	origCreateGitClientFactory := createGitClientFactory
	origCreateWorkflowWorktreeManager := createWorkflowWorktreeManager
	defer func() {
		createGitClient = origCreateGitClient
		createWorktreeManager = origCreateWorktreeManager
		createGitHubClient = origCreateGitHubClient
		createGitClientFactory = origCreateGitClientFactory
		createWorkflowWorktreeManager = origCreateWorkflowWorktreeManager
	}()

	// Mock factories to return nil
	createGitClient = func(cwd string) (core.GitClient, error) {
		return nil, nil
	}
	createWorktreeManager = func(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager {
		return nil
	}
	createGitHubClient = func() (core.GitHubClient, error) {
		return nil, nil
	}
	createGitClientFactory = func() GitClientFactory {
		return nil
	}
	createWorkflowWorktreeManager = func(gc core.GitClient, repoRoot, worktreeDir string, logger *logging.Logger) (core.WorkflowWorktreeManager, error) {
		return nil, nil
	}

	cfg := &config.Config{
		Git: config.GitConfig{
			Finalization: config.GitFinalizationConfig{
				AutoPR: true,
			},
		},
	}

	b := NewRunnerBuilder().WithConfig(cfg)
	logger := logging.NewNop()

	wm, gc, ghc, gcf := b.createGitComponents(logger)

	// Just verify the method runs without crashing
	_ = wm
	_ = gc
	_ = ghc
	_ = gcf
}

func TestRunnerBuilder_createGitComponents_WithProjectRoot_Coverage(t *testing.T) {
	// Cannot run in parallel due to global factory mutation
	// Save original factories
	origCreateGitClient := createGitClient
	origCreateGitClientFactory := createGitClientFactory
	origCreateWorktreeManager := createWorktreeManager
	origCreateGitHubClient := createGitHubClient
	origCreateWorkflowWorktreeManager := createWorkflowWorktreeManager
	defer func() {
		createGitClient = origCreateGitClient
		createGitClientFactory = origCreateGitClientFactory
		createWorktreeManager = origCreateWorktreeManager
		createGitHubClient = origCreateGitHubClient
		createWorkflowWorktreeManager = origCreateWorkflowWorktreeManager
	}()

	var capturedPath string
	createGitClient = func(cwd string) (core.GitClient, error) {
		capturedPath = cwd
		return nil, nil
	}
	createGitClientFactory = func() GitClientFactory {
		return nil
	}
	createWorktreeManager = func(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager {
		return nil
	}
	createGitHubClient = func() (core.GitHubClient, error) {
		return nil, nil
	}
	createWorkflowWorktreeManager = func(gc core.GitClient, repoRoot, worktreeDir string, logger *logging.Logger) (core.WorkflowWorktreeManager, error) {
		return nil, nil
	}

	cfg := &config.Config{}
	projectRoot := "/custom/project/root"

	b := NewRunnerBuilder().
		WithConfig(cfg).
		WithProjectRoot(projectRoot)

	logger := logging.NewNop()

	b.createGitComponents(logger)

	if capturedPath != projectRoot {
		t.Errorf("createGitClient called with %q, want %q", capturedPath, projectRoot)
	}
}
