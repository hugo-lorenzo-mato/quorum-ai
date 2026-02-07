package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// LogCallback is called for each line of stderr output during execution.
// This enables real-time visibility into agent progress.
type LogCallback func(line string)

// AgentConfig holds adapter configuration.
//
// Agent names are aliases - the Name field can be any identifier.
// The actual CLI type is determined by the Path field or the factory used.
// This allows defining multiple agent entries using the same CLI but with
// different models, which is useful for multi-agent analysis with CLIs like
// copilot that support multiple models (e.g., "copilot-claude", "copilot-gpt").
type AgentConfig struct {
	Name    string
	Path    string
	Model   string
	Timeout time.Duration
	WorkDir string
	// EnableStreaming enables real-time event streaming if supported
	EnableStreaming bool
	// Phases controls which workflow phases/roles this agent participates in.
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	// If nil, agent is available for all phases.
	Phases map[string]bool
	// ReasoningEffort is the default reasoning effort for all phases.
	// Codex: minimal, low, medium, high, xhigh.
	// Claude: low, medium, high, max (via CLAUDE_CODE_EFFORT_LEVEL env var, Opus 4.6 only).
	ReasoningEffort string
	// ReasoningEffortPhases allows per-phase overrides of reasoning effort.
	ReasoningEffortPhases map[string]string
	// TokenDiscrepancyThreshold is the ratio threshold for detecting token reporting errors.
	// If reported tokens differ from estimated by more than this factor, use estimated.
	// Default: 5 (meaning reported must be within 1/5 to 5x of estimated).
	// Set to 0 to disable discrepancy detection.
	TokenDiscrepancyThreshold float64
}

// DefaultTokenDiscrepancyThreshold is the default ratio for token discrepancy detection.
const DefaultTokenDiscrepancyThreshold = 5.0

// IsEnabledForPhase returns true if the agent is enabled for the given phase.
// Uses strict opt-in model: only phases explicitly set to true are enabled.
// If phases map is empty or missing, the agent is enabled for NO phases.
func (c AgentConfig) IsEnabledForPhase(phase string) bool {
	enabled, exists := c.Phases[phase]
	if !exists {
		return false // Phase not specified = disabled (opt-in model)
	}
	return enabled
}

// GetReasoningEffort returns the reasoning effort for a phase.
// Priority: phase-specific > default > empty (adapter uses hardcoded defaults).
func (c AgentConfig) GetReasoningEffort(phase string) string {
	// Check phase-specific override first
	if c.ReasoningEffortPhases != nil {
		if effort, ok := c.ReasoningEffortPhases[phase]; ok && effort != "" {
			return effort
		}
	}
	// Fall back to default
	return c.ReasoningEffort
}

// BaseAdapter provides common CLI execution functionality.
type BaseAdapter struct {
	config       AgentConfig
	logger       *logging.Logger
	logCallback  LogCallback
	eventHandler core.AgentEventHandler
	aggregator   *EventAggregator

	// ExtraEnv holds additional environment variables to set for command execution.
	// Values are applied on top of the current process environment.
	ExtraEnv map[string]string

	// Diagnostics integration for resource monitoring and crash recovery
	safeExec   *diagnostics.SafeExecutor
	dumpWriter *diagnostics.CrashDumpWriter
}

// SetLogCallback sets a callback that receives stderr lines in real-time.
func (b *BaseAdapter) SetLogCallback(cb LogCallback) {
	b.logCallback = cb
}

// SetEventHandler sets the handler that will receive streaming events.
func (b *BaseAdapter) SetEventHandler(handler core.AgentEventHandler) {
	b.eventHandler = handler
	if handler != nil && b.aggregator == nil {
		b.aggregator = NewEventAggregator()
	}
}

// WithDiagnostics configures the adapter with diagnostics support.
// When configured, the adapter will perform preflight checks before command execution
// and write crash dumps if panics occur.
func (b *BaseAdapter) WithDiagnostics(exec *diagnostics.SafeExecutor, dump *diagnostics.CrashDumpWriter) {
	b.safeExec = exec
	b.dumpWriter = dump
}

