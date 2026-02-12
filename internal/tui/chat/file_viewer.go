package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxFileSize  = 1024 * 1024 // 1MB max
	maxLineCount = 10000       // 10K lines max
)

// FileViewer displays file contents in an overlay
type FileViewer struct {
	filePath  string
	fileName  string
	content   string
	lines     []string
	lineCount int
	fileSize  int64
	isBinary  bool
	error     string

	viewport         viewport.Model
	width            int
	height           int
	visible          bool
	ready            bool
	horizontalOffset int // horizontal scroll offset
	maxLineWidth     int // max width of any line (for scroll limits)
}

// NewFileViewer creates a new file viewer
func NewFileViewer() *FileViewer {
	return &FileViewer{
		visible: false,
	}
}

// SetFile loads a file for viewing
func (f *FileViewer) SetFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		f.error = fmt.Sprintf("Cannot resolve path: %v", err)
		return err
	}
	if root, err := os.Getwd(); err == nil {
		rel, relErr := filepath.Rel(root, absPath)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			f.error = "Cannot open file outside project root"
			return fmt.Errorf("path outside project root")
		}
	}

	f.filePath = absPath
	f.fileName = filepath.Base(absPath)
	f.error = ""
	f.isBinary = false
	f.content = ""
	f.lines = nil
	f.horizontalOffset = 0
	f.maxLineWidth = 0

	// Check file info
	info, err := os.Stat(absPath)
	if err != nil {
		f.error = fmt.Sprintf("Cannot access file: %v", err)
		return err
	}

	if info.IsDir() {
		f.error = "Cannot view directory"
		return fmt.Errorf("is a directory")
	}

	f.fileSize = info.Size()

	// Check file size
	if f.fileSize > maxFileSize {
		f.error = fmt.Sprintf("File too large: %s (max %s)", formatSize(f.fileSize), formatSize(maxFileSize))
		return fmt.Errorf("file too large")
	}

	// Read file
	// #nosec G304 -- absPath is validated to be within the project root
	data, err := os.ReadFile(absPath)
	if err != nil {
		f.error = fmt.Sprintf("Cannot read file: %v", err)
		return err
	}

	// Check if binary
	if isBinaryContent(data) {
		f.isBinary = true
		f.error = "Binary file - cannot display"
		return nil
	}

	// Convert to string and split into lines
	f.content = string(data)
	f.lines = strings.Split(f.content, "\n")
	f.lineCount = len(f.lines)

	// Limit line count
	if f.lineCount > maxLineCount {
		f.lines = f.lines[:maxLineCount]
		f.lineCount = maxLineCount
		f.error = fmt.Sprintf("Showing first %d lines (file has more)", maxLineCount)
	}

	f.updateViewport()
	return nil
}

// Toggle toggles visibility
func (f *FileViewer) Toggle() {
	f.visible = !f.visible
}

// Show makes the viewer visible
func (f *FileViewer) Show() {
	f.visible = true
}

// Hide hides the viewer
func (f *FileViewer) Hide() {
	f.visible = false
}

// IsVisible returns whether the viewer is visible
func (f *FileViewer) IsVisible() bool {
	return f.visible
}

// GetFilePath returns the current file path
func (f *FileViewer) GetFilePath() string {
	return f.filePath
}

// SetSize sets the viewer dimensions
func (f *FileViewer) SetSize(width, height int) {
	f.width = width
	f.height = height

	contentHeight := height - 6 // Header + footer + borders + padding
	if contentHeight < 3 {
		contentHeight = 3
	}

	contentWidth := width - 4 // Borders + padding
	if contentWidth < 20 {
		contentWidth = 20
	}

	if !f.ready {
		f.viewport = viewport.New(contentWidth, contentHeight)
		f.ready = true
	} else {
		f.viewport.Width = contentWidth
		f.viewport.Height = contentHeight
	}

	f.updateViewport()
}

// ScrollUp scrolls up
func (f *FileViewer) ScrollUp() {
	f.viewport.ScrollUp(1)
}

// ScrollDown scrolls down
func (f *FileViewer) ScrollDown() {
	f.viewport.ScrollDown(1)
}

// PageUp scrolls up a page
func (f *FileViewer) PageUp() {
	f.viewport.HalfPageUp()
}

// PageDown scrolls down a page
func (f *FileViewer) PageDown() {
	f.viewport.HalfPageDown()
}

// ScrollTop goes to top
func (f *FileViewer) ScrollTop() {
	f.viewport.GotoTop()
}

