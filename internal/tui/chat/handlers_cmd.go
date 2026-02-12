package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

// handleCommandLoad handles the "/load" command: loads a workflow by ID or lists available workflows.
func (m Model) handleCommandLoad(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow already running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	ctx := context.Background()

	// If no args, show available workflows to select from
	if len(args) == 0 {
		workflows, err := m.runner.ListWorkflows(ctx)
		if err != nil {
			addSystem(fmt.Sprintf("Error listing workflows: %v", err))
			m.updateViewport()
			return m, nil
		}
		if len(workflows) == 0 {
			addSystem("No workflows found. Use '/analyze <prompt>' to start one.")
			m.updateViewport()
			return m, nil
		}
		var sb strings.Builder
		for i, wf := range workflows {
			marker := "  "
			if wf.IsActive {
				marker = "> "
			}
			prompt := wf.Prompt
			if len(prompt) > 60 {
				prompt = prompt[:57] + "..."
			}
			status := strings.ToUpper(string(wf.Status))
			sb.WriteString(fmt.Sprintf("%s#%d  %-11s @%-8s  %s\n", marker, i+1, status, wf.CurrentPhase, wf.WorkflowID))
			sb.WriteString(fmt.Sprintf("    %q\n\n", prompt))
		}
		sb.WriteString("> = active  |  /load <ID> to switch")
		addSystem(sb.String())
		m.updateViewport()
		return m, nil
	}

	// Load the specified workflow
	workflowID := args[0]
	state, err := m.runner.LoadWorkflow(ctx, workflowID)
	if err != nil {
		addSystem(fmt.Sprintf("Error loading workflow: %v", err))
		m.updateViewport()
		return m, nil
	}

	// Update internal state
	m.workflowState = state
	m.tasksPanel.SetState(state)
	m.updateQuorumPanel(state)

	// Show success message with workflow details
	var sb strings.Builder

	// Status with icon
	statusIcon := "○"
	switch state.Status {
	case core.WorkflowStatusCompleted:
		statusIcon = "●"
	case core.WorkflowStatusRunning:
		statusIcon = "◐"
	case core.WorkflowStatusFailed:
		statusIcon = "✗"
	}
	sb.WriteString(fmt.Sprintf("%s Loaded  |  %s @%s\n\n", statusIcon, strings.ToUpper(string(state.Status)), state.CurrentPhase))

	// Prompt
	if state.Prompt != "" {
		prompt := state.Prompt
		if len(prompt) > 70 {
			prompt = prompt[:67] + "..."
		}
		sb.WriteString(fmt.Sprintf("%q\n\n", prompt))
	}

	// Metrics inline
	var info []string
	if state.Metrics != nil && state.Metrics.ConsensusScore > 0 {
		info = append(info, fmt.Sprintf("Consensus: %.0f%%", state.Metrics.ConsensusScore*100))
	}
	if len(state.Tasks) > 0 {
		info = append(info, fmt.Sprintf("Issues: %d", len(state.Tasks)))
	}
	if len(info) > 0 {
		sb.WriteString(strings.Join(info, "  |  ") + "\n\n")
	}

	// Next action
	sb.WriteString("Next: ")
	switch state.CurrentPhase {
	case core.PhaseAnalyze:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("/plan")
		} else {
			sb.WriteString("/analyze")
		}
	case core.PhasePlan:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("/execute")
		} else {
			sb.WriteString("/plan")
		}
	case core.PhaseExecute:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("Done!")
		} else {
			sb.WriteString("/execute")
		}
	default:
		sb.WriteString("/analyze <prompt>")
	}
	sb.WriteString("  |  /status for details")
	addSystem(sb.String())
	m.updateViewport()
	return m, nil // Important: return modified m with workflowState set
}