// emitEvent sends an event to the handler if one is configured.
func (b *BaseAdapter) emitEvent(event core.AgentEvent) {
	if b.eventHandler == nil {
		return
	}
	if b.aggregator != nil && !b.aggregator.ShouldEmit(event) {
		return
	}
	b.eventHandler(event)
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
// If a LogCallback is set, stderr lines are streamed in real-time.
// The optTimeout parameter allows overriding the default timeout; pass 0 to use config default.
func (b *BaseAdapter) ExecuteCommand(ctx context.Context, args []string, stdin, workDir string, optTimeout time.Duration) (*CommandResult, error) {
	// Apply timeout: prefer explicit timeout, then config, then default
	timeout := optTimeout
	if timeout == 0 {
		timeout = b.config.Timeout
	}
	if timeout == 0 {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Preflight check: verify system has sufficient resources
	if b.safeExec != nil {
		preflight := b.safeExec.RunPreflight()
		if !preflight.OK {
			return nil, core.ErrExecution("PREFLIGHT_FAILED",
				fmt.Sprintf("preflight check failed: %v", preflight.Errors))
		}
		for _, w := range preflight.Warnings {
			b.logger.Warn("preflight warning before command execution",
				"warning", w,
				"adapter", b.config.Name,
			)
		}
	}

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

	// Capture stdout in buffer
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Set up stderr: stream if callback exists, otherwise buffer
	var stderr bytes.Buffer
	var stderrPipe io.ReadCloser
	var pipeErr error

	if b.logCallback != nil {
		stderrPipe, pipeErr = cmd.StderrPipe()
		if pipeErr != nil {
			// Fall back to buffer if pipe fails
			cmd.Stderr = &stderr
			stderrPipe = nil
		}
	} else {
		cmd.Stderr = &stderr
	}

	// Set environment and identification
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "QUORUM_MANAGED=true", fmt.Sprintf("QUORUM_AGENT=%s", b.config.Name))

	// Apply extra environment variables (e.g. CLAUDE_CODE_EFFORT_LEVEL)
	for k, v := range b.ExtraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Log command execution start with truncated stdin for debugging
	stdinPreview := stdin
	if len(stdinPreview) > 500 {
		stdinPreview = stdinPreview[:500] + "... [truncated]"
	}
	b.logger.Info("cli: executing command",
		"adapter", b.config.Name,
		"path", cmdPath,
		"args", args,
		"work_dir", cmd.Dir,
		"stdin_length", len(stdin),
		"stdin_preview", stdinPreview,
		"timeout", timeout,
	)

	startTime := time.Now()

	// Start the command
	if err := cmd.Start(); err != nil {
		// CRITICAL: Close pipe if Start() fails to prevent FD leak
		if stderrPipe != nil {
			_ = stderrPipe.Close()
		}
		return nil, fmt.Errorf("starting command: %w", err)
	}

	b.logger.Info("cli: process started", "adapter", b.config.Name, "pid", cmd.Process.Pid)

	// Stream stderr if we have a pipe
	var wg sync.WaitGroup
	if stderrPipe != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.streamStderr(stderrPipe, &stderr, b.config.Name)
		}()
	}

	// Wait for command to complete
	err := cmd.Wait()

	// Wait for stderr streaming to finish
	wg.Wait()

	duration := time.Since(startTime)

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// Helper to truncate output for logging
	truncateForLog := func(s string, maxLen int) string {
		if len(s) > maxLen {
			return s[:maxLen] + "... [truncated]"
		}
		return s
	}

	if ctx.Err() == context.DeadlineExceeded {
		b.logger.Error("cli: command timeout",
			"adapter", b.config.Name,
			"path", cmdPath,
			"duration", duration,
			"timeout", timeout,
			"stdout_length", len(result.Stdout),
			"stderr_length", len(result.Stderr),
			"stderr_preview", truncateForLog(result.Stderr, 1000),
		)
		return result, core.ErrTimeout(fmt.Sprintf("command timed out after %v", timeout))
	}
	if ctx.Err() == context.Canceled {
		b.logger.Info("cli: command cancelled",
			"adapter", b.config.Name,
			"path", cmdPath,
			"duration", duration,
		)
		return result, core.ErrState("CANCELLED", "workflow cancelled by user")
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			b.logger.Error("cli: command failed",
				"adapter", b.config.Name,
				"path", cmdPath,
				"exit_code", result.ExitCode,
				"duration", duration,
				"stdout_length", len(result.Stdout),
				"stderr_length", len(result.Stderr),
				"stderr", truncateForLog(result.Stderr, 2000),
				"stdout_preview", truncateForLog(result.Stdout, 500),
			)
			return result, b.classifyError(result)
		}
		b.logger.Error("cli: command execution error",
			"adapter", b.config.Name,
			"path", cmdPath,
			"error", err,
			"duration", duration,
		)
		return result, fmt.Errorf("executing command: %w", err)
	}

	// Log successful completion
	b.logger.Info("cli: command completed",
		"adapter", b.config.Name,
		"path", cmdPath,
		"exit_code", 0,
		"duration", duration,
		"stdout_length", len(result.Stdout),
		"stderr_length", len(result.Stderr),
		"stdout_preview", truncateForLog(result.Stdout, 300),
	)

	result.ExitCode = 0
	return result, nil
}

