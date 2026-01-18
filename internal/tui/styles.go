package tui

import "github.com/charmbracelet/lipgloss"

// Base styles
var (
	// HeaderStyle is the style for headers.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBackground).
			Padding(0, 1).
			MarginBottom(1)

	// FooterStyle is the style for footers.
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			MarginTop(1)

	// BoxStyle is the style for containers.
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	// TaskStyle is the style for task items.
	TaskStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2)

	// SelectedTaskStyle is the style for selected task items.
	SelectedTaskStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorHighlight).
				Bold(true).
				PaddingLeft(2)

	// Status styles
	PendingStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	RunningStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	CompletedStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	FailedStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	SkippedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	// Log styles
	LogStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	ErrorLogStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	WarnLogStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	InfoLogStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	DebugLogStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// ErrorStyle is for error display.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(1, 2)

	// Progress bar styles
	ProgressBarFilledStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	// Phase badge styles
	AnalyzeBadge = lipgloss.NewStyle().
			Background(ColorAnalyze).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	PlanBadge = lipgloss.NewStyle().
			Background(ColorPlan).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	ExecuteBadge = lipgloss.NewStyle().
			Background(ColorExecute).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	// HelpStyle is for help text.
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	// TitleStyle is for titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// SubtleStyle is for subtle text.
	SubtleStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// SidebarStyle for the agent sidebar panel.
	SidebarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#13101c")).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3b0764")).
			Padding(1)

	// MainContentStyle for the main content area.
	MainContentStyle = lipgloss.NewStyle().
				Padding(1, 2)

	// ProgressCardStyle for workflow progress display.
	ProgressCardStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1a1528")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#3b0764")).
				Padding(1).
				MarginTop(1)

	// InputBoxStyle for the input area.
	InputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// PromptStyle for user prompts.
	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
)

// AgentOutputStyle returns style for agent output with colored left border.
func AgentOutputStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(color).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)
}

// StatusStyle returns style for a task status.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "pending":
		return PendingStyle
	case "running":
		return RunningStyle
	case "completed":
		return CompletedStyle
	case "failed":
		return FailedStyle
	case "skipped":
		return SkippedStyle
	default:
		return TaskStyle
	}
}

// PhaseBadge returns badge style for a phase.
func PhaseBadge(phase string) lipgloss.Style {
	switch phase {
	case "analyze":
		return AnalyzeBadge
	case "plan":
		return PlanBadge
	case "execute":
		return ExecuteBadge
	default:
		return lipgloss.NewStyle()
	}
}
