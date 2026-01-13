package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// CopilotAdapter implements Agent for GitHub Copilot CLI (standalone).
// This adapter uses the new `copilot` CLI (npm install -g @github/copilot)
// which replaced the deprecated `gh copilot` extension.
type CopilotAdapter struct {
	config       AgentConfig
	logger       *logging.Logger
	capabilities core.Capabilities
}

// NewCopilotAdapter creates a new Copilot adapter.
func NewCopilotAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "copilot"
	}

	logger := logging.NewNop().With("adapter", "copilot")

	adapter := &CopilotAdapter{
		config: cfg,
		logger: logger,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: true,
			SupportsImages:    false,
			SupportsTools:     true,
			MaxContextTokens:  200000,
			MaxOutputTokens:   16384,
			SupportedModels:   []string{"claude-sonnet-4-5", "claude-sonnet-4", "gpt-5"},
			DefaultModel:      "claude-sonnet-4-5",
		},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (c *CopilotAdapter) Name() string {
	return "copilot"
}

// Capabilities returns adapter capabilities.
func (c *CopilotAdapter) Capabilities() core.Capabilities {
	return c.capabilities
}

// Ping checks if Copilot CLI is available.
func (c *CopilotAdapter) Ping(ctx context.Context) error {
	// Check copilot is installed
	path := strings.Fields(c.config.Path)[0]
	_, err := exec.LookPath(path)
	if err != nil {
		return core.ErrNotFound("CLI", "copilot")
	}

	// Check copilot responds to --version or help
	// #nosec G204 -- path is from trusted config
	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		// Try help as fallback
		// #nosec G204 -- path is from trusted config
		cmd = exec.CommandContext(ctx, path, "help")
		if err := cmd.Run(); err != nil {
			return core.ErrNotFound("CLI", "copilot")
		}
	}

	return nil
}

// Execute runs a prompt through Copilot CLI.
func (c *CopilotAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := c.buildArgs(opts)

	// Create command
	cmdParts := strings.Fields(c.config.Path)
	allArgs := make([]string, 0, len(cmdParts[1:])+len(args))
	allArgs = append(allArgs, cmdParts[1:]...)
	allArgs = append(allArgs, args...)

	// Add prompt
	allArgs = append(allArgs, "-p", opts.Prompt)

	// #nosec G204 -- command path is from trusted config
	cmd := exec.CommandContext(ctx, cmdParts[0], allArgs...)
	cmd.Dir = opts.WorkDir
	cmd.Env = os.Environ()

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = c.config.Timeout
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	startTime := time.Now()

	// Run command
	err := cmd.Run()
	duration := time.Since(startTime)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, core.ErrTimeout(fmt.Sprintf("copilot timed out after %v", timeout))
	}

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	execResult, parseErr := c.parseOutput(result, opts.Format)
	if parseErr != nil {
		return nil, parseErr
	}

	// Extract usage information
	c.extractUsage(result, execResult)

	if err != nil && execResult.Output == "" {
		return execResult, fmt.Errorf("copilot execution failed: %w", err)
	}

	return execResult, nil
}

// buildArgs constructs CLI arguments for Copilot.
func (c *CopilotAdapter) buildArgs(opts core.ExecuteOptions) []string {
	args := []string{}

	// Model selection - Copilot CLI uses /model slash command or config file
	// Model passed via opts is stored for tracking but not sent as CLI flag
	_ = opts.Model // Acknowledge model selection (used for tracking/logging)

	// YOLO mode - auto-approve all tools for non-interactive execution
	args = append(args, "--allow-all-tools")

	// Allow all paths and URLs for full access
	args = append(args, "--allow-all-paths")
	args = append(args, "--allow-all-urls")

	// Output format
	if opts.Format == core.OutputFormatJSON {
		args = append(args, "--output-format", "json")
	}

	return args
}

// parseOutput parses Copilot CLI output.
func (c *CopilotAdapter) parseOutput(result *CommandResult, format core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	// Clean ANSI escape sequences
	output = c.cleanANSI(output)

	execResult := &core.ExecuteResult{
		Output:   strings.TrimSpace(output),
		Duration: result.Duration,
	}

	// Try to parse JSON if requested
	if format == core.OutputFormatJSON && output != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(output), &parsed); err == nil {
			execResult.Parsed = parsed
		}
	}

	return execResult, nil
}

// extractUsage extracts token and cost information from output.
func (c *CopilotAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Look for token patterns
	tokenPatterns := []struct {
		pattern string
		field   *int
	}{
		{`input[_\s]?tokens?:?\s*(\d+)`, &execResult.TokensIn},
		{`output[_\s]?tokens?:?\s*(\d+)`, &execResult.TokensOut},
		{`prompt[_\s]?tokens?:?\s*(\d+)`, &execResult.TokensIn},
		{`completion[_\s]?tokens?:?\s*(\d+)`, &execResult.TokensOut},
	}

	for _, tp := range tokenPatterns {
		re := regexp.MustCompile(`(?i)` + tp.pattern)
		if matches := re.FindStringSubmatch(combined); len(matches) > 1 {
			if val, err := strconv.Atoi(matches[1]); err == nil {
				*tp.field = val
			}
		}
	}

	// Estimate cost (Copilot is subscription-based, but we estimate for tracking)
	execResult.CostUSD = c.estimateCost(execResult.TokensIn, execResult.TokensOut)
}

// estimateCost provides rough cost estimate.
// Note: Copilot CLI is included in GitHub Copilot subscription, so actual cost is $0
// but we track usage for comparison purposes using Claude Sonnet pricing as proxy.
func (c *CopilotAdapter) estimateCost(tokensIn, tokensOut int) float64 {
	// Using Claude Sonnet 4.5 pricing as baseline since that's the default model
	// Input: $3/MTok, Output: $15/MTok
	inputCost := float64(tokensIn) * 3.0 / 1_000_000
	outputCost := float64(tokensOut) * 15.0 / 1_000_000
	return inputCost + outputCost
}

// cleanANSI removes ANSI escape sequences from output.
func (c *CopilotAdapter) cleanANSI(s string) string {
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiPattern.ReplaceAllString(s, "")
}

// estimateTokens provides rough token estimate.
func (c *CopilotAdapter) estimateTokens(text string) int {
	return len(text) / 4
}

// Config returns the adapter configuration.
func (c *CopilotAdapter) Config() AgentConfig {
	return c.config
}

// Ensure CopilotAdapter implements core.Agent
var _ core.Agent = (*CopilotAdapter)(nil)
