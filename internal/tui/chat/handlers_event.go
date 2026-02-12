package chat

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleAgentResponse processes an AgentResponseMsg: updates streaming state,
// handles error/success, updates token counts, and logs completion info.
func (m *Model) handleAgentResponse(msg AgentResponseMsg) {
	elapsed := time.Since(m.chatStartedAt)
	m.streaming = false
	agentLower := strings.ToLower(msg.Agent)
	if msg.Error != nil {
		errMsg := msg.Error.Error()
		// Check for timeout errors
		if strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timed out") {
			m.history.Add(NewSystemMessage(fmt.Sprintf("⏱ Request timed out after %s", formatDuration(elapsed))))
			m.logsPanel.AddError(agentLower, fmt.Sprintf("⏱ Timeout after %s - consider using a faster model", formatDuration(elapsed)))
		} else {
			m.history.Add(NewSystemMessage("Error: " + errMsg))
			m.logsPanel.AddError(agentLower, fmt.Sprintf("✗ Error after %s: %s", formatDuration(elapsed), errMsg))
		}
	} else {
		m.history.Add(NewAgentMessage(msg.Agent, msg.Content))
		// Update token counts for the agent (validate to avoid corrupted values)
		// Cap matches the adapter-level cap (500k) to ensure consistency
		const maxReasonableTokens = 500_000
		for _, a := range m.agentInfos {
			if strings.EqualFold(a.Name, msg.Agent) {
				if m.chatModel != "" {
					a.Model = m.chatModel
				}
				if msg.TokensIn > 0 && msg.TokensIn <= maxReasonableTokens {
					a.TokensIn += msg.TokensIn
				}
				if msg.TokensOut > 0 && msg.TokensOut <= maxReasonableTokens {
					a.TokensOut += msg.TokensOut
				}
				break
			}
		}
		// Build detailed completion log
		stats := []string{fmt.Sprintf("%d chars", len(msg.Content))}
		if msg.TokensIn > 0 || msg.TokensOut > 0 {
			stats = append(stats, fmt.Sprintf("↑%d ↓%d tok", msg.TokensIn, msg.TokensOut))
		}
		stats = append(stats, formatDuration(elapsed))
		m.logsPanel.AddSuccess(agentLower, fmt.Sprintf("✓ Response [%s]", strings.Join(stats, " | ")))
	}
	m.updateViewport()
	m.updateLogsPanelTokenStats()
	m.updateTokenPanelStats()
}

// handleShellOutput processes a ShellOutputMsg: handles shell command output/error
// and refreshes the explorer panel.
func (m *Model) handleShellOutput(msg ShellOutputMsg) {
	// Handle shell command output
	if msg.Error != "" {
		m.history.Add(NewSystemMessage("Error executing command: " + msg.Error))
		m.logsPanel.AddError("shell", "Command failed: "+msg.Error)
	} else {
		output := msg.Output
		if output == "" {
			output = "(no output)"
		}
		// Format output as code block
		formattedOutput := fmt.Sprintf("```\n%s```", output)
		if msg.ExitCode != 0 {
			formattedOutput += fmt.Sprintf("\n*Exit code: %d*", msg.ExitCode)
		}
		m.history.Add(NewAgentMessage("Shell", formattedOutput))
		m.logsPanel.AddSuccess("shell", fmt.Sprintf("Command completed (exit %d)", msg.ExitCode))
	}
	// Refresh explorer to show any new files created by shell command
	if m.explorerPanel != nil {
		_ = m.explorerPanel.Refresh()
	}
	m.updateViewport()
}

// handleWorkflowStarted processes a WorkflowStartedMsg: sets running state,
// resets agents, auto-shows logs panel, and returns a spinner tick command.
func (m *Model) handleWorkflowStarted(msg WorkflowStartedMsg) tea.Cmd {
	m.workflowRunning = true
	m.workflowStartedAt = time.Now()
	m.workflowPhase = "running"
	m.consensusPanel.ClearOutputs()
	// Reset all agents to idle - actual agent events will set them to running
	for _, a := range m.agentInfos {
		if a.Status != AgentStatusDisabled {
			a.Status = AgentStatusIdle
		}
	}
	m.history.Add(NewSystemBubbleMessage("Starting workflow..."))
	m.logsPanel.AddInfo("workflow", "Workflow started: "+msg.Prompt)
	// Auto-show logs panel when workflow starts so user can see progress
	if !m.showLogs {
		m.showLogs = true
		// Recalculate layout with logs panel visible
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
	}
	m.updateViewport()
	return m.spinner.Tick
}

