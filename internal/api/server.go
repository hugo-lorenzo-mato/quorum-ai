// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// Server provides HTTP REST API endpoints for workflow management.
type Server struct {
	router          chi.Router
	stateManager    core.StateManager
	eventBus        *events.EventBus
	agentRegistry   core.AgentRegistry
	logger          *slog.Logger
	chatHandler     *webadapters.ChatHandler
	resourceMonitor *diagnostics.ResourceMonitor
	configLoader    *config.Loader // for workflow execution configuration
	root            string         // root directory for file operations

	// Control planes for running workflows (enables pause/resume/cancel)
	controlPlanesMu sync.RWMutex
	controlPlanes   map[string]*control.ControlPlane
}

// ServerOption configures the server.
type ServerOption func(*Server)

// WithLogger sets the server logger.
func WithLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithAgentRegistry sets the agent registry for chat and workflow execution.
func WithAgentRegistry(registry core.AgentRegistry) ServerOption {
	return func(s *Server) {
		s.agentRegistry = registry
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

// WithRoot sets the root directory for file operations.
func WithRoot(root string) ServerOption {
	return func(s *Server) {
		s.root = root
	}
}

// NewServer creates a new API server.
func NewServer(stateManager core.StateManager, eventBus *events.EventBus, opts ...ServerOption) *Server {
	wd, _ := os.Getwd() // Best effort default
	s := &Server{
		stateManager:  stateManager,
		eventBus:      eventBus,
		logger:        slog.Default(),
		root:          wd,
		controlPlanes: make(map[string]*control.ControlPlane),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Create chat handler with agent registry (may be nil)
	s.chatHandler = webadapters.NewChatHandler(s.agentRegistry, eventBus)

	s.router = s.setupRouter()
	return s
}

// registerControlPlane registers a ControlPlane for a running workflow.
func (s *Server) registerControlPlane(workflowID string, cp *control.ControlPlane) {
	s.controlPlanesMu.Lock()
	defer s.controlPlanesMu.Unlock()
	s.controlPlanes[workflowID] = cp
}

// unregisterControlPlane removes a ControlPlane when workflow finishes.
func (s *Server) unregisterControlPlane(workflowID string) {
	s.controlPlanesMu.Lock()
	defer s.controlPlanesMu.Unlock()
	delete(s.controlPlanes, workflowID)
}

// getControlPlane retrieves a ControlPlane for a workflow.
func (s *Server) getControlPlane(workflowID string) (*control.ControlPlane, bool) {
	s.controlPlanesMu.RLock()
	defer s.controlPlanesMu.RUnlock()
	cp, ok := s.controlPlanes[workflowID]
	return cp, ok
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.router
}

// setupRouter configures Chi router with all routes and middleware.
func (s *Server) setupRouter() chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(s.loggingMiddleware)

	// CORS for frontend access
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Requested-With"},
		AllowCredentials: false,
		MaxAge:           300,
	})
	r.Use(corsHandler.Handler)

	// Health check
	r.Get("/health", s.handleHealth)
	r.Get("/health/deep", s.handleDeepHealth)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Workflow endpoints
		r.Route("/workflows", func(r chi.Router) {
			r.Get("/", s.handleListWorkflows)
			r.Post("/", s.handleCreateWorkflow)
			r.Get("/active", s.handleGetActiveWorkflow)

			r.Route("/{workflowID}", func(r chi.Router) {
				r.Get("/", s.handleGetWorkflow)
				r.Put("/", s.handleUpdateWorkflow)
				r.Patch("/", s.handleUpdateWorkflow)
				r.Delete("/", s.handleDeleteWorkflow)
				r.Post("/activate", s.handleActivateWorkflow)
				r.Post("/run", s.HandleRunWorkflow)
				r.Post("/cancel", s.handleCancelWorkflow)
				r.Post("/pause", s.handlePauseWorkflow)
				r.Post("/resume", s.handleResumeWorkflow)

				// Task endpoints nested under workflow
				r.Route("/tasks", func(r chi.Router) {
					r.Get("/", s.handleListTasks)
					r.Get("/{taskID}", s.handleGetTask)
				})
			})
		})

		// SSE endpoint for real-time updates
		r.Get("/events", s.handleSSE)
		// Also expose at /sse/events for frontend compatibility
		r.Route("/sse", func(r chi.Router) {
			r.Get("/events", s.handleSSE)
		})

		// Chat endpoints
		r.Route("/chat", func(r chi.Router) {
			r.Post("/sessions", s.chatHandler.CreateSession)
			r.Get("/sessions", s.chatHandler.ListSessions)
			r.Get("/sessions/{sessionID}", s.chatHandler.GetSession)
			r.Delete("/sessions/{sessionID}", s.chatHandler.DeleteSession)
			r.Get("/sessions/{sessionID}/messages", s.chatHandler.GetMessages)
			r.Post("/sessions/{sessionID}/messages", s.chatHandler.SendMessage)
			r.Put("/sessions/{sessionID}/agent", s.chatHandler.SetAgent)
			r.Put("/sessions/{sessionID}/model", s.chatHandler.SetModel)
		})

		// File browser endpoints
		r.Route("/files", func(r chi.Router) {
			r.Get("/", s.handleListFiles)
			r.Get("/content", s.handleGetFileContent)
			r.Get("/tree", s.handleGetFileTree)
		})

		// Configuration endpoints
		r.Route("/config", func(r chi.Router) {
			r.Get("/", s.handleGetConfig)
			r.Patch("/", s.handleUpdateConfig)
			r.Post("/validate", s.handleValidateConfig)
			r.Post("/reset", s.handleResetConfig)
			r.Get("/agents", s.handleGetAgents)
			r.Get("/schema", s.handleGetConfigSchema)
			r.Get("/enums", s.handleGetEnums)
		})
	})

	return r
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			s.logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
				"bytes", ww.BytesWritten(),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// respondJSON sends a JSON response.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

// respondError sends a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// DeepHealthResponse contains detailed health information.
type DeepHealthResponse struct {
	Status    string                        `json:"status"`
	Time      string                        `json:"time"`
	Resources *diagnostics.ResourceSnapshot `json:"resources,omitempty"`
	Trend     *diagnostics.ResourceTrend    `json:"trend,omitempty"`
	Warnings  []diagnostics.HealthWarning   `json:"warnings,omitempty"`
}

// handleDeepHealth returns detailed system health including resource metrics.
func (s *Server) handleDeepHealth(w http.ResponseWriter, _ *http.Request) {
	response := DeepHealthResponse{
		Status: "healthy",
		Time:   time.Now().UTC().Format(time.RFC3339),
	}

	if s.resourceMonitor != nil {
		// Get current resource snapshot
		snapshot := s.resourceMonitor.TakeSnapshot()
		response.Resources = &snapshot

		// Get resource trends
		trend := s.resourceMonitor.GetTrend()
		response.Trend = &trend

		// Check for warnings
		warnings := s.resourceMonitor.CheckHealth()
		response.Warnings = warnings

		// Determine overall status
		if !trend.IsHealthy {
			response.Status = "degraded"
		}
		for _, warn := range warnings {
			if warn.Level == "critical" {
				response.Status = "critical"
				break
			} else if warn.Level == "warning" && response.Status == "healthy" {
				response.Status = "degraded"
			}
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.logger.Info("starting API server", "addr", addr)
	return srv.ListenAndServe()
}
