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
	eventHandler core.AgentEventHandler
	aggregator   *EventAggregator
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
			SupportsJSON:      false, // Copilot CLI does not support --output-format json
			SupportsStreaming: true,
			SupportsImages:    false,
			SupportsTools:     true,
			MaxContextTokens:  200000,
			MaxOutputTokens:   16384,
			SupportedModels: []string{
				"claude-sonnet-4.5",
				"claude-haiku-4.5",
				"claude-opus-4.5",
				"claude-sonnet-4",
				"gpt-5.2-codex",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.2",
				"gpt-5.1",
				"gpt-5",
				"gpt-5.1-codex-mini",
				"gpt-5-mini",
				"gpt-4.1",
				"gemini-3-pro-preview",
			},
			DefaultModel: "claude-sonnet-4.5",
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

// SetEventHandler sets the handler for streaming events.
func (c *CopilotAdapter) SetEventHandler(handler core.AgentEventHandler) {
	c.eventHandler = handler
	if handler != nil && c.aggregator == nil {
		c.aggregator = NewEventAggregator()
	}
}

// emitEvent sends an event to the handler if one is configured.
func (c *CopilotAdapter) emitEvent(event core.AgentEvent) {
	if c.eventHandler == nil {
		return
	}
	if c.aggregator != nil && !c.aggregator.ShouldEmit(event) {
		return
	}
	c.eventHandler(event)
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
	// Emit started event
	c.emitEvent(core.NewAgentEvent(core.AgentEventStarted, "copilot", "Starting execution"))

	args := c.buildArgs(opts)

	// Create command
	cmdParts := strings.Fields(c.config.Path)
	allArgs := make([]string, 0, len(cmdParts[1:])+len(args))
	allArgs = append(allArgs, cmdParts[1:]...)
	allArgs = append(allArgs, args...)

	// Copilot CLI doesn't have --system-prompt, so prepend to user prompt
	prompt := opts.Prompt
	if opts.SystemPrompt != "" && prompt != "" {
		prompt = "[System Instructions]\n" + opts.SystemPrompt + "\n\n[User Message]\n" + prompt
	}

	// Add prompt
	allArgs = append(allArgs, "-p", prompt)

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

	// Emit progress event
	c.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", "Processing request..."))

	// Run command
	err := cmd.Run()
	duration := time.Since(startTime)

	if ctx.Err() == context.DeadlineExceeded {
		c.emitEvent(core.NewAgentEvent(core.AgentEventError, "copilot", "Execution timed out"))
		return nil, core.ErrTimeout(fmt.Sprintf("copilot timed out after %v", timeout))
	}

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	execResult, parseErr := c.parseOutput(result, opts.Format)
	if parseErr != nil {
		c.emitEvent(core.NewAgentEvent(core.AgentEventError, "copilot", "Failed to parse output"))
		return nil, parseErr
	}

	// Extract usage information
	c.extractUsage(result, execResult)

	if err != nil && execResult.Output == "" {
		errMsg := fmt.Sprintf("copilot execution failed: %v", err)
		if result.Stderr != "" {
			errMsg = fmt.Sprintf("%s\nstderr: %s", errMsg, strings.TrimSpace(result.Stderr))
		}
		c.emitEvent(core.NewAgentEvent(core.AgentEventError, "copilot", "Execution failed"))
		return execResult, fmt.Errorf("%s", errMsg)
	}

	// Emit completed event
	c.emitEvent(core.NewAgentEvent(core.AgentEventCompleted, "copilot", "Execution completed").WithData(map[string]any{
		"duration_ms": duration.Milliseconds(),
		"tokens_in":   execResult.TokensIn,
		"tokens_out":  execResult.TokensOut,
	}))

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

	// Silent mode for cleaner output (only agent response, no stats)
	args = append(args, "--silent")

	// Note: Copilot CLI does not support --output-format json.
	// JSON output format requested via opts.Format is acknowledged but not applied.
	// The output will be plain text and parsed as-is.
	_ = opts.Format

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

	// Fallback: estimate tokens if not found in output
	// Copilot CLI doesn't always report tokens, so we estimate based on content
	if execResult.TokensIn == 0 && execResult.TokensOut == 0 {
		// Estimate output tokens from response (roughly 4 chars per token)
		execResult.TokensOut = c.estimateTokens(execResult.Output)
		// Estimate input tokens as ~30% of output for typical prompts
		if execResult.TokensOut > 0 {
			execResult.TokensIn = execResult.TokensOut / 3
			if execResult.TokensIn < 10 {
				execResult.TokensIn = 10
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

// Ensure CopilotAdapter implements core.Agent and core.StreamingCapable
var _ core.Agent = (*CopilotAdapter)(nil)
var _ core.StreamingCapable = (*CopilotAdapter)(nil)
