package tui

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

// TUIWriter implements io.Writer and routes log output to the TUI.
// Before connection to a TUIOutput, it writes to the fallback writer.
// After connection, it parses log lines and sends them as LogMsg to the TUI.
type TUIWriter struct {
	mu       sync.Mutex
	output   *TUIOutput // Connected TUI output (nil until connected)
	buffer   []byte     // Buffer for incomplete lines
	fallback io.Writer  // Fallback writer when not connected or TUI unavailable
}

// NewTUIWriter creates a new TUIWriter with the given fallback writer.
// The fallback is used when the TUI is not yet connected or becomes unavailable.
func NewTUIWriter(fallback io.Writer) *TUIWriter {
	return &TUIWriter{
		fallback: fallback,
		buffer:   make([]byte, 0, 256),
	}
}

// Connect associates this writer with a TUIOutput.
// After connection, log writes will be routed to the TUI instead of fallback.
func (w *TUIWriter) Connect(output *TUIOutput) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.output = output

	// Flush any buffered content to the TUI
	if len(w.buffer) > 0 {
		w.processLines(w.buffer)
		w.buffer = w.buffer[:0]
	}
}

// Disconnect removes the TUIOutput connection.
// Subsequent writes will go to the fallback writer.
func (w *TUIWriter) Disconnect() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.output = nil
}

// Write implements io.Writer.
// It buffers incomplete lines and processes complete lines.
func (w *TUIWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If not connected to TUI, write to fallback
	if w.output == nil {
		return w.fallback.Write(p)
	}

	// Append to buffer
	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		idx := bytes.IndexByte(w.buffer, '\n')
		if idx < 0 {
			break
		}

		line := w.buffer[:idx]
		w.buffer = w.buffer[idx+1:]

		w.processLine(line)
	}

	return len(p), nil
}

// processLines processes multiple lines from a byte slice.
func (w *TUIWriter) processLines(data []byte) {
	lines := bytes.Split(data, []byte{'\n'})
	for _, line := range lines {
		if len(line) > 0 {
			w.processLine(line)
		}
	}
}

// processLine parses a single log line and sends it to the TUI.
// Expected format from PrettyHandler: "15:04:05 INF message key=value"
func (w *TUIWriter) processLine(line []byte) {
	if w.output == nil {
		return
	}

	level, message := parseLogLine(string(line))
	w.output.Log(level, message)
}

// parseLogLine extracts the level and message from a log line.
// It handles the PrettyHandler format: "15:04:05 INF message key=value"
// Returns normalized level (info, warn, error, debug) and the message.
func parseLogLine(line string) (level, message string) {
	// Skip empty lines
	line = strings.TrimSpace(line)
	if line == "" {
		return "info", ""
	}

	// PrettyHandler format: "15:04:05 INF message key=value"
	// Try to find the level marker after the timestamp
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return "info", line
	}

	// Check if second part is a level indicator
	levelStr := strings.ToUpper(parts[1])

	// Extract message (third part if exists, empty otherwise)
	msg := ""
	if len(parts) > 2 {
		msg = parts[2]
	}

	switch levelStr {
	case "INF", "INFO":
		return "info", msg
	case "WRN", "WARN", "WARNING":
		return "warn", msg
	case "ERR", "ERROR":
		return "error", msg
	case "DBG", "DEBUG":
		return "debug", msg
	default:
		// Not a recognized format, return the whole line as message
		return "info", line
	}
}

// IsConnected returns whether the writer is connected to a TUIOutput.
func (w *TUIWriter) IsConnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.output != nil
}
