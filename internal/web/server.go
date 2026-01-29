package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// Server represents the HTTP server for the Quorum web interface.
type Server struct {
	router           chi.Router
	httpServer       *http.Server
	config           Config
	logger           *slog.Logger
	eventBus         *events.EventBus
	agentRegistry    core.AgentRegistry
	stateManager     core.StateManager
	chatStore        core.ChatStore
	resourceMonitor  *diagnostics.ResourceMonitor
	configLoader     *config.Loader             // for workflow execution configuration
	workflowExecutor *api.WorkflowExecutor      // for centralized workflow execution
	heartbeatManager *workflow.HeartbeatManager // for zombie workflow detection
	apiServer        *api.Server
}

// Config holds the server configuration.
type Config struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
	EnableCORS      bool
	ServeStatic     bool
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            8080,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CORSOrigins:     []string{"http://localhost:5173"},
		EnableCORS:      true,
		ServeStatic:     true,
	}
}

// ServerOption configures the server.
type ServerOption func(*Server)

// WithEventBus sets the event bus for SSE streaming.
func WithEventBus(bus *events.EventBus) ServerOption {
	return func(s *Server) {
		s.eventBus = bus
	}
}

// WithAgentRegistry sets the agent registry for chat and workflow execution.
func WithAgentRegistry(registry core.AgentRegistry) ServerOption {
	return func(s *Server) {
		s.agentRegistry = registry
	}
}

// WithStateManager sets the state manager for workflow persistence.
func WithStateManager(stateManager core.StateManager) ServerOption {
	return func(s *Server) {
		s.stateManager = stateManager
	}
}

// WithResourceMonitor sets the resource monitor for deep health checks.
func WithResourceMonitor(monitor *diagnostics.ResourceMonitor) ServerOption {
	return func(s *Server) {
		s.resourceMonitor = monitor
	}
}

// WithConfigLoader sets the configuration loader for workflow execution.
func WithConfigLoader(loader *config.Loader) ServerOption {
	return func(s *Server) {
		s.configLoader = loader
	}
}

// WithWorkflowExecutor sets the workflow executor for centralized execution management.
func WithWorkflowExecutor(executor *api.WorkflowExecutor) ServerOption {
	return func(s *Server) {
		s.workflowExecutor = executor
	}
}

// WithHeartbeatManager sets the heartbeat manager for zombie workflow detection.
func WithHeartbeatManager(hb *workflow.HeartbeatManager) ServerOption {
	return func(s *Server) {
		s.heartbeatManager = hb
	}
}

// WithChatStore sets the chat store for chat session persistence.
func WithChatStore(store core.ChatStore) ServerOption {
	return func(s *Server) {
		s.chatStore = store
	}
}

// New creates a new Server instance with the given configuration.
func New(cfg Config, logger *slog.Logger, opts ...ServerOption) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		config: cfg,
		logger: logger,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create API server if we have an event bus
	if s.eventBus != nil {
		apiOpts := []api.ServerOption{api.WithLogger(logger), api.WithAgentRegistry(s.agentRegistry)}
		if s.resourceMonitor != nil {
			apiOpts = append(apiOpts, api.WithResourceMonitor(s.resourceMonitor))
		}
		if s.configLoader != nil {
			apiOpts = append(apiOpts, api.WithConfigLoader(s.configLoader))
		}
		if s.workflowExecutor != nil {
			apiOpts = append(apiOpts, api.WithWorkflowExecutor(s.workflowExecutor))
		}
		if s.heartbeatManager != nil {
			apiOpts = append(apiOpts, api.WithHeartbeatManager(s.heartbeatManager))
		}
		if s.chatStore != nil {
			apiOpts = append(apiOpts, api.WithChatStore(s.chatStore))
		}
		s.apiServer = api.NewServer(s.stateManager, s.eventBus, apiOpts...)
		if s.agentRegistry != nil && s.stateManager != nil {
			s.logger.Info("API server initialized with event bus, agent registry, and state manager")
		} else if s.agentRegistry != nil {
			s.logger.Info("API server initialized with event bus and agent registry")
		} else if s.stateManager != nil {
			s.logger.Info("API server initialized with event bus and state manager")
		} else {
			s.logger.Info("API server initialized with event bus only")
		}
	}

	s.router = s.setupRouter()
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      s.router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// setupRouter configures the Chi router with middleware and routes.
func (s *Server) setupRouter() chi.Router {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.loggingMiddleware)
	r.Use(middleware.Recoverer)

	// CORS configuration
	if s.config.EnableCORS {
		corsMiddleware := cors.New(cors.Options{
			AllowedOrigins:   s.config.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           300,
		})
		r.Use(corsMiddleware.Handler)
	}

	// Health check endpoint
	r.Get("/health", s.handleHealth)

	// Mount API server if available (provides /api/v1/*)
	if s.apiServer != nil {
		// The API server handler includes all /api/v1/* routes
		r.Mount("/", s.apiServer.Handler())
		s.logger.Info("API routes mounted (workflows, files, config, events)")
	} else {
		// Fallback: basic API root endpoint only
		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/", s.handleAPIRoot)
		})
	}

	// Serve embedded frontend static files
	if s.config.ServeStatic {
		staticHandler, err := StaticHandler()
		if err != nil {
			s.logger.Warn("frontend not available, static file serving disabled",
				slog.String("error", err.Error()))
		} else {
			// Mount static file handler for all other routes
			r.NotFound(staticHandler.ServeHTTP)
			s.logger.Info("static file serving enabled for embedded frontend")
		}
	}

	return r
}

// loggingMiddleware logs HTTP requests using structured logging.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			s.logger.Info("http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("remote_addr", r.RemoteAddr),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

// handleAPIRoot returns API information.
func (s *Server) handleAPIRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"version":"v1","name":"quorum-api"}`))
}

// Start starts the HTTP server in a non-blocking manner.
func (s *Server) Start() error {
	s.logger.Info("starting http server",
		slog.String("addr", s.httpServer.Addr),
	)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("http server error", slog.String("error", err.Error()))
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("http server stopped")
	return nil
}

// Router returns the underlying chi router for route registration.
func (s *Server) Router() chi.Router {
	return s.router
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}
