package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// OutputStyle returns a style for agent output with colored left border.
func OutputStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(color).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)
}

// RenderAgentOutput renders an agent's output with styled header and content.
func RenderAgentOutput(agent *Agent, width int) string {
	if agent.Output == "" {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(agent.Color).
		Bold(true)

	header := headerStyle.Render(agent.Name)
	if agent.Duration > 0 {
		header += "  " + dimStyle.Render(fmt.Sprintf("%dms", agent.Duration.Milliseconds()))
	}

	content := header + "\n" + agent.Output

	return OutputStyle(agent.Color).Width(width - 6).Render(content)
}
