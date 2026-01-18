package tui

import (
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Output is the interface for all output handlers.
// It provides a uniform API for TUI, plain text, and JSON output modes.
type Output interface {
	// WorkflowStarted is called when a workflow begins.
	WorkflowStarted(prompt string)
	// PhaseStarted is called when a phase begins.
	PhaseStarted(phase core.Phase)
	// TaskStarted is called when a task begins.
	TaskStarted(task *core.Task)
	// TaskCompleted is called when a task finishes successfully.
	TaskCompleted(task *core.Task, duration time.Duration)
	// TaskFailed is called when a task fails.
	TaskFailed(task *core.Task, err error)
	// TaskSkipped is called when a task is skipped.
	TaskSkipped(task *core.Task, reason string)
	// WorkflowStateUpdated is called when workflow state changes (e.g., tasks created).
	// This is semantically different from WorkflowCompleted.
	WorkflowStateUpdated(state *core.WorkflowState)
	// WorkflowCompleted is called when the workflow finishes successfully.
	// This should only be called ONCE at the end of the workflow.
	WorkflowCompleted(state *core.WorkflowState)
	// WorkflowFailed is called when the workflow fails.
	WorkflowFailed(err error)
	// Log outputs a log message.
	Log(level, message string)
	// Close cleans up resources.
	Close() error
}

// TUIOutput wraps the bubbletea Model for the Output interface.
type TUIOutput struct {
	model     Model
	program   *tea.Program
	updateC   chan tea.Msg // Normal updates (ring buffer behavior)
	priorityC chan tea.Msg // Critical updates (blocking, never drop)
	dropCount int64        // Counter for dropped events
	mu        sync.Mutex
}

// NewTUIOutput creates a new TUI output handler.
func NewTUIOutput() *TUIOutput {
	t := &TUIOutput{
		model:     New(),
		updateC:   make(chan tea.Msg, 100),
		priorityC: make(chan tea.Msg, 20),
	}
	return t
}

// DroppedEvents returns the count of dropped events.
func (t *TUIOutput) DroppedEvents() int64 {
	return atomic.LoadInt64(&t.dropCount)
}

// Start starts the TUI program (should be called in a goroutine).
func (t *TUIOutput) Start() error {
	// Create program with the update channel
	t.program = tea.NewProgram(t.model, tea.WithAltScreen())

	// Start goroutine for normal updates
	go func() {
		for msg := range t.updateC {
			if t.program != nil {
				t.program.Send(msg)
			}
		}
	}()

	// Start goroutine for priority updates (separate to avoid blocking)
	go func() {
		for msg := range t.priorityC {
			if t.program != nil {
				t.program.Send(msg)
			}
		}
	}()

	_, err := t.program.Run()
	return err
}

// sendNormal sends a message with ring buffer behavior (drops oldest if full).
func (t *TUIOutput) sendNormal(msg tea.Msg) {
	if t.updateC == nil {
		return
	}

	dropped := false
	droppedCount := int64(0)
	select {
	case t.updateC <- msg:
		// Sent successfully
	default:
		// Buffer full - implement ring buffer by dropping oldest
		select {
		case <-t.updateC:
			// Dropped oldest
			dropped = true
			droppedCount = atomic.AddInt64(&t.dropCount, 1)
		default:
			// Channel empty (shouldn't happen, but handle gracefully)
		}
		// Try again
		select {
		case t.updateC <- msg:
		default:
			// Still can't send, drop current
			dropped = true
			droppedCount = atomic.AddInt64(&t.dropCount, 1)
		}
	}

	if dropped {
		t.sendDropped(droppedCount)
	}
}

func (t *TUIOutput) sendDropped(count int64) {
	if t.updateC == nil {
		return
	}
	select {
	case t.updateC <- DroppedEventsMsg{Count: count}:
	default:
	}
}

// sendPriority sends a message with blocking behavior (never drops).
func (t *TUIOutput) sendPriority(msg tea.Msg) {
	if t.priorityC == nil {
		return
	}
	// Blocking send - will wait until there's room
	t.priorityC <- msg
}

// WorkflowStarted implements Output.
func (t *TUIOutput) WorkflowStarted(_ string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(LogMsg{Time: time.Now(), Level: "info", Message: "Workflow started"})
}

// PhaseStarted implements Output.
func (t *TUIOutput) PhaseStarted(phase core.Phase) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(PhaseUpdateMsg{Phase: phase})
	t.sendNormal(LogMsg{Time: time.Now(), Level: "info", Message: "Phase: " + string(phase)})
}

