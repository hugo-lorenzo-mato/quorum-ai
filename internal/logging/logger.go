package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"golang.org/x/term"
)

// Logger wraps slog.Logger with additional features.
type Logger struct {
	*slog.Logger
	sanitizer *Sanitizer
}

// Config configures the logger.
type Config struct {
	Level     string
	Format    string // auto, text, json
	Output    io.Writer
	AddSource bool
}

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    "auto",
		Output:    os.Stdout,
		AddSource: false,
	}
}

// New creates a new logger.
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	level := parseLevel(cfg.Level)
	sanitizer := NewSanitizer()

	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{
			Level:     level,
			AddSource: cfg.AddSource,
		})
	case "text":
		handler = slog.NewTextHandler(cfg.Output, &slog.HandlerOptions{
			Level:     level,
			AddSource: cfg.AddSource,
		})
	default: // auto
		if isTerminal(cfg.Output) {
			handler = NewPrettyHandler(cfg.Output, level)
		} else {
			handler = slog.NewJSONHandler(cfg.Output, &slog.HandlerOptions{
				Level:     level,
				AddSource: cfg.AddSource,
			})
		}
	}

	// Wrap with sanitizing handler
	handler = NewSanitizingHandler(handler, sanitizer)

	return &Logger{
		Logger:    slog.New(handler),
		sanitizer: sanitizer,
	}
}

// NewNop creates a no-op logger for testing.
func NewNop() *Logger {
	return &Logger{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		sanitizer: NewSanitizer(),
	}
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// WithContext returns a logger with context values.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract trace ID, request ID, etc. from context if present
	_ = ctx
	return l
}

// WithTask returns a logger with task context.
func (l *Logger) WithTask(taskID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With("task_id", taskID),
		sanitizer: l.sanitizer,
	}
}

// WithPhase returns a logger with phase context.
func (l *Logger) WithPhase(phase string) *Logger {
	return &Logger{
		Logger:    l.Logger.With("phase", phase),
		sanitizer: l.sanitizer,
	}
}

// WithWorkflow returns a logger with workflow context.
func (l *Logger) WithWorkflow(workflowID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With("workflow_id", workflowID),
		sanitizer: l.sanitizer,
	}
}

// WithAgent returns a logger with agent context.
func (l *Logger) WithAgent(agent string) *Logger {
	return &Logger{
		Logger:    l.Logger.With("agent", agent),
		sanitizer: l.sanitizer,
	}
}

// With returns a logger with custom fields.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger:    l.Logger.With(args...),
		sanitizer: l.sanitizer,
	}
}

// Sanitizer returns the sanitizer used by this logger.
func (l *Logger) Sanitizer() *Sanitizer {
	return l.sanitizer
}

// Sanitize sanitizes a string using the logger's sanitizer.
func (l *Logger) Sanitize(input string) string {
	return l.sanitizer.Sanitize(input)
}
