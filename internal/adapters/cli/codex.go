package cli

import (
	"context"
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
	prompt := opts.Prompt
	if opts.SystemPrompt != "" && prompt != "" {
		prompt = "[System Instructions]\n" + opts.SystemPrompt + "\n\n[User Message]\n" + prompt
	}
	if prompt != "" {
		args = append(args, prompt)
	}

	result, err := c.ExecuteCommand(ctx, args, "", opts.WorkDir)
	if err != nil {
		return nil, err
	}

	return c.parseOutput(result, opts.Format)
}

// buildArgs constructs CLI arguments for Codex.
func (c *CodexAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{"exec", "--skip-git-repo-check"}

	// Determine reasoning effort based on phase
	// Use "xhigh" (extra high) for analyze/optimize/plan, "high" for execute
	reasoningEffort := "high"
	switch opts.Phase {
	case core.PhaseOptimize, core.PhaseAnalyze, core.PhasePlan:
		reasoningEffort = "xhigh"
	case core.PhaseExecute:
		reasoningEffort = "high"
	}

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

	// Output format
	if opts.Format == core.OutputFormatJSON {
		args = append(args, "--json")
	}

	return args
}

// parseOutput parses Codex CLI output.
func (c *CodexAdapter) parseOutput(result *CommandResult, format core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	c.extractUsage(result, execResult)

	if format == core.OutputFormatJSON {
		var parsed map[string]interface{}
		if err := c.ParseJSON(output, &parsed); err == nil {
			execResult.Parsed = parsed
		}
	}

	return execResult, nil
}

// extractUsage attempts to extract token usage.
func (c *CodexAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// OpenAI-style usage patterns
	promptTokens := regexp.MustCompile(`prompt_tokens?:?\s*(\d+)`)
	if matches := promptTokens.FindStringSubmatch(combined); len(matches) == 2 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
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
	}
	if execResult.TokensIn == 0 && execResult.TokensOut > 0 {
		// Heuristic: input is typically 20-50% of output for conversational prompts
		execResult.TokensIn = execResult.TokensOut / 3
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

// Ensure CodexAdapter implements core.Agent
var _ core.Agent = (*CodexAdapter)(nil)
