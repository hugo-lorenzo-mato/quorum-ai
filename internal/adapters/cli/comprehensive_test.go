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
			opts: core.ExecuteOptions{Format: core.OutputFormatJSON},
			want: []string{"--model", "gemini-2.5-flash", "--output-format", "json", "--approval-mode", "yolo"},
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

func TestGeminiAdapter_ExtractContentScenarios(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	gemini := adapter.(*GeminiAdapter)

	tests := []struct {
		name string
		resp *geminiJSONResponse
		want string
	}{
		{
			name: "empty response",
			resp: &geminiJSONResponse{},
			want: "",
		},
		{
			name: "single part",
			resp: &geminiJSONResponse{
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
								{Text: "Hello world"},
							},
						},
					},
				},
			},
			want: "Hello world",
		},
		{
			name: "multiple parts",
			resp: &geminiJSONResponse{
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
								{Text: "Part 1"},
								{Text: "Part 2"},
							},
						},
					},
				},
			},
			want: "Part 1\nPart 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gemini.extractContent(tt.resp)
			if got != tt.want {
				t.Errorf("extractContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeminiAdapter_EstimateCostCalc(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	gemini := adapter.(*GeminiAdapter)

	cost := gemini.estimateCost(1000000, 1000000)
	// Input: 1M tokens * $0.075/MTok = $0.075
	// Output: 1M tokens * $0.30/MTok = $0.30
	// Total: $0.375
	if cost < 0.35 || cost > 0.40 {
		t.Errorf("estimateCost(1M, 1M) = %f, expected ~0.375", cost)
	}
}

func TestGeminiAdapter_ParseOutputFormats(t *testing.T) {
	adapter, _ := NewGeminiAdapter(AgentConfig{Path: "gemini"})
	gemini := adapter.(*GeminiAdapter)

	tests := []struct {
		name       string
		result     *CommandResult
		format     core.OutputFormat
		wantParsed bool
	}{
		{
			name: "text output",
			result: &CommandResult{
				Stdout:   "plain text response",
				Duration: time.Second,
			},
			format:     core.OutputFormatText,
			wantParsed: false,
		},
		{
			name: "JSON output",
			result: &CommandResult{
				Stdout:   `{"response": "hello"}`,
				Duration: time.Second,
			},
			format:     core.OutputFormatJSON,
			wantParsed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gemini.parseOutput(tt.result, tt.format)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			hasParsed := got.Parsed != nil
			if hasParsed != tt.wantParsed {
				t.Errorf("hasParsed = %v, want %v", hasParsed, tt.wantParsed)
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

	tests := []struct {
		name     string
		opts     core.ExecuteOptions
		wantExec bool
		wantJSON bool
	}{
		{
			name:     "default args",
			opts:     core.ExecuteOptions{},
			wantExec: true,
		},
		{
			name:     "with JSON format",
			opts:     core.ExecuteOptions{Format: core.OutputFormatJSON},
			wantJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codex.buildArgs(tt.opts)
			if tt.wantExec && got[0] != "exec" {
				t.Errorf("expected first arg to be 'exec', got %q", got[0])
			}
			if tt.wantJSON {
				hasJSON := false
				for _, arg := range got {
					if arg == "--json" {
						hasJSON = true
						break
					}
				}
				if !hasJSON {
					t.Error("expected --json flag in args")
				}
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

func TestCodexAdapter_ParseOutputJSON(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	result := &CommandResult{
		Stdout:   `{"code": "print('hello')"}`,
		Duration: time.Second,
	}

	got, err := codex.parseOutput(result, core.OutputFormatJSON)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}
	if got.Parsed == nil {
		t.Error("expected parsed JSON")
	}
}

func TestCodexAdapter_EstimateCostCalc(t *testing.T) {
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	cost := codex.estimateCost(1000000, 1000000)
	// Input: 1M tokens * $2.50/MTok = $2.50
	// Output: 1M tokens * $10.00/MTok = $10.00
	// Total: $12.50
	if cost < 12.0 || cost > 13.0 {
		t.Errorf("estimateCost(1M, 1M) = %f, expected ~12.50", cost)
	}
}

// Test AiderAdapter specific functions

func TestAiderAdapter_BuildArgsModels(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{
		Path:  "aider",
		Model: "gpt-4o",
	})
	aider := adapter.(*AiderAdapter)

	tests := []struct {
		name    string
		opts    core.ExecuteOptions
		wantLen int
	}{
		{
			name: "default with GPT model",
			opts: core.ExecuteOptions{},
		},
		{
			name: "with Claude Opus",
			opts: core.ExecuteOptions{Model: "claude-3-opus-20240229"},
		},
		{
			name: "with Claude Sonnet",
			opts: core.ExecuteOptions{Model: "claude-3-5-sonnet-20241022"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aider.buildArgs(tt.opts)
			// Should contain --no-git, --no-auto-commits, --yes, --message
			hasNoGit := false
			hasYes := false
			hasMessage := false
			for _, arg := range got {
				switch arg {
				case "--no-git":
					hasNoGit = true
				case "--yes":
					hasYes = true
				case "--message":
					hasMessage = true
				}
			}
			if !hasNoGit || !hasYes || !hasMessage {
				t.Errorf("missing required flags: no-git=%v, yes=%v, message=%v", hasNoGit, hasYes, hasMessage)
			}
		})
	}
}

func TestAiderAdapter_CleanOutputSpinners(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{Path: "aider"})
	aider := adapter.(*AiderAdapter)

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "clean output",
			input:  "clean text",
			expect: "clean text",
		},
		{
			name:   "with progress indicators",
			input:  "[loading] some text [done]",
			expect: "some text",
		},
		{
			name:   "with spinners",
			input:  "⠋ loading ⠙ done",
			expect: "loading  done",
		},
		{
			name:   "with ANSI codes",
			input:  "\x1b[31mred text\x1b[0m",
			expect: "red text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aider.cleanOutput(tt.input)
			if got != tt.expect {
				t.Errorf("cleanOutput() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestAiderAdapter_ExtractUsageTokens(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{Path: "aider"})
	aider := adapter.(*AiderAdapter)

	tests := []struct {
		name     string
		stdout   string
		stderr   string
		wantIn   int
		wantOut  int
		wantCost float64
	}{
		{
			name:    "with token info",
			stdout:  "Tokens: 100 sent, 50 received",
			stderr:  "",
			wantIn:  100,
			wantOut: 50,
		},
		{
			name:     "with cost info",
			stdout:   "Cost: $0.05",
			stderr:   "",
			wantCost: 0.05,
		},
		{
			name:     "with both",
			stdout:   "Tokens: 200 sent, 100 received. Cost: $0.10",
			stderr:   "",
			wantIn:   200,
			wantOut:  100,
			wantCost: 0.10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CommandResult{Stdout: tt.stdout, Stderr: tt.stderr}
			execResult := &core.ExecuteResult{}
			aider.extractUsage(result, execResult)

			if tt.wantIn > 0 && execResult.TokensIn != tt.wantIn {
				t.Errorf("TokensIn = %d, want %d", execResult.TokensIn, tt.wantIn)
			}
			if tt.wantOut > 0 && execResult.TokensOut != tt.wantOut {
				t.Errorf("TokensOut = %d, want %d", execResult.TokensOut, tt.wantOut)
			}
			if tt.wantCost > 0 && execResult.CostUSD != tt.wantCost {
				t.Errorf("CostUSD = %f, want %f", execResult.CostUSD, tt.wantCost)
			}
		})
	}
}

func TestAiderAdapter_ParseOutputClean(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{Path: "aider"})
	aider := adapter.(*AiderAdapter)

	result := &CommandResult{
		Stdout:   "[loading] some output [done]",
		Duration: time.Second,
	}

	got, err := aider.parseOutput(result, core.OutputFormatText)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}
	// Should have cleaned the output
	if got.Output != "some output" {
		t.Errorf("Output = %q, expected cleaned output", got.Output)
	}
}

func TestAiderAdapter_EstimateCost(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{Path: "aider"})
	aider := adapter.(*AiderAdapter)

	cost := aider.estimateCost(1000000, 1000000)
	// Input: 1M tokens * $2.50/MTok = $2.50
	// Output: 1M tokens * $10.00/MTok = $10.00
	// Total: $12.50
	if cost < 12.0 || cost > 13.0 {
		t.Errorf("estimateCost(1M, 1M) = %f, expected ~12.50", cost)
	}
}

func TestAiderAdapter_WithEditFormatOptions(t *testing.T) {
	adapter, _ := NewAiderAdapter(AgentConfig{Path: "aider"})
	aider := adapter.(*AiderAdapter)

	tests := []struct {
		format string
		want   []string
	}{
		{"", []string{"--edit-format", "whole"}},
		{"diff", []string{"--edit-format", "diff"}},
		{"diff-fenced", []string{"--edit-format", "diff-fenced"}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := aider.WithEditFormat(tt.format)
			if len(got) != 2 || got[0] != tt.want[0] || got[1] != tt.want[1] {
				t.Errorf("WithEditFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
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
		wantCost   float64
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
			wantCost:   0.05,
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
			if tt.wantCost > 0 && execResult.CostUSD != tt.wantCost {
				t.Errorf("CostUSD = %f, want %f", execResult.CostUSD, tt.wantCost)
			}
		})
	}
}

func TestClaudeAdapter_ParseOutputFormats(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	tests := []struct {
		name       string
		result     *CommandResult
		format     core.OutputFormat
		wantParsed bool
	}{
		{
			name: "text output",
			result: &CommandResult{
				Stdout:   "plain text response",
				Duration: time.Second,
			},
			format:     core.OutputFormatText,
			wantParsed: false,
		},
		{
			name: "JSON output",
			result: &CommandResult{
				Stdout:   `{"response": "hello"}`,
				Duration: time.Second,
			},
			format:     core.OutputFormatJSON,
			wantParsed: true,
		},
		{
			name: "invalid JSON output",
			result: &CommandResult{
				Stdout:   `not valid json`,
				Duration: time.Second,
			},
			format:     core.OutputFormatJSON,
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := claude.parseOutput(tt.result, tt.format)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			if got.Output != tt.result.Stdout {
				t.Errorf("Output = %q, want %q", got.Output, tt.result.Stdout)
			}
			hasParsed := got.Parsed != nil
			if hasParsed != tt.wantParsed {
				t.Errorf("hasParsed = %v, want %v", hasParsed, tt.wantParsed)
			}
		})
	}
}

func TestClaudeAdapter_EstimateCostCalc(t *testing.T) {
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	// Test cost estimation
	cost := claude.estimateCost(1000, 500)
	// Input: 1000 tokens * $3/MTok = $0.003
	// Output: 500 tokens * $15/MTok = $0.0075
	// Total: ~$0.0105
	if cost < 0.01 || cost > 0.02 {
		t.Errorf("estimateCost(1000, 500) = %f, expected ~0.0105", cost)
	}
}

// Test CopilotAdapter specific functions not covered elsewhere

func TestCopilotAdapter_CleanANSIExtended(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "gh copilot"})
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

func TestCopilotAdapter_ExtractSuggestionPatterns(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "gh copilot"})
	copilot := adapter.(*CopilotAdapter)

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "with suggestion marker",
			output: "Thinking...\nSuggestion:\nls -la\n$ ",
			want:   "ls -la",
		},
		{
			name:   "multiple lines after suggestion",
			output: "Suggestion:\nfirst line\nsecond line\n? ",
			want:   "first line\nsecond line",
		},
		{
			name:   "no suggestion marker",
			output: "raw command output",
			want:   "raw command output",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilot.extractSuggestion(tt.output)
			if got != tt.want {
				t.Errorf("extractSuggestion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopilotAdapter_EstimateTokens(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "gh copilot"})
	copilot := adapter.(*CopilotAdapter)

	got := copilot.estimateTokens("hello world")
	want := 2 // 11 chars / 4 = 2
	if got != want {
		t.Errorf("estimateTokens() = %d, want %d", got, want)
	}
}

func TestCopilotAdapter_IsOutputCompleteScenarios(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "gh copilot"})
	copilot := adapter.(*CopilotAdapter)

	// Note: isOutputComplete uses TrimSpace before checking HasSuffix
	// So the marker must be at the end after trimming
	tests := []struct {
		output   string
		complete bool
	}{
		{"output ending with Suggestion:", true},
		{"incomplete output", false},
		{"", false},
		{"simple text without markers", false},
	}

	for _, tt := range tests {
		got := copilot.isOutputComplete(tt.output)
		if got != tt.complete {
			t.Errorf("isOutputComplete(%q) = %v, want %v", tt.output, got, tt.complete)
		}
	}
}

func TestCopilotAdapter_BuildArgsShell(t *testing.T) {
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "gh copilot"})
	copilot := adapter.(*CopilotAdapter)

	args := copilot.buildArgs(core.ExecuteOptions{})
	expected := []string{"suggest", "-t", "shell"}

	if len(args) != len(expected) {
		t.Errorf("buildArgs() len = %d, want %d", len(args), len(expected))
		return
	}
	for i := range args {
		if args[i] != expected[i] {
			t.Errorf("buildArgs()[%d] = %q, want %q", i, args[i], expected[i])
		}
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
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "gemini",
			wantPath:    "gemini",
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "copilot",
			wantPath:    "gh copilot",
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "codex",
			wantPath:    "codex",
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "aider",
			wantPath:    "aider",
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "unknown",
			wantPath:    "",
			wantTimeout: 5 * time.Minute,
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
}

func TestNewAiderAdapter_Capabilities(t *testing.T) {
	adapter, err := NewAiderAdapter(AgentConfig{Path: "aider"})
	if err != nil {
		t.Fatalf("NewAiderAdapter() error = %v", err)
	}
	if adapter.Name() != "aider" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "aider")
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
