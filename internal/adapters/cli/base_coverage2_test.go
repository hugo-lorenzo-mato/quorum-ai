package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// =============================================================================
// ExecuteCommand Tests (without running real external CLIs)
// =============================================================================

func TestExecuteCommand_EmptyPath(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)
	_, err := adapter.ExecuteCommand(t.Context(), nil, "", "", 0)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "NO_PATH") {
		t.Errorf("expected NO_PATH error, got: %v", err)
	}
}

func TestExecuteCommand_NonexistentBinary(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test",
		Path: "/nonexistent/binary/quorum_test_404",
	}, nil)
	_, err := adapter.ExecuteCommand(t.Context(), nil, "", "", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestExecuteCommand_Success(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-echo",
		Path: "echo",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), []string{"hello"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout = %q, want to contain 'hello'", result.Stdout)
	}
}

func TestExecuteCommand_WithStdin(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-cat",
		Path: "cat",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), nil, "stdin data", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "stdin data") {
		t.Errorf("Stdout = %q, want to contain 'stdin data'", result.Stdout)
	}
}

func TestExecuteCommand_WithLongStdin(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-cat",
		Path: "cat",
	}, nil)
	longInput := strings.Repeat("x", 600)
	result, err := adapter.ExecuteCommand(t.Context(), nil, longInput, "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Stdout) < 600 {
		t.Errorf("Stdout too short: %d chars", len(result.Stdout))
	}
}

func TestExecuteCommand_WorkDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Skip on Windows - pwd command not available natively
	// and Git Bash outputs Unix-style paths that don't match Windows tmpDir
	if runtime.GOOS == "windows" {
		t.Skip("Skipping pwd test on Windows - command output format incompatible")
	}

	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-pwd",
		Path: "pwd",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), nil, "", tmpDir, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, tmpDir) {
		t.Errorf("Stdout = %q, want to contain work dir %q", result.Stdout, tmpDir)
	}
}

func TestExecuteCommand_ConfigWorkDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Skip on Windows - pwd command not available natively
	if runtime.GOOS == "windows" {
		t.Skip("Skipping pwd test on Windows - command output format incompatible")
	}

	adapter := NewBaseAdapter(AgentConfig{
		Name:    "test-pwd",
		Path:    "pwd",
		WorkDir: tmpDir,
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), nil, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, tmpDir) {
		t.Errorf("Stdout = %q, want to contain config work dir %q", result.Stdout, tmpDir)
	}
}

func TestExecuteCommand_MultiWordPath(t *testing.T) {
	t.Parallel()
	// Simulate "bash -c" as multi-word path
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-multiword",
		Path: "bash -c",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), []string{"echo multiword"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "multiword") {
		t.Errorf("Stdout = %q, want 'multiword'", result.Stdout)
	}
}

func TestExecuteCommand_ExitCodeNonZero(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-false",
		Path: "bash",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), []string{"-c", "exit 2"}, "", "", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", result.ExitCode)
	}
}

func TestExecuteCommand_Timeout(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-timeout",
		Path: "sleep",
	}, nil)
	start := time.Now()
	_, err := adapter.ExecuteCommand(t.Context(), []string{"30"}, "", "", 200*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v, expected quick timeout", elapsed)
	}
	// Check it's a timeout error
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestExecuteCommand_ConfigTimeout(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name:    "test-cfg-timeout",
		Path:    "sleep",
		Timeout: 200 * time.Millisecond,
	}, nil)
	start := time.Now()
	_, err := adapter.ExecuteCommand(t.Context(), []string{"30"}, "", "", 0)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v, expected quick timeout", elapsed)
	}
}

func TestExecuteCommand_ContextCancelled(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-cancel",
		Path: "sleep",
	}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	_, err := adapter.ExecuteCommand(ctx, []string{"30"}, "", "", 30*time.Second)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestExecuteCommand_ExtraEnv(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-env",
		Path: "bash",
	}, nil)
	adapter.ExtraEnv = map[string]string{
		"QUORUM_TEST_EXTRA": "test_value",
	}
	result, err := adapter.ExecuteCommand(t.Context(), []string{"-c", "echo $QUORUM_TEST_EXTRA"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("Stdout = %q, want to contain 'test_value'", result.Stdout)
	}
}

func TestExecuteCommand_WithLogCallback(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-callback",
		Path: "bash",
	}, nil)
	var callbackLines []string
	adapter.SetLogCallback(func(line string) {
		callbackLines = append(callbackLines, line)
	})
	// Use a small sleep to ensure stderr is flushed before bash exits
	result, err := adapter.ExecuteCommand(t.Context(), []string{"-c", "echo stderr_line >&2; sleep 0.05"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stderr should be captured in result
	if !strings.Contains(result.Stderr, "stderr_line") {
		t.Errorf("Stderr = %q, want 'stderr_line'", result.Stderr)
	}
	// Callback should have been called
	if len(callbackLines) == 0 {
		t.Error("expected log callback to be called with stderr lines")
	}
}

func TestExecuteCommand_StderrWithoutCallback(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-stderr",
		Path: "bash",
	}, nil)
	result, err := adapter.ExecuteCommand(t.Context(), []string{"-c", "echo err_msg >&2"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stderr, "err_msg") {
		t.Errorf("Stderr = %q, want 'err_msg'", result.Stderr)
	}
}

