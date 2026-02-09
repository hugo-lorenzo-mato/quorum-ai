package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// CheckpointManager manages workflow checkpoints.
type CheckpointManager struct {
	state  core.StateManager
	logger *logging.Logger
}

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager(state core.StateManager, logger *logging.Logger) *CheckpointManager {
	return &CheckpointManager{
		state:  state,
		logger: logger,
	}
}

// CheckpointType indicates the type of checkpoint.
type CheckpointType string

const (
	CheckpointPhaseStart       CheckpointType = "phase_start"
	CheckpointPhaseComplete    CheckpointType = "phase_complete"
	CheckpointTaskStart        CheckpointType = "task_start"
	CheckpointTaskComplete     CheckpointType = "task_complete"
	CheckpointConsensus        CheckpointType = "consensus"
	CheckpointError            CheckpointType = "error"
	CheckpointModeratorRound   CheckpointType = "moderator_round"   // Checkpoint after each moderator evaluation
	CheckpointAnalysisRound    CheckpointType = "analysis_round"    // Checkpoint after each V(n) refinement
	CheckpointAnalysisComplete CheckpointType = "analysis_complete" // Per-agent analysis completion with metrics
)

// CreateCheckpoint saves a checkpoint at the current state.
func (m *CheckpointManager) CreateCheckpoint(ctx context.Context, state *core.WorkflowState, cpType CheckpointType, metadata map[string]interface{}) error {
	checkpoint := core.Checkpoint{
		ID:        generateCheckpointID(),
		Type:      string(cpType),
		Phase:     state.CurrentPhase,
		Timestamp: time.Now(),
	}

	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshaling checkpoint metadata: %w", err)
		}
		checkpoint.Data = data
	}

	// Append checkpoint to state
	state.Checkpoints = append(state.Checkpoints, checkpoint)
	state.UpdatedAt = time.Now()

	// Save state
	if err := m.state.Save(ctx, state); err != nil {
		return fmt.Errorf("saving checkpoint: %w", err)
	}

	m.logger.Info("checkpoint created",
		"checkpoint_id", checkpoint.ID,
		"type", cpType,
		"phase", state.CurrentPhase,
	)

	return nil
}

