package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// SanitizingHandler wraps another handler and sanitizes log attributes.
type SanitizingHandler struct {
	handler   slog.Handler
	sanitizer *Sanitizer
}

// NewSanitizingHandler creates a new sanitizing handler.
func NewSanitizingHandler(handler slog.Handler, sanitizer *Sanitizer) *SanitizingHandler {
	return &SanitizingHandler{
		handler:   handler,
		sanitizer: sanitizer,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle sanitizes the record and passes it to the underlying handler.
func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Sanitize the message
	sanitizedMsg := h.sanitizer.Sanitize(r.Message)

	// Create a new record with sanitized values
	newRecord := slog.NewRecord(r.Time, r.Level, sanitizedMsg, r.PC)

	// Sanitize attributes
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.sanitizeAttr(a))
		return true
	})

	return h.handler.Handle(ctx, newRecord)
}

// WithAttrs returns a new handler with sanitized attrs.
func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitizedAttrs := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		sanitizedAttrs[i] = h.sanitizeAttr(attr)
	}
	return &SanitizingHandler{
		handler:   h.handler.WithAttrs(sanitizedAttrs),
		sanitizer: h.sanitizer,
	}
}

// WithGroup returns a new handler with a group.
func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{
		handler:   h.handler.WithGroup(name),
		sanitizer: h.sanitizer,
	}
}

func (h *SanitizingHandler) sanitizeAttr(a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.Attr{
			Key:   a.Key,
			Value: slog.StringValue(h.sanitizer.Sanitize(a.Value.String())),
		}
	case slog.KindGroup:
		attrs := a.Value.Group()
		sanitized := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			sanitized[i] = h.sanitizeAttr(attr)
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(sanitized...),
		}
	default:
		return a
	}
}

// PrettyHandler provides colorized console output for TTY.
type PrettyHandler struct {
	mu     sync.Mutex
	w      io.Writer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// NewPrettyHandler creates a new pretty handler.
func NewPrettyHandler(w io.Writer, level slog.Level) *PrettyHandler {
	return &PrettyHandler{
		w:     w,
		level: level,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes the log record.
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Format level with color
	levelStr := h.formatLevel(r.Level)

	// Format time
	timeStr := r.Time.Format("15:04:05")

	// Build the log line
	line := fmt.Sprintf("%s %s %s", timeStr, levelStr, r.Message)

	// Add pre-set attrs
	for _, attr := range h.attrs {
		line += h.formatAttr(attr)
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		line += h.formatAttr(a)
		return true
	})

	_, err := fmt.Fprintln(h.w, line)
	return err
}

// WithAttrs returns a new handler with attrs.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &PrettyHandler{
		w:      h.w,
		level:  h.level,
		attrs:  make([]slog.Attr, len(h.attrs)+len(attrs)),
		groups: h.groups,
	}
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.attrs[len(h.attrs):], attrs)
	return newHandler
}

// WithGroup returns a new handler with a group.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	newHandler := &PrettyHandler{
		w:      h.w,
		level:  h.level,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
	return newHandler
}

func (h *PrettyHandler) formatLevel(level slog.Level) string {
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
		colorGray   = "\033[90m"
	)

	switch level {
	case slog.LevelDebug:
		return colorGray + "DBG" + colorReset
	case slog.LevelInfo:
		return colorBlue + "INF" + colorReset
	case slog.LevelWarn:
		return colorYellow + "WRN" + colorReset
	case slog.LevelError:
		return colorRed + "ERR" + colorReset
	default:
		return level.String()[:3]
	}
}

func (h *PrettyHandler) formatAttr(a slog.Attr) string {
	const (
		colorReset = "\033[0m"
		colorCyan  = "\033[36m"
	)

	if a.Value.Kind() == slog.KindGroup {
		var result string
		for _, attr := range a.Value.Group() {
			result += h.formatAttr(attr)
		}
		return result
	}

	key := a.Key
	for i := len(h.groups) - 1; i >= 0; i-- {
		key = h.groups[i] + "." + key
	}

	return fmt.Sprintf(" %s%s%s=%v", colorCyan, key, colorReset, a.Value.Any())
}
