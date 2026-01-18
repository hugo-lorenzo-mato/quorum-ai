package tui

import (
	"context"
	"log/slog"
	"sync"
)

// TUILogHandler is a slog.Handler that routes logs directly to a TUIOutput.
// This bypasses string parsing entirely, solving the JSON vs Pretty format issue.
type TUILogHandler struct {
	output *TUIOutput
	level  slog.Level
	attrs  []slog.Attr
	groups []string
	mu     sync.RWMutex
}

// NewTUILogHandler creates a new TUI log handler.
func NewTUILogHandler(output *TUIOutput, level slog.Level) *TUILogHandler {
	return &TUILogHandler{
		output: output,
		level:  level,
		attrs:  make([]slog.Attr, 0),
		groups: make([]string, 0),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *TUILogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle handles the Record by sending a LogMsg to the TUI.
func (h *TUILogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.RLock()
	output := h.output
	h.mu.RUnlock()

	if output == nil {
		return nil
	}

	// Convert slog level to string
	level := levelToString(r.Level)

	// Build message with attributes
	message := r.Message

	// Add any record attributes to the message
	var attrParts []string
	r.Attrs(func(a slog.Attr) bool {
		attrParts = append(attrParts, a.Key+"="+a.Value.String())
		return true
	})

	if len(attrParts) > 0 {
		message += " " + joinStrings(attrParts, " ")
	}

	// Send directly to TUI as LogMsg
	output.Log(level, message)
	return nil
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *TUILogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &TUILogHandler{
		output: h.output,
		level:  h.level,
		attrs:  make([]slog.Attr, len(h.attrs)+len(attrs)),
		groups: make([]string, len(h.groups)),
	}
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.attrs[len(h.attrs):], attrs)
	copy(newHandler.groups, h.groups)
	return newHandler
}

// WithGroup returns a new Handler with the given group appended to the receiver's
// existing groups.
func (h *TUILogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newHandler := &TUILogHandler{
		output: h.output,
		level:  h.level,
		attrs:  make([]slog.Attr, len(h.attrs)),
		groups: make([]string, len(h.groups)+1),
	}
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.groups, h.groups)
	newHandler.groups[len(h.groups)] = name
	return newHandler
}

// SetOutput updates the TUIOutput (for late binding).
func (h *TUILogHandler) SetOutput(output *TUIOutput) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.output = output
}

func levelToString(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warn"
	case level >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