// handleCommandNew handles the "/new" command: deactivates, archives, or purges workflows.
func (m Model) handleCommandNew(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow is running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	ctx := context.Background()

	// Parse flags from args
	archive := false
	purge := false
	for _, arg := range args {
		switch arg {
		case "--archive", "-a":
			archive = true
		case "--purge", "-p":
			purge = true
		}
	}

	// Handle purge (most destructive)
	if purge {
		deleted, err := m.runner.PurgeAllWorkflows(ctx)
		if err != nil {
			addSystem(fmt.Sprintf("Error purging workflows: %v", err))
			m.updateViewport()
			return m, nil
		}
		m.workflowState = nil
		m.tasksPanel.SetState(nil)
		m.updateQuorumPanel(nil)
		addSystem(fmt.Sprintf("Purged %d workflow(s). All state deleted.\nUse '/analyze <prompt>' to start fresh.", deleted))
		m.updateViewport()
		return m, nil
	}

	// Handle archive
	if archive {
		archived, err := m.runner.ArchiveWorkflows(ctx)
		if err != nil {
			addSystem(fmt.Sprintf("Error archiving workflows: %v", err))
			m.updateViewport()
			return m, nil
		}
		if err := m.runner.DeactivateWorkflow(ctx); err != nil {
			addSystem(fmt.Sprintf("Error deactivating workflow: %v", err))
			m.updateViewport()
			return m, nil
		}
		m.workflowState = nil
		m.tasksPanel.SetState(nil)
		m.updateQuorumPanel(nil)
		msg := "Ready for new task."
		if archived > 0 {
			msg = fmt.Sprintf("Archived %d completed workflow(s). %s", archived, msg)
		}
		addSystem(msg + "\nUse '/analyze <prompt>' to start a new workflow.")
		m.updateViewport()
		return m, nil
	}

	// Default: just deactivate
	if err := m.runner.DeactivateWorkflow(ctx); err != nil {
		addSystem(fmt.Sprintf("Error deactivating workflow: %v", err))
		m.updateViewport()
		return m, nil
	}
	m.workflowState = nil
	m.tasksPanel.SetState(nil)
	m.updateQuorumPanel(nil)
	addSystem("Workflow deactivated. Ready for new task.\nUse '/analyze <prompt>' to start a new workflow.\nPrevious workflows available via '/workflows'.")
	m.updateViewport()
	return m, nil
}

// handleCommandDelete handles the "/delete" command: deletes a workflow by ID.
func (m Model) handleCommandDelete(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Cannot delete while workflow is running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}
	if len(args) == 0 {
		addSystem("Usage: /delete <workflow-id>\nUse /workflows to see available IDs.")
		m.updateViewport()
		return m, nil
	}

	ctx := context.Background()
	workflowID := args[0]

	// Load workflow to verify it exists and check status
	wf, err := m.runner.LoadWorkflow(ctx, workflowID)
	if err != nil || wf == nil {
		addSystem(fmt.Sprintf("Workflow not found: %s", workflowID))
		m.updateViewport()
		return m, nil
	}

	// Prevent deletion of running workflows
	if wf.Status == core.WorkflowStatusRunning {
		addSystem("Cannot delete running workflow. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	// Delete the workflow
	if err := m.runner.DeleteWorkflow(ctx, workflowID); err != nil {
		addSystem(fmt.Sprintf("Error deleting workflow: %v", err))
		m.updateViewport()
		return m, nil
	}

	// Clear state if we just deleted the active workflow
	if m.workflowState != nil && string(m.workflowState.WorkflowID) == workflowID {
		m.workflowState = nil
		m.tasksPanel.SetState(nil)
		m.updateQuorumPanel(nil)
	}

	addSystem(fmt.Sprintf("Workflow %s deleted.", workflowID))
	m.updateViewport()
	return m, nil
}

// handleCommandPlan handles the "/plan" command: continues planning from active workflow or starts a new one.
func (m Model) handleCommandPlan(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow already running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	// If no args, continue from active workflow (after /analyze)
	if len(args) == 0 {
		// Try to load active workflow state if not in memory
		if m.workflowState == nil {
			if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
				m.workflowState = state
				m.tasksPanel.SetState(state)
				m.updateQuorumPanel(state)
			}
		}

		// Check if we can continue to plan phase:
		// - Already in plan phase, OR
		// - Analyze phase completed (status=completed with analyze as current phase)
		canContinue := false
		if m.workflowState != nil {
			if m.workflowState.CurrentPhase == core.PhasePlan {
				canContinue = true
			} else if m.workflowState.CurrentPhase == core.PhaseAnalyze && m.workflowState.Status == core.WorkflowStatusCompleted {
				// Analyze completed - can advance to plan
				canContinue = true
			}
		}

		if canContinue {
			addSystem("Continuing to planning phase from active workflow...")
			m.updateViewport()
			return m, m.runPlanPhase()
		}
		addSystem("No active workflow to continue. Use '/plan <prompt>' to start new or '/analyze' first.")
		m.updateViewport()
		return m, nil
	}

	// With args, start new workflow
	prompt := strings.Join(args, " ")
	m.updateViewport()
	return m, m.runWorkflow(prompt)
}

