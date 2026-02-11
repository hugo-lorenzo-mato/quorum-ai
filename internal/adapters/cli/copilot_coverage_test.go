package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// CopilotAdapter Constructor Tests
// =============================================================================

func TestNewCopilotAdapter_DefaultPathWhenEmpty(t *testing.T) {
	t.Parallel()
	agent, err := NewCopilotAdapter(AgentConfig{Path: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := agent.(*CopilotAdapter)
	if c.config.Path != "copilot" {
		t.Errorf("Path = %q, want 'copilot'", c.config.Path)
	}
}

func TestNewCopilotAdapter_PreservesCustomPath(t *testing.T) {
	t.Parallel()
	agent, err := NewCopilotAdapter(AgentConfig{Path: "/custom/copilot"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := agent.(*CopilotAdapter)
	if c.config.Path != "/custom/copilot" {
		t.Errorf("Path = %q, want '/custom/copilot'", c.config.Path)
	}
}

func TestNewCopilotAdapter_SetsCapabilities(t *testing.T) {
	t.Parallel()
	agent, err := NewCopilotAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	caps := agent.Capabilities()
	if caps.SupportsJSON {
		t.Error("Copilot should NOT support JSON")
	}
	if !caps.SupportsStreaming {
		t.Error("Copilot should support streaming")
	}
	if caps.SupportsImages {
		t.Error("Copilot should NOT support images")
	}
	if !caps.SupportsTools {
		t.Error("Copilot should support tools")
	}
	if caps.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d, want 200000", caps.MaxContextTokens)
	}
	if caps.MaxOutputTokens != 16384 {
		t.Errorf("MaxOutputTokens = %d, want 16384", caps.MaxOutputTokens)
	}
	if len(caps.SupportedModels) == 0 {
		t.Error("SupportedModels should not be empty")
	}
	if caps.DefaultModel == "" {
		t.Error("DefaultModel should not be empty")
	}
}

func TestNewCopilotAdapter_HasNopLogger(t *testing.T) {
	t.Parallel()
	agent, err := NewCopilotAdapter(AgentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := agent.(*CopilotAdapter)
	if c.logger == nil {
		t.Error("logger should not be nil (should be nop)")
	}
}

// =============================================================================
// Name Tests
// =============================================================================

func TestCopilotAdapter_NameAlwaysCopilot(t *testing.T) {
	t.Parallel()
	agent, _ := NewCopilotAdapter(AgentConfig{Name: "custom-name"})
	if agent.Name() != "copilot" {
		t.Errorf("Name() = %q, want 'copilot'", agent.Name())
	}
}

// =============================================================================
// buildArgs Tests (extended)
// =============================================================================

func TestCopilotAdapter_BuildArgs_AlwaysHasExpectedFlags(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{}}

	// No matter what options are passed, certain flags must be present
	tests := []struct {
		name string
		opts core.ExecuteOptions
	}{
		{"empty opts", core.ExecuteOptions{}},
		{"with model", core.ExecuteOptions{Model: "gpt-4o"}},
		{"with format", core.ExecuteOptions{Format: core.OutputFormatJSON}},
		{"with timeout", core.ExecuteOptions{Timeout: time.Minute}},
	}

	requiredFlags := []string{
		"--allow-all-tools",
		"--allow-all-paths",
		"--allow-all-urls",
		"--silent",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := c.buildArgs(tt.opts)

			for _, flag := range requiredFlags {
				found := false
				for _, arg := range args {
					if arg == flag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildArgs(%s) missing required flag %q in %v", tt.name, flag, args)
				}
			}
		})
	}
}

func TestCopilotAdapter_BuildArgs_DoesNotIncludeModelFlag(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{Model: "gpt-4o"}}
	args := c.buildArgs(core.ExecuteOptions{Model: "gpt-5"})

	for _, arg := range args {
		if arg == "--model" || arg == "gpt-4o" || arg == "gpt-5" {
			t.Errorf("buildArgs should NOT include model flag or value, found: %q in %v", arg, args)
		}
	}
}