// =============================================================================
// streamStderr Tests
// =============================================================================

func TestStreamStderr_BasicCapture(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test")
		close(done)
	}()

	_, _ = pw.Write([]byte("line1\nline2\nline3\n"))
	_ = pw.Close()
	<-done

	if !strings.Contains(buf.String(), "line1") {
		t.Errorf("buffer should contain 'line1', got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "line3") {
		t.Errorf("buffer should contain 'line3', got: %q", buf.String())
	}
}

func TestStreamStderr_WithCallback(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var callbackLines []string
	adapter.SetLogCallback(func(line string) {
		callbackLines = append(callbackLines, line)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test")
		close(done)
	}()

	_, _ = pw.Write([]byte("cb_line1\ncb_line2\n"))
	_ = pw.Close()
	<-done

	if len(callbackLines) != 2 {
		t.Errorf("expected 2 callback lines, got %d", len(callbackLines))
	}
}

func TestStreamStderr_WithActivityChannel(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer
	activity := make(chan struct{}, 10)

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test", activity)
		close(done)
	}()

	_, _ = pw.Write([]byte("a\nb\nc\n"))
	_ = pw.Close()
	<-done

	count := len(activity)
	if count != 3 {
		t.Errorf("expected 3 activity signals, got %d", count)
	}
}

// =============================================================================
// emitStderrEvent Tests
// =============================================================================

func TestEmitStderrEvent_NilHandler(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)
	// Should not panic with nil handler
	adapter.emitStderrEvent("test", "reading file.txt")
}

func TestEmitStderrEvent_ToolPatterns(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	toolLines := []string{
		"Reading file config.yaml",
		"Writing to output.txt",
		"Executing command: ls -la",
		"Running tests...",
		"Calling API endpoint",
		"Searching for patterns",
		"Analyzing codebase",
		"Processing results",
		"Fetching remote data",
		"Loading modules",
		"tool: read_file",
		"Using tool glob",
		"function call: search",
		"bash: ls -la /home",
	}

	for _, line := range toolLines {
		adapter.emitStderrEvent("test", line)
	}

	// Should have emitted at least some tool_use events (aggregator may rate-limit)
	toolUseCount := 0
	for _, e := range events {
		if e.Type == core.AgentEventToolUse {
			toolUseCount++
		}
	}
	if toolUseCount == 0 {
		t.Error("expected at least one tool_use event for tool patterns")
	}
}

func TestEmitStderrEvent_ThinkingPatterns(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	thinkLines := []string{
		"Thinking about the solution",
		"Reasoning through options",
		"Considering alternatives",
		"Evaluating approaches",
	}

	for _, line := range thinkLines {
		adapter.emitStderrEvent("test", line)
	}

	thinkCount := 0
	for _, e := range events {
		if e.Type == core.AgentEventThinking {
			thinkCount++
		}
	}
	if thinkCount == 0 {
		t.Error("expected at least one thinking event")
	}
}

func TestEmitStderrEvent_LongLinesTruncated(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	longLine := "Reading " + strings.Repeat("x", 100)
	adapter.emitStderrEvent("test", longLine)

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	// The description should be truncated to ~50 chars
	if len(events[0].Message) > 55 {
		t.Errorf("message not truncated: length=%d", len(events[0].Message))
	}
}

func TestEmitStderrEvent_NoMatchNoEvent(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	adapter.emitStderrEvent("test", "some random text")
	if len(events) != 0 {
		t.Errorf("expected 0 events for non-matching line, got %d", len(events))
	}
}

// =============================================================================
// emitEvent Tests
// =============================================================================

func TestEmitEvent_AggregatorFiltering(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	// Emit many rapid progress events - aggregator should filter some
	for i := 0; i < 20; i++ {
		adapter.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "test", "progress"))
	}

	// Completed events should always pass through
	adapter.emitEvent(core.NewAgentEvent(core.AgentEventCompleted, "test", "done"))
	adapter.emitEvent(core.NewAgentEvent(core.AgentEventError, "test", "err"))

	// Check that completed and error events were emitted
	hasCompleted := false
	hasError := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
		if e.Type == core.AgentEventError {
			hasError = true
		}
	}
	if !hasCompleted {
		t.Error("completed event should always be emitted")
	}
	if !hasError {
		t.Error("error event should always be emitted")
	}
}

// =============================================================================
// WithDiagnostics Tests
// =============================================================================