// handleCommandExecute handles the "/execute" command: continues to execution phase from active workflow.
func (m Model) handleCommandExecute(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow already running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	// Try to load active workflow state if not in memory
	if m.workflowState == nil {
		if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
			m.workflowState = state
			m.tasksPanel.SetState(state)
			m.updateQuorumPanel(state)
		}
	}

	// Check if we can continue to execute phase:
	// - Already in execute phase, OR
	// - Plan phase completed (status=completed with plan as current phase), OR
	// - Tasks exist (even if plan status is "failed" - tasks may have been created successfully)
	canContinue := false
	needsStateRepair := false
	if m.workflowState != nil {
		if m.workflowState.CurrentPhase == core.PhaseExecute {
			canContinue = true
		} else if m.workflowState.CurrentPhase == core.PhasePlan && m.workflowState.Status == core.WorkflowStatusCompleted {
			// Plan completed - can advance to execute
			canContinue = true
		} else if len(m.workflowState.Tasks) > 0 {
			// Tasks exist! Even if plan "failed", we can execute the existing tasks.
			// This handles the case where task files were created but manifest parsing failed.
			canContinue = true
			needsStateRepair = true
			addSystem(fmt.Sprintf("Found %d existing tasks. Recovering workflow state...", len(m.workflowState.Tasks)))
		}
	}

	if canContinue {
		if needsStateRepair {
			// Repair the state before executing
			m.workflowState.Status = core.WorkflowStatusRunning
			m.workflowState.CurrentPhase = core.PhaseExecute
			m.workflowState.UpdatedAt = time.Now()
			// Add a checkpoint to indicate plan is complete
			m.workflowState.Checkpoints = append(m.workflowState.Checkpoints, core.Checkpoint{
				ID:        fmt.Sprintf("cp-repair-%d", time.Now().UnixNano()),
				Type:      "phase_complete",
				Phase:     core.PhasePlan,
				Timestamp: time.Now(),
			})
			// Save the repaired state
			if err := m.runner.SaveState(context.Background(), m.workflowState); err != nil {
				addSystem(fmt.Sprintf("Warning: Failed to save repaired state: %v", err))
			} else {
				addSystem("Workflow state repaired successfully.")
			}
		}
		addSystem("Continuing to execution phase from active workflow...")
		m.updateViewport()
		return m, m.runExecutePhase()
	}
	addSystem("No active workflow to execute. Use '/plan' first.")
	m.updateViewport()
	return m, nil
}

// handleCommandReplan handles the "/replan" command: clears existing issues and regenerates the plan.
func (m Model) handleCommandReplan(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow already running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	// Try to load active workflow state if not in memory
	if m.workflowState == nil {
		if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
			m.workflowState = state
			m.tasksPanel.SetState(state)
			m.updateQuorumPanel(state)
		}
	}

	if m.workflowState == nil {
		addSystem("No active workflow to replan. Use '/analyze' first.")
		m.updateViewport()
		return m, nil
	}

	// Get additional context if provided
	additionalContext := ""
	if len(args) > 0 {
		additionalContext = strings.Join(args, " ")
	}

	if additionalContext != "" {
		addSystem(fmt.Sprintf("Replanning with additional context (%d chars)...", len(additionalContext)))
	} else {
		addSystem("Replanning: clearing existing issues and regenerating...")
	}
	m.updateViewport()
	return m, m.runReplanPhase(additionalContext)
}

