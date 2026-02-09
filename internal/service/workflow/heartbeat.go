package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// HeartbeatConfig configures the heartbeat system.
type HeartbeatConfig struct {
	// Interval is how often to write heartbeats (default: 30s)
	Interval time.Duration

	// StaleThreshold is when to consider a workflow zombie (default: 2m)
	StaleThreshold time.Duration

	// CheckInterval is how often to check for zombies (default: 60s)
	CheckInterval time.Duration

	// AutoResume enables automatic resume of zombie workflows
	AutoResume bool

	// MaxResumes is the maximum auto-resume attempts per workflow (default: 3)
	MaxResumes int
}

// DefaultHeartbeatConfig returns the default heartbeat configuration.
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Interval:       30 * time.Second,
		StaleThreshold: 2 * time.Minute,
		CheckInterval:  60 * time.Second,
		AutoResume:     false,
		MaxResumes:     3,
	}
}

// ZombieHandler is called when a zombie workflow is detected.
type ZombieHandler func(state *core.WorkflowState)

// HeartbeatManager manages heartbeats for running workflows and detects zombies.
type HeartbeatManager struct {
	config       HeartbeatConfig
	stateManager core.StateManager
	logger       *slog.Logger

	// Track active heartbeat goroutines
	mu     sync.Mutex
	active map[core.WorkflowID]context.CancelFunc

	// Track last successful heartbeat write per workflow (in-memory, cheap to check)
	lastWriteSuccess map[core.WorkflowID]time.Time

	// Per-workflow StateManagers for multi-project support.
	// When a workflow belongs to a project with its own DB, the project-scoped
	// StateManager is stored here so heartbeats and zombie detection target the
	// correct database.
	workflowSMs map[core.WorkflowID]core.StateManager

	// Zombie detector
	detectorCancel context.CancelFunc
	zombieHandler  ZombieHandler
}

// NewHeartbeatManager creates a new heartbeat manager.
func NewHeartbeatManager(
	config HeartbeatConfig,
	stateManager core.StateManager,
	logger *slog.Logger,
) *HeartbeatManager {
	return &HeartbeatManager{
		config:           config,
		stateManager:     stateManager,
		logger:           logger,
		active:           make(map[core.WorkflowID]context.CancelFunc),
		lastWriteSuccess: make(map[core.WorkflowID]time.Time),
		workflowSMs:      make(map[core.WorkflowID]core.StateManager),
	}
}

// Config returns the current configuration.
func (h *HeartbeatManager) Config() HeartbeatConfig {
	return h.config
}

// Start begins heartbeat tracking for a workflow.
// If sm is non-nil, heartbeats for this workflow will be written to sm instead
// of the global StateManager. This supports multi-project setups where each
// project has its own database.
func (h *HeartbeatManager) Start(workflowID core.WorkflowID, sm core.StateManager) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If already tracking, do nothing
	if _, exists := h.active[workflowID]; exists {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.active[workflowID] = cancel
	h.lastWriteSuccess[workflowID] = time.Now()

	if sm != nil {
		h.workflowSMs[workflowID] = sm
	}

	go h.heartbeatLoop(ctx, workflowID)

	h.logger.Debug("started heartbeat tracking", "workflow_id", workflowID)
}

// Stop ends heartbeat tracking for a workflow.
func (h *HeartbeatManager) Stop(workflowID core.WorkflowID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if cancel, exists := h.active[workflowID]; exists {
		cancel()
		delete(h.active, workflowID)
		delete(h.lastWriteSuccess, workflowID)
		delete(h.workflowSMs, workflowID)
		h.logger.Debug("stopped heartbeat tracking", "workflow_id", workflowID)
	}
}

// IsTracking checks if a workflow is being tracked.
func (h *HeartbeatManager) IsTracking(workflowID core.WorkflowID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, exists := h.active[workflowID]
	return exists
}

// IsHealthy reports whether a workflow's heartbeat is being written successfully.
// Returns false if the workflow is not tracked or its last successful write
// is older than StaleThreshold.
func (h *HeartbeatManager) IsHealthy(workflowID core.WorkflowID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	lastWrite, ok := h.lastWriteSuccess[workflowID]
	if !ok {
		return false
	}
	return time.Since(lastWrite) < h.config.StaleThreshold
}

