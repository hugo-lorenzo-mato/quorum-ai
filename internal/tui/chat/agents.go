package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AgentStatus represents the current state of an agent
type AgentStatus int

const (
	AgentStatusDisabled AgentStatus = iota
	AgentStatusIdle
	AgentStatusRunning
	AgentStatusDone
	AgentStatusError
)

// AgentInfo holds the display state of an agent
type AgentInfo struct {
	Name      string
	Color     lipgloss.Color
	Status    AgentStatus
	TokensIn  int
	TokensOut int
	Time      string
	Output    string
	Error     string
}

// Default agent colors
var agentColors = map[string]lipgloss.Color{
	"claude":  lipgloss.Color("#a855f7"), // purple
	"gemini":  lipgloss.Color("#3b82f6"), // blue
	"codex":   lipgloss.Color("#22c55e"), // green
	"copilot": lipgloss.Color("#06b6d4"), // cyan
	"llama":   lipgloss.Color("#f97316"), // orange
	"mistral": lipgloss.Color("#ec4899"), // pink
	"gpt":     lipgloss.Color("#10b981"), // emerald
}

// GetAgentColor returns the color for an agent name
func GetAgentColor(name string) lipgloss.Color {
	if color, ok := agentColors[strings.ToLower(name)]; ok {
		return color
	}
	return lipgloss.Color("#71717a") // default gray
}

// Compact bar styles
var (
	agentDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717a"))
	agentSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	agentWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308"))
	agentErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
)

// RenderAgentsCompact renders a compact horizontal bar of agents
// Example: ● Claude (847)  │  ◐ Gemini  │  ○ Codex  │  ○ Copilot
func RenderAgentsCompact(agents []*AgentInfo) string {
	if len(agents) == 0 {
		return agentDimStyle.Render("No agents configured")
	}

	var parts []string

	for _, agent := range agents {
		var icon, name string
		var style lipgloss.Style

		switch agent.Status {
		case AgentStatusDisabled:
			icon = "○"
			style = agentDimStyle
			name = agent.Name
		case AgentStatusIdle:
			icon = "●"
			style = lipgloss.NewStyle().Foreground(agent.Color)
			name = agent.Name
		case AgentStatusRunning:
			icon = "◐"
			style = agentWarnStyle.Bold(true)
			name = agent.Name
		case AgentStatusDone:
			icon = "●"
			style = agentSuccessStyle
			name = agent.Name
			totalTokens := agent.TokensIn + agent.TokensOut
			if totalTokens > 0 {
				name += fmt.Sprintf(" (%d)", totalTokens)
			}
		case AgentStatusError:
			icon = "✗"
			style = agentErrorStyle
			name = agent.Name
		}

		part := style.Render(icon + " " + name)
		parts = append(parts, part)
	}

	return strings.Join(parts, agentDimStyle.Render("  │  "))
}

