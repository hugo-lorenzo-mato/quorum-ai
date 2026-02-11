package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Test helper: creates a HistorySearch with a temp file and a properly
// initialised textinput.Model.  Note: we do NOT call Focus() because
// the cursor blink channel needs a running tea.Program.
// ---------------------------------------------------------------------------

func newTestHistorySearch(t *testing.T) *HistorySearch {
	t.Helper()
	ti := textinput.New()
	ti.Placeholder = "Search history..."
	ti.CharLimit = 256
	ti.Width = 40

	return &HistorySearch{
		entries:       make([]HistoryEntry, 0),
		filtered:      make([]HistoryEntry, 0),
		input:         ti,
		selectedIndex: 0,
		visible:       false,
		maxEntries:    1000,
		historyFile:   filepath.Join(t.TempDir(), "history.json"),
	}
}

// showTestHistory simulates Show() without calling input.Focus() which
// panics outside a tea.Program.
func showTestHistory(h *HistorySearch) {
	h.visible = true
	h.input.Reset()
	// Skip h.input.Focus() -- it panics without a tea.Program
	h.selectedIndex = 0
	h.filter("")
}

// ---------------------------------------------------------------------------
// NewHistorySearch
// ---------------------------------------------------------------------------

func TestNewHistorySearch(t *testing.T) {
	// Use a temp dir to avoid loading real history
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	hs := NewHistorySearch()
	if hs == nil {
		t.Fatal("NewHistorySearch returned nil")
	}
	if hs.visible {
		t.Error("new history search should not be visible")
	}
	if hs.maxEntries != 1000 {
		t.Errorf("expected maxEntries=1000, got %d", hs.maxEntries)
	}
}

// ---------------------------------------------------------------------------
// Toggle / Show / Hide / IsVisible
// ---------------------------------------------------------------------------

