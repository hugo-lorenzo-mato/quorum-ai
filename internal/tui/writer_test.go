package tui

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestNewTUIWriter(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)

	if w == nil {
		t.Fatal("NewTUIWriter returned nil")
	}
	if w.fallback != fallback {
		t.Error("fallback writer not set correctly")
	}
	if w.output != nil {
		t.Error("output should be nil before connection")
	}
	if !w.IsConnected() == false {
		t.Error("IsConnected should return false before connection")
	}
}

func TestTUIWriter_WriteBeforeConnection(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)

	msg := "test message\n"
	n, err := w.Write([]byte(msg))

	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned wrong byte count: got %d, want %d", n, len(msg))
	}
	if fallback.String() != msg {
		t.Errorf("fallback buffer content: got %q, want %q", fallback.String(), msg)
	}
}

func TestTUIWriter_WriteAfterConnection(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)

	// Create a TUIOutput to connect
	tuiOutput := NewTUIOutput()

	// Connect the writer
	w.Connect(tuiOutput)

	if !w.IsConnected() {
		t.Error("IsConnected should return true after connection")
	}

	// Write a log line
	msg := "15:04:05 INF test message key=value\n"
	n, err := w.Write([]byte(msg))

	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned wrong byte count: got %d, want %d", n, len(msg))
	}

	// Fallback should not have received the message
	if fallback.Len() > 0 {
		t.Errorf("fallback should be empty when connected, got: %q", fallback.String())
	}
}

func TestTUIWriter_Disconnect(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()

	w.Connect(tuiOutput)
	if !w.IsConnected() {
		t.Error("should be connected after Connect")
	}

	w.Disconnect()
	if w.IsConnected() {
		t.Error("should not be connected after Disconnect")
	}

	// Write should go to fallback after disconnect
	msg := "after disconnect\n"
	_, _ = w.Write([]byte(msg))
	if fallback.String() != msg {
		t.Errorf("write after disconnect should go to fallback")
	}
}

func TestTUIWriter_PartialLineBuffering(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	// Write partial line (no newline)
	_, _ = w.Write([]byte("15:04:05 INF partial"))

	// Buffer should hold the partial line
	w.mu.Lock()
	bufLen := len(w.buffer)
	w.mu.Unlock()

	if bufLen == 0 {
		t.Error("buffer should contain partial line")
	}

	// Complete the line
	_, _ = w.Write([]byte(" message\n"))

	// Buffer should be empty after complete line
	w.mu.Lock()
	bufLen = len(w.buffer)
	w.mu.Unlock()

	if bufLen != 0 {
		t.Errorf("buffer should be empty after complete line, got %d bytes", bufLen)
	}
}

func TestTUIWriter_MultipleLines(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	// Write multiple lines at once
	msg := "15:04:05 INF line one\n15:04:06 WRN line two\n15:04:07 ERR line three\n"
	n, err := w.Write([]byte(msg))

	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned wrong byte count: got %d, want %d", n, len(msg))
	}
}

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantLevel   string
		wantMessage string
	}{
		{
			name:        "info level",
			input:       "15:04:05 INF test message key=value",
			wantLevel:   "info",
			wantMessage: "test message key=value",
		},
		{
			name:        "warn level",
			input:       "15:04:05 WRN warning message",
			wantLevel:   "warn",
			wantMessage: "warning message",
		},
		{
			name:        "error level",
			input:       "15:04:05 ERR error occurred",
			wantLevel:   "error",
			wantMessage: "error occurred",
		},
		{
			name:        "debug level",
			input:       "15:04:05 DBG debug info",
			wantLevel:   "debug",
			wantMessage: "debug info",
		},
		{
			name:        "INFO uppercase",
			input:       "15:04:05 INFO full info level",
			wantLevel:   "info",
			wantMessage: "full info level",
		},
		{
			name:        "WARN uppercase",
			input:       "15:04:05 WARN full warn level",
			wantLevel:   "warn",
			wantMessage: "full warn level",
		},
		{
			name:        "ERROR uppercase",
			input:       "15:04:05 ERROR full error level",
			wantLevel:   "error",
			wantMessage: "full error level",
		},
		{
			name:        "DEBUG uppercase",
			input:       "15:04:05 DEBUG full debug level",
			wantLevel:   "debug",
			wantMessage: "full debug level",
		},
		{
			name:        "unknown format",
			input:       "some random text",
			wantLevel:   "info",
			wantMessage: "some random text",
		},
		{
			name:        "empty string",
			input:       "",
			wantLevel:   "info",
			wantMessage: "",
		},
		{
			name:        "only timestamp and level",
			input:       "15:04:05 INF",
			wantLevel:   "info",
			wantMessage: "",
		},
		{
			name:        "whitespace",
			input:       "   ",
			wantLevel:   "info",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, message := parseLogLine(tt.input)
			if level != tt.wantLevel {
				t.Errorf("parseLogLine() level = %q, want %q", level, tt.wantLevel)
			}
			if message != tt.wantMessage {
				t.Errorf("parseLogLine() message = %q, want %q", message, tt.wantMessage)
			}
		})
	}
}

