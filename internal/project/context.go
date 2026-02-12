package project

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/chat"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ProjectContext encapsulates all resources for a single project.
// It provides isolated state management, event bus, configuration, and attachments
// for multi-project support in a single Quorum server instance.
type ProjectContext struct {
	// Identity
	ID   string
	Root string

	// Core Services - exported for direct access
	StateManager core.StateManager
	EventBus     *events.EventBus
	ConfigLoader *config.Loader
	Attachments  *attachments.Store
	ChatStore    core.ChatStore
	ConfigMode   string // "inherit_global" | "custom"

	// Metadata
	CreatedAt    time.Time
	LastAccessed time.Time

	// Internal state
	mu     sync.RWMutex
	logger *slog.Logger
	closed bool
}

// contextOptions holds configuration for context creation
type contextOptions struct {
	logger          *slog.Logger
	eventBufferSize int
	configMode      string
}

// ContextOption configures a ProjectContext
type ContextOption func(*contextOptions)

// WithContextLogger sets the logger for the context
func WithContextLogger(logger *slog.Logger) ContextOption {
	return func(o *contextOptions) {
		o.logger = logger
	}
}

// WithEventBufferSize sets the event bus buffer size
func WithEventBufferSize(size int) ContextOption {
	return func(o *contextOptions) {
		if size > 0 {
			o.eventBufferSize = size
		}
	}
}

// WithConfigMode sets the configuration mode for this project context.
// Values: "inherit_global" | "custom". Unknown values fall back to "custom".
func WithConfigMode(mode string) ContextOption {
	return func(o *contextOptions) {
		o.configMode = mode
	}
}

// NewProjectContext creates a new context for the given project.
// The id parameter is the unique project identifier from the registry.
// The root parameter is the absolute path to the project directory.
func NewProjectContext(id, root string, opts ...ContextOption) (*ProjectContext, error) {
	// Apply default options
	options := &contextOptions{
		logger:          slog.Default(),
		eventBufferSize: 100,
		configMode:      ConfigModeCustom,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Ensure root is absolute
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	// Validate project directory
	quorumDir := filepath.Join(absRoot, ".quorum")
	if _, err := os.Stat(quorumDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a valid quorum project (no .quorum directory): %s", absRoot)
	}

	pc := &ProjectContext{
		ID:           id,
		Root:         absRoot,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		logger:       options.logger.With("project_id", id, "root", absRoot),
		ConfigMode:   options.configMode,
	}

	// Initialize all services in order
	var initErr error

	// 1. Config Loader (critical for operations)
	if initErr = pc.initConfigLoader(); initErr != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("initializing config loader: %w", initErr)
	}

	// 2. State Manager (critical - fail if unavailable)
	if initErr = pc.initStateManager(options); initErr != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("initializing state manager: %w", initErr)
	}

	// 3. Event Bus (critical)
	if initErr = pc.initEventBus(options); initErr != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("initializing event bus: %w", initErr)
	}

	// 4. Attachments (required for file handling)
	if initErr = pc.initAttachments(); initErr != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("initializing attachments: %w", initErr)
	}

	// 5. Chat Store (for chat session persistence)
	if initErr = pc.initChatStore(options); initErr != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("initializing chat store: %w", initErr)
	}

	pc.logger.Info("project context initialized",
		"state_backend", "sqlite",
		"event_buffer_size", options.eventBufferSize)

	return pc, nil
}

