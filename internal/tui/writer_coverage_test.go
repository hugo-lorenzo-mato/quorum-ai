package tui

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewTUIWriter
// ---------------------------------------------------------------------------

func TestNewTUIWriter_Defaults(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	if w == nil {
		t.Fatal("NewTUIWriter should return non-nil")
	}
	if w.output != nil {
		t.Error("output should be nil before Connect")
	}
	if w.fallback != &buf {
		t.Error("fallback should be the provided writer")
	}
	if len(w.buffer) != 0 {
		t.Errorf("buffer should start empty, got len=%d", len(w.buffer))
	}
}

// ---------------------------------------------------------------------------
// Write to fallback when not connected
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_Fallback(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	data := []byte("hello fallback\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}
	if buf.String() != "hello fallback\n" {
		t.Errorf("fallback buf = %q, want %q", buf.String(), "hello fallback\n")
	}
}

// ---------------------------------------------------------------------------
// IsConnected
// ---------------------------------------------------------------------------

func TestTUIWriter_IsConnected(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	if w.IsConnected() {
		t.Error("should not be connected initially")
	}

	output := NewTUIOutput()
	w.Connect(output)
	if !w.IsConnected() {
		t.Error("should be connected after Connect")
	}

	w.Disconnect()
	if w.IsConnected() {
		t.Error("should not be connected after Disconnect")
	}
}

// ---------------------------------------------------------------------------
// Connect flushes buffer
// ---------------------------------------------------------------------------

func TestTUIWriter_Connect_FlushesBuffer(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	// Pre-populate buffer manually (simulating partial write)
	w.buffer = append(w.buffer, []byte("15:04:05 INF buffered msg")...)

	output := NewTUIOutput()
	w.Connect(output)

	// After connect, buffer should be cleared
	if len(w.buffer) != 0 {
		t.Errorf("buffer should be flushed after Connect, got len=%d", len(w.buffer))
	}
}

// ---------------------------------------------------------------------------
// Write to connected TUI (complete lines)
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_Connected_CompleteLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	data := []byte("15:04:05 INF test message key=value\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}

	// The line was processed, so buffer should be empty
	if len(w.buffer) != 0 {
		t.Errorf("buffer should be empty after complete line, got len=%d, content=%q", len(w.buffer), w.buffer)
	}

	// Nothing should go to fallback when connected
	if buf.Len() != 0 {
		t.Errorf("fallback should be empty when connected, got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Write to connected TUI (partial line)
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_Connected_PartialLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	// Write partial (no newline)
	_, err := w.Write([]byte("15:04:05 INF partial"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if len(w.buffer) == 0 {
		t.Error("buffer should hold partial data")
	}

	// Complete the line
	_, err = w.Write([]byte(" message\n"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if len(w.buffer) != 0 {
		t.Errorf("buffer should be empty after completing line, got %q", w.buffer)
	}
}

// ---------------------------------------------------------------------------
// Write to connected TUI (multiple lines in one write)
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_Connected_MultipleLines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	data := []byte("15:04:05 INF line one\n15:04:06 WRN line two\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}
	if len(w.buffer) != 0 {
		t.Errorf("buffer should be empty, got %q", w.buffer)
	}
}

// ---------------------------------------------------------------------------
// Write to connected TUI (multiple lines with trailing partial)
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_Connected_LinesWithPartial(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	data := []byte("15:04:05 INF full line\n15:04:06 INF part")
	_, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Should have "15:04:06 INF part" remaining
	if !strings.Contains(string(w.buffer), "part") {
		t.Errorf("buffer should contain partial data, got %q", w.buffer)
	}
}

// ---------------------------------------------------------------------------
// Disconnect then write goes to fallback
// ---------------------------------------------------------------------------

func TestTUIWriter_Disconnect_WriteGoesToFallback(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)
	w.Disconnect()

	data := []byte("after disconnect\n")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}
	if buf.String() != "after disconnect\n" {
		t.Errorf("fallback buf = %q, want %q", buf.String(), "after disconnect\n")
	}
}

// ---------------------------------------------------------------------------
// processLine with nil output (safety check)
// ---------------------------------------------------------------------------

func TestTUIWriter_ProcessLine_NilOutput(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	// Directly call processLine with nil output - should not panic
	w.processLine([]byte("15:04:05 INF hello"))
}

// ---------------------------------------------------------------------------
// parseLogLine tests
// ---------------------------------------------------------------------------

func TestParseLogLine_EmptyString(t *testing.T) {
	t.Parallel()
	level, msg := parseLogLine("")
	if level != "info" {
		t.Errorf("level = %q, want %q", level, "info")
	}
	if msg != "" {
		t.Errorf("msg = %q, want empty", msg)
	}
}

func TestParseLogLine_WhitespaceOnly(t *testing.T) {
	t.Parallel()
	level, msg := parseLogLine("   \t  ")
	if level != "info" {
		t.Errorf("level = %q, want %q", level, "info")
	}
	if msg != "" {
		t.Errorf("msg = %q, want empty", msg)
	}
}

func TestParseLogLine_JSONWithMsgField(t *testing.T) {
	t.Parallel()
	level, msg := parseLogLine(`{"level":"info","msg":"hello world"}`)
	if level != "info" {
		t.Errorf("level = %q, want %q", level, "info")
	}
	if msg != "hello world" {
		t.Errorf("msg = %q, want %q", msg, "hello world")
	}
}

func TestParseLogLine_JSONWithMessageField(t *testing.T) {
	t.Parallel()
	level, msg := parseLogLine(`{"level":"warn","message":"disk full"}`)
	if level != "warn" {
		t.Errorf("level = %q, want %q", level, "warn")
	}
	if msg != "disk full" {
		t.Errorf("msg = %q, want %q", msg, "disk full")
	}
}

func TestParseLogLine_JSONMsgTakesPrecedence(t *testing.T) {
	t.Parallel()
	// When both "msg" and "message" are present, "msg" should be preferred
	level, msg := parseLogLine(`{"level":"debug","msg":"primary","message":"fallback"}`)
	if level != "debug" {
		t.Errorf("level = %q, want %q", level, "debug")
	}
	if msg != "primary" {
		t.Errorf("msg = %q, want %q (msg field should take precedence)", msg, "primary")
	}
}

func TestParseLogLine_InvalidJSON(t *testing.T) {
	t.Parallel()
	// Starts with { but is not valid JSON - should fall back to pretty format
	level, msg := parseLogLine("{not-valid-json")
	// Falls through to pretty handler: parts = ["{not-valid-json"]
	// With only 1 part, it returns ("info", line)
	if level != "info" {
		t.Errorf("level = %q, want %q", level, "info")
	}
	if msg != "{not-valid-json" {
		t.Errorf("msg = %q, want %q", msg, "{not-valid-json")
	}
}

func TestParseLogLine_PrettyFormat_TwoParts(t *testing.T) {
	t.Parallel()
	// Only time and level, no message
	level, msg := parseLogLine("15:04:05 ERR")
	if level != "error" {
		t.Errorf("level = %q, want %q", level, "error")
	}
	if msg != "" {
		t.Errorf("msg = %q, want empty", msg)
	}
}

func TestParseLogLine_SingleWord(t *testing.T) {
	t.Parallel()
	// Just a single word - less than 2 parts
	level, msg := parseLogLine("hello")
	if level != "info" {
		t.Errorf("level = %q, want %q", level, "info")
	}
	if msg != "hello" {
		t.Errorf("msg = %q, want %q", msg, "hello")
	}
}

// ---------------------------------------------------------------------------
// normalizeLevel tests
// ---------------------------------------------------------------------------

func TestNormalizeLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"INF", "info"},
		{"INFO", "info"},
		{"info", "info"},
		{"WRN", "warn"},
		{"WARN", "warn"},
		{"WARNING", "warn"},
		{"warn", "warn"},
		{"ERR", "error"},
		{"ERROR", "error"},
		{"error", "error"},
		{"DBG", "debug"},
		{"DEBUG", "debug"},
		{"debug", "debug"},
		{"UNKNOWN", "info"},   // default
		{"", "info"},          // empty
		{"  INFO  ", "info"},  // whitespace
		{"  WRN  ", "warn"},   // whitespace
		{"something", "info"}, // arbitrary
	}

	for _, tc := range tests {
		result := normalizeLevel(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeLevel(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// processLines tests
// ---------------------------------------------------------------------------

func TestTUIWriter_ProcessLines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	// processLines is called internally; test it directly
	w.processLines([]byte("15:04:05 INF line1\n15:04:06 WRN line2\n"))
	// Should not panic and should process both lines
}

func TestTUIWriter_ProcessLines_EmptyLines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	// Empty lines should be skipped
	w.processLines([]byte("\n\n"))
}

// ---------------------------------------------------------------------------
// Concurrent access safety
// ---------------------------------------------------------------------------

func TestTUIWriter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)

	output := NewTUIOutput()
	w.Connect(output)

	done := make(chan struct{})

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = w.Write([]byte("15:04:05 INF msg\n"))
		}
		done <- struct{}{}
	}()

	// Concurrent IsConnected checks
	go func() {
		for i := 0; i < 100; i++ {
			_ = w.IsConnected()
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

func TestTUIWriter_ConcurrentConnectDisconnect(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)
	output := NewTUIOutput()

	done := make(chan struct{})

	go func() {
		for i := 0; i < 50; i++ {
			w.Connect(output)
			w.Disconnect()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 50; i++ {
			_, _ = w.Write([]byte("15:04:05 INF test\n"))
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

// ---------------------------------------------------------------------------
// Write returns correct byte count
// ---------------------------------------------------------------------------

func TestTUIWriter_Write_ReturnsCorrectLen(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewTUIWriter(&buf)
	output := NewTUIOutput()
	w.Connect(output)

	data := []byte("15:04:05 INF hello\nworld\npartial")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}
}
