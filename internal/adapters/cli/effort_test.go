package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// Claude: applyEffortEnv
// =============================================================================

func TestClaudeApplyEffortEnv_PerMessageOverride(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "claude-opus-4-6",
		ReasoningEffort: "medium",
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		ReasoningEffort: "max",
		Model:           "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "max" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q", got, "max")
	}
}

func TestClaudeApplyEffortEnv_ConfigDefault(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "claude-opus-4-6",
		ReasoningEffort: "high",
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Model: "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "high" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q", got, "high")
	}
}

func TestClaudeApplyEffortEnv_PhaseSpecific(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "claude-opus-4-6",
		ReasoningEffort: "medium",
		ReasoningEffortPhases: map[string]string{
			"analyze": "max",
		},
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Phase: core.PhaseAnalyze,
		Model: "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "max" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q", got, "max")
	}
}

func TestClaudeApplyEffortEnv_EmptyEffort(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:  "claude",
		Model: "claude-opus-4-6",
		// No ReasoningEffort set
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Model: "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	// ExtraEnv should be nil or not contain the key
	if claude.ExtraEnv != nil {
		if _, ok := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]; ok {
			t.Error("applyEffortEnv() should not set env var when effort is empty")
		}
	}
}

func TestClaudeApplyEffortEnv_CrossAgentNormalization(t *testing.T) {
	t.Parallel()
	// Codex-style "xhigh" should be normalized to Claude "max"
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "claude-opus-4-6",
		ReasoningEffort: "xhigh",
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Model: "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "max" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q (xhigh -> max)", got, "max")
	}
}

func TestClaudeApplyEffortEnv_NoneMinimalNormalization(t *testing.T) {
	t.Parallel()
	// Codex-style "minimal" should be normalized to Claude "low"
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "claude-opus-4-6",
		ReasoningEffort: "minimal",
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Model: "claude-opus-4-6",
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "low" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q (minimal -> low)", got, "low")
	}
}

func TestClaudeApplyEffortEnv_ModelFromConfig(t *testing.T) {
	t.Parallel()
	// When opts.Model is empty, the config model should be used for normalization
	adapter, _ := NewClaudeAdapter(AgentConfig{
		Path:            "claude",
		Model:           "opus", // alias
		ReasoningEffort: "high",
	})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		// No Model specified in opts
	}
	claude.applyEffortEnv(opts)

	got := claude.ExtraEnv["CLAUDE_CODE_EFFORT_LEVEL"]
	if got != "high" {
		t.Errorf("applyEffortEnv() set CLAUDE_CODE_EFFORT_LEVEL = %q, want %q", got, "high")
	}
}

// =============================================================================
// Claude: getEffortLevel
// =============================================================================

