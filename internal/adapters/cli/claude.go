package cli

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// ClaudeAdapter implements Agent for Claude CLI.
type ClaudeAdapter struct {
	*BaseAdapter
	capabilities core.Capabilities
}

// NewClaudeAdapter creates a new Claude adapter.
func NewClaudeAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "claude"
	}

	logger := logging.NewNop().With("adapter", "claude")
	base := NewBaseAdapter(cfg, logger)

	adapter := &ClaudeAdapter{
		BaseAdapter: base,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: true,
			SupportsImages:    true,
			SupportsTools:     true,
			MaxContextTokens:  200000,
			MaxOutputTokens:   8192,
			SupportedModels: []string{
				"claude-sonnet-4-20250514",
				"claude-opus-4-20250514",
				"claude-3-5-sonnet-20241022",
				"claude-3-5-haiku-20241022",
			},
			DefaultModel: "claude-sonnet-4-20250514",
		},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (c *ClaudeAdapter) Name() string {
	return "claude"
}

// Capabilities returns adapter capabilities.
func (c *ClaudeAdapter) Capabilities() core.Capabilities {
	return c.capabilities
}

// Ping checks if Claude CLI is available.
func (c *ClaudeAdapter) Ping(ctx context.Context) error {
	if err := c.CheckAvailability(ctx); err != nil {
		return err
	}

	_, err := c.GetVersion(ctx, "--version")
	return err
}

// Execute runs a prompt through Claude CLI.
func (c *ClaudeAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := c.buildArgs(opts)

	// Build the full prompt, including conversation history if provided
	fullPrompt := c.buildPromptWithHistory(opts)
	if fullPrompt != "" {
		args = append(args, fullPrompt)
	}

	result, err := c.ExecuteCommand(ctx, args, "", opts.WorkDir)
	if err != nil {
		return nil, err
	}

	return c.parseOutput(result, opts.Format)
}

// buildPromptWithHistory constructs a prompt including conversation history.
// For CLI adapters, this converts the Messages array to a text format.
// API-based adapters should use Messages directly instead.
func (c *ClaudeAdapter) buildPromptWithHistory(opts core.ExecuteOptions) string {
	// If no messages, just return the prompt
	if len(opts.Messages) == 0 {
		return opts.Prompt
	}

	// Build conversation context from Messages
	var sb strings.Builder
	sb.WriteString("<conversation_history>\n")

	for _, msg := range opts.Messages {
		switch msg.Role {
		case "user":
			sb.WriteString("<user>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</user>\n")
		case "assistant":
			sb.WriteString("<assistant>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</assistant>\n")
		}
	}

	sb.WriteString("</conversation_history>\n\n")
	sb.WriteString("<current_message>\n")
	sb.WriteString(opts.Prompt)
	sb.WriteString("\n</current_message>")

	return sb.String()
}

// buildArgs constructs CLI arguments.
func (c *ClaudeAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{}

	// Print mode for non-interactive
	args = append(args, "--print")

	// Model selection
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// System prompt (for customizing assistant behavior)
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	// Output format
	if opts.Format == core.OutputFormatJSON {
		args = append(args, "--output-format", "json")
	}

	// Auto-accept for non-interactive mode
	args = append(args, "--dangerously-skip-permissions")

	return args
}

// parseOutput parses Claude CLI output.
func (c *ClaudeAdapter) parseOutput(result *CommandResult, format core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	// Try to extract usage information from stderr or output
	c.extractUsage(result, execResult)

	// Parse JSON if requested
	if format == core.OutputFormatJSON {
		var parsed map[string]interface{}
		if err := c.ParseJSON(output, &parsed); err == nil {
			execResult.Parsed = parsed
		}
	}

	return execResult, nil
}

// extractUsage attempts to extract token usage from output.
func (c *ClaudeAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Pattern for token usage
	// Example: "tokens: 1234 in, 567 out"
	tokenPattern := regexp.MustCompile(`tokens?:?\s*(\d+)\s*in\D*(\d+)\s*out`)
	if matches := tokenPattern.FindStringSubmatch(combined); len(matches) == 3 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
		}
		if out, err := strconv.Atoi(matches[2]); err == nil {
			execResult.TokensOut = out
		}
	}

	// Pattern for cost
	// Example: "cost: $0.0123"
	costPattern := regexp.MustCompile(`cost:?\s*\$?([\d.]+)`)
	if matches := costPattern.FindStringSubmatch(combined); len(matches) == 2 {
		if cost, err := strconv.ParseFloat(matches[1], 64); err == nil {
			execResult.CostUSD = cost
		}
	}

	// Estimate tokens if not found
	if execResult.TokensIn == 0 {
		execResult.TokensIn = c.TokenEstimate(result.Stdout)
	}
	if execResult.TokensOut == 0 {
		execResult.TokensOut = c.TokenEstimate(result.Stdout)
	}

	// Estimate cost if not found
	if execResult.CostUSD == 0 {
		execResult.CostUSD = c.estimateCost(execResult.TokensIn, execResult.TokensOut)
	}
}

// estimateCost provides rough cost estimation.
func (c *ClaudeAdapter) estimateCost(tokensIn, tokensOut int) float64 {
	// Sonnet pricing (approximate): $3/MTok in, $15/MTok out
	inputCost := float64(tokensIn) / 1000000 * 3
	outputCost := float64(tokensOut) / 1000000 * 15
	return inputCost + outputCost
}

// Ensure ClaudeAdapter implements core.Agent
var _ core.Agent = (*ClaudeAdapter)(nil)