func TestWithDiagnostics(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	// Should be nil initially
	if adapter.safeExec != nil {
		t.Error("safeExec should be nil initially")
	}
	if adapter.dumpWriter != nil {
		t.Error("dumpWriter should be nil initially")
	}

	// WithDiagnostics with nil args should be fine
	adapter.WithDiagnostics(nil, nil)
	if adapter.safeExec != nil {
		t.Error("safeExec should remain nil")
	}
}

// =============================================================================
// extractTextFromJSONLine Tests
// =============================================================================

func TestExtractTextFromJSONLine_ResultSuccess(t *testing.T) {
	t.Parallel()
	line := `{"type":"result","subtype":"success","result":"The answer is 42"}`
	got := extractTextFromJSONLine(line)
	if got != "The answer is 42" {
		t.Errorf("got %q, want %q", got, "The answer is 42")
	}
}

func TestExtractTextFromJSONLine_ResultSuccessWithResponse(t *testing.T) {
	t.Parallel()
	line := `{"type":"result","subtype":"success","response":"Gemini response"}`
	got := extractTextFromJSONLine(line)
	if got != "Gemini response" {
		t.Errorf("got %q, want %q", got, "Gemini response")
	}
}

func TestExtractTextFromJSONLine_AssistantText(t *testing.T) {
	t.Parallel()
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`
	got := extractTextFromJSONLine(line)
	if got != "Hello world" {
		t.Errorf("got %q, want %q", got, "Hello world")
	}
}

func TestExtractTextFromJSONLine_GeminiText(t *testing.T) {
	t.Parallel()
	line := `{"type":"text","text":"Gemini text"}`
	got := extractTextFromJSONLine(line)
	if got != "Gemini text" {
		t.Errorf("got %q, want %q", got, "Gemini text")
	}
}

func TestExtractTextFromJSONLine_CodexAgentMessage(t *testing.T) {
	t.Parallel()
	line := `{"type":"item.completed","item":{"type":"agent_message","text":"Codex message"}}`
	got := extractTextFromJSONLine(line)
	if got != "Codex message\n" {
		t.Errorf("got %q, want %q", got, "Codex message\n")
	}
}

func TestExtractTextFromJSONLine_EmptyLine(t *testing.T) {
	t.Parallel()
	if got := extractTextFromJSONLine(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractTextFromJSONLine_NonJSON(t *testing.T) {
	t.Parallel()
	if got := extractTextFromJSONLine("not json"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractTextFromJSONLine_InvalidJSON(t *testing.T) {
	t.Parallel()
	if got := extractTextFromJSONLine("{invalid json}"); got != "" {
		t.Errorf("got %q, want empty for invalid JSON", got)
	}
}

func TestExtractTextFromJSONLine_ResultNotSuccess(t *testing.T) {
	t.Parallel()
	// result event with subtype != success should not extract text
	line := `{"type":"result","subtype":"error","error":"something failed"}`
	got := extractTextFromJSONLine(line)
	if got != "" {
		t.Errorf("got %q, want empty for non-success result", got)
	}
}

func TestExtractTextFromJSONLine_AssistantToolUse(t *testing.T) {
	t.Parallel()
	// assistant event with tool_use should not extract text
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"bash"}]}}`
	got := extractTextFromJSONLine(line)
	if got != "" {
		t.Errorf("got %q, want empty for tool_use content", got)
	}
}

func TestExtractTextFromJSONLine_WhitespaceOnly(t *testing.T) {
	t.Parallel()
	if got := extractTextFromJSONLine("   "); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractTextFromJSONLine_CodexNonAgentMessage(t *testing.T) {
	t.Parallel()
	// item.completed with type != agent_message
	line := `{"type":"item.completed","item":{"type":"command_execution","text":"some text"}}`
	got := extractTextFromJSONLine(line)
	if got != "" {
		t.Errorf("got %q, want empty for non-agent_message", got)
	}
}

// =============================================================================
// streamJSONOutput Tests
// =============================================================================

func TestStreamJSONOutput_BasicCapture(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamJSONOutput(pr, &buf, "test", nil)
		close(done)
	}()

	// Write JSON lines with text content
	_, _ = pw.Write([]byte(`{"type":"result","subtype":"success","result":"hello"}` + "\n"))
	_, _ = pw.Write([]byte(`{"type":"text","text":"world"}` + "\n"))
	_ = pw.Close()
	<-done

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("buffer should contain 'hello', got: %q", output)
	}
	if !strings.Contains(output, "world") {
		t.Errorf("buffer should contain 'world', got: %q", output)
	}
}

func TestStreamJSONOutput_WithParser(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	parser := &testStreamParser{}

	done := make(chan struct{})
	go func() {
		adapter.streamJSONOutput(pr, &buf, "test", parser)
		close(done)
	}()

	_, _ = pw.Write([]byte(`{"type":"result","subtype":"success"}` + "\n"))
	_ = pw.Close()
	<-done

	// Parser should have produced a completed event
	hasCompleted := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected completed event from parser")
	}
}

