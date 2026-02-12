package chat

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handlePanelNavKeys handles keyboard input during panel navigation mode (tmux-style: Ctrl+z then arrow keys).
// Returns (model, cmd, handled).
func (m Model) handlePanelNavKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	refreshPanelNav := func() tea.Cmd {
		m.panelNavSeq++
		m.panelNavTill = time.Now().Add(panelNavWindow)
		return panelNavTimeoutCmd(m.panelNavSeq)
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter, tea.KeySpace:
		m.panelNavMode = false
		m.panelNavSeq++
		m.focusInput()
		return m, nil, true
	case tea.KeyLeft:
		if !(m.explorerFocus || m.tokensFocus) {
			m.focusLeftSidebar("")
		}
		return m, refreshPanelNav(), true
	case tea.KeyRight:
		if !(m.logsFocus || m.statsFocus) {
			m.focusRightSidebar("")
		}
		return m, refreshPanelNav(), true
	case tea.KeyUp:
		if m.logsFocus || m.statsFocus {
			m.focusRightSidebar("logs")
		} else if m.explorerFocus || m.tokensFocus {
			m.focusLeftSidebar("explorer")
		} else if m.showLogs || m.showStats {
			m.focusRightSidebar("logs")
		} else {
			m.focusLeftSidebar("explorer")
		}
		return m, refreshPanelNav(), true
	case tea.KeyDown:
		if m.logsFocus || m.statsFocus {
			m.focusRightSidebar("stats")
		} else if m.explorerFocus || m.tokensFocus {
			m.focusLeftSidebar("tokens")
		} else if m.showLogs || m.showStats {
			m.focusRightSidebar("stats")
		} else {
			m.focusLeftSidebar("tokens")
		}
		return m, refreshPanelNav(), true
	default:
		m.panelNavMode = false
		m.panelNavSeq++
		m.focusInput()
		return m, nil, true
	}
}

// focusInput returns focus to the chat input.
func (m *Model) focusInput() {
	m.explorerFocus = false
	m.logsFocus = false
	m.statsFocus = false
	m.tokensFocus = false
	m.inputFocused = true
	m.textarea.Focus()
	m.explorerPanel.SetFocused(false)
}

// focusLeftSidebar focuses a panel in the left sidebar (explorer or tokens).
func (m *Model) focusLeftSidebar(prefer string) {
	if !m.showExplorer && !m.showTokens {
		return
	}
	if !(m.explorerFocus || m.tokensFocus) && prefer == "" {
		prefer = "explorer"
	}
	if prefer == "explorer" && m.showExplorer {
		m.explorerFocus = true
		m.tokensFocus = false
	} else if prefer == "tokens" && m.showTokens {
		m.tokensFocus = true
		m.explorerFocus = false
	} else if m.showExplorer {
		m.explorerFocus = true
		m.tokensFocus = false
	} else {
		m.tokensFocus = true
		m.explorerFocus = false
	}
	if m.explorerFocus || m.tokensFocus {
		m.logsFocus = false
		m.statsFocus = false
		m.inputFocused = false
		m.textarea.Blur()
		m.explorerPanel.SetFocused(m.explorerFocus)
	}
}

// focusRightSidebar focuses a panel in the right sidebar (logs or stats).
func (m *Model) focusRightSidebar(prefer string) {
	if !m.showLogs && !m.showStats {
		return
	}
	if !(m.logsFocus || m.statsFocus) && prefer == "" {
		prefer = "logs"
	}
	if prefer == "logs" && m.showLogs {
		m.logsFocus = true
		m.statsFocus = false
	} else if prefer == "stats" && m.showStats {
		m.statsFocus = true
		m.logsFocus = false
	} else if m.showLogs {
		m.logsFocus = true
		m.statsFocus = false
	} else {
		m.statsFocus = true
		m.logsFocus = false
	}
	if m.logsFocus || m.statsFocus {
		m.explorerFocus = false
		m.tokensFocus = false
		m.inputFocused = false
		m.textarea.Blur()
		m.explorerPanel.SetFocused(false)
	}
}

