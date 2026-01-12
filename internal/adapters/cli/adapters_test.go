package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// Registry Tests
// =============================================================================

func TestRegistry_NewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	// Should have built-in factories
	if !r.Has("claude") {
		t.Error("registry should have claude factory")
	}
	if !r.Has("gemini") {
		t.Error("registry should have gemini factory")
	}
	if !r.Has("codex") {
		t.Error("registry should have codex factory")
	}
	if !r.Has("copilot") {
		t.Error("registry should have copilot factory")
	}
	if !r.Has("aider") {
		t.Error("registry should have aider factory")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	list := r.List()

	if len(list) != 5 {
		t.Errorf("List() returned %d items, want 5", len(list))
	}

	// Check all expected adapters are present
	expected := map[string]bool{
		"claude": true, "gemini": true, "codex": true,
		"copilot": true, "aider": true,
	}

	for _, name := range list {
		if !expected[name] {
			t.Errorf("unexpected adapter: %s", name)
		}
		delete(expected, name)
	}

	if len(expected) > 0 {
		t.Errorf("missing adapters: %v", expected)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	agent, err := r.Get("claude")
	if err != nil {
		t.Fatalf("Get(claude) error = %v", err)
	}
	if agent == nil {
		t.Fatal("Get(claude) returned nil agent")
	}
	if agent.Name() != "claude" {
		t.Errorf("agent.Name() = %s, want claude", agent.Name())
	}
}

func TestRegistry_Get_Unknown(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("unknown")
	if err == nil {
		t.Error("Get(unknown) should return error")
	}
}

func TestRegistry_Get_Caching(t *testing.T) {
	r := NewRegistry()

	agent1, _ := r.Get("claude")
	agent2, _ := r.Get("claude")

	if agent1 != agent2 {
		t.Error("Get should return same cached agent")
	}
}

func TestRegistry_Configure(t *testing.T) {
	r := NewRegistry()

	// Get agent first
	agent1, _ := r.Get("claude")

	// Configure with new settings
	r.Configure("claude", AgentConfig{
		Name:  "claude",
		Path:  "/custom/path/claude",
		Model: "custom-model",
	})

	// Get agent again - should be new instance
	agent2, _ := r.Get("claude")

	if agent1 == agent2 {
		t.Error("Configure should invalidate cached agent")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	mockAgent := &mockTestAgent{name: "mock"}
	err := r.Register("mock", mockAgent)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Should be able to get it
	agent, err := r.Get("mock")
	if err != nil {
		t.Fatalf("Get(mock) error = %v", err)
	}
	if agent.Name() != "mock" {
		t.Errorf("agent.Name() = %s, want mock", agent.Name())
	}
}

func TestRegistry_ListEnabled(t *testing.T) {
	r := NewRegistry()

	// Initially no enabled (configured) agents
	enabled := r.ListEnabled()
	if len(enabled) != 0 {
		t.Errorf("ListEnabled() = %d, want 0 before configuration", len(enabled))
	}

	// Configure some agents
	r.Configure("claude", AgentConfig{Name: "claude"})
	r.Configure("gemini", AgentConfig{Name: "gemini"})

	enabled = r.ListEnabled()
	if len(enabled) != 2 {
		t.Errorf("ListEnabled() = %d, want 2", len(enabled))
	}
}

// =============================================================================
// Claude Adapter Tests
// =============================================================================

func TestClaudeAdapter_Name(t *testing.T) {
	adapter, err := NewClaudeAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter() error = %v", err)
	}

	if adapter.Name() != "claude" {
		t.Errorf("Name() = %s, want claude", adapter.Name())
	}
}

func TestClaudeAdapter_Capabilities(t *testing.T) {
	adapter, err := NewClaudeAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter() error = %v", err)
	}

	caps := adapter.Capabilities()

	if !caps.SupportsJSON {
		t.Error("should support JSON")
	}
	if !caps.SupportsStreaming {
		t.Error("should support streaming")
	}
	if !caps.SupportsImages {
		t.Error("should support images")
	}
	if caps.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
	if len(caps.SupportedModels) == 0 {
		t.Error("should have supported models")
	}
}