// ScrollBottom goes to bottom
func (f *FileViewer) ScrollBottom() {
	f.viewport.GotoBottom()
}

// ScrollLeft scrolls left horizontally
func (f *FileViewer) ScrollLeft() {
	if f.horizontalOffset > 0 {
		f.horizontalOffset -= 4
		if f.horizontalOffset < 0 {
			f.horizontalOffset = 0
		}
		f.updateViewport()
	}
}

// ScrollRight scrolls right horizontally
func (f *FileViewer) ScrollRight() {
	// Calculate max scroll based on content width vs viewport width
	maxScroll := f.maxLineWidth - (f.viewport.Width - 10) // Leave some margin
	if maxScroll < 0 {
		maxScroll = 0
	}
	if f.horizontalOffset < maxScroll {
		f.horizontalOffset += 4
		f.updateViewport()
	}
}

// ScrollHome goes to beginning of line (horizontal)
func (f *FileViewer) ScrollHome() {
	f.horizontalOffset = 0
	f.updateViewport()
}

// ScrollEnd goes to end of longest line (horizontal)
func (f *FileViewer) ScrollEnd() {
	maxScroll := f.maxLineWidth - (f.viewport.Width - 10)
	if maxScroll > 0 {
		f.horizontalOffset = maxScroll
		f.updateViewport()
	}
}

// updateViewport updates the viewport content
func (f *FileViewer) updateViewport() {
	if !f.ready || len(f.lines) == 0 {
		return
	}

	ext := strings.ToLower(filepath.Ext(f.fileName))
	var sb strings.Builder

	// Get syntax color for file type
	syntaxColor := getSyntaxColor(ext)

	// Render lines with line numbers
	lineNumWidth := len(fmt.Sprintf("%d", f.lineCount))
	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	codeStyle := lipgloss.NewStyle().Foreground(syntaxColor)

	// Calculate display width for content (after line number)
	displayWidth := f.viewport.Width - lineNumWidth - 4
	if displayWidth < 10 {
		displayWidth = 10
	}

	// Track max line width for horizontal scroll limits
	f.maxLineWidth = 0

	for i, line := range f.lines {
		// Replace tabs with spaces for consistent width calculation
		expandedLine := strings.ReplaceAll(line, "\t", "    ")
		lineWidth := lipgloss.Width(expandedLine)
		if lineWidth > f.maxLineWidth {
			f.maxLineWidth = lineWidth
		}

		lineNum := fmt.Sprintf("%*d", lineNumWidth, i+1)
		sb.WriteString(lineNumStyle.Render(lineNum))
		sb.WriteString(" │ ")

		// Apply horizontal offset and get visible portion
		displayLine := getVisiblePortion(expandedLine, f.horizontalOffset, displayWidth)
		sb.WriteString(codeStyle.Render(displayLine))

		if i < len(f.lines)-1 {
			sb.WriteString("\n")
		}
	}

	f.viewport.SetContent(sb.String())
}