// handleCommandUsePlan handles the "/useplan", "/up", "/useplans" commands: loads existing task files from filesystem.
func (m Model) handleCommandUsePlan(args []string, addSystem func(string)) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		addSystem("Workflow runner not configured")
		m.updateViewport()
		return m, nil
	}
	if m.workflowRunning {
		addSystem("Workflow already running. Use /cancel first.")
		m.updateViewport()
		return m, nil
	}

	// Try to load active workflow state if not in memory
	if m.workflowState == nil {
		if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
			m.workflowState = state
			m.tasksPanel.SetState(state)
			m.updateQuorumPanel(state)
		}
	}

	if m.workflowState == nil {
		addSystem("No active workflow found. Use '/analyze' first.")
		m.updateViewport()
		return m, nil
	}

	addSystem("Loading existing task files from filesystem (skipping agent call)...")
	m.updateViewport()
	return m, m.runUsePlanPhase()
}

// handleCommandUI handles UI-related commands: help, clear, model, agent, copy, logs, explorer, theme.
// Returns (model, cmd, handled).
func (m Model) handleCommandUI(cmd *Command, args []string, addSystem func(string)) (tea.Model, tea.Cmd, bool) {
	switch cmd.Name {
	case "help":
		var helpText string
		if len(args) > 0 {
			helpText = m.commands.Help(args[0])
		} else {
			helpText = m.commands.Help("")
		}
		addSystem(helpText)
		m.updateViewport()
		return m, nil, true

	case "clear":
		m.history.Clear()
		m.updateViewport()
		return m, nil, true

	case "model":
		if len(args) > 0 {
			m.currentModel = args[0]
			addSystem("Model: " + m.currentModel)
		} else {
			modelInfo := m.currentModel
			if modelInfo == "" {
				if models, ok := m.agentModels[m.currentAgent]; ok && len(models) > 0 {
					modelInfo = models[0] + " (default)"
				} else {
					modelInfo = "(unknown)"
				}
			}
			addSystem("Current model: " + modelInfo)
		}
		m.updateViewport()
		return m, nil, true

	case "agent":
		if len(args) > 0 {
			m.currentAgent = args[0]
			m.currentModel = ""
			addSystem("Agent: " + m.currentAgent + " (using default model)")
		} else {
			modelInfo := m.currentModel
			if modelInfo == "" {
				modelInfo = "default"
			}
			addSystem(fmt.Sprintf("Current agent: %s (model: %s)", m.currentAgent, modelInfo))
		}
		m.updateViewport()
		return m, nil, true

	case "copy":
		newModel, _, _ := m.copyLastResponse()
		return newModel.(Model), nil, true

	case "copyall":
		newModel, _, _ := m.copyConversation()
		return newModel.(Model), nil, true

	case "logs":
		m.showLogs = !m.showLogs
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		m.updateViewport()
		return m, nil, true

	case "clearlogs":
		m.logsPanel.Clear()
		addSystem("Logs cleared")
		m.updateViewport()
		return m, nil, true

	case "copylogs":
		newModel, _, _ := m.copyLogsToClipboard()
		m = newModel.(Model)
		m.updateViewport()
		return m, nil, true

	case "explorer":
		m.showExplorer = !m.showExplorer
		if m.showExplorer {
			m.explorerFocus = true
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
			addSystem("File explorer opened (arrows to navigate, Esc to return)")
		} else {
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			addSystem("File explorer closed")
		}
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		m.updateViewport()
		return m, nil, true

	case "theme":
		var themeName string
		if len(args) > 0 {
			themeName = strings.ToLower(args[0])
		} else {
			if m.darkTheme {
				themeName = "light"
			} else {
				themeName = "dark"
			}
		}
		switch themeName {
		case "dark":
			m.darkTheme = true
			tui.SetColorScheme(tui.DarkScheme)
			applyDarkTheme()
			ApplyDarkThemeMessages()
			m.messageStyles = NewMessageStyles(m.viewport.Width)
			addSystem("Theme: dark")
		case "light":
			m.darkTheme = false
			tui.SetColorScheme(tui.LightScheme)
			applyLightTheme()
			ApplyLightThemeMessages()
			m.messageStyles = NewMessageStyles(m.viewport.Width)
			addSystem("Theme: light")
		default:
			addSystem("Usage: /theme [dark|light]")
		}
		m.updateViewport()
		return m, nil, true
	}
	return m, nil, false
}

