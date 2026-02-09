package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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
			SupportedModels:   core.GetSupportedModels(core.AgentCopilot),
			DefaultModel:      core.GetDefaultModel(core.AgentCopilot),
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
	args := c.buildArgs(opts)

	// Create command
	cmdParts := strings.Fields(c.config.Path)
	allArgs := make([]string, 0, len(cmdParts[1:])+len(args))
	allArgs = append(allArgs, cmdParts[1:]...)
	allArgs = append(allArgs, args...)

	// Copilot CLI doesn't have --system-prompt, so prepend to user prompt
	// Pass via stdin for robustness with long prompts and special characters
	prompt := opts.Prompt
	if opts.SystemPrompt != "" && prompt != "" {
		prompt = "[System Instructions]\n" + opts.SystemPrompt + "\n\n[User Message]\n" + prompt
	}

	// Set timeout: prefer explicit timeout, then config, then default
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = c.config.Timeout
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// Emit started event with timeout info
	c.emitEvent(core.NewAgentEvent(core.AgentEventStarted, "copilot", "Starting execution").
		WithData(map[string]any{"timeout_seconds": int(timeout.Seconds())}))

	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- command path is from trusted config
	cmd := exec.CommandContext(ctx, cmdParts[0], allArgs...)
	cmd.Dir = opts.WorkDir
	cmd.Env = os.Environ()

	// Pass prompt via stdin
	if prompt != "" {
		cmd.Stdin = strings.NewReader(prompt)
	}

	// Use pipes to stream stdout in real-time
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return nil, core.ErrTimeout(fmt.Sprintf("starting command: %v (context: %v)", err, ctx.Err()))
		}
		return nil, fmt.Errorf("starting command: %w", err)
	}

	// Stream stdout and detect activity patterns
	var stdout bytes.Buffer
	c.streamStdoutWithEvents(stdoutPipe, &stdout)

	// Wait for command
	err = cmd.Wait()
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

// streamStdoutWithEvents reads stdout line by line and emits each line as a progress event.
// This gives real-time visibility into what Copilot is doing.
func (c *CopilotAdapter) streamStdoutWithEvents(pipe io.ReadCloser, buf *bytes.Buffer) {
	scanner := bufio.NewScanner(pipe)

	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")

		// Skip empty lines and stats output
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip statistics lines at the end
		if strings.HasPrefix(trimmed, "Total usage") ||
			strings.HasPrefix(trimmed, "Total duration") ||
			strings.HasPrefix(trimmed, "Total code changes") ||
			strings.HasPrefix(trimmed, "Usage by model") {
			continue
		}

		// Emit the line as progress (truncate if too long)
		activity := trimmed
		if len(activity) > 60 {
			activity = activity[:57] + "..."
		}
		c.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", activity))
	}
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

	// Silent mode - output only agent response without stats (reduces meta-information)
	args = append(args, "--silent")

	// Note: Copilot CLI does not support --output-format json or stream-json.
	// Streaming is handled via log files (see streaming.go StreamMethodLogFile).

	return args
}

// parseOutput parses Copilot CLI output.
func (c *CopilotAdapter) parseOutput(result *CommandResult, _ core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	// Clean ANSI escape sequences
	output = c.cleanANSI(output)

	execResult := &core.ExecuteResult{
		Output:   strings.TrimSpace(output),
		Duration: result.Duration,
	}

	return execResult, nil
}

// extractUsage extracts token and cost information from output.
func (c *CopilotAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Debug: track source of token values
	var tokenSource string

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
				tokenSource = "parsed"
			}
		}
	}

	// Estimate tokens from output length for comparison/fallback
	estimatedTokensOut := c.estimateTokens(execResult.Output)

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
				"copilot",
				fmt.Sprintf("[WARN] Token discrepancy (too low): reported=%d, estimated=%d (threshold=%.1fx). Using estimated.",
					execResult.TokensOut, estimatedTokensOut, threshold),
			).WithData(map[string]any{
				"reported_tokens":  execResult.TokensOut,
				"estimated_tokens": estimatedTokensOut,
				"output_length":    len(execResult.Output),
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
				"copilot",
				fmt.Sprintf("[WARN] Token discrepancy (too high): reported=%d, estimated=%d (threshold=%.1fx). Using estimated.",
					execResult.TokensOut, estimatedTokensOut, threshold),
			).WithData(map[string]any{
				"reported_tokens":  execResult.TokensOut,
				"estimated_tokens": estimatedTokensOut,
				"output_length":    len(execResult.Output),
				"threshold":        threshold,
				"source":           tokenSource,
				"action":           "using_estimated",
				"discrepancy_type": "too_high",
			}))
			execResult.TokensOut = estimatedTokensOut
			tokenSource = "estimated_discrepancy"
		}
	}

	// Fallback: estimate tokens if not found in output
	// Copilot CLI doesn't always report tokens, so we estimate based on content
	if execResult.TokensIn == 0 && execResult.TokensOut == 0 {
		// Estimate output tokens from response (roughly 4 chars per token)
		execResult.TokensOut = estimatedTokensOut
		tokenSource = "estimated"
		// Estimate input tokens as ~30% of output for typical prompts
		if execResult.TokensOut > 0 {
			execResult.TokensIn = execResult.TokensOut / 3
			if execResult.TokensIn < 10 {
				execResult.TokensIn = 10
			}
		}
	}

	// Cap token values to avoid corrupted/unrealistic values
	// Max reasonable is ~500k (very large context + response)
	const maxReasonableTokens = 500_000
	if execResult.TokensIn > maxReasonableTokens {
		c.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"copilot",
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
			"copilot",
			fmt.Sprintf("[WARN] Capped unrealistic TokensOut: %d -> %d", execResult.TokensOut, maxReasonableTokens),
		).WithData(map[string]any{
			"original":      execResult.TokensOut,
			"capped":        maxReasonableTokens,
			"source":        tokenSource,
			"stdout_sample": truncateForDebug(result.Stdout, 200),
		}))
		execResult.TokensOut = maxReasonableTokens
	}

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
