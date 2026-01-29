package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/web"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server",
	Long: `Start the Quorum web server with embedded frontend.

The server provides a REST API and serves the embedded React frontend
for interacting with Quorum workflows through a web interface.

Examples:
  # Start with defaults (localhost:8080)
  quorum serve

  # Start on custom host and port
  quorum serve --host 0.0.0.0 --port 3000

  # Disable CORS (for production behind a reverse proxy)
  quorum serve --no-cors`,
	RunE: runServe,
}

var (
	serveHost   string
	servePort   int
	serveNoCORS bool
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveHost, "host", "localhost",
		"Host address to bind to")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080,
		"Port to listen on")
	serveCmd.Flags().BoolVar(&serveNoCORS, "no-cors", false,
		"Disable CORS headers")
}

func runServe(_ *cobra.Command, _ []string) error {
	// Create logger
	logger := logging.New(logging.Config{
		Level:  logLevel,
		Format: logFormat,
		Output: os.Stdout,
	})

	// Load quorum configuration
	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	quorumCfg, err := loader.Load()
	if err != nil {
		logger.Warn("failed to load quorum config, agents will not be available", slog.String("error", err.Error()))
	}

	// Create agent registry
	var registry *cli.Registry
	if quorumCfg != nil {
		registry = cli.NewRegistry()
		if err := configureAgentsFromConfig(registry, quorumCfg, loader); err != nil {
			logger.Warn("failed to configure agents", slog.String("error", err.Error()))
			registry = nil
		} else {
			logger.Info("agents configured", slog.Any("available", registry.List()))
		}
	}

	// Create state manager for workflow persistence
	var stateManager core.StateManager
	if quorumCfg != nil {
		statePath := quorumCfg.State.Path
		if statePath == "" {
			statePath = ".quorum/state/state.json"
		}
		backend := quorumCfg.State.EffectiveBackend()
		sm, err := state.NewStateManager(backend, statePath)
		if err != nil {
			logger.Warn("failed to create state manager", slog.String("error", err.Error()))
		} else {
			stateManager = sm
			logger.Info("state manager initialized", slog.String("backend", backend), slog.String("path", statePath))
		}
	}

	// Create server configuration
	cfg := web.DefaultConfig()
	cfg.Host = serveHost
	cfg.Port = servePort
	cfg.EnableCORS = !serveNoCORS

	// Ensure state manager is closed on exit
	if stateManager != nil {
		defer func() {
			if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
				logger.Warn("failed to close state manager", slog.String("error", closeErr.Error()))
			}
		}()
	}

	// Create event bus for SSE streaming
	eventBus := events.New(100)
	defer eventBus.Close()

	// Initialize diagnostics if enabled
	var resourceMonitor *diagnostics.ResourceMonitor
	ctx := context.Background()

	if quorumCfg != nil && quorumCfg.Diagnostics.Enabled {
		// Parse monitoring interval
		monitorInterval, err := time.ParseDuration(quorumCfg.Diagnostics.ResourceMonitoring.Interval)
		if err != nil {
			monitorInterval = 30 * time.Second
		}

		// Create resource monitor
		resourceMonitor = diagnostics.NewResourceMonitor(
			monitorInterval,
			quorumCfg.Diagnostics.ResourceMonitoring.FDThresholdPercent,
			quorumCfg.Diagnostics.ResourceMonitoring.GoroutineThreshold,
			quorumCfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB,
			quorumCfg.Diagnostics.ResourceMonitoring.HistorySize,
			logger.Logger,
		)
		resourceMonitor.Start(ctx)
		defer resourceMonitor.Stop()

		// Create crash dump writer
		crashDumpWriter := diagnostics.NewCrashDumpWriter(
			quorumCfg.Diagnostics.CrashDump.Dir,
			quorumCfg.Diagnostics.CrashDump.MaxFiles,
			quorumCfg.Diagnostics.CrashDump.IncludeStack,
			quorumCfg.Diagnostics.CrashDump.IncludeEnv,
			logger.Logger,
			resourceMonitor,
		)

		// Create safe executor
		safeExecutor := diagnostics.NewSafeExecutor(
			resourceMonitor,
			crashDumpWriter,
			logger.Logger,
			quorumCfg.Diagnostics.PreflightChecks.Enabled,
			quorumCfg.Diagnostics.PreflightChecks.MinFreeFDPercent,
			quorumCfg.Diagnostics.PreflightChecks.MinFreeMemoryMB,
		)

		// Inject diagnostics into agent registry if available
		if registry != nil {
			registry.SetDiagnostics(safeExecutor, crashDumpWriter)
		}

		logger.Info("diagnostics enabled",
			slog.String("interval", monitorInterval.String()),
			slog.Int("fd_threshold", quorumCfg.Diagnostics.ResourceMonitoring.FDThresholdPercent),
		)
	}

	// Create heartbeat manager for zombie workflow detection (if enabled)
	var heartbeatManager *workflow.HeartbeatManager
	if quorumCfg != nil && quorumCfg.Workflow.Heartbeat.Enabled && stateManager != nil {
		heartbeatCfg := buildHeartbeatConfig(quorumCfg.Workflow.Heartbeat)
		heartbeatManager = workflow.NewHeartbeatManager(heartbeatCfg, stateManager, logger.Logger)
		logger.Info("heartbeat manager initialized",
			slog.Duration("interval", heartbeatCfg.Interval),
			slog.Duration("stale_threshold", heartbeatCfg.StaleThreshold),
			slog.Bool("auto_resume", heartbeatCfg.AutoResume))
	}

	// Create server options
	serverOpts := []web.ServerOption{web.WithEventBus(eventBus)}
	if registry != nil {
		serverOpts = append(serverOpts, web.WithAgentRegistry(registry))
	}
	if resourceMonitor != nil {
		serverOpts = append(serverOpts, web.WithResourceMonitor(resourceMonitor))
	}
	if stateManager != nil {
		serverOpts = append(serverOpts, web.WithStateManager(stateManager))
	}
	if quorumCfg != nil {
		serverOpts = append(serverOpts, web.WithConfigLoader(loader))
	}
	if heartbeatManager != nil {
		serverOpts = append(serverOpts, web.WithHeartbeatManager(heartbeatManager))
	}

	// Create and start server with event bus and agent registry
	server := web.New(cfg, logger.Logger, serverOpts...)

	if err := server.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	// Recover zombie workflows (workflows stuck in "running" state from previous crash/restart)
	if stateManager != nil {
		if recovered, err := recoverZombieWorkflows(ctx, stateManager, logger.Logger); err != nil {
			logger.Warn("failed to recover zombie workflows", slog.String("error", err.Error()))
		} else if recovered > 0 {
			logger.Info("recovered zombie workflows", slog.Int("count", recovered))
		}
	}

	// Start zombie detector if heartbeat is enabled
	if heartbeatManager != nil {
		heartbeatManager.StartZombieDetector(func(state *core.WorkflowState) {
			// Log zombie detection - auto-resume is handled by HandleZombie if enabled
			logger.Warn("zombie workflow detected by heartbeat manager",
				slog.String("workflow_id", string(state.WorkflowID)),
				slog.String("phase", string(state.CurrentPhase)))
			// Mark as failed since we don't have executor reference here
			// The HandleZombie method requires an executor interface, which we don't have in serve.go
			// For now, just mark failed - auto-resume would need tighter integration
			state.Status = core.WorkflowStatusFailed
			state.Error = "Zombie workflow detected (stale heartbeat)"
			state.UpdatedAt = time.Now()
			if err := stateManager.Save(ctx, state); err != nil {
				logger.Error("failed to save zombie state", slog.String("error", err.Error()))
			}
		})
		defer heartbeatManager.Shutdown()
	}

	logger.Info("server started",
		slog.String("addr", server.Addr()),
		slog.Bool("cors", cfg.EnableCORS),
	)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx := context.Background()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

