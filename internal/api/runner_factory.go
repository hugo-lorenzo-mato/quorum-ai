// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cli "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
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
// The StateManager is obtained from the context if a ProjectContext is available,
// otherwise falls back to the factory's default StateManager.
//
// Parameters:
//   - ctx: Context for the runner (should have appropriate timeout)
//   - workflowID: The ID of the workflow being executed
//   - cp: Optional ControlPlane for pause/resume/cancel (may be nil)
//   - wfConfig: Optional workflow-specific configuration overrides (may be nil)
//   - state: Optional loaded workflow state (used for config snapshot + checkpoints)
//
// Returns:
//   - *workflow.Runner: Fully configured runner
//   - *webadapters.WebOutputNotifier: The notifier (for lifecycle events)
//   - error: Any error during setup
func (f *RunnerFactory) CreateRunner(ctx context.Context, workflowID string, cp *control.ControlPlane, bp *core.Blueprint, state *core.WorkflowState) (*workflow.Runner, *webadapters.WebOutputNotifier, error) {
	// Get project-scoped StateManager if available
	stateManager := GetStateManagerFromContext(ctx, f.stateManager)
	logger := f.logger
	if logger == nil {
		logger = logging.NewNop()
	}

	// Validate prerequisites
	if stateManager == nil {
		return nil, nil, fmt.Errorf("state manager not configured")
	}
	if f.eventBus == nil {
		return nil, nil, fmt.Errorf("event bus not configured")
	}

	// Resolve the effective config deterministically from ProjectContext.
	effCfg, err := ResolveEffectiveExecutionConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving effective execution config: %w", err)
	}
	cfg := effCfg.Config

	// Build a fresh agent registry from the (project-scoped) config.
	// This makes config changes effective immediately without requiring server restart.
	registry := cli.NewRegistry()
	if err := cli.ConfigureRegistryFromConfig(registry, cfg); err != nil {
		return nil, nil, fmt.Errorf("configuring agent registry: %w", err)
	}

	// Get project-scoped EventBus if available
	eventBus := GetEventBusFromContext(ctx, f.eventBus)

	// Get project root for multi-project support
	projectRoot := GetProjectRootFromContext(ctx)

	// Create web output notifier (bridges to EventBus)
	outputNotifier := webadapters.NewWebOutputNotifier(eventBus, workflowID)

	// Connect agent streaming events to the output notifier for real-time progress
	registry.SetEventHandler(func(event core.AgentEvent) {
		outputNotifier.AgentEvent(string(event.Type), event.Agent, event.Message, event.Data)
	})

	// Snapshot the config used for this execution attempt (best-effort).
	// This must happen before the runner starts so that failures are still auditable.
	predictedExecID := 0
	reportRelPath := filepath.Join(".quorum", "runs", workflowID)
	snapshotRelPath := ""
	snapshotFullPath := ""
	st := state
	if st == nil {
		loaded, loadErr := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
		if loadErr != nil {
			logger.Warn("failed to load workflow state for config snapshot", "workflow_id", workflowID, "error", loadErr)
		} else {
			st = loaded
		}
	}

	if st != nil {
		predictedExecID = st.ExecutionID + 1
		if strings.TrimSpace(st.ReportPath) != "" {
			reportRelPath = st.ReportPath
		}

		snapshotName := fmt.Sprintf("config-used-exec-%d.yaml", predictedExecID)
		snapshotRelPath = filepath.Join(reportRelPath, snapshotName)
		snapshotFullPath = filepath.Join(projectRoot, snapshotRelPath)

		// Ensure report directory exists and write snapshot file (do not fail runner creation on errors).
		if err := os.MkdirAll(filepath.Dir(snapshotFullPath), 0o750); err != nil {
			logger.Warn("failed to create report directory for config snapshot", "path", filepath.Dir(snapshotFullPath), "error", err)
		} else {
			if _, err := os.Stat(snapshotFullPath); err == nil {
				logger.Warn("config snapshot already exists, not overwriting", "path", snapshotFullPath)
			} else if os.IsNotExist(err) {
				if err := os.WriteFile(snapshotFullPath, effCfg.RawYAML, 0o600); err != nil {
					logger.Warn("failed to write config snapshot", "path", snapshotFullPath, "error", err)
				}
			} else {
				logger.Warn("failed to stat config snapshot", "path", snapshotFullPath, "error", err)
			}
		}

		// Persist checkpoint metadata for debugging/retries.
		if st.Checkpoints == nil {
			st.Checkpoints = make([]core.Checkpoint, 0)
		}

		meta := map[string]interface{}{
			"execution_id":   predictedExecID,
			"config_path":    effCfg.ConfigPath,
			"config_scope":   effCfg.ConfigScope,
			"config_mode":    effCfg.ConfigMode,
			"file_etag":      effCfg.FileETag,
			"effective_etag": effCfg.EffectiveETag,
			"snapshot_path":  snapshotRelPath,
		}
		data, _ := json.Marshal(meta) // Best-effort metadata

		st.Checkpoints = append(st.Checkpoints, core.Checkpoint{
			ID:        fmt.Sprintf("config-snapshot-%d", time.Now().UnixNano()),
			Type:      "config_snapshot",
			Phase:     st.CurrentPhase,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Loaded execution config (%s/%s): %s", effCfg.ConfigScope, effCfg.ConfigMode, effCfg.ConfigPath),
			Data:      data,
		})
		if err := stateManager.Save(ctx, st); err != nil {
			logger.Warn("failed to persist config snapshot checkpoint", "workflow_id", workflowID, "error", err)
		}

		// Emit SSE event so the Web UI can display config provenance.
		eventBus.Publish(events.NewConfigLoadedEvent(
			workflowID,
			getProjectID(ctx),
			effCfg.ConfigPath,
			effCfg.ConfigScope,
			effCfg.ConfigMode,
			effCfg.FileETag,
			effCfg.EffectiveETag,
			predictedExecID,
			snapshotRelPath,
			"",
		))
	}

	// Build runner using RunnerBuilder (Task-6 unification)
	builder := workflow.NewRunnerBuilder().
		WithConfig(cfg).
		WithStateManager(stateManager).
		WithAgentRegistry(registry).
		WithLogger(logger).
		WithOutputNotifier(outputNotifier).
		WithControlPlane(cp).
		WithHeartbeat(f.heartbeat).
		WithProjectRoot(projectRoot)

	// Apply workflow-level overrides if provided
	if bp != nil {
		builder.WithWorkflowConfig(&workflow.WorkflowConfigOverride{
			ExecutionMode:              bp.ExecutionMode,
			SingleAgentName:            bp.SingleAgent.Agent,
			SingleAgentModel:           bp.SingleAgent.Model,
			SingleAgentReasoningEffort: bp.SingleAgent.ReasoningEffort,
			ConsensusThreshold:         bp.Consensus.Threshold,
			MaxRetries:                 bp.MaxRetries,
			Timeout:                    bp.Timeout,
			DryRun:                     bp.DryRun,
			Sandbox:                    bp.Sandbox,
			// Since these come from core.Blueprint which already has resolved values,
			// we treat them as explicit overrides.
			HasDryRun:  true,
			HasSandbox: true,
		})
	}

	runner, err := builder.Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("building runner: %w", err)
	}

	return runner, outputNotifier, nil
}

