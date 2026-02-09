package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("DefaultConfig().Level = %q, want \"info\"", cfg.Level)
	}
	if cfg.Format != "auto" {
		t.Errorf("DefaultConfig().Format = %q, want \"auto\"", cfg.Format)
	}
	if cfg.Output == nil {
		t.Error("DefaultConfig().Output should not be nil")
	}
	if cfg.AddSource {
		t.Error("DefaultConfig().AddSource should be false")
	}
}

func TestLogger_NilOutput(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Level:  "info",
		Format: "text",
		Output: nil, // Should default to os.Stdout
	}

	logger := New(cfg)
	if logger == nil {
		t.Fatal("New() with nil output should not return nil")
	}

	// Should not panic
	logger.Info("test message")
}

func TestLogger_WithAddSource(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cfg := Config{
		Level:     "info",
		Format:    "text",
		Output:    &buf,
		AddSource: true,
	}

	logger := New(cfg)
	logger.Info("test message")

	// With AddSource, output should include source information
	output := buf.String()
	if output == "" {
		t.Error("Expected log output")
	}
}

func TestLogger_With(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	withLogger := logger.With("key1", "value1", "key2", 42)
	withLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key1") {
		t.Error("Expected key1 in output")
	}
	if !strings.Contains(output, "value1") {
		t.Error("Expected value1 in output")
	}
}

func TestLogger_WithContextExtended(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())

	ctx := context.Background()
	ctxLogger := logger.WithContext(ctx)

	if ctxLogger == nil {
		t.Error("WithContext() should not return nil")
	}

	// Should be the same logger (context extraction not implemented)
	if ctxLogger != logger {
		t.Error("WithContext() should return same logger for now")
	}
}

func TestLogger_SanitizerAccess(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())

	sanitizer := logger.Sanitizer()
	if sanitizer == nil {
		t.Error("Sanitizer() should not return nil")
	}

	// Test that it's functional
	result := sanitizer.Sanitize("key=sk-1234567890abcdefghijklmnop")
	if !strings.Contains(result, "[REDACTED]") {
		t.Error("Sanitizer should redact API keys")
	}
}

func TestLogger_ChainedWith(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	// Chain multiple With calls
	finalLogger := logger.
		WithWorkflow("wf-123").
		WithPhase("analyze").
		WithTask("task-1").
		WithAgent("claude")

	finalLogger.Info("chained log")

	output := buf.String()
	if !strings.Contains(output, "wf-123") {
		t.Error("Expected workflow_id in output")
	}
	if !strings.Contains(output, "analyze") {
		t.Error("Expected phase in output")
	}
	if !strings.Contains(output, "task-1") {
		t.Error("Expected task_id in output")
	}
	if !strings.Contains(output, "claude") {
		t.Error("Expected agent in output")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	logger.Info("test message", "key", "value")

	output := buf.String()
	// JSON format should contain JSON structure
	if !strings.Contains(output, "{") {
		t.Error("JSON format should produce JSON output")
	}
	if !strings.Contains(output, "test message") {
		t.Error("JSON output should contain message")
	}
}

func TestLogger_TextFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "text",
		Output: &buf,
	})

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("Text output should contain message")
	}
}

func TestParseLevel_AllLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "INFO"}, // Case sensitive, defaults to INFO
		{"info", "INFO"},
		{"warn", "WARN"},
		{"warning", "INFO"}, // Only "warn" is recognized
		{"error", "ERROR"},
		{"err", "INFO"}, // Only "error" is recognized
		{"fatal", "INFO"},
		{"", "INFO"},
		{"unknown", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got.String() != tt.want {
				t.Errorf("parseLevel(%q) = %s, want %s", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestSanitizer_MultiplePatterns(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	// Input with multiple sensitive values
	// GitHub PAT requires exactly 36 chars after ghp_
	input := "OpenAI: sk-1234567890abcdefghijklmnop, GitHub: ghp_abcdefghij1234567890abcdefghij123456"
	result := sanitizer.Sanitize(input)

	// Both should be redacted
	if strings.Contains(result, "sk-1234567890") {
		t.Error("OpenAI key should be redacted")
	}
	if strings.Contains(result, "ghp_abcdefghij") {
		t.Error("GitHub token should be redacted")
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Error("Should contain [REDACTED]")
	}
}

func TestSanitizer_EmptyInput(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	result := sanitizer.Sanitize("")
	if result != "" {
		t.Error("Empty input should produce empty output")
	}
}

func TestSanitizer_SanitizeMap_EmptyMap(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	result := sanitizer.SanitizeMap(map[string]interface{}{})
	if len(result) != 0 {
		t.Error("Empty map should produce empty result")
	}
}

func TestSanitizer_SanitizeMap_NilValue(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"null_key": nil,
		"string":   "value",
	}

	result := sanitizer.SanitizeMap(input)
	if result["null_key"] != nil {
		t.Error("Nil value should remain nil")
	}
	if result["string"] != "value" {
		t.Error("String value should be unchanged")
	}
}

func TestSanitizer_SanitizeMap_Slice(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"slice": []interface{}{"a", "b", "sk-1234567890abcdefghijklmnop"},
	}

	result := sanitizer.SanitizeMap(input)
	slice, ok := result["slice"].([]interface{})
	if !ok {
		t.Fatal("Slice should be preserved")
	}
	if len(slice) != 3 {
		t.Error("Slice length should be 3")
	}
}

func TestNewNop_Operations(t *testing.T) {
	t.Parallel()
	logger := NewNop()

	// All operations should work without panic
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.With("key", "value").Info("with key")
	logger.WithTask("task-1").Info("with task")
	logger.WithPhase("analyze").Info("with phase")
	logger.WithWorkflow("wf-1").Info("with workflow")
	logger.WithAgent("claude").Info("with agent")
	logger.WithContext(context.Background()).Info("with context")
}

func TestPrettyHandler_AllLevels(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	handler := NewPrettyHandler(&buf, parseLevel("debug"))
	logger := &Logger{
		Logger:    slog.New(handler),
		sanitizer: NewSanitizer(),
	}

	// Test all levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "DBG") {
		t.Error("Expected DBG level marker")
	}
	if !strings.Contains(output, "INF") {
		t.Error("Expected INF level marker")
	}
	if !strings.Contains(output, "WRN") {
		t.Error("Expected WRN level marker")
	}
	if !strings.Contains(output, "ERR") {
		t.Error("Expected ERR level marker")
	}
}