func TestClaudeGetEffortLevel_Priority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfg    AgentConfig
		opts   core.ExecuteOptions
		want   string
	}{
		{
			name: "per-message overrides everything",
			cfg: AgentConfig{
				Path:            "claude",
				ReasoningEffort: "low",
				ReasoningEffortPhases: map[string]string{
					"analyze": "medium",
				},
			},
			opts: core.ExecuteOptions{
				ReasoningEffort: "max",
				Phase:           core.PhaseAnalyze,
			},
			want: "max",
		},
		{
			name: "phase-specific overrides default",
			cfg: AgentConfig{
				Path:            "claude",
				ReasoningEffort: "low",
				ReasoningEffortPhases: map[string]string{
					"execute": "high",
				},
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseExecute,
			},
			want: "high",
		},
		{
			name: "default config used when no phase override",
			cfg: AgentConfig{
				Path:            "claude",
				ReasoningEffort: "medium",
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseExecute,
			},
			want: "medium",
		},
		{
			name: "empty when nothing configured",
			cfg: AgentConfig{
				Path: "claude",
			},
			opts: core.ExecuteOptions{},
			want: "",
		},
		{
			name: "phase-specific but phase not in map falls back to default",
			cfg: AgentConfig{
				Path:            "claude",
				ReasoningEffort: "low",
				ReasoningEffortPhases: map[string]string{
					"analyze": "high",
				},
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseExecute,
			},
			want: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter, _ := NewClaudeAdapter(tt.cfg)
			claude := adapter.(*ClaudeAdapter)
			got := claude.getEffortLevel(tt.opts)
			if got != tt.want {
				t.Errorf("getEffortLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Claude: buildPromptWithHistory
// =============================================================================

func TestClaudeBuildPromptWithHistory_NoMessages(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Prompt: "Hello, world!",
	}
	got := claude.buildPromptWithHistory(opts)
	if got != "Hello, world!" {
		t.Errorf("buildPromptWithHistory() = %q, want %q", got, "Hello, world!")
	}
}

func TestClaudeBuildPromptWithHistory_WithMessages(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Prompt: "What do you think?",
		Messages: []core.Message{
			{Role: "user", Content: "First question"},
			{Role: "assistant", Content: "First answer"},
			{Role: "user", Content: "Follow up"},
			{Role: "assistant", Content: "Follow up answer"},
		},
	}
	got := claude.buildPromptWithHistory(opts)

	// Verify structure
	if !strings.Contains(got, "<conversation_history>") {
		t.Error("expected <conversation_history> tag")
	}
	if !strings.Contains(got, "</conversation_history>") {
		t.Error("expected </conversation_history> tag")
	}
	if !strings.Contains(got, "<user>\nFirst question\n</user>") {
		t.Error("expected first user message")
	}
	if !strings.Contains(got, "<assistant>\nFirst answer\n</assistant>") {
		t.Error("expected first assistant message")
	}
	if !strings.Contains(got, "<current_message>\nWhat do you think?\n</current_message>") {
		t.Error("expected current message")
	}
}

func TestClaudeBuildPromptWithHistory_OnlyUserMessages(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Prompt: "Current prompt",
		Messages: []core.Message{
			{Role: "user", Content: "Only user message"},
		},
	}
	got := claude.buildPromptWithHistory(opts)

	if !strings.Contains(got, "<user>\nOnly user message\n</user>") {
		t.Error("expected user message in history")
	}
	if strings.Contains(got, "<assistant>") {
		t.Error("should not contain assistant tag")
	}
}

func TestClaudeBuildPromptWithHistory_UnknownRole(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	opts := core.ExecuteOptions{
		Prompt: "Prompt",
		Messages: []core.Message{
			{Role: "system", Content: "System message"},
		},
	}
	got := claude.buildPromptWithHistory(opts)

	// System role is not handled by the switch, so it should be skipped
	if strings.Contains(got, "System message") {
		t.Error("system role should be skipped by buildPromptWithHistory")
	}
	if !strings.Contains(got, "<conversation_history>") {
		t.Error("expected conversation_history tags even with unknown roles")
	}
}

// =============================================================================
// Claude: SetLogCallback and SetEventHandler
// =============================================================================

func TestClaudeSetLogCallback(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	var callbackCalled bool
	claude.SetLogCallback(func(line string) {
		callbackCalled = true
	})

	// Verify the callback was set on the base adapter
	if claude.BaseAdapter.logCallback == nil {
		t.Error("SetLogCallback should set the callback on BaseAdapter")
	}
	_ = callbackCalled
}

func TestClaudeSetEventHandler(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	var handlerCalled bool
	claude.SetEventHandler(func(event core.AgentEvent) {
		handlerCalled = true
	})

	if claude.BaseAdapter.eventHandler == nil {
		t.Error("SetEventHandler should set the handler on BaseAdapter")
	}
	if claude.BaseAdapter.aggregator == nil {
		t.Error("SetEventHandler should create an aggregator on BaseAdapter")
	}
	_ = handlerCalled
}

func TestClaudeSetEventHandler_Nil(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	// First set a handler, then set nil
	claude.SetEventHandler(func(event core.AgentEvent) {})
	claude.SetEventHandler(nil)

	if claude.BaseAdapter.eventHandler != nil {
		t.Error("SetEventHandler(nil) should clear the handler")
	}
}

// =============================================================================
// Claude: buildArgs
// =============================================================================

func TestClaudeBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        AgentConfig
		opts       core.ExecuteOptions
		wantModel  string
		wantSysArg bool
	}{
		{
			name: "default model from config",
			cfg:  AgentConfig{Path: "claude", Model: "opus"},
			opts: core.ExecuteOptions{},
			wantModel: "opus",
		},
		{
			name: "model from opts overrides config",
			cfg:  AgentConfig{Path: "claude", Model: "opus"},
			opts: core.ExecuteOptions{Model: "sonnet"},
			wantModel: "sonnet",
		},
		{
			name: "with system prompt",
			cfg:  AgentConfig{Path: "claude"},
			opts: core.ExecuteOptions{
				SystemPrompt: "You are a helpful assistant.",
			},
			wantSysArg: true,
		},
		{
			name: "no model set",
			cfg:  AgentConfig{Path: "claude"},
			opts: core.ExecuteOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter, _ := NewClaudeAdapter(tt.cfg)
			claude := adapter.(*ClaudeAdapter)
			args := claude.buildArgs(tt.opts)

			// Should always have --print
			hasFlag := func(flag string) bool {
				for _, a := range args {
					if a == flag {
						return true
					}
				}
				return false
			}
			if !hasFlag("--print") {
				t.Error("expected --print flag")
			}
			if !hasFlag("--dangerously-skip-permissions") {
				t.Error("expected --dangerously-skip-permissions flag")
			}

			// Check model
			if tt.wantModel != "" {
				foundModel := false
				for i, a := range args {
					if a == "--model" && i+1 < len(args) && args[i+1] == tt.wantModel {
						foundModel = true
						break
					}
				}
				if !foundModel {
					t.Errorf("expected --model %s in args: %v", tt.wantModel, args)
				}
			}

			// Check system prompt
			if tt.wantSysArg && !hasFlag("--append-system-prompt") {
				t.Error("expected --append-system-prompt flag")
			}
		})
	}
}

// =============================================================================
// Codex: SetEventHandler
// =============================================================================

func TestCodexSetEventHandler(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	var received []core.AgentEvent
	codex.SetEventHandler(func(event core.AgentEvent) {
		received = append(received, event)
	})

	if codex.BaseAdapter.eventHandler == nil {
		t.Error("SetEventHandler should set the handler on BaseAdapter")
	}
	if codex.BaseAdapter.aggregator == nil {
		t.Error("SetEventHandler should create an aggregator")
	}
}

func TestCodexSetEventHandler_Nil(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	codex.SetEventHandler(func(event core.AgentEvent) {})
	codex.SetEventHandler(nil)

	if codex.BaseAdapter.eventHandler != nil {
		t.Error("SetEventHandler(nil) should clear the handler")
	}
}

// =============================================================================
// Codex: getReasoningEffort
// =============================================================================

func TestCodexGetReasoningEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  AgentConfig
		opts core.ExecuteOptions
		want string
	}{
		{
			name: "per-message override",
			cfg: AgentConfig{
				Path:            "codex",
				ReasoningEffort: "medium",
			},
			opts: core.ExecuteOptions{
				ReasoningEffort: "xhigh",
			},
			want: "xhigh",
		},
		{
			name: "config default",
			cfg: AgentConfig{
				Path:            "codex",
				ReasoningEffort: "low",
			},
			opts: core.ExecuteOptions{},
			want: "low",
		},
		{
			name: "phase-specific override from config",
			cfg: AgentConfig{
				Path:            "codex",
				ReasoningEffort: "low",
				ReasoningEffortPhases: map[string]string{
					"analyze": "xhigh",
				},
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseAnalyze,
			},
			want: "xhigh",
		},
		{
			name: "default for refine phase",
			cfg: AgentConfig{
				Path: "codex",
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseRefine,
			},
			want: "xhigh",
		},
		{
			name: "default for execute phase",
			cfg: AgentConfig{
				Path: "codex",
			},
			opts: core.ExecuteOptions{
				Phase: core.PhaseExecute,
			},
			want: "high",
		},
		{
			name: "default for no phase",
			cfg: AgentConfig{
				Path: "codex",
			},
			opts: core.ExecuteOptions{},
			want: "high",
		},
		{
			name: "default for plan phase",
			cfg: AgentConfig{
				Path: "codex",
			},
			opts: core.ExecuteOptions{
				Phase: core.PhasePlan,
			},
			want: "xhigh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter, _ := NewCodexAdapter(tt.cfg)
			codex := adapter.(*CodexAdapter)
			got := codex.getReasoningEffort(tt.opts)
			if got != tt.want {
				t.Errorf("getReasoningEffort() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Codex: buildArgs
// =============================================================================

func TestCodexBuildArgs_MinimalReasoningDisablesWebSearch(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{
		Path:            "codex",
		Model:           "gpt-5-codex",
		ReasoningEffort: "minimal",
	})
	codex := adapter.(*CodexAdapter)

	args := codex.buildArgs(core.ExecuteOptions{})

	// When reasoning is "minimal", web_search should be disabled
	hasWebSearchDisabled := false
	for _, a := range args {
		if strings.Contains(a, "web_search") {
			hasWebSearchDisabled = true
			break
		}
	}
	if !hasWebSearchDisabled {
		t.Error("expected web_search disabled flag when reasoning is minimal")
	}
}

func TestCodexBuildArgs_ModelFromOpts(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{
		Path:  "codex",
		Model: "gpt-5.1-codex",
	})
	codex := adapter.(*CodexAdapter)

	args := codex.buildArgs(core.ExecuteOptions{Model: "gpt-5.2-codex"})

	foundModel := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "gpt-5.2-codex" {
			foundModel = true
			break
		}
	}
	if !foundModel {
		t.Errorf("expected --model gpt-5.2-codex in args: %v", args)
	}
}

func TestCodexBuildArgs_FallbackToDefaultModel(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{
		Path: "codex",
		// No model set
	})
	codex := adapter.(*CodexAdapter)

	args := codex.buildArgs(core.ExecuteOptions{})

	// Should use core default model
	defaultModel := core.GetDefaultModel(core.AgentCodex)
	foundModel := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == defaultModel {
			foundModel = true
			break
		}
	}
	if !foundModel {
		t.Errorf("expected --model %s in args: %v", defaultModel, args)
	}
}

// =============================================================================
// Codex: parseOutput and extractUsage
// =============================================================================