// streamStderr reads stderr line by line, calling the callback for each line
// while also writing to the buffer for final capture.
// Also emits agent events based on common progress patterns.
func (b *BaseAdapter) streamStderr(pipe io.ReadCloser, buf *bytes.Buffer, adapterName string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		// Write to buffer for final result
		buf.WriteString(line)
		buf.WriteString("\n")
		// Call callback for real-time streaming
		if b.logCallback != nil {
			b.logCallback(line)
		}
		// Emit events based on common stderr patterns
		b.emitStderrEvent(adapterName, line)
	}
	// Ignore scanner errors - pipe may close abruptly on timeout
}

// emitStderrEvent parses stderr lines for progress indicators and emits appropriate events.
func (b *BaseAdapter) emitStderrEvent(adapterName, line string) {
	if b.eventHandler == nil {
		return
	}

	lineLower := strings.ToLower(line)

	// Tool/action patterns
	toolPatterns := []string{
		"reading", "writing", "executing", "running", "calling",
		"searching", "analyzing", "processing", "fetching", "loading",
		"tool:", "using tool", "function call", "bash:",
	}

	for _, pattern := range toolPatterns {
		if strings.Contains(lineLower, pattern) {
			// Extract a meaningful description from the line
			desc := line
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			b.emitEvent(core.NewAgentEvent(core.AgentEventToolUse, adapterName, desc))
			return
		}
	}

	// Thinking patterns
	thinkingPatterns := []string{
		"thinking", "reasoning", "considering", "evaluating",
	}

	for _, pattern := range thinkingPatterns {
		if strings.Contains(lineLower, pattern) {
			b.emitEvent(core.NewAgentEvent(core.AgentEventThinking, adapterName, "Thinking..."))
			return
		}
	}
}