func TestCopilotAdapter_BuildArgs_DoesNotIncludeOutputFormat(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{}}
	args := c.buildArgs(core.ExecuteOptions{Format: core.OutputFormatJSON})

	for _, arg := range args {
		if arg == "--output-format" || arg == "json" || arg == "stream-json" {
			t.Errorf("buildArgs should NOT include output format flags, found: %q in %v", arg, args)
		}
	}
}

// =============================================================================
// parseOutput Tests (extended)
// =============================================================================

func TestCopilotAdapter_ParseOutput_ANSICleaning(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	tests := []struct {
		name   string
		stdout string
		want   string
	}{
		{
			name:   "bold and reset",
			stdout: "\x1b[1mBold\x1b[0m normal",
			want:   "Bold normal",
		},
		{
			name:   "multiple colors",
			stdout: "\x1b[31mred\x1b[0m \x1b[32mgreen\x1b[0m \x1b[34mblue\x1b[0m",
			want:   "red green blue",
		},
		{
			name:   "cursor control codes",
			stdout: "\x1b[0G\x1b[2K\x1b[1mTitle\x1b[0m",
			want:   "Title",
		},
		{
			name:   "no ANSI",
			stdout: "plain output text",
			want:   "plain output text",
		},
		{
			name:   "empty string",
			stdout: "",
			want:   "",
		},
		{
			name:   "only whitespace with ANSI",
			stdout: "  \x1b[1m\x1b[0m  ",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := c.parseOutput(&CommandResult{
				Stdout:   tt.stdout,
				Duration: time.Second,
			}, core.OutputFormatText)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			if result.Output != tt.want {
				t.Errorf("Output = %q, want %q", result.Output, tt.want)
			}
		})
	}
}

func TestCopilotAdapter_ParseOutput_PreservesDuration(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	dur := 3 * time.Second
	result, _ := c.parseOutput(&CommandResult{
		Stdout:   "output",
		Duration: dur,
	}, core.OutputFormatText)
	if result.Duration != dur {
		t.Errorf("Duration = %v, want %v", result.Duration, dur)
	}
}

// =============================================================================
// cleanANSI Tests (extended)
// =============================================================================