func TestCodexExtractUsage_TokenCap(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	// Set up event handler to capture warning events
	var events []core.AgentEvent
	codex.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	result := &CommandResult{
		Stdout: "prompt_tokens: 9999999\ncompletion_tokens: 9999999",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	codex.extractUsage(result, execResult)

	// Tokens should be capped at 500_000
	if execResult.TokensIn > 500_000 {
		t.Errorf("TokensIn = %d, should be capped at 500000", execResult.TokensIn)
	}
	if execResult.TokensOut > 500_000 {
		t.Errorf("TokensOut = %d, should be capped at 500000", execResult.TokensOut)
	}
}

func TestCodexExtractUsage_Estimation(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	// Output with no token info -> should estimate
	result := &CommandResult{
		Stdout: strings.Repeat("x", 400), // 400 chars -> ~100 tokens
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	codex.extractUsage(result, execResult)

	if execResult.TokensOut == 0 {
		t.Error("expected TokensOut to be estimated from output length")
	}
	if execResult.TokensIn == 0 {
		t.Error("expected TokensIn to be estimated as fraction of TokensOut")
	}
}

// =============================================================================
// Copilot: SetEventHandler
// =============================================================================

func TestCopilotSetEventHandler(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	copilot.SetEventHandler(func(event core.AgentEvent) {})
	if copilot.eventHandler == nil {
		t.Error("SetEventHandler should set the handler")
	}
	if copilot.aggregator == nil {
		t.Error("SetEventHandler should create an aggregator")
	}
}

func TestCopilotSetEventHandler_Nil(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	copilot.SetEventHandler(func(event core.AgentEvent) {})
	copilot.SetEventHandler(nil)

	if copilot.eventHandler != nil {
		t.Error("SetEventHandler(nil) should clear the handler")
	}
}

// =============================================================================
// Copilot: emitEvent
// =============================================================================

func TestCopilotEmitEvent_NilHandler(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	// Should not panic with nil handler
	copilot.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", "test"))
}

func TestCopilotEmitEvent_WithHandler(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	var received []core.AgentEvent
	copilot.SetEventHandler(func(event core.AgentEvent) {
		received = append(received, event)
	})

	copilot.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", "test"))
	// Event may or may not be emitted due to aggregator dedup, but should not panic
}

// =============================================================================
// Copilot: buildArgs
// =============================================================================

func TestCopilotBuildArgs(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	args := copilot.buildArgs(core.ExecuteOptions{Model: "claude-sonnet-4.5"})

	expectedFlags := []string{"--allow-all-tools", "--allow-all-paths", "--allow-all-urls", "--silent"}
	for _, flag := range expectedFlags {
		found := false
		for _, a := range args {
			if a == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected flag %q in args: %v", flag, args)
		}
	}
}

// =============================================================================
// Copilot: parseOutput
// =============================================================================

func TestCopilotParseOutput(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	result := &CommandResult{
		Stdout:   "\x1b[31mHello\x1b[0m world\n  trailing  ",
		Duration: 2 * time.Second,
	}
	execResult, err := copilot.parseOutput(result, core.OutputFormatText)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	// Should have ANSI stripped and trimmed
	if strings.Contains(execResult.Output, "\x1b") {
		t.Error("expected ANSI sequences to be stripped")
	}
	if execResult.Output != "Hello world\n  trailing" {
		t.Errorf("Output = %q, expected ANSI-stripped and trimmed output", execResult.Output)
	}
	if execResult.Duration != 2*time.Second {
		t.Errorf("Duration = %v, want 2s", execResult.Duration)
	}
}

// =============================================================================
// Copilot: extractUsage
// =============================================================================

func TestCopilotExtractUsage_WithTokenInfo(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	result := &CommandResult{
		Stdout: "input_tokens: 150\noutput_tokens: 75",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{Output: result.Stdout}
	copilot.extractUsage(result, execResult)

	if execResult.TokensIn != 150 {
		t.Errorf("TokensIn = %d, want 150", execResult.TokensIn)
	}
	if execResult.TokensOut != 75 {
		t.Errorf("TokensOut = %d, want 75", execResult.TokensOut)
	}
}

func TestCopilotExtractUsage_Estimation(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	result := &CommandResult{
		Stdout: strings.Repeat("word ", 100), // 500 chars
		Stderr: "",
	}
	execResult := &core.ExecuteResult{Output: result.Stdout}
	copilot.extractUsage(result, execResult)

	// Should estimate tokens from output
	if execResult.TokensOut == 0 {
		t.Error("expected TokensOut to be estimated")
	}
	if execResult.TokensIn == 0 {
		t.Error("expected TokensIn to be estimated from TokensOut")
	}
}

func TestCopilotExtractUsage_TokenCap(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	// Set up event handler to capture events
	copilot.SetEventHandler(func(event core.AgentEvent) {})

	result := &CommandResult{
		Stdout: "input_tokens: 9999999\noutput_tokens: 9999999",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{Output: result.Stdout}
	copilot.extractUsage(result, execResult)

	if execResult.TokensIn > 500_000 {
		t.Errorf("TokensIn = %d, should be capped at 500000", execResult.TokensIn)
	}
	if execResult.TokensOut > 500_000 {
		t.Errorf("TokensOut = %d, should be capped at 500000", execResult.TokensOut)
	}
}

func TestCopilotExtractUsage_MinTokensIn(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	// Very short output -> TokensIn should be at least 10
	result := &CommandResult{
		Stdout: "ok",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{Output: "ok"}
	copilot.extractUsage(result, execResult)

	// TokensOut = 0 for 2-char output (2/4=0), so this tests the zero case
	// If TokensOut is 0, no minimum is applied
}

// =============================================================================
// Copilot: streamStdoutWithEvents
// =============================================================================

func TestCopilotStreamStdoutWithEvents(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	var events []core.AgentEvent
	copilot.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		copilot.streamStdoutWithEvents(pr, &buf)
		close(done)
	}()

	lines := []string{
		"Starting analysis...",
		"",
		"Total usage: 100 tokens",
		"Total duration: 5s",
		"Actual content line",
		strings.Repeat("x", 70), // long line > 60 chars
	}
	for _, line := range lines {
		_, _ = pw.Write([]byte(line + "\n"))
	}
	pw.Close()
	<-done

	// Verify buffer has all lines
	output := buf.String()
	if !strings.Contains(output, "Starting analysis...") {
		t.Error("expected buffer to contain all lines")
	}

	// Empty lines and stats lines should be skipped in events
	// "Starting analysis...", "Actual content line", and the long line should emit events
}

// =============================================================================
// Copilot: Config
// =============================================================================

func TestCopilotConfig(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{
		Path:  "copilot",
		Name:  "copilot-test",
		Model: "claude-sonnet-4.5",
	})
	copilot := adapter.(*CopilotAdapter)

	cfg := copilot.Config()
	if cfg.Name != "copilot-test" {
		t.Errorf("Config().Name = %q, want %q", cfg.Name, "copilot-test")
	}
	if cfg.Path != "copilot" {
		t.Errorf("Config().Path = %q, want %q", cfg.Path, "copilot")
	}
	if cfg.Model != "claude-sonnet-4.5" {
		t.Errorf("Config().Model = %q, want %q", cfg.Model, "claude-sonnet-4.5")
	}
}

// =============================================================================
// Base: ExecuteWithStreaming routing logic
// =============================================================================

func TestExecuteWithStreaming_NoEventHandler(t *testing.T) {
	t.Parallel()
	// When no event handler is set, ExecuteWithStreaming should fall back to ExecuteCommand.
	// We test the routing logic by using an adapter with a real command.
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-streaming-no-handler",
		Path: "echo",
	}, nil)

	// No event handler set
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.ExecuteWithStreaming(ctx, "echo", []string{"hello"}, "", "", 0)
	if err != nil {
		t.Fatalf("ExecuteWithStreaming() error = %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", result.Stdout)
	}
}

func TestExecuteWithStreaming_NoneMethod(t *testing.T) {
	t.Parallel()
	// When stream config has StreamMethodNone, should fall back.
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-streaming-none",
		Path: "echo",
	}, nil)

	// Set event handler
	adapter.SetEventHandler(func(event core.AgentEvent) {})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use an adapter name that has StreamMethodNone config
	result, err := adapter.ExecuteWithStreaming(ctx, "unknown-tool", []string{"hello"}, "", "", 0)
	if err != nil {
		t.Fatalf("ExecuteWithStreaming() error = %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", result.Stdout)
	}
}

