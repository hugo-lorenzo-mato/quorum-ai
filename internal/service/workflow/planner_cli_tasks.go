package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// ComprehensiveTaskManifest is the response from the comprehensive planning prompt.
// The CLI generates all task files directly and returns this manifest.
type ComprehensiveTaskManifest struct {
	Tasks           []TaskManifestItem `json:"tasks"`
	ExecutionLevels [][]string         `json:"execution_levels"` // Tasks grouped by parallelization level
}

// TaskManifestItem represents a task in the manifest.
type TaskManifestItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	File         string   `json:"file"` // Path to the task specification file
	Dependencies []string `json:"dependencies"`
	Complexity   string   `json:"complexity"`
	CLI          string   `json:"cli"`
}

// runCLIGeneratedTaskPlanning executes the comprehensive single-call planning flow.
// The planning CLI receives ALL context and in a SINGLE session:
// 1. Analyzes the work and divides it into tasks
// 2. Assigns optimal CLI/agent to each task based on strengths
// 3. Defines dependencies and parallelization opportunities
// 4. Writes ALL ultraexhaustive task specification files directly to disk
// 5. Returns a manifest of what was created
func (p *Planner) runCLIGeneratedTaskPlanning(ctx context.Context, wctx *Context) error {
	wctx.State.CurrentPhase = core.PhasePlan
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhasePlan)
		wctx.Output.Log("info", "planner", "Starting comprehensive single-call task planning")
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhasePlan, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Get consolidated analysis - this is the COMPLETE context for planning
	analysis := GetConsolidatedAnalysis(wctx.State)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found")
	}

	wctx.Logger.Info("starting comprehensive planning",
		"analysis_size", len(analysis),
	)

	// Determine tasks directory
	var tasksDir string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if err := wctx.Report.EnsureTasksDir(); err != nil {
			wctx.Logger.Warn("failed to create tasks directory", "error", err)
		}
		tasksDir = wctx.Report.TasksDir()
	} else {
		tasksDir = ".quorum/tasks"
		if err := os.MkdirAll(tasksDir, 0o750); err != nil {
			return fmt.Errorf("creating tasks directory: %w", err)
		}
	}

	// Make tasks directory absolute relative to project root
	if !filepath.IsAbs(tasksDir) {
		tasksDir = filepath.Join(wctx.ProjectRoot, tasksDir)
	}

	// Collect available agent information
	availableAgents := p.collectAgentInfo(ctx, wctx)

	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", fmt.Sprintf(
			"Planning with %d available agents, output to: %s",
			len(availableAgents), tasksDir,
		))
	}

	// Execute comprehensive planning - SINGLE CALL
	manifest, err := p.executeComprehensivePlanning(ctx, wctx, ComprehensivePlanParams{
		Prompt:               GetEffectivePrompt(wctx.State),
		ConsolidatedAnalysis: analysis,
		AvailableAgents:      availableAgents,
		TasksDir:             tasksDir,
		NamingConvention:     "{id}-{name}.md",
	})
	if err != nil {
		return fmt.Errorf("comprehensive planning: %w", err)
	}

	if len(manifest.Tasks) == 0 {
		return fmt.Errorf("planning produced no tasks")
	}

	wctx.Logger.Info("comprehensive planning completed",
		"task_count", len(manifest.Tasks),
		"execution_levels", len(manifest.ExecutionLevels),
	)

	// Verify task files were created and collect info
	var verifiedTasks int
	var totalFileSize int64
	for _, item := range manifest.Tasks {
		if info, err := os.Stat(item.File); err == nil {
			verifiedTasks++
			totalFileSize += info.Size()
		} else {
			wctx.Logger.Warn("task file not found",
				"task_id", item.ID,
				"expected_path", item.File,
			)
		}
	}

	wctx.Logger.Info("task files verified",
		"verified", verifiedTasks,
		"total", len(manifest.Tasks),
		"total_size_kb", totalFileSize/1024,
	)

	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", fmt.Sprintf(
			"Verified %d/%d task files (total: %dKB)",
			verifiedTasks, len(manifest.Tasks), totalFileSize/1024,
		))
	}

	// Create tasks from manifest
	tasks := p.createTasksFromManifest(ctx, wctx, manifest)

	// Add tasks to state and DAG
	for _, task := range tasks {
		wctx.State.Tasks[task.ID] = &core.TaskState{
			ID:           task.ID,
			Phase:        task.Phase,
			Name:         task.Name,
			Status:       task.Status,
			CLI:          task.CLI,
			Model:        task.Model,
			Dependencies: task.Dependencies,
		}
		wctx.State.TaskOrder = append(wctx.State.TaskOrder, task.ID)
		_ = p.dag.AddTask(task)
	}

	// Notify output that tasks have been created
	if wctx.Output != nil {
		wctx.Output.WorkflowStateUpdated(wctx.State)
	}

	// Build dependency graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			_ = p.dag.AddDependency(task.ID, dep)
		}
	}

	// Validate DAG and get execution levels
	dagState, err := p.dag.Build()
	if err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	// Write execution graph
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if ds, ok := dagState.(*service.DAGState); ok {
			p.writeExecutionGraph(wctx, tasks, ds)
		}
	}

	wctx.Logger.Info("plan phase completed",
		"task_count", len(tasks),
		"verified_files", verifiedTasks,
	)

	if wctx.Output != nil {
		wctx.Output.Log("success", "planner", fmt.Sprintf(
			"Planning completed: %d tasks with ultraexhaustive specifications",
			len(tasks),
		))
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhasePlan, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return p.stateSaver.Save(ctx, wctx.State)
}

