package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizer_OpenAI(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "Using API key sk-1234567890abcdefghijklmnop"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected OpenAI key to be redacted, got: %s", result)
	}
	if strings.Contains(result, "sk-1234567890") {
		t.Errorf("expected OpenAI key to be removed, got: %s", result)
	}
}

func TestSanitizer_Anthropic(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "Using Anthropic key sk-ant-api03-1234567890abcdefghij1234567890abcdefghij"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected Anthropic key to be redacted, got: %s", result)
	}
}

func TestSanitizer_GitHub(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	tests := []struct {
		name  string
		input string
	}{
		{"PAT", "ghp_1234567890abcdefghijklmnopqrstuvwxyz"},
		{"OAuth", "gho_1234567890abcdefghijklmnopqrstuvwxyz"},
		{"App User", "ghu_1234567890abcdefghijklmnopqrstuvwxyz"},
		{"App Server", "ghs_1234567890abcdefghijklmnopqrstuvwxyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize("Token: " + tt.input)
			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("expected GitHub %s to be redacted, got: %s", tt.name, result)
			}
		})
	}
}

func TestSanitizer_AWS(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "AWS key: AKIAIOSFODNN7EXAMPLE"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected AWS key to be redacted, got: %s", result)
	}
}

func TestSanitizer_GoogleAI(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "Google API key: AIzaSyD00000000000000000000000000000000"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected Google AI key to be redacted, got: %s", result)
	}
}

func TestSanitizer_Slack(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "Slack token: xoxb-1234567890-1234567890123-abcdefghij"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected Slack token to be redacted, got: %s", result)
	}
}

func TestSanitizer_Bearer(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected Bearer token to be redacted, got: %s", result)
	}
}

