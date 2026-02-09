package tui

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestTUILogHandler_Handle(t *testing.T) {
	t.Parallel()
	output := NewTUIOutput()
	handler := NewTUILogHandler(output, slog.LevelInfo)

	// Create a record
	record := slog.NewRecord(time.Now(), slog.LevelWarn, "test warning", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle returned error: %v", err)
	}

	// Verify log was sent (would need to check the channel in a real test)
}

func TestTUILogHandler_LevelFiltering(t *testing.T) {
	t.Parallel()
	output := NewTUIOutput()
	handler := NewTUILogHandler(output, slog.LevelWarn)

	// Debug should be filtered
	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug should be filtered at Warn level")
	}

	// Warn should pass
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("Warn should be enabled at Warn level")
	}

	// Error should pass
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error should be enabled at Warn level")
	}
}