// getVisiblePortion extracts the visible portion of a line based on horizontal offset
func getVisiblePortion(line string, offset, width int) string {
	if offset == 0 && lipgloss.Width(line) <= width {
		return line
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(line)

	// Skip characters until we reach the offset
	currentWidth := 0
	startIdx := 0
	for i, r := range runes {
		if currentWidth >= offset {
			startIdx = i
			break
		}
		currentWidth += runeWidth(r)
		startIdx = i + 1
	}

	// If offset is beyond line length, return empty
	if startIdx >= len(runes) {
		return ""
	}

	// Build visible portion up to width
	var result strings.Builder
	visibleWidth := 0
	for i := startIdx; i < len(runes); i++ {
		r := runes[i]
		rw := runeWidth(r)
		if visibleWidth+rw > width {
			break
		}
		result.WriteRune(r)
		visibleWidth += rw
	}

	// Add scroll indicator if there's more content
	visibleStr := result.String()
	if offset > 0 {
		// Show left indicator
		visibleStr = "◀" + visibleStr[min(1, len(visibleStr)):]
	}
	if startIdx+len([]rune(visibleStr)) < len(runes) {
		// Show right indicator
		if visibleStr != "" {
			visibleRunes := []rune(visibleStr)
			visibleStr = string(visibleRunes[:len(visibleRunes)-1]) + "▶"
		}
	}

	return visibleStr
}

// runeWidth returns the display width of a rune
func runeWidth(_ rune) int {
	// Simple approximation - most characters are width 1
	// Wide characters (CJK, emoji) would need proper handling
	return 1
}

// Render renders the file viewer
func (f *FileViewer) Render() string {
	if !f.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1).
		Width(f.width - 2).
		Height(f.height - 2)

	var sb strings.Builder

	// Header with file icon and name
	icon := getFileIcon(f.fileName)
	header := headerStyle.Render(icon + " " + f.fileName)

	// File info
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	info := infoStyle.Render(fmt.Sprintf(" (%s, %d lines)", formatSize(f.fileSize), f.lineCount))

	sb.WriteString(header)
	sb.WriteString(info)
	sb.WriteString("\n")

	// Divider
	divider := dimStyle.Render(strings.Repeat("─", f.width-6))
	sb.WriteString(divider)
	sb.WriteString("\n")

	// Content or error
	if f.error != "" && (f.isBinary || len(f.lines) == 0) {
		// Show error centered
		errorMsg := errorStyle.Render(f.error)
		sb.WriteString("\n")
		sb.WriteString(errorMsg)
		sb.WriteString("\n")
	} else {
		// Show file content
		sb.WriteString(f.viewport.View())

		// Show truncation warning if applicable
		if f.error != "" {
			sb.WriteString("\n")
			sb.WriteString(dimStyle.Render(f.error))
		}
	}

	sb.WriteString("\n")

	// Footer with scroll info and help
	scrollPercent := f.viewport.ScrollPercent() * 100
	scrollInfo := dimStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent))
	helpText := dimStyle.Render("↑↓←→ scroll • g/G top/bottom • 0/$ start/end • e edit • q close")

	// Calculate gap
	footerWidth := f.width - 6
	gap := footerWidth - lipgloss.Width(scrollInfo) - lipgloss.Width(helpText)
	if gap < 1 {
		gap = 1
	}

	sb.WriteString(scrollInfo)
	sb.WriteString(strings.Repeat(" ", gap))
	sb.WriteString(helpText)

	return boxStyle.Render(sb.String())
}

// getSyntaxColor returns a color based on file extension
func getSyntaxColor(ext string) lipgloss.Color {
	switch ext {
	case ".go":
		return lipgloss.Color("#00add8") // Go cyan
	case ".js", ".jsx":
		return lipgloss.Color("#f7df1e") // JS yellow
	case ".ts", ".tsx":
		return lipgloss.Color("#3178c6") // TS blue
	case ".py":
		return lipgloss.Color("#3776ab") // Python blue
	case ".rs":
		return lipgloss.Color("#dea584") // Rust orange
	case ".rb":
		return lipgloss.Color("#cc342d") // Ruby red
	case ".java":
		return lipgloss.Color("#b07219") // Java orange
	case ".c", ".h":
		return lipgloss.Color("#555555") // C gray
	case ".cpp", ".hpp", ".cc":
		return lipgloss.Color("#f34b7d") // C++ pink
	case ".md":
		return lipgloss.Color("#083fa1") // Markdown blue
	case ".json":
		return lipgloss.Color("#a855f7") // JSON purple
	case ".yaml", ".yml":
		return lipgloss.Color("#cb171e") // YAML red
	case ".toml":
		return lipgloss.Color("#9c4221") // TOML brown
	case ".sh", ".bash":
		return lipgloss.Color("#22c55e") // Shell green
	case ".sql":
		return lipgloss.Color("#e38c00") // SQL orange
	case ".html", ".htm":
		return lipgloss.Color("#e34c26") // HTML orange
	case ".css":
		return lipgloss.Color("#563d7c") // CSS purple
	case ".xml":
		return lipgloss.Color("#0060ac") // XML blue
	default:
		return lipgloss.Color("#e5e7eb") // Default light gray
	}
}

// getFileIcon returns an icon based on filename
func getFileIcon(_ string) string {
	// Icons are intentionally disabled for now (TUI font/width portability).
	// Keep this as a single return to avoid a dead conditional tree.
	return ""
}

// isBinaryContent checks if content appears to be binary
func isBinaryContent(data []byte) bool {
	// Check for null bytes or high ratio of non-printable characters
	if len(data) == 0 {
		return false
	}

	// Sample first 8KB
	sample := data
	if len(sample) > 8192 {
		sample = data[:8192]
	}

	// Check if valid UTF-8
	if !utf8.Valid(sample) {
		return true
	}

	// Count non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range sample {
		if b == 0 {
			return true // Null byte = definitely binary
		}
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If more than 10% non-printable, consider binary
	return float64(nonPrintable)/float64(len(sample)) > 0.1
}
