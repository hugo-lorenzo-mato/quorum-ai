package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// CodexAdapter implements Agent for OpenAI Codex CLI.
type CodexAdapter struct {
	*BaseAdapter
	capabilities core.Capabilities
}

// NewCodexAdapter creates a new Codex adapter.
func NewCodexAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "codex"
	}

	logger := logging.NewNop().With("adapter", "codex")
	base := NewBaseAdapter(cfg, logger)

	adapter := &CodexAdapter{
		BaseAdapter: base,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: true,
			SupportsImages:    false,
			SupportsTools:     true,
			MaxContextTokens:  128000,
			MaxOutputTokens:   16384,
			SupportedModels:   core.GetSupportedModels(core.AgentCodex),
			DefaultModel:      core.GetDefaultModel(core.AgentCodex),
		},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (c *CodexAdapter) Name() string {
	return "codex"
}

// Capabilities returns adapter capabilities.
func (c *CodexAdapter) Capabilities() core.Capabilities {
	return c.capabilities
}

// SetEventHandler sets the handler for streaming events.
func (c *CodexAdapter) SetEventHandler(handler core.AgentEventHandler) {
	c.BaseAdapter.SetEventHandler(handler)
}

// Ping checks if Codex CLI is available.
func (c *CodexAdapter) Ping(ctx context.Context) error {
	if err := c.CheckAvailability(ctx); err != nil {
		return err
	}

	_, err := c.GetVersion(ctx, "--version")
	return err
}

// Execute runs a prompt through Codex CLI.
func (c *CodexAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := c.buildArgs(opts)

	// Codex CLI handles system prompt via -c developer_instructions or prepend to user prompt
	// Pass via stdin for robustness with long prompts and special characters
	prompt := opts.Prompt
	if opts.SystemPrompt != "" && prompt != "" {
		prompt = "[System Instructions]\n" + opts.SystemPrompt + "\n\n[User Message]\n" + prompt
	}

	// Use streaming execution if event handler is configured
	var result *CommandResult
	var err error
	if c.eventHandler != nil {
		result, err = c.ExecuteWithStreaming(ctx, "codex", args, prompt, opts.WorkDir, opts.Timeout)
	} else {
		result, err = c.ExecuteCommand(ctx, args, prompt, opts.WorkDir, opts.Timeout)
	}
	if err != nil {
		return nil, err
	}

	return c.parseOutput(result, opts.Format)
}

// buildArgs constructs CLI arguments for Codex.
func (c *CodexAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{"exec", "--skip-git-repo-check"}

	// Model selection (opts overrides config; fall back to core default).
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}
	if model == "" {
		model = core.GetDefaultModel(core.AgentCodex)
	}

	// Determine reasoning effort: opts (per-message) > config > phase-based defaults
	reasoningEffort := c.getReasoningEffort(opts)
	reasoningEffort = core.NormalizeReasoningEffortForModel(model, reasoningEffort)

	// Headless approvals/sandbox via config overrides
	args = append(args,
		"-c", `approval_policy="never"`,
		"-c", `sandbox_mode="workspace-write"`,
		"-c", `model_reasoning_effort="`+reasoningEffort+`"`,
		"-c", `skip_git_repo_check=true`,
	)

	// `minimal` reasoning effort is incompatible with web_search; disable it explicitly
	// to avoid Codex API errors when users have web_search enabled in ~/.codex/config.toml.
	if reasoningEffort == core.ReasoningMinimal {
		args = append(args, "-c", `web_search="disabled"`)
	}

	// Model selection
	if model != "" {
		args = append(args, "--model", model)
	}

	// System prompt via developer_instructions (for chat mode optimizations)
	// Note: This is in addition to the prepended instructions in Execute()
	// The -c config approach can be used for more structured control if needed in the future

	// Max tokens and temperature are configured via Codex config files,
	// not as CLI flags for `codex exec`.

	// Note: --json flag is added by ExecuteWithStreaming via streaming config
	// This enables real-time JSONL events while the LLM writes output files directly

	return args
}

// getReasoningEffort returns reasoning effort for the given options.
// Priority: opts.ReasoningEffort (per-message) > config > phase-based defaults.
func (c *CodexAdapter) getReasoningEffort(opts core.ExecuteOptions) string {
	// Check per-message override first
	if opts.ReasoningEffort != "" {
		return opts.ReasoningEffort
	}

	// Check config
	if effort := c.config.GetReasoningEffort(string(opts.Phase)); effort != "" {
		return effort
	}

	// Fall back to phase-based defaults
	switch opts.Phase {
	case core.PhaseRefine, core.PhaseAnalyze, core.PhasePlan:
		return "xhigh"
	case core.PhaseExecute:
		return "high"
	default:
		return "high"
	}
}

// parseOutput parses Codex CLI output.
func (c *CodexAdapter) parseOutput(result *CommandResult, _ core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	c.extractUsage(result, execResult)

	return execResult, nil
}