// handleFileViewerKeys handles keyboard input when the file viewer is visible.
// Returns (model, cmd, handled).
func (m Model) handleFileViewerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp:
		m.fileViewer.ScrollUp()
		return m, nil, true
	case tea.KeyDown:
		m.fileViewer.ScrollDown()
		return m, nil, true
	case tea.KeyLeft:
		m.fileViewer.ScrollLeft()
		return m, nil, true
	case tea.KeyRight:
		m.fileViewer.ScrollRight()
		return m, nil, true
	case tea.KeyPgUp:
		m.fileViewer.PageUp()
		return m, nil, true
	case tea.KeyPgDown:
		m.fileViewer.PageDown()
		return m, nil, true
	}
	switch msg.String() {
	case "q":
		m.fileViewer.Hide()
		return m, nil, true
	case "e":
		// Open file in editor (config > $EDITOR > $VISUAL > vi)
		filePath := m.fileViewer.GetFilePath()
		if filePath != "" {
			editor := m.editorCmd
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				editor = "vi" // fallback
			}
			editorPath, err := exec.LookPath(editor)
			if err != nil {
				m.logsPanel.AddError("editor", fmt.Sprintf("Editor not found: %s", editor))
				return m, nil, true
			}
			m.fileViewer.Hide()
			// #nosec G204 -- editor is user-configured and resolved via LookPath
			cmd := exec.Command(editorPath, filePath)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return editorFinishedMsg{filePath: filePath, err: err}
			}), true
		}
		return m, nil, true
	case "h":
		m.fileViewer.ScrollLeft()
		return m, nil, true
	case "l":
		m.fileViewer.ScrollRight()
		return m, nil, true
	case "0":
		m.fileViewer.ScrollHome()
		return m, nil, true
	case "$":
		m.fileViewer.ScrollEnd()
		return m, nil, true
	case "g":
		m.fileViewer.ScrollTop()
		return m, nil, true
	case "G":
		m.fileViewer.ScrollBottom()
		return m, nil, true
	}
	// Block other keys when file viewer is open
	return m, nil, true
}

// handleOverlayNavKeys handles keyboard input for overlays: shortcuts, history search, and diff view.
// Returns (model, cmd, handled). Returns false if no overlay handled the key.
func (m Model) handleOverlayNavKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// Handle ? for shortcuts overlay (only when input is empty)
	if msg.String() == "?" && m.textarea.Value() == "" {
		m.shortcutsOverlay.Toggle()
		return m, nil, true
	}

	// Handle F1 for shortcuts overlay (always works)
	if msg.Type == tea.KeyF1 {
		m.shortcutsOverlay.Toggle()
		return m, nil, true
	}

	// Close any visible overlays on Escape
	if msg.Type == tea.KeyEsc {
		if m.shortcutsOverlay.IsVisible() {
			m.shortcutsOverlay.Hide()
			return m, nil, true
		}
		if m.historySearch.IsVisible() {
			m.historySearch.Hide()
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if m.diffView.IsVisible() {
			m.diffView.Hide()
			return m, nil, true
		}
		if m.consensusPanel.IsVisible() {
			m.consensusPanel.Toggle()
			return m, nil, true
		}
		if m.tasksPanel.IsVisible() {
			m.tasksPanel.Hide()
			return m, nil, true
		}
		// Note: fileViewer uses 'q' to close, not Escape
	}

	// Handle history search navigation when visible
	if m.historySearch.IsVisible() {
		switch msg.Type {
		case tea.KeyUp:
			m.historySearch.MoveUp()
			return m, nil, true
		case tea.KeyDown:
			m.historySearch.MoveDown()
			return m, nil, true
		case tea.KeyEnter:
			// Select command and insert into textarea
			selected := m.historySearch.GetSelected()
			if selected != "" {
				m.textarea.SetValue(selected)
				m.textarea.CursorEnd()
			}
			m.historySearch.Hide()
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		default:
			// Pass to history search input
			m.historySearch.UpdateInput(msg)
			return m, nil, true
		}
	}

	// Handle diff view navigation when visible
	if m.diffView.IsVisible() {
		switch msg.Type {
		case tea.KeyUp:
			m.diffView.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.diffView.ScrollDown()
			return m, nil, true
		case tea.KeyTab:
			m.diffView.NextPair()
			return m, nil, true
		case tea.KeyLeft, tea.KeyRight:
			// Switch agent pair
			if msg.Type == tea.KeyLeft {
				m.diffView.PrevPair()
			} else {
				m.diffView.NextPair()
			}
			return m, nil, true
		}
	}

	return m, nil, false
}

