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

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/web/sse"
)

// Server represents the HTTP server for the Quorum web interface.
type Server struct {
	router      chi.Router
	httpServer  *http.Server
	config      Config
	logger      *slog.Logger
	eventBus    *events.EventBus
	sseHandler  *sse.Handler
	chatHandler *webadapters.ChatHandler
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

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/", s.handleAPIRoot)

		// SSE routes for real-time events
		if s.eventBus != nil {
			r.Route("/sse", func(r chi.Router) {
				s.sseHandler = sse.RegisterRoutes(r, s.eventBus)
				s.logger.Info("SSE endpoint registered at /api/v1/sse/events")
			})
		}
	})

	// Chat routes (nil AgentRegistry means chat features are limited)
	s.chatHandler = webadapters.NewChatHandler(nil, s.eventBus)
	s.chatHandler.RegisterRoutes(r)
	s.logger.Info("Chat routes registered at /chat/*")

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
