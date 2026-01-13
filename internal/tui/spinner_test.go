package tui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

func TestSpinner_Tick(t *testing.T) {
	spinner := tui.NewSpinner()

	// Get the tick command
	cmd := spinner.Tick()
	if cmd == nil {
		t.Error("Tick() returned nil command")
	}
}

func TestSpinner_Update(t *testing.T) {
	spinner := tui.NewSpinner()

	// Create a tick message
	msg := tui.SpinnerTickMsg(time.Now())

	// Update should handle the tick
	newSpinner, cmd := spinner.Update(msg)
	// newSpinner is a value type, check its view works
	view := newSpinner.View()
	if len(view) == 0 {
		t.Error("Updated spinner View() is empty")
	}
	// cmd should be another tick command
	if cmd == nil {
		t.Error("Update() with tick message should return a new tick command")
	}
}

func TestSpinner_Update_NonTickMessage(t *testing.T) {
	spinner := tui.NewSpinner()

	// Update with a non-tick message
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newSpinner, cmd := spinner.Update(msg)
	// newSpinner is a value type, check its view works
	view := newSpinner.View()
	if len(view) == 0 {
		t.Error("Updated spinner View() is empty")
	}
	if cmd != nil {
		t.Error("Update() with non-tick message should return nil cmd")
	}
}

func TestSpinner_WithStyle_Dots(t *testing.T) {
	spinner := tui.NewSpinner().WithStyle("dots")
	view := spinner.View()
	if len(view) == 0 {
		t.Error("Spinner view with dots style is empty")
	}
}

func TestSpinner_WithStyle_Unknown(t *testing.T) {
	// Unknown style should default to something
	spinner := tui.NewSpinner().WithStyle("unknown")
	view := spinner.View()
	if len(view) == 0 {
		t.Error("Spinner view with unknown style is empty")
	}
}

func TestSpinnerTickMsg(t *testing.T) {
	now := time.Now()
	msg := tui.SpinnerTickMsg(now)
	if time.Time(msg) != now {
		t.Error("SpinnerTickMsg did not preserve time")
	}
}
