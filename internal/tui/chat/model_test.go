package chat

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestModel creates a minimal Model suitable for unit tests.
// It calls WithChatConfig(0,0) so defaults are applied.
func newTestModel() Model {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	// Apply window size so the model is "ready"
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// newTestModelWithRunner creates a model with a mock workflow runner.
func newTestModelWithRunner(r *mockWorkflowRunner) Model {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	m.runner = r
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// cleanupModel closes the explorer panel to prevent goroutine leaks.
// Uses recover to handle the case where Close() was already called (e.g. by Ctrl+C).
func cleanupModel(t *testing.T, m *Model) {
	t.Helper()
	t.Cleanup(func() {
		defer func() { _ = recover() }()
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})
}

// submitInput sets the textarea value and calls handleSubmit, returning the updated model.
func submitInput(m Model, input string) Model {
	m.textarea.SetValue(input)
	updated, cmd := m.handleSubmit()
	m = updated.(Model)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestModel_Init_ReturnsCmd(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil batch command")
	}
}

// ---------------------------------------------------------------------------
// WithXxx builders
// ---------------------------------------------------------------------------

func TestModel_WithVersion(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	m = m.WithVersion("1.2.3")
	if m.version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", m.version)
	}
}

func TestModel_WithEditor(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	m = m.WithEditor("nvim")
	if m.editorCmd != "nvim" {
		t.Errorf("expected editor nvim, got %q", m.editorCmd)
	}
}

func TestModel_WithAgentModels(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	agents := []string{"claude", "gemini"}
	models := map[string][]string{"claude": {"opus", "sonnet"}, "gemini": {"pro"}}
	m = m.WithAgentModels(agents, models)
	if len(m.availableAgents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(m.availableAgents))
	}
	if len(m.agentModels["claude"]) != 2 {
		t.Errorf("expected 2 claude models, got %d", len(m.agentModels["claude"]))
	}
}

func TestModel_WithChatConfig_Defaults(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	m = m.WithChatConfig(0, 0)
	if m.chatTimeout != 20*time.Minute {
		t.Errorf("expected default timeout 20m, got %v", m.chatTimeout)
	}
	if m.chatProgressInterval != 15*time.Second {
		t.Errorf("expected default progress 15s, got %v", m.chatProgressInterval)
	}
}

func TestModel_WithChatConfig_Custom(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)
	m = m.WithChatConfig(5*time.Minute, 10*time.Second)
	if m.chatTimeout != 5*time.Minute {
		t.Errorf("expected 5m timeout, got %v", m.chatTimeout)
	}
	if m.chatProgressInterval != 10*time.Second {
		t.Errorf("expected 10s progress, got %v", m.chatProgressInterval)
	}
}

// ---------------------------------------------------------------------------
// WindowSizeMsg
// ---------------------------------------------------------------------------

func TestModel_Update_WindowSizeMsg(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	cleanupModel(t, &m)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	m = updated.(Model)

	if m.width != 200 || m.height != 60 {
		t.Errorf("expected 200x60, got %dx%d", m.width, m.height)
	}
	if !m.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
}

// ---------------------------------------------------------------------------
// View branches
// ---------------------------------------------------------------------------

func TestModel_View_BeforeReady(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	cleanupModel(t, &m)
	// Not ready yet (no WindowSizeMsg sent)
	v := m.View()
	if !strings.Contains(v, "Initializing") {
		t.Error("View before ready should show 'Initializing'")
	}
}

func TestModel_View_Quitting(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.quitting = true
	v := m.View()
	if !strings.Contains(v, "Goodbye") {
		t.Error("View when quitting should show 'Goodbye'")
	}
}

func TestModel_View_EmptyHistory(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	v := m.View()
	if !strings.Contains(v, "Quorum") {
		t.Error("View should contain 'Quorum' in header/welcome")
	}
}

func TestModel_View_WithVersion(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.version = "v2.0.0"
	m.updateViewport()
	v := m.View()
	if !strings.Contains(v, "v2.0.0") {
		t.Error("View should show version when set")
	}
}

// ---------------------------------------------------------------------------
// Key messages: Ctrl+C, Ctrl+X, Ctrl+L, Ctrl+E, Ctrl+R, Ctrl+T, etc.
// ---------------------------------------------------------------------------

func TestModel_KeyCtrlC_Quits(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if !m.quitting {
		t.Error("Ctrl+C should set quitting=true")
	}
	if cmd == nil {
		t.Error("Ctrl+C should return a quit command")
	}
}

