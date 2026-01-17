package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
	// Skip if optimization is disabled
	if !o.config.Enabled {
		wctx.Logger.Info("prompt optimization disabled, skipping phase")
		return nil
	}

	wctx.Logger.Info("starting optimize phase", "workflow_id", wctx.State.WorkflowID)

	wctx.State.CurrentPhase = core.PhaseOptimize
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseOptimize)
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseOptimize, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Skip actual optimization in dry-run mode
	if wctx.Config.DryRun {
		wctx.Logger.Info("dry-run mode: skipping actual optimization")
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

	// Execute optimization
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		model := o.config.Model
		if model == "" {
			model = ResolvePhaseModel(wctx.Config, agentName, core.PhaseOptimize, "")
		}
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   model,
			Timeout: 3 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		// On error, fall back to original prompt
		wctx.Logger.Warn("prompt optimization failed, using original", "error", err)
		wctx.State.OptimizedPrompt = wctx.State.Prompt
	} else {
		// Parse the optimized prompt
		optimized, parseErr := parseOptimizationResult(result.Output)
		if parseErr != nil {
			wctx.Logger.Warn("failed to parse optimization result, using original", "error", parseErr)
			wctx.State.OptimizedPrompt = wctx.State.Prompt
		} else {
			wctx.State.OptimizedPrompt = optimized
			wctx.Logger.Info("prompt optimized successfully",
				"original_length", len(wctx.State.Prompt),
				"optimized_length", len(optimized),
			)
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

// optimizationResult represents the expected JSON response from the optimizer.
type optimizationResult struct {
	OptimizedPrompt string   `json:"optimized_prompt"`
	ChangesMade     []string `json:"changes_made"`
	Reasoning       string   `json:"reasoning"`
}

// parseOptimizationResult extracts the optimized prompt from agent output.
func parseOptimizationResult(output string) (string, error) {
	var result optimizationResult

	// First, try to parse the output as a direct optimization result
	if err := json.Unmarshal([]byte(output), &result); err == nil && result.OptimizedPrompt != "" {
		return result.OptimizedPrompt, nil
	}

	// Try to extract from CLI JSON wrapper (e.g., Claude CLI --output-format json)
	// The wrapper has format: {"type":"result","result":"...content...","session_id":"..."}
	var cliWrapper struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &cliWrapper); err == nil && cliWrapper.Result != "" {
		// The result field contains the model's response, which may contain our JSON
		// Try to parse it directly
		if err := json.Unmarshal([]byte(cliWrapper.Result), &result); err == nil && result.OptimizedPrompt != "" {
			return result.OptimizedPrompt, nil
		}
		// Try to extract JSON from markdown code blocks within the result
		extracted := extractJSONFromMarkdown(cliWrapper.Result)
		if extracted != "" {
			if err := json.Unmarshal([]byte(extracted), &result); err == nil && result.OptimizedPrompt != "" {
				return result.OptimizedPrompt, nil
			}
		}
	}

	// Try to extract JSON from markdown code blocks in original output
	extracted := extractJSONFromMarkdown(output)
	if extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &result); err == nil && result.OptimizedPrompt != "" {
			return result.OptimizedPrompt, nil
		}
	}

	return "", fmt.Errorf("optimization result missing optimized_prompt field")
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks.
func extractJSONFromMarkdown(text string) string {
	// Match ```json ... ``` or ``` ... ```
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// GetEffectivePrompt returns the optimized prompt if available, otherwise the original.
func GetEffectivePrompt(state *core.WorkflowState) string {
	if state.OptimizedPrompt != "" {
		return state.OptimizedPrompt
	}
	return state.Prompt
}
