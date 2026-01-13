package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// AgentConfig holds adapter configuration.
type AgentConfig struct {
	Name        string
	Path        string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
	WorkDir     string
}

// BaseAdapter provides common CLI execution functionality.
type BaseAdapter struct {
	config AgentConfig
	logger *logging.Logger
}

// NewBaseAdapter creates a new base adapter.
func NewBaseAdapter(cfg AgentConfig, logger *logging.Logger) *BaseAdapter {
	if logger == nil {
		logger = logging.NewNop()
	}
	return &BaseAdapter{
		config: cfg,
		logger: logger,
	}
}

// CommandResult holds the result of a CLI execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// ExecuteCommand runs a CLI command with the given options.
func (b *BaseAdapter) ExecuteCommand(ctx context.Context, args []string, stdin, workDir string) (*CommandResult, error) {
	// Apply timeout
	timeout := b.config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	cmdPath := b.config.Path
	if cmdPath == "" {
		return nil, core.ErrValidation("NO_PATH", "adapter path not configured")
	}

	// Handle multi-word commands (e.g., "gh copilot")
	cmdParts := strings.Fields(cmdPath)
	if len(cmdParts) > 1 {
		cmdPath = cmdParts[0]
		args = append(cmdParts[1:], args...)
	}

	// #nosec G204 -- command path and args come from validated config
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	if workDir != "" {
		cmd.Dir = workDir
	} else if b.config.WorkDir != "" {
		cmd.Dir = b.config.WorkDir
	}

	// Set up stdin
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	cmd.Env = os.Environ()

	b.logger.Debug("executing command",
		"path", cmdPath,
		"args", args,
		"work_dir", cmd.Dir,
	)

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, core.ErrTimeout(fmt.Sprintf("command timed out after %v", timeout))
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, b.classifyError(result)
		}
		return result, fmt.Errorf("executing command: %w", err)
	}

	result.ExitCode = 0
	return result, nil
}

// classifyError converts command errors to domain errors.
func (b *BaseAdapter) classifyError(result *CommandResult) error {
	stderr := strings.ToLower(result.Stderr)

	// Rate limit detection
	if containsAny(stderr, []string{"rate limit", "too many requests", "429", "quota"}) {
		return core.ErrRateLimit(result.Stderr)
	}

	// Authentication errors
	if containsAny(stderr, []string{"unauthorized", "authentication", "api key", "token"}) {
		return core.ErrAuth(result.Stderr)
	}

	// Network errors
	if containsAny(stderr, []string{"connection", "network", "timeout", "unreachable"}) {
		return core.ErrExecution("NETWORK", result.Stderr)
	}

	// Generic execution error
	return core.ErrExecution("CLI_ERROR",
		fmt.Sprintf("command failed with exit code %d: %s", result.ExitCode, result.Stderr),
	)
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ParseJSON attempts to parse JSON from output.
func (b *BaseAdapter) ParseJSON(output string, v interface{}) error {
	// Try direct parse
	if err := json.Unmarshal([]byte(output), v); err == nil {
		return nil
	}

	// Try to extract JSON from mixed output
	extracted := b.ExtractJSON(output)
	if extracted != "" {
		if err := json.Unmarshal([]byte(extracted), v); err == nil {
			return nil
		}
	}

	return fmt.Errorf("no valid JSON found in output")
}

// ExtractJSON finds and extracts JSON from mixed text output.
func (b *BaseAdapter) ExtractJSON(output string) string {
	// Try to find JSON object
	start := strings.Index(output, "{")
	if start == -1 {
		// Try JSON array
		start = strings.Index(output, "[")
	}
	if start == -1 {
		return ""
	}

	// Find matching end
	depth := 0
	inString := false
	escaped := false
	openChar := output[start]
	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}

	for i := start; i < len(output); i++ {
		c := output[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == openChar {
			depth++
		} else if c == closeChar {
			depth--
			if depth == 0 {
				return output[start : i+1]
			}
		}
	}

	return ""
}

// ExtractByPattern extracts content matching a regex pattern.
func (b *BaseAdapter) ExtractByPattern(output, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return re.FindAllString(output, -1), nil
}

// GetVersion retrieves the CLI version.
func (b *BaseAdapter) GetVersion(ctx context.Context, versionArg string) (string, error) {
	result, err := b.ExecuteCommand(ctx, []string{versionArg}, "", "")
	if err != nil {
		return "", err
	}

	// Extract version from output
	output := result.Stdout + result.Stderr
	versionPattern := `v?\d+\.\d+(\.\d+)?(-[a-zA-Z0-9]+)?`
	re := regexp.MustCompile(versionPattern)
	match := re.FindString(output)
	if match != "" {
		return match, nil
	}

	return strings.TrimSpace(output), nil
}

// CheckAvailability verifies the CLI is installed and accessible.
func (b *BaseAdapter) CheckAvailability(_ context.Context) error {
	cmdPath := b.config.Path
	if cmdPath == "" {
		return core.ErrValidation("NO_PATH", "adapter path not configured")
	}

	// Handle multi-word commands
	cmdParts := strings.Fields(cmdPath)
	cmdPath = cmdParts[0]

	// Check if command exists
	_, err := exec.LookPath(cmdPath)
	if err != nil {
		return core.ErrNotFound("CLI", cmdPath)
	}

	return nil
}

// TokenEstimate provides a rough token count estimate.
func (b *BaseAdapter) TokenEstimate(text string) int {
	// Rough estimate: ~4 characters per token for English
	return len(text) / 4
}

// TruncateToTokenLimit truncates text to approximately fit within token limit.
func (b *BaseAdapter) TruncateToTokenLimit(text string, maxTokens int) string {
	charLimit := maxTokens * 4
	if len(text) <= charLimit {
		return text
	}
	return text[:charLimit] + "\n...[truncated]"
}

// Config returns the adapter configuration.
func (b *BaseAdapter) Config() AgentConfig {
	return b.config
}