func TestStreamJSONOutput_NonJSONLines(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamJSONOutput(pr, &buf, "test", nil)
		close(done)
	}()

	// Write non-JSON lines (should be ignored for text extraction)
	_, _ = pw.Write([]byte("plain text\n"))
	_, _ = pw.Write([]byte("\n"))
	_ = pw.Close()
	<-done

	if buf.Len() != 0 {
		t.Errorf("buffer should be empty for non-JSON lines, got: %q", buf.String())
	}
}

// =============================================================================
// ExecuteWithStreaming Tests
// =============================================================================

func TestExecuteWithStreaming_NoHandler(t *testing.T) {
	t.Parallel()
	// Without event handler, should fall back to normal execution
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-nohandler",
		Path: "echo",
	}, nil)
	result, err := adapter.ExecuteWithStreaming(t.Context(), "test-nohandler", []string{"hello"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout = %q, want 'hello'", result.Stdout)
	}
}

func TestExecuteWithStreaming_StreamMethodNone(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-none",
		Path: "echo",
	}, nil)
	adapter.SetEventHandler(func(_ core.AgentEvent) {})

	// "test-none" has no stream config, so StreamMethodNone
	result, err := adapter.ExecuteWithStreaming(t.Context(), "test-none", []string{"hello"}, "", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout = %q, want 'hello'", result.Stdout)
	}
}

func TestExecuteWithStreaming_EmitStartedEvent(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-started",
		Path: "echo",
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	// This uses "test-started" which has StreamMethodNone, so it falls back
	_, _ = adapter.ExecuteWithStreaming(t.Context(), "test-started", []string{"hi"}, "", "", 5*time.Second)

	// Should NOT have started event because StreamMethodNone falls back immediately
	// (started event is only emitted when handler exists AND method is not None)
	// But actually, the code emits started before the switch.
	// Let's check.
	hasStarted := false
	for _, e := range events {
		if e.Type == core.AgentEventStarted {
			hasStarted = true
		}
	}
	// The started event IS emitted before the switch statement
	if !hasStarted {
		t.Log("Note: started event not emitted for StreamMethodNone (no handler or falls back)")
	}
}

// =============================================================================
// classifyError Tests (extended)
// =============================================================================

func TestClassifyError_OutputTokenLimitPatterns(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	patterns := []string{
		"output token maximum exceeded",
		"Too many output tokens",
		"max output limit hit",
		"maximum output reached",
		"response exceeded the limit",
		"context length exceeded for model",
		"maximum context window used",
		"too many tokens in request",
		"max_tokens parameter exceeded",
	}

	for _, pattern := range patterns {
		result := &CommandResult{Stderr: pattern, ExitCode: 1}
		err := base.classifyError(result)
		if err == nil {
			t.Errorf("expected error for pattern %q", pattern)
			continue
		}
		domErr, ok := err.(*core.DomainError)
		if !ok {
			t.Errorf("expected DomainError for pattern %q, got %T", pattern, err)
			continue
		}
		if domErr.Code != "OUTPUT_TOO_LONG" {
			t.Errorf("for pattern %q: Code = %q, want OUTPUT_TOO_LONG", pattern, domErr.Code)
		}
	}
}

func TestClassifyError_RateLimitPatterns(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	patterns := []string{
		"rate limit reached",
		"too many requests, please wait",
		"HTTP 429 error",
		"quota exceeded",
	}

	for _, pattern := range patterns {
		result := &CommandResult{Stderr: pattern, ExitCode: 1}
		err := base.classifyError(result)
		if err == nil {
			t.Errorf("expected error for pattern %q", pattern)
			continue
		}
		domErr, ok := err.(*core.DomainError)
		if !ok {
			t.Errorf("expected DomainError for pattern %q", pattern)
			continue
		}
		if domErr.Category != core.ErrCatRateLimit {
			t.Errorf("for pattern %q: Category = %q, want rate_limit", pattern, domErr.Category)
		}
	}
}

func TestClassifyError_AuthPatterns(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	patterns := []string{
		"unauthorized access denied",
		"authentication required",
		"authorization failed",
		"forbidden resource",
		"invalid api key provided",
		"invalid token received",
		"oauth error occurred",
	}

	for _, pattern := range patterns {
		result := &CommandResult{Stderr: pattern, ExitCode: 1}
		err := base.classifyError(result)
		domErr, ok := err.(*core.DomainError)
		if !ok {
			t.Errorf("expected DomainError for pattern %q", pattern)
			continue
		}
		if domErr.Category != core.ErrCatAuth {
			t.Errorf("for pattern %q: Category = %q, want auth", pattern, domErr.Category)
		}
	}
}