// PhaseCheckpoint creates a checkpoint for phase transitions.
func (m *CheckpointManager) PhaseCheckpoint(ctx context.Context, state *core.WorkflowState, phase core.Phase, complete bool) error {
	cpType := CheckpointPhaseStart
	if complete {
		cpType = CheckpointPhaseComplete
	}

	checkpoint := core.Checkpoint{
		ID:        generateCheckpointID(),
		Type:      string(cpType),
		Phase:     phase, // Use explicit phase parameter (state.CurrentPhase may already have advanced).
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(map[string]interface{}{
		"phase": string(phase),
	})
	if err != nil {
		return fmt.Errorf("marshaling checkpoint metadata: %w", err)
	}
	checkpoint.Data = data

	// Append checkpoint to state
	state.Checkpoints = append(state.Checkpoints, checkpoint)
	state.UpdatedAt = time.Now()

	// Save state
	if err := m.state.Save(ctx, state); err != nil {
		return fmt.Errorf("saving checkpoint: %w", err)
	}

	m.logger.Info("checkpoint created",
		"checkpoint_id", checkpoint.ID,
		"type", cpType,
		"phase", phase,
	)

	return nil
}

// TaskCheckpoint creates a checkpoint for task transitions.
func (m *CheckpointManager) TaskCheckpoint(ctx context.Context, state *core.WorkflowState, task *core.Task, complete bool) error {
	cpType := CheckpointTaskStart
	if complete {
		cpType = CheckpointTaskComplete
	}

	return m.CreateCheckpoint(ctx, state, cpType, map[string]interface{}{
		"task_id":   string(task.ID),
		"task_name": task.Name,
		"status":    string(task.Status),
	})
}

// ErrorCheckpoint creates a checkpoint on error.
func (m *CheckpointManager) ErrorCheckpoint(ctx context.Context, state *core.WorkflowState, err error) error {
	return m.CreateCheckpoint(ctx, state, CheckpointError, map[string]interface{}{
		"error": err.Error(),
	})
}

// ErrorCheckpointWithContext creates a detailed error checkpoint with full context.
// This includes agent name, phase, round number, and any additional context
// that will help with debugging and recovery.
func (m *CheckpointManager) ErrorCheckpointWithContext(ctx context.Context, state *core.WorkflowState, err error, details ErrorCheckpointDetails) error {
	metadata := map[string]interface{}{
		"error":       err.Error(),
		"error_type":  fmt.Sprintf("%T", err),
		"phase":       string(state.CurrentPhase),
		"occurred_at": time.Now().Format(time.RFC3339),
	}

	// Add optional context fields
	if details.Agent != "" {
		metadata["agent"] = details.Agent
	}
	if details.Model != "" {
		metadata["model"] = details.Model
	}
	if details.Round > 0 {
		metadata["round"] = details.Round
	}
	if details.Attempt > 0 {
		metadata["attempt"] = details.Attempt
	}
	if details.DurationMS > 0 {
		metadata["duration_ms"] = details.DurationMS
	}
	if details.TokensIn > 0 {
		metadata["tokens_in"] = details.TokensIn
	}
	if details.TokensOut > 0 {
		metadata["tokens_out"] = details.TokensOut
	}
	if details.OutputSample != "" {
		// Truncate to avoid huge checkpoints
		sample := details.OutputSample
		if len(sample) > 500 {
			sample = sample[:500] + "...[truncated]"
		}
		metadata["output_sample"] = sample
	}
	if details.IsTransient {
		metadata["is_transient"] = true
	}
	if details.IsValidationError {
		metadata["is_validation_error"] = true
	}
	if len(details.FallbacksTried) > 0 {
		metadata["fallbacks_tried"] = details.FallbacksTried
	}
	if details.Extra != nil {
		for k, v := range details.Extra {
			metadata[k] = v
		}
	}

	return m.CreateCheckpoint(ctx, state, CheckpointError, metadata)
}

// ErrorCheckpointDetails contains detailed information about an error for checkpointing.
type ErrorCheckpointDetails struct {
	Agent             string            // Agent that failed
	Model             string            // Model being used
	Round             int               // Round number (for moderator/analysis)
	Attempt           int               // Retry attempt number
	DurationMS        int64             // How long the operation ran before failing
	TokensIn          int               // Input tokens (if known)
	TokensOut         int               // Output tokens (if known)
	OutputSample      string            // Sample of output before failure (for debugging)
	IsTransient       bool              // Whether error appears transient
	IsValidationError bool              // Whether error is validation-related
	FallbacksTried    []string          // List of fallback agents already tried
	Extra             map[string]string // Additional context-specific fields
}

// GetLastCheckpoint returns the most recent checkpoint.
func (m *CheckpointManager) GetLastCheckpoint(state *core.WorkflowState) *core.Checkpoint {
	if len(state.Checkpoints) == 0 {
		return nil
	}
	return &state.Checkpoints[len(state.Checkpoints)-1]
}

// GetLastCheckpointOfType returns the most recent checkpoint of a specific type.
func (m *CheckpointManager) GetLastCheckpointOfType(state *core.WorkflowState, cpType CheckpointType) *core.Checkpoint {
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		if state.Checkpoints[i].Type == string(cpType) {
			return &state.Checkpoints[i]
		}
	}
	return nil
}

// ResumePoint describes where and how to resume a workflow.
type ResumePoint struct {
	Phase          core.Phase
	TaskID         core.TaskID
	CheckpointID   string
	FromStart      bool
	RestartPhase   bool
	RestartTask    bool
	AfterError     bool
	ModeratorRound int                    // Round number from moderator checkpoint (for resuming analysis)
	SavedOutputs   string                 // Serialized agent outputs from checkpoint
	Metadata       map[string]interface{} // Full checkpoint metadata
}