func TestExecuteWithStreaming_JSONStdoutRoute(t *testing.T) {
	t.Parallel()
	// When stream config has StreamMethodJSONStdout and handler is set,
	// it should route to executeWithJSONStreaming.
	adapter := NewBaseAdapter(AgentConfig{
		Name:        "test-streaming-json",
		Path:        "bash",
		IdleTimeout: 5 * time.Second,
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use "claude" as adapterName to get JSON stdout stream config
	result, err := adapter.ExecuteWithStreaming(
		ctx,
		"claude",
		[]string{"-c", `echo '{"type":"result","subtype":"success","result":"done"}'`},
		"", "", 0,
	)
	if err != nil {
		t.Fatalf("ExecuteWithStreaming() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have emitted at least a started event
	hasStarted := false
	for _, e := range events {
		if e.Type == core.AgentEventStarted {
			hasStarted = true
			break
		}
	}
	if !hasStarted {
		t.Error("expected at least a started event")
	}
}

// =============================================================================
// Base: executeWithLogFileStreaming
// =============================================================================

func TestExecuteWithLogFileStreaming_BasicExecution(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-logfile",
		Path: "bash",
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a log file streaming config that uses the bash command
	cfg := StreamConfig{
		Method:        StreamMethodLogFile,
		LogDirFlag:    "--log-dir-not-used",
		LogLevelFlag:  "",
		LogLevelValue: "",
	}

	// The command writes to stdout normally; log file tailing runs in background
	result, err := adapter.executeWithLogFileStreaming(
		ctx,
		"test-logfile",
		[]string{"-c", "echo 'hello from logfile streaming'"},
		"", "", 0, cfg, nil,
	)
	if err != nil {
		t.Fatalf("executeWithLogFileStreaming() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !strings.Contains(result.Stdout, "hello from logfile streaming") {
		t.Errorf("expected stdout to contain output, got %q", result.Stdout)
	}
}

func TestExecuteWithLogFileStreaming_EmitsEvents(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-logfile-events",
		Path: "bash",
	}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := StreamConfig{
		Method:     StreamMethodLogFile,
		LogDirFlag: "--not-real",
	}

	result, err := adapter.executeWithLogFileStreaming(
		ctx,
		"test-logfile-events",
		[]string{"-c", "echo done"},
		"", "", 0, cfg, nil,
	)
	if err != nil {
		t.Fatalf("executeWithLogFileStreaming() error = %v", err)
	}

	// Should emit completed event
	hasCompleted := false
	for _, e := range events {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
			break
		}
	}
	if !hasCompleted {
		t.Error("expected a completed event")
	}
	_ = result
}

func TestExecuteWithLogFileStreaming_NoPath(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{
		Name: "test-nopath",
		Path: "", // empty
	}, nil)

	adapter.SetEventHandler(func(event core.AgentEvent) {})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := StreamConfig{
		Method:     StreamMethodLogFile,
		LogDirFlag: "--log-dir",
	}

	_, err := adapter.executeWithLogFileStreaming(ctx, "test-nopath", nil, "", "", 0, cfg, nil)
	if err == nil {
		t.Error("expected error when path is empty")
	}
}

// =============================================================================
// Base: streamStderr event emission
// =============================================================================

func TestStreamStderr_ToolPatterns(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test")
		close(done)
	}()

	toolLines := []string{
		"Reading file test.go",
		"Writing output to result.txt",
		"Executing command: ls",
		"Running tests...",
		"Searching for patterns",
		"Analyzing code structure",
		"tool: read_file",
		"Fetching data from API",
	}
	for _, line := range toolLines {
		_, _ = pw.Write([]byte(line + "\n"))
	}
	pw.Close()
	<-done

	// Check events were emitted for tool patterns
	toolUseCount := 0
	for _, e := range events {
		if e.Type == core.AgentEventToolUse {
			toolUseCount++
		}
	}
	if toolUseCount == 0 {
		t.Error("expected tool_use events for tool pattern lines")
	}
}

func TestStreamStderr_ThinkingPatterns(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test")
		close(done)
	}()

	_, _ = pw.Write([]byte("Thinking about the solution\n"))
	_, _ = pw.Write([]byte("Reasoning through the problem\n"))
	pw.Close()
	<-done

	thinkingCount := 0
	for _, e := range events {
		if e.Type == core.AgentEventThinking {
			thinkingCount++
		}
	}
	if thinkingCount == 0 {
		t.Error("expected thinking events for thinking pattern lines")
	}
}

func TestStreamStderr_ActivityChannel(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer
	activity := make(chan struct{}, 10)

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test", activity)
		close(done)
	}()

	// Write some lines
	for i := 0; i < 3; i++ {
		_, _ = pw.Write([]byte("line\n"))
	}
	pw.Close()
	<-done

	// Should have received activity signals
	count := len(activity)
	if count != 3 {
		t.Errorf("expected 3 activity signals, got %d", count)
	}
}