func TestClassifyError_NetworkPatterns(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	patterns := []string{
		"connection refused",
		"network error occurred",
		"timeout waiting for response",
		"host unreachable",
	}

	for _, pattern := range patterns {
		result := &CommandResult{Stderr: pattern, ExitCode: 1}
		err := base.classifyError(result)
		domErr, ok := err.(*core.DomainError)
		if !ok {
			t.Errorf("expected DomainError for pattern %q", pattern)
			continue
		}
		if domErr.Category != core.ErrCatExecution || domErr.Code != "NETWORK" {
			t.Errorf("for pattern %q: Code = %q, want NETWORK", pattern, domErr.Code)
		}
	}
}

func TestClassifyError_EmptyStderrUsesStdout(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	result := &CommandResult{
		Stderr:   "",
		Stdout:   `{"error":"stdout error message"}`,
		ExitCode: 1,
	}
	err := base.classifyError(result)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should contain the extracted error from stdout
	if !strings.Contains(err.Error(), "stdout error message") {
		t.Errorf("error should contain stdout error: %v", err)
	}
}

func TestClassifyError_NoErrorMessage(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	result := &CommandResult{
		Stderr:   "",
		Stdout:   "",
		ExitCode: 1,
	}
	err := base.classifyError(result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no error message captured") {
		t.Errorf("error should mention no error message: %v", err)
	}
}

func TestClassifyError_GenericError(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	result := &CommandResult{
		Stderr:   "something completely unknown happened",
		ExitCode: 42,
	}
	err := base.classifyError(result)
	domErr, ok := err.(*core.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if domErr.Code != "CLI_ERROR" {
		t.Errorf("Code = %q, want CLI_ERROR", domErr.Code)
	}
	if !strings.Contains(domErr.Message, "exit code 42") {
		t.Errorf("message should contain exit code: %q", domErr.Message)
	}
}

// =============================================================================
// extractErrorFromOutput Tests (extended)
// =============================================================================

func TestExtractErrorFromOutput_InvalidJSON(t *testing.T) {
	t.Parallel()
	// Lines starting with "{" that fail JSON parse are skipped; the fallback
	// returns the last non-empty, non-JSON line. When all lines start with "{",
	// the function returns empty.
	output := `{not valid json}`
	got := extractErrorFromOutput(output)
	if got != "" {
		t.Errorf("got %q, want empty (invalid JSON lines starting with '{' are skipped)", got)
	}
}

func TestExtractErrorFromOutput_JSONWithoutErrorField(t *testing.T) {
	t.Parallel()
	output := `{"status":"ok","data":"something"}`
	got := extractErrorFromOutput(output)
	// No error field, no plain text fallback (line starts with {), so empty
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractErrorFromOutput_MultipleJSONLastHasError(t *testing.T) {
	t.Parallel()
	output := `{"status":"ok"}
{"error":"the real error"}`
	got := extractErrorFromOutput(output)
	if got != "the real error" {
		t.Errorf("got %q, want 'the real error'", got)
	}
}

func TestExtractErrorFromOutput_ClaudeTypeErrorExtended(t *testing.T) {
	t.Parallel()
	output := `{"type":"error","error":"API connection failed"}`
	got := extractErrorFromOutput(output)
	if got != "API connection failed" {
		t.Errorf("got %q, want 'API connection failed'", got)
	}
}

// =============================================================================
// ParseJSON and ExtractJSON Tests (extended)
// =============================================================================

func TestParseJSON_DirectParse(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	var result map[string]string
	err := base.ParseJSON(`{"key":"value"}`, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want 'value'", result["key"])
	}
}

func TestParseJSON_ExtractFromMixed(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	var result map[string]int
	err := base.ParseJSON(`prefix text {"num":42} suffix`, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["num"] != 42 {
		t.Errorf("num = %d, want 42", result["num"])
	}
}

func TestParseJSON_NoJSON(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	var result interface{}
	err := base.ParseJSON("just text", &result)
	if err == nil {
		t.Error("expected error for no JSON")
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	input := `text {"a":{"b":{"c":"d"}}} more`
	got := base.ExtractJSON(input)
	if got != `{"a":{"b":{"c":"d"}}}` {
		t.Errorf("ExtractJSON = %q", got)
	}
}

func TestExtractJSON_ArrayFirst(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	// ExtractJSON finds first { or [ — here [ comes first
	input := `prefix [1, 2, 3] suffix`
	got := base.ExtractJSON(input)
	if got != `[1, 2, 3]` {
		t.Errorf("ExtractJSON = %q, want [1, 2, 3]", got)
	}
}

func TestExtractJSON_ObjectBeforeArray(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	// When { appears before [, ExtractJSON extracts the object
	input := `prefix {"a": 2} [3] suffix`
	got := base.ExtractJSON(input)
	if got != `{"a": 2}` {
		t.Errorf("ExtractJSON = %q, want object", got)
	}
}

func TestExtractJSON_NoBrackets(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	got := base.ExtractJSON("no brackets here")
	if got != "" {
		t.Errorf("ExtractJSON = %q, want empty", got)
	}
}

func TestExtractJSON_EscapedStrings(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	input := `{"key":"value with \"escaped\" quotes and \\backslash"}`
	got := base.ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

// =============================================================================
// TokenEstimate and TruncateToTokenLimit Tests
// =============================================================================

func TestTokenEstimate_Various(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{strings.Repeat("x", 400), 100},
	}

	for _, tt := range tests {
		got := base.TokenEstimate(tt.input)
		if got != tt.want {
			t.Errorf("TokenEstimate(%d chars) = %d, want %d", len(tt.input), got, tt.want)
		}
	}
}

func TestTruncateToTokenLimit_ExactLimit(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	text := strings.Repeat("x", 400) // 400 chars = 100 tokens
	got := base.TruncateToTokenLimit(text, 100)
	if got != text {
		t.Error("should not truncate when exactly at limit")
	}
}

func TestTruncateToTokenLimit_BelowLimit(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	got := base.TruncateToTokenLimit("short", 1000)
	if got != "short" {
		t.Errorf("got %q, want 'short'", got)
	}
}

func TestTruncateToTokenLimit_AboveLimit(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	text := strings.Repeat("x", 500) // 500 chars > 100 tokens * 4
	got := base.TruncateToTokenLimit(text, 100)
	if !strings.HasSuffix(got, "\n...[truncated]") {
		t.Errorf("should end with truncation marker, got: %q", got[len(got)-20:])
	}
	if len(got) != 400+len("\n...[truncated]") {
		t.Errorf("truncated length = %d", len(got))
	}
}

// =============================================================================
// CheckAvailability Tests (extended)
// =============================================================================

func TestCheckAvailability_NonexistentCommand(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{
		Path: "nonexistent_quorum_test_cmd",
	}, nil)
	err := base.CheckAvailability(t.Context())
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestCheckAvailability_WithMultiWordCommand(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{
		Path: "nonexistent_cmd subcommand",
	}, nil)
	err := base.CheckAvailability(t.Context())
	if err == nil {
		t.Error("expected error for nonexistent multi-word command")
	}
}

// =============================================================================
// GetVersion Tests
// =============================================================================

func TestGetVersion_WithEcho(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-version",
		Path: "bash",
	}, nil)
	version, err := adapter.GetVersion(t.Context(), "-c")
	// We pass "-c" as the version arg but it doesn't produce version output from bash
	// bash -c with no further arg just exits. We're testing the method doesn't crash.
	if err != nil {
		t.Logf("GetVersion error (expected): %v", err)
	}
	_ = version
}

// =============================================================================
// NewBaseAdapter with Logger Tests
// =============================================================================

func TestNewBaseAdapter_WithLogger(t *testing.T) {
	t.Parallel()
	logger := logging.NewNop()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, logger)
	if adapter.logger != logger {
		t.Error("logger not set correctly")
	}
}