// recoverZombieWorkflows marks workflows stuck in "running" state as failed.
// This handles cases where the server crashed or restarted while workflows were executing.
func recoverZombieWorkflows(ctx context.Context, stateManager core.StateManager, logger *slog.Logger) (int, error) {
	workflows, err := stateManager.ListWorkflows(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing workflows: %w", err)
	}

	recovered := 0
	for _, summary := range workflows {
		if summary.Status != core.WorkflowStatusRunning {
			continue
		}

		// Load full workflow state
		state, err := stateManager.LoadByID(ctx, summary.WorkflowID)
		if err != nil {
			logger.Warn("failed to load zombie workflow",
				slog.String("workflow_id", string(summary.WorkflowID)),
				slog.String("error", err.Error()))
			continue
		}
		if state == nil {
			continue
		}

		// Determine what phase was interrupted
		lastPhase := state.CurrentPhase
		checkpointCount := len(state.Checkpoints)
		taskCount := len(state.Tasks)

		// Mark as failed with informative error message
		state.Status = core.WorkflowStatusFailed
		state.Error = fmt.Sprintf("Workflow interrupted during %s phase (server restart)", lastPhase)
		state.UpdatedAt = time.Now()

		// Add checkpoint explaining the recovery with detailed context
		recoveryMessage := fmt.Sprintf(
			"Server restarted while workflow was in '%s' phase. "+
				"Had %d checkpoint(s) and %d task(s). "+
				"Click 'Start' to retry from the beginning, or check the report for partial results.",
			lastPhase, checkpointCount, taskCount)

		state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
			ID:        fmt.Sprintf("recovery-%d", time.Now().UnixNano()),
			Type:      "recovery",
			Phase:     lastPhase,
			Timestamp: time.Now(),
			Message:   recoveryMessage,
		})

		// Save updated state
		if err := stateManager.Save(ctx, state); err != nil {
			logger.Warn("failed to save recovered workflow",
				slog.String("workflow_id", string(summary.WorkflowID)),
				slog.String("error", err.Error()))
			continue
		}

		logger.Info("recovered zombie workflow",
			slog.String("workflow_id", string(summary.WorkflowID)),
			slog.String("phase", string(lastPhase)),
			slog.Int("checkpoints", checkpointCount),
			slog.Int("tasks", taskCount))
		recovered++
	}

	return recovered, nil
}

// buildHeartbeatConfig converts config.HeartbeatConfig to workflow.HeartbeatConfig.
func buildHeartbeatConfig(cfg config.HeartbeatConfig) workflow.HeartbeatConfig {
	result := workflow.DefaultHeartbeatConfig()

	// Parse interval
	if cfg.Interval != "" {
		if d, err := time.ParseDuration(cfg.Interval); err == nil {
			result.Interval = d
		}
	}

	// Parse stale threshold
	if cfg.StaleThreshold != "" {
		if d, err := time.ParseDuration(cfg.StaleThreshold); err == nil {
			result.StaleThreshold = d
		}
	}

	// Parse check interval
	if cfg.CheckInterval != "" {
		if d, err := time.ParseDuration(cfg.CheckInterval); err == nil {
			result.CheckInterval = d
		}
	}

	result.AutoResume = cfg.AutoResume

	if cfg.MaxResumes > 0 {
		result.MaxResumes = cfg.MaxResumes
	}

	return result
}
