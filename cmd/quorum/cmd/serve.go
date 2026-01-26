package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

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

	// Create server configuration
	cfg := web.DefaultConfig()
	cfg.Host = serveHost
	cfg.Port = servePort
	cfg.EnableCORS = !serveNoCORS

	// Create event bus for SSE streaming
	eventBus := events.New(100)
	defer eventBus.Close()

	// Create and start server with event bus
	server := web.New(cfg, logger.Logger, web.WithEventBus(eventBus))

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
