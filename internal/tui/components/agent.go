package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// AgentStatus represents the current state of an agent.
type AgentStatus int

const (
	StatusIdle AgentStatus = iota
	StatusWorking
	StatusDone
	StatusError
)

// Agent represents an LLM agent in the UI.
type Agent struct {
	ID       string
	Name     string
	Status   AgentStatus
	Color    lipgloss.Color
	Duration time.Duration
	Output   string
	Error    string
}

// AgentColors maps agent names to their colors.
var AgentColors = map[string]lipgloss.Color{
	"claude":  lipgloss.Color("#7c3aed"), // purple
	"gemini":  lipgloss.Color("#3b82f6"), // blue
	"codex":   lipgloss.Color("#10b981"), // green
	"openai":  lipgloss.Color("#10b981"), // green
	"gpt":     lipgloss.Color("#10b981"), // green
	"default": lipgloss.Color("#6b7280"), // gray
}

// GetAgentColor returns the color for an agent.
func GetAgentColor(name string) lipgloss.Color {
	if color, ok := AgentColors[name]; ok {
		return color
	}
	return AgentColors["default"]
}

// Styles for agent cards
var (
	agentCardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1)

	agentActiveStyle = func(color lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(color).
			Padding(0, 1).
			MarginBottom(1)
	}

	agentNameStyle = func(color lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(color)
	}

	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
)

// RenderAgentCard renders an agent card with status indicator.
func RenderAgentCard(agent *Agent, sp spinner.Model) string {
	var icon string
	var style lipgloss.Style

	switch agent.Status {
	case StatusIdle:
		icon = dimStyle.Render("○")
		style = agentCardStyle
	case StatusWorking:
		icon = sp.View()
		style = agentActiveStyle(agent.Color)
	case StatusDone:
		icon = greenStyle.Render("●")
		style = agentActiveStyle(agent.Color)
	case StatusError:
		icon = errorStyle.Render("✗")
		style = agentActiveStyle(agent.Color)
	}

	nameStyle := agentNameStyle(agent.Color)
	content := icon + " " + nameStyle.Render(agent.Name)

	if agent.Status == StatusWorking {
		content += "\n  " + dimStyle.Render("processing...")
	} else if agent.Status == StatusDone && agent.Duration > 0 {
		content += "\n  " + greenStyle.Render(fmt.Sprintf("✓ %dms", agent.Duration.Milliseconds()))
	} else if agent.Status == StatusError {
		content += "\n  " + errorStyle.Render("error")
	}

	return style.Render(content)
}