func TestClaudeAdapter_BuildArgs(t *testing.T) {
	cfg := AgentConfig{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
	}
	adapter, _ := NewClaudeAdapter(cfg)
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Format:  core.OutputFormatJSON,
		WorkDir: "/test/dir",
	}

	args := claude.buildArgs(opts)

	// Check required args
	if !containsString(args, "--print") {
		t.Error("should include --print")
	}
	if !containsString(args, "--dangerously-skip-permissions") {
		t.Error("should include --dangerously-skip-permissions")
	}
	if !containsString(args, "--output-format") {
		t.Error("should include --output-format for JSON")
	}
	if !containsString(args, "--cwd") {
		t.Error("should include --cwd for work dir")
	}
}

func TestClaudeAdapter_EstimateCost(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{})
	claude := adapter.(*ClaudeAdapter)

	// 1M input tokens = $3, 1M output tokens = $15
	cost := claude.estimateCost(1000000, 1000000)

	expected := 18.0 // $3 + $15
	if cost != expected {
		t.Errorf("estimateCost(1M, 1M) = %v, want %v", cost, expected)
	}
}

func TestClaudeAdapter_ExtractUsage(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{})
	claude := adapter.(*ClaudeAdapter)

	result := &CommandResult{
		Stdout: "Some output",
		Stderr: "tokens: 100 in, 50 out. cost: $0.01",
	}
	execResult := &core.ExecuteResult{}

	claude.extractUsage(result, execResult)

	if execResult.TokensIn != 100 {
		t.Errorf("TokensIn = %d, want 100", execResult.TokensIn)
	}
	if execResult.TokensOut != 50 {
		t.Errorf("TokensOut = %d, want 50", execResult.TokensOut)
	}
	if execResult.CostUSD != 0.01 {
		t.Errorf("CostUSD = %v, want 0.01", execResult.CostUSD)
	}
}

// =============================================================================
// Gemini Adapter Tests
// =============================================================================

func TestGeminiAdapter_Name(t *testing.T) {
	adapter, err := NewGeminiAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAdapter() error = %v", err)
	}

	if adapter.Name() != "gemini" {
		t.Errorf("Name() = %s, want gemini", adapter.Name())
	}
}

func TestGeminiAdapter_Capabilities(t *testing.T) {
	adapter, err := NewGeminiAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAdapter() error = %v", err)
	}

	caps := adapter.Capabilities()

	if !caps.SupportsJSON {
		t.Error("should support JSON")
	}
	if caps.MaxContextTokens != 1000000 {
		t.Errorf("MaxContextTokens = %d, want 1000000", caps.MaxContextTokens)
	}
}

func TestGeminiAdapter_BuildArgs(t *testing.T) {
	cfg := AgentConfig{Model: "gemini-2.0-flash"}
	adapter, _ := NewGeminiAdapter(cfg)
	gemini := adapter.(*GeminiAdapter)

	opts := core.ExecuteOptions{
		Format: core.OutputFormatJSON,
	}

	args := gemini.buildArgs(opts)

	if !containsString(args, "--model") {
		t.Error("should include --model")
	}
	if !containsString(args, "--json") {
		t.Error("should include --json for JSON format")
	}
	if !containsString(args, "--non-interactive") {
		t.Error("should include --non-interactive")
	}
	if !containsString(args, "--prompt") {
		t.Error("should include --prompt")
	}
}

func TestGeminiAdapter_EstimateCost(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{})
	gemini := adapter.(*GeminiAdapter)

	// 1M input = $0.075, 1M output = $0.30
	cost := gemini.estimateCost(1000000, 1000000)

	expected := 0.375
	if cost != expected {
		t.Errorf("estimateCost(1M, 1M) = %v, want %v", cost, expected)
	}
}

func TestGeminiAdapter_ExtractContent(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{})
	gemini := adapter.(*GeminiAdapter)

	resp := &geminiJSONResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		}{
			{
				Content: struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}{
					Parts: []struct {
						Text string `json:"text"`
					}{
						{Text: "Hello"},
						{Text: "World"},
					},
				},
			},
		},
	}

	content := gemini.extractContent(resp)
	if content != "Hello\nWorld" {
		t.Errorf("extractContent() = %q, want %q", content, "Hello\nWorld")
	}
}