// collectAgentInfo gathers information about available agents for task assignment.
func (p *Planner) collectAgentInfo(ctx context.Context, wctx *Context) []AgentInfo {
	// Get agents enabled for execute phase
	agentNames := wctx.Agents.AvailableForPhaseWithConfig(ctx, "execute", wctx.Config.ProjectAgentPhases)
	if len(agentNames) == 0 {
		// Fallback to all available agents
		agentNames = wctx.Agents.Available(ctx)
	}

	agents := make([]AgentInfo, 0, len(agentNames))
	for _, name := range agentNames {
		agent, err := wctx.Agents.Get(name)
		if err != nil {
			continue
		}

		caps := agent.Capabilities()
		info := AgentInfo{
			Name:         name,
			Model:        ResolvePhaseModel(wctx.Config, name, core.PhaseExecute, ""),
			Strengths:    getAgentStrengths(name),
			Capabilities: formatCapabilities(caps),
		}
		agents = append(agents, info)
	}

	return agents
}

// getAgentStrengths returns a description of agent strengths based on CLI type.
func getAgentStrengths(agentName string) string {
	// Map of known agent strengths
	strengths := map[string]string{
		"claude": "Excellent at complex reasoning, code generation, architectural decisions, and detailed documentation. " +
			"Best for tasks requiring deep understanding, nuanced analysis, or creative problem-solving. " +
			"Strong at multi-file refactoring and system design.",
		"codex": "Optimized for code completion and generation. " +
			"Excellent for straightforward implementation tasks, boilerplate code, and following established patterns. " +
			"Fast and efficient for well-defined coding tasks.",
		"gemini": "Strong at understanding large codebases and context. " +
			"Good for analysis tasks, code review, and understanding complex systems. " +
			"Effective at synthesizing information from multiple sources.",
		"copilot": "Fast and efficient for common coding patterns. " +
			"Good for standard implementations, bug fixes, and incremental changes. " +
			"Integrates well with existing code style.",
		"opencode": "AI coding agent with MCP support and project context awareness (Git, LSP). " +
			"Excellent at autonomous coding tasks and understanding project-wide dependencies. " +
			"Supports local execution via Ollama.",
	}

	// Check for exact match first
	if s, ok := strengths[agentName]; ok {
		return s
	}

	// Check for prefix match (e.g., "copilot-claude" -> "copilot")
	for prefix, s := range strengths {
		if strings.HasPrefix(agentName, prefix) {
			return s
		}
	}

	return "General-purpose AI agent capable of code generation and analysis."
}

// formatCapabilities formats agent capabilities as a readable string.
func formatCapabilities(caps core.Capabilities) string {
	var parts []string
	if caps.SupportsJSON {
		parts = append(parts, "JSON output")
	}
	if caps.SupportsStreaming {
		parts = append(parts, "streaming")
	}
	if caps.SupportsTools {
		parts = append(parts, "tool use")
	}
	if caps.SupportsImages {
		parts = append(parts, "image analysis")
	}
	if caps.MaxContextTokens > 0 {
		parts = append(parts, fmt.Sprintf("%dK context", caps.MaxContextTokens/1000))
	}
	if len(parts) == 0 {
		return "standard capabilities"
	}
	return strings.Join(parts, ", ")
}

