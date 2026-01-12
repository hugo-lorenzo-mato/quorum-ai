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
				"gpt-4o",
				"gpt-4-turbo",
				"gpt-4",
				"gpt-3.5-turbo",
			},
			DefaultModel: "gpt-4o",
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

	result, err := c.ExecuteCommand(ctx, args, opts.Prompt)
	if err != nil {
		return nil, err
	}

	return c.parseOutput(result, opts.Format)
}

// buildArgs constructs CLI arguments for Codex.
func (c *CodexAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{}

	// Model selection
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Max tokens
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxTokens
	}
	if maxTokens > 0 {
		args = append(args, "--max-tokens", strconv.Itoa(maxTokens))
	}

	// Temperature
	temp := opts.Temperature
	if temp == 0 {
		temp = c.config.Temperature
	}
	if temp > 0 {
		args = append(args, "--temperature", strconv.FormatFloat(temp, 'f', 2, 64))
	}

	// Output format
	if opts.Format == core.OutputFormatJSON {
		args = append(args, "--json")
	}

	// Full auto mode for non-interactive
	args = append(args, "--full-auto")

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

	// Estimate if not found
	if execResult.TokensIn == 0 {
		execResult.TokensIn = c.TokenEstimate(result.Stdout)
	}
	if execResult.TokensOut == 0 {
		execResult.TokensOut = c.TokenEstimate(result.Stdout)
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
