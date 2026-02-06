package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Test Registry extended functionality

func TestRegistry_GetCapabilities(t *testing.T) {
	r := NewRegistry()

	// Register a mock agent with known capabilities
	mockAgent := &mockAgentForTest{
		name: "mock",
		caps: core.Capabilities{
			SupportsJSON:     true,
			MaxContextTokens: 10000,
			DefaultModel:     "mock-model",
		},
	}
	r.Register("mock", mockAgent)

	caps, err := r.GetCapabilities("mock")
	if err != nil {
		t.Fatalf("GetCapabilities() error = %v", err)
	}
	if !caps.SupportsJSON {
		t.Error("expected SupportsJSON to be true")
	}
	if caps.MaxContextTokens != 10000 {
		t.Errorf("MaxContextTokens = %d, want 10000", caps.MaxContextTokens)
	}
}

func TestRegistry_GetCapabilities_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.GetCapabilities("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestRegistry_Ping_NotFound(t *testing.T) {
	r := NewRegistry()
	err := r.Ping(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestRegistry_Ping_Success(t *testing.T) {
	r := NewRegistry()
	mockAgent := &mockAgentForTest{name: "mock", pingErr: nil}
	r.Register("mock", mockAgent)

	err := r.Ping(context.Background(), "mock")
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestRegistry_Ping_Failure(t *testing.T) {
	r := NewRegistry()
	mockAgent := &mockAgentForTest{
		name:    "mock",
		pingErr: core.ErrNotFound("CLI", "mock"),
	}
	r.Register("mock", mockAgent)

	err := r.Ping(context.Background(), "mock")
	if err == nil {
		t.Error("expected error for ping failure")
	}
}

func TestRegistry_PingAll(t *testing.T) {
	r := NewRegistry()

	// Register a successful mock agent
	r.Register("mock1", &mockAgentForTest{name: "mock1", pingErr: nil})
	r.configs["mock1"] = AgentConfig{Name: "mock1", Path: "mock1"}

	// Register a failing mock agent
	r.Register("mock2", &mockAgentForTest{
		name:    "mock2",
		pingErr: core.ErrNotFound("CLI", "mock2"),
	})
	r.configs["mock2"] = AgentConfig{Name: "mock2", Path: "mock2"}

	results := r.PingAll(context.Background())

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results["mock1"] != nil {
		t.Errorf("mock1 ping should succeed, got error: %v", results["mock1"])
	}
	if results["mock2"] == nil {
		t.Error("mock2 ping should fail")
	}
}

func TestRegistry_Available(t *testing.T) {
	r := NewRegistry()

	// Register agents with different ping results
	r.Register("mock1", &mockAgentForTest{name: "mock1", pingErr: nil})
	r.configs["mock1"] = AgentConfig{Name: "mock1", Path: "mock1"}

	r.Register("mock2", &mockAgentForTest{
		name:    "mock2",
		pingErr: core.ErrNotFound("CLI", "mock2"),
	})
	r.configs["mock2"] = AgentConfig{Name: "mock2", Path: "mock2"}

	available := r.Available(context.Background())

	if len(available) != 1 {
		t.Errorf("expected 1 available agent, got %d", len(available))
	}
	if len(available) > 0 && available[0] != "mock1" {
		t.Errorf("expected mock1 to be available, got %s", available[0])
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry()

	mockAgent := &mockAgentForTest{name: "mock"}
	r.Register("mock", mockAgent)

	// Verify agent is cached
	agent, err := r.Get("mock")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent to be returned")
	}

	// Clear and verify cache is empty
	r.Clear()

	r.mu.RLock()
	_, exists := r.agents["mock"]
	r.mu.RUnlock()

	if exists {
		t.Error("expected cache to be cleared")
	}
}

// Test GeminiAdapter specific functions

func TestGeminiAdapter_BuildArgsExtended(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{
		Path:  "gemini",
		Model: "gemini-2.5-flash",
	})
	gemini := adapter.(*GeminiAdapter)

	tests := []struct {
		name string
		opts core.ExecuteOptions
		want []string
	}{
		{
			name: "default args",
			opts: core.ExecuteOptions{},
			want: []string{"--model", "gemini-2.5-flash", "--approval-mode", "yolo"},
		},
		{
			name: "with JSON format",
			// Note: --output-format is now added by streaming config, not buildArgs
			opts: core.ExecuteOptions{Format: core.OutputFormatJSON},
			want: []string{"--model", "gemini-2.5-flash", "--approval-mode", "yolo"},
		},
		{
			name: "with custom model",
			opts: core.ExecuteOptions{Model: "gemini-3-pro-preview"},
			want: []string{"--model", "gemini-3-pro-preview", "--approval-mode", "yolo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gemini.buildArgs(tt.opts)
			if len(got) != len(tt.want) {
				t.Errorf("buildArgs() len = %d, want %d. got: %v", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGeminiAdapter_ExtractUsageTokens(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	gemini := adapter.(*GeminiAdapter)

	tests := []struct {
		name    string
		stdout  string
		stderr  string
		wantIn  int
		wantOut int
	}{
		{
			name:    "with input tokens",
			stdout:  "input_tokens: 100",
			stderr:  "",
			wantIn:  100,
			wantOut: 0,
		},
		{
			name:    "with output tokens",
			stdout:  "output_tokens: 50",
			stderr:  "",
			wantIn:  0,
			wantOut: 50,
		},
		{
			name:    "with both tokens",
			stdout:  "input tokens: 200 output tokens: 100",
			stderr:  "",
			wantIn:  200,
			wantOut: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CommandResult{Stdout: tt.stdout, Stderr: tt.stderr}
			execResult := &core.ExecuteResult{}
			gemini.extractUsage(result, execResult)

			if tt.wantIn > 0 && execResult.TokensIn != tt.wantIn {
				t.Errorf("TokensIn = %d, want %d", execResult.TokensIn, tt.wantIn)
			}
			if tt.wantOut > 0 && execResult.TokensOut != tt.wantOut {
				t.Errorf("TokensOut = %d, want %d", execResult.TokensOut, tt.wantOut)
			}
		})
	}
}

// Note: TestGeminiAdapter_ExtractContentScenarios removed - geminiJSONResponse and extractContent
// were removed as part of the stream-json migration (JSON parsing no longer needed)

func TestGeminiAdapter_ParseOutputFormats(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	gemini := adapter.(*GeminiAdapter)

	// Note: parseOutput no longer parses JSON as part of stream-json migration.
	// The LLM writes output directly to files, so JSON parsing is not needed.
	tests := []struct {
		name   string
		result *CommandResult
		format core.OutputFormat
	}{
		{
			name: "text output",
			result: &CommandResult{
				Stdout:   "plain text response",
				Duration: time.Second,
			},
			format: core.OutputFormatText,
		},
		{
			name: "JSON output",
			result: &CommandResult{
				Stdout:   `{"response": "hello"}`,
				Duration: time.Second,
			},
			format: core.OutputFormatJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gemini.parseOutput(tt.result, tt.format)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			// Verify basic parsing works (returns output and duration)
			if got.Output != tt.result.Stdout {
				t.Errorf("Output = %q, want %q", got.Output, tt.result.Stdout)
			}
			if got.Duration != tt.result.Duration {
				t.Errorf("Duration = %v, want %v", got.Duration, tt.result.Duration)
			}
		})
	}
}

// Test CodexAdapter specific functions

func TestCodexAdapter_BuildArgsExtended(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{
		Path:  "codex",
		Model: "gpt-5.1-codex",
	})
	codex := adapter.(*CodexAdapter)

	// Note: --json is now added by streaming config, not buildArgs
	tests := []struct {
		name     string
		opts     core.ExecuteOptions
		wantExec bool
	}{
		{
			name:     "default args",
			opts:     core.ExecuteOptions{},
			wantExec: true,
		},
		{
			name:     "with JSON format",
			opts:     core.ExecuteOptions{Format: core.OutputFormatJSON},
			wantExec: true, // Still has exec, --json added by streaming
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codex.buildArgs(tt.opts)
			if tt.wantExec && got[0] != "exec" {
				t.Errorf("expected first arg to be 'exec', got %q", got[0])
			}
		})
	}
}

func TestCodexAdapter_ExtractUsageTokens(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	tests := []struct {
		name    string
		stdout  string
		stderr  string
		wantIn  int
		wantOut int
	}{
		{
			name:    "with prompt tokens",
			stdout:  "prompt_tokens: 100",
			stderr:  "",
			wantIn:  100,
			wantOut: 0,
		},
		{
			name:    "with completion tokens",
			stdout:  "completion_tokens: 50",
			stderr:  "",
			wantIn:  0,
			wantOut: 50,
		},
		{
			name:    "with both token types",
			stdout:  "prompt_tokens: 200 completion_tokens: 100",
			stderr:  "",
			wantIn:  200,
			wantOut: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CommandResult{Stdout: tt.stdout, Stderr: tt.stderr}
			execResult := &core.ExecuteResult{}
			codex.extractUsage(result, execResult)

			if tt.wantIn > 0 && execResult.TokensIn != tt.wantIn {
				t.Errorf("TokensIn = %d, want %d", execResult.TokensIn, tt.wantIn)
			}
			if tt.wantOut > 0 && execResult.TokensOut != tt.wantOut {
				t.Errorf("TokensOut = %d, want %d", execResult.TokensOut, tt.wantOut)
			}
		})
	}
}

func TestCodexAdapter_ParseOutput(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	// Note: parseOutput no longer parses JSON as part of stream-json migration.
	// The LLM writes output directly to files, so JSON parsing is not needed.
	result := &CommandResult{
		Stdout:   `{"code": "print('hello')"}`,
		Duration: time.Second,
	}

	got, err := codex.parseOutput(result, core.OutputFormatJSON)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}
	// Verify basic parsing works
	if got.Output != result.Stdout {
		t.Errorf("Output = %q, want %q", got.Output, result.Stdout)
	}
}

// Test ClaudeAdapter specific functions not covered elsewhere

func TestClaudeAdapter_ExtractUsagePatterns(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	tests := []struct {
		name       string
		stdout     string
		stderr     string
		wantIn     int
		wantOut    int
		hasNumbers bool
	}{
		{
			name:       "with token info",
			stdout:     "tokens: 100 in, 50 out",
			stderr:     "",
			wantIn:     100,
			wantOut:    50,
			hasNumbers: true,
		},
		{
			name:       "with cost info",
			stdout:     "cost: $0.05",
			stderr:     "",
			hasNumbers: false,
		},
		{
			name:       "no usage info",
			stdout:     "just some output",
			stderr:     "",
			hasNumbers: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CommandResult{Stdout: tt.stdout, Stderr: tt.stderr}
			execResult := &core.ExecuteResult{}
			claude.extractUsage(result, execResult)

			if tt.hasNumbers {
				if execResult.TokensIn != tt.wantIn {
					t.Errorf("TokensIn = %d, want %d", execResult.TokensIn, tt.wantIn)
				}
				if execResult.TokensOut != tt.wantOut {
					t.Errorf("TokensOut = %d, want %d", execResult.TokensOut, tt.wantOut)
				}
			}
		})
	}
}

func TestClaudeAdapter_ParseOutputFormats(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	// Note: parseOutput no longer parses JSON as part of stream-json migration.
	// The LLM writes output directly to files, so JSON parsing is not needed.
	tests := []struct {
		name   string
		result *CommandResult
		format core.OutputFormat
	}{
		{
			name: "text output",
			result: &CommandResult{
				Stdout:   "plain text response",
				Duration: time.Second,
			},
			format: core.OutputFormatText,
		},
		{
			name: "JSON output",
			result: &CommandResult{
				Stdout:   `{"response": "hello"}`,
				Duration: time.Second,
			},
			format: core.OutputFormatJSON,
		},
		{
			name: "invalid JSON output",
			result: &CommandResult{
				Stdout:   `not valid json`,
				Duration: time.Second,
			},
			format: core.OutputFormatJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := claude.parseOutput(tt.result, tt.format)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			// Verify basic parsing works - output should be returned as-is
			if got.Output != tt.result.Stdout {
				t.Errorf("Output = %q, want %q", got.Output, tt.result.Stdout)
			}
			if got.Duration != tt.result.Duration {
				t.Errorf("Duration = %v, want %v", got.Duration, tt.result.Duration)
			}
		})
	}
}