// ExecuteWithStreaming runs a CLI command with real-time event streaming.
// It uses the appropriate streaming method based on the CLI's StreamConfig.
// The optTimeout parameter allows overriding the default timeout; pass 0 to use config default.
func (b *BaseAdapter) ExecuteWithStreaming(ctx context.Context, adapterName string, args []string, stdin, workDir string, optTimeout time.Duration) (*CommandResult, error) {
	streamCfg := GetStreamConfig(adapterName)
	parser := GetStreamParser(adapterName)

	// If no event handler or streaming not supported, fall back to normal execution
	if b.eventHandler == nil || streamCfg.Method == StreamMethodNone {
		return b.ExecuteCommand(ctx, args, stdin, workDir, optTimeout)
	}

	// Build command string for logging (before streaming args are added)
	cmdPath := b.config.Path
	cmdParts := strings.Fields(cmdPath)
	var fullCmd string
	if len(cmdParts) > 1 {
		fullCmd = cmdParts[0] + " " + strings.Join(append(cmdParts[1:], args...), " ")
	} else {
		fullCmd = cmdPath + " " + strings.Join(args, " ")
	}

	// Calculate actual timeout for event data
	actualTimeout := optTimeout
	if actualTimeout == 0 {
		actualTimeout = b.config.Timeout
	}
	if actualTimeout == 0 {
		actualTimeout = 3 * time.Hour
	}

	// Emit started event with command info and timeout
	b.emitEvent(core.NewAgentEvent(core.AgentEventStarted, adapterName, "Starting execution").
		WithData(map[string]any{
			"command":         fullCmd,
			"timeout_seconds": int(actualTimeout.Seconds()),
		}))

	switch streamCfg.Method {
	case StreamMethodJSONStdout:
		return b.executeWithJSONStreaming(ctx, adapterName, args, stdin, workDir, optTimeout, streamCfg, parser)
	case StreamMethodLogFile:
		return b.executeWithLogFileStreaming(ctx, adapterName, args, stdin, workDir, optTimeout, streamCfg, parser)
	default:
		return b.ExecuteCommand(ctx, args, stdin, workDir, optTimeout)
	}
}

// executeWithJSONStreaming handles CLIs that output JSON events to stdout.
func (b *BaseAdapter) executeWithJSONStreaming(
	ctx context.Context,
	adapterName string,
	args []string,
	stdin, workDir string,
	optTimeout time.Duration,
	cfg StreamConfig,
	parser StreamParser,
) (*CommandResult, error) {
	// Modify args to enable streaming output
	streamArgs := b.addStreamingArgs(args, cfg)

	// Apply timeout: prefer explicit timeout, then config, then default
	timeout := optTimeout
	if timeout == 0 {
		timeout = b.config.Timeout
	}
	if timeout == 0 {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Preflight check: verify system has sufficient resources
	if b.safeExec != nil {
		preflight := b.safeExec.RunPreflight()
		if !preflight.OK {
			return nil, core.ErrExecution("PREFLIGHT_FAILED",
				fmt.Sprintf("preflight check failed: %v", preflight.Errors))
		}
		for _, w := range preflight.Warnings {
			b.logger.Warn("preflight warning before streaming execution",
				"warning", w,
				"adapter", adapterName,
			)
		}
	}

	// Build command
	cmdPath := b.config.Path
	if cmdPath == "" {
		return nil, core.ErrValidation("NO_PATH", "adapter path not configured")
	}

	// Handle multi-word commands
	cmdParts := strings.Fields(cmdPath)
	if len(cmdParts) > 1 {
		cmdPath = cmdParts[0]
		streamArgs = append(cmdParts[1:], streamArgs...)
	}

	resolvedPath, err := exec.LookPath(cmdPath)
	if err != nil {
		return nil, fmt.Errorf("locating command: %w", err)
	}
	// #nosec G204 -- command path is resolved from config and validated via LookPath
	cmd := exec.CommandContext(ctx, resolvedPath, streamArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	} else if b.config.WorkDir != "" {
		cmd.Dir = b.config.WorkDir
	}

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Set up pipes for both stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		// CRITICAL: Close stdout pipe if stderr pipe creation fails
		_ = stdoutPipe.Close()
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "QUORUM_MANAGED=true", fmt.Sprintf("QUORUM_AGENT=%s", adapterName))

	// Apply extra environment variables (e.g. CLAUDE_CODE_EFFORT_LEVEL)
	for k, v := range b.ExtraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	b.logger.Debug("executing command with JSON streaming",
		"path", cmdPath,
		"args", streamArgs,
	)

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		// CRITICAL: Close both pipes if Start() fails to prevent FD leak
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		return nil, fmt.Errorf("starting command: %w", err)
	}

	b.logger.Info("cli: streaming process started", "adapter", adapterName, "pid", cmd.Process.Pid)

	// Stream both stdout and stderr
	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup

	// Stream stdout (JSON events)
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.streamJSONOutput(stdoutPipe, &stdout, adapterName, parser)
	}()

	// Stream stderr (progress messages)
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.streamStderr(stderrPipe, &stderr, adapterName)
	}()

	// Wait for command
	err = cmd.Wait()
	wg.Wait()

	duration := time.Since(startTime)

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// Emit completed or error event
	if ctx.Err() == context.DeadlineExceeded {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution timed out"))
		return result, core.ErrTimeout(fmt.Sprintf("command timed out after %v", timeout))
	}
	if ctx.Err() == context.Canceled {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution cancelled"))
		return result, core.ErrState("CANCELLED", "workflow cancelled by user")
	}

	if err != nil {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution failed"))
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, b.classifyError(result)
		}
		return result, fmt.Errorf("executing command: %w", err)
	}

	b.emitEvent(core.NewAgentEvent(core.AgentEventCompleted, adapterName, "Execution completed").WithData(map[string]any{
		"duration_ms": duration.Milliseconds(),
	}))

	result.ExitCode = 0
	return result, nil
}