// TaskStarted implements Output.
func (t *TUIOutput) TaskStarted(task *core.Task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusRunning})
}

// TaskCompleted implements Output.
func (t *TUIOutput) TaskCompleted(task *core.Task, _ time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusCompleted})
}

// TaskFailed implements Output.
func (t *TUIOutput) TaskFailed(task *core.Task, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	t.sendNormal(TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusFailed, Error: errStr})
}

// WorkflowStateUpdated implements Output.
func (t *TUIOutput) WorkflowStateUpdated(state *core.WorkflowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(WorkflowUpdateMsg{State: state})
}

// TaskSkipped implements Output.
func (t *TUIOutput) TaskSkipped(task *core.Task, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusSkipped, Error: reason})
}

// WorkflowCompleted implements Output.
func (t *TUIOutput) WorkflowCompleted(state *core.WorkflowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// PRIORITY: This event should never be dropped
	t.sendPriority(WorkflowUpdateMsg{State: state})
}

// WorkflowFailed implements Output.
func (t *TUIOutput) WorkflowFailed(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// PRIORITY: This event should never be dropped
	t.sendPriority(ErrorMsg{Error: err})
}

// Log implements Output.
func (t *TUIOutput) Log(level, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(LogMsg{Time: time.Now(), Level: level, Message: message})
}

// Close implements Output.
func (t *TUIOutput) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		close(t.updateC)
		t.updateC = nil
	}
	if t.priorityC != nil {
		close(t.priorityC)
		t.priorityC = nil
	}
	if t.program != nil {
		t.program.Quit()
	}
	return nil
}

// SendWorkflowState sends a full workflow state update to the TUI.
func (t *TUIOutput) SendWorkflowState(state *core.WorkflowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendNormal(WorkflowUpdateMsg{State: state})
}

// FallbackOutputAdapter adapts FallbackOutput to the Output interface.
type FallbackOutputAdapter struct {
	fallback *FallbackOutput
}

// NewFallbackOutputAdapter creates a new adapter.
func NewFallbackOutputAdapter(useColor, verbose bool) *FallbackOutputAdapter {
	return &FallbackOutputAdapter{
		fallback: NewFallbackOutput(useColor, verbose),
	}
}

// WorkflowStarted implements Output.
func (f *FallbackOutputAdapter) WorkflowStarted(prompt string) {
	f.fallback.WorkflowStarted(prompt)
}

// PhaseStarted implements Output.
func (f *FallbackOutputAdapter) PhaseStarted(phase core.Phase) {
	f.fallback.PhaseStarted(phase)
}

// TaskStarted implements Output.
func (f *FallbackOutputAdapter) TaskStarted(task *core.Task) {
	f.fallback.TaskStarted(task)
}

// TaskCompleted implements Output.
func (f *FallbackOutputAdapter) TaskCompleted(task *core.Task, duration time.Duration) {
	f.fallback.TaskCompleted(task, duration)
}

// TaskFailed implements Output.
func (f *FallbackOutputAdapter) TaskFailed(task *core.Task, err error) {
	f.fallback.TaskFailed(task, err)
}

// WorkflowStateUpdated implements Output.
func (f *FallbackOutputAdapter) WorkflowStateUpdated(state *core.WorkflowState) {
	// For fallback, state updates are logged as progress
	if f.fallback != nil {
		completed := 0
		total := len(state.Tasks)
		for _, t := range state.Tasks {
			if t.Status == core.TaskStatusCompleted {
				completed++
			}
		}
		// Only show progress if there are tasks to track
		if total > 0 {
			f.fallback.Progress(completed, total, string(state.CurrentPhase))
		}
	}
}

// TaskSkipped implements Output.
func (f *FallbackOutputAdapter) TaskSkipped(task *core.Task, reason string) {
	f.fallback.TaskSkipped(task, reason)
}

// WorkflowCompleted implements Output.
func (f *FallbackOutputAdapter) WorkflowCompleted(state *core.WorkflowState) {
	f.fallback.WorkflowCompleted(state)
}

// WorkflowFailed implements Output.
func (f *FallbackOutputAdapter) WorkflowFailed(err error) {
	f.fallback.WorkflowFailed(err)
}

// Log implements Output.
func (f *FallbackOutputAdapter) Log(level, message string) {
	f.fallback.Log(level, message)
}

// Close implements Output.
func (f *FallbackOutputAdapter) Close() error {
	return nil
}

// JSONOutputAdapter adapts JSONOutput to the Output interface.
type JSONOutputAdapter struct {
	json *JSONOutput
}