func TestCopilotAdapter_CleanANSI_Comprehensive(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no ansi", "plain text", "plain text"},
		{"red text", "\x1b[31mred\x1b[0m", "red"},
		{"green bold", "\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"256 color", "\x1b[38;5;196mcolor\x1b[0m", "color"},
		{"cursor move", "\x1b[5A\x1b[3B text", " text"},
		{"clear line", "\x1b[2K cleared", " cleared"},
		{"multiple sequences", "\x1b[1m\x1b[31m\x1b[4mformatted\x1b[0m", "formatted"},
		{"mixed content", "before \x1b[33myellow\x1b[0m after", "before yellow after"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := c.cleanANSI(tt.input)
			if got != tt.want {
				t.Errorf("cleanANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// estimateTokens Tests (extended)
// =============================================================================

func TestCopilotAdapter_EstimateTokens_Various(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"a", 0},        // 1/4 = 0
		{"abcd", 1},     // 4/4 = 1
		{"abcdefgh", 2}, // 8/4 = 2
		{strings.Repeat("x", 400), 100},
		{strings.Repeat("x", 4000), 1000},
	}

	for _, tt := range tests {
		got := c.estimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("estimateTokens(%d chars) = %d, want %d", len(tt.text), got, tt.want)
		}
	}
}

// =============================================================================
// extractUsage Tests (extended)
// =============================================================================

func TestCopilotAdapter_ExtractUsage_InputTokenPattern(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	result := &CommandResult{
		Stdout: "input_tokens: 500\noutput_tokens: 200",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 800)} // ~200 tokens

	c.extractUsage(result, execResult)

	if execResult.TokensIn != 500 {
		t.Errorf("TokensIn = %d, want 500", execResult.TokensIn)
	}
	if execResult.TokensOut != 200 {
		t.Errorf("TokensOut = %d, want 200", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_PromptCompletionPattern(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	result := &CommandResult{
		Stdout: "prompt_tokens: 300\ncompletion_tokens: 150",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 600)} // ~150 tokens

	c.extractUsage(result, execResult)

	if execResult.TokensIn != 300 {
		t.Errorf("TokensIn = %d, want 300", execResult.TokensIn)
	}
	if execResult.TokensOut != 150 {
		t.Errorf("TokensOut = %d, want 150", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_FallbackEstimate(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	result := &CommandResult{
		Stdout: "no token info here",
	}
	// Output is 400 chars = ~100 tokens estimated
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 400)}

	c.extractUsage(result, execResult)

	// Should use estimates
	if execResult.TokensOut != 100 { // 400/4
		t.Errorf("TokensOut = %d, want 100 (estimated)", execResult.TokensOut)
	}
	// TokensIn estimated as TokensOut/3 (min 10)
	expectedIn := 100 / 3
	if expectedIn < 10 {
		expectedIn = 10
	}
	if execResult.TokensIn != expectedIn {
		t.Errorf("TokensIn = %d, want %d (estimated)", execResult.TokensIn, expectedIn)
	}
}

func TestCopilotAdapter_ExtractUsage_FallbackEstimateSmallOutput(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	result := &CommandResult{Stdout: "no token info"}
	execResult := &core.ExecuteResult{Output: "hi"} // 2 chars = 0 tokens

	c.extractUsage(result, execResult)

	// With 0 estimated output tokens, input should also be 0
	if execResult.TokensOut != 0 {
		t.Errorf("TokensOut = %d, want 0", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_DiscrepancyTooLow(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	// Report suspiciously low tokens: 10 reported vs ~500 estimated (2000/4)
	result := &CommandResult{
		Stdout: "output_tokens: 10",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 2000)} // ~500 tokens

	c.extractUsage(result, execResult)

	// Should correct to estimated
	if execResult.TokensOut != 500 {
		t.Errorf("TokensOut = %d, want 500 (corrected to estimate)", execResult.TokensOut)
	}

	// Should have emitted a warning event
	hasDiscrepancyWarning := false
	for _, e := range emittedEvents {
		if strings.Contains(e.Message, "Token discrepancy") {
			hasDiscrepancyWarning = true
			break
		}
	}
	if !hasDiscrepancyWarning {
		t.Error("expected token discrepancy warning event")
	}
}

func TestCopilotAdapter_ExtractUsage_DiscrepancyTooHigh(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	// Report suspiciously high tokens: 100000 reported vs ~500 estimated
	result := &CommandResult{
		Stdout: "output_tokens: 100000",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 2000)} // ~500 tokens

	c.extractUsage(result, execResult)

	// Should correct to estimated
	if execResult.TokensOut != 500 {
		t.Errorf("TokensOut = %d, want 500 (corrected to estimate)", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_WithinThreshold(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	// Report tokens within acceptable range: 400 reported vs ~500 estimated
	result := &CommandResult{
		Stdout: "output_tokens: 400",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 2000)} // ~500 tokens

	c.extractUsage(result, execResult)

	// Should keep reported value
	if execResult.TokensOut != 400 {
		t.Errorf("TokensOut = %d, want 400 (within threshold)", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_CapUnrealisticTokensIn(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 0}} // disable discrepancy check

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	result := &CommandResult{
		Stdout: "input_tokens: 999999\noutput_tokens: 100",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 400)}

	c.extractUsage(result, execResult)

	// TokensIn should be capped at 500000
	if execResult.TokensIn != 500000 {
		t.Errorf("TokensIn = %d, want 500000 (capped)", execResult.TokensIn)
	}

	hasCappedWarning := false
	for _, e := range emittedEvents {
		if strings.Contains(e.Message, "Capped unrealistic TokensIn") {
			hasCappedWarning = true
			break
		}
	}
	if !hasCappedWarning {
		t.Error("expected capped warning event for TokensIn")
	}
}

func TestCopilotAdapter_ExtractUsage_CapUnrealisticTokensOut(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 0}} // disable discrepancy check

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	result := &CommandResult{
		Stdout: "input_tokens: 100\noutput_tokens: 888888",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 400)}

	c.extractUsage(result, execResult)

	// TokensOut should be capped at 500000
	if execResult.TokensOut != 500000 {
		t.Errorf("TokensOut = %d, want 500000 (capped)", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_DefaultThreshold(t *testing.T) {
	t.Parallel()
	// When threshold is <= 0, it defaults to DefaultTokenDiscrepancyThreshold (5)
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: -1}}

	result := &CommandResult{
		Stdout: "output_tokens: 400",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 2000)} // ~500 tokens

	c.extractUsage(result, execResult)

	// 400 is within 5x threshold of 500, so should be kept
	if execResult.TokensOut != 400 {
		t.Errorf("TokensOut = %d, want 400", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_StderrTokens(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	// Tokens reported in stderr
	result := &CommandResult{
		Stdout: "some output",
		Stderr: "input_tokens: 250\noutput_tokens: 120",
	}
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 480)} // ~120 tokens

	c.extractUsage(result, execResult)

	if execResult.TokensIn != 250 {
		t.Errorf("TokensIn = %d, want 250", execResult.TokensIn)
	}
	if execResult.TokensOut != 120 {
		t.Errorf("TokensOut = %d, want 120", execResult.TokensOut)
	}
}

func TestCopilotAdapter_ExtractUsage_FallbackMinInputTokens(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{config: AgentConfig{TokenDiscrepancyThreshold: 5}}

	result := &CommandResult{Stdout: "no token info"}
	// Output is small: 20 chars = 5 tokens
	execResult := &core.ExecuteResult{Output: strings.Repeat("x", 20)}

	c.extractUsage(result, execResult)

	if execResult.TokensOut != 5 {
		t.Errorf("TokensOut = %d, want 5", execResult.TokensOut)
	}
	// TokensIn = max(5/3, 10) = 10
	if execResult.TokensIn != 10 {
		t.Errorf("TokensIn = %d, want 10 (minimum)", execResult.TokensIn)
	}
}

// =============================================================================
// emitEvent Tests
// =============================================================================

func TestCopilotAdapter_EmitEvent_NilHandler(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}
	// Should not panic
	c.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", "test"))
}

func TestCopilotAdapter_EmitEvent_WithHandler(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	var received []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		received = append(received, e)
	})

	c.emitEvent(core.NewAgentEvent(core.AgentEventCompleted, "copilot", "done"))

	hasCompleted := false
	for _, e := range received {
		if e.Type == core.AgentEventCompleted {
			hasCompleted = true
		}
	}
	if !hasCompleted {
		t.Error("expected completed event to be emitted")
	}
}

func TestCopilotAdapter_EmitEvent_AggregatorFilters(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	var received []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		received = append(received, e)
	})

	// Emit many rapid progress events
	for i := 0; i < 50; i++ {
		c.emitEvent(core.NewAgentEvent(core.AgentEventProgress, "copilot", "progress"))
	}

	// The aggregator should filter some of these
	if len(received) >= 50 {
		t.Errorf("expected aggregator to filter some events, got %d/50", len(received))
	}
}

// =============================================================================
// streamStdoutWithEvents Tests
// =============================================================================

func TestCopilotAdapter_StreamStdoutWithEvents_BasicCapture(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		c.streamStdoutWithEvents(pr, &buf)
		close(done)
	}()

	_, _ = pw.Write([]byte("Hello world\nSecond line\n"))
	_ = pw.Close()
	<-done

	output := buf.String()
	if !strings.Contains(output, "Hello world") {
		t.Errorf("buffer should contain 'Hello world', got: %q", output)
	}
	if !strings.Contains(output, "Second line") {
		t.Errorf("buffer should contain 'Second line', got: %q", output)
	}
}

func TestCopilotAdapter_StreamStdoutWithEvents_SkipsEmptyLines(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		c.streamStdoutWithEvents(pr, &buf)
		close(done)
	}()

	_, _ = pw.Write([]byte("\n\n\nContent line\n\n"))
	_ = pw.Close()
	<-done

	// Only "Content line" should generate a progress event (not empty lines)
	progressCount := 0
	for _, e := range emittedEvents {
		if e.Type == core.AgentEventProgress {
			progressCount++
		}
	}
	if progressCount == 0 {
		t.Error("expected at least one progress event for content line")
	}
}