// heartbeatLoop writes heartbeats periodically until stopped.
func (h *HeartbeatManager) heartbeatLoop(ctx context.Context, workflowID core.WorkflowID) {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	// Write initial heartbeat
	h.writeHeartbeat(workflowID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.writeHeartbeat(workflowID)
		}
	}
}

// getWorkflowSM returns the StateManager for a specific workflow.
// If a per-workflow SM was registered via Start(), it is returned;
// otherwise the global (server-level) StateManager is used.
func (h *HeartbeatManager) getWorkflowSM(workflowID core.WorkflowID) core.StateManager {
	h.mu.Lock()
	sm, ok := h.workflowSMs[workflowID]
	h.mu.Unlock()
	if ok {
		return sm
	}
	return h.stateManager
}

// writeHeartbeat updates the heartbeat timestamp in the database.
func (h *HeartbeatManager) writeHeartbeat(workflowID core.WorkflowID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sm := h.getWorkflowSM(workflowID)
	if err := sm.UpdateHeartbeat(ctx, workflowID); err != nil {
		h.logger.Warn("failed to write heartbeat",
			"workflow_id", workflowID,
			"error", err)
	} else {
		h.mu.Lock()
		if _, active := h.active[workflowID]; active {
			h.lastWriteSuccess[workflowID] = time.Now()
		}
		h.mu.Unlock()
	}
}

// StartZombieDetector begins periodic zombie detection.
func (h *HeartbeatManager) StartZombieDetector(handler ZombieHandler) {
	if handler == nil {
		h.logger.Warn("zombie detector started without handler")
		return
	}

	h.zombieHandler = handler

	ctx, cancel := context.WithCancel(context.Background())
	h.detectorCancel = cancel

	go h.zombieDetectorLoop(ctx)

	h.logger.Info("zombie detector started",
		"check_interval", h.config.CheckInterval,
		"stale_threshold", h.config.StaleThreshold,
		"auto_resume", h.config.AutoResume)
}

// StopZombieDetector stops the zombie detector.
func (h *HeartbeatManager) StopZombieDetector() {
	if h.detectorCancel != nil {
		h.detectorCancel()
		h.detectorCancel = nil
		h.logger.Info("zombie detector stopped")
	}
}

// zombieDetectorLoop checks for zombie workflows periodically.
func (h *HeartbeatManager) zombieDetectorLoop(ctx context.Context) {
	ticker := time.NewTicker(h.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.detectZombies()
		}
	}
}

// detectZombies finds and handles zombie workflows.
// It queries all unique StateManagers (global + per-workflow) so that zombies
// in project-scoped databases are also detected.
func (h *HeartbeatManager) detectZombies() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Collect unique StateManagers to query.
	h.mu.Lock()
	uniqueSMs := make(map[core.StateManager]struct{})
	uniqueSMs[h.stateManager] = struct{}{}
	for _, sm := range h.workflowSMs {
		uniqueSMs[sm] = struct{}{}
	}
	h.mu.Unlock()

	// Query each SM and deduplicate results by WorkflowID.
	seen := make(map[core.WorkflowID]struct{})
	var zombies []*core.WorkflowState
	for sm := range uniqueSMs {
		found, err := sm.FindZombieWorkflows(ctx, h.config.StaleThreshold)
		if err != nil {
			h.logger.Error("failed to find zombie workflows", "error", err)
			continue
		}
		for _, z := range found {
			if _, dup := seen[z.WorkflowID]; !dup {
				seen[z.WorkflowID] = struct{}{}
				zombies = append(zombies, z)
			}
		}
	}

	for _, zombie := range zombies {
		if h.IsTracking(zombie.WorkflowID) {
			// Tracked workflows might have temporary heartbeat write failures.
			// Only treat as zombie if heartbeat is critically stale (3x threshold).
			if zombie.HeartbeatAt != nil {
				staleDuration := time.Since(*zombie.HeartbeatAt)
				if staleDuration < 3*h.config.StaleThreshold {
					continue // Likely temporary — skip
				}
			}
			// Critically stale despite being tracked — real zombie within this server session
			h.logger.Warn("tracked workflow has critically stale heartbeat, treating as zombie",
				"workflow_id", zombie.WorkflowID,
				"heartbeat_at", zombie.HeartbeatAt)
			h.Stop(zombie.WorkflowID) // Clean up tracking before handling
		}

		h.logger.Warn("zombie workflow detected",
			"workflow_id", zombie.WorkflowID,
			"phase", zombie.CurrentPhase,
			"heartbeat_at", zombie.HeartbeatAt)

		if h.zombieHandler != nil {
			h.zombieHandler(zombie)
		}
	}
}

