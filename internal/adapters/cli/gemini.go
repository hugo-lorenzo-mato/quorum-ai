package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// GeminiAdapter implements Agent for Gemini CLI.
type GeminiAdapter struct {
	*BaseAdapter
	capabilities core.Capabilities
}

// NewGeminiAdapter creates a new Gemini adapter.
func NewGeminiAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "gemini"
	}

	logger := logging.NewNop().With("adapter", "gemini")
	base := NewBaseAdapter(cfg, logger)

	adapter := &GeminiAdapter{
		BaseAdapter: base,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: true,
			SupportsImages:    true,
			SupportsTools:     true,
			MaxContextTokens:  1000000, // 1M context window
			MaxOutputTokens:   8192,
			SupportedModels: []string{
				// Gemini 2.5 family (stable, recommended)
				"gemini-2.5-pro",        // Most powerful, best for coding and agentic tasks
				"gemini-2.5-flash",      // Best price/performance balance with thinking
				"gemini-2.5-flash-lite", // Fast, low-cost, 1M context
				// Gemini 2.0 family (retiring March 2026)
				"gemini-2.0-flash",
				"gemini-2.0-flash-lite",
				// Gemini 3 preview models
				"gemini-3-pro-preview",
				"gemini-3-flash-preview",
			},
			DefaultModel: "gemini-2.5-flash",
		},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (g *GeminiAdapter) Name() string {
	return "gemini"
}

// Capabilities returns adapter capabilities.
func (g *GeminiAdapter) Capabilities() core.Capabilities {
	return g.capabilities
}

// SetLogCallback sets a callback for real-time stderr streaming.
func (g *GeminiAdapter) SetLogCallback(cb LogCallback) {
	g.BaseAdapter.SetLogCallback(cb)
}

// SetEventHandler sets the handler for streaming events.
func (g *GeminiAdapter) SetEventHandler(handler core.AgentEventHandler) {
	g.BaseAdapter.SetEventHandler(handler)
}

// Ping checks if Gemini CLI is available.
func (g *GeminiAdapter) Ping(ctx context.Context) error {
	if err := g.CheckAvailability(ctx); err != nil {
		return err
	}

	_, err := g.GetVersion(ctx, "--version")
	return err
}

// Execute runs a prompt through Gemini CLI.
func (g *GeminiAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := g.buildArgs(opts)

	// Gemini CLI doesn't have --system-prompt, so prepend to user prompt
	// Pass via stdin for robustness with long prompts and special characters
	prompt := opts.Prompt
	if opts.SystemPrompt != "" && prompt != "" {
		prompt = "[System Instructions]\n" + opts.SystemPrompt + "\n\n[User Message]\n" + prompt
	}

	// Use streaming execution if event handler is configured
	var result *CommandResult
	var err error
	if g.eventHandler != nil {
		result, err = g.ExecuteWithStreaming(ctx, "gemini", args, prompt, opts.WorkDir, opts.Timeout)
	} else {
		result, err = g.ExecuteCommand(ctx, args, prompt, opts.WorkDir, opts.Timeout)
	}
	if err != nil {
		return nil, err
	}

	return g.parseOutput(result, opts.Format)
}

// buildArgs constructs CLI arguments for Gemini.
func (g *GeminiAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{}

	// Model selection
	model := opts.Model
	if model == "" {
		model = g.config.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Note: --output-format stream-json is added by ExecuteWithStreaming via streaming config
	// This enables real-time progress monitoring while the LLM writes output files directly

	// Headless auto-approval
	args = append(args, "--approval-mode", "yolo")

	return args
}

// parseOutput parses Gemini CLI output.
func (g *GeminiAdapter) parseOutput(result *CommandResult, _ core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	// Extract usage from output
	g.extractUsage(result, execResult)

	return execResult, nil
}

// extractUsage attempts to extract token usage.
func (g *GeminiAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Debug: track source of token values
	var tokenSource string

	// Gemini-specific token patterns
	inputPattern := regexp.MustCompile(`input[_\s]?tokens?:?\s*(\d+)`)
	if matches := inputPattern.FindStringSubmatch(combined); len(matches) == 2 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
			tokenSource = "parsed"
		}
	}

	outputPattern := regexp.MustCompile(`output[_\s]?tokens?:?\s*(\d+)`)
	if matches := outputPattern.FindStringSubmatch(combined); len(matches) == 2 {
		if out, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensOut = out
		}
	}

	// Estimate tokens if not found
	// Note: TokensIn should be based on INPUT (prompt), TokensOut on OUTPUT (response)
	// Since we only have the output here, we estimate TokensOut from it
	// and use a heuristic for TokensIn (typically prompts are shorter than responses)
	if execResult.TokensOut == 0 {
		execResult.TokensOut = g.TokenEstimate(result.Stdout)
		tokenSource = "estimated"
	}
	if execResult.TokensIn == 0 && execResult.TokensOut > 0 {
		// Heuristic: input is typically 20-50% of output for conversational prompts
		execResult.TokensIn = execResult.TokensOut / 3
	}

	// Cap token values to avoid corrupted/unrealistic values
	// Max reasonable is ~500k (very large context + response)
	const maxReasonableTokens = 500_000
	if execResult.TokensIn > maxReasonableTokens {
		g.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"gemini",
			fmt.Sprintf("[WARN] Capped unrealistic TokensIn: %d -> %d", execResult.TokensIn, maxReasonableTokens),
		).WithData(map[string]any{
			"original":      execResult.TokensIn,
			"capped":        maxReasonableTokens,
			"source":        tokenSource,
			"stdout_sample": truncateForDebug(result.Stdout, 200),
		}))
		execResult.TokensIn = maxReasonableTokens
	}
	if execResult.TokensOut > maxReasonableTokens {
		g.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"gemini",
			fmt.Sprintf("[WARN] Capped unrealistic TokensOut: %d -> %d", execResult.TokensOut, maxReasonableTokens),
		).WithData(map[string]any{
			"original":      execResult.TokensOut,
			"capped":        maxReasonableTokens,
			"source":        tokenSource,
			"stdout_sample": truncateForDebug(result.Stdout, 200),
		}))
		execResult.TokensOut = maxReasonableTokens
	}

	// Estimate cost
	execResult.CostUSD = g.estimateCost(execResult.TokensIn, execResult.TokensOut)
}

// estimateCost provides rough cost estimation for Gemini.
func (g *GeminiAdapter) estimateCost(tokensIn, tokensOut int) float64 {
	// Gemini Flash pricing (approximate)
	// Input: $0.075/1M tokens, Output: $0.30/1M tokens
	inputCost := float64(tokensIn) / 1000000 * 0.075
	outputCost := float64(tokensOut) / 1000000 * 0.30
	return inputCost + outputCost
}

// Ensure GeminiAdapter implements core.Agent and core.StreamingCapable
var _ core.Agent = (*GeminiAdapter)(nil)
var _ core.StreamingCapable = (*GeminiAdapter)(nil)
