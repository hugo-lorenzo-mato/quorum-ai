package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// RefinerConfig configures the refiner (prompt refinement) phase.
type RefinerConfig struct {
	Enabled bool
	Agent   string
	// Model is resolved from AgentPhaseModels[Agent][optimize] at runtime.
}

// Refiner runs the prompt refinement phase.
type Refiner struct {
	config RefinerConfig
}

// NewRefiner creates a new refiner.
func NewRefiner(config RefinerConfig) *Refiner {
	return &Refiner{config: config}
}

// Run executes the refine phase (prompt refinement).
func (r *Refiner) Run(ctx context.Context, wctx *Context) error {
	// Write original prompt report (always, even if refinement is disabled)
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteOriginalPrompt(wctx.State.Prompt); reportErr != nil {
			wctx.Logger.Warn("failed to write original prompt report", "error", reportErr)
		}
	}

	// Skip if refinement is disabled
	if !r.config.Enabled {
		wctx.Logger.Info("prompt refinement disabled, skipping phase")
		if wctx.Output != nil {
			wctx.Output.Log("info", "refiner", "Refinement disabled, skipping phase")
		}
		return nil
	}

	wctx.Logger.Info("starting refine phase", "workflow_id", wctx.State.WorkflowID)

	wctx.State.CurrentPhase = core.PhaseRefine
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseRefine)
		wctx.Output.Log("info", "refiner", "Starting prompt refinement phase")
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseRefine, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Skip actual refinement in dry-run mode
	if wctx.Config.DryRun {
		wctx.Logger.Info("dry-run mode: skipping actual refinement")
		if wctx.Output != nil {
			wctx.Output.Log("info", "refiner", "Dry-run mode: using original prompt")
		}
		wctx.State.OptimizedPrompt = wctx.State.Prompt // Use original as-is
		return wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseRefine, true)
	}

	// Get the refiner agent
	agentName := r.config.Agent
	if agentName == "" {
		return fmt.Errorf("phases.analyze.refiner.agent is not configured. " +
			"Please set 'phases.analyze.refiner.agent' in your .quorum/config.yaml file, " +
			"or disable the refiner with 'phases.analyze.refiner.enabled: false'")
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return fmt.Errorf("getting refiner agent %s: %w", agentName, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return fmt.Errorf("rate limit for refiner: %w", err)
	}

	// Render refinement prompt
	prompt, err := wctx.Prompts.RenderRefinePrompt(RefinePromptParams{
		OriginalPrompt: wctx.State.Prompt,
	})
	if err != nil {
		return fmt.Errorf("rendering refine prompt: %w", err)
	}

	// Resolve model from agent's phase_models.optimize or default model
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseRefine, "")

	if wctx.Output != nil {
		wctx.Output.Log("info", "refiner", fmt.Sprintf("Refining prompt with %s", agentName))
	}

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Refining prompt", map[string]interface{}{
			"phase":           "refine",
			"model":           model,
			"prompt_length":   len(wctx.State.Prompt),
			"timeout_seconds": 600, // 10 minutes for refiner
		})
	}

	// Execute refinement
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: 10 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseRefine,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		// On error, fall back to original prompt
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "refine",
				"model":       model,
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
				"fallback":    true,
			})
		}
		wctx.Logger.Warn("prompt refinement failed, using original", "error", err)
		if wctx.Output != nil {
			wctx.Output.Log("warn", "refiner", "Refinement failed, using original prompt")
		}
		wctx.State.OptimizedPrompt = wctx.State.Prompt
	} else {
		// Emit completed event
		if wctx.Output != nil {
			wctx.Output.AgentEvent("completed", agentName, "Prompt refinement completed", map[string]interface{}{
				"phase":           "refine",
				"model":           result.Model,
				"tokens_in":       result.TokensIn,
				"tokens_out":      result.TokensOut,
				"duration_ms":     durationMS,
				"original_length": len(wctx.State.Prompt),
				"refined_length":  len(result.Output),
			})
		}
		// Parse the refined prompt
		refined, parseErr := parseRefinementResult(result.Output)
		if parseErr != nil {
			wctx.Logger.Warn("failed to parse refinement result, using original",
				"error", parseErr,
				"output_length", len(result.Output),
				"output_preview", truncateForLog(result.Output, 1000),
			)
			if wctx.Output != nil {
				wctx.Output.Log("warn", "refiner", fmt.Sprintf("Parse failed: %s", parseErr.Error()))
				// Show first 500 chars to help diagnose format issues
				preview := result.Output
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				wctx.Output.Log("debug", "refiner", fmt.Sprintf("Output preview: %s", preview))
			}
			wctx.State.OptimizedPrompt = wctx.State.Prompt
		} else {
			wctx.State.OptimizedPrompt = refined
			wctx.Logger.Info("prompt refined successfully",
				"original_length", len(wctx.State.Prompt),
				"refined_length", len(refined),
			)
			if wctx.Output != nil {
				wctx.Output.Log("success", "refiner", fmt.Sprintf("Prompt enhanced: %d → %d chars", len(wctx.State.Prompt), len(refined)))
			}
			// Write refined prompt to report
			if wctx.Report != nil {
				if reportErr := wctx.Report.WriteRefinedPrompt("", refined, report.PromptMetrics{}); reportErr != nil {
					wctx.Logger.Warn("failed to write refined prompt report", "error", reportErr)
				}
			}
		}
	}

	// Create checkpoint with refinement details
	if err := wctx.Checkpoint.CreateCheckpoint(wctx.State, "prompt_refinement", map[string]interface{}{
		"original_length": len(wctx.State.Prompt),
		"refined_length":  len(wctx.State.OptimizedPrompt),
		"agent":           agentName,
	}); err != nil {
		wctx.Logger.Warn("failed to create refinement checkpoint", "error", err)
	}

	return wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseRefine, true)
}

// parseRefinementResult extracts the refined prompt from agent output.
// The agent should return just the enhanced prompt directly (no structure needed).
func parseRefinementResult(output string) (string, error) {
	// Try to extract from CLI JSON wrapper first (e.g., Claude CLI --output-format json)
	// The wrapper has format: {"type":"result","result":"...content...","session_id":"..."}
	var cliWrapper struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &cliWrapper); err == nil && cliWrapper.Type == "result" {
		// This is a CLI wrapper - use the result field (may be empty)
		output = cliWrapper.Result
	}

	// Clean up the output
	prompt := strings.TrimSpace(output)

	// If empty or too short, fail
	if len(prompt) < 10 {
		return "", fmt.Errorf("output too short (%d chars)", len(prompt))
	}

	return prompt, nil
}

// GetEffectivePrompt returns the optimized prompt if available, otherwise the original.
func GetEffectivePrompt(state *core.WorkflowState) string {
	base := state.Prompt
	if state.OptimizedPrompt != "" {
		base = state.OptimizedPrompt
	}

	if len(state.Attachments) == 0 {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n## Attached Documents\n")
	sb.WriteString("The user attached the following documents. They are stored under `.quorum/attachments/` and can be read from disk if needed.\n\n")
	for _, a := range state.Attachments {
		sb.WriteString(fmt.Sprintf("- %s (%d bytes): %s\n", a.Name, a.Size, a.Path))
	}
	sb.WriteString("\n")
	return sb.String()
}

// truncateForLog truncates a string to maxLen characters for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