// GetResumePoint determines where to resume from.
func (m *CheckpointManager) GetResumePoint(state *core.WorkflowState) (*ResumePoint, error) {
	if len(state.Checkpoints) == 0 {
		return &ResumePoint{
			Phase:     core.PhaseAnalyze,
			FromStart: true,
		}, nil
	}

	lastCP := m.GetLastCheckpoint(state)
	if lastCP == nil {
		return nil, fmt.Errorf("no valid checkpoint found")
	}

	resumePoint := &ResumePoint{
		Phase:        lastCP.Phase,
		CheckpointID: lastCP.ID,
	}

	// Parse metadata
	var metadata map[string]interface{}
	if len(lastCP.Data) > 0 {
		if err := json.Unmarshal(lastCP.Data, &metadata); err != nil {
			m.logger.Warn("failed to parse checkpoint metadata", "error", err)
		} else {
			if taskID, ok := metadata["task_id"].(string); ok {
				resumePoint.TaskID = core.TaskID(taskID)
			}
		}
	}

	// Store metadata for later use
	resumePoint.Metadata = metadata

	// Determine restart behavior based on checkpoint type
	switch CheckpointType(lastCP.Type) {
	case CheckpointPhaseStart:
		resumePoint.RestartPhase = true
	case CheckpointPhaseComplete:
		// Phase completed - advance to the next phase
		// This is critical for /plan after /analyze to work correctly
		resumePoint.Phase = nextPhase(lastCP.Phase)
	case CheckpointTaskStart:
		resumePoint.RestartTask = true
	case CheckpointError:
		resumePoint.AfterError = true
	case CheckpointModeratorRound:
		// Resume from moderator round checkpoint - extract round info
		if round, ok := metadata["round"].(float64); ok {
			resumePoint.ModeratorRound = int(round)
		}
		if outputs, ok := metadata["outputs"].(string); ok {
			resumePoint.SavedOutputs = outputs
		}
		// Don't restart the phase, continue from saved state
		resumePoint.RestartPhase = false
	case CheckpointAnalysisRound:
		// Similar to moderator round but for V(n) analysis outputs
		if round, ok := metadata["round"].(float64); ok {
			resumePoint.ModeratorRound = int(round)
		}
		if outputs, ok := metadata["outputs"].(string); ok {
			resumePoint.SavedOutputs = outputs
		}
		resumePoint.RestartPhase = false
	}

	return resumePoint, nil
}

// nextPhase returns the phase that follows the given phase.
func nextPhase(current core.Phase) core.Phase {
	switch current {
	case core.PhaseRefine:
		return core.PhaseAnalyze
	case core.PhaseAnalyze:
		return core.PhasePlan
	case core.PhasePlan:
		return core.PhaseExecute
	default:
		return current // No next phase
	}
}

// CleanupOldCheckpoints removes checkpoints older than the retention period.
func (m *CheckpointManager) CleanupOldCheckpoints(state *core.WorkflowState, retention time.Duration) int {
	cutoff := time.Now().Add(-retention)
	cleaned := 0

	newCheckpoints := make([]core.Checkpoint, 0)
	for _, cp := range state.Checkpoints {
		if cp.Timestamp.After(cutoff) {
			newCheckpoints = append(newCheckpoints, cp)
		} else {
			cleaned++
		}
	}

	state.Checkpoints = newCheckpoints
	return cleaned
}

// Progress contains workflow progress information.
type Progress struct {
	Phase            core.Phase
	TotalTasks       int
	CompletedTasks   int
	RunningTasks     int
	FailedTasks      int
	Percentage       float64
	TotalCheckpoints int
}

// GetProgress returns workflow progress information.
func (m *CheckpointManager) GetProgress(state *core.WorkflowState) *Progress {
	progress := &Progress{
		Phase:            state.CurrentPhase,
		TotalCheckpoints: len(state.Checkpoints),
	}

	// Count tasks by status
	for _, task := range state.Tasks {
		progress.TotalTasks++
		switch task.Status {
		case core.TaskStatusCompleted:
			progress.CompletedTasks++
		case core.TaskStatusRunning:
			progress.RunningTasks++
		case core.TaskStatusFailed:
			progress.FailedTasks++
		}
	}

	if progress.TotalTasks > 0 {
		progress.Percentage = float64(progress.CompletedTasks) / float64(progress.TotalTasks) * 100
	}

	return progress
}

func generateCheckpointID() string {
	return fmt.Sprintf("cp-%d", time.Now().UnixNano())
}
