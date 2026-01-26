package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
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

	// Create server options
	serverOpts := []web.ServerOption{web.WithEventBus(eventBus)}
	if registry != nil {
		serverOpts = append(serverOpts, web.WithAgentRegistry(registry))
	}
	if stateManager != nil {
		serverOpts = append(serverOpts, web.WithStateManager(stateManager))
	}

	// Create and start server with event bus and agent registry
	server := web.New(cfg, logger.Logger, serverOpts...)

	if err := server.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
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
	ctx := context.Background()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
