package cli

import (
	"context"
	"fmt"
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
			SupportedModels:   core.GetSupportedModels(core.AgentClaude),
			DefaultModel:      core.GetDefaultModel(core.AgentClaude),
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

// SetLogCallback sets a callback for real-time stderr streaming.
func (c *ClaudeAdapter) SetLogCallback(cb LogCallback) {
	c.BaseAdapter.SetLogCallback(cb)
}

// SetEventHandler sets the handler for streaming events.
func (c *ClaudeAdapter) SetEventHandler(handler core.AgentEventHandler) {
	c.BaseAdapter.SetEventHandler(handler)
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

	// Set Claude effort level via env var if reasoning effort is configured.
	// Claude CLI uses CLAUDE_CODE_EFFORT_LEVEL env var (low/medium/high/max).
	c.applyEffortEnv(opts)

	// Build the full prompt, including conversation history if provided
	// Pass via stdin for robustness with long prompts and special characters
	fullPrompt := c.buildPromptWithHistory(opts)

	// Use streaming execution if event handler is configured
	var result *CommandResult
	var err error
	if c.eventHandler != nil {
		result, err = c.ExecuteWithStreaming(ctx, "claude", args, fullPrompt, opts.WorkDir, opts.Timeout)
	} else {
		result, err = c.ExecuteCommand(ctx, args, fullPrompt, opts.WorkDir, opts.Timeout)
	}
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

	// System prompt (append to default system prompt for customizing assistant behavior)
	if opts.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.SystemPrompt)
	}

	// Note: --output-format stream-json is added by ExecuteWithStreaming via streaming config
	// This enables real-time progress monitoring while the LLM writes output files directly

	// Auto-accept for non-interactive mode
	args = append(args, "--dangerously-skip-permissions")

	return args
}

// parseOutput parses Claude CLI output.
func (c *ClaudeAdapter) parseOutput(result *CommandResult, _ core.OutputFormat) (*core.ExecuteResult, error) {
	output := result.Stdout

	execResult := &core.ExecuteResult{
		Output:   output,
		Duration: result.Duration,
	}

	// Try to extract usage information from stderr or output
	c.extractUsage(result, execResult)

	return execResult, nil
}

// extractUsage attempts to extract token usage from output.
func (c *ClaudeAdapter) extractUsage(result *CommandResult, execResult *core.ExecuteResult) {
	combined := result.Stdout + result.Stderr

	// Debug: track source of token values
	var tokenSource string

	// Pattern for token usage
	// Example: "tokens: 1234 in, 567 out"
	tokenPattern := regexp.MustCompile(`tokens?:?\s*(\d+)\s*in\D*(\d+)\s*out`)
	if matches := tokenPattern.FindStringSubmatch(combined); len(matches) == 3 {
		if in, err := strconv.Atoi(matches[1]); err == nil {
			execResult.TokensIn = in
			tokenSource = "parsed"
		}
		if out, err := strconv.Atoi(matches[2]); err == nil {
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
			"claude",
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
			"claude",
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

// applyEffortEnv sets the CLAUDE_CODE_EFFORT_LEVEL env var based on reasoning effort config.
// Priority: per-message opts > phase-specific config > default config.
func (c *ClaudeAdapter) applyEffortEnv(opts core.ExecuteOptions) {
	effort := c.getEffortLevel(opts)
	if effort == "" {
		return
	}

	// Resolve the model to normalize effort
	model := opts.Model
	if model == "" {
		model = c.config.Model
	}

	// Normalize to Claude-supported effort level
	effort = core.NormalizeClaudeEffort(model, effort)
	if effort == "" {
		return
	}

	if c.ExtraEnv == nil {
		c.ExtraEnv = make(map[string]string)
	}
	c.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"] = effort
}

// getEffortLevel determines the effort level from options and config.
// Priority: per-message > phase-specific > default config.
func (c *ClaudeAdapter) getEffortLevel(opts core.ExecuteOptions) string {
	// Per-message override (from chat or workflow)
	if opts.ReasoningEffort != "" {
		return opts.ReasoningEffort
	}

	// Phase-specific override
	if opts.Phase != "" {
		if effort := c.config.GetReasoningEffort(string(opts.Phase)); effort != "" {
			return effort
		}
	}

	// Default reasoning effort from config
	return c.config.ReasoningEffort
}

// Ensure ClaudeAdapter implements core.Agent and core.StreamingCapable
var _ core.Agent = (*ClaudeAdapter)(nil)
var _ core.StreamingCapable = (*ClaudeAdapter)(nil)
