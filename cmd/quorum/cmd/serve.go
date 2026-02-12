package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/chat"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api"
	apimiddleware "github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
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

// serveInfra holds all server infrastructure components initialized during setup.
type serveInfra struct {
	logger           *logging.Logger
	loader           *config.Loader
	quorumCfg        *config.Config
	registry         *cli.Registry
	stateManager     core.StateManager
	chatStore        core.ChatStore
	eventBus         *events.EventBus
	resourceMonitor  *diagnostics.ResourceMonitor
	heartbeatManager *workflow.HeartbeatManager
	unifiedTracker   *api.UnifiedTracker
	workflowExecutor *api.WorkflowExecutor
	projectRegistry  project.Registry
	projectReg       *project.FileRegistry
	statePool        *project.StatePool
	kanbanEngine     *kanban.Engine
}

func runServe(_ *cobra.Command, _ []string) error {
	logger := logging.New(logging.Config{
		Level: logLevel, Format: logFormat, Output: os.Stdout,
	})

	infra := &serveInfra{logger: logger}

	setupServeConfigAndAgents(infra)
	setupServeStateAndChat(infra)

	cfg := web.DefaultConfig()
	cfg.Host = serveHost
	cfg.Port = servePort
	cfg.EnableCORS = !serveNoCORS

	if infra.stateManager != nil {
		defer func() {
			if closeErr := state.CloseStateManager(infra.stateManager); closeErr != nil {
				logger.Warn("failed to close state manager", slog.String("error", closeErr.Error()))
			}
		}()
	}
	if infra.chatStore != nil {
		defer func() {
			if closeErr := chat.CloseChatStore(infra.chatStore); closeErr != nil {
				logger.Warn("failed to close chat store", slog.String("error", closeErr.Error()))
			}
		}()
	}

	infra.eventBus = events.New(100)
	defer infra.eventBus.Close()

	ctx := context.Background()
	setupServeDiagnostics(ctx, infra)
	setupServeWorkflowInfra(infra)
	setupServeProjectInfra(infra)
	setupServeKanbanEngine(infra)

	serverOpts := buildServeServerOptions(infra)
	server := web.New(cfg, logger.Logger, serverOpts...)

	if err := server.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	runServePostStart(ctx, infra)
	startServeBackgroundServices(ctx, infra)

	serverURL := fmt.Sprintf("http://%s", server.Addr())
	fmt.Printf("\n  Quorum server running at: \033[1;36m%s\033[0m\n\n", serverURL)
	logger.Info("server started",
		slog.String("addr", server.Addr()), slog.String("url", serverURL), slog.Bool("cors", cfg.EnableCORS))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down server...")
	shutdownCtx := context.Background()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

func setupServeConfigAndAgents(infra *serveInfra) {
	logger := infra.logger
	infra.loader = config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		infra.loader.WithConfigFile(cfgFile)
	}
	quorumCfg, err := infra.loader.Load()
	if err != nil {
		logger.Warn("failed to load quorum config, agents will not be available", slog.String("error", err.Error()))
	}
	infra.quorumCfg = quorumCfg

	if quorumCfg != nil {
		registry := cli.NewRegistry()
		if err := configureAgentsFromConfig(registry, quorumCfg, infra.loader); err != nil {
			logger.Warn("failed to configure agents", slog.String("error", err.Error()))
		} else {
			infra.registry = registry
			logger.Info("agents configured", slog.Any("available", registry.List()))
		}
	}
}

func setupServeStateAndChat(infra *serveInfra) {
	logger := infra.logger
	quorumCfg := infra.quorumCfg
	if quorumCfg == nil {
		return
	}

	statePath := quorumCfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.db"
	}

	stateOpts := state.StateManagerOptions{BackupPath: quorumCfg.State.BackupPath}
	if quorumCfg.State.LockTTL != "" {
		if lockTTL, err := time.ParseDuration(quorumCfg.State.LockTTL); err == nil {
			stateOpts.LockTTL = lockTTL
		} else {
			logger.Warn("invalid state.lock_ttl, using default", slog.String("value", quorumCfg.State.LockTTL), slog.String("error", err.Error()))
		}
	}

	sm, err := state.NewStateManagerWithOptions(statePath, stateOpts)
	if err != nil {
		logger.Warn("failed to create state manager", slog.String("error", err.Error()))
	} else {
		infra.stateManager = sm
		logger.Info("state manager initialized", slog.String("backend", "sqlite"), slog.String("path", statePath))
	}

	chatPath := filepath.Join(filepath.Dir(statePath), "chat.db")
	cs, err := chat.NewChatStore(chatPath)
	if err != nil {
		logger.Warn("failed to create chat store", slog.String("error", err.Error()))
	} else {
		infra.chatStore = cs
		logger.Info("chat store initialized", slog.String("backend", "sqlite"), slog.String("path", chatPath))
	}
}