func TestTUIWriter_ConcurrentWrites(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	var wg sync.WaitGroup
	numGoroutines := 10
	writesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				msg := "15:04:05 INF concurrent message\n"
				_, err := w.Write([]byte(msg))
				if err != nil {
					t.Errorf("concurrent write error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestTUIWriter_FlushBufferOnConnect(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)

	// Write partial content before connection
	_, _ = w.Write([]byte("15:04:05 INF buffered"))
	_, _ = w.Write([]byte(" content\n"))

	// At this point, content went to fallback
	fallbackContent := fallback.String()
	if fallbackContent == "" {
		t.Error("content should have gone to fallback before connection")
	}

	// Now connect - any buffered content should be flushed
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	// Buffer should be empty after connection
	w.mu.Lock()
	bufLen := len(w.buffer)
	w.mu.Unlock()

	if bufLen != 0 {
		t.Errorf("buffer should be empty after connection, got %d bytes", bufLen)
	}
}

func TestTUIWriter_ConnectWithBufferedContent(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)

	// First write goes to fallback (not connected)
	_, _ = w.Write([]byte("15:04:05 INF first line\n"))

	// Content should be in fallback
	if fallback.Len() == 0 {
		t.Error("first line should be in fallback")
	}

	// Connect
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	// Write after connection should go to TUI
	fallback.Reset()
	_, _ = w.Write([]byte("15:04:06 INF second line\n"))

	// Fallback should remain empty (not receive new writes)
	if fallback.Len() > 0 {
		t.Error("writes after connection should not go to fallback")
	}
}

// BenchmarkTUIWriter_Write benchmarks the write performance.
func BenchmarkTUIWriter_Write(b *testing.B) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()
	w.Connect(tuiOutput)

	msg := []byte("15:04:05 INF benchmark message with some content\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = w.Write(msg)
	}
}

// BenchmarkParseLogLine benchmarks log line parsing.
func BenchmarkParseLogLine(b *testing.B) {
	line := "15:04:05 INF benchmark message with key=value pairs"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseLogLine(line)
	}
}

// TestTUIWriter_Integration tests the integration with real TUIOutput.
func TestTUIWriter_Integration(t *testing.T) {
	fallback := &bytes.Buffer{}
	w := NewTUIWriter(fallback)
	tuiOutput := NewTUIOutput()

	// Don't start the TUI program, just test message routing
	w.Connect(tuiOutput)

	// Write several log messages
	messages := []string{
		"15:04:05 INF starting workflow\n",
		"15:04:06 INF task started task_id=1\n",
		"15:04:07 WRN resource limited\n",
		"15:04:08 ERR task failed error=timeout\n",
		"15:04:09 INF workflow completed\n",
	}

	for _, msg := range messages {
		n, err := w.Write([]byte(msg))
		if err != nil {
			t.Errorf("Write error: %v", err)
		}
		if n != len(msg) {
			t.Errorf("Write byte count mismatch: got %d, want %d", n, len(msg))
		}
	}

	// Give time for async processing
	time.Sleep(10 * time.Millisecond)

	// Verify no errors occurred
	if fallback.Len() > 0 {
		t.Errorf("unexpected fallback content: %q", fallback.String())
	}
}