// handleWorkflowCompleted processes a WorkflowCompletedMsg: updates state,
// marks agents done, and builds a completion summary.
func (m *Model) handleWorkflowCompleted(msg WorkflowCompletedMsg) {
	elapsed := time.Since(m.workflowStartedAt)
	m.workflowRunning = false
	m.workflowPhase = "done"
	m.workflowState = msg.State
	m.tasksPanel.SetState(msg.State)
	m.updateQuorumPanel(msg.State)
	// Mark all running agents as done
	for _, a := range m.agentInfos {
		if a.Status == AgentStatusRunning {
			a.Status = AgentStatusDone
			m.logsPanel.AddSuccess(strings.ToLower(a.Name), "Agent completed")
		}
	}
	if msg.State != nil && msg.State.Metrics != nil {
		m.totalTokensIn = msg.State.Metrics.TotalTokensIn
		m.totalTokensOut = msg.State.Metrics.TotalTokensOut
	}
	// Build a user-friendly completion summary
	summaryParts := []string{fmt.Sprintf("✓ Workflow completed in %s", formatDuration(elapsed))}
	if msg.State != nil && msg.State.Metrics != nil {
		summaryParts = append(summaryParts, fmt.Sprintf("Tokens: %s in / %s out", formatTokens(m.totalTokensIn), formatTokens(m.totalTokensOut)))
		if msg.State.Metrics.ConsensusScore > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("Consensus: %.0f%%", msg.State.Metrics.ConsensusScore*100))
		}
	}
	m.logsPanel.AddSuccess("workflow", strings.Join(summaryParts, " | "))
	m.history.Add(NewSystemBubbleMessage(strings.Join(summaryParts, "\n")))
	if msg.State != nil {
		status := strings.TrimPrefix(formatWorkflowStatus(msg.State), "/status\n\n")
		m.history.Add(NewSystemBubbleMessage(status))
	}
	m.updateViewport()
}

// handleWorkflowError processes a WorkflowErrorMsg: marks agents as error
// and logs the failure.
func (m *Model) handleWorkflowError(msg WorkflowErrorMsg) {
	elapsed := time.Since(m.workflowStartedAt)
	m.workflowRunning = false
	m.workflowPhase = "idle"
	// Mark running agent as error
	for _, a := range m.agentInfos {
		if a.Status == AgentStatusRunning {
			a.Status = AgentStatusError
			a.Error = msg.Error.Error()
			m.logsPanel.AddError(strings.ToLower(a.Name), "Agent failed: "+msg.Error.Error())
			break
		}
	}
	m.logsPanel.AddError("workflow", fmt.Sprintf("Workflow failed after %s: %s", formatDuration(elapsed), msg.Error.Error()))
	m.history.Add(NewSystemBubbleMessage(fmt.Sprintf("Workflow failed after %s: %v", formatDuration(elapsed), msg.Error)))
	m.updateViewport()
}

// handleAgentStreamEvent processes an AgentStreamMsg with sub-cases for each
// event kind (started, tool_use, thinking, chunk, progress, completed, error).
// Returns a tea.Cmd to continue listening for log events.
func (m *Model) handleAgentStreamEvent(msg AgentStreamMsg) tea.Cmd {
	source := msg.Agent
	if source == "" {
		source = "agent"
	}

	switch msg.Kind {
	case "started":
		m.handleStreamStarted(msg, source)
	case "tool_use":
		UpdateAgentActivity(m.agentInfos, msg.Agent, "🔧", msg.Message)
	case "thinking":
		UpdateAgentActivity(m.agentInfos, msg.Agent, "💭", "thinking...")
	case "chunk":
		// Skip chunk events - too noisy
	case "progress":
		m.handleStreamProgress(msg, source)
	case "completed":
		m.handleStreamCompleted(msg, source)
	case "error":
		m.handleStreamError(msg, source)
	}
	return m.listenForLogEvents()
}