// RunnerFactory returns a factory for creating workflow runners.
// Returns nil if required dependencies are not configured.
// Deprecated: Use RunnerFactoryForContext for project-scoped resources.
func (s *Server) RunnerFactory() *RunnerFactory {
	return s.RunnerFactoryForContext(context.Background())
}

// RunnerFactoryForContext returns a factory for creating workflow runners
// using project-scoped resources from the request context.
// Returns nil if required dependencies are not configured.
func (s *Server) RunnerFactoryForContext(ctx context.Context) *RunnerFactory {
	// Get project-scoped resources
	stateManager := s.getProjectStateManager(ctx)
	eventBus := s.getProjectEventBus(ctx)
	configLoader := s.getProjectConfigLoader(ctx)

	// NOTE: configLoader may be nil in some legacy/server-startup scenarios; runner creation
	// resolves the effective execution config from ProjectContext.
	if stateManager == nil || eventBus == nil {
		return nil
	}

	// Create a logging.Logger from the slog.Logger
	var logger *logging.Logger
	if s.logger != nil {
		logger = logging.NewWithHandler(s.logger.Handler())
	}

	factory := NewRunnerFactory(
		stateManager,
		s.agentRegistry,
		eventBus,
		configLoader,
		logger,
	)

	// Add heartbeat manager if available
	if s.heartbeat != nil {
		factory.WithHeartbeat(s.heartbeat)
	}

	return factory
}
