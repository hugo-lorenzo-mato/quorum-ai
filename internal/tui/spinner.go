package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerModel represents a spinner component.
type SpinnerModel struct {
	frames []string
	index  int
	style  string
}

// Spinner styles
var spinnerStyles = map[string][]string{
	"dots": {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"line": {"-", "\\", "|", "/"},
	"grow": {"▁", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃"},
}

// NewSpinner creates a new spinner with the default style.
func NewSpinner() SpinnerModel {
	return SpinnerModel{
		frames: spinnerStyles["dots"],
		style:  "dots",
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
	if _, ok := msg.(SpinnerTickMsg); ok {
		s.index = (s.index + 1) % len(s.frames)
		return s, s.Tick()
	}
	return s, nil
}

// View renders the spinner.
func (s SpinnerModel) View() string {
	return RunningStyle.Render(s.frames[s.index])
}