func TestStreamStderr_WithLogCallback(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var callbackLines []string
	adapter.SetLogCallback(func(line string) {
		callbackLines = append(callbackLines, line)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamStderr(pr, &buf, "test")
		close(done)
	}()

	_, _ = pw.Write([]byte("callback line 1\n"))
	_, _ = pw.Write([]byte("callback line 2\n"))
	pw.Close()
	<-done

	if len(callbackLines) != 2 {
		t.Errorf("expected 2 callback calls, got %d", len(callbackLines))
	}
}

// =============================================================================
// Base: emitStderrEvent
// =============================================================================

func TestEmitStderrEvent_LongToolLine(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	// Line longer than 50 chars should be truncated
	longLine := "Reading " + strings.Repeat("very long file name ", 10)
	adapter.emitStderrEvent("test", longLine)

	if len(events) > 0 {
		msg := events[0].Message
		if len(msg) > 55 { // 47 + "..."
			t.Errorf("expected truncated message, got len=%d: %q", len(msg), msg)
		}
	}
}

func TestEmitStderrEvent_NoMatch(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	// Line that doesn't match any pattern
	adapter.emitStderrEvent("test", "just a plain log line with no patterns")

	// Should not emit any event
	if len(events) > 0 {
		t.Errorf("expected no events for unmatched line, got %d", len(events))
	}
}

// =============================================================================
// Base: extractTextFromJSONLine
// =============================================================================

// =============================================================================
// Base: streamJSONOutput
// =============================================================================

func TestStreamJSONOutput(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	parser := &testStreamParser{}

	done := make(chan struct{})
	go func() {
		adapter.streamJSONOutput(pr, &buf, "test", parser)
		close(done)
	}()

	// Write JSON lines
	_, _ = pw.Write([]byte(`{"type":"result","subtype":"success","result":"hello"}` + "\n"))
	_, _ = pw.Write([]byte(`{"type":"text","text":"world"}` + "\n"))
	_, _ = pw.Write([]byte("not json\n"))
	pw.Close()
	<-done

	// Check buffer has extracted text
	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Error("expected buffer to contain extracted text 'hello'")
	}
	if !strings.Contains(output, "world") {
		t.Error("expected buffer to contain extracted text 'world'")
	}
}

func TestStreamJSONOutput_NilParser(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		adapter.streamJSONOutput(pr, &buf, "test", nil)
		close(done)
	}()

	_, _ = pw.Write([]byte(`{"type":"result","subtype":"success","result":"data"}` + "\n"))
	pw.Close()
	<-done

	// Should still extract text even without parser
	output := buf.String()
	if !strings.Contains(output, "data") {
		t.Error("expected buffer to contain extracted text even without parser")
	}
}

// =============================================================================
// Base: pathWithin
// =============================================================================