// NewJSONOutputAdapter creates a new adapter.
func NewJSONOutputAdapter() *JSONOutputAdapter {
	return &JSONOutputAdapter{
		json: NewJSONOutput(),
	}
}

// WorkflowStarted implements Output.
func (j *JSONOutputAdapter) WorkflowStarted(prompt string) {
	j.json.WorkflowStarted(prompt)
}

// PhaseStarted implements Output.
func (j *JSONOutputAdapter) PhaseStarted(phase core.Phase) {
	j.json.PhaseStarted(phase)
}

// TaskStarted implements Output.
func (j *JSONOutputAdapter) TaskStarted(task *core.Task) {
	j.json.emit("task_started", map[string]interface{}{
		"task_id": task.ID,
		"name":    task.Name,
	})
}

// TaskCompleted implements Output.
func (j *JSONOutputAdapter) TaskCompleted(task *core.Task, duration time.Duration) {
	j.json.TaskCompleted(task, duration)
}

// TaskFailed implements Output.
func (j *JSONOutputAdapter) TaskFailed(task *core.Task, err error) {
	j.json.TaskFailed(task, err)
}

// WorkflowStateUpdated implements Output.
func (j *JSONOutputAdapter) WorkflowStateUpdated(state *core.WorkflowState) {
	j.json.emit("workflow_state_updated", map[string]interface{}{
		"phase":         string(state.CurrentPhase),
		"total_tasks":   len(state.Tasks),
		"current_phase": string(state.CurrentPhase),
	})
}

// TaskSkipped implements Output.
func (j *JSONOutputAdapter) TaskSkipped(task *core.Task, reason string) {
	j.json.emit("task_skipped", map[string]interface{}{
		"task_id": task.ID,
		"name":    task.Name,
		"reason":  reason,
	})
}

// WorkflowCompleted implements Output.
func (j *JSONOutputAdapter) WorkflowCompleted(state *core.WorkflowState) {
	j.json.WorkflowCompleted(state)
}

// WorkflowFailed implements Output.
func (j *JSONOutputAdapter) WorkflowFailed(err error) {
	j.json.WorkflowFailed(err)
}

// Log implements Output.
func (j *JSONOutputAdapter) Log(level, message string) {
	j.json.Log(level, message)
}

// Close implements Output.
func (j *JSONOutputAdapter) Close() error {
	return nil
}

// QuietOutput is an output handler that suppresses output.
type QuietOutput struct{}

// NewQuietOutput creates a new quiet output handler.
func NewQuietOutput() *QuietOutput {
	return &QuietOutput{}
}

// WorkflowStarted implements Output.
func (q *QuietOutput) WorkflowStarted(_ string) {}

// PhaseStarted implements Output.
func (q *QuietOutput) PhaseStarted(_ core.Phase) {}

// TaskStarted implements Output.
func (q *QuietOutput) TaskStarted(_ *core.Task) {}

// TaskCompleted implements Output.
func (q *QuietOutput) TaskCompleted(_ *core.Task, _ time.Duration) {}

// TaskFailed implements Output.
func (q *QuietOutput) TaskFailed(_ *core.Task, _ error) {}

// WorkflowStateUpdated implements Output.
func (q *QuietOutput) WorkflowStateUpdated(_ *core.WorkflowState) {}

// TaskSkipped implements Output.
func (q *QuietOutput) TaskSkipped(_ *core.Task, _ string) {}

// WorkflowCompleted implements Output.
func (q *QuietOutput) WorkflowCompleted(_ *core.WorkflowState) {}

// WorkflowFailed implements Output.
func (q *QuietOutput) WorkflowFailed(_ error) {}

// Log implements Output.
func (q *QuietOutput) Log(_, _ string) {}

// Close implements Output.
func (q *QuietOutput) Close() error {
	return nil
}

// NewOutput creates an output handler based on the output mode.
func NewOutput(mode OutputMode, useColor, verbose bool) Output {
	switch mode {
	case ModeTUI:
		return NewTUIOutput()
	case ModePlain:
		return NewFallbackOutputAdapter(useColor, verbose)
	case ModeJSON:
		return NewJSONOutputAdapter()
	case ModeQuiet:
		return NewQuietOutput()
	default:
		return NewFallbackOutputAdapter(useColor, verbose)
	}
}

// OutputNotifierAdapter adapts an Output to the workflow.OutputNotifier interface.
// This allows the workflow package to send real-time updates to the TUI.
type OutputNotifierAdapter struct {
	output Output
}