func TestSanitizer_GenericPatterns(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	tests := []struct {
		name  string
		input string
	}{
		{"api_key", `api_key="abc123def456ghi789jkl012"`},
		{"api-key", `api-key: abc123def456ghi789jkl012`},
		{"secret", `secret="my_super_secret_key_12345"`},
		{"password", `password="verysecretpassword123"`},
		{"token", `token="some_long_token_value_here"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)
			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("expected %s to be redacted, got: %s", tt.name, result)
			}
		})
	}
}

func TestSanitizer_NoFalsePositives(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	safeStrings := []string{
		"Hello, world!",
		"This is a normal log message",
		"Processing task-123",
		"File path: /home/user/project",
		"HTTP status: 200 OK",
		"UUID: 550e8400-e29b-41d4-a716-446655440000",
		"Email: user@example.com",
		"URL: https://example.com/api/v1",
		"Short token: abc123", // Too short for patterns
	}

	for _, input := range safeStrings {
		result := sanitizer.Sanitize(input)
		if strings.Contains(result, "[REDACTED]") {
			t.Errorf("false positive for: %s, got: %s", input, result)
		}
	}
}

func TestSanitizer_SanitizeMap(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	input := map[string]interface{}{
		"api_key": `api_key="sk-1234567890abcdefghijklmnop"`,
		"normal":  "hello world",
		"number":  42,
		"nested": map[string]interface{}{
			"secret": `secret="nested_secret_value_here123"`,
		},
	}

	result := sanitizer.SanitizeMap(input)

	if !strings.Contains(result["api_key"].(string), "[REDACTED]") {
		t.Errorf("expected api_key to be redacted")
	}

	if result["normal"] != "hello world" {
		t.Errorf("expected normal to be unchanged")
	}

	if result["number"] != 42 {
		t.Errorf("expected number to be unchanged")
	}

	nested := result["nested"].(map[string]interface{})
	if !strings.Contains(nested["secret"].(string), "[REDACTED]") {
		t.Errorf("expected nested secret to be redacted")
	}
}

func TestSanitizer_AddPattern(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()

	// Add a custom pattern for a fictional service
	err := sanitizer.AddPattern(`myservice_[a-z0-9]{20}`)
	if err != nil {
		t.Fatalf("AddPattern() error = %v", err)
	}

	input := "Using myservice_abcdefghij1234567890"
	result := sanitizer.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected custom pattern to be redacted, got: %s", result)
	}
}

func TestSanitizer_AddPatternInvalid(t *testing.T) {
	t.Parallel()
	sanitizer := NewSanitizer()
	err := sanitizer.AddPattern(`[invalid`)
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestLogger_Creation(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	if logger == nil {
		t.Fatal("expected logger to be created")
	}
	if logger.Logger == nil {
		t.Error("expected underlying slog.Logger to be created")
	}
	if logger.sanitizer == nil {
		t.Error("expected sanitizer to be created")
	}
}

func TestLogger_WithContext(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	taskLogger := logger.WithTask("task-123")
	if taskLogger == nil {
		t.Fatal("expected logger with task to be created")
	}
}

func TestLogger_WithPhase(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	phaseLogger := logger.WithPhase("analyze")
	if phaseLogger == nil {
		t.Fatal("expected logger with phase to be created")
	}
}

func TestLogger_WithWorkflow(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	workflowLogger := logger.WithWorkflow("workflow-456")
	if workflowLogger == nil {
		t.Fatal("expected logger with workflow to be created")
	}
}

func TestLogger_WithAgent(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	agentLogger := logger.WithAgent("claude")
	if agentLogger == nil {
		t.Fatal("expected logger with agent to be created")
	}
}

func TestLogger_Nop(t *testing.T) {
	t.Parallel()
	logger := NewNop()
	if logger == nil {
		t.Fatal("expected nop logger to be created")
	}
	// Should not panic
	logger.Info("test message")
}

func TestLogger_Formats(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"text", "text"},
		{"auto", "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(Config{
				Level:  "info",
				Format: tt.format,
				Output: &buf,
			})
			logger.Info("test message")

			if buf.Len() == 0 {
				t.Error("expected log output")
			}
		})
	}
}

func TestLogger_Levels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		level   string
		logFunc func(l *Logger)
		expect  bool
	}{
		{"debug at debug", "debug", func(l *Logger) { l.Debug("test") }, true},
		{"info at debug", "debug", func(l *Logger) { l.Info("test") }, true},
		{"debug at info", "info", func(l *Logger) { l.Debug("test") }, false},
		{"info at info", "info", func(l *Logger) { l.Info("test") }, true},
		{"warn at error", "error", func(l *Logger) { l.Warn("test") }, false},
		{"error at error", "error", func(l *Logger) { l.Error("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(Config{
				Level:  tt.level,
				Format: "text",
				Output: &buf,
			})
			tt.logFunc(logger)

			hasOutput := buf.Len() > 0
			if hasOutput != tt.expect {
				t.Errorf("expected output=%v, got output=%v", tt.expect, hasOutput)
			}
		})
	}
}

func TestLogger_SanitizesOutput(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "text",
		Output: &buf,
	})

	logger.Info("Processing with API key", "key", "sk-1234567890abcdefghijklmnop")
	output := buf.String()

	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected API key to be sanitized, got: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output, got: %s", output)
	}
}

func TestLogger_SanitizeMethod(t *testing.T) {
	t.Parallel()
	logger := New(DefaultConfig())
	input := "API key: sk-1234567890abcdefghijklmnop"
	result := logger.Sanitize(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected sanitize method to work, got: %s", result)
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"invalid", "INFO"}, // defaults to info
		{"", "INFO"},        // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level.String() != tt.expected {
				t.Errorf("parseLevel(%q) = %s, want %s", tt.input, level.String(), tt.expected)
			}
		})
	}
}

func TestSanitizingHandler_WithGroup(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	})

	grouped := logger.Logger.WithGroup("request")
	grouped.Info("test", "api_key", `api_key="sk-1234567890abcdefghijklmnop"`)

	output := buf.String()
	if strings.Contains(output, "sk-1234567890") {
		t.Errorf("expected API key in group to be sanitized, got: %s", output)
	}
}

func TestPrettyHandler_Levels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		level string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewPrettyHandler(&buf, parseLevel(tt.level))
			logger := New(Config{
				Level:  tt.level,
				Format: "auto",
				Output: &buf,
			})

			// Just ensure no panic
			_ = handler
			logger.Info("test message")
		})
	}
}
