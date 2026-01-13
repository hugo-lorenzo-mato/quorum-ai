package tui

import (
	"sync"
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
	// WorkflowCompleted is called when the workflow finishes successfully.
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
	model   Model
	program *tea.Program
	updateC chan tea.Msg
	mu      sync.Mutex
}

// NewTUIOutput creates a new TUI output handler.
func NewTUIOutput() *TUIOutput {
	t := &TUIOutput{
		model:   New(),
		updateC: make(chan tea.Msg, 100),
	}
	return t
}

// Start starts the TUI program (should be called in a goroutine).
func (t *TUIOutput) Start() error {
	// Create program with the update channel
	t.program = tea.NewProgram(t.model, tea.WithAltScreen())

	// Start a goroutine to send updates to the program
	go func() {
		for msg := range t.updateC {
			if t.program != nil {
				t.program.Send(msg)
			}
		}
	}()

	_, err := t.program.Run()
	return err
}

// WorkflowStarted implements Output.
func (t *TUIOutput) WorkflowStarted(_ string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- LogMsg{Time: time.Now(), Level: "info", Message: "Workflow started"}:
		default:
		}
	}
}

// PhaseStarted implements Output.
func (t *TUIOutput) PhaseStarted(phase core.Phase) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- LogMsg{Time: time.Now(), Level: "info", Message: "Phase: " + string(phase)}:
		default:
		}
	}
}

// TaskStarted implements Output.
func (t *TUIOutput) TaskStarted(task *core.Task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusRunning}:
		default:
		}
	}
}

// TaskCompleted implements Output.
func (t *TUIOutput) TaskCompleted(task *core.Task, _ time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusCompleted}:
		default:
		}
	}
}

// TaskFailed implements Output.
func (t *TUIOutput) TaskFailed(task *core.Task, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		select {
		case t.updateC <- TaskUpdateMsg{TaskID: task.ID, Status: core.TaskStatusFailed, Error: errStr}:
		default:
		}
	}
}

// WorkflowCompleted implements Output.
func (t *TUIOutput) WorkflowCompleted(state *core.WorkflowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- WorkflowUpdateMsg{State: state}:
		default:
		}
	}
}

// WorkflowFailed implements Output.
func (t *TUIOutput) WorkflowFailed(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- ErrorMsg{Error: err}:
		default:
		}
	}
}

// Log implements Output.
func (t *TUIOutput) Log(level, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		select {
		case t.updateC <- LogMsg{Time: time.Now(), Level: level, Message: message}:
		default:
		}
	}
}

// Close implements Output.
func (t *TUIOutput) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.updateC != nil {
		close(t.updateC)
		t.updateC = nil
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
	if t.updateC != nil {
		select {
		case t.updateC <- WorkflowUpdateMsg{State: state}:
		default:
		}
	}
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
func (a *OutputNotifierAdapter) WorkflowStateUpdated(state *core.WorkflowState) {
	// Use WorkflowCompleted to send the full state - it works for intermediate updates too
	a.output.WorkflowCompleted(state)
}
