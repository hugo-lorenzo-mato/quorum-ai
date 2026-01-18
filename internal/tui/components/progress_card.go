package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// ProgressCardConfig holds progress card configuration.
type ProgressCardConfig struct {
	Width       int
	Title       string
	Percentage  float64
	ShowPipeline bool
	Agents      []*Agent
}

var (
	progressCardStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1a1528")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#3b0764")).
				Padding(1).
				MarginTop(1)

	progressTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7c3aed")).
				Bold(true)

	progressPctStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6b7280"))
)

// NewProgressBar creates a new progress bar with gradient.
func NewProgressBar() progress.Model {
	return progress.New(
		progress.WithScaledGradient("#7c3aed", "#3b82f6"),
		progress.WithoutPercentage(),
	)
}

// RenderProgressCard renders the workflow progress card.
func RenderProgressCard(cfg ProgressCardConfig, pb progress.Model) string {
	pct := int(cfg.Percentage * 100)
	header := progressTitleStyle.Render("Workflow: "+cfg.Title) +
		"  " + progressPctStyle.Render(fmt.Sprintf("%d%%", pct))

	content := header + "\n\n"

	// Progress bar
	pb.Width = cfg.Width - 10
	content += pb.ViewAs(cfg.Percentage) + "\n\n"

	// Pipeline visual if enabled
	if cfg.ShowPipeline && len(cfg.Agents) > 0 {
		nodes := BuildPipelineFromAgents(cfg.Agents)
		content += RenderPipeline(nodes)
	}

	return progressCardStyle.Width(cfg.Width - 4).Render(content)
}