// streamJSONOutput reads JSON events from stdout and parses them into AgentEvents.
// It extracts text content from the stream instead of storing raw JSON.
func (b *BaseAdapter) streamJSONOutput(pipe io.ReadCloser, buf *bytes.Buffer, _ string, parser StreamParser) {
	scanner := bufio.NewScanner(pipe)
	// Increase buffer size for large JSON lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Extract text content from JSON events instead of storing raw JSON
		text := extractTextFromJSONLine(line)
		if text != "" {
			buf.WriteString(text)
		}

		// Parse and emit events
		if parser != nil {
			events := parser.ParseLine(line)
			for _, event := range events {
				b.emitEvent(event)
			}
		}
	}
}

// extractTextFromJSONLine extracts human-readable text from a JSON stream line.
// It handles various event formats from different CLI tools (Claude, Gemini, Codex).
func extractTextFromJSONLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return ""
	}

	// Generic structure to parse any streaming event
	var event struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Result  string `json:"result"` // Claude/Gemini result
		Text    string `json:"text"`   // Gemini direct text
		Message *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"` // Claude assistant message
		Item *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"` // Codex item
		Response string `json:"response"` // Gemini final response
	}

	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return ""
	}

	// Handle result events (final output)
	if event.Type == "result" && event.Subtype == "success" {
		if event.Result != "" {
			return event.Result
		}
		if event.Response != "" {
			return event.Response
		}
	}

	// Handle Claude assistant message with text content
	if event.Type == "assistant" && event.Message != nil {
		for _, content := range event.Message.Content {
			if content.Type == "text" && content.Text != "" {
				return content.Text
			}
		}
	}

	// Handle Gemini direct text events
	if event.Type == "text" && event.Text != "" {
		return event.Text
	}

	// Handle Codex agent_message
	if event.Type == "item.completed" && event.Item != nil {
		if event.Item.Type == "agent_message" && event.Item.Text != "" {
			return event.Item.Text
		}
	}

	return ""
}