func TestCopilotAdapter_StreamStdoutWithEvents_SkipsStatsLines(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		c.streamStdoutWithEvents(pr, &buf)
		close(done)
	}()

	statsLines := "Total usage: 100 tokens\nTotal duration: 5s\nTotal code changes: 3\nUsage by model: gpt-4o\n"
	_, _ = pw.Write([]byte(statsLines))
	_ = pw.Close()
	<-done

	// Stats lines should be skipped for events (but still written to buffer)
	for _, e := range emittedEvents {
		if e.Type == core.AgentEventProgress {
			t.Errorf("stats line should not generate progress event: %q", e.Message)
		}
	}
}

func TestCopilotAdapter_StreamStdoutWithEvents_TruncatesLongLines(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	var emittedEvents []core.AgentEvent
	c.SetEventHandler(func(e core.AgentEvent) {
		emittedEvents = append(emittedEvents, e)
	})

	pr, pw := io.Pipe()
	var buf bytes.Buffer

	done := make(chan struct{})
	go func() {
		c.streamStdoutWithEvents(pr, &buf)
		close(done)
	}()

	longLine := strings.Repeat("x", 100) + "\n"
	_, _ = pw.Write([]byte(longLine))
	_ = pw.Close()
	<-done

	// Find progress events
	for _, e := range emittedEvents {
		if e.Type == core.AgentEventProgress {
			if len(e.Message) > 65 {
				t.Errorf("long line should be truncated, got length %d", len(e.Message))
			}
			if !strings.HasSuffix(e.Message, "...") {
				t.Errorf("truncated message should end with '...': %q", e.Message)
			}
		}
	}
}