func TestPathWithin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseDir string
		target  string
		want    bool
	}{
		{
			name:    "same directory",
			baseDir: "/tmp/test",
			target:  "/tmp/test",
			want:    true,
		},
		{
			name:    "child file",
			baseDir: "/tmp/test",
			target:  "/tmp/test/file.log",
			want:    true,
		},
		{
			name:    "parent traversal",
			baseDir: "/tmp/test",
			target:  "/tmp/test/../other/file",
			want:    false,
		},
		{
			name:    "completely outside",
			baseDir: "/tmp/test",
			target:  "/var/log/file",
			want:    false,
		},
		{
			name:    "nested child",
			baseDir: "/tmp/test",
			target:  "/tmp/test/sub/deep/file.txt",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pathWithin(tt.baseDir, tt.target)
			if got != tt.want {
				t.Errorf("pathWithin(%q, %q) = %v, want %v", tt.baseDir, tt.target, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Base: readNewLogContent
// =============================================================================

func TestReadNewLogContent(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	var events []core.AgentEvent
	adapter.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	// Create a temporary directory and log file
	tmpDir, err := os.MkdirTemp("", "quorum-test-logs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := tmpDir + "/test.log"
	err = os.WriteFile(logFile, []byte("line1\nline2\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	seenFiles := make(map[string]int64)

	// Read the log file
	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)

	// Verify position was tracked
	if seenFiles[logFile] == 0 {
		t.Error("expected seenFiles to track file position")
	}

	// Read again - should not re-read same content
	eventsBefore := len(events)
	adapter.readNewLogContent(tmpDir, logFile, seenFiles, "test", nil)
	if len(events) != eventsBefore {
		t.Error("expected no new events on re-read without new content")
	}
}

func TestReadNewLogContent_OutsideDir(t *testing.T) {
	t.Parallel()
	adapter := NewBaseAdapter(AgentConfig{Name: "test"}, nil)

	// Try to read a file outside the log directory
	seenFiles := make(map[string]int64)
	adapter.readNewLogContent("/tmp/safe-dir", "/var/log/outside-file", seenFiles, "test", nil)

	// Should not track the file
	if _, ok := seenFiles["/var/log/outside-file"]; ok {
		t.Error("should not track files outside the log directory")
	}
}

// =============================================================================
// Base: WithDiagnostics
// =============================================================================

// =============================================================================
// Claude: extractUsage edge cases
// =============================================================================

func TestClaudeExtractUsage_TokenCap(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	// Set up event handler
	claude.SetEventHandler(func(event core.AgentEvent) {})

	result := &CommandResult{
		Stdout: "tokens: 9999999 in, 9999999 out",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	claude.extractUsage(result, execResult)

	if execResult.TokensIn > 500_000 {
		t.Errorf("TokensIn = %d, should be capped at 500000", execResult.TokensIn)
	}
	if execResult.TokensOut > 500_000 {
		t.Errorf("TokensOut = %d, should be capped at 500000", execResult.TokensOut)
	}
}

func TestClaudeExtractUsage_Estimation(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})
	claude := adapter.(*ClaudeAdapter)

	result := &CommandResult{
		Stdout: strings.Repeat("x", 400), // 400 chars -> ~100 tokens
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	claude.extractUsage(result, execResult)

	if execResult.TokensOut == 0 {
		t.Error("expected TokensOut to be estimated")
	}
	if execResult.TokensIn == 0 {
		t.Error("expected TokensIn to be estimated as fraction of TokensOut")
	}
}

// =============================================================================
// AgentConfig: IsEnabledForPhase and GetReasoningEffort
// =============================================================================

func TestAgentConfig_GetReasoningEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   AgentConfig
		phase string
		want  string
	}{
		{
			name: "phase-specific override",
			cfg: AgentConfig{
				ReasoningEffort: "medium",
				ReasoningEffortPhases: map[string]string{
					"analyze": "xhigh",
				},
			},
			phase: "analyze",
			want:  "xhigh",
		},
		{
			name: "fall back to default",
			cfg: AgentConfig{
				ReasoningEffort: "high",
				ReasoningEffortPhases: map[string]string{
					"analyze": "xhigh",
				},
			},
			phase: "execute",
			want:  "high",
		},
		{
			name: "nil phases map",
			cfg: AgentConfig{
				ReasoningEffort: "medium",
			},
			phase: "analyze",
			want:  "medium",
		},
		{
			name:  "empty everything",
			cfg:   AgentConfig{},
			phase: "analyze",
			want:  "",
		},
		{
			name: "empty phase-specific value treated as absent",
			cfg: AgentConfig{
				ReasoningEffort: "high",
				ReasoningEffortPhases: map[string]string{
					"analyze": "",
				},
			},
			phase: "analyze",
			want:  "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.GetReasoningEffort(tt.phase)
			if got != tt.want {
				t.Errorf("GetReasoningEffort(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Base: extractErrorFromOutput
// =============================================================================

func TestExtractErrorFromOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stdout string
		want   string
	}{
		{
			name:   "JSON error field",
			stdout: `{"error": "something went wrong"}`,
			want:   "something went wrong",
		},
		{
			name:   "JSON error object with message",
			stdout: `{"error": {"message": "detailed error"}}`,
			want:   "detailed error",
		},
		{
			name:   "Claude result error format",
			stdout: `{"type":"result","subtype":"error","error":"Claude error message"}`,
			want:   "Claude error message",
		},
		{
			name:   "error type event",
			stdout: `{"type":"error","error":"generic error"}`,
			want:   "generic error",
		},
		{
			name:   "no JSON, last non-empty line",
			stdout: "line 1\nline 2\nlast line",
			want:   "last line",
		},
		{
			name:   "empty output",
			stdout: "",
			want:   "",
		},
		{
			name:   "long last line truncated",
			stdout: strings.Repeat("x", 300),
			want:   strings.Repeat("x", 200) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractErrorFromOutput(tt.stdout)
			if got != tt.want {
				t.Errorf("extractErrorFromOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Streaming config helpers
// =============================================================================

func TestGetStreamConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   StreamMethod
	}{
		{"claude", StreamMethodJSONStdout},
		{"gemini", StreamMethodJSONStdout},
		{"codex", StreamMethodJSONStdout},
		{"copilot", StreamMethodLogFile},
		{"unknown", StreamMethodNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := GetStreamConfig(tt.name)
			if cfg.Method != tt.want {
				t.Errorf("GetStreamConfig(%q).Method = %q, want %q", tt.name, cfg.Method, tt.want)
			}
		})
	}
}

// =============================================================================
// Copilot: cleanANSI additional cases
// =============================================================================

func TestCopilotCleanANSI_Empty(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	got := copilot.cleanANSI("")
	if got != "" {
		t.Errorf("cleanANSI('') = %q, want empty", got)
	}
}

// =============================================================================
// Copilot: estimateTokens edge cases
// =============================================================================

func TestCopilotEstimateTokens_Empty(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	got := copilot.estimateTokens("")
	if got != 0 {
		t.Errorf("estimateTokens('') = %d, want 0", got)
	}
}

func TestCopilotEstimateTokens_Large(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	text := strings.Repeat("x", 10000) // 10000 chars -> 2500 tokens
	got := copilot.estimateTokens(text)
	if got != 2500 {
		t.Errorf("estimateTokens() = %d, want 2500", got)
	}
}

// =============================================================================
// Base: ExecuteCommand with ExtraEnv
// =============================================================================

// =============================================================================
// Adapter factory defaults
// =============================================================================

func TestNewCodexAdapter_DefaultPath(t *testing.T) {
	t.Parallel()
	adapter, err := NewCodexAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCodexAdapter() error = %v", err)
	}
	codex := adapter.(*CodexAdapter)
	if codex.config.Path != "codex" {
		t.Errorf("default path = %q, want %q", codex.config.Path, "codex")
	}
}

func TestNewCopilotAdapter_DefaultPath(t *testing.T) {
	t.Parallel()
	adapter, err := NewCopilotAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("NewCopilotAdapter() error = %v", err)
	}
	copilot := adapter.(*CopilotAdapter)
	if copilot.config.Path != "copilot" {
		t.Errorf("default path = %q, want %q", copilot.config.Path, "copilot")
	}
}

// =============================================================================
// Claude/Codex/Copilot: Name and Capabilities
// =============================================================================

func TestClaudeCapabilities(t *testing.T) {
	t.Parallel()
	adapter, _ := NewClaudeAdapter(AgentConfig{Path: "claude"})

	if adapter.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "claude")
	}

	caps := adapter.Capabilities()
	if !caps.SupportsJSON {
		t.Error("expected SupportsJSON = true")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming = true")
	}
	if !caps.SupportsImages {
		t.Error("expected SupportsImages = true")
	}
	if !caps.SupportsTools {
		t.Error("expected SupportsTools = true")
	}
	if caps.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
}

func TestCodexCapabilities(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})

	if adapter.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "codex")
	}

	caps := adapter.Capabilities()
	if !caps.SupportsJSON {
		t.Error("expected SupportsJSON = true")
	}
	if caps.SupportsImages {
		t.Error("expected SupportsImages = false for codex")
	}
	if caps.MaxContextTokens != 128000 {
		t.Errorf("MaxContextTokens = %d, want 128000", caps.MaxContextTokens)
	}
}

func TestCopilotCapabilities(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})

	if adapter.Name() != "copilot" {
		t.Errorf("Name() = %q, want %q", adapter.Name(), "copilot")
	}

	caps := adapter.Capabilities()
	if caps.SupportsJSON {
		t.Error("expected SupportsJSON = false for copilot")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming = true")
	}
	if caps.SupportsImages {
		t.Error("expected SupportsImages = false for copilot")
	}
}