// handleStreamStarted processes agent "started" events.
func (m *Model) handleStreamStarted(msg AgentStreamMsg, source string) {
	phase := ""
	if p, ok := msg.Data["phase"].(string); ok {
		phase = p
	}
	model := ""
	if mdl, ok := msg.Data["model"].(string); ok {
		model = mdl
	}
	var maxTimeout time.Duration
	if t, ok := msg.Data["timeout_seconds"].(float64); ok && t > 0 {
		maxTimeout = time.Duration(t) * time.Second
	} else if t, ok := msg.Data["timeout_seconds"].(int); ok && t > 0 {
		maxTimeout = time.Duration(t) * time.Second
	}
	StartAgent(m.agentInfos, msg.Agent, phase, maxTimeout, model)

	if phase != "" {
		details := msg.Message
		if model, ok := msg.Data["model"].(string); ok && model != "" {
			details += fmt.Sprintf(" [%s]", model)
		}
		details = fmt.Sprintf("[%s] %s", phase, details)
		m.logsPanel.AddInfo(source, "▶ "+details)
	}
}

// handleStreamProgress processes agent "progress" events.
func (m *Model) handleStreamProgress(msg AgentStreamMsg, source string) {
	details := msg.Message
	isRetry := false
	if attempt, ok := msg.Data["attempt"].(int); ok && attempt > 0 {
		isRetry = true
		if errMsg, ok := msg.Data["error"].(string); ok {
			details = fmt.Sprintf("retry #%d: %s", attempt, errMsg)
		}
	}
	UpdateAgentActivity(m.agentInfos, msg.Agent, "⟳", details)
	if isRetry {
		m.logsPanel.AddWarn(source, "⟳ "+details)
	}
}

// handleStreamCompleted processes agent "completed" events.
func (m *Model) handleStreamCompleted(msg AgentStreamMsg, source string) {
	tokensIn := extractTokenValue(msg.Data, "tokens_in")
	tokensOut := extractTokenValue(msg.Data, "tokens_out")
	if model, ok := msg.Data["model"].(string); ok && model != "" {
		for _, a := range m.agentInfos {
			if strings.EqualFold(a.Name, msg.Agent) {
				a.Model = model
				break
			}
		}
	}

	found, rejectedIn, rejectedOut := CompleteAgent(m.agentInfos, msg.Agent, tokensIn, tokensOut)
	if found && (rejectedIn > 0 || rejectedOut > 0) {
		m.logsPanel.AddWarn(source, fmt.Sprintf("⚠ Rejected suspicious tokens: in=%d out=%d (raw types: %T / %T)",
			rejectedIn, rejectedOut, msg.Data["tokens_in"], msg.Data["tokens_out"]))
	}

	details := msg.Message
	stats := buildStreamStats(msg.Data, tokensIn, tokensOut)
	if len(stats) > 0 {
		details += " [" + strings.Join(stats, " | ") + "]"
	}
	m.logsPanel.AddSuccess(source, "✓ "+details)

	m.updateLogsPanelTokenStats()
	m.updateTokenPanelStats()
}

// handleStreamError processes agent "error" events.
func (m *Model) handleStreamError(msg AgentStreamMsg, source string) {
	FailAgent(m.agentInfos, msg.Agent, msg.Message)

	details := msg.Message
	var errorInfo []string
	if errType, ok := msg.Data["error_type"].(string); ok && errType != "" {
		errorInfo = append(errorInfo, errType)
	}
	if model, ok := msg.Data["model"].(string); ok && model != "" {
		errorInfo = append(errorInfo, model)
	}
	if phase, ok := msg.Data["phase"].(string); ok {
		errorInfo = append(errorInfo, phase)
	}
	if durationMS, ok := msg.Data["duration_ms"].(int64); ok {
		errorInfo = append(errorInfo, fmt.Sprintf("%dms", durationMS))
	}
	if retries, ok := msg.Data["retries"].(int); ok && retries > 0 {
		errorInfo = append(errorInfo, fmt.Sprintf("%d retries", retries))
	}
	if len(errorInfo) > 0 {
		details += " [" + strings.Join(errorInfo, " | ") + "]"
	}
	m.logsPanel.AddError(source, "✗ "+details)
}