// NewOutputNotifierAdapter creates a new adapter wrapping an Output.
func NewOutputNotifierAdapter(output Output) *OutputNotifierAdapter {
	return &OutputNotifierAdapter{output: output}
}

// PhaseStarted implements workflow.OutputNotifier.
func (a *OutputNotifierAdapter) PhaseStarted(phase core.Phase) {
	a.output.PhaseStarted(phase)
}

// TaskStarted implements workflow.OutputNotifier.
func (a *OutputNotifierAdapter) TaskStarted(task *core.Task) {
	a.output.TaskStarted(task)
}

// TaskCompleted implements workflow.OutputNotifier.
func (a *OutputNotifierAdapter) TaskCompleted(task *core.Task, duration time.Duration) {
	a.output.TaskCompleted(task, duration)
}

// TaskFailed implements workflow.OutputNotifier.
func (a *OutputNotifierAdapter) TaskFailed(task *core.Task, err error) {
	a.output.TaskFailed(task, err)
}

// WorkflowStateUpdated implements workflow.OutputNotifier.
// It sends the full workflow state to update the TUI task list.
// NOTE: This is semantically different from WorkflowCompleted.
func (a *OutputNotifierAdapter) WorkflowStateUpdated(state *core.WorkflowState) {
	a.output.WorkflowStateUpdated(state)
}

// TaskSkipped implements workflow.OutputNotifier.
func (a *OutputNotifierAdapter) TaskSkipped(task *core.Task, reason string) {
	a.output.TaskSkipped(task, reason)
}

// TraceNotifier is an interface for trace event recording.
// This matches service.TraceOutputNotifier but is defined here to avoid circular imports.
type TraceNotifier interface {
	PhaseStarted(phase string)
	TaskStarted(taskID, taskName, cli string)
	TaskCompleted(taskID, taskName string, duration time.Duration, tokensIn, tokensOut int, costUSD float64)
	TaskFailed(taskID, taskName string, err error)
	WorkflowStateUpdated(status string, totalTasks int)
	Close() error
}

// TracingOutputNotifierAdapter wraps an OutputNotifierAdapter and adds trace event recording.
// It implements workflow.OutputNotifier and delegates to both the base notifier and the trace notifier.
type TracingOutputNotifierAdapter struct {
	base   *OutputNotifierAdapter
	tracer TraceNotifier
}

// NewTracingOutputNotifierAdapter creates a new tracing output notifier adapter.
func NewTracingOutputNotifierAdapter(base *OutputNotifierAdapter, tracer TraceNotifier) *TracingOutputNotifierAdapter {
	return &TracingOutputNotifierAdapter{
		base:   base,
		tracer: tracer,
	}
}

// PhaseStarted implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) PhaseStarted(phase core.Phase) {
	if t.tracer != nil {
		t.tracer.PhaseStarted(string(phase))
	}
	if t.base != nil {
		t.base.PhaseStarted(phase)
	}
}

// TaskStarted implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) TaskStarted(task *core.Task) {
	if t.tracer != nil {
		t.tracer.TaskStarted(string(task.ID), task.Name, task.CLI)
	}
	if t.base != nil {
		t.base.TaskStarted(task)
	}
}

// TaskCompleted implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) TaskCompleted(task *core.Task, duration time.Duration) {
	if t.tracer != nil {
		t.tracer.TaskCompleted(string(task.ID), task.Name, duration, task.TokensIn, task.TokensOut, task.CostUSD)
	}
	if t.base != nil {
		t.base.TaskCompleted(task, duration)
	}
}

// TaskFailed implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) TaskFailed(task *core.Task, err error) {
	if t.tracer != nil {
		t.tracer.TaskFailed(string(task.ID), task.Name, err)
	}
	if t.base != nil {
		t.base.TaskFailed(task, err)
	}
}

// WorkflowStateUpdated implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) WorkflowStateUpdated(state *core.WorkflowState) {
	if t.tracer != nil {
		t.tracer.WorkflowStateUpdated(string(state.Status), len(state.Tasks))
	}
	if t.base != nil {
		t.base.WorkflowStateUpdated(state)
	}
}

// TaskSkipped implements workflow.OutputNotifier.
func (t *TracingOutputNotifierAdapter) TaskSkipped(task *core.Task, reason string) {
	// Trace writer doesn't have a TaskSkipped method, so we just delegate to base
	if t.base != nil {
		t.base.TaskSkipped(task, reason)
	}
}

// Close closes the trace notifier.
func (t *TracingOutputNotifierAdapter) Close() error {
	if t.tracer != nil {
		return t.tracer.Close()
	}
	return nil
}
