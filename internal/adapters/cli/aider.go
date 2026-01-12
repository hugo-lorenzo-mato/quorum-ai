package cli

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// AiderAdapter implements Agent for Aider CLI.
type AiderAdapter struct {
	*BaseAdapter
	capabilities core.Capabilities
}

// NewAiderAdapter creates a new Aider adapter.
func NewAiderAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "aider"
	}

	logger := logging.NewNop().With("adapter", "aider")
	base := NewBaseAdapter(cfg, logger)

	adapter := &AiderAdapter{
		BaseAdapter: base,
		capabilities: core.Capabilities{
			SupportsJSON:      false,
			SupportsStreaming: true,
			SupportsImages:    false,
			SupportsTools:     false,
			MaxContextTokens:  128000,
			MaxOutputTokens:   8192,
			SupportedModels: []string{
				"gpt-4o",
				"gpt-4-turbo",
				"claude-3-5-sonnet-20241022",
				"claude-3-opus-20240229",
			},
			DefaultModel: "gpt-4o",
		},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (a *AiderAdapter) Name() string {
	return "aider"
}

// Capabilities returns adapter capabilities.
func (a *AiderAdapter) Capabilities() core.Capabilities {
	return a.capabilities
}

// Ping checks if Aider CLI is available.
func (a *AiderAdapter) Ping(ctx context.Context) error {
	if err := a.CheckAvailability(ctx); err != nil {
		return err
	}

	_, err := a.GetVersion(ctx, "--version")
	return err
}

// Execute runs a prompt through Aider CLI.
func (a *AiderAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := a.buildArgs(opts)

	result, err := a.ExecuteCommand(ctx, args, opts.Prompt)
	if err != nil {
		return nil, err
	}

	return a.parseOutput(result, opts.Format)
}

// buildArgs constructs CLI arguments for Aider.
func (a *AiderAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{}

	// Model selection
	model := opts.Model
	if model == "" {
		model = a.config.Model
	}
	if model != "" {
		// Aider uses different flags for different model providers
		if strings.HasPrefix(model, "claude") {
			if strings.Contains(model, "opus") {
				args = append(args, "--opus")
			} else {
				args = append(args, "--sonnet")
			}
		} else if strings.HasPrefix(model, "gpt") {
			args = append(args, "--model", model)
		}
	}

	// No git operations
	args = append(args, "--no-git")

	// No auto commits
	args = append(args, "--no-auto-commits")

	// Yes to all (non-interactive)
	args = append(args, "--yes")

	// Message mode for single prompt
	args = append(args, "--message")

	return args
}

// parseOutput parses Aider CLI output.
func (a *AiderAdapter) parseOutput(result *CommandResult, _ core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	// Clean Aider-specific output markers
	output = a.cleanOutput(output)

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	a.extractUsage(result, execResult)

	return execResult, nil
}

// cleanOutput removes Aider-specific markers from output.
func (a *AiderAdapter) cleanOutput(output string) string {
	// Remove progress indicators
	output = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(output, "")

	// Remove spinner characters
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, spinner := range spinners {
		output = strings.ReplaceAll(output, spinner, "")
	}

	// Remove ANSI codes
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	output = ansiPattern.ReplaceAllString(output, "")

	return strings.TrimSpace(output)
}

// extractUsage attempts to extract token usage.
func (a *AiderAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Aider outputs token usage in format:
	// "Tokens: X sent, Y received. Cost: $0.XX"
	tokenPattern := regexp.MustCompile(`Tokens:\s*(\d+)\s*sent,\s*(\d+)\s*received`)
	if matches := tokenPattern.FindStringSubmatch(combined); len(matches) == 3 {
		if sent, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = sent
		}
		if received, err := strconv.Atoi(matches[2]); err == nil {
			execResult.TokensOut = received
		}
	}

	// Cost pattern
	costPattern := regexp.MustCompile(`Cost:\s*\$?([\d.]+)`)
	if matches := costPattern.FindStringSubmatch(combined); len(matches) == 2 {
		if cost, err := strconv.ParseFloat(matches[1], 64); err == nil {
			execResult.CostUSD = cost
		}
	}

	// Estimate if not found
	if execResult.TokensIn == 0 {
		execResult.TokensIn = a.TokenEstimate(result.Stdout)
	}
	if execResult.TokensOut == 0 {
		execResult.TokensOut = a.TokenEstimate(result.Stdout)
	}

	if execResult.CostUSD == 0 {
		execResult.CostUSD = a.estimateCost(execResult.TokensIn, execResult.TokensOut)
	}
}

// estimateCost provides rough cost estimation.
func (a *AiderAdapter) estimateCost(tokensIn, tokensOut int) float64 {
	// GPT-4o pricing as default (Aider often uses this)
	inputCost := float64(tokensIn) / 1000000 * 2.50
	outputCost := float64(tokensOut) / 1000000 * 10.00
	return inputCost + outputCost
}

// AiderConfig holds Aider-specific configuration.
type AiderConfig struct {
	AgentConfig
	NoGit        bool
	NoAutoCommit bool
	EditFormat   string // whole, diff, diff-fenced
}

// WithEditFormat returns args for specifying Aider's edit format.
func (a *AiderAdapter) WithEditFormat(format string) []string {
	if format == "" {
		format = "whole"
	}
	return []string{"--edit-format", format}
}

// Ensure AiderAdapter implements core.Agent
var _ core.Agent = (*AiderAdapter)(nil)
