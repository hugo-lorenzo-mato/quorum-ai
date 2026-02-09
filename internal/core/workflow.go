package core

import (
	"fmt"
	"time"
)

// WorkflowID uniquely identifies a workflow run.
type WorkflowID string

// WorkflowStatus represents the current state of a workflow.
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusAborted   WorkflowStatus = "aborted"
)

// Workflow represents a complete orchestration run.
type Workflow struct {
	ID             WorkflowID
	Status         WorkflowStatus
	CurrentPhase   Phase
	Prompt         string
	Tasks          map[TaskID]*Task
	TaskOrder      []TaskID
	Blueprint      *Blueprint
	ConsensusScore float64
	TotalTokensIn  int
	TotalTokensOut int
	CreatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Error          string
}

// Blueprint captures the complete orchestration recipe for a workflow.
// It replaces the former WorkflowConfig with richer, structured sub-sections.
type Blueprint struct {
	ExecutionMode   string                   `json:"execution_mode"`
	SingleAgent     BlueprintSingleAgent     `json:"single_agent,omitempty"`
	Phases          BlueprintPhases          `json:"phases"`
	Consensus       BlueprintConsensus       `json:"consensus"`
	Refiner         BlueprintRefiner         `json:"refiner"`
	Synthesizer     BlueprintSynthesizer     `json:"synthesizer"`
	PlanSynthesizer BlueprintPlanSynthesizer `json:"plan_synthesizer"`
	MaxRetries      int                      `json:"max_retries"`
	Timeout         time.Duration            `json:"timeout"`
	DryRun          bool                     `json:"dry_run"`
}