// handleExplorerKeys handles keyboard input when the explorer panel has focus.
// Returns (model, cmd, handled).
func (m Model) handleExplorerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp:
		m.explorerPanel.MoveUp()
		return m, nil, true
	case tea.KeyDown:
		m.explorerPanel.MoveDown()
		return m, nil, true
	case tea.KeyLeft:
		// Collapse directory or go up
		entry := m.explorerPanel.GetSelectedEntry()
		if entry != nil && entry.Type == FileTypeDir && entry.Expanded {
			m.explorerPanel.Toggle()
		} else {
			m.explorerPanel.GoUp()
		}
		return m, nil, true
	case tea.KeyRight:
		// Expand directory
		entry := m.explorerPanel.GetSelectedEntry()
		if entry != nil && entry.Type == FileTypeDir && !entry.Expanded {
			m.explorerPanel.Toggle()
		}
		return m, nil, true
	case tea.KeyEnter:
		// Enter directory or open file viewer
		path := m.explorerPanel.Enter()
		if path != "" {
			// File selected - open in file viewer
			if err := m.fileViewer.SetFile(path); err != nil {
				m.logsPanel.AddError("explorer", "Cannot open: "+err.Error())
			} else {
				m.fileViewer.Show()
			}
		}
		return m, nil, true
	case tea.KeyTab:
		// Insert selected path into chat with @ prefix and switch focus
		relPath := m.explorerPanel.GetSelectedRelativePath()
		if relPath != "" {
			// Insert path reference with @ prefix into textarea
			currentValue := m.textarea.Value()
			pathRef := "@" + relPath
			if currentValue != "" && !strings.HasSuffix(currentValue, " ") && !strings.HasSuffix(currentValue, "\n") {
				pathRef = " " + pathRef
			}
			m.textarea.SetValue(currentValue + pathRef)
			// Move cursor to end
			m.textarea.CursorEnd()
		}
		m.explorerFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		m.explorerPanel.SetFocused(false)
		return m, nil, true
	}
	return m, nil, false
}

// handleFocusedPanelKeys handles keyboard input for focused panels (tokens, stats, logs) and Ctrl+Shift+C copy.
// Returns (model, cmd, handled).
func (m Model) handleFocusedPanelKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// Handle token panel scrolling
	if m.tokensFocus && m.showTokens {
		if handled := scrollPanel(m.tokenPanel, msg); handled {
			return m, nil, true
		}
		if msg.Type == tea.KeyTab {
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
	}

	// Handle stats panel scrolling
	if m.statsFocus && m.showStats {
		if handled := scrollPanel(m.statsPanel, msg); handled {
			return m, nil, true
		}
		if msg.Type == tea.KeyTab {
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
	}

	// Handle logs panel scrolling
	if m.logsFocus && m.showLogs {
		if handled := scrollPanel(m.logsPanel, msg); handled {
			return m, nil, true
		}
		switch msg.Type {
		case tea.KeyTab:
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'c' || msg.Runes[0] == 'y') {
				return m.copyLogsToClipboard()
			}
		}
	}

	if msg.String() == "ctrl+shift+c" {
		return m.copyLastResponse()
	}

	return m, nil, false
}

// scrollable defines the interface for scrollable panels.
type scrollable interface {
	ScrollUp()
	ScrollDown()
	PageUp()
	PageDown()
	GotoTop()
	GotoBottom()
}

// scrollPanel handles standard scroll keys for any scrollable panel.
func scrollPanel(p scrollable, msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		p.ScrollUp()
	case tea.KeyDown:
		p.ScrollDown()
	case tea.KeyPgUp:
		p.PageUp()
	case tea.KeyPgDown:
		p.PageDown()
	case tea.KeyHome:
		p.GotoTop()
	case tea.KeyEnd:
		p.GotoBottom()
	default:
		return false
	}
	return true
}

