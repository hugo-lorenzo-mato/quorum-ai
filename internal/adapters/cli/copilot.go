package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// CopilotAdapter implements Agent for GitHub Copilot CLI.
// Note: PTY support requires the creack/pty package, but for simplicity
// we implement a fallback approach using standard I/O.
type CopilotAdapter struct {
	config       AgentConfig
	logger       *logging.Logger
	capabilities core.Capabilities
}

// NewCopilotAdapter creates a new Copilot adapter.
func NewCopilotAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "gh copilot"
	}

	logger := logging.NewNop().With("adapter", "copilot")

	adapter := &CopilotAdapter{
		config: cfg,
		logger: logger,
		capabilities: core.Capabilities{
			SupportsJSON:      false,
			SupportsStreaming: true,
			SupportsImages:    false,
			SupportsTools:     false,
			MaxContextTokens:  8000,
			MaxOutputTokens:   2048,
			SupportedModels:   []string{},
			DefaultModel:      "",
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
	// Check gh is installed
	_, err := exec.LookPath("gh")
	if err != nil {
		return core.ErrNotFound("CLI", "gh")
	}

	// Check copilot extension
	cmd := exec.CommandContext(ctx, "gh", "copilot", "--help")
	if err := cmd.Run(); err != nil {
		return core.ErrNotFound("extension", "gh-copilot")
	}

	return nil
}

// Execute runs a prompt through Copilot CLI.
// This uses standard I/O rather than PTY for simplicity.
func (c *CopilotAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	args := c.buildArgs(opts)

	// Create command
	cmdParts := strings.Fields(c.config.Path)
	allArgs := make([]string, 0, len(cmdParts[1:])+len(args))
	allArgs = append(allArgs, cmdParts[1:]...)
	allArgs = append(allArgs, args...)

	cmd := exec.CommandContext(ctx, cmdParts[0], allArgs...) //nolint:gosec // Command path is from trusted config
	cmd.Dir = opts.WorkDir
	cmd.Env = os.Environ()

	// Provide the prompt via stdin
	cmd.Stdin = strings.NewReader(opts.Prompt + "\n")

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

	output := stdout.String()

	// Clean ANSI escape sequences
	output = c.cleanANSI(output)

	// Extract suggestion
	suggestion := c.extractSuggestion(output)

	execResult := &core.ExecuteResult{
		Output:    suggestion,
		Duration:  duration,
		TokensIn:  c.estimateTokens(opts.Prompt),
		TokensOut: c.estimateTokens(suggestion),
		CostUSD:   0, // Copilot is included in GitHub subscription
	}

	if err != nil && suggestion == "" {
		return execResult, fmt.Errorf("copilot execution failed: %w", err)
	}

	return execResult, nil
}

// buildArgs constructs CLI arguments for Copilot.
func (c *CopilotAdapter) buildArgs(_ core.ExecuteOptions) []string {
	args := []string{"suggest"}

	// Specify shell type
	args = append(args, "-t", "shell")

	return args
}

// cleanANSI removes ANSI escape sequences from output.
func (c *CopilotAdapter) cleanANSI(s string) string {
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiPattern.ReplaceAllString(s, "")
}

// extractSuggestion extracts the actual suggestion from Copilot output.
func (c *CopilotAdapter) extractSuggestion(output string) string {
	// Look for suggestion markers
	lines := strings.Split(output, "\n")
	var suggestion []string
	capturing := false

	for _, line := range lines {
		if strings.Contains(line, "Suggestion:") {
			capturing = true
			continue
		}
		if capturing {
			// Stop at interactive prompts
			if strings.HasPrefix(line, "?") || strings.HasPrefix(line, "$") {
				break
			}
			suggestion = append(suggestion, line)
		}
	}

	if len(suggestion) > 0 {
		return strings.TrimSpace(strings.Join(suggestion, "\n"))
	}

	// If no suggestion markers found, return cleaned output
	return strings.TrimSpace(output)
}

// estimateTokens provides rough token estimate.
func (c *CopilotAdapter) estimateTokens(text string) int {
	return len(text) / 4
}

// isOutputComplete checks if Copilot has finished responding.
func (c *CopilotAdapter) isOutputComplete(output string) bool {
	// Look for completion markers
	completionMarkers := []string{
		"$ ",          // Shell prompt
		"Suggestion:", // Copilot output marker
		"? ",          // Interactive prompt
	}

	for _, marker := range completionMarkers {
		if strings.HasSuffix(strings.TrimSpace(output), marker) {
			return true
		}
	}

	return false
}

// Ensure CopilotAdapter implements core.Agent
var _ core.Agent = (*CopilotAdapter)(nil)