// =============================================================================
// Config Tests
// =============================================================================

func TestCopilotAdapter_Config_ReturnsFullConfig(t *testing.T) {
	t.Parallel()
	cfg := AgentConfig{
		Name:                      "copilot-test",
		Path:                      "/usr/bin/copilot",
		Model:                     "gpt-4o",
		Timeout:                   3 * time.Minute,
		WorkDir:                   "/workspace",
		TokenDiscrepancyThreshold: 3.0,
	}
	agent, _ := NewCopilotAdapter(cfg)
	c := agent.(*CopilotAdapter)

	got := c.Config()
	if got.Name != "copilot-test" {
		t.Errorf("Name = %q, want 'copilot-test'", got.Name)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Model = %q, want 'gpt-4o'", got.Model)
	}
	if got.Timeout != 3*time.Minute {
		t.Errorf("Timeout = %v, want 3m", got.Timeout)
	}
	if got.TokenDiscrepancyThreshold != 3.0 {
		t.Errorf("TokenDiscrepancyThreshold = %f, want 3.0", got.TokenDiscrepancyThreshold)
	}
}

// =============================================================================
// SetEventHandler Tests (extended)
// =============================================================================

func TestCopilotAdapter_SetEventHandler_AggregatorCreation(t *testing.T) {
	t.Parallel()
	c := &CopilotAdapter{}

	if c.aggregator != nil {
		t.Error("aggregator should be nil initially")
	}

	c.SetEventHandler(func(_ core.AgentEvent) {})
	if c.aggregator == nil {
		t.Error("aggregator should be created when handler is set")
	}

	// Setting handler again should reuse existing aggregator
	firstAgg := c.aggregator
	c.SetEventHandler(func(_ core.AgentEvent) {})
	if c.aggregator != firstAgg {
		t.Error("aggregator should be reused when already created")
	}

	// Setting nil handler should keep aggregator
	c.SetEventHandler(nil)
	if c.eventHandler != nil {
		t.Error("eventHandler should be nil after setting nil")
	}
	// aggregator is preserved
}

// =============================================================================
// Interface compliance Tests
// =============================================================================

func TestCopilotAdapter_ImplementsAgent(t *testing.T) {
	t.Parallel()
	var _ core.Agent = (*CopilotAdapter)(nil)
}

func TestCopilotAdapter_ImplementsStreamingCapable(t *testing.T) {
	t.Parallel()
	var _ core.StreamingCapable = (*CopilotAdapter)(nil)
}
