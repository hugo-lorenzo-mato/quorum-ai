package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestNewFallbackOutput(t *testing.T) {
	f := NewFallbackOutput(true, true)

	if f == nil {
		t.Fatal("NewFallbackOutput returned nil")
	}
	if !f.useColor {
		t.Error("useColor should be true")
	}
	if !f.verbose {
		t.Error("verbose should be true")
	}
}

func TestFallbackOutput_WithWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	if f.writer != buf {
		t.Error("WithWriter did not set writer")
	}
}

func TestFallbackOutput_WorkflowStarted(t *testing.T) {
	tests := []struct {
		name     string
		useColor bool
		verbose  bool
		prompt   string
		want     []string
	}{
		{
			name:     "basic",
			useColor: false,
			verbose:  false,
			prompt:   "Test prompt",
			want:     []string{"Workflow Started"},
		},
		{
			name:     "verbose",
			useColor: false,
			verbose:  true,
			prompt:   "Test prompt",
			want:     []string{"Workflow Started", "Prompt:", "Test prompt"},
		},
		{
			name:     "long prompt truncation",
			useColor: false,
			verbose:  true,
			prompt:   strings.Repeat("a", 150),
			want:     []string{"...", "Prompt:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			f := NewFallbackOutput(tt.useColor, tt.verbose).WithWriter(buf)

			f.WorkflowStarted(tt.prompt)

			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got %q", want, output)
				}
			}
		})
	}
}

func TestFallbackOutput_PhaseStarted(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	f.PhaseStarted(core.PhaseAnalyze)

	output := buf.String()
	if !strings.Contains(output, "ANALYZE") {
		t.Errorf("output should contain phase name, got %q", output)
	}
}

func TestFallbackOutput_TaskStarted(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	task := &core.Task{ID: "task-1", Name: "Test Task"}
	f.TaskStarted(task)

	output := buf.String()
	if !strings.Contains(output, "RUNNING") {
		t.Errorf("output should contain RUNNING, got %q", output)
	}
	if !strings.Contains(output, "Test Task") {
		t.Errorf("output should contain task name, got %q", output)
	}
}

func TestFallbackOutput_TaskCompleted(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		task    *core.Task
		want    []string
	}{
		{
			name:    "basic",
			verbose: false,
			task:    &core.Task{ID: "task-1", Name: "Test Task"},
			want:    []string{"DONE", "Test Task"},
		},
		{
			name:    "verbose with tokens",
			verbose: true,
			task:    &core.Task{ID: "task-1", Name: "Test Task", TokensIn: 100, TokensOut: 50},
			want:    []string{"DONE", "Test Task", "Tokens:", "100", "50"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			f := NewFallbackOutput(false, tt.verbose).WithWriter(buf)

			f.TaskCompleted(tt.task, 1*time.Second)

			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got %q", want, output)
				}
			}
		})
	}
}

func TestFallbackOutput_TaskFailed(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	task := &core.Task{ID: "task-1", Name: "Test Task"}
	err := errors.New("test error")
	f.TaskFailed(task, err)

	output := buf.String()
	if !strings.Contains(output, "FAILED") {
		t.Errorf("output should contain FAILED, got %q", output)
	}
	if !strings.Contains(output, "test error") {
		t.Errorf("output should contain error, got %q", output)
	}
}

func TestFallbackOutput_TaskSkipped(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	task := &core.Task{ID: "task-1", Name: "Test Task"}
	f.TaskSkipped(task, "dependency failed")

	output := buf.String()
	if !strings.Contains(output, "SKIPPED") {
		t.Errorf("output should contain SKIPPED, got %q", output)
	}
	if !strings.Contains(output, "dependency failed") {
		t.Errorf("output should contain reason, got %q", output)
	}
}

func TestFallbackOutput_WorkflowCompleted(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {Status: core.TaskStatusCompleted},
				"task-2": {Status: core.TaskStatusCompleted},
			},
			Metrics: &core.StateMetrics{},
		},
	}

	f.WorkflowCompleted(state)

	output := buf.String()
	if !strings.Contains(output, "Workflow Completed") {
		t.Errorf("output should contain 'Workflow Completed', got %q", output)
	}
	if !strings.Contains(output, "Tasks:") {
		t.Errorf("output should contain task count, got %q", output)
	}
}

func TestFallbackOutput_WorkflowFailed(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	err := errors.New("workflow error")
	f.WorkflowFailed(err)

	output := buf.String()
	if !strings.Contains(output, "Workflow Failed") {
		t.Errorf("output should contain 'Workflow Failed', got %q", output)
	}
	if !strings.Contains(output, "workflow error") {
		t.Errorf("output should contain error, got %q", output)
	}
}

func TestFallbackOutput_Log(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		verbose bool
		want    bool
	}{
		{"info verbose", "info", true, true},
		{"info not verbose", "info", false, true},
		{"debug verbose", "debug", true, true},
		{"debug not verbose", "debug", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			f := NewFallbackOutput(false, tt.verbose).WithWriter(buf)

			f.Log(tt.level, "test message")

			output := buf.String()
			if tt.want && !strings.Contains(output, "test message") {
				t.Errorf("output should contain message, got %q", output)
			}
			if !tt.want && strings.Contains(output, "test message") {
				t.Errorf("output should not contain message, got %q", output)
			}
		})
	}
}

func TestFallbackOutput_Progress(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	f.Progress(5, 10, "Loading...")

	output := buf.String()
	if !strings.Contains(output, "50%") {
		t.Errorf("output should contain percentage, got %q", output)
	}
	if !strings.Contains(output, "Loading...") {
		t.Errorf("output should contain message, got %q", output)
	}
}

