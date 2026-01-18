package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// HistoryEntry represents a command in history
type HistoryEntry struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	Agent     string    `json:"agent,omitempty"`
}

// HistorySearch provides fuzzy search over command history
type HistorySearch struct {
	entries       []HistoryEntry
	filtered      []HistoryEntry
	input         textinput.Model
	selectedIndex int
	visible       bool
	width         int
	height        int
	maxEntries    int
	historyFile   string
}

// NewHistorySearch creates a new history search component
func NewHistorySearch() *HistorySearch {
	ti := textinput.New()
	ti.Placeholder = "Search history..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	// Default history file location
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".quorum", "history.json")

	hs := &HistorySearch{
		entries:       make([]HistoryEntry, 0),
		filtered:      make([]HistoryEntry, 0),
		input:         ti,
		selectedIndex: 0,
		visible:       false,
		maxEntries:    1000,
		historyFile:   historyFile,
	}

	// Load existing history
	hs.Load()

	return hs
}

// SetHistoryFile sets the path to the history file
func (h *HistorySearch) SetHistoryFile(path string) {
	h.historyFile = path
	h.Load()
}

// Add adds a command to history
func (h *HistorySearch) Add(command string, agent string) {
	// Don't add empty or duplicate consecutive commands
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1].Command == command {
		return
	}

	entry := HistoryEntry{
		Command:   command,
		Timestamp: time.Now(),
		Agent:     agent,
	}

	h.entries = append(h.entries, entry)

	// Trim to max entries
	if len(h.entries) > h.maxEntries {
		h.entries = h.entries[len(h.entries)-h.maxEntries:]
	}

	// Persist
	h.Save()
}

// Load loads history from file
func (h *HistorySearch) Load() error {
	data, err := os.ReadFile(h.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history yet
		}
		return err
	}

	return json.Unmarshal(data, &h.entries)
}

// Save saves history to file
func (h *HistorySearch) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(h.historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.historyFile, data, 0644)
}

// Show shows the history search
func (h *HistorySearch) Show() {
	h.visible = true
	h.input.Reset()
	h.input.Focus()
	h.selectedIndex = 0
	h.filter("")
}

// Hide hides the history search
func (h *HistorySearch) Hide() {
	h.visible = false
}

// Toggle toggles visibility
func (h *HistorySearch) Toggle() {
	if h.visible {
		h.Hide()
	} else {
		h.Show()
	}
}

// IsVisible returns whether the search is visible
func (h *HistorySearch) IsVisible() bool {
	return h.visible
}

// SetSize sets the component dimensions
func (h *HistorySearch) SetSize(width, height int) {
	h.width = width
	h.height = height
	h.input.Width = width - 10
}

// UpdateInput updates the text input and filters
func (h *HistorySearch) UpdateInput(msg interface{}) {
	var cmd interface{}
	h.input, cmd = h.input.Update(msg)
	_ = cmd // We don't need the command

	h.filter(h.input.Value())
}

// filter filters history based on query
func (h *HistorySearch) filter(query string) {
	if query == "" {
		// Show all entries in reverse order (most recent first)
		h.filtered = make([]HistoryEntry, len(h.entries))
		for i, entry := range h.entries {
			h.filtered[len(h.entries)-1-i] = entry
		}
		return
	}

	// Get all commands for fuzzy matching
	commands := make([]string, len(h.entries))
	for i, entry := range h.entries {
		commands[i] = entry.Command
	}

	// Fuzzy match
	matches := fuzzy.Find(query, commands)

	h.filtered = make([]HistoryEntry, len(matches))
	for i, match := range matches {
		h.filtered[i] = h.entries[match.Index]
	}

	// Reset selection
	h.selectedIndex = 0
}

// MoveUp moves selection up
func (h *HistorySearch) MoveUp() {
	if h.selectedIndex > 0 {
		h.selectedIndex--
	}
}

// MoveDown moves selection down
func (h *HistorySearch) MoveDown() {
	if h.selectedIndex < len(h.filtered)-1 {
		h.selectedIndex++
	}
}

// GetSelected returns the selected command
func (h *HistorySearch) GetSelected() string {
	if h.selectedIndex >= 0 && h.selectedIndex < len(h.filtered) {
		return h.filtered[h.selectedIndex].Command
	}
	return ""
}

// Count returns total history entries
func (h *HistorySearch) Count() int {
	return len(h.entries)
}

// FilteredCount returns number of filtered entries
func (h *HistorySearch) FilteredCount() int {
	return len(h.filtered)
}

