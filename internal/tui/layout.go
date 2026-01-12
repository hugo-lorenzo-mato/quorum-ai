package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layout provides layout utilities.
type Layout struct {
	Width  int
	Height int
}

// NewLayout creates a new layout helper.
func NewLayout(width, height int) *Layout {
	return &Layout{
		Width:  width,
		Height: height,
	}
}

// Center centers content horizontally.
func (l *Layout) Center(content string) string {
	return lipgloss.PlaceHorizontal(l.Width, lipgloss.Center, content)
}

// Right aligns content to the right.
func (l *Layout) Right(content string) string {
	return lipgloss.PlaceHorizontal(l.Width, lipgloss.Right, content)
}

// Columns creates a multi-column layout.
func (l *Layout) Columns(cols ...string) string {
	if len(cols) == 0 {
		return ""
	}

	colWidth := l.Width / len(cols)
	styled := make([]string, len(cols))

	for i, col := range cols {
		styled[i] = lipgloss.NewStyle().
			Width(colWidth).
			Render(col)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, styled...)
}

// Box wraps content in a bordered box.
func (l *Layout) Box(title, content string) string {
	boxWidth := l.Width - 4
	if boxWidth < 20 {
		boxWidth = 20
	}

	style := BoxStyle.Width(boxWidth)

	if title != "" {
		titleLine := TitleStyle.Render(title)
		return style.Render(titleLine + "\n" + content)
	}

	return style.Render(content)
}

// Truncate truncates text to fit width.
func Truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// Wrap wraps text to fit width.
func Wrap(s string, width int) string {
	if width <= 0 {
		return s
	}

	var result strings.Builder
	words := strings.Fields(s)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLen+wordLen+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}

		if i > 0 && lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}

		result.WriteString(word)
		lineLen += wordLen
	}

	return result.String()
}

// Divider creates a horizontal divider.
func Divider(width int, char string) string {
	if char == "" {
		char = "─"
	}
	return SubtleStyle.Render(strings.Repeat(char, width))
}

// Spacer creates vertical space.
func Spacer(lines int) string {
	return strings.Repeat("\n", lines)
}

// Badge creates a small labeled badge.
func Badge(label string, style lipgloss.Style) string {
	return style.Render(label)
}

// Table renders a simple table.
func Table(headers []string, rows [][]string, width int) string {
	if len(headers) == 0 {
		return ""
	}

	colWidth := width / len(headers)
	var result strings.Builder

	// Headers
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = lipgloss.NewStyle().
			Bold(true).
			Width(colWidth).
			Render(h)
	}
	result.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerRow...))
	result.WriteString("\n")
	result.WriteString(Divider(width, "─"))
	result.WriteString("\n")

	// Rows
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			cells[i] = lipgloss.NewStyle().
				Width(colWidth).
				Render(Truncate(val, colWidth-2))
		}
		result.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		result.WriteString("\n")
	}

	return result.String()
}