// executeWithLogFileStreaming handles CLIs that write logs to a file (like Copilot).
func (b *BaseAdapter) executeWithLogFileStreaming(
	ctx context.Context,
	adapterName string,
	args []string,
	stdin, workDir string,
	optTimeout time.Duration,
	cfg StreamConfig,
	parser StreamParser,
) (*CommandResult, error) {
	// Create temporary log directory
	logDir, err := os.MkdirTemp("", "quorum-logs-*")
	if err != nil {
		// Fall back to normal execution
		return b.ExecuteCommand(ctx, args, stdin, workDir, optTimeout)
	}
	defer os.RemoveAll(logDir)

	// Add log flags to args (create new slice to avoid modifying original)
	logArgs := make([]string, len(args), len(args)+4)
	copy(logArgs, args)
	logArgs = append(logArgs, cfg.LogDirFlag, logDir)
	if cfg.LogLevelFlag != "" && cfg.LogLevelValue != "" {
		logArgs = append(logArgs, cfg.LogLevelFlag, cfg.LogLevelValue)
	}

	// Apply timeout: prefer explicit timeout, then config, then default
	timeout := optTimeout
	if timeout == 0 {
		timeout = b.config.Timeout
	}
	if timeout == 0 {
		timeout = 3 * time.Hour
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	cmdPath := b.config.Path
	if cmdPath == "" {
		return nil, core.ErrValidation("NO_PATH", "adapter path not configured")
	}

	cmdParts := strings.Fields(cmdPath)
	if len(cmdParts) > 1 {
		cmdPath = cmdParts[0]
		logArgs = append(cmdParts[1:], logArgs...)
	}

	resolvedPath, err := exec.LookPath(cmdPath)
	if err != nil {
		return nil, fmt.Errorf("locating command: %w", err)
	}
	// #nosec G204 -- command path is resolved from config and validated via LookPath
	cmd := exec.CommandContext(ctx, resolvedPath, logArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	} else if b.config.WorkDir != "" {
		cmd.Dir = b.config.WorkDir
	}

	// Apply extra environment variables if set
	if len(b.ExtraEnv) > 0 {
		cmd.Env = os.Environ()
		for k, v := range b.ExtraEnv {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "QUORUM_MANAGED=true", fmt.Sprintf("QUORUM_AGENT=%s", adapterName))

	b.logger.Debug("executing command with log file streaming",
		"path", cmdPath,
		"args", logArgs,
		"log_dir", logDir,
	)

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

	b.logger.Info("cli: log-streaming process started", "adapter", adapterName, "pid", cmd.Process.Pid)

	// Start log file tailer in background
	stopTail := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.tailLogFiles(ctx, logDir, adapterName, parser, stopTail)
	}()

	// Wait for command
	err = cmd.Wait()

	// Stop tailing
	close(stopTail)
	wg.Wait()

	duration := time.Since(startTime)

	result := &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution timed out"))
		return result, core.ErrTimeout(fmt.Sprintf("command timed out after %v", timeout))
	}
	if ctx.Err() == context.Canceled {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution cancelled"))
		return result, core.ErrState("CANCELLED", "workflow cancelled by user")
	}

	if err != nil {
		b.emitEvent(core.NewAgentEvent(core.AgentEventError, adapterName, "Execution failed"))
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, b.classifyError(result)
		}
		return result, fmt.Errorf("executing command: %w", err)
	}

	b.emitEvent(core.NewAgentEvent(core.AgentEventCompleted, adapterName, "Execution completed").WithData(map[string]any{
		"duration_ms": duration.Milliseconds(),
	}))

	result.ExitCode = 0
	return result, nil
}

// tailLogFiles watches for new log files and tails them for events.
func (b *BaseAdapter) tailLogFiles(ctx context.Context, logDir, adapterName string, parser StreamParser, stop <-chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	seenFiles := make(map[string]int64) // filename -> last read position

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			// List log files
			entries, err := os.ReadDir(logDir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				name := entry.Name()
				if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".txt") {
					continue
				}

				filePath := logDir + "/" + name
				b.readNewLogContent(logDir, filePath, seenFiles, adapterName, parser)
			}
		}
	}
}

// readNewLogContent reads new content from a log file since last read.
func (b *BaseAdapter) readNewLogContent(logDir, filePath string, seenFiles map[string]int64, _ string, parser StreamParser) {
	if !pathWithin(logDir, filePath) {
		return
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return
	}

	lastPos := seenFiles[filePath]
	currentSize := info.Size()

	if currentSize <= lastPos {
		return // No new content
	}

	// #nosec G304 -- filePath is validated to be within logDir
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to last position
	if lastPos > 0 {
		_, _ = file.Seek(lastPos, 0)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if parser != nil {
			events := parser.ParseLine(line)
			for _, event := range events {
				b.emitEvent(event)
			}
		}
	}

	seenFiles[filePath] = currentSize
}