func TestHistorySearch_ToggleShowHide(t *testing.T) {
	hs := newTestHistorySearch(t)

	if hs.IsVisible() {
		t.Error("should be hidden initially")
	}

	// Use our safe Show wrapper
	showTestHistory(hs)
	if !hs.IsVisible() {
		t.Error("should be visible after Show")
	}

	hs.Hide()
	if hs.IsVisible() {
		t.Error("should be hidden after Hide")
	}

	// Toggle from hidden -> calls Show internally, use wrapper
	hs.visible = false
	showTestHistory(hs)
	if !hs.IsVisible() {
		t.Error("should be visible after toggle from hidden")
	}

	// Toggle from visible -> calls Hide
	hs.Toggle()
	if hs.IsVisible() {
		t.Error("should be hidden after Toggle from visible")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestHistorySearch_SetSize(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 30)
	if hs.width != 80 || hs.height != 30 {
		t.Errorf("unexpected size %d x %d", hs.width, hs.height)
	}
	// Also verify input.Width was updated
	if hs.input.Width != 70 { // width - 10
		t.Errorf("expected input width 70, got %d", hs.input.Width)
	}
}

// ---------------------------------------------------------------------------
// Add - basic
// ---------------------------------------------------------------------------

func TestHistorySearch_Add(t *testing.T) {
	hs := newTestHistorySearch(t)

	hs.Add("hello world", "claude")
	if hs.Count() != 1 {
		t.Errorf("expected 1 entry, got %d", hs.Count())
	}

	// Verify entry
	if hs.entries[0].Command != "hello world" {
		t.Errorf("unexpected command: %q", hs.entries[0].Command)
	}
	if hs.entries[0].Agent != "claude" {
		t.Errorf("unexpected agent: %q", hs.entries[0].Agent)
	}
}

func TestHistorySearch_Add_Empty(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("", "claude")
	if hs.Count() != 0 {
		t.Error("empty command should not be added")
	}
}

func TestHistorySearch_Add_Whitespace(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("   ", "claude")
	if hs.Count() != 0 {
		t.Error("whitespace-only command should not be added")
	}
}

func TestHistorySearch_Add_DuplicateConsecutive(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("test", "claude")
	hs.Add("test", "claude")
	if hs.Count() != 1 {
		t.Errorf("duplicate consecutive should not be added, got %d", hs.Count())
	}
}

func TestHistorySearch_Add_DuplicateNonConsecutive(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("first", "claude")
	hs.Add("second", "claude")
	hs.Add("first", "claude")
	if hs.Count() != 3 {
		t.Errorf("non-consecutive duplicate should be added, got %d", hs.Count())
	}
}

func TestHistorySearch_Add_MaxEntries(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.maxEntries = 5

	for i := 0; i < 10; i++ {
		hs.Add(strings.Repeat("x", i+1), "claude")
	}
	if hs.Count() != 5 {
		t.Errorf("expected 5 entries after trimming, got %d", hs.Count())
	}
	// First entry should be the 6th added (trimmed oldest)
	if hs.entries[0].Command != "xxxxxx" {
		t.Errorf("unexpected first entry after trim: %q", hs.entries[0].Command)
	}
}

// ---------------------------------------------------------------------------
// Load / Save / SetHistoryFile
// ---------------------------------------------------------------------------

func TestHistorySearch_SaveAndLoad(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("command one", "claude")
	hs.Add("command two", "gemini")

	if err := hs.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(hs.historyFile)
	if err != nil {
		t.Fatalf("cannot read history file: %v", err)
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("cannot parse history file: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in file, got %d", len(entries))
	}

	// Load into a new instance
	hs2 := newTestHistorySearch(t)
	hs2.historyFile = hs.historyFile
	if err := hs2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if hs2.Count() != 2 {
		t.Errorf("expected 2 entries after load, got %d", hs2.Count())
	}
}

func TestHistorySearch_Load_NoFile(t *testing.T) {
	hs := newTestHistorySearch(t)
	// File does not exist yet
	err := hs.Load()
	if err != nil {
		t.Errorf("Load should not error for nonexistent file, got: %v", err)
	}
}

func TestHistorySearch_SetHistoryFile(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("test", "claude")
	_ = hs.Save()

	newPath := filepath.Join(t.TempDir(), "other_history.json")
	// Write some data there
	entries := []HistoryEntry{{Command: "from other", Timestamp: time.Now()}}
	data, _ := json.Marshal(entries)
	_ = os.WriteFile(newPath, data, 0o600)

	hs.SetHistoryFile(newPath)
	if hs.historyFile != newPath {
		t.Error("historyFile not updated")
	}
	// Should have loaded the new file
	if hs.Count() != 1 {
		t.Errorf("expected 1 entry after SetHistoryFile, got %d", hs.Count())
	}
}

// ---------------------------------------------------------------------------
// Show (resets state) - using safe wrapper
// ---------------------------------------------------------------------------

func TestHistorySearch_Show_ResetsState(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("cmd1", "claude")
	hs.Add("cmd2", "claude")

	hs.selectedIndex = 5
	showTestHistory(hs)

	if !hs.IsVisible() {
		t.Error("should be visible after Show")
	}
	if hs.selectedIndex != 0 {
		t.Error("selectedIndex should be reset to 0")
	}
	if hs.FilteredCount() != 2 {
		t.Errorf("expected 2 filtered entries, got %d", hs.FilteredCount())
	}
}

// ---------------------------------------------------------------------------
// filter
// ---------------------------------------------------------------------------

func TestHistorySearch_Filter_Empty(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("alpha", "claude")
	hs.Add("beta", "claude")
	hs.Add("gamma", "claude")

	hs.filter("")
	// All entries in reverse order
	if hs.FilteredCount() != 3 {
		t.Errorf("expected 3, got %d", hs.FilteredCount())
	}
	// Most recent first
	if hs.filtered[0].Command != "gamma" {
		t.Errorf("expected 'gamma' first, got %q", hs.filtered[0].Command)
	}
}

func TestHistorySearch_Filter_WithQuery(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("hello world", "claude")
	hs.Add("goodbye world", "claude")
	hs.Add("hello earth", "claude")

	hs.filter("hello")
	if hs.FilteredCount() < 1 {
		t.Error("expected at least 1 match for 'hello'")
	}
}

func TestHistorySearch_Filter_NoMatch(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("hello", "claude")

	hs.filter("zzzzzzz")
	if hs.FilteredCount() != 0 {
		t.Errorf("expected 0 matches, got %d", hs.FilteredCount())
	}
}

// ---------------------------------------------------------------------------
// MoveUp / MoveDown / GetSelected
// ---------------------------------------------------------------------------

func TestHistorySearch_Navigation(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("first", "claude")
	hs.Add("second", "claude")
	hs.Add("third", "claude")
	hs.filter("")

	// Initially at index 0
	if hs.GetSelected() != "third" { // reverse order
		t.Errorf("expected 'third', got %q", hs.GetSelected())
	}

	hs.MoveDown()
	if hs.selectedIndex != 1 {
		t.Errorf("expected index 1, got %d", hs.selectedIndex)
	}
	if hs.GetSelected() != "second" {
		t.Errorf("expected 'second', got %q", hs.GetSelected())
	}

	hs.MoveDown()
	if hs.GetSelected() != "first" {
		t.Errorf("expected 'first', got %q", hs.GetSelected())
	}

	// Can't go past end
	hs.MoveDown()
	if hs.selectedIndex != 2 {
		t.Errorf("should stay at 2, got %d", hs.selectedIndex)
	}

	hs.MoveUp()
	if hs.selectedIndex != 1 {
		t.Errorf("expected index 1, got %d", hs.selectedIndex)
	}

	// Can't go past start
	hs.MoveUp()
	hs.MoveUp()
	if hs.selectedIndex != 0 {
		t.Errorf("should stay at 0, got %d", hs.selectedIndex)
	}
}

func TestHistorySearch_GetSelected_Empty(t *testing.T) {
	hs := newTestHistorySearch(t)
	if hs.GetSelected() != "" {
		t.Error("GetSelected should return empty string with no entries")
	}
}

func TestHistorySearch_GetSelected_OutOfBounds(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.selectedIndex = 10
	if hs.GetSelected() != "" {
		t.Error("GetSelected should return empty for out-of-bounds index")
	}
}

// ---------------------------------------------------------------------------
// Count / FilteredCount
// ---------------------------------------------------------------------------

func TestHistorySearch_Counts(t *testing.T) {
	hs := newTestHistorySearch(t)
	if hs.Count() != 0 {
		t.Error("initial count should be 0")
	}
	if hs.FilteredCount() != 0 {
		t.Error("initial filtered count should be 0")
	}

	hs.Add("test", "claude")
	if hs.Count() != 1 {
		t.Error("count should be 1")
	}
}

// ---------------------------------------------------------------------------
// itoa
// ---------------------------------------------------------------------------

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
		{-123, "-123"},
		{999999, "999999"},
	}
	for _, tc := range tests {
		got := itoa(tc.input)
		if got != tc.expected {
			t.Errorf("itoa(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// highlightMatches
// ---------------------------------------------------------------------------

func TestHighlightMatches_EmptyQuery(t *testing.T) {
	matchStyle := lipgloss.NewStyle().Bold(true)
	normalStyle := lipgloss.NewStyle()

	result := highlightMatches("hello", "", matchStyle, normalStyle)
	if result == "" {
		t.Error("should return rendered text")
	}
}

func TestHighlightMatches_WithQuery(t *testing.T) {
	matchStyle := lipgloss.NewStyle().Bold(true)
	normalStyle := lipgloss.NewStyle()

	result := highlightMatches("hello world", "hel", matchStyle, normalStyle)
	if result == "" {
		t.Error("should return rendered text")
	}
}

func TestHighlightMatches_CaseInsensitive(t *testing.T) {
	matchStyle := lipgloss.NewStyle().Bold(true)
	normalStyle := lipgloss.NewStyle()

	result := highlightMatches("Hello World", "hello", matchStyle, normalStyle)
	if result == "" {
		t.Error("should return rendered text for case-insensitive match")
	}
}

func TestHighlightMatches_NoMatch(t *testing.T) {
	matchStyle := lipgloss.NewStyle().Bold(true)
	normalStyle := lipgloss.NewStyle()

	result := highlightMatches("hello", "xyz", matchStyle, normalStyle)
	if result == "" {
		t.Error("should still return rendered text even without match")
	}
}

func TestHighlightMatches_PartialFuzzyMatch(t *testing.T) {
	matchStyle := lipgloss.NewStyle().Bold(true)
	normalStyle := lipgloss.NewStyle()

	// Characters in query scattered across text
	result := highlightMatches("abcdef", "ace", matchStyle, normalStyle)
	if result == "" {
		t.Error("should return rendered text for partial fuzzy match")
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestHistorySearch_Render_NotVisible(t *testing.T) {
	hs := newTestHistorySearch(t)
	if hs.Render() != "" {
		t.Error("hidden history search should render empty string")
	}
}

func TestHistorySearch_Render_Visible_Empty(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 30)
	showTestHistory(hs)

	rendered := hs.Render()
	if rendered == "" {
		t.Fatal("visible history search should render something")
	}
	if !strings.Contains(rendered, "History") {
		t.Error("render should contain 'History' header")
	}
	if !strings.Contains(rendered, "No matching commands") {
		t.Error("empty history should show 'No matching commands'")
	}
}

func TestHistorySearch_Render_WithEntries(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 30)
	hs.Add("test command 1", "claude")
	hs.Add("test command 2", "gemini")
	showTestHistory(hs)

	rendered := hs.Render()
	if rendered == "" {
		t.Fatal("render should produce output")
	}
	if !strings.Contains(rendered, "2/2") {
		t.Error("render should show count")
	}
}

func TestHistorySearch_Render_WithScrollIndicators(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 15) // Small height

	// Add many entries
	for i := 0; i < 20; i++ {
		hs.Add(strings.Repeat("cmd", i+1), "claude")
	}
	showTestHistory(hs)

	// Move selection to trigger scroll
	for i := 0; i < 15; i++ {
		hs.MoveDown()
	}

	rendered := hs.Render()
	if !strings.Contains(rendered, "more above") {
		t.Error("render should show scroll-up indicator")
	}
}

func TestHistorySearch_Render_WithQuery(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 30)
	hs.Add("hello world", "claude")
	hs.Add("goodbye world", "claude")
	showTestHistory(hs)

	// Simulate typing a query
	hs.filter("hello")

	rendered := hs.Render()
	if rendered == "" {
		t.Fatal("render should produce output with query")
	}
}

func TestHistorySearch_Render_SmallHeight(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 8) // Very small
	hs.Add("test", "claude")
	showTestHistory(hs)

	rendered := hs.Render()
	if rendered == "" {
		t.Error("render should work with small height")
	}
}