// =============================================================================
// Codex Adapter Tests
// =============================================================================

func TestCodexAdapter_Name(t *testing.T) {
	adapter, err := NewCodexAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCodexAdapter() error = %v", err)
	}

	if adapter.Name() != "codex" {
		t.Errorf("Name() = %s, want codex", adapter.Name())
	}
}

func TestCodexAdapter_Capabilities(t *testing.T) {
	adapter, err := NewCodexAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCodexAdapter() error = %v", err)
	}

	caps := adapter.Capabilities()

	if !caps.SupportsJSON {
		t.Error("should support JSON")
	}
	if caps.SupportsImages {
		t.Error("should not support images")
	}
	if caps.MaxContextTokens != 128000 {
		t.Errorf("MaxContextTokens = %d, want 128000", caps.MaxContextTokens)
	}
}

func TestCodexAdapter_BuildArgs(t *testing.T) {
	cfg := AgentConfig{
		Model:       "gpt-4o",
		MaxTokens:   4096,
		Temperature: 0.7,
	}
	adapter, _ := NewCodexAdapter(cfg)
	codex := adapter.(*CodexAdapter)

	opts := core.ExecuteOptions{
		Format: core.OutputFormatJSON,
	}

	args := codex.buildArgs(opts)

	if !containsString(args, "--model") {
		t.Error("should include --model")
	}
	if !containsString(args, "--max-tokens") {
		t.Error("should include --max-tokens")
	}
	if !containsString(args, "--temperature") {
		t.Error("should include --temperature")
	}
	if !containsString(args, "--json") {
		t.Error("should include --json for JSON format")
	}
	if !containsString(args, "--full-auto") {
		t.Error("should include --full-auto")
	}
}

func TestCodexAdapter_EstimateCost(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{})
	codex := adapter.(*CodexAdapter)

	// 1M input = $2.50, 1M output = $10.00
	cost := codex.estimateCost(1000000, 1000000)

	expected := 12.50
	if cost != expected {
		t.Errorf("estimateCost(1M, 1M) = %v, want %v", cost, expected)
	}
}

// =============================================================================
// Copilot Adapter Tests
// =============================================================================

func TestCopilotAdapter_Name(t *testing.T) {
	adapter, err := NewCopilotAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCopilotAdapter() error = %v", err)
	}

	if adapter.Name() != "copilot" {
		t.Errorf("Name() = %s, want copilot", adapter.Name())
	}
}

func TestCopilotAdapter_Capabilities(t *testing.T) {
	adapter, err := NewCopilotAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCopilotAdapter() error = %v", err)
	}

	caps := adapter.Capabilities()

	if caps.SupportsJSON {
		t.Error("copilot should not support JSON")
	}
	if caps.SupportsImages {
		t.Error("copilot should not support images")
	}
	if caps.MaxContextTokens != 8000 {
		t.Errorf("MaxContextTokens = %d, want 8000", caps.MaxContextTokens)
	}
}

func TestCopilotAdapter_CleanANSI(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{})
	copilot := adapter.(*CopilotAdapter)

	input := "\x1b[32mGreen Text\x1b[0m and \x1b[1;34mBold Blue\x1b[0m"
	expected := "Green Text and Bold Blue"

	result := copilot.cleanANSI(input)
	if result != expected {
		t.Errorf("cleanANSI() = %q, want %q", result, expected)
	}
}

