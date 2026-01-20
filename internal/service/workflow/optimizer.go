package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// OptimizerConfig configures the optimizer phase.
type OptimizerConfig struct {
	Enabled bool
	Agent   string
	Model   string
}

// Optimizer runs the prompt optimization phase.
type Optimizer struct {
	config OptimizerConfig
}

// NewOptimizer creates a new optimizer.
func NewOptimizer(config OptimizerConfig) *Optimizer {
	return &Optimizer{config: config}
}

// Run executes the optimize phase.
func (o *Optimizer) Run(ctx context.Context, wctx *Context) error {
	// Write original prompt report (always, even if optimization is disabled)
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteOriginalPrompt(wctx.State.Prompt); reportErr != nil {
			wctx.Logger.Warn("failed to write original prompt report", "error", reportErr)
		}
	}

	// Skip if optimization is disabled
	if !o.config.Enabled {
		wctx.Logger.Info("prompt optimization disabled, skipping phase")
		if wctx.Output != nil {
			wctx.Output.Log("info", "optimizer", "Optimization disabled, skipping phase")
		}
		return nil
	}

	wctx.Logger.Info("starting optimize phase", "workflow_id", wctx.State.WorkflowID)

	wctx.State.CurrentPhase = core.PhaseOptimize
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseOptimize)
		wctx.Output.Log("info", "optimizer", "Starting prompt optimization phase")
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseOptimize, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Skip actual optimization in dry-run mode
	if wctx.Config.DryRun {
		wctx.Logger.Info("dry-run mode: skipping actual optimization")
		if wctx.Output != nil {
			wctx.Output.Log("info", "optimizer", "Dry-run mode: using original prompt")
		}
		wctx.State.OptimizedPrompt = wctx.State.Prompt // Use original as-is
		return wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseOptimize, true)
	}

	// Get the optimization agent
	agentName := o.config.Agent
	if agentName == "" {
		agentName = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return fmt.Errorf("getting optimizer agent %s: %w", agentName, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return fmt.Errorf("rate limit for optimizer: %w", err)
	}

	// Render optimization prompt
	prompt, err := wctx.Prompts.RenderOptimizePrompt(OptimizePromptParams{
		OriginalPrompt: wctx.State.Prompt,
	})
	if err != nil {
		return fmt.Errorf("rendering optimize prompt: %w", err)
	}

	// Resolve model
	model := o.config.Model
	if model == "" {
		model = ResolvePhaseModel(wctx.Config, agentName, core.PhaseOptimize, "")
	}

	if wctx.Output != nil {
		wctx.Output.Log("info", "optimizer", fmt.Sprintf("Optimizing prompt with %s", agentName))
	}

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Optimizing prompt", map[string]interface{}{
			"phase":         "optimize",
			"model":         model,
			"prompt_length": len(wctx.State.Prompt),
		})
	}

	// Execute optimization
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: 3 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseOptimize,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		// On error, fall back to original prompt
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "optimize",
				"model":       model,
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
				"fallback":    true,
			})
		}
		wctx.Logger.Warn("prompt optimization failed, using original", "error", err)
		if wctx.Output != nil {
			wctx.Output.Log("warn", "optimizer", "Optimization failed, using original prompt")
		}
		wctx.State.OptimizedPrompt = wctx.State.Prompt
	} else {
		// Emit completed event
		if wctx.Output != nil {
			wctx.Output.AgentEvent("completed", agentName, "Prompt optimization completed", map[string]interface{}{
				"phase":            "optimize",
				"model":            result.Model,
				"tokens_in":        result.TokensIn,
				"tokens_out":       result.TokensOut,
				"cost_usd":         result.CostUSD,
				"duration_ms":      durationMS,
				"original_length":  len(wctx.State.Prompt),
				"optimized_length": len(result.Output),
			})
		}
		// Parse the optimized prompt
		optimized, parseErr := parseOptimizationResult(result.Output)
		if parseErr != nil {
			wctx.Logger.Warn("failed to parse optimization result, using original",
				"error", parseErr,
				"output_length", len(result.Output),
				"output_preview", truncateForLog(result.Output, 1000),
			)
			if wctx.Output != nil {
				wctx.Output.Log("warn", "optimizer", fmt.Sprintf("Parse failed: %s", parseErr.Error()))
				// Show first 500 chars to help diagnose format issues
				preview := result.Output
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				wctx.Output.Log("debug", "optimizer", fmt.Sprintf("Output preview: %s", preview))
			}
			wctx.State.OptimizedPrompt = wctx.State.Prompt
		} else {
			wctx.State.OptimizedPrompt = optimized
			wctx.Logger.Info("prompt optimized successfully",
				"original_length", len(wctx.State.Prompt),
				"optimized_length", len(optimized),
			)
			if wctx.Output != nil {
				wctx.Output.Log("success", "optimizer", fmt.Sprintf("Prompt enhanced: %d → %d chars", len(wctx.State.Prompt), len(optimized)))
			}
		}
	}

	// Create checkpoint with optimization details
	if err := wctx.Checkpoint.CreateCheckpoint(wctx.State, "prompt_optimization", map[string]interface{}{
		"original_length":  len(wctx.State.Prompt),
		"optimized_length": len(wctx.State.OptimizedPrompt),
		"agent":            agentName,
	}); err != nil {
		wctx.Logger.Warn("failed to create optimization checkpoint", "error", err)
	}

	return wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseOptimize, true)
}

// parseOptimizationResult extracts the optimized prompt from agent output.
// The agent should return just the enhanced prompt directly (no structure needed).
func parseOptimizationResult(output string) (string, error) {
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
	if state.OptimizedPrompt != "" {
		return state.OptimizedPrompt
	}
	return state.Prompt
}

// truncateForLog truncates a string to maxLen characters for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
