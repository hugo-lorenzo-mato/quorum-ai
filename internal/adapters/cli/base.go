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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// LogCallback is called for each line of stderr output during execution.
// This enables real-time visibility into agent progress.
type LogCallback func(line string)

// AgentConfig holds adapter configuration.
type AgentConfig struct {
	Name        string
	Path        string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
	WorkDir     string
	// EnableStreaming enables real-time event streaming if supported
	EnableStreaming bool
}

// BaseAdapter provides common CLI execution functionality.
type BaseAdapter struct {
	config       AgentConfig
	logger       *logging.Logger
	logCallback  LogCallback
	eventHandler core.AgentEventHandler
	aggregator   *EventAggregator
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

	// Set environment
	cmd.Env = os.Environ()

	b.logger.Debug("executing command",
		"path", cmdPath,
		"args", args,
		"work_dir", cmd.Dir,
	)

	startTime := time.Now()

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

	// Stream stderr if we have a pipe
	var wg sync.WaitGroup
	if stderrPipe != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.streamStderr(stderrPipe, &stderr)
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

// streamStderr reads stderr line by line, calling the callback for each line
// while also writing to the buffer for final capture.
func (b *BaseAdapter) streamStderr(pipe io.ReadCloser, buf *bytes.Buffer) {
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
	}
	// Ignore scanner errors - pipe may close abruptly on timeout
}

// ExecuteWithStreaming runs a CLI command with real-time event streaming.
// It uses the appropriate streaming method based on the CLI's StreamConfig.
func (b *BaseAdapter) ExecuteWithStreaming(ctx context.Context, adapterName string, args []string, stdin, workDir string) (*CommandResult, error) {
	streamCfg := GetStreamConfig(adapterName)
	parser := GetStreamParser(adapterName)

	// If no event handler or streaming not supported, fall back to normal execution
	if b.eventHandler == nil || streamCfg.Method == StreamMethodNone {
		return b.ExecuteCommand(ctx, args, stdin, workDir)
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

	// Emit started event with command info
	b.emitEvent(core.NewAgentEvent(core.AgentEventStarted, adapterName, "Starting execution").
		WithData(map[string]any{"command": fullCmd}))

	switch streamCfg.Method {
	case StreamMethodJSONStdout:
		return b.executeWithJSONStreaming(ctx, adapterName, args, stdin, workDir, streamCfg, parser)
	case StreamMethodLogFile:
		return b.executeWithLogFileStreaming(ctx, adapterName, args, stdin, workDir, streamCfg, parser)
	default:
		return b.ExecuteCommand(ctx, args, stdin, workDir)
	}
}

// executeWithJSONStreaming handles CLIs that output JSON events to stdout.
func (b *BaseAdapter) executeWithJSONStreaming(
	ctx context.Context,
	adapterName string,
	args []string,
	stdin, workDir string,
	cfg StreamConfig,
	parser StreamParser,
) (*CommandResult, error) {
	// Modify args to enable streaming output
	streamArgs := b.addStreamingArgs(args, cfg)

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

	// Handle multi-word commands
	cmdParts := strings.Fields(cmdPath)
	if len(cmdParts) > 1 {
		cmdPath = cmdParts[0]
		streamArgs = append(cmdParts[1:], streamArgs...)
	}

	cmd := exec.CommandContext(ctx, cmdPath, streamArgs...)
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
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	cmd.Env = os.Environ()

	b.logger.Debug("executing command with JSON streaming",
		"path", cmdPath,
		"args", streamArgs,
	)

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

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
		b.streamStderr(stderrPipe, &stderr)
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
func (b *BaseAdapter) streamJSONOutput(pipe io.ReadCloser, buf *bytes.Buffer, adapterName string, parser StreamParser) {
	scanner := bufio.NewScanner(pipe)
	// Increase buffer size for large JSON lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")

		// Parse and emit events
		if parser != nil {
			events := parser.ParseLine(line)
			for _, event := range events {
				b.emitEvent(event)
			}
		}
	}
}

// executeWithLogFileStreaming handles CLIs that write logs to a file (like Copilot).
func (b *BaseAdapter) executeWithLogFileStreaming(
	ctx context.Context,
	adapterName string,
	args []string,
	stdin, workDir string,
	cfg StreamConfig,
	parser StreamParser,
) (*CommandResult, error) {
	// Create temporary log directory
	logDir, err := os.MkdirTemp("", "quorum-logs-*")
	if err != nil {
		// Fall back to normal execution
		return b.ExecuteCommand(ctx, args, stdin, workDir)
	}
	defer os.RemoveAll(logDir)

	// Add log flags to args
	logArgs := append(args, cfg.LogDirFlag, logDir)
	if cfg.LogLevelFlag != "" && cfg.LogLevelValue != "" {
		logArgs = append(logArgs, cfg.LogLevelFlag, cfg.LogLevelValue)
	}

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

	cmdParts := strings.Fields(cmdPath)
	if len(cmdParts) > 1 {
		cmdPath = cmdParts[0]
		logArgs = append(cmdParts[1:], logArgs...)
	}

	cmd := exec.CommandContext(ctx, cmdPath, logArgs...)
	if workDir != "" {
		cmd.Dir = workDir
	} else if b.config.WorkDir != "" {
		cmd.Dir = b.config.WorkDir
	}

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()

	b.logger.Debug("executing command with log file streaming",
		"path", cmdPath,
		"args", logArgs,
		"log_dir", logDir,
	)

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

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
				b.readNewLogContent(filePath, seenFiles, adapterName, parser)
			}
		}
	}
}

// readNewLogContent reads new content from a log file since last read.
func (b *BaseAdapter) readNewLogContent(filePath string, seenFiles map[string]int64, adapterName string, parser StreamParser) {
	info, err := os.Stat(filePath)
	if err != nil {
		return
	}

	lastPos := seenFiles[filePath]
	currentSize := info.Size()

	if currentSize <= lastPos {
		return // No new content
	}

	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to last position
	if lastPos > 0 {
		file.Seek(lastPos, 0)
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