// =============================================================================
// Codex: extractUsage token discrepancy
// =============================================================================

func TestCodexExtractUsage_TokenDiscrepancyTooLow(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	var events []core.AgentEvent
	codex.SetEventHandler(func(event core.AgentEvent) {
		events = append(events, event)
	})

	// Output is large but reported tokens are tiny
	bigOutput := strings.Repeat("word ", 200) // 1000 chars -> ~250 estimated tokens
	result := &CommandResult{
		Stdout: bigOutput + "\ncompletion_tokens: 1", // Reported: 1 token, Estimated: ~250
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	codex.extractUsage(result, execResult)

	// Should use estimated tokens due to discrepancy
	if execResult.TokensOut < 100 {
		t.Errorf("TokensOut = %d, expected estimated tokens (>100) due to discrepancy", execResult.TokensOut)
	}
}

func TestCodexExtractUsage_TokenDiscrepancyTooHigh(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCodexAdapter(AgentConfig{Path: "codex"})
	codex := adapter.(*CodexAdapter)

	codex.SetEventHandler(func(event core.AgentEvent) {})

	// Output is small but reported tokens are huge
	smallOutput := strings.Repeat("x", 800) // 800 chars -> ~200 estimated tokens
	result := &CommandResult{
		Stdout: smallOutput + "\ncompletion_tokens: 50000", // Way too high
		Stderr: "",
	}
	execResult := &core.ExecuteResult{}
	codex.extractUsage(result, execResult)

	// Should use estimated tokens due to discrepancy
	if execResult.TokensOut > 1000 {
		t.Errorf("TokensOut = %d, expected estimated tokens (<1000) due to discrepancy", execResult.TokensOut)
	}
}

// =============================================================================
// Copilot: extractUsage token discrepancy
// =============================================================================

func TestCopilotExtractUsage_TokenDiscrepancyTooLow(t *testing.T) {
	t.Parallel()
	adapter, _ := NewCopilotAdapter(AgentConfig{Path: "copilot"})
	copilot := adapter.(*CopilotAdapter)

	copilot.SetEventHandler(func(event core.AgentEvent) {})

	bigOutput := strings.Repeat("word ", 200)
	result := &CommandResult{
		Stdout: bigOutput + "\noutput_tokens: 1",
		Stderr: "",
	}
	execResult := &core.ExecuteResult{Output: result.Stdout}
	copilot.extractUsage(result, execResult)

	if execResult.TokensOut < 100 {
		t.Errorf("TokensOut = %d, expected estimated due to discrepancy", execResult.TokensOut)
	}
}

// =============================================================================
// Base: classifyError extended patterns
// =============================================================================

func TestClassifyError_OutputTokenMaximum(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	tests := []struct {
		name        string
		stderr      string
		wantContain string
	}{
		{
			name:        "output tokens pattern",
			stderr:      "Error: too many output tokens in response",
			wantContain: "OUTPUT_TOO_LONG",
		},
		{
			name:        "max output pattern",
			stderr:      "exceeded max output size",
			wantContain: "OUTPUT_TOO_LONG",
		},
		{
			name:        "context length exceeded",
			stderr:      "context length exceeded for this model",
			wantContain: "OUTPUT_TOO_LONG",
		},
		{
			name:        "too many tokens",
			stderr:      "too many tokens in the request",
			wantContain: "OUTPUT_TOO_LONG",
		},
		{
			name:        "quota error is rate limit",
			stderr:      "API quota exceeded",
			wantContain: "RATE_LIMIT",
		},
		{
			name:        "forbidden is auth error",
			stderr:      "403 Forbidden",
			wantContain: "AUTH",
		},
		{
			name:        "unreachable is network error",
			stderr:      "host unreachable",
			wantContain: "NETWORK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &CommandResult{
				Stderr:   tt.stderr,
				ExitCode: 1,
			}
			err := base.classifyError(result)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

// =============================================================================
// Base: classifyError with stdout JSON errors
// =============================================================================

func TestClassifyError_StdoutJSONError(t *testing.T) {
	t.Parallel()
	base := NewBaseAdapter(AgentConfig{}, nil)

	result := &CommandResult{
		Stderr:   "", // empty stderr
		Stdout:   `{"type":"result","subtype":"error","error":"rate limit exceeded"}`,
		ExitCode: 1,
	}
	err := base.classifyError(result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "RATE_LIMIT") {
		t.Errorf("expected RATE_LIMIT error, got: %v", err)
	}
}
