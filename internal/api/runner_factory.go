// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"context"
	"fmt"

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// RunnerFactory creates workflow.Runner instances for web execution context.
// It handles all the dependency wiring that would otherwise be duplicated
// in each handler that needs to run workflows.
type RunnerFactory struct {
	stateManager  core.StateManager
	agentRegistry core.AgentRegistry
	eventBus      *events.EventBus
	configLoader  *config.Loader
	logger        *logging.Logger
	heartbeat     *workflow.HeartbeatManager
}

// NewRunnerFactory creates a new runner factory.
func NewRunnerFactory(
	stateManager core.StateManager,
	agentRegistry core.AgentRegistry,
	eventBus *events.EventBus,
	configLoader *config.Loader,
	logger *logging.Logger,
) *RunnerFactory {
	return &RunnerFactory{
		stateManager:  stateManager,
		agentRegistry: agentRegistry,
		eventBus:      eventBus,
		configLoader:  configLoader,
		logger:        logger,
	}
}

// WithHeartbeat sets the heartbeat manager for zombie detection support.
func (f *RunnerFactory) WithHeartbeat(hb *workflow.HeartbeatManager) *RunnerFactory {
	f.heartbeat = hb
	return f
}

// CreateRunner creates a new workflow.Runner for executing a workflow.
// It creates all necessary dependencies and adapters for the web context.
//
// Parameters:
//   - ctx: Context for the runner (should have appropriate timeout)
//   - workflowID: The ID of the workflow being executed
//   - cp: Optional ControlPlane for pause/resume/cancel (may be nil)
//
// Returns:
//   - *workflow.Runner: Fully configured runner
//   - *webadapters.WebOutputNotifier: The notifier (for lifecycle events)
//   - error: Any error during setup
func (f *RunnerFactory) CreateRunner(ctx context.Context, workflowID string, cp *control.ControlPlane) (*workflow.Runner, *webadapters.WebOutputNotifier, error) {
	// Validate prerequisites
	if f.stateManager == nil {
		return nil, nil, fmt.Errorf("state manager not configured")
	}
	if f.agentRegistry == nil {
		return nil, nil, fmt.Errorf("agent registry not configured")
	}
	if f.eventBus == nil {
		return nil, nil, fmt.Errorf("event bus not configured")
	}
	if f.configLoader == nil {
		return nil, nil, fmt.Errorf("config loader not configured")
	}

	// Load configuration
	cfg, err := f.configLoader.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	// Create web output notifier (bridges to EventBus)
	outputNotifier := webadapters.NewWebOutputNotifier(f.eventBus, workflowID)

	// Build runner using RunnerBuilder (Task-6 unification)
	runner, err := workflow.NewRunnerBuilder().
		WithConfig(cfg).
		WithStateManager(f.stateManager).
		WithAgentRegistry(f.agentRegistry).
		WithLogger(f.logger).
		WithOutputNotifier(outputNotifier).
		WithControlPlane(cp).
		WithHeartbeat(f.heartbeat).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("building runner: %w", err)
	}

	return runner, outputNotifier, nil
}

// RunnerFactory returns a factory for creating workflow runners.
// Returns nil if required dependencies are not configured.
func (s *Server) RunnerFactory() *RunnerFactory {
	if s.stateManager == nil || s.agentRegistry == nil || s.eventBus == nil || s.configLoader == nil {
		return nil
	}

	// Create a logging.Logger from the slog.Logger
	var logger *logging.Logger
	if s.logger != nil {
		logger = logging.NewWithHandler(s.logger.Handler())
	}

	factory := NewRunnerFactory(
		s.stateManager,
		s.agentRegistry,
		s.eventBus,
		s.configLoader,
		logger,
	)

	// Add heartbeat manager if available
	if s.heartbeat != nil {
		factory.WithHeartbeat(s.heartbeat)
	}

	return factory
}