func setupServeDiagnostics(ctx context.Context, infra *serveInfra) {
	logger := infra.logger
	quorumCfg := infra.quorumCfg
	if quorumCfg == nil || !quorumCfg.Diagnostics.Enabled {
		return
	}

	monitorInterval, err := time.ParseDuration(quorumCfg.Diagnostics.ResourceMonitoring.Interval)
	if err != nil {
		monitorInterval = 30 * time.Second
	}

	infra.resourceMonitor = diagnostics.NewResourceMonitor(
		monitorInterval,
		quorumCfg.Diagnostics.ResourceMonitoring.FDThresholdPercent,
		quorumCfg.Diagnostics.ResourceMonitoring.GoroutineThreshold,
		quorumCfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB,
		quorumCfg.Diagnostics.ResourceMonitoring.HistorySize,
		logger.Logger,
	)
	infra.resourceMonitor.Start(ctx)

	crashDumpWriter := diagnostics.NewCrashDumpWriter(
		quorumCfg.Diagnostics.CrashDump.Dir, quorumCfg.Diagnostics.CrashDump.MaxFiles,
		quorumCfg.Diagnostics.CrashDump.IncludeStack, quorumCfg.Diagnostics.CrashDump.IncludeEnv,
		logger.Logger, infra.resourceMonitor,
	)
	safeExecutor := diagnostics.NewSafeExecutor(
		infra.resourceMonitor, crashDumpWriter, logger.Logger,
		quorumCfg.Diagnostics.PreflightChecks.Enabled,
		quorumCfg.Diagnostics.PreflightChecks.MinFreeFDPercent,
		quorumCfg.Diagnostics.PreflightChecks.MinFreeMemoryMB,
	)

	if infra.registry != nil {
		infra.registry.SetDiagnostics(safeExecutor, crashDumpWriter)
	}
	logger.Info("diagnostics enabled",
		slog.String("interval", monitorInterval.String()),
		slog.Int("fd_threshold", quorumCfg.Diagnostics.ResourceMonitoring.FDThresholdPercent))
}

func setupServeWorkflowInfra(infra *serveInfra) {
	logger := infra.logger
	if infra.quorumCfg != nil && infra.stateManager != nil {
		heartbeatCfg := buildHeartbeatConfig(infra.quorumCfg.Workflow.Heartbeat)
		infra.heartbeatManager = workflow.NewHeartbeatManager(heartbeatCfg, infra.stateManager, logger.Logger)
		logger.Info("heartbeat manager initialized",
			slog.Duration("interval", heartbeatCfg.Interval),
			slog.Duration("stale_threshold", heartbeatCfg.StaleThreshold),
			slog.Bool("auto_resume", heartbeatCfg.AutoResume))
	}

	if infra.stateManager != nil {
		infra.unifiedTracker = api.NewUnifiedTracker(infra.stateManager, infra.heartbeatManager, logger.Logger, api.DefaultUnifiedTrackerConfig())
		logger.Info("unified tracker initialized")
	}

	if infra.registry != nil && infra.stateManager != nil && infra.quorumCfg != nil {
		runnerFactory := api.NewRunnerFactory(infra.stateManager, infra.registry, infra.eventBus, infra.loader, logger)
		infra.workflowExecutor = api.NewWorkflowExecutor(runnerFactory, infra.stateManager, infra.eventBus, logger.Logger, infra.unifiedTracker)
		logger.Info("workflow executor initialized for Kanban engine")
	}
}

func setupServeProjectInfra(infra *serveInfra) {
	logger := infra.logger
	projectReg, err := project.NewFileRegistry(project.WithLogger(logger.Logger))
	if err != nil {
		logger.Warn("failed to create project registry, multi-project support disabled", slog.String("error", err.Error()))
	} else {
		infra.projectReg = projectReg
		infra.projectRegistry = projectReg
		logger.Info("project registry initialized", slog.Int("project_count", projectReg.Count()))
	}

	if infra.projectRegistry != nil {
		infra.statePool = project.NewStatePool(
			infra.projectRegistry,
			project.WithPoolLogger(logger.Logger),
			project.WithMaxActiveContexts(10),
			project.WithEvictionGracePeriod(30*time.Minute),
		)
		logger.Info("state pool initialized for multi-project support")
	}
}