func TestModel_KeyCtrlL_TogglesLogs(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	initial := m.showLogs

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	if m.showLogs == initial {
		t.Error("Ctrl+L should toggle logs panel")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	if m.showLogs != initial {
		t.Error("Ctrl+L twice should restore original state")
	}
}

func TestModel_KeyCtrlE_TogglesExplorer(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	initial := m.showExplorer

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = updated.(Model)
	if m.showExplorer == initial {
		t.Error("Ctrl+E should toggle explorer panel")
	}
}

func TestModel_KeyCtrlR_TogglesStats(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	initial := m.showStats

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(Model)
	if m.showStats == initial {
		t.Error("Ctrl+R should toggle stats panel")
	}
}

func TestModel_KeyCtrlT_TogglesTokens(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	initial := m.showTokens

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.showTokens == initial {
		t.Error("Ctrl+T should toggle tokens panel")
	}
}

func TestModel_KeyCtrlQ_TogglesConsensus(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)
	if !m.consensusPanel.IsVisible() {
		t.Error("Ctrl+Q should show consensus panel")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)
	if m.consensusPanel.IsVisible() {
		t.Error("Ctrl+Q again should hide consensus panel")
	}
}

func TestModel_KeyCtrlH_TogglesHistorySearch(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	m = updated.(Model)
	if !m.historySearch.IsVisible() {
		t.Error("Ctrl+H should show history search")
	}
	if m.inputFocused {
		t.Error("input should be blurred when history search is visible")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	m = updated.(Model)
	if m.historySearch.IsVisible() {
		t.Error("Ctrl+H again should hide history search")
	}
}

func TestModel_KeyCtrlD_DiffView(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)

	// No content: should not show
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	if m.diffView.IsVisible() {
		t.Error("Ctrl+D should not show diff view when no content")
	}

	// Add content to diff view
	m.diffView.SetContent("output from claude", "output from gemini")
	m.diffView.SetAgents("claude", "gemini")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	if !m.diffView.IsVisible() {
		t.Error("Ctrl+D should show diff view when content is present")
	}
}

func TestModel_KeyCtrlX_CancelsStreaming(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.streaming = true
	cancelled := false
	m.cancelFunc = func() { cancelled = true }

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.streaming {
		t.Error("Ctrl+X should stop streaming")
	}
	if !cancelled {
		t.Error("Ctrl+X should call cancelFunc")
	}
}

func TestModel_KeyCtrlX_CancelsWorkflow(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.workflowRunning = true
	m.agentInfos = []*AgentInfo{
		{Name: "Claude", Status: AgentStatusRunning},
	}
	cancelled := false
	m.cancelFunc = func() { cancelled = true }

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.workflowRunning {
		t.Error("Ctrl+X should stop workflow")
	}
	if !cancelled {
		t.Error("Ctrl+X should call cancelFunc for workflow")
	}
	if m.agentInfos[0].Status != AgentStatusIdle {
		t.Error("running agents should be reset to idle on cancel")
	}
}

func TestModel_KeyF1_ShortcutsOverlay(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(Model)
	if !m.shortcutsOverlay.IsVisible() {
		t.Error("F1 should show shortcuts overlay")
	}
}

func TestModel_KeyQuestionMark_ShortcutsOverlay(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	// Input must be empty for ? to trigger shortcut
	m.textarea.Reset()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.shortcutsOverlay.IsVisible() {
		t.Error("? on empty input should show shortcuts overlay")
	}
}

// ---------------------------------------------------------------------------
// Escape key branches
// ---------------------------------------------------------------------------

func TestModel_KeyEsc_ClosesSuggestions(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.showSuggestions = true
	m.suggestions = []string{"help", "status"}
	m.textarea.SetValue("/")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showSuggestions {
		t.Error("Esc should close suggestions")
	}
}

func TestModel_KeyEsc_ClosesExplorerFocus(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.showExplorer = true
	m.explorerFocus = true
	m.inputFocused = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.explorerFocus {
		t.Error("Esc should defocus explorer")
	}
	if !m.inputFocused {
		t.Error("Esc should return focus to input")
	}
}

func TestModel_KeyEsc_ClosesLogsFocus(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.showLogs = true
	m.logsFocus = true
	m.inputFocused = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.logsFocus {
		t.Error("Esc should defocus logs")
	}
	if !m.inputFocused {
		t.Error("Esc should return focus to input")
	}
}

func TestModel_KeyEsc_ClearsInput(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.inputFocused = true
	m.textarea.SetValue("some text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.textarea.Value() != "" {
		t.Error("Esc should clear input when focused")
	}
}

func TestModel_KeyEsc_ClosesShortcutsOverlay(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.shortcutsOverlay.Toggle() // show it

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.shortcutsOverlay.IsVisible() {
		t.Error("Esc should close shortcuts overlay")
	}
}

func TestModel_KeyEsc_ClosesHistorySearch(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.historySearch.Toggle() // show it

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.historySearch.IsVisible() {
		t.Error("Esc should close history search")
	}
}

func TestModel_KeyEsc_ClosesConsensusPanel(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.consensusPanel.Toggle() // show it

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.consensusPanel.IsVisible() {
		t.Error("Esc should close consensus panel")
	}
}

func TestModel_KeyEsc_ClosesTasksPanel(t *testing.T) {
	m := newTestModel()
	cleanupModel(t, &m)
	m.tasksPanel.Toggle() // show it

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.tasksPanel.IsVisible() {
		t.Error("Esc should close tasks panel")
	}
}