func TestCopilotAdapter_ExtractSuggestion(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{})
	copilot := adapter.(*CopilotAdapter)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with suggestion marker",
			input: "Some intro\nSuggestion:\nls -la\nrm -rf /tmp\n? Continue?",
			want:  "ls -la\nrm -rf /tmp",
		},
		{
			name:  "no marker",
			input: "Direct output text",
			want:  "Direct output text",
		},
		{
			name:  "shell prompt end",
			input: "Suggestion:\ngit status\n$ ",
			want:  "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := copilot.extractSuggestion(tt.input)
			if result != tt.want {
				t.Errorf("extractSuggestion() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestCopilotAdapter_IsOutputComplete(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{})
	copilot := adapter.(*CopilotAdapter)

	// Note: The function uses TrimSpace before checking suffixes
	// So trailing whitespace is removed before the suffix match
	// "$ " marker won't match because TrimSpace removes the trailing space
	// Only "Suggestion:" (no trailing space) can match after trimming
	tests := []struct {
		input string
		want  bool
	}{
		{"Some text $", false},               // $ without space
		{"Some text with Suggestion:", true}, // Ends with Suggestion:
		{"Interactive ?", false},             // ? without space
		{"Still waiting...", false},          // No markers
		{"Another Suggestion:", true},        // Ends with Suggestion:
		{"  Suggestion:  ", true},            // After trim ends with Suggestion:
	}

	for _, tt := range tests {
		result := copilot.isOutputComplete(tt.input)
		if result != tt.want {
			t.Errorf("isOutputComplete(%q) = %v, want %v", tt.input, result, tt.want)
		}
	}
}

// =============================================================================
// Aider Adapter Tests
// =============================================================================

func TestAiderAdapter_Name(t *testing.T) {
	adapter, err := NewAiderAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewAiderAdapter() error = %v", err)
	}

	if adapter.Name() != "aider" {
		t.Errorf("Name() = %s, want aider", adapter.Name())
	}
}

func TestAiderAdapter_Capabilities(t *testing.T) {
	adapter, err := NewAiderAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewAiderAdapter() error = %v", err)
	}

	caps := adapter.Capabilities()

	if caps.SupportsJSON {
		t.Error("aider should not support JSON")
	}
	if caps.MaxContextTokens != 128000 {
		t.Errorf("MaxContextTokens = %d, want 128000", caps.MaxContextTokens)
	}
}

func TestAiderAdapter_BuildArgs_Claude(t *testing.T) {
	cfg := AgentConfig{Model: "claude-3-5-sonnet-20241022"}
	adapter, _ := NewAiderAdapter(cfg)
	aider := adapter.(*AiderAdapter)

	opts := core.ExecuteOptions{}
	args := aider.buildArgs(opts)

	if !containsString(args, "--sonnet") {
		t.Error("should include --sonnet for Claude Sonnet")
	}
	if !containsString(args, "--no-git") {
		t.Error("should include --no-git")
	}
	if !containsString(args, "--no-auto-commits") {
		t.Error("should include --no-auto-commits")
	}
	if !containsString(args, "--yes") {
		t.Error("should include --yes")
	}
	if !containsString(args, "--message") {
		t.Error("should include --message")
	}
}

func TestAiderAdapter_BuildArgs_GPT(t *testing.T) {
	cfg := AgentConfig{Model: "gpt-4o"}
	adapter, _ := NewAiderAdapter(cfg)
	aider := adapter.(*AiderAdapter)

	opts := core.ExecuteOptions{}
	args := aider.buildArgs(opts)

	if !containsString(args, "--model") {
		t.Error("should include --model for GPT models")
	}
}

func TestAiderAdapter_BuildArgs_Opus(t *testing.T) {
	cfg := AgentConfig{Model: "claude-3-opus-20240229"}
	adapter, _ := NewAiderAdapter(cfg)
	aider := adapter.(*AiderAdapter)

	opts := core.ExecuteOptions{}
	args := aider.buildArgs(opts)

	if !containsString(args, "--opus") {
		t.Error("should include --opus for Claude Opus")
	}
}

func TestAiderAdapter_CleanOutput(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{})
	aider := adapter.(*AiderAdapter)

	input := "⠋ Loading...\x1b[32m[Done]\x1b[0m Result here"
	result := aider.cleanOutput(input)

	if containsString([]string{result}, "⠋") {
		t.Error("should remove spinner characters")
	}
	if containsString([]string{result}, "\x1b") {
		t.Error("should remove ANSI codes")
	}
}