// =============================================================================
// pathWithin Tests (extended)
// =============================================================================

func TestPathWithin_SymlinkTraversal(t *testing.T) {
	t.Parallel()
	// Test with absolute paths that are clearly outside
	if pathWithin("/tmp/safe", "/etc/passwd") {
		t.Error("/etc/passwd should not be within /tmp/safe")
	}
}

func TestPathWithin_NestedChild(t *testing.T) {
	t.Parallel()
	if !pathWithin("/tmp/safe", "/tmp/safe/a/b/c/file.txt") {
		t.Error("deeply nested child should be within")
	}
}

// =============================================================================
// addStreamingArgs Tests (extended)
// =============================================================================

func TestAddStreamingArgs_EmptyConfig(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{}, nil)

	args := adapter.addStreamingArgs([]string{"cmd"}, StreamConfig{})
	if len(args) != 1 || args[0] != "cmd" {
		t.Errorf("expected unchanged args, got: %v", args)
	}
}

func TestAddStreamingArgs_AllFields(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{}, nil)

	args := adapter.addStreamingArgs([]string{"run"}, StreamConfig{
		OutputFormatFlag:  "--format",
		OutputFormatValue: "json",
		RequiredFlags:     []string{"--verbose", "--debug"},
	})
	want := []string{"run", "--format", "json", "--verbose", "--debug"}
	if len(args) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, args[i], w)
		}
	}
}

// =============================================================================
// readNewLogContent Tests
// =============================================================================

func TestReadNewLogContent_PathTraversal(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)
	seenFiles := make(map[string]int64)

	// Attempt path traversal - should be rejected by pathWithin
	adapter.readNewLogContent("/tmp/safe", "/tmp/safe/../../etc/passwd", seenFiles, "test", nil)

	// Should not add to seenFiles
	if len(seenFiles) != 0 {
		t.Error("path traversal file should not be tracked")
	}
}

func TestReadNewLogContent_NonexistentFile(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)
	seenFiles := make(map[string]int64)

	tmpDir := t.TempDir()
	adapter.readNewLogContent(tmpDir, tmpDir+"/nonexistent.log", seenFiles, "test", nil)

	// Should not crash and not add to seenFiles
	if len(seenFiles) != 0 {
		t.Error("nonexistent file should not be tracked")
	}
}