func setupServeKanbanEngine(infra *serveInfra) {
	logger := infra.logger
	if infra.workflowExecutor == nil {
		return
	}

	engineCfg := kanban.EngineConfig{
		Executor: infra.workflowExecutor, EventBus: infra.eventBus, Logger: logger.Logger,
	}

	if infra.statePool != nil && infra.projectRegistry != nil {
		engineCfg.ProjectProvider = api.NewKanbanStatePoolProvider(infra.statePool, infra.projectRegistry)
		logger.Info("Kanban engine using multi-project mode")
	} else if infra.stateManager != nil {
		if kanbanSM, ok := infra.stateManager.(kanban.KanbanStateManager); ok {
			engineCfg.StateManager = kanbanSM
			logger.Info("Kanban engine using single-project mode (legacy)")
		} else {
			logger.Warn("state manager does not implement KanbanStateManager, Kanban engine disabled")
		}
	}

	if engineCfg.ProjectProvider != nil || engineCfg.StateManager != nil {
		infra.kanbanEngine = kanban.NewEngine(engineCfg)
		logger.Info("Kanban engine initialized")
	}
}

func buildServeServerOptions(infra *serveInfra) []web.ServerOption {
	opts := []web.ServerOption{web.WithEventBus(infra.eventBus)}
	if infra.registry != nil {
		opts = append(opts, web.WithAgentRegistry(infra.registry))
	}
	if infra.resourceMonitor != nil {
		opts = append(opts, web.WithResourceMonitor(infra.resourceMonitor))
	}
	if infra.stateManager != nil {
		opts = append(opts, web.WithStateManager(infra.stateManager))
	}
	if infra.chatStore != nil {
		opts = append(opts, web.WithChatStore(infra.chatStore))
	}
	if infra.quorumCfg != nil {
		opts = append(opts, web.WithConfigLoader(infra.loader))
	}
	if infra.heartbeatManager != nil {
		opts = append(opts, web.WithHeartbeatManager(infra.heartbeatManager))
	}
	if infra.workflowExecutor != nil {
		opts = append(opts, web.WithWorkflowExecutor(infra.workflowExecutor))
	}
	if infra.kanbanEngine != nil {
		opts = append(opts, web.WithKanbanEngine(infra.kanbanEngine))
	}
	if infra.unifiedTracker != nil {
		opts = append(opts, web.WithUnifiedTracker(infra.unifiedTracker))
	}
	if infra.projectRegistry != nil {
		opts = append(opts, web.WithProjectRegistry(infra.projectRegistry))
	}
	if infra.statePool != nil {
		opts = append(opts, web.WithStatePool(infra.statePool))
	}
	return opts
}

func runServePostStart(ctx context.Context, infra *serveInfra) {
	logger := infra.logger

	if infra.stateManager != nil {
		if recovered, err := recoverZombieWorkflows(ctx, infra.stateManager, logger.Logger); err != nil {
			logger.Warn("failed to recover zombie workflows", slog.String("error", err.Error()))
		} else if recovered > 0 {
			logger.Info("recovered zombie workflows", slog.Int("count", recovered))
		}
	}

	if infra.unifiedTracker != nil {
		runOrphanCleanup(ctx, infra)
	}

	if infra.stateManager != nil {
		if migrated, err := migrateWorkflowsToKanban(ctx, infra.stateManager, logger.Logger); err != nil {
			logger.Warn("failed to migrate workflows to Kanban", slog.String("error", err.Error()))
		} else if migrated > 0 {
			logger.Info("migrated workflows to Kanban board", slog.Int("count", migrated))
		}
	}
}

func runOrphanCleanup(ctx context.Context, infra *serveInfra) {
	logger := infra.logger
	const reconcilerInterval = 30 * time.Second

	runCleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		totalCleaned := 0
		if infra.projectRegistry != nil && infra.statePool != nil {
			projects, err := infra.projectRegistry.ListProjects(cleanupCtx)
			if err != nil {
				logger.Warn("failed to list projects for orphan cleanup", slog.String("error", err.Error()))
			} else {
				for _, p := range projects {
					if p == nil || !p.IsAccessible() {
						continue
					}
					pc, err := infra.statePool.GetContext(cleanupCtx, p.ID)
					if err != nil {
						logger.Warn("failed to get project context for orphan cleanup",
							slog.String("project_id", p.ID), slog.String("error", err.Error()))
						continue
					}
					projCtx := apimiddleware.WithProjectContext(cleanupCtx, pc)
					cleaned, err := infra.unifiedTracker.CleanupOrphanedWorkflows(projCtx)
					if err != nil {
						logger.Warn("failed to clean orphaned workflows for project",
							slog.String("project_id", p.ID), slog.String("error", err.Error()))
						continue
					}
					totalCleaned += cleaned
				}
			}
		} else {
			cleaned, err := infra.unifiedTracker.CleanupOrphanedWorkflows(cleanupCtx)
			if err != nil {
				logger.Warn("failed to clean up orphaned workflows", slog.String("error", err.Error()))
				return
			}
			totalCleaned += cleaned
		}
		if totalCleaned > 0 {
			logger.Info("cleaned up orphaned running_workflows entries", slog.Int("count", totalCleaned))
		}
	}

	runCleanup()
	go func() {
		ticker := time.NewTicker(reconcilerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup()
			}
		}
	}()
}