// Test CopilotAdapter specific functions not covered elsewhere

func TestCopilotAdapter_CleanANSIExtended(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	tests := []struct {
		input string
		want  string
	}{
		{
			input: "normal text",
			want:  "normal text",
		},
		{
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			input: "\x1b[1;32mbold green\x1b[0m",
			want:  "bold green",
		},
		{
			input: "\x1b[0G\x1b[2K? text",
			want:  "? text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := copilot.cleanANSI(tt.input)
			if got != tt.want {
				t.Errorf("cleanANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopilotAdapter_EstimateTokensShort(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	got := copilot.estimateTokens("hello world")
	want := 2 // 11 chars / 4 = 2
	if got != want {
		t.Errorf("estimateTokens() = %d, want %d", got, want)
	}
}

func TestCopilotAdapter_BuildArgsYOLO(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	args := copilot.buildArgs(core.ExecuteOptions{})

	// Check YOLO mode flags are present
	hasAllowAllTools := false
	hasAllowAllPaths := false
	hasAllowAllUrls := false

	for _, arg := range args {
		switch arg {
		case "--allow-all-tools":
			hasAllowAllTools = true
		case "--allow-all-paths":
			hasAllowAllPaths = true
		case "--allow-all-urls":
			hasAllowAllUrls = true
		}
	}

	if !hasAllowAllTools {
		t.Error("expected --allow-all-tools flag in args")
	}
	if !hasAllowAllPaths {
		t.Error("expected --allow-all-paths flag in args")
	}
	if !hasAllowAllUrls {
		t.Error("expected --allow-all-urls flag in args")
	}
}

// Test default config extended

func TestDefaultConfigAllAdapters(t *testing.T) {
	tests := []struct {
		name        string
		wantPath    string
		wantTimeout time.Duration
	}{
		{
			name:        "claude",
			wantPath:    "claude",
			wantTimeout: 0, // Defer to phase timeout
		},
		{
			name:        "gemini",
			wantPath:    "gemini",
			wantTimeout: 0, // Defer to phase timeout
		},
		{
			name:        "copilot",
			wantPath:    "copilot",
			wantTimeout: 0, // Defer to phase timeout
		},
		{
			name:        "codex",
			wantPath:    "codex",
			wantTimeout: 0, // Defer to phase timeout
		},
		{
			name:        "unknown",
			wantPath:    "",
			wantTimeout: 0, // Defer to phase timeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig(tt.name)
			if cfg.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", cfg.Path, tt.wantPath)
			}
			if cfg.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %v, want %v", cfg.Timeout, tt.wantTimeout)
			}
		})
	}
}

// Test adapter factories with default paths

func TestNewClaudeAdapter_DefaultPath(t *testing.T) {
	adapter, err := NewClaudeAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter() error = %v", err)
	}
	claude := adapter.(*ClaudeAdapter)
	if claude.config.Path != "claude" {
		t.Errorf("default Path = %q, want %q", claude.config.Path, "claude")
	}
}

