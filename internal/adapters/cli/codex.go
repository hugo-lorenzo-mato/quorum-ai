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
			SupportedModels: []string{
				"gpt-5.2",
				"gpt-5.2-codex",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1-codex-mini",
				"gpt-5.1",
				"gpt-5",
				"gpt-5-mini",
				"gpt-4.1",
				"o3",
				"o4-mini",
			},
			DefaultModel: "gpt-5.2",
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

	// Codex CLI doesn't have --system-prompt, so prepend to user prompt
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

	// Determine reasoning effort: config > phase-based defaults
	reasoningEffort := c.getReasoningEffort(opts.Phase)

	// Headless approvals/sandbox via config overrides
	args = append(args,
		"-c", `approval_policy="never"`,
		"-c", `sandbox_mode="workspace-write"`,
		"-c", `model_reasoning_effort="`+reasoningEffort+`"`,
		"-c", `skip_git_repo_check=true`,
	)

	// Model selection
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Max tokens and temperature are configured via Codex config files,
	// not as CLI flags for `codex exec`.

	// Note: --json flag is added by ExecuteWithStreaming via streaming config
	// This enables real-time JSONL events while the LLM writes output files directly

	return args
}

// getReasoningEffort returns reasoning effort for the given phase.
// Priority: config > phase-based defaults.
func (c *CodexAdapter) getReasoningEffort(phase core.Phase) string {
	// Check config first
	if effort := c.config.GetReasoningEffort(string(phase)); effort != "" {
		return effort
	}

	// Fall back to phase-based defaults
	switch phase {
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
	promptTokens := regexp.MustCompile(`prompt_tokens?:?\s*(\d+)`)
	if matches := promptTokens.FindStringSubmatch(combined); len(matches) == 2 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
			tokenSource = "parsed"
		}
	}

	completionTokens := regexp.MustCompile(`completion_tokens?:?\s*(\d+)`)
	if matches := completionTokens.FindStringSubmatch(combined); len(matches) == 2 {
		if out, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensOut = out
		}
	}

	// Estimate tokens if not found
	// Note: TokensIn should be based on INPUT (prompt), TokensOut on OUTPUT (response)
	// Since we only have the output here, we estimate TokensOut from it
	// and use a heuristic for TokensIn (typically prompts are shorter than responses)
	if execResult.TokensOut == 0 {
		execResult.TokensOut = c.TokenEstimate(result.Stdout)
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
