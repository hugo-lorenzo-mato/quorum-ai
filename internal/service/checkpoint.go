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
	CheckpointPhaseStart    CheckpointType = "phase_start"
	CheckpointPhaseComplete CheckpointType = "phase_complete"
	CheckpointTaskStart     CheckpointType = "task_start"
	CheckpointTaskComplete  CheckpointType = "task_complete"
	CheckpointConsensus     CheckpointType = "consensus"
	CheckpointError         CheckpointType = "error"
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

	return m.CreateCheckpoint(ctx, state, cpType, map[string]interface{}{
		"phase": string(phase),
	})
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
	Phase        core.Phase
	TaskID       core.TaskID
	CheckpointID string
	FromStart    bool
	RestartPhase bool
	RestartTask  bool
	AfterError   bool
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
