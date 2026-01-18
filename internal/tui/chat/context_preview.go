package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ContextFile represents a file in the context
type ContextFile struct {
	Path string
	Size int64 // in bytes
}

// ContextPreviewPanel shows the active context for the current agent
type ContextPreviewPanel struct {
	files        []ContextFile
	directories  []string
	messageCount int
	tokensUsed   int
	tokensMax    int
	currentAgent string
	width        int
	height       int
	visible      bool
}

// NewContextPreviewPanel creates a new context preview panel
func NewContextPreviewPanel() *ContextPreviewPanel {
	return &ContextPreviewPanel{
		files:       make([]ContextFile, 0),
		directories: make([]string, 0),
		visible:     true,
		tokensMax:   100000, // Default max tokens
	}
}

// SetFiles sets the files in context
func (p *ContextPreviewPanel) SetFiles(files []ContextFile) {
	p.files = files
}

// AddFile adds a file to the context
func (p *ContextPreviewPanel) AddFile(path string, size int64) {
	p.files = append(p.files, ContextFile{Path: path, Size: size})
}

// AddDirectory adds a directory to the context
func (p *ContextPreviewPanel) AddDirectory(dir string) {
	p.directories = append(p.directories, dir)
}

// SetMessageCount sets the number of messages in conversation
func (p *ContextPreviewPanel) SetMessageCount(count int) {
	p.messageCount = count
}

// SetTokens sets the token usage
func (p *ContextPreviewPanel) SetTokens(used, maxTokens int) {
	p.tokensUsed = used
	p.tokensMax = maxTokens
}

// SetCurrentAgent sets the current agent name
func (p *ContextPreviewPanel) SetCurrentAgent(agent string) {
	p.currentAgent = agent
}

// Clear resets the context
func (p *ContextPreviewPanel) Clear() {
	p.files = make([]ContextFile, 0)
	p.directories = make([]string, 0)
	p.messageCount = 0
	p.tokensUsed = 0
}

// Toggle toggles panel visibility
func (p *ContextPreviewPanel) Toggle() {
	p.visible = !p.visible
}

// IsVisible returns whether the panel is visible
func (p *ContextPreviewPanel) IsVisible() bool {
	return p.visible
}

// SetSize sets the panel dimensions
func (p *ContextPreviewPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// formatSize formats bytes into human-readable format
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%db", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fkb", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fmb", float64(bytes)/(1024*1024))
}

// formatTokens formats token count
func formatTokens(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	} else if tokens < 1000000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
}

// Render renders the context preview panel
func (p *ContextPreviewPanel) Render() string {
	if !p.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#06b6d4")). // Cyan
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	fileStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e5e7eb"))

	dirStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")) // Amber

	sizeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	// Build content
	var sb strings.Builder

	// Header
	header := headerStyle.Render(" Context")
	if p.currentAgent != "" {
		header += " " + dimStyle.Render("("+p.currentAgent+")")
	}
	sb.WriteString(header)
	sb.WriteString("\n")

	// Separator
	sb.WriteString(dimStyle.Render(strings.Repeat("─", p.width-6)))
	sb.WriteString("\n")

	// Directories
	maxItems := 4 // Max items to show before truncating
	shown := 0

	for _, dir := range p.directories {
		if shown >= maxItems {
			break
		}
		dirName := filepath.Base(dir)
		if dirName == "." || dirName == "" {
			dirName = dir
		}
		line := dirStyle.Render("") + " " + fileStyle.Render(dirName+"/")
		sb.WriteString(line)
		sb.WriteString("\n")
		shown++
	}

	// Files
	for _, file := range p.files {
		if shown >= maxItems {
			break
		}
		fileName := filepath.Base(file.Path)
		sizeStr := sizeStyle.Render("(" + formatSize(file.Size) + ")")
		line := dimStyle.Render("") + " " + fileStyle.Render(fileName) + " " + sizeStr
		sb.WriteString(line)
		sb.WriteString("\n")
		shown++
	}

	// Show more indicator
	total := len(p.directories) + len(p.files)
	if total > maxItems {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  +%d more...", total-maxItems)))
		sb.WriteString("\n")
	}

	// Messages count
	msgIcon := ""
	msgLine := dimStyle.Render(msgIcon) + " " + fileStyle.Render(fmt.Sprintf("%d mensajes", p.messageCount))
	sb.WriteString(msgLine)
	sb.WriteString("\n")

	// Token usage with progress bar
	tokenPct := 0.0
	if p.tokensMax > 0 {
		tokenPct = float64(p.tokensUsed) / float64(p.tokensMax) * 100
	}

	// Token bar color
	var tokenColor lipgloss.Color
	switch {
	case tokenPct >= 90:
		tokenColor = lipgloss.Color("#ef4444") // Red
	case tokenPct >= 70:
		tokenColor = lipgloss.Color("#eab308") // Yellow
	default:
		tokenColor = lipgloss.Color("#22c55e") // Green
	}

	tokenBarStyle := lipgloss.NewStyle().Foreground(tokenColor)
	emptyBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	barWidth := p.width - 20
	if barWidth < 8 {
		barWidth = 8
	}
	filled := int(tokenPct * float64(barWidth) / 100)
	if filled > barWidth {
		filled = barWidth
	}

	tokenBar := tokenBarStyle.Render(strings.Repeat("█", filled)) +
		emptyBarStyle.Render(strings.Repeat("░", barWidth-filled))

	tokenLine := dimStyle.Render("") + " " +
		fileStyle.Render(formatTokens(p.tokensUsed)) +
		dimStyle.Render("/") +
		fileStyle.Render(formatTokens(p.tokensMax))
	sb.WriteString(tokenLine)
	sb.WriteString("\n")
	sb.WriteString(tokenBar)
	sb.WriteString(" ")
	sb.WriteString(tokenBarStyle.Render(fmt.Sprintf("%.1f%%", tokenPct)))

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		Width(p.width - 2)

	return boxStyle.Render(sb.String())
}

// CompactRender renders a one-line summary
func (p *ContextPreviewPanel) CompactRender() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))

	parts := []string{}

	if len(p.files) > 0 {
		parts = append(parts, dimStyle.Render("")+" "+valueStyle.Render(fmt.Sprintf("%d", len(p.files))))
	}

	if p.messageCount > 0 {
		parts = append(parts, dimStyle.Render("")+" "+valueStyle.Render(fmt.Sprintf("%d", p.messageCount)))
	}

	if p.tokensUsed > 0 {
		parts = append(parts, dimStyle.Render("")+" "+valueStyle.Render(formatTokens(p.tokensUsed)))
	}

	return strings.Join(parts, dimStyle.Render(" │ "))
}