// buildStreamStats builds stats strings from completed event data.
func buildStreamStats(data map[string]interface{}, tokensIn, tokensOut int) []string {
	var stats []string
	if model, ok := data["model"].(string); ok && model != "" {
		stats = append(stats, model)
	}
	if tokensIn > 0 || tokensOut > 0 {
		stats = append(stats, fmt.Sprintf("↑%d ↓%d tok", tokensIn, tokensOut))
	}
	if durationMS, ok := data["duration_ms"].(int64); ok {
		if durationMS >= 1000 {
			stats = append(stats, fmt.Sprintf("%.1fs", float64(durationMS)/1000))
		} else {
			stats = append(stats, fmt.Sprintf("%dms", durationMS))
		}
	}
	if toolCalls, ok := data["tool_calls"].(int); ok && toolCalls > 0 {
		stats = append(stats, fmt.Sprintf("%d tools", toolCalls))
	}
	return stats
}

// handleWorkflowMsg dispatches workflow-related messages.
// Returns (cmd, handled). Caller should append cmd to batch.
func (m *Model) handleWorkflowMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case WorkflowUpdateMsg:
		m.workflowState = msg.State
		m.tasksPanel.SetState(msg.State)
		m.updateQuorumPanel(msg.State)
		return nil, true

	case TaskUpdateMsg:
		if m.workflowState != nil && m.workflowState.Tasks != nil {
			if task, ok := m.workflowState.Tasks[msg.TaskID]; ok {
				task.Status = msg.Status
				m.tasksPanel.SetState(m.workflowState)
				m.updateQuorumPanel(m.workflowState)
				m.updateViewport()
			}
		}
		return nil, true

	case PhaseUpdateMsg:
		if m.workflowState != nil {
			m.workflowState.CurrentPhase = msg.Phase
			m.tasksPanel.SetState(m.workflowState)
			m.updateQuorumPanel(m.workflowState)
			m.updateViewport()
		}
		return nil, true

	case BatchedEventsMsg:
		var cmds []tea.Cmd
		for _, evt := range msg.Events {
			newModel, innerCmd := (*m).Update(evt)
			*m = newModel.(Model)
			if innerCmd != nil {
				cmds = append(cmds, innerCmd)
			}
		}
		m.updateViewport()
		return tea.Batch(cmds...), true

	case ActiveWorkflowLoadMsg:
		if msg.State != nil {
			m.workflowState = msg.State
			m.tasksPanel.SetState(msg.State)
			m.updateQuorumPanel(msg.State)
			prompt := msg.State.Prompt
			if len(prompt) > 50 {
				prompt = prompt[:47] + "..."
			}
			m.history.Add(NewSystemBubbleMessage(fmt.Sprintf("Session restored: %s @%s\n%q",
				strings.ToUpper(string(msg.State.Status)),
				msg.State.CurrentPhase,
				prompt)))
			m.updateViewport()
		}
		return nil, true

	case WorkflowStartedMsg:
		cmd := m.handleWorkflowStarted(msg)
		return cmd, true

	case WorkflowCompletedMsg:
		m.handleWorkflowCompleted(msg)
		return nil, true

	case WorkflowErrorMsg:
		m.handleWorkflowError(msg)
		return nil, true

	case WorkflowLogMsg:
		switch msg.Level {
		case "success":
			m.logsPanel.AddSuccess(msg.Source, msg.Message)
		case "error":
			m.logsPanel.AddError(msg.Source, msg.Message)
		case "warn":
			m.logsPanel.AddWarn(msg.Source, msg.Message)
		case "debug":
			m.logsPanel.AddDebug(msg.Source, msg.Message)
		default:
			m.logsPanel.AddInfo(msg.Source, msg.Message)
		}
		return m.listenForLogEvents(), true

	case AgentStreamMsg:
		cmd := m.handleAgentStreamEvent(msg)
		return cmd, true
	}
	return nil, false
}