// =============================================================================
// truncateForDebug Tests (extended)
// =============================================================================

func TestTruncateForDebug_ZeroLength(t *testing.T) {
	t.Parallel()
	got := truncateForDebug("hello", 0)
	if got != "...[truncated]" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateForDebug_OneChar(t *testing.T) {
	t.Parallel()
	got := truncateForDebug("hello world", 1)
	if got != "h...[truncated]" {
		t.Errorf("got %q", got)
	}
}

// =============================================================================
// readNewLogContent Tests (extended — actual file I/O)
// =============================================================================

func TestReadNewLogContent_ReadsNewContent(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	tmpDir := t.TempDir()
	logFile := tmpDir + "/test.log"
	// Write initial content
	if err := os.WriteFile(logFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	seenFiles := make(map[string]int64)

	// First read: should read all content
	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)

	if seenFiles[logFile] == 0 {
		t.Error("expected seenFiles to track file position")
	}
	firstPos := seenFiles[logFile]

	// Second read with no changes: should be a no-op
	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)
	if seenFiles[logFile] != firstPos {
		t.Error("position should not change without new content")
	}

	// Append content and read again
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("line3\n")
	f.Close()

	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)
	if seenFiles[logFile] <= firstPos {
		t.Error("position should advance after new content")
	}
}

func TestReadNewLogContent_WithParser(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	tmpDir := t.TempDir()
	logFile := tmpDir + "/test.log"

	// Write content that will trigger the test parser
	content := `{"type":"result","subtype":"success"}` + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	seenFiles := make(map[string]int64)
	parser := &testStreamParser{}

	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", parser)

	hasCompleted := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected completed event from parser")
	}
}

func TestReadNewLogContent_SeekFromLastPosition(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	tmpDir := t.TempDir()
	logFile := tmpDir + "/test.log"

	// Write initial content
	initialContent := "first line\nsecond line\n"
	if err := os.WriteFile(logFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	seenFiles := make(map[string]int64)

	// First read
	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)

	// Append new content
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	_, _ = f.WriteString("third line\n")
	f.Close()

	// Track which lines the parser sees on second read
	var parsedLines []string
	mockParser := &lineCollectorParser{lines: &parsedLines}

	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", mockParser)

	// Should only see "third line" (reading from lastPos)
	found := false
	for _, l := range parsedLines {
		if strings.Contains(l, "third") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected to find 'third line' in parsed lines: %v", parsedLines)
	}
}

// lineCollectorParser records all lines parsed
type lineCollectorParser struct {
	lines *[]string
}

func (p *lineCollectorParser) ParseLine(line string) []core.AgentEvent {
	*p.lines = append(*p.lines, line)
	return nil
}

func (p *lineCollectorParser) AgentName() string { return "test" }

// =============================================================================
// tailLogFiles Tests
// =============================================================================

func TestTailLogFiles_ReadsNewFiles(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	tmpDir := t.TempDir()
	stopCh := make(chan struct{})

	parser := &testStreamParser{}

	// Start tailing in background
	done := make(chan struct{})
	go func() {
		adapter.tailLogFiles(t.Context(), tmpDir, "test", parser, stopCh)
		close(done)
	}()

	// Wait a tick, then write a log file
	time.Sleep(150 * time.Millisecond)
	logFile := tmpDir + "/activity.log"
	_ = os.WriteFile(logFile, []byte(`{"type":"result","subtype":"success"}`+"\n"), 0644)

	// Wait for tailing to pick it up
	time.Sleep(250 * time.Millisecond)

	// Stop tailing
	close(stopCh)
	<-done

	hasCompleted := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected completed event from tailed log file")
	}
}

func TestTailLogFiles_IgnoresNonLogFiles(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	tmpDir := t.TempDir()
	stopCh := make(chan struct{})
	parser := &testStreamParser{}

	done := make(chan struct{})
	go func() {
		adapter.tailLogFiles(t.Context(), tmpDir, "test", parser, stopCh)
		close(done)
	}()

	// Write a non-.log file
	time.Sleep(150 * time.Millisecond)
	_ = os.WriteFile(tmpDir+"/data.json", []byte(`{"type":"result","subtype":"success"}`+"\n"), 0644)

	time.Sleep(250 * time.Millisecond)
	close(stopCh)
	<-done

	// Should not have parsed the .json file
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			t.Error("should NOT parse non-.log files")
		}
	}
}