func startServeBackgroundServices(ctx context.Context, infra *serveInfra) {
	logger := infra.logger

	if infra.kanbanEngine != nil {
		if err := infra.kanbanEngine.Start(ctx); err != nil {
			logger.Error("failed to start kanban engine", slog.String("error", err.Error()))
		} else {
			defer func() {
				stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := infra.kanbanEngine.Stop(stopCtx); err != nil {
					logger.Warn("failed to stop kanban engine", slog.String("error", err.Error()))
				}
			}()
		}
	}

	if infra.heartbeatManager != nil {
		infra.heartbeatManager.StartZombieDetector(func(state *core.WorkflowState) {
			logger.Warn("zombie workflow detected by heartbeat manager",
				slog.String("workflow_id", string(state.WorkflowID)),
				slog.String("phase", string(state.CurrentPhase)),
				slog.Int("resume_count", state.ResumeCount),
				slog.Int("max_resumes", state.MaxResumes))

			if infra.workflowExecutor != nil {
				infra.heartbeatManager.HandleZombie(state, infra.workflowExecutor)
			} else if infra.unifiedTracker != nil {
				if err := infra.unifiedTracker.ForceStop(ctx, state.WorkflowID); err != nil {
					logger.Error("failed to force-stop zombie workflow",
						slog.String("workflow_id", string(state.WorkflowID)), slog.String("error", err.Error()))
				}
			} else {
				state.Status = core.WorkflowStatusFailed
				state.Error = "Zombie workflow detected (stale heartbeat, no executor/tracker)"
				state.UpdatedAt = time.Now()
				if err := infra.stateManager.Save(ctx, state); err != nil {
					logger.Error("failed to save zombie state", slog.String("error", err.Error()))
				}
			}
		})
		defer infra.heartbeatManager.Shutdown()
	}
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

		// Clear running_workflows entry to prevent zombie re-detection
		if clearer, ok := stateManager.(interface {
			ClearWorkflowRunning(context.Context, core.WorkflowID) error
		}); ok {
			if err := clearer.ClearWorkflowRunning(ctx, summary.WorkflowID); err != nil {
				logger.Warn("failed to clear running_workflows entry",
					slog.String("workflow_id", string(summary.WorkflowID)),
					slog.String("error", err.Error()))
			}
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

// migrateWorkflowsToKanban assigns existing workflows without a Kanban column to the appropriate column.
// Completed workflows go to "done", failed to "refinement", running to "in_progress", others to "refinement".
func migrateWorkflowsToKanban(ctx context.Context, stateManager core.StateManager, logger *slog.Logger) (int, error) {
	// Check if state manager supports Kanban operations
	kanbanSM, ok := stateManager.(kanban.KanbanStateManager)
	if !ok {
		return 0, nil // State manager doesn't support Kanban, skip migration
	}

	workflows, err := stateManager.ListWorkflows(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing workflows: %w", err)
	}

	migrated := 0
	for _, summary := range workflows {
		// Load full workflow state to check Kanban column
		state, err := stateManager.LoadByID(ctx, summary.WorkflowID)
		if err != nil {
			logger.Warn("failed to load workflow for Kanban migration",
				slog.String("workflow_id", string(summary.WorkflowID)),
				slog.String("error", err.Error()))
			continue
		}
		if state == nil {
			continue
		}

		// Skip if already has a Kanban column assigned
		if state.KanbanColumn != "" {
			continue
		}

		// Determine appropriate column based on workflow status
		var targetColumn string
		switch state.Status {
		case core.WorkflowStatusCompleted:
			targetColumn = "done"
		case core.WorkflowStatusRunning:
			targetColumn = "in_progress"
		case core.WorkflowStatusFailed:
			targetColumn = "refinement"
		default:
			targetColumn = "refinement"
		}

		// Move workflow to the target column
		if err := kanbanSM.MoveWorkflow(ctx, string(state.WorkflowID), targetColumn, 0); err != nil {
			logger.Warn("failed to migrate workflow to Kanban",
				slog.String("workflow_id", string(summary.WorkflowID)),
				slog.String("target_column", targetColumn),
				slog.String("error", err.Error()))
			continue
		}

		logger.Debug("migrated workflow to Kanban",
			slog.String("workflow_id", string(summary.WorkflowID)),
			slog.String("column", targetColumn))
		migrated++
	}

	return migrated, nil
}