func TestIsTerminal_NonFile(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	result := isTerminal(&buf)
	if result {
		t.Error("bytes.Buffer should not be detected as terminal")
	}
}

func TestSetRedactedPlaceholder(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	// Set custom placeholder
	sanitizer.SetRedactedPlaceholder("[HIDDEN]")

	input := "API key: sk-1234567890abcdefghijklmnop"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[HIDDEN]") {
		t.Errorf("Expected custom placeholder [HIDDEN], got: %s", result)
	}
	if strings.Contains(result, "[REDACTED]") {
		t.Error("Should not contain default [REDACTED] placeholder")
	}
}

func TestSanitizingHandler_WithAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	sanitizer := NewSanitizer()
	handler := NewSanitizingHandler(baseHandler, sanitizer)

	// Create attrs with sensitive data
	attrs := []slog.Attr{
		slog.String("api_key", "sk-1234567890abcdefghijklmnop"),
		slog.String("normal", "hello"),
	}

	newHandler := handler.WithAttrs(attrs)
	if newHandler == nil {
		t.Fatal("WithAttrs should not return nil")
	}

	// The new handler should be a SanitizingHandler
	_, ok := newHandler.(*SanitizingHandler)
	if !ok {
		t.Error("WithAttrs should return a SanitizingHandler")
	}
}

func TestSanitizingHandler_WithGroupDirect(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	sanitizer := NewSanitizer()
	handler := NewSanitizingHandler(baseHandler, sanitizer)

	newHandler := handler.WithGroup("request")
	if newHandler == nil {
		t.Fatal("WithGroup should not return nil")
	}

	// The new handler should be a SanitizingHandler
	_, ok := newHandler.(*SanitizingHandler)
	if !ok {
		t.Error("WithGroup should return a SanitizingHandler")
	}
}

func TestSanitizingHandler_SanitizeAttr_Group(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	sanitizer := NewSanitizer()
	handler := NewSanitizingHandler(baseHandler, sanitizer)

	// Log with a group containing sensitive data
	logger := slog.New(handler)
	logger.Info("test",
		slog.Group("credentials",
			slog.String("api_key", "sk-1234567890abcdefghijklmnop"),
			slog.String("username", "user"),
		),
	)

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("API key in group should be sanitized, got: %s", output)
	}
}

func TestSanitizingHandler_SanitizeAttr_NonString(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	sanitizer := NewSanitizer()
	handler := NewSanitizingHandler(baseHandler, sanitizer)

	// Log with non-string attrs
	logger := slog.New(handler)
	logger.Info("test",
		slog.Int("count", 42),
		slog.Bool("active", true),
		slog.Float64("ratio", 3.14),
	)

	output := buf.String()
	if !strings.Contains(output, "42") {
		t.Error("Int value should be preserved")
	}
	if !strings.Contains(output, "true") {
		t.Error("Bool value should be preserved")
	}
}

func TestPrettyHandler_WithAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, slog.LevelInfo)

	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	newHandler := handler.WithAttrs(attrs)
	if newHandler == nil {
		t.Fatal("WithAttrs should not return nil")
	}

	// The new handler should be a PrettyHandler
	ph, ok := newHandler.(*PrettyHandler)
	if !ok {
		t.Error("WithAttrs should return a PrettyHandler")
	}

	// Log to verify attrs are used
	logger := slog.New(ph)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key1") {
		t.Error("Expected key1 in output")
	}
}

func TestPrettyHandler_WithGroup(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, slog.LevelInfo)

	newHandler := handler.WithGroup("mygroup")
	if newHandler == nil {
		t.Fatal("WithGroup should not return nil")
	}

	// The new handler should be a PrettyHandler
	ph, ok := newHandler.(*PrettyHandler)
	if !ok {
		t.Error("WithGroup should return a PrettyHandler")
	}

	// Log with attrs to verify group prefix is applied
	grouped := slog.New(ph).With("key", "value")
	grouped.Info("test")

	output := buf.String()
	if !strings.Contains(output, "mygroup") {
		t.Error("Expected group name in output")
	}
}

func TestPrettyHandler_FormatAttr_Group(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler)

	// Log with a group
	logger.Info("test",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/v1"),
		),
	)

	output := buf.String()
	if !strings.Contains(output, "method") {
		t.Error("Expected method in output")
	}
	if !strings.Contains(output, "path") {
		t.Error("Expected path in output")
	}
}

func TestPrettyHandler_FormatLevel_Unknown(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, slog.LevelDebug) // Use debug to enable all levels

	ctx := context.Background()

	// Use a custom level value that's not DEBUG/INFO/WARN/ERROR
	logger := slog.New(handler)
	logger.Log(ctx, slog.Level(100), "unknown level message")

	output := buf.String()
	// The default case returns level.String()[:3]
	if output == "" {
		t.Error("Expected some output for unknown level")
	}
}