func pathWithin(baseDir, target string) bool {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	sep := string(os.PathSeparator)
	return !strings.HasPrefix(rel, ".."+sep) && rel != ".."
}

// addStreamingArgs adds the necessary flags for streaming output.
func (b *BaseAdapter) addStreamingArgs(args []string, cfg StreamConfig) []string {
	result := make([]string, len(args))
	copy(result, args)

	// Add output format flag
	if cfg.OutputFormatFlag != "" {
		if cfg.OutputFormatValue != "" {
			result = append(result, cfg.OutputFormatFlag, cfg.OutputFormatValue)
		} else {
			// Boolean flag (like Codex's --json)
			result = append(result, cfg.OutputFormatFlag)
		}
	}

	// Add any required flags
	result = append(result, cfg.RequiredFlags...)

	return result
}

// classifyError converts command errors to domain errors.
func (b *BaseAdapter) classifyError(result *CommandResult) error {
	// Try to get error message from stderr first, then stdout
	errorMsg := strings.TrimSpace(result.Stderr)
	if errorMsg == "" {
		// Try to extract error from stdout (some CLIs output errors as JSON to stdout)
		errorMsg = extractErrorFromOutput(result.Stdout)
	}
	if errorMsg == "" {
		errorMsg = "(no error message captured)"
	}

	errorMsgLower := strings.ToLower(errorMsg)

	// Rate limit detection
	if containsAny(errorMsgLower, []string{"rate limit", "too many requests", "429", "quota"}) {
		return core.ErrRateLimit(errorMsg)
	}

	// Authentication errors
	if containsAny(errorMsgLower, []string{"unauthorized", "authentication", "api key", "token"}) {
		return core.ErrAuth(errorMsg)
	}

	// Network errors
	if containsAny(errorMsgLower, []string{"connection", "network", "timeout", "unreachable"}) {
		return core.ErrExecution("NETWORK", errorMsg)
	}

	// Generic execution error
	return core.ErrExecution("CLI_ERROR",
		fmt.Sprintf("command failed with exit code %d: %s", result.ExitCode, errorMsg),
	)
}

// extractErrorFromOutput tries to extract error messages from stdout.
// Many CLIs output JSON with error fields to stdout.
func extractErrorFromOutput(stdout string) string {
	// Try to find JSON error objects in the output
	lines := strings.Split(stdout, "\n")
	for i := len(lines) - 1; i >= 0; i-- { // Start from end, errors often at the end
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}

		// Try to parse as JSON and extract error
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// Check for common error fields
		if errMsg, ok := obj["error"].(string); ok && errMsg != "" {
			return errMsg
		}
		if errObj, ok := obj["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				return msg
			}
		}
		// Claude CLI format: {"type":"result","subtype":"error","error":"..."}
		if objType, ok := obj["type"].(string); ok && objType == "result" {
			if subtype, ok := obj["subtype"].(string); ok && subtype == "error" {
				if errMsg, ok := obj["error"].(string); ok && errMsg != "" {
					return errMsg
				}
			}
		}
		// Claude CLI format: {"type":"error","error":"..."}
		if objType, ok := obj["type"].(string); ok && objType == "error" {
			if errMsg, ok := obj["error"].(string); ok && errMsg != "" {
				return errMsg
			}
		}
	}

	// If no JSON error found, return last non-empty line as fallback
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "{") {
			// Limit length
			if len(line) > 200 {
				return line[:200] + "..."
			}
			return line
		}
	}

	return ""
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
	result, err := b.ExecuteCommand(ctx, []string{versionArg}, "", "", 0)
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

// truncateForDebug truncates a string for debug output.
func truncateForDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
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