// executeComprehensivePlanning runs the single comprehensive planning call.
func (p *Planner) executeComprehensivePlanning(
	ctx context.Context,
	wctx *Context,
	params ComprehensivePlanParams,
) (*ComprehensiveTaskManifest, error) {
	agent, err := wctx.Agents.Get(wctx.Config.DefaultAgent)
	if err != nil {
		return nil, fmt.Errorf("getting plan agent: %w", err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(wctx.Config.DefaultAgent)
	if err := limiter.Acquire(); err != nil {
		return nil, fmt.Errorf("rate limit for planning: %w", err)
	}

	// Render comprehensive planning prompt
	prompt, err := wctx.Prompts.RenderPlanComprehensive(params)
	if err != nil {
		return nil, fmt.Errorf("rendering comprehensive plan prompt: %w", err)
	}

	agentName := wctx.Config.DefaultAgent
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhasePlan, "")
	startTime := time.Now()

	wctx.Logger.Info("executing comprehensive planning",
		"agent", agentName,
		"model", model,
		"prompt_size", len(prompt),
		"analysis_size", len(params.ConsolidatedAnalysis),
		"available_agents", len(params.AvailableAgents),
	)

	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Comprehensive task planning (single call)", map[string]interface{}{
			"phase":             "plan",
			"model":             model,
			"timeout_seconds":   int(wctx.Config.PhaseTimeouts.Plan.Seconds()),
			"analysis_size":     len(params.ConsolidatedAnalysis),
			"available_agents":  len(params.AvailableAgents),
			"tasks_dir":         params.TasksDir,
			"naming_convention": params.NamingConvention,
		})
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON, // Response should be JSON manifest only
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Plan,
			Phase:   core.PhasePlan,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "plan",
				"model":       model,
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return nil, err
	}

	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, "Comprehensive planning completed", map[string]interface{}{
			"phase":       "plan",
			"model":       result.Model,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": durationMS,
		})
	}

	// Update metrics
	wctx.UpdateMetrics(func(m *core.StateMetrics) {
		m.TotalTokensIn += result.TokensIn
		m.TotalTokensOut += result.TokensOut
	})

	// Generate manifest from filesystem (robust approach)
	// The agent writes task files to disk; we scan them instead of parsing JSON output
	manifest, err := generateManifestFromFilesystem(params.TasksDir)
	if err != nil {
		wctx.Logger.Warn("filesystem manifest generation failed, trying output parse",
			"tasks_dir", params.TasksDir,
			"error", err,
		)
		// Fallback: try parsing agent output (backward compatibility)
		manifest, err = parseComprehensiveManifest(result.Output)
		if err != nil {
			wctx.Logger.Error("failed to generate manifest from both filesystem and output",
				"tasks_dir", params.TasksDir,
				"output_preview", truncateForLog(result.Output, 500),
				"error", err,
			)
			return nil, fmt.Errorf("manifest generation failed: filesystem scan and output parse both failed")
		}
		wctx.Logger.Info("manifest generated from output parse (fallback)")
	} else {
		wctx.Logger.Info("manifest generated from filesystem",
			"tasks_count", len(manifest.Tasks),
			"levels_count", len(manifest.ExecutionLevels),
		)
	}

	return manifest, nil
}

// parseComprehensiveManifest parses the JSON manifest from CLI output.
func parseComprehensiveManifest(output string) (*ComprehensiveTaskManifest, error) {
	// Try direct parse first
	var manifest ComprehensiveTaskManifest
	if err := json.Unmarshal([]byte(output), &manifest); err == nil && len(manifest.Tasks) > 0 {
		return &manifest, nil
	}

	// Try to extract JSON from markdown code block or mixed content
	extracted := extractJSON(output)
	if extracted != "" && extracted != output {
		if err := json.Unmarshal([]byte(extracted), &manifest); err == nil && len(manifest.Tasks) > 0 {
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("failed to parse task manifest from output (length: %d)", len(output))
}

// createTasksFromManifest creates core.Task objects from the manifest.
func (p *Planner) createTasksFromManifest(ctx context.Context, wctx *Context, manifest *ComprehensiveTaskManifest) []*core.Task {
	tasks := make([]*core.Task, 0, len(manifest.Tasks))

	for _, item := range manifest.Tasks {
		cli := resolveTaskAgent(ctx, wctx, item.CLI)
		task := &core.Task{
			ID:          core.TaskID(item.ID),
			Name:        item.Name,
			Description: fmt.Sprintf("See specification file: %s", item.File),
			Phase:       core.PhaseExecute,
			Status:      core.TaskStatusPending,
			CLI:         cli,
		}

		for _, dep := range item.Dependencies {
			task.Dependencies = append(task.Dependencies, core.TaskID(dep))
		}

		tasks = append(tasks, task)
	}

	return tasks
}
