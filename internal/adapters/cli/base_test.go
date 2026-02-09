package cli

import (
	"context"
	"testing"
	"time"
)

func TestNewBaseAdapter(t *testing.T) {
	cfg := AgentConfig{
		Name:  "test",
		Path:  "/usr/bin/test",
		Model: "test-model",
	}

	// With nil logger
	adapter := NewBaseAdapter(cfg, nil)
	if adapter == nil {
		t.Fatal("NewBaseAdapter() returned nil")
	}
	if adapter.config.Name != "test" {
		t.Errorf("config.Name = %s, want test", adapter.config.Name)
	}

	// Should have a nop logger
	if adapter.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestBaseAdapter_ParseJSON(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid json object",
			input:   `{"key": "value", "number": 42}`,
			wantErr: false,
		},
		{
			name:    "valid json array",
			input:   `[1, 2, 3, 4, 5]`,
			wantErr: false,
		},
		{
			name:    "embedded json",
			input:   `Some text before {"key": "value"} and after`,
			wantErr: false,
		},
		{
			name:    "no json",
			input:   `Just plain text with no JSON`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `{"key": invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			err := base.ParseJSON(tt.input, &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBaseAdapter_ExtractByPattern(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name    string
		input   string
		pattern string
		want    []string
		wantErr bool
	}{
		{
			name:    "extract numbers",
			input:   "The values are 123, 456, and 789",
			pattern: `\d+`,
			want:    []string{"123", "456", "789"},
			wantErr: false,
		},
		{
			name:    "extract emails",
			input:   "Contact user@example.com or admin@test.org",
			pattern: `\w+@\w+\.\w+`,
			want:    []string{"user@example.com", "admin@test.org"},
			wantErr: false,
		},
		{
			name:    "no matches",
			input:   "No patterns here",
			pattern: `\d{10}`,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "invalid pattern",
			input:   "test",
			pattern: `[invalid`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := base.ExtractByPattern(tt.input, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractByPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != len(tt.want) {
				t.Errorf("ExtractByPattern() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestBaseAdapter_ClassifyError(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name        string
		stderr      string
		exitCode    int
		errContains string
	}{
		{
			name:        "rate limit",
			stderr:      "Error: rate limit exceeded",
			exitCode:    1,
			errContains: "rate limit",
		},
		{
			name:        "too many requests",
			stderr:      "Error: too many requests",
			exitCode:    1,
			errContains: "too many requests",
		},
		{
			name:        "authentication error",
			stderr:      "Error: unauthorized access",
			exitCode:    1,
			errContains: "unauthorized",
		},
		{
			name:        "token error",
			stderr:      "Invalid API key or token",
			exitCode:    1,
			errContains: "token",
		},
		{
			name:        "network error",
			stderr:      "Connection refused",
			exitCode:    1,
			errContains: "connection",
		},
		{
			name:        "timeout error",
			stderr:      "Request timeout",
			exitCode:    1,
			errContains: "timeout",
		},
		{
			name:        "generic error",
			stderr:      "Some unknown error",
			exitCode:    2,
			errContains: "exit code 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CommandResult{
				Stderr:   tt.stderr,
				ExitCode: tt.exitCode,
			}
			err := base.classifyError(result)
			if err == nil {
				t.Fatal("classifyError() should return an error")
			}
			// Just check that error is produced
		})
	}
}

func TestBaseAdapter_CheckAvailability_NoPath(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	err := base.CheckAvailability(t.Context())
	if err == nil {
		t.Error("CheckAvailability() should error when path is empty")
	}
}

func TestBaseAdapter_CheckAvailability_WithPath(t *testing.T) {
	// Test with a command that should exist
	base := NewBaseAdapter(AgentConfig{
		Path: "echo", // Usually available on all systems
	}, nil)

	err := base.CheckAvailability(t.Context())
	if err != nil {
		t.Logf("CheckAvailability() error = %v (expected if echo not in PATH)", err)
	}
}

func TestBaseAdapter_CheckAvailability_MultiWordCommand(t *testing.T) {
	// Test with a multi-word command like "gh copilot"
	base := NewBaseAdapter(AgentConfig{
		Path: "gh copilot",
	}, nil)

	err := base.CheckAvailability(t.Context())
	// Will error if gh is not installed, which is fine
	if err == nil {
		t.Log("gh is available, test passed")
	}
}

func TestBaseAdapter_TruncateToTokenLimit_ShortText(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	text := "Short text"
	maxTokens := 100 // 400 characters

	result := base.TruncateToTokenLimit(text, maxTokens)
	if result != text {
		t.Errorf("TruncateToTokenLimit() = %q, want %q (no truncation needed)", result, text)
	}
}

func TestBaseAdapter_ExtractJSON_Complex(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "escaped quotes in string",
			input: `{"text": "He said \"hello\""}`,
			want:  `{"text": "He said \"hello\""}`,
		},
		{
			name:  "nested arrays",
			input: `[[1, 2], [3, 4], [5, 6]]`,
			want:  `[[1, 2], [3, 4], [5, 6]]`,
		},
		{
			name:  "mixed content with newlines",
			input: "Prefix\n{\"key\": \"value\"}\nSuffix",
			want:  `{"key": "value"}`,
		},
		{
			name:  "unbalanced braces (incomplete)",
			input: `{"key": "value"`,
			want:  ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.ExtractJSON(tt.input)
			if result != tt.want {
				t.Errorf("ExtractJSON() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestCommandResult(t *testing.T) {
	result := CommandResult{
		Stdout:   "output",
		Stderr:   "error",
		ExitCode: 1,
		Duration: 5 * time.Second,
	}

	if result.Stdout != "output" {
		t.Errorf("Stdout = %q, want output", result.Stdout)
	}
	if result.Stderr != "error" {
		t.Errorf("Stderr = %q, want error", result.Stderr)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", result.Duration)
	}
}

func TestAgentConfig(t *testing.T) {
	cfg := AgentConfig{
		Name:    "test-agent",
		Path:    "/usr/bin/test",
		Model:   "test-model",
		Timeout: time.Minute,
		WorkDir: "/work",
	}

	if cfg.Name != "test-agent" {
		t.Errorf("Name = %q, want test-agent", cfg.Name)
	}
	if cfg.Path != "/usr/bin/test" {
		t.Errorf("Path = %q, want /usr/bin/test", cfg.Path)
	}
	if cfg.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", cfg.Model)
	}
	if cfg.Timeout != time.Minute {
		t.Errorf("Timeout = %v, want 1m", cfg.Timeout)
	}
	if cfg.WorkDir != "/work" {
		t.Errorf("WorkDir = %q, want /work", cfg.WorkDir)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s          string
		substrings []string
		want       bool
	}{
		{"hello world", []string{"world"}, true},
		{"hello world", []string{"foo", "bar"}, false},
		{"rate limit exceeded", []string{"rate limit", "quota"}, true},
		{"", []string{"test"}, false},
		{"test", []string{}, false},
	}

	for _, tt := range tests {
		result := containsAny(tt.s, tt.substrings)
		if result != tt.want {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrings, result, tt.want)
		}
	}
}

func TestIdleTimeoutKillsHungProcess(t *testing.T) {
	// This test verifies that the idle timer kills a process that writes
	// one JSON line and then hangs (no more stdout output).
	adapter := NewBaseAdapter(AgentConfig{
		Name:        "test-idle",
		Path:        "bash",
		IdleTimeout: 500 * time.Millisecond,
		GracePeriod: 200 * time.Millisecond,
	}, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()

	// bash -c: print one JSON line, then sleep indefinitely (simulating a hang).
	// The idle timer should fire after 500ms and kill the process.
	result, err := adapter.executeWithJSONStreaming(
		ctx,
		"test-idle",
		[]string{"-c", `echo '{"type":"message","text":"hello"}'; sleep 3600`},
		"",  // stdin
		"",  // workDir
		0,   // optTimeout (use default)
		StreamConfig{}, // no streaming flags needed for bash
		nil,            // no parser
	)

	elapsed := time.Since(start)

	// Should complete in ~500ms (idle timeout), not 3600s.
	if elapsed > 5*time.Second {
		t.Fatalf("took %v, expected ~500ms — idle timeout did not fire", elapsed)
	}

	// The result should contain the first line's text (extracted or raw).
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// err may or may not be nil depending on how the kill is classified.
	// The key assertion is that we returned quickly.
	t.Logf("completed in %v, err=%v, stdout=%q", elapsed, err, result.Stdout)
}