// HandleZombie processes a detected zombie workflow.
// This is the default handler that can be used or customized.
func (h *HeartbeatManager) HandleZombie(state *core.WorkflowState, executor interface {
	Resume(ctx context.Context, workflowID core.WorkflowID) error
}) {
	ctx := context.Background()

	// Check if we can auto-resume
	canResume := h.config.AutoResume &&
		state.ResumeCount < state.MaxResumes &&
		state.MaxResumes > 0

	if canResume {
		h.logger.Info("auto-resuming zombie workflow",
			"workflow_id", state.WorkflowID,
			"resume_count", state.ResumeCount+1,
			"max_resumes", state.MaxResumes)

		// Increment resume count
		state.ResumeCount++
		now := time.Now()
		state.HeartbeatAt = &now
		state.UpdatedAt = now

		// Add checkpoint explaining the auto-resume
		state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
			ID:        fmt.Sprintf("auto-resume-%d", time.Now().UnixNano()),
			Type:      "auto-resume",
			Phase:     state.CurrentPhase,
			Timestamp: now,
			Message: fmt.Sprintf("Auto-resumed after detecting stale heartbeat (attempt %d/%d)",
				state.ResumeCount, state.MaxResumes),
		})

		// Save updated state (use per-workflow SM if available)
		sm := h.getWorkflowSM(state.WorkflowID)
		if err := sm.Save(ctx, state); err != nil {
			h.logger.Error("failed to save state before auto-resume",
				"workflow_id", state.WorkflowID,
				"error", err)
			return
		}

		// Trigger resume
		if executor != nil {
			if err := executor.Resume(ctx, state.WorkflowID); err != nil {
				h.logger.Error("auto-resume failed",
					"workflow_id", state.WorkflowID,
					"error", err)
			}
		}

	} else {
		// Can't resume - mark as failed
		reason := "Zombie workflow detected (stale heartbeat)"
		if state.MaxResumes > 0 && state.ResumeCount >= state.MaxResumes {
			reason = fmt.Sprintf("Max auto-resumes reached (%d/%d)", state.ResumeCount, state.MaxResumes)
		} else if !h.config.AutoResume {
			reason = "Zombie workflow detected (auto-resume disabled)"
		}

		h.logger.Warn("marking zombie workflow as failed",
			"workflow_id", state.WorkflowID,
			"reason", reason)

		state.Status = core.WorkflowStatusFailed
		state.Error = reason
		state.UpdatedAt = time.Now()

		state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
			ID:        fmt.Sprintf("zombie-%d", time.Now().UnixNano()),
			Type:      "zombie_detected",
			Phase:     state.CurrentPhase,
			Timestamp: time.Now(),
			Message:   reason,
		})

		sm := h.getWorkflowSM(state.WorkflowID)
		if err := sm.Save(ctx, state); err != nil {
			h.logger.Error("failed to save zombie state",
				"workflow_id", state.WorkflowID,
				"error", err)
		}

		// Clear running_workflows entry to prevent zombie re-detection
		if clearer, ok := sm.(interface {
			ClearWorkflowRunning(context.Context, core.WorkflowID) error
		}); ok {
			_ = clearer.ClearWorkflowRunning(ctx, state.WorkflowID)
		}
	}
}

// Shutdown stops all heartbeat tracking and the zombie detector.
func (h *HeartbeatManager) Shutdown() {
	// Stop zombie detector
	h.StopZombieDetector()

	// Stop all active heartbeats
	h.mu.Lock()
	defer h.mu.Unlock()

	for workflowID, cancel := range h.active {
		cancel()
		delete(h.active, workflowID)
	}
	h.lastWriteSuccess = make(map[core.WorkflowID]time.Time)
	h.workflowSMs = make(map[core.WorkflowID]core.StateManager)

	h.logger.Info("heartbeat manager shutdown complete")
}
