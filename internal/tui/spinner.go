package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerModel represents a spinner component.
type SpinnerModel struct {
	frames         []string
	index          int
	style          string
	bubblesSpinner spinner.Model // For use with components
}

// Spinner styles
var spinnerStyles = map[string][]string{
	"dots": {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"line": {"-", "\\", "|", "/"},
	"grow": {"▁", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃"},
}

// NewSpinner creates a new spinner with the default style.
func NewSpinner() SpinnerModel {
	// Create bubbles spinner for component use
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b"))

	return SpinnerModel{
		frames:         spinnerStyles["dots"],
		style:          "dots",
		bubblesSpinner: sp,
	}
}

// WithStyle sets the spinner style.
func (s SpinnerModel) WithStyle(style string) SpinnerModel {
	if frames, ok := spinnerStyles[style]; ok {
		s.frames = frames
		s.style = style
	}
	return s
}

// Tick returns a command that ticks the spinner.
func (s SpinnerModel) Tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return SpinnerTickMsg(t)
	})
}

// Update updates the spinner state.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Update custom spinner
	if _, ok := msg.(SpinnerTickMsg); ok {
		s.index = (s.index + 1) % len(s.frames)
		cmds = append(cmds, s.Tick())
	}

	// Update bubbles spinner
	if _, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		s.bubblesSpinner, cmd = s.bubblesSpinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	if len(cmds) > 0 {
		return s, tea.Batch(cmds...)
	}
	return s, nil
}

// BubblesSpinner returns the bubbles spinner for component use.
func (s SpinnerModel) BubblesSpinner() spinner.Model {
	return s.bubblesSpinner
}

// View renders the spinner.
func (s SpinnerModel) View() string {
	return RunningStyle.Render(s.frames[s.index])
}