// Render renders the history search overlay
func (h *HistorySearch) Render() string {
	if !h.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true).
		Padding(0, 1)

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e")).
		Bold(true)

	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#eab308")).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1)

	var sb strings.Builder

	// Header with count
	header := headerStyle.Render(" History")
	countStr := dimStyle.Render(" (" + string('0'+rune(len(h.filtered)%10)) + "/" +
		string('0'+rune(len(h.entries)%10)) + ")")
	if len(h.filtered) >= 10 || len(h.entries) >= 10 {
		countStr = dimStyle.Render(" (" +
			strings.TrimLeft(strings.Replace(string(rune('0'+len(h.filtered)/10))+string(rune('0'+len(h.filtered)%10)), "\x00", "", -1), "0") + "/" +
			strings.TrimLeft(strings.Replace(string(rune('0'+len(h.entries)/10))+string(rune('0'+len(h.entries)%10)), "\x00", "", -1), "0") + ")")
	}
	// Simple count display
	countStr = dimStyle.Render(" (" + itoa(len(h.filtered)) + "/" + itoa(len(h.entries)) + ")")
	sb.WriteString(header + countStr)
	sb.WriteString("\n")

	// Input field
	sb.WriteString(promptStyle.Render("> "))
	sb.WriteString(h.input.View())
	sb.WriteString("\n")

	// Separator
	sb.WriteString(dimStyle.Render(strings.Repeat("─", h.width-6)))
	sb.WriteString("\n")

	// Results
	maxShow := 8
	if h.height > 0 {
		maxShow = h.height - 8
		if maxShow < 3 {
			maxShow = 3
		}
	}

	// Calculate visible window
	start := 0
	if h.selectedIndex >= maxShow {
		start = h.selectedIndex - maxShow + 1
	}
	end := start + maxShow
	if end > len(h.filtered) {
		end = len(h.filtered)
	}

	// Show scroll indicator at top
	if start > 0 {
		sb.WriteString(dimStyle.Render("  ↑ " + itoa(start) + " more above"))
		sb.WriteString("\n")
	}

	query := h.input.Value()
	for i := start; i < end; i++ {
		entry := h.filtered[i]
		cmd := entry.Command

		// Truncate if too long
		maxLen := h.width - 10
		if maxLen < 20 {
			maxLen = 20
		}
		if len(cmd) > maxLen {
			cmd = cmd[:maxLen-3] + "..."
		}

		// Highlight matching characters if there's a query
		if query != "" && i == h.selectedIndex {
			// Simple highlight - just show selected
			sb.WriteString("  ")
			sb.WriteString(selectedStyle.Render("❯ " + cmd))
		} else if i == h.selectedIndex {
			sb.WriteString("  ")
			sb.WriteString(selectedStyle.Render("❯ " + cmd))
		} else {
			// Highlight matching characters
			highlighted := highlightMatches(cmd, query, matchStyle, itemStyle)
			sb.WriteString("    ")
			sb.WriteString(highlighted)
		}
		sb.WriteString("\n")
	}

	// Show scroll indicator at bottom
	if end < len(h.filtered) {
		sb.WriteString(dimStyle.Render("  ↓ " + itoa(len(h.filtered)-end) + " more below"))
		sb.WriteString("\n")
	}

	// Empty state
	if len(h.filtered) == 0 {
		sb.WriteString(dimStyle.Render("  No matching commands"))
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("\n")
	footer := keyStyle.Render("↑↓") + dimStyle.Render(" navigate") +
		"  " + keyStyle.Render("Enter") + dimStyle.Render(" select") +
		"  " + keyStyle.Render("Esc") + dimStyle.Render(" close")
	sb.WriteString(footer)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		BorderBackground(lipgloss.Color("#1f1f23")).
		Background(lipgloss.Color("#1f1f23")).
		Padding(0, 1).
		Width(h.width - 2)

	return boxStyle.Render(sb.String())
}

// itoa converts int to string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// highlightMatches highlights matching characters in a string
func highlightMatches(text, query string, matchStyle, normalStyle lipgloss.Style) string {
	if query == "" {
		return normalStyle.Render(text)
	}

	queryLower := strings.ToLower(query)
	textLower := strings.ToLower(text)

	var result strings.Builder
	queryIdx := 0

	for i, r := range text {
		if queryIdx < len(queryLower) && strings.ToLower(string(r)) == string(queryLower[queryIdx]) {
			result.WriteString(matchStyle.Render(string(r)))
			queryIdx++
		} else {
			// Check if character matches anywhere in remaining query
			matched := false
			for j := queryIdx; j < len(queryLower); j++ {
				if i < len(textLower) && textLower[i] == queryLower[j] {
					result.WriteString(matchStyle.Render(string(r)))
					matched = true
					break
				}
			}
			if !matched {
				result.WriteString(normalStyle.Render(string(r)))
			}
		}
	}

	return result.String()
}