func TestNewGeminiAdapter_Capabilities(t *testing.T) {
	adapter, err := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	if err != nil {
		t.Fatalf("NewGeminiAdapter() error = %v", err)
	}
	if adapter.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "gemini")
	}
	caps := adapter.Capabilities()
	if caps.MaxContextTokens != 1000000 {
		t.Errorf("MaxContextTokens = %d, want 1000000", caps.MaxContextTokens)
	}
}

func TestNewCopilotAdapter_Capabilities(t *testing.T) {
	adapter, err := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	if err != nil {
		t.Fatalf("NewCopilotAdapter() error = %v", err)
	}
	if adapter.Name() != "copilot" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "copilot")
	}
	caps := adapter.Capabilities()
	if caps.SupportsJSON {
		t.Error("expected SupportsJSON to be false for copilot")
	}
	if !caps.SupportsTools {
		t.Error("expected SupportsTools to be true for copilot")
	}
	if caps.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
}

func TestNewCodexAdapter_Capabilities(t *testing.T) {
	adapter, err := NewCodexAdapter(AgentConfig{Path: "codex"})
	if err != nil {
		t.Fatalf("NewCodexAdapter() error = %v", err)
	}
	if adapter.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "codex")
	}
}

// Mock agent for testing

type mockAgentForTest struct {
	name    string
	caps    core.Capabilities
	pingErr error
	execErr error
	result  *core.ExecuteResult
}

func (m *mockAgentForTest) Name() string {
	return m.name
}

func (m *mockAgentForTest) Capabilities() core.Capabilities {
	return m.caps
}

func (m *mockAgentForTest) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockAgentForTest) Execute(_ context.Context, _ core.ExecuteOptions) (*core.ExecuteResult, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return m.result, nil
}