// handleCommandWorkflowOps handles workflow operation commands: status, workflows, cancel, analyze, run, retry.
// Returns (model, cmd, handled).
func (m Model) handleCommandWorkflowOps(cmd *Command, args []string, addSystem func(string)) (tea.Model, tea.Cmd, bool) {
	switch cmd.Name {
	case "status":
		if m.workflowState != nil {
			status := strings.TrimPrefix(formatWorkflowStatus(m.workflowState), "/status\n\n")
			addSystem(status)
		} else {
			var hint string
			if m.runner != nil {
				ctx := context.Background()
				if workflows, err := m.runner.ListWorkflows(ctx); err == nil {
					for _, wf := range workflows {
						if wf.IsActive {
							hint = fmt.Sprintf("\n\nTip: /load %s to continue your previous session", wf.WorkflowID)
							break
						}
					}
				}
			}
			addSystem("No workflow loaded in this session." + hint)
		}
		m.updateViewport()
		return m, nil, true

	case "workflows":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil, true
		}
		ctx := context.Background()
		workflows, err := m.runner.ListWorkflows(ctx)
		if err != nil {
			addSystem(fmt.Sprintf("Error listing workflows: %v", err))
			m.updateViewport()
			return m, nil, true
		}
		if len(workflows) == 0 {
			addSystem("No workflows found. Use '/analyze <prompt>' to start one.")
			m.updateViewport()
			return m, nil, true
		}
		var sb strings.Builder
		for i, wf := range workflows {
			marker := "  "
			if wf.IsActive {
				marker = "> "
			}
			prompt := wf.Prompt
			if len(prompt) > 60 {
				prompt = prompt[:57] + "..."
			}
			status := strings.ToUpper(string(wf.Status))
			sb.WriteString(fmt.Sprintf("%s#%d  %-11s @%-8s  %s\n", marker, i+1, status, wf.CurrentPhase, wf.WorkflowID))
			sb.WriteString(fmt.Sprintf("    %q\n\n", prompt))
		}
		sb.WriteString("> = active  |  /load <ID> to switch")
		addSystem(sb.String())
		m.updateViewport()
		return m, nil, true

	case "cancel":
		if m.controlPlane != nil && m.workflowRunning {
			m.controlPlane.Cancel()
			m.workflowRunning = false
			addSystem("Workflow cancelled")
		} else {
			addSystem("No active workflow")
		}
		m.updateViewport()
		return m, nil, true

	case "analyze":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil, true
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil, true
		}
		if len(args) == 0 {
			addSystem("Usage: /analyze <prompt>")
			m.updateViewport()
			return m, nil, true
		}
		prompt := strings.Join(args, " ")
		m.updateViewport()
		return m, m.runAnalyze(prompt), true

	case "run":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil, true
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil, true
		}
		if len(args) == 0 {
			addSystem("Usage: /run <prompt>")
			m.updateViewport()
			return m, nil, true
		}
		prompt := strings.Join(args, " ")
		m.updateViewport()
		return m, m.runWorkflow(prompt), true

	case "retry":
		if m.controlPlane == nil {
			addSystem("No control plane")
			m.updateViewport()
			return m, nil, true
		}
		if len(args) > 0 {
			m.controlPlane.RetryTask(core.TaskID(args[0]))
			addSystem(fmt.Sprintf("Retrying: %s", args[0]))
		} else {
			addSystem("Usage: /retry <task_id>")
		}
		m.updateViewport()
		return m, nil, true
	}
	return m, nil, false
}
