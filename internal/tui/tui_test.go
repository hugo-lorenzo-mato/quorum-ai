package tui_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

func TestModel_Init(t *testing.T) {
	m := tui.New()
	cmd := m.Init()
	testutil.AssertTrue(t, cmd != nil, "Init should return a command")
}

func TestModel_View_NotReady(t *testing.T) {
	m := tui.New()
	view := m.View()
	testutil.AssertContains(t, view, "Initializing")
}

func TestStatusStyle(t *testing.T) {
	tests := []string{"pending", "running", "completed", "failed", "skipped", "unknown"}
	for _, status := range tests {
		style := tui.StatusStyle(status)
		// Just verify it doesn't panic and returns something
		_ = style.Render("test")
	}
}

func TestPhaseBadge(t *testing.T) {
	phases := []string{"analyze", "plan", "execute", "unknown"}
	for _, phase := range phases {
		badge := tui.PhaseBadge(phase)
		_ = badge.Render("test")
	}
}

func TestLayout_Center(t *testing.T) {
	layout := tui.NewLayout(80, 24)
	centered := layout.Center("test")
	testutil.AssertTrue(t, len(centered) > 0, "centered text should not be empty")
}

func TestLayout_Columns(t *testing.T) {
	layout := tui.NewLayout(80, 24)
	cols := layout.Columns("col1", "col2", "col3")
	testutil.AssertContains(t, cols, "col1")
	testutil.AssertContains(t, cols, "col2")
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"ab", 2, "ab"},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		result := tui.Truncate(tt.input, tt.width)
		testutil.AssertEqual(t, result, tt.expected)
	}
}

func TestWrap(t *testing.T) {
	result := tui.Wrap("this is a test string for wrapping", 10)
	testutil.AssertContains(t, result, "\n")
}

func TestDivider(t *testing.T) {
	div := tui.Divider(10, "-")
	testutil.AssertEqual(t, len(div) > 0, true)
}

func TestTable(t *testing.T) {
	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
	}

	table := tui.Table(headers, rows, 40)
	testutil.AssertContains(t, table, "Name")
	testutil.AssertContains(t, table, "Value")
}

func TestOutputMode_String(t *testing.T) {
	tests := []struct {
		mode     tui.OutputMode
		expected string
	}{
		{tui.ModeTUI, "tui"},
		{tui.ModePlain, "plain"},
		{tui.ModeJSON, "json"},
		{tui.ModeQuiet, "quiet"},
	}

	for _, tt := range tests {
		testutil.AssertEqual(t, tt.mode.String(), tt.expected)
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		input    string
		expected tui.OutputMode
	}{
		{"tui", tui.ModeTUI},
		{"plain", tui.ModePlain},
		{"json", tui.ModeJSON},
		{"quiet", tui.ModeQuiet},
		{"unknown", tui.ModeTUI},
	}

	for _, tt := range tests {
		result := tui.ParseOutputMode(tt.input)
		testutil.AssertEqual(t, result, tt.expected)
	}
}

func TestDetector_ForceMode(t *testing.T) {
	d := tui.NewDetector().ForceMode(tui.ModeJSON)
	mode := d.Detect()
	testutil.AssertEqual(t, mode, tui.ModeJSON)
}

func TestFallbackOutput_WorkflowStarted(t *testing.T) {
	var buf bytes.Buffer
	output := tui.NewFallbackOutput(false, true).WithWriter(&buf)

	output.WorkflowStarted("test prompt")

	testutil.AssertContains(t, buf.String(), "Workflow Started")
}

func TestFallbackOutput_TaskCompleted(t *testing.T) {
	var buf bytes.Buffer
	output := tui.NewFallbackOutput(false, true).WithWriter(&buf)

	task := &core.Task{
		ID:        "test-1",
		Name:      "Test Task",
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.01,
	}

	output.TaskCompleted(task, 500*time.Millisecond)

	testutil.AssertContains(t, buf.String(), "DONE")
	testutil.AssertContains(t, buf.String(), "Test Task")
}

func TestFallbackOutput_Progress(t *testing.T) {
	var buf bytes.Buffer
	output := tui.NewFallbackOutput(false, false).WithWriter(&buf)

	output.Progress(5, 10, "processing")

	testutil.AssertContains(t, buf.String(), "50%")
}

func TestJSONOutput_WorkflowStarted(t *testing.T) {
	var buf bytes.Buffer
	output := tui.NewJSONOutput().WithWriter(&buf)

	output.WorkflowStarted("test prompt")

	testutil.AssertContains(t, buf.String(), "workflow_started")
	testutil.AssertContains(t, buf.String(), "test prompt")
}

func TestSpinner_View(t *testing.T) {
	spinner := tui.NewSpinner()
	view := spinner.View()
	testutil.AssertTrue(t, len(view) > 0, "spinner view should not be empty")
}

func TestSpinner_WithStyle(t *testing.T) {
	spinner := tui.NewSpinner().WithStyle("line")
	view := spinner.View()
	testutil.AssertTrue(t, len(view) > 0, "spinner view should not be empty")
}