// RenderPipeline renders a visual pipeline of agent execution
// Example: C ─── G ─── C ─── X    ███░░░░░░░░░░░░ 25%
func RenderPipeline(agents []*AgentInfo) string {
	var s strings.Builder
	s.WriteString(agentDimStyle.Render("Pipeline: "))

	// Only active agents (not disabled)
	var active []*AgentInfo
	for _, a := range agents {
		if a.Status != AgentStatusDisabled {
			active = append(active, a)
		}
	}

	if len(active) == 0 {
		return s.String() + agentDimStyle.Render("No active agents")
	}

	done := 0
	for i, a := range active {
		// Node - first letter of agent name
		var nodeStyle lipgloss.Style
		char := string(a.Name[0])

		switch a.Status {
		case AgentStatusDone:
			nodeStyle = lipgloss.NewStyle().
				Background(a.Color).
				Foreground(lipgloss.Color("#fafafa")).
				Padding(0, 1)
			done++
		case AgentStatusRunning:
			nodeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3f3f46")).
				Foreground(lipgloss.Color("#eab308")).
				Padding(0, 1).
				Bold(true)
		default:
			nodeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3f3f46")).
				Foreground(lipgloss.Color("#71717a")).
				Padding(0, 1)
		}

		s.WriteString(nodeStyle.Render(char))

		// Connector (colored if node is done)
		if i < len(active)-1 {
			conn := agentDimStyle.Render("───")
			if a.Status == AgentStatusDone {
				conn = lipgloss.NewStyle().Foreground(a.Color).Render("───")
			}
			s.WriteString(conn)
		}
	}

	// Progress bar
	pct := 0
	if len(active) > 0 {
		pct = (done * 100) / len(active)
	}

	s.WriteString("   ")

	barWidth := 15
	filled := (barWidth * pct) / 100
	bar := agentSuccessStyle.Render(strings.Repeat("█", filled))
	bar += agentDimStyle.Render(strings.Repeat("░", barWidth-filled))

	pctStyle := agentDimStyle
	if pct == 100 {
		pctStyle = agentSuccessStyle
	}
	s.WriteString(bar + " " + pctStyle.Render(fmt.Sprintf("%d%%", pct)))

	return s.String()
}

// RenderAgentResults renders the results from each agent
func RenderAgentResults(agents []*AgentInfo, width int) string {
	var s strings.Builder

	hasResults := false
	for _, agent := range agents {
		if agent.Status == AgentStatusDone && agent.Output != "" {
			hasResults = true
			// Header: ● Claude 1.2s (847 tok)
			header := lipgloss.NewStyle().
				Foreground(agent.Color).
				Bold(true).
				Render("● " + agent.Name)

			if agent.Time != "" {
				header += " " + agentDimStyle.Render(agent.Time)
			}
			totalTokens := agent.TokensIn + agent.TokensOut
			if totalTokens > 0 {
				header += " " + agentDimStyle.Render(fmt.Sprintf("(%d tok)", totalTokens))
			}
			s.WriteString(header + "\n")

			// Output with colored left border
			outputWidth := width - 12
			if outputWidth < 40 {
				outputWidth = 40
			}
			output := lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(agent.Color).
				PaddingLeft(1).
				MarginLeft(2).
				Width(outputWidth).
				Render(agent.Output)
			s.WriteString(output + "\n\n")
		}
	}

	if !hasResults {
		return ""
	}

	return s.String()
}

// RenderWorkflowLog renders a log box showing workflow progress
func RenderWorkflowLog(agents []*AgentInfo, width int) string {
	var lines []string
	lines = append(lines, "▶ Workflow iniciado")

	for _, agent := range agents {
		switch agent.Status {
		case AgentStatusDone:
			timeStr := ""
			if agent.Time != "" {
				timeStr = " " + agentDimStyle.Render(agent.Time)
			}
			lines = append(lines, agentSuccessStyle.Render("✓")+" "+agent.Name+": completado"+timeStr)
		case AgentStatusRunning:
			lines = append(lines, agentWarnStyle.Render("◐")+" "+agent.Name+": procesando...")
		case AgentStatusError:
			errMsg := "error"
			if agent.Error != "" {
				errMsg = agent.Error
			}
			lines = append(lines, agentErrorStyle.Render("✗")+" "+agent.Name+": "+errMsg)
		}
	}

	logWidth := width - 8
	if logWidth < 30 {
		logWidth = 30
	}

	logBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3f3f46")).
		Padding(0, 1).
		Width(logWidth).
		Render(strings.Join(lines, "\n"))

	return agentDimStyle.Render("Log:") + "\n" + logBox
}

// GetStats calculates statistics from agents
func GetStats(agents []*AgentInfo) (active, total, tokens int, runningAgent string) {
	for _, a := range agents {
		total++
		if a.Status != AgentStatusDisabled {
			active++
		}
		tokens += a.TokensIn + a.TokensOut
		if a.Status == AgentStatusRunning && runningAgent == "" {
			runningAgent = a.Name
		}
	}
	return
}