// BlueprintSingleAgent configures single-agent execution mode.
type BlueprintSingleAgent struct {
	Agent           string `json:"agent,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// BlueprintPhases holds per-phase timeout configuration.
type BlueprintPhases struct {
	Analyze BlueprintPhaseTimeout `json:"analyze"`
	Plan    BlueprintPhaseTimeout `json:"plan"`
	Execute BlueprintPhaseTimeout `json:"execute"`
}

// BlueprintPhaseTimeout holds timeout for a single phase.
type BlueprintPhaseTimeout struct {
	Timeout time.Duration `json:"timeout"`
}

// BlueprintConsensus configures multi-agent consensus evaluation.
type BlueprintConsensus struct {
	Enabled             bool               `json:"enabled"`
	Agent               string             `json:"agent"`
	Threshold           float64            `json:"threshold"`
	Thresholds          map[string]float64 `json:"thresholds,omitempty"`
	MinRounds           int                `json:"min_rounds"`
	MaxRounds           int                `json:"max_rounds"`
	WarningThreshold    float64            `json:"warning_threshold"`
	StagnationThreshold float64            `json:"stagnation_threshold"`
}

// BlueprintRefiner configures the prompt refinement phase.
type BlueprintRefiner struct {
	Enabled bool   `json:"enabled"`
	Agent   string `json:"agent"`
}

// BlueprintSynthesizer configures the analysis synthesis phase.
type BlueprintSynthesizer struct {
	Agent string `json:"agent"`
}

// BlueprintPlanSynthesizer configures multi-agent plan synthesis.
type BlueprintPlanSynthesizer struct {
	Enabled bool   `json:"enabled"`
	Agent   string `json:"agent"`
}

// NewWorkflow creates a new workflow instance.
func NewWorkflow(id WorkflowID, prompt string, bp *Blueprint) *Workflow {
	if bp == nil {
		bp = &Blueprint{
			Consensus: BlueprintConsensus{
				Threshold: 0.75,
			},
			MaxRetries: 3,
			Timeout:    time.Hour,
		}
	}
	return &Workflow{
		ID:           id,
		Status:       WorkflowStatusPending,
		CurrentPhase: PhaseAnalyze,
		Prompt:       prompt,
		Tasks:        make(map[TaskID]*Task),
		TaskOrder:    make([]TaskID, 0),
		Blueprint:    bp,
		CreatedAt:    time.Now(),
	}
}

// AddTask adds a task to the workflow.
func (w *Workflow) AddTask(task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if _, exists := w.Tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}
	w.Tasks[task.ID] = task
	w.TaskOrder = append(w.TaskOrder, task.ID)
	return nil
}

// GetTask retrieves a task by ID.
func (w *Workflow) GetTask(id TaskID) (*Task, bool) {
	task, ok := w.Tasks[id]
	return task, ok
}

// TasksByPhase returns all tasks for a given phase.
func (w *Workflow) TasksByPhase(phase Phase) []*Task {
	var tasks []*Task
	for _, id := range w.TaskOrder {
		if task := w.Tasks[id]; task.Phase == phase {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// CompletedTasks returns a map of completed task IDs.
func (w *Workflow) CompletedTasks() map[TaskID]bool {
	completed := make(map[TaskID]bool)
	for id, task := range w.Tasks {
		if task.Status == TaskStatusCompleted {
			completed[id] = true
		}
	}
	return completed
}

// ReadyTasks returns tasks that are ready to execute.
func (w *Workflow) ReadyTasks() []*Task {
	completed := w.CompletedTasks()
	var ready []*Task
	for _, id := range w.TaskOrder {
		task := w.Tasks[id]
		if task.IsReady(completed) {
			ready = append(ready, task)
		}
	}
	return ready
}

// UpdateMetrics recalculates aggregated metrics.
func (w *Workflow) UpdateMetrics() {
	w.TotalTokensIn = 0
	w.TotalTokensOut = 0
	for _, task := range w.Tasks {
		w.TotalTokensIn += task.TokensIn
		w.TotalTokensOut += task.TokensOut
	}
}

// Progress returns the completion percentage.
func (w *Workflow) Progress() float64 {
	if len(w.Tasks) == 0 {
		return 0
	}
	completed := 0
	for _, task := range w.Tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusSkipped {
			completed++
		}
	}
	return float64(completed) / float64(len(w.Tasks)) * 100
}

// Start transitions workflow to running state.
func (w *Workflow) Start() error {
	if w.Status != WorkflowStatusPending && w.Status != WorkflowStatusPaused {
		return fmt.Errorf("cannot start workflow in %s state", w.Status)
	}
	w.Status = WorkflowStatusRunning
	if w.StartedAt == nil {
		now := time.Now()
		w.StartedAt = &now
	}
	return nil
}

// Pause transitions workflow to paused state.
func (w *Workflow) Pause() error {
	if w.Status != WorkflowStatusRunning {
		return fmt.Errorf("cannot pause workflow in %s state", w.Status)
	}
	w.Status = WorkflowStatusPaused
	return nil
}

// Resume transitions workflow from paused to running.
func (w *Workflow) Resume() error {
	if w.Status != WorkflowStatusPaused {
		return fmt.Errorf("cannot resume workflow in %s state", w.Status)
	}
	w.Status = WorkflowStatusRunning
	return nil
}

// Complete transitions workflow to completed state.
func (w *Workflow) Complete() error {
	if w.Status != WorkflowStatusRunning {
		return fmt.Errorf("cannot complete workflow in %s state", w.Status)
	}
	w.Status = WorkflowStatusCompleted
	now := time.Now()
	w.CompletedAt = &now
	w.UpdateMetrics()
	return nil
}

// Fail transitions workflow to failed state.
func (w *Workflow) Fail(err error) error {
	w.Status = WorkflowStatusFailed
	w.Error = err.Error()
	now := time.Now()
	w.CompletedAt = &now
	w.UpdateMetrics()
	return nil
}

// Abort transitions workflow to aborted state.
func (w *Workflow) Abort(reason string) error {
	w.Status = WorkflowStatusAborted
	w.Error = reason
	now := time.Now()
	w.CompletedAt = &now
	w.UpdateMetrics()
	return nil
}

// Duration returns the workflow execution duration.
func (w *Workflow) Duration() time.Duration {
	if w.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if w.CompletedAt != nil {
		end = *w.CompletedAt
	}
	return end.Sub(*w.StartedAt)
}

// IsTerminal returns true if the workflow is in a terminal state.
func (w *Workflow) IsTerminal() bool {
	return w.Status == WorkflowStatusCompleted ||
		w.Status == WorkflowStatusFailed ||
		w.Status == WorkflowStatusAborted
}

// AdvancePhase moves to the next phase.
func (w *Workflow) AdvancePhase() error {
	next := NextPhase(w.CurrentPhase)
	if next == "" {
		return fmt.Errorf("already at final phase: %s", w.CurrentPhase)
	}
	w.CurrentPhase = next
	return nil
}

// Validate checks workflow invariants.
func (w *Workflow) Validate() error {
	if w.ID == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "WORKFLOW_ID_REQUIRED",
			Message:  "workflow ID cannot be empty",
		}
	}
	if w.Prompt == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "WORKFLOW_PROMPT_REQUIRED",
			Message:  "workflow prompt cannot be empty",
		}
	}
	return nil
}
