package chat

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Message colors
var (
	// User message - rose/coral (unique, not used by any CLI)
	userBorderColor = lipgloss.Color("#f43f5e") // Rose

	// Message text color
	msgTextColor = lipgloss.Color("#c9d1d9") // Light text

	// System message
	systemMsgColor = lipgloss.Color("#F59E0B") // Amber

	// Timestamp
	timestampMsgColor = lipgloss.Color("#6b7280") // Gray muted
)

// MessageStyles contains styles for chat message rendering
type MessageStyles struct {
	width int
}

// NewMessageStyles creates message styles for the given width
func NewMessageStyles(width int) *MessageStyles {
	if width < 40 {
		width = 80
	}
	return &MessageStyles{width: width}
}

// FormatUserMessage formats a user message with bubble style (same as bot)
func (s *MessageStyles) FormatUserMessage(content string, timestamp string, isCommand bool) string {
	// Container width: 85% of viewport
	containerWidth := s.width * 85 / 100
	if containerWidth < 40 {
		containerWidth = 40
	}

	// === HEADER: "You 02:36" ===
	headerStyle := lipgloss.NewStyle().
		Foreground(userBorderColor).
		Bold(true)
	tsStyle := lipgloss.NewStyle().
		Foreground(timestampMsgColor)

	header := headerStyle.Render("You")
	if timestamp != "" {
		header += " " + tsStyle.Render(timestamp)
	}

	// === CONTENT ===
	msgContent := content
	if isCommand {
		msgContent = "⚡ " + content
	}

	// === BUBBLE CONTAINER with rounded border ===
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(userBorderColor).
		Foreground(msgTextColor).
		Padding(0, 1).
		Width(containerWidth)

	messageBox := containerStyle.Render(msgContent)

	return header + "\n" + messageBox
}

// FormatBotMessage formats an agent message with bubble style
func (s *MessageStyles) FormatBotMessage(agentName, content, timestamp string, consensus int, tokens string) string {
	// Get agent color from agents.go for consistency with header bar
	agentColor := GetAgentColor(agentName)

	// Container width: 85% of viewport
	containerWidth := s.width * 85 / 100
	if containerWidth < 40 {
		containerWidth = 40
	}

	// === HEADER: "Claude 02:37" ===
	displayName := agentName
	if displayName == "" {
		displayName = "Quorum"
	} else {
		displayName = strings.ToUpper(displayName[:1]) + strings.ToLower(displayName[1:])
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(agentColor).
		Bold(true)
	tsStyle := lipgloss.NewStyle().
		Foreground(timestampMsgColor)

	header := headerStyle.Render(displayName)
	if timestamp != "" {
		header += " " + tsStyle.Render(timestamp)
	}

	// === BUBBLE CONTAINER with rounded border ===
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(agentColor).
		Foreground(msgTextColor).
		Padding(0, 1).
		Width(containerWidth)

	messageBox := containerStyle.Render(content)

	return header + "\n" + messageBox
}

// FormatSystemMessage formats a system message
func (s *MessageStyles) FormatSystemMessage(content string) string {
	style := lipgloss.NewStyle().
		Foreground(systemMsgColor)

	return style.Render("⚙ " + content)
}

// Theme functions

// ApplyDarkThemeMessages sets dark theme colors
func ApplyDarkThemeMessages() {
	userBorderColor = lipgloss.Color("#f43f5e") // Rose
	msgTextColor = lipgloss.Color("#c9d1d9")
	systemMsgColor = lipgloss.Color("#F59E0B")
	timestampMsgColor = lipgloss.Color("#6b7280")
}

// ApplyLightThemeMessages sets light theme colors
func ApplyLightThemeMessages() {
	userBorderColor = lipgloss.Color("#e11d48") // Rose darker for light theme
	msgTextColor = lipgloss.Color("#1f2937")
	systemMsgColor = lipgloss.Color("#d97706")
	timestampMsgColor = lipgloss.Color("#9ca3af")
}