func TestAiderAdapter_ExtractUsage(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{})
	aider := adapter.(*AiderAdapter)

	result := &CommandResult{
		Stdout: "Response text",
		Stderr: "Tokens: 500 sent, 200 received. Cost: $0.05",
	}
	execResult := &core.ExecuteResult{}

	aider.extractUsage(result, execResult)

	if execResult.TokensIn != 500 {
		t.Errorf("TokensIn = %d, want 500", execResult.TokensIn)
	}
	if execResult.TokensOut != 200 {
		t.Errorf("TokensOut = %d, want 200", execResult.TokensOut)
	}
	if execResult.CostUSD != 0.05 {
		t.Errorf("CostUSD = %v, want 0.05", execResult.CostUSD)
	}
}

func TestAiderAdapter_WithEditFormat(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{})
	aider := adapter.(*AiderAdapter)

	args := aider.WithEditFormat("diff")
	if len(args) != 2 || args[0] != "--edit-format" || args[1] != "diff" {
		t.Errorf("WithEditFormat(diff) = %v, want [--edit-format diff]", args)
	}

	args = aider.WithEditFormat("")
	if args[1] != "whole" {
		t.Errorf("WithEditFormat('') should default to 'whole', got %s", args[1])
	}
}

// =============================================================================
// Base Adapter Tests
// =============================================================================

func TestBaseAdapter_TokenEstimate(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	// ~4 chars per token
	text := "This is a test string with about 40 chars"
	estimate := base.TokenEstimate(text)

	expected := len(text) / 4
	if estimate != expected {
		t.Errorf("TokenEstimate() = %d, want %d", estimate, expected)
	}
}

func TestBaseAdapter_TruncateToTokenLimit(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	text := "This is a very long text that should be truncated to fit within the token limit"
	maxTokens := 5 // 20 characters

	result := base.TruncateToTokenLimit(text, maxTokens)

	if len(result) > 20+len("\n...[truncated]") {
		t.Errorf("TruncateToTokenLimit() too long: %d chars", len(result))
	}
}

func TestBaseAdapter_ExtractJSON(t *testing.T) {
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple object",
			input: `Some text {"key": "value"} more text`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "nested object",
			input: `{"outer": {"inner": 123}}`,
			want:  `{"outer": {"inner": 123}}`,
		},
		{
			name:  "array",
			input: `Results: [1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "no json",
			input: `Just plain text`,
			want:  ``,
		},
		{
			name:  "with strings containing braces",
			input: `{"message": "Use { and } carefully"}`,
			want:  `{"message": "Use { and } carefully"}`,
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

func TestBaseAdapter_Config(t *testing.T) {
	cfg := AgentConfig{
		Name:        "test",
		Path:        "/usr/bin/test",
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
		Timeout:     time.Minute,
	}

	base := NewBaseAdapter(cfg, nil)
	result := base.Config()

	if result.Name != cfg.Name {
		t.Errorf("Config().Name = %s, want %s", result.Name, cfg.Name)
	}
	if result.Path != cfg.Path {
		t.Errorf("Config().Path = %s, want %s", result.Path, cfg.Path)
	}
	if result.Model != cfg.Model {
		t.Errorf("Config().Model = %s, want %s", result.Model, cfg.Model)
	}
}

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name     string
		wantPath string
	}{
		{"claude", "claude"},
		{"gemini", "gemini"},
		{"codex", "codex"},
		{"copilot", "gh copilot"},
		{"aider", "aider"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig(tt.name)
			if cfg.Path != tt.wantPath {
				t.Errorf("defaultConfig(%s).Path = %s, want %s", tt.name, cfg.Path, tt.wantPath)
			}
		})
	}
}

// =============================================================================
// Helper Types
// =============================================================================

type mockTestAgent struct {
	name string
}

func (m *mockTestAgent) Name() string {
	return m.name
}

func (m *mockTestAgent) Capabilities() core.Capabilities {
	return core.Capabilities{}
}

func (m *mockTestAgent) Ping(ctx context.Context) error {
	return nil
}

func (m *mockTestAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	return &core.ExecuteResult{Output: "mock output"}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

var _ core.Agent = (*mockTestAgent)(nil)
