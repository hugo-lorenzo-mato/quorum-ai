package chat

import tea "github.com/charmbracelet/bubbletea"

// handleLeftSidebarClick handles mouse clicks in the left sidebar area (explorer/tokens).
func (m Model) handleLeftSidebarClick(y int) (tea.Model, tea.Cmd, bool) {
	if m.showExplorer && m.showTokens {
		explorerHeight := m.explorerPanel.Height()
		if y < explorerHeight {
			if !m.explorerFocus {
				m.explorerFocus = true
				m.tokensFocus = false
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(true)
				return m, nil, true
			}
			return m, nil, false
		}
		if !m.tokensFocus {
			m.tokensFocus = true
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		m.tokensFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	if m.showExplorer {
		if !m.explorerFocus {
			m.explorerFocus = true
			m.tokensFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
			return m, nil, true
		}
		return m, nil, false
	}
	if m.showTokens {
		if !m.tokensFocus {
			m.tokensFocus = true
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		m.tokensFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	return m, nil, false
}

// handleRightSidebarClick handles mouse clicks in the right sidebar area (logs/stats).
func (m Model) handleRightSidebarClick(y int) (tea.Model, tea.Cmd, bool) {
	if m.showLogs && m.showStats {
		logsHeight := m.logsPanel.Height()
		if y < logsHeight {
			if !m.logsFocus {
				m.logsFocus = true
				m.statsFocus = false
				m.explorerFocus = false
				m.tokensFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if !m.statsFocus {
			m.statsFocus = true
			m.logsFocus = false
			m.explorerFocus = false
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		m.statsFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	if m.showLogs {
		if !m.logsFocus {
			m.logsFocus = true
			m.statsFocus = false
			m.explorerFocus = false
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		m.logsFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	if m.showStats {
		if !m.statsFocus {
			m.statsFocus = true
			m.logsFocus = false
			m.explorerFocus = false
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		m.statsFocus = false
		m.inputFocused = true
		m.textarea.Focus()
		return m, nil, true
	}
	return m, nil, false
}