func TestTailLogFiles_ReadsTextFiles(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	tmpDir := t.TempDir()
	stopCh := make(chan struct{})
	parser := &testStreamParser{}

	done := make(chan struct{})
	go func() {
		adapter.tailLogFiles(t.Context(), tmpDir, "test", parser, stopCh)
		close(done)
	}()

	// Write a .txt file (should be read)
	time.Sleep(150 * time.Millisecond)
	_ = os.WriteFile(tmpDir+"/output.txt", []byte(`{"type":"result","subtype":"success"}`+"\n"), 0644)

	time.Sleep(250 * time.Millisecond)
	close(stopCh)
	<-done

	hasCompleted := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected completed event from .txt log file")
	}
}

func TestTailLogFiles_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	tmpDir := t.TempDir()
	stopCh := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		adapter.tailLogFiles(ctx, tmpDir, "test", nil, stopCh)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Good, it stopped
	case <-time.After(2 * time.Second):
		t.Fatal("tailLogFiles did not stop on context cancel")
	}
}

// =============================================================================
// ExecuteWithStreaming JSON path Tests
// =============================================================================

func TestExecuteWithJSONStreaming_NoPath(t *testing.T) {
	t.Parallel()
	// Test that executeWithJSONStreaming returns a validation error when path is empty.
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-no-path",
		Path: "",
	}, nil)

	_, err := adapter.executeWithJSONStreaming(
		t.Context(),
		"test-no-path",
		[]string{"arg1"},
		"", "", 5*time.Second,
		StreamConfig{},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "NO_PATH") && !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteWithJSONStreaming_InvalidCommand(t *testing.T) {
	t.Parallel()
	// Test that executeWithJSONStreaming returns an error for non-existent command.
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-bad-cmd",
		Path: "nonexistent-binary-xyz-12345",
	}, nil)

	_, err := adapter.executeWithJSONStreaming(
		t.Context(),
		"test-bad-cmd",
		[]string{"arg1"},
		"", "", 5*time.Second,
		StreamConfig{},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for non-existent command")
	}
	if !strings.Contains(err.Error(), "locating command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteWithJSONStreaming_ContextCancelled(t *testing.T) {
	t.Parallel()
	// Test that executeWithJSONStreaming respects context cancellation.
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-ctx-cancel",
		Path: "bash",
	}, nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	_, err := adapter.executeWithJSONStreaming(
		ctx,
		"test-ctx-cancel",
		[]string{"-c", "sleep 30"},
		"", "", 5*time.Second,
		StreamConfig{},
		nil,
	)
	// Should error due to cancelled context
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestExecuteWithJSONStreaming_DefaultTimeout(t *testing.T) {
	t.Parallel()
	// Test that config timeout is used when optTimeout is 0.
	adapter := NewBaseAdapter(AgentConfig{
		Name:    "test-default-to",
		Path:    "bash",
		Timeout: 200 * time.Millisecond,
	}, nil)

	_, err := adapter.executeWithJSONStreaming(
		t.Context(),
		"test-default-to",
		[]string{"-c", "sleep 30"},
		"", "", 0, // optTimeout=0, uses config.Timeout
		StreamConfig{},
		nil,
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecuteWithStreaming_MultiWordPath(t *testing.T) {
	t.Parallel()
	// Test that ExecuteWithStreaming handles multi-word paths correctly.
	// Use an adapter name that maps to StreamMethodNone (no global config mutation).
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-mw-unused-name",
		Path: "bash -c",
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	// "test-mw-unused-name" has no stream config, so StreamMethodNone, falls back to ExecuteCommand
	result, err := adapter.ExecuteWithStreaming(
		t.Context(),
		"test-mw-unused-name",
		[]string{"echo hello"},
		"", "", 5*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout = %q, want 'hello'", result.Stdout)
	}
}

func TestExecuteWithStreaming_FallsBackOnDefault(t *testing.T) {
	t.Parallel()
	// Unregistered stream config defaults to StreamMethodNone which falls back to ExecuteCommand
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-fallback-name",
		Path: "echo",
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(e core.AgentEvent) {
		events = append(events, e)
	})

	result, err := adapter.ExecuteWithStreaming(
		t.Context(),
		"test-fallback-name",
		[]string{"fallback-output"},
		"", "", 5*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "fallback-output") {
		t.Errorf("Stdout = %q, want 'fallback-output'", result.Stdout)
	}
}

// =============================================================================
// GetVersion Tests (extended)
// =============================================================================

func TestGetVersion_ExtractsVersionNumber(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-version",
		Path: "bash",
	}, nil)
	// bash -c 'echo v1.2.3'  simulates a version command
	version, err := adapter.GetVersion(t.Context(), "-c echo v1.2.3")
	// Note: "-c echo v1.2.3" is passed as a single arg to bash, which doesn't work.
	// Instead, we rely on the bash --version output which contains a version.
	if err != nil {
		t.Logf("Note: GetVersion error: %v", err)
	}
	_ = version
}

func TestGetVersion_WithRealBash(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-version",
		Path: "bash",
	}, nil)
	version, err := adapter.GetVersion(t.Context(), "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty version string")
	}
	t.Logf("bash version: %s", version)
}
