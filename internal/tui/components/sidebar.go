package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// SidebarConfig holds sidebar configuration.
type SidebarConfig struct {
	Width     int
	Height    int
	ShowStats bool
	TotalCost float64
	TotalReqs int
}

// DefaultSidebarConfig returns default sidebar configuration.
func DefaultSidebarConfig() SidebarConfig {
	return SidebarConfig{
		Width:     24,
		Height:    20,
		ShowStats: true,
	}
}

var (
	sidebarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#13101c")).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3b0764")).
			Padding(1)

	sidebarTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6b7280")).
				Bold(true)

	sidebarDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	statsLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6b7280"))

	statsValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f59e0b"))
)

// RenderSidebar renders the agent sidebar panel.
func RenderSidebar(agents []*Agent, sp spinner.Model, cfg SidebarConfig) string {
	var sb strings.Builder

	sb.WriteString(sidebarTitleStyle.Render("AGENTS") + "\n\n")

	for _, agent := range agents {
		sb.WriteString(RenderAgentCard(agent, sp))
		sb.WriteString("\n")
	}

	if cfg.ShowStats {
		divider := sidebarDividerStyle.Render(strings.Repeat("─", cfg.Width-4))
		sb.WriteString("\n" + divider + "\n")
		sb.WriteString(statsLabelStyle.Render("Session: ") + fmt.Sprintf("%d req\n", cfg.TotalReqs))
		sb.WriteString(statsLabelStyle.Render("Cost:    ") + statsValueStyle.Render(fmt.Sprintf("$%.3f", cfg.TotalCost)) + "\n")
	}

	return sidebarStyle.
		Width(cfg.Width).
		Height(cfg.Height).
		Render(sb.String())
}