// extractUsage attempts to extract token usage.
func (c *CodexAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Debug: track source of token values
	var tokenSource string

	// OpenAI-style usage patterns
	// IMPORTANT: Use word boundaries and require explicit separators to avoid
	// matching unrelated numbers in the output (like file sizes, line numbers, etc.)
	// Also limit digits to max 7 (10M tokens) to avoid matching corrupted values
	promptTokens := regexp.MustCompile(`\b(?:prompt|input)_tokens?\s*[=:]\s*(\d{1,7})\b`)
	if matches := promptTokens.FindStringSubmatch(combined); len(matches) == 2 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
			tokenSource = "parsed"
		}
	}

	completionTokens := regexp.MustCompile(`\b(?:completion|output)_tokens?\s*[=:]\s*(\d{1,7})\b`)
	if matches := completionTokens.FindStringSubmatch(combined); len(matches) == 2 {
		if out, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensOut = out
		}
	}

	// Estimate tokens from output length for comparison/fallback
	estimatedTokensOut := c.TokenEstimate(result.Stdout)

	// Detect token reporting discrepancy: reported tokens suspiciously different from actual output
	// This catches cases where CLI reports wrong token counts
	threshold := c.config.TokenDiscrepancyThreshold
	if threshold <= 0 {
		threshold = DefaultTokenDiscrepancyThreshold
	}
	if execResult.TokensOut > 0 && estimatedTokensOut > 100 && threshold > 0 {
		// If reported tokens are less than 1/threshold of estimated (too low)
		if float64(execResult.TokensOut) < float64(estimatedTokensOut)/threshold {
			c.emitEvent(core.NewAgentEvent(
				core.AgentEventProgress,
				"codex",
				fmt.Sprintf("[WARN] Token discrepancy (too low): reported=%d, estimated=%d (threshold=%.1fx). Using estimated.",
					execResult.TokensOut, estimatedTokensOut, threshold),
			).WithData(map[string]any{
				"reported_tokens":  execResult.TokensOut,
				"estimated_tokens": estimatedTokensOut,
				"output_length":    len(result.Stdout),
				"threshold":        threshold,
				"source":           tokenSource,
				"action":           "using_estimated",
				"discrepancy_type": "too_low",
			}))
			execResult.TokensOut = estimatedTokensOut
			tokenSource = "estimated_discrepancy"
		}
		// If reported tokens are more than threshold*estimated (too high)
		if float64(execResult.TokensOut) > float64(estimatedTokensOut)*threshold {
			c.emitEvent(core.NewAgentEvent(
				core.AgentEventProgress,
				"codex",
				fmt.Sprintf("[WARN] Token discrepancy (too high): reported=%d, estimated=%d (threshold=%.1fx). Using estimated.",
					execResult.TokensOut, estimatedTokensOut, threshold),
			).WithData(map[string]any{
				"reported_tokens":  execResult.TokensOut,
				"estimated_tokens": estimatedTokensOut,
				"output_length":    len(result.Stdout),
				"threshold":        threshold,
				"source":           tokenSource,
				"action":           "using_estimated",
				"discrepancy_type": "too_high",
			}))
			execResult.TokensOut = estimatedTokensOut
			tokenSource = "estimated_discrepancy"
		}
	}

	// Estimate tokens if not found
	// Note: TokensIn should be based on INPUT (prompt), TokensOut on OUTPUT (response)
	// Since we only have the output here, we estimate TokensOut from it
	// and use a heuristic for TokensIn (typically prompts are shorter than responses)
	if execResult.TokensOut == 0 {
		execResult.TokensOut = estimatedTokensOut
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
		c.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"codex",
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
		c.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"codex",
			fmt.Sprintf("[WARN] Capped unrealistic TokensOut: %d -> %d", execResult.TokensOut, maxReasonableTokens),
		).WithData(map[string]any{
			"original":      execResult.TokensOut,
			"capped":        maxReasonableTokens,
			"source":        tokenSource,
			"stdout_sample": truncateForDebug(result.Stdout, 200),
		}))
		execResult.TokensOut = maxReasonableTokens
	}

	execResult.CostUSD = c.estimateCost(execResult.TokensIn, execResult.TokensOut)
}

// estimateCost provides rough cost estimation for Codex/GPT.
func (c *CodexAdapter) estimateCost(tokensIn, tokensOut int) float64 {
	// GPT-4o pricing (approximate)
	// Input: $2.50/1M tokens, Output: $10.00/1M tokens
	inputCost := float64(tokensIn) / 1000000 * 2.50
	outputCost := float64(tokensOut) / 1000000 * 10.00
	return inputCost + outputCost
}

// Ensure CodexAdapter implements core.Agent and core.StreamingCapable
var _ core.Agent = (*CodexAdapter)(nil)
var _ core.StreamingCapable = (*CodexAdapter)(nil)
