package chat

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ShortcutCategory represents a category of shortcuts
type ShortcutCategory struct {
	Name      string
	Shortcuts []Shortcut
}

// Shortcut represents a single keyboard shortcut
type Shortcut struct {
	Key         string
	Description string
}

// ShortcutsOverlay displays the keyboard shortcuts cheatsheet
type ShortcutsOverlay struct {
	categories []ShortcutCategory
	width      int
	height     int
	visible    bool
}

// NewShortcutsOverlay creates a new shortcuts overlay
func NewShortcutsOverlay() *ShortcutsOverlay {
	return &ShortcutsOverlay{
		categories: []ShortcutCategory{
			{
				Name: "Navigation",
				Shortcuts: []Shortcut{
					{Key: "Tab", Description: "Next agent / Complete"},
					{Key: "Shift+Tab", Description: "Previous agent"},
					{Key: "↑/↓", Description: "Scroll / Navigate"},
					{Key: "PgUp/PgDn", Description: "Page scroll"},
					{Key: "Home/End", Description: "Top / Bottom"},
				},
			},
			{
				Name: "Panels",
				Shortcuts: []Shortcut{
					{Key: "Ctrl+E", Description: "Toggle explorer"},
					{Key: "Ctrl+L", Description: "Toggle logs"},
					{Key: "Ctrl+K", Description: "Toggle consensus"},
				},
			},
			{
				Name: "Actions",
				Shortcuts: []Shortcut{
					{Key: "Enter", Description: "Send message"},
					{Key: "Esc", Description: "Cancel / Close"},
					{Key: "Ctrl+C", Description: "Quit"},
					{Key: "Ctrl+Y", Description: "Copy response"},
				},
			},
			{
				Name: "Tools",
				Shortcuts: []Shortcut{
					{Key: "Ctrl+D", Description: "Agent diff view"},
					{Key: "Ctrl+R", Description: "Search history"},
					{Key: "/", Description: "Commands"},
					{Key: "!", Description: "Shell command"},
					{Key: "?/F1", Description: "This help"},
				},
			},
		},
		visible: false,
	}
}

// Toggle toggles visibility
func (s *ShortcutsOverlay) Toggle() {
	s.visible = !s.visible
}

// Show shows the overlay
func (s *ShortcutsOverlay) Show() {
	s.visible = true
}

// Hide hides the overlay
func (s *ShortcutsOverlay) Hide() {
	s.visible = false
}

// IsVisible returns visibility
func (s *ShortcutsOverlay) IsVisible() bool {
	return s.visible
}

// SetSize sets the overlay dimensions
func (s *ShortcutsOverlay) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// Render renders the shortcuts overlay
func (s *ShortcutsOverlay) Render() string {
	if !s.visible {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")). // Purple
		Bold(true).
		Padding(0, 1)

	categoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#06b6d4")). // Cyan
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22d3ee")). // Cyan color for keys
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	// Calculate column width (two columns of categories)
	colWidth := (s.width - 10) / 2
	if colWidth < 30 {
		colWidth = 30
	}

	// The box has Width(s.width-4) and Padding(1,2), so inner width is s.width-4-4 = s.width-8
	innerWidth := s.width - 8
	if innerWidth < 0 {
		innerWidth = 0
	}

	var sb strings.Builder

	// Title
	title := titleStyle.Render(" Shortcuts")
	titlePadding := (innerWidth - lipgloss.Width(title)) / 2
	if titlePadding < 0 {
		titlePadding = 0
	}
	sb.WriteString(strings.Repeat(" ", titlePadding))
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// Render categories in two columns
	leftCol := &strings.Builder{}
	rightCol := &strings.Builder{}

	for i, cat := range s.categories {
		col := leftCol
		if i >= 2 {
			col = rightCol
		}

		// Category header
		col.WriteString(categoryStyle.Render(cat.Name))
		col.WriteString("\n")
		col.WriteString(dimStyle.Render(strings.Repeat("─", 20)))
		col.WriteString("\n")

		// Shortcuts
		for _, shortcut := range cat.Shortcuts {
			key := keyStyle.Render(shortcut.Key)
			desc := descStyle.Render(shortcut.Description)

			// Pad key to align descriptions
			keyWidth := lipgloss.Width(key)
			padding := 12 - keyWidth
			if padding < 1 {
				padding = 1
			}

			col.WriteString(key)
			col.WriteString(strings.Repeat(" ", padding))
			col.WriteString("→ ")
			col.WriteString(desc)
			col.WriteString("\n")
		}
		col.WriteString("\n")
	}

	// Join columns side by side
	leftLines := strings.Split(leftCol.String(), "\n")
	rightLines := strings.Split(rightCol.String(), "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	for i := 0; i < maxLines; i++ {
		left := ""
		right := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// Pad left column
		leftWidth := lipgloss.Width(left)
		if leftWidth < colWidth {
			left += strings.Repeat(" ", colWidth-leftWidth)
		}

		sb.WriteString(left)
		sb.WriteString("  ")
		sb.WriteString(right)
		sb.WriteString("\n")
	}

	// Footer
	footer := footerStyle.Render("Press any key to close")
	sb.WriteString("\n")
	footerPadding := (innerWidth - lipgloss.Width(footer)) / 2
	if footerPadding < 0 {
		footerPadding = 0
	}
	sb.WriteString(strings.Repeat(" ", footerPadding))
	sb.WriteString(footer)

	// Box style with double border for overlay feel
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2).
		Width(s.width - 4).
		Align(lipgloss.Left)

	return boxStyle.Render(sb.String())
}

// RenderCentered renders the overlay centered in the given dimensions
func (s *ShortcutsOverlay) RenderCentered(screenWidth, screenHeight int) string {
	if !s.visible {
		return ""
	}

	content := s.Render()
	contentWidth := lipgloss.Width(content)
	contentHeight := lipgloss.Height(content)

	// Calculate padding to center
	padLeft := (screenWidth - contentWidth) / 2
	padTop := (screenHeight - contentHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	// Build centered output
	var sb strings.Builder

	// Top padding
	for i := 0; i < padTop; i++ {
		sb.WriteString(strings.Repeat(" ", screenWidth))
		sb.WriteString("\n")
	}

	// Content with left padding
	lines := strings.Split(content, "\n")
	leftPadding := strings.Repeat(" ", padLeft)
	for _, line := range lines {
		sb.WriteString(leftPadding)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}
