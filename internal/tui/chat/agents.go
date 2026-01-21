package chat

import (
	"fmt"
	"strings"
	"time"

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

	// Real-time activity tracking
	CurrentActivity string        // Current activity description (e.g., "read_file config.go")
	ActivityIcon    string        // Icon for current activity (e.g., "🔧", "💭")
	StartedAt       time.Time     // When agent started running
	MaxTimeout      time.Duration // Maximum timeout for this phase
	Phase           string        // Current workflow phase (e.g., "analyze", "critique")
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
		if agent.Status != AgentStatusDone || agent.Output == "" {
			continue
		}
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

// Progress bar characters
const (
	progressFilled = "▓"
	progressEmpty  = "░"
	progressWidth  = 10
)

// estimateProgress calculates estimated progress (0-100) based on elapsed time.
// Assumes typical agent execution takes ~2 minutes.
func estimateProgress(startedAt time.Time, status AgentStatus) int {
	if status == AgentStatusDone {
		return 100
	}
	if status != AgentStatusRunning || startedAt.IsZero() {
		return 0
	}

	elapsed := time.Since(startedAt)
	// Assume 2 minutes = 100%, but cap at 95% while still running
	expectedDuration := 2 * time.Minute
	pct := int((elapsed.Seconds() / expectedDuration.Seconds()) * 100)
	if pct > 95 {
		pct = 95 // Never show 100% while still running
	}
	return pct
}

// formatElapsed formats elapsed time compactly
func formatElapsed(startedAt time.Time, maxTimeout time.Duration) string {
	if startedAt.IsZero() {
		return ""
	}
	elapsed := time.Since(startedAt)

	// Format elapsed time
	var elapsedStr string
	if elapsed < time.Minute {
		elapsedStr = fmt.Sprintf("%ds", int(elapsed.Seconds()))
	} else {
		elapsedStr = fmt.Sprintf("%dm%02ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	}

	// Add max timeout if available
	if maxTimeout > 0 {
		var maxStr string
		if maxTimeout < time.Minute {
			maxStr = fmt.Sprintf("%ds", int(maxTimeout.Seconds()))
		} else if maxTimeout < time.Hour {
			maxStr = fmt.Sprintf("%dm", int(maxTimeout.Minutes()))
		} else {
			maxStr = fmt.Sprintf("%dh", int(maxTimeout.Hours()))
		}
		return elapsedStr + "/" + maxStr
	}

	return elapsedStr
}

// RenderAgentProgressBars renders progress bars for all agents with current activity.
// Example output:
//
//	claude  [▓▓▓▓▓▓▓▓░░] 🔧 read_file config.go     45s
//	gemini  [▓▓▓▓░░░░░░] 💭 thinking...             32s
//	codex   [▓▓░░░░░░░░] 🔧 glob **/*.go            28s
//	copilot [░░░░░░░░░░] ○ queued
func RenderAgentProgressBars(agents []*AgentInfo, width int) string {
	if len(agents) == 0 {
		return agentDimStyle.Render("No agents configured")
	}

	// Calculate max name length for alignment
	maxNameLen := 0
	for _, a := range agents {
		if len(a.Name) > maxNameLen {
			maxNameLen = len(a.Name)
		}
	}
	if maxNameLen < 7 {
		maxNameLen = 7 // minimum for "copilot"
	}

	var lines []string

	for _, agent := range agents {
		var line strings.Builder

		// Agent name (left-aligned, padded)
		nameStyle := lipgloss.NewStyle().Foreground(agent.Color).Bold(agent.Status == AgentStatusRunning)
		name := agent.Name
		if len(name) < maxNameLen {
			name += strings.Repeat(" ", maxNameLen-len(name))
		}
		line.WriteString(nameStyle.Render(name))
		line.WriteString(" ")

		// Progress bar
		pct := estimateProgress(agent.StartedAt, agent.Status)
		filled := (progressWidth * pct) / 100
		if filled > progressWidth {
			filled = progressWidth
		}

		var barStyle lipgloss.Style
		switch agent.Status {
		case AgentStatusDone:
			barStyle = agentSuccessStyle
		case AgentStatusRunning:
			barStyle = lipgloss.NewStyle().Foreground(agent.Color)
		case AgentStatusError:
			barStyle = agentErrorStyle
		default:
			barStyle = agentDimStyle
		}

		bar := "[" + barStyle.Render(strings.Repeat(progressFilled, filled)) +
			agentDimStyle.Render(strings.Repeat(progressEmpty, progressWidth-filled)) + "]"
		line.WriteString(bar)
		line.WriteString(" ")

		// Activity icon and description
		// Fixed width for activity section to ensure time column alignment
		// Format: "icon desc" where icon is ~2 chars wide (emoji)
		activityWidth := width - maxNameLen - progressWidth - 20 // room for time column (e.g., "5m42s/1h")
		if activityWidth < 30 {
			activityWidth = 30
		}

		var activity string
		var activityLen int // track visible length for padding
		switch agent.Status {
		case AgentStatusDisabled:
			activity = agentDimStyle.Render("○ disabled")
			activityLen = 10
		case AgentStatusIdle:
			activity = agentDimStyle.Render("○ idle")
			activityLen = 6
		case AgentStatusRunning:
			icon := agent.ActivityIcon
			if icon == "" {
				icon = "◐"
			}
			desc := agent.CurrentActivity
			if desc == "" {
				desc = "processing..."
			}
			// Icon takes ~2 visual chars + space = 3. Leave room for desc.
			maxDescLen := activityWidth - 3
			if maxDescLen < 10 {
				maxDescLen = 10
			}
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen-3] + "..."
			}
			activity = agentWarnStyle.Render(icon) + " " + desc
			activityLen = 2 + 1 + len(desc) // icon(2) + space(1) + desc
		case AgentStatusDone:
			tokens := agent.TokensIn + agent.TokensOut
			if tokens > 0 {
				tokStr := fmt.Sprintf("done (%d tok)", tokens)
				activity = agentSuccessStyle.Render("✓") + " " + agentDimStyle.Render(tokStr)
				activityLen = 2 + 1 + len(tokStr)
			} else {
				activity = agentSuccessStyle.Render("✓ done")
				activityLen = 6
			}
		case AgentStatusError:
			errMsg := agent.Error
			if errMsg == "" {
				errMsg = "failed"
			}
			maxErrLen := activityWidth - 3
			if maxErrLen < 10 {
				maxErrLen = 10
			}
			if len(errMsg) > maxErrLen {
				errMsg = errMsg[:maxErrLen-3] + "..."
			}
			activity = agentErrorStyle.Render("✗ " + errMsg)
			activityLen = 2 + len(errMsg)
		}

		// Pad activity to exactly activityWidth for consistent time alignment
		if activityLen < activityWidth {
			activity += strings.Repeat(" ", activityWidth-activityLen)
		}
		line.WriteString(activity)

		// Elapsed time (right-aligned)
		if agent.Status == AgentStatusRunning && !agent.StartedAt.IsZero() {
			elapsed := formatElapsed(agent.StartedAt, agent.MaxTimeout)
			line.WriteString(" ")
			line.WriteString(agentDimStyle.Render(elapsed))
		} else if agent.Time != "" {
			line.WriteString(" ")
			line.WriteString(agentDimStyle.Render(agent.Time))
		}

		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n")
}

// UpdateAgentActivity updates an agent's current activity.
// Returns true if the agent was found and updated.
func UpdateAgentActivity(agents []*AgentInfo, name, icon, activity string) bool {
	for _, a := range agents {
		if strings.EqualFold(a.Name, name) {
			a.ActivityIcon = icon
			a.CurrentActivity = activity
			return true
		}
	}
	return false
}

// StartAgent marks an agent as running and records the start time.
func StartAgent(agents []*AgentInfo, name, phase string, maxTimeout time.Duration) bool {
	for _, a := range agents {
		if !strings.EqualFold(a.Name, name) {
			continue
		}
		a.Status = AgentStatusRunning
		a.StartedAt = time.Now()
		a.MaxTimeout = maxTimeout
		a.Phase = phase
		a.CurrentActivity = "working..."
		a.ActivityIcon = "◐"
		return true
	}
	return false
}

// CompleteAgent marks an agent as done and records elapsed time.
func CompleteAgent(agents []*AgentInfo, name string, tokensIn, tokensOut int) bool {
	for _, a := range agents {
		if !strings.EqualFold(a.Name, name) {
			continue
		}
		a.Status = AgentStatusDone
		a.TokensIn += tokensIn
		a.TokensOut += tokensOut
		if !a.StartedAt.IsZero() {
			a.Time = formatElapsed(a.StartedAt, a.MaxTimeout)
		}
		a.CurrentActivity = ""
		a.ActivityIcon = ""
		return true
	}
	return false
}

// FailAgent marks an agent as failed with an error message.
func FailAgent(agents []*AgentInfo, name, errMsg string) bool {
	for _, a := range agents {
		if !strings.EqualFold(a.Name, name) {
			continue
		}
		a.Status = AgentStatusError
		a.Error = errMsg
		if !a.StartedAt.IsZero() {
			a.Time = formatElapsed(a.StartedAt, a.MaxTimeout)
		}
		a.CurrentActivity = ""
		a.ActivityIcon = ""
		return true
	}
	return false
}