func TestFallbackOutput_ProgressComplete(t *testing.T) {
	buf := &bytes.Buffer{}
	f := NewFallbackOutput(false, false).WithWriter(buf)

	f.Progress(10, 10, "Done")

	output := buf.String()
	if !strings.Contains(output, "100%") {
		t.Errorf("output should contain 100%%, got %q", output)
	}
	if !strings.Contains(output, "\n") {
		t.Errorf("output should end with newline when complete, got %q", output)
	}
}

func TestFallbackOutput_StatusIcons(t *testing.T) {
	f := NewFallbackOutput(false, false)

	icons := []string{"pending", "running", "completed", "failed", "skipped"}
	for _, status := range icons {
		icon := f.statusIcon(status)
		if icon == "" {
			t.Errorf("statusIcon(%q) returned empty string", status)
		}
	}
}

func TestFallbackOutput_Colorize(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		color     string
		wantCodes bool
	}{
		{"valid color red", "test", "red", true},
		{"valid color green", "test", "green", true},
		{"valid color blue", "test", "blue", true},
		{"unknown color", "test", "unknown", false},
		{"empty color", "test", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFallbackOutput(true, false)
			result := f.colorize(tt.text, tt.color)

			if tt.wantCodes {
				if !strings.Contains(result, "\033[") {
					t.Errorf("colorize should add color codes for color %q", tt.color)
				}
			} else {
				if strings.Contains(result, "\033[") {
					t.Errorf("colorize should not add color codes for color %q", tt.color)
				}
			}

			// Always should contain the original text
			if !strings.Contains(result, tt.text) {
				t.Errorf("colorize result should contain original text %q", tt.text)
			}
		})
	}
}

func TestFallbackOutput_ProgressBar(t *testing.T) {
	f := NewFallbackOutput(false, false)

	bar := f.progressBar(50.0, 10)

	if !strings.Contains(bar, "[") || !strings.Contains(bar, "]") {
		t.Errorf("progress bar should have brackets, got %q", bar)
	}
}

func TestFallbackOutput_CountCompleted(t *testing.T) {
	f := NewFallbackOutput(false, false)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {Status: core.TaskStatusCompleted},
				"task-2": {Status: core.TaskStatusCompleted},
				"task-3": {Status: core.TaskStatusPending},
				"task-4": {Status: core.TaskStatusFailed},
			},
		},
	}

	count := f.countCompleted(state)
	if count != 2 {
		t.Errorf("countCompleted() = %d, want 2", count)
	}
}

func TestNewJSONOutput(t *testing.T) {
	j := NewJSONOutput()

	if j == nil {
		t.Fatal("NewJSONOutput returned nil")
	}
	if j.enc == nil {
		t.Error("encoder should be initialized")
	}
}

func TestJSONOutput_WithWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	if j.writer != buf {
		t.Error("WithWriter did not set writer")
	}
}

func TestJSONOutput_WorkflowStarted(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	j.WorkflowStarted("test prompt")

	output := buf.String()
	if !strings.Contains(output, "workflow_started") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "test prompt") {
		t.Errorf("output should contain prompt, got %q", output)
	}
}

func TestJSONOutput_PhaseStarted(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	j.PhaseStarted(core.PhaseAnalyze)

	output := buf.String()
	if !strings.Contains(output, "phase_started") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "analyze") {
		t.Errorf("output should contain phase, got %q", output)
	}
}

func TestJSONOutput_TaskCompleted(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	task := &core.Task{ID: "task-1", Name: "Test Task", TokensIn: 100, TokensOut: 50}
	j.TaskCompleted(task, 1*time.Second)

	output := buf.String()
	if !strings.Contains(output, "task_completed") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "task-1") {
		t.Errorf("output should contain task id, got %q", output)
	}
}

func TestJSONOutput_TaskFailed(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	task := &core.Task{ID: "task-1", Name: "Test Task"}
	err := errors.New("test error")
	j.TaskFailed(task, err)

	output := buf.String()
	if !strings.Contains(output, "task_failed") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "test error") {
		t.Errorf("output should contain error, got %q", output)
	}
}

func TestJSONOutput_WorkflowCompleted(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {Status: core.TaskStatusCompleted},
			},
			Metrics: &core.StateMetrics{},
		},
	}
	j.WorkflowCompleted(state)

	output := buf.String()
	if !strings.Contains(output, "workflow_completed") {
		t.Errorf("output should contain event type, got %q", output)
	}
}

func TestJSONOutput_WorkflowFailed(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	err := errors.New("workflow error")
	j.WorkflowFailed(err)

	output := buf.String()
	if !strings.Contains(output, "workflow_failed") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "workflow error") {
		t.Errorf("output should contain error, got %q", output)
	}
}

func TestJSONOutput_Log(t *testing.T) {
	buf := &bytes.Buffer{}
	j := NewJSONOutput().WithWriter(buf)

	j.Log("info", "test message")

	output := buf.String()
	if !strings.Contains(output, "log") {
		t.Errorf("output should contain event type, got %q", output)
	}
	if !strings.Contains(output, "info") {
		t.Errorf("output should contain level, got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain message, got %q", output)
	}
}

func TestJSONEvent_Fields(t *testing.T) {
	event := JSONEvent{
		Type:      "test_event",
		Timestamp: time.Now(),
		Data:      map[string]string{"key": "value"},
	}

	if event.Type != "test_event" {
		t.Errorf("Type = %q, want %q", event.Type, "test_event")
	}
	if event.Data == nil {
		t.Error("Data should not be nil")
	}
}