// initStateManager creates the state manager for this project
func (pc *ProjectContext) initStateManager(opts *contextOptions) error {
	_ = opts // reserved for future pool-level overrides

	// Default to project-local SQLite state DB.
	statePath := filepath.Join(pc.Root, ".quorum", "state", "state.db")
	backupPath := ""
	var lockTTL time.Duration

	// Best-effort: load config to honor state.path/backup_path/lock_ttl overrides.
	if pc.ConfigLoader != nil {
		if cfg, err := pc.ConfigLoader.Load(); err == nil && cfg != nil {
			if strings.TrimSpace(cfg.State.Path) != "" {
				statePath = cfg.State.Path
			}
			backupPath = cfg.State.BackupPath
			if strings.TrimSpace(cfg.State.LockTTL) != "" {
				if ttl, err := time.ParseDuration(cfg.State.LockTTL); err == nil {
					lockTTL = ttl
				} else {
					pc.logger.Warn("invalid state.lock_ttl in project config, using default",
						"value", cfg.State.LockTTL, "error", err)
				}
			}
		} else if err != nil {
			pc.logger.Warn("failed to load project config for state manager, using defaults", "error", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	smOpts := state.StateManagerOptions{
		BackupPath: backupPath,
	}
	if lockTTL > 0 {
		smOpts.LockTTL = lockTTL
	}

	sm, err := state.NewStateManagerWithOptions(statePath, smOpts)
	if err != nil {
		return fmt.Errorf("creating state manager at %s: %w", statePath, err)
	}

	pc.StateManager = sm
	pc.logger.Debug("state manager initialized", "path", statePath, "backend", "sqlite")
	return nil
}

// initEventBus creates the event bus for this project
func (pc *ProjectContext) initEventBus(opts *contextOptions) error {
	pc.EventBus = events.New(opts.eventBufferSize)
	pc.logger.Debug("event bus initialized", "buffer_size", opts.eventBufferSize)
	return nil
}

// initConfigLoader creates the config loader for this project
func (pc *ProjectContext) initConfigLoader() error {
	configPath := filepath.Join(pc.Root, ".quorum", "config.yaml")

	mode := pc.ConfigMode
	if mode != ConfigModeInheritGlobal && mode != ConfigModeCustom {
		mode = ConfigModeCustom
	}

	if mode == ConfigModeInheritGlobal {
		globalPath, err := config.EnsureGlobalConfigFile()
		if err != nil {
			return err
		}
		configPath = globalPath
	}

	// IMPORTANT: Resolve relative paths relative to the project root (not the config file location).
	// This is required for global config inheritance where the config file lives outside the project.
	pc.ConfigLoader = config.NewLoader().
		WithConfigFile(configPath).
		WithProjectDir(pc.Root)
	pc.logger.Debug("config loader initialized", "config_path", configPath, "config_mode", mode)
	return nil
}

// initAttachments creates the attachments store for this project
func (pc *ProjectContext) initAttachments() error {
	pc.Attachments = attachments.NewStore(pc.Root)
	pc.logger.Debug("attachments store initialized")
	return nil
}

// initChatStore creates the chat store for this project
func (pc *ProjectContext) initChatStore(opts *contextOptions) error {
	_ = opts // reserved for future pool-level overrides

	// Place chat DB next to the workflow state DB (default: .quorum/state/chat.db).
	statePath := filepath.Join(pc.Root, ".quorum", "state", "state.db")
	if pc.ConfigLoader != nil {
		if cfg, err := pc.ConfigLoader.Load(); err == nil && cfg != nil && strings.TrimSpace(cfg.State.Path) != "" {
			statePath = cfg.State.Path
		}
	}
	chatPath := filepath.Join(filepath.Dir(statePath), "chat.db")

	cs, err := chat.NewChatStore(chatPath)
	if err != nil {
		return fmt.Errorf("creating chat store at %s: %w", chatPath, err)
	}

	pc.ChatStore = cs
	pc.logger.Debug("chat store initialized", "path", chatPath, "backend", "sqlite")
	return nil
}

// Close releases all resources held by the context
func (pc *ProjectContext) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil // Already closed
	}

	pc.logger.Info("closing project context")

	var errs []error

	// Close state manager using the factory helper
	if pc.StateManager != nil {
		if err := state.CloseStateManager(pc.StateManager); err != nil {
			errs = append(errs, fmt.Errorf("state manager: %w", err))
			pc.logger.Error("error closing state manager", "error", err)
		}
		pc.StateManager = nil
	}

	// Close event bus
	if pc.EventBus != nil {
		pc.EventBus.Close()
		pc.EventBus = nil
	}

	// Close chat store
	if pc.ChatStore != nil {
		if err := chat.CloseChatStore(pc.ChatStore); err != nil {
			errs = append(errs, fmt.Errorf("chat store: %w", err))
			pc.logger.Error("error closing chat store", "error", err)
		}
		pc.ChatStore = nil
	}

	// Clear other resources
	pc.ConfigLoader = nil
	pc.Attachments = nil

	pc.closed = true
	pc.logger.Info("project context closed")

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}
	return nil
}

// IsClosed returns whether the context has been closed
func (pc *ProjectContext) IsClosed() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.closed
}

// Validate checks that all resources are accessible
func (pc *ProjectContext) Validate(_ context.Context) error {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return fmt.Errorf("context is closed")
	}

	// Check root directory
	if _, err := os.Stat(pc.Root); err != nil {
		return fmt.Errorf("project directory not accessible: %w", err)
	}

	// Check .quorum directory
	quorumDir := filepath.Join(pc.Root, ".quorum")
	if _, err := os.Stat(quorumDir); err != nil {
		return fmt.Errorf(".quorum directory not accessible: %w", err)
	}

	// Check state manager
	if pc.StateManager != nil && !pc.StateManager.Exists() {
		return fmt.Errorf("state database not accessible")
	}

	return nil
}

// Touch updates the last accessed timestamp
func (pc *ProjectContext) Touch() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.LastAccessed = time.Now()
}

// ProjectID returns the project identifier (implements middleware.ProjectContext interface)
func (pc *ProjectContext) ProjectID() string {
	return pc.ID
}

// ProjectRoot returns the project root directory path (implements middleware.ProjectContext interface)
func (pc *ProjectContext) ProjectRoot() string {
	return pc.Root
}

// GetLastAccessed returns the last access time (thread-safe)
func (pc *ProjectContext) GetLastAccessed() time.Time {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.LastAccessed
}

// GetConfig loads and returns the project configuration
func (pc *ProjectContext) GetConfig() (*config.Config, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return nil, fmt.Errorf("context is closed")
	}

	if pc.ConfigLoader == nil {
		return nil, fmt.Errorf("config loader not initialized")
	}

	return pc.ConfigLoader.Load()
}

// HasRunningWorkflows checks if this project has any running workflows
func (pc *ProjectContext) HasRunningWorkflows(ctx context.Context) (bool, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return false, fmt.Errorf("context is closed")
	}

	if pc.StateManager == nil {
		return false, fmt.Errorf("state manager not initialized")
	}

	running, err := pc.StateManager.ListRunningWorkflows(ctx)
	if err != nil {
		return false, err
	}

	return len(running) > 0, nil
}

// String returns a string representation for logging
func (pc *ProjectContext) String() string {
	return fmt.Sprintf("ProjectContext{id=%s, root=%s, closed=%v}", pc.ID, pc.Root, pc.closed)
}

// Ensure ProjectContext can be used where io.Closer is expected
var _ io.Closer = (*ProjectContext)(nil)