// handleEnterWithSuggestions handles the Enter key when the suggestion dropdown is visible.
// Returns (model, cmd, handled).
func (m Model) handleEnterWithSuggestions(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	selected := m.suggestions[m.suggestionIndex]

	switch m.suggestionType {
	case "agent":
		// Complete and execute /agent command
		m.textarea.SetValue("/agent " + selected)
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		model, teaCmd := m.handleSubmit()
		return model, teaCmd, true

	case "model":
		// Complete and execute /model command
		m.textarea.SetValue("/model " + selected)
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		model, teaCmd := m.handleSubmit()
		return model, teaCmd, true

	case "workflow":
		// Complete and execute /load command
		m.textarea.SetValue("/load " + selected)
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		m.workflowCache = nil // Clear cache to refresh on next use
		model, teaCmd := m.handleSubmit()
		return model, teaCmd, true

	case "theme":
		// Complete and execute /theme command
		m.textarea.SetValue("/theme " + selected)
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		model, teaCmd := m.handleSubmit()
		return model, teaCmd, true

	default:
		// Command suggestion
		selectedCmd := m.commands.Get(selected)

		// If command requires arguments, autocomplete like Tab (let user add args)
		if selectedCmd != nil && selectedCmd.RequiresArg() {
			m.textarea.SetValue("/" + selected + " ")
			m.textarea.CursorEnd()
			m.showSuggestions = false
			m.suggestionIndex = 0
			m.suggestionType = ""
			return m, nil, true
		}

		// Command doesn't require args - execute immediately
		m.textarea.SetValue("/" + selected)
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		model, teaCmd := m.handleSubmit()
		return model, teaCmd, true
	}
}

