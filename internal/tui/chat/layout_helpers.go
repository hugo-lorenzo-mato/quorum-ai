package chat

// calculateSidebarWidths computes left/right sidebar widths and remaining main width.
func (m *Model) calculateSidebarWidths() (leftSidebarWidth, rightSidebarWidth, mainWidth int) {
	mainWidth = m.width
	showLeftSidebar := m.showExplorer || m.showTokens
	showRightSidebar := m.showStats || m.showLogs
	bothSidebarsOpen := showLeftSidebar && showRightSidebar
	oneSidebarOpen := (showLeftSidebar || showRightSidebar) && !bothSidebarsOpen

	if showLeftSidebar {
		if oneSidebarOpen {
			leftSidebarWidth = m.width * 2 / 5
			if leftSidebarWidth < 35 {
				leftSidebarWidth = 35
			}
			if leftSidebarWidth > 70 {
				leftSidebarWidth = 70
			}
		} else {
			leftSidebarWidth = m.width / 4
			if leftSidebarWidth < 30 {
				leftSidebarWidth = 30
			}
			if leftSidebarWidth > 50 {
				leftSidebarWidth = 50
			}
		}
		mainWidth -= leftSidebarWidth
	}

	if showRightSidebar {
		if oneSidebarOpen {
			rightSidebarWidth = m.width * 2 / 5
			if rightSidebarWidth < 40 {
				rightSidebarWidth = 40
			}
			if rightSidebarWidth > 80 {
				rightSidebarWidth = 80
			}
		} else {
			rightSidebarWidth = m.width / 4
			if rightSidebarWidth < 35 {
				rightSidebarWidth = 35
			}
			if rightSidebarWidth > 60 {
				rightSidebarWidth = 60
			}
		}
		mainWidth -= rightSidebarWidth
	}

	return leftSidebarWidth, rightSidebarWidth, mainWidth
}

// normalizePanelWidths adjusts panel widths to prevent overflow and ensure minimums.
func (m *Model) normalizePanelWidths(leftSidebarWidth, rightSidebarWidth, mainWidth int) (int, int, int) {
	showLeftSidebar := m.showExplorer || m.showTokens
	showRightSidebar := m.showStats || m.showLogs

	// === NORMALIZATION OF WIDTHS TO PREVENT OVERFLOW ===
	totalUsed := mainWidth
	if showLeftSidebar {
		totalUsed += leftSidebarWidth
	}
	if showRightSidebar {
		totalUsed += rightSidebarWidth
	}

	if totalUsed > m.width {
		excess := totalUsed - m.width
		if mainWidth-excess >= 40 {
			mainWidth -= excess
		} else {
			reduction := excess / 2
			if showLeftSidebar && leftSidebarWidth-reduction >= 25 {
				leftSidebarWidth -= reduction
				excess -= reduction
			}
			if showRightSidebar && rightSidebarWidth-reduction >= 30 {
				rightSidebarWidth -= reduction
				excess -= reduction
			}
			if excess > 0 && mainWidth-excess >= 40 {
				mainWidth -= excess
			}
		}
	}

	// Ensure minimum main width
	if mainWidth < 40 {
		mainWidth = 40
	}

	// === FINAL OVERFLOW CHECK ===
	finalTotal := mainWidth
	if showLeftSidebar {
		finalTotal += leftSidebarWidth
	}
	if showRightSidebar {
		finalTotal += rightSidebarWidth
	}

	for finalTotal > m.width && (leftSidebarWidth > 20 || rightSidebarWidth > 20) {
		if showRightSidebar && rightSidebarWidth > 20 {
			rightSidebarWidth--
			finalTotal--
		}
		if finalTotal > m.width && showLeftSidebar && leftSidebarWidth > 20 {
			leftSidebarWidth--
			finalTotal--
		}
	}

	if finalTotal > m.width {
		excess := finalTotal - m.width
		mainWidth -= excess
		if mainWidth < 30 {
			mainWidth = 30
		}
	}

	return leftSidebarWidth, rightSidebarWidth, mainWidth
}

// applySidebarSizes sets the dimensions of all sidebar panels.
func (m *Model) applySidebarSizes(leftSidebarWidth, rightSidebarWidth, sidebarHeight int) {
	if m.showExplorer || m.showTokens {
		if m.showExplorer && m.showTokens {
			explorerHeight := sidebarHeight / 2
			tokenHeight := sidebarHeight - explorerHeight
			m.explorerPanel.SetSize(leftSidebarWidth, explorerHeight)
			m.tokenPanel.SetSize(leftSidebarWidth, tokenHeight)
		} else if m.showExplorer {
			m.explorerPanel.SetSize(leftSidebarWidth, sidebarHeight)
		} else if m.showTokens {
			m.tokenPanel.SetSize(leftSidebarWidth, sidebarHeight)
		}
	}

	if m.showStats && m.showLogs {
		logsHeight := sidebarHeight / 2
		statsHeight := sidebarHeight - logsHeight
		m.logsPanel.SetSize(rightSidebarWidth, logsHeight)
		m.statsPanel.SetSize(rightSidebarWidth, statsHeight)
	} else if m.showStats {
		m.statsPanel.SetSize(rightSidebarWidth, sidebarHeight)
	} else if m.showLogs {
		m.logsPanel.SetSize(rightSidebarWidth, sidebarHeight)
	}
}