func TestHistorySearch_Render_SmallWidth(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(25, 30) // Very narrow

	longCmd := strings.Repeat("a", 100)
	hs.Add(longCmd, "claude")
	showTestHistory(hs)

	rendered := hs.Render()
	if rendered == "" {
		t.Error("render should work with small width")
	}
}

func TestHistorySearch_Render_BottomScrollIndicator(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 15)

	// Add many entries so there are entries below the visible window
	for i := 0; i < 30; i++ {
		hs.Add(strings.Repeat("z", i+1), "claude")
	}
	showTestHistory(hs)

	// Stay at top; there should be entries below
	rendered := hs.Render()
	if !strings.Contains(rendered, "more below") {
		t.Error("render should show scroll-down indicator")
	}
}

func TestHistorySearch_Render_SelectedWithQuery(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.SetSize(80, 30)
	hs.Add("hello world", "claude")
	hs.Add("hello earth", "claude")
	hs.Add("goodbye", "claude")
	showTestHistory(hs)

	// Filter and select
	hs.filter("hello")
	hs.selectedIndex = 0

	rendered := hs.Render()
	if rendered == "" {
		t.Error("render should produce output")
	}
}

// ---------------------------------------------------------------------------
// UpdateInput
// ---------------------------------------------------------------------------

func TestHistorySearch_UpdateInput(t *testing.T) {
	hs := newTestHistorySearch(t)
	hs.Add("hello", "claude")
	hs.Add("world", "claude")
	showTestHistory(hs)

	// UpdateInput takes an interface{}; passing nil should not panic
	hs.UpdateInput(nil)
}

// ---------------------------------------------------------------------------
// HistoryEntry struct
// ---------------------------------------------------------------------------

func TestHistoryEntry_Struct(t *testing.T) {
	entry := HistoryEntry{
		Command:   "test command",
		Timestamp: time.Now(),
		Agent:     "claude",
	}

	if entry.Command != "test command" {
		t.Error("unexpected command")
	}
	if entry.Agent != "claude" {
		t.Error("unexpected agent")
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}