// handlePanelToggleKeys handles Ctrl key combinations that toggle panels and cancel operations.
// Returns (model, cmd, handled).
func (m Model) handlePanelToggleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyCtrlL:
		// Toggle logs panel
		m.showLogs = !m.showLogs
		if m.showLogs {
			m.logsFocus = true
			m.explorerFocus = false
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
		} else {
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlE:
		// Toggle explorer panel
		m.showExplorer = !m.showExplorer
		if m.showExplorer {
			m.explorerFocus = true
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
		} else {
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlR:
		// Toggle stats panel (resources)
		m.showStats = !m.showStats
		if m.showStats {
			m.statsFocus = true
			m.logsFocus = false
			m.tokensFocus = false
			m.explorerFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
		} else {
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlT:
		// Toggle tokens panel (left sidebar)
		m.showTokens = !m.showTokens
		if m.showTokens {
			m.tokensFocus = true
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
		} else {
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlX:
		if newModel, cmd, handled := m.handleCancelOperation(); handled {
			return newModel, cmd, true
		}

	case tea.KeyCtrlQ, tea.KeyCtrlK:
		// Toggle quorum panel
		m.consensusPanel.Toggle()
		return m, nil, true

	case tea.KeyCtrlD:
		// Toggle diff view
		if m.diffView.HasContent() {
			m.diffView.Toggle()
		}
		return m, nil, true

	case tea.KeyCtrlH:
		// Toggle history search
		m.historySearch.Toggle()
		if m.historySearch.IsVisible() {
			m.inputFocused = false
			m.textarea.Blur()
		} else {
			m.inputFocused = true
			m.textarea.Focus()
		}
		return m, nil, true
	}

	return m, nil, false
}

// handleCancelOperation cancels the current streaming or workflow operation (Ctrl+X).
func (m Model) handleCancelOperation() (tea.Model, tea.Cmd, bool) {
	if !m.streaming && !m.workflowRunning {
		return m, nil, false
	}
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	if m.controlPlane != nil && m.workflowRunning {
		m.controlPlane.Cancel()
	}
	wasStreaming := m.streaming
	wasWorkflow := m.workflowRunning
	m.streaming = false
	m.workflowRunning = false

	if wasWorkflow {
		m.workflowPhase = "idle"
		for _, a := range m.agentInfos {
			if a.Status == AgentStatusRunning {
				a.Status = AgentStatusIdle
			}
		}
		m.logsPanel.AddWarn("system", "Workflow interrupted by user (Ctrl+X)")
		m.history.Add(NewSystemMessage("Workflow interrupted"))
	} else if wasStreaming {
		m.logsPanel.AddWarn("system", "Request interrupted by user (Ctrl+X)")
		m.history.Add(NewSystemMessage("Request interrupted"))
	}
	m.updateViewport()
	return m, nil, true
}

// handleTabKey handles the Tab key: autocomplete suggestion or toggle issues panel.
// Returns (model, cmd, handled).
func (m Model) handleTabKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.showSuggestions && len(m.suggestions) > 0 {
		// Complete with selected suggestion based on type
		switch m.suggestionType {
		case "agent":
			m.textarea.SetValue("/agent " + m.suggestions[m.suggestionIndex])
		case "model":
			m.textarea.SetValue("/model " + m.suggestions[m.suggestionIndex])
		case "workflow":
			m.textarea.SetValue("/load " + m.suggestions[m.suggestionIndex])
		case "theme":
			m.textarea.SetValue("/theme " + m.suggestions[m.suggestionIndex])
		default:
			m.textarea.SetValue("/" + m.suggestions[m.suggestionIndex] + " ")
		}
		m.textarea.CursorEnd()
		m.showSuggestions = false
		m.suggestionIndex = 0
		m.suggestionType = ""
		return m, nil, true
	}
	// Close other overlays/focus states before toggling issues panel
	if m.explorerFocus {
		m.explorerFocus = false
		m.explorerPanel.SetFocused(false)
	}
	if m.logsFocus {
		m.logsFocus = false
	}
	if m.tokensFocus {
		m.tokensFocus = false
	}
	if m.diffView.IsVisible() {
		m.diffView.Hide()
	}
	if m.historySearch.IsVisible() {
		m.historySearch.Hide()
	}
	m.inputFocused = true
	m.textarea.Focus()
	m.tasksPanel.Toggle()
	return m, nil, true
}

// handleEscapeInContext handles Escape key behavior based on current focus/input context.
// Returns (model, cmd, handled). Returns false if no context-specific action was taken.
func (m Model) handleEscapeInContext(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.explorerFocus && m.showExplorer {
		m.explorerFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		m.explorerPanel.SetFocused(false)
		return m, nil, true
	}
	if m.logsFocus && m.showLogs {
		m.logsFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	if m.pendingInputRequest != nil {
		if m.controlPlane != nil {
			_ = m.controlPlane.CancelUserInput(m.pendingInputRequest.ID)
		}
		m.pendingInputRequest = nil
		m.history.Add(NewSystemMessage("Input cancelled"))
		m.updateViewport()
		return m, nil, true
	}
	if m.inputFocused && m.textarea.Value() != "" {
		m.textarea.Reset()
		m.showSuggestions = false
		m.recalculateLayout()
		return m, nil, true
	}
	return m, nil, false
}

// handleCtrlSpaceKey handles Ctrl+Space/Ctrl+@ to force show autocomplete.
// Returns (model, cmd, handled).
func (m Model) handleCtrlSpaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.streaming || m.workflowRunning {
		return m, nil, true
	}
	val := m.textarea.Value()
	if val == "" {
		m.textarea.SetValue("/")
		val = "/"
	}
	if strings.HasPrefix(val, "/") {
		m.suggestions = m.commands.Suggest(val)
		m.showSuggestions = len(m.suggestions) > 0
		m.suggestionIndex = 0
		return m, nil, true
	}
	return m, nil, false
}

// handleSuggestionNav handles Up/Down arrow keys when suggestion dropdown is visible.
// Returns (model, cmd, handled). Returns false if suggestions are not visible.
func (m Model) handleSuggestionNav(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !m.showSuggestions || len(m.suggestions) == 0 {
		return m, nil, false
	}
	switch msg.Type {
	case tea.KeyUp:
		m.suggestionIndex--
		if m.suggestionIndex < 0 {
			m.suggestionIndex = len(m.suggestions) - 1
		}
		return m, nil, true
	case tea.KeyDown:
		m.suggestionIndex++
		if m.suggestionIndex >= len(m.suggestions) {
			m.suggestionIndex = 0
		}
		return m, nil, true
	}
	return m, nil, false
}
