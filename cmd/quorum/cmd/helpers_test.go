package cmd

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// ---------------------------------------------------------------------------
// parseLogLevel (run.go) — previously untested
// ---------------------------------------------------------------------------

func TestParseLogLevel_Unknown(t *testing.T) {
	t.Parallel()
	if got := parseLogLevel("unknown"); got != slog.LevelInfo {
		t.Errorf("expected LevelInfo (default) for unknown string, got %d", got)
	}
}

func TestParseLogLevel_CaseSensitive(t *testing.T) {
	t.Parallel()
	// "Debug" (capitalized) should fall through to default
	if got := parseLogLevel("Debug"); got != slog.LevelInfo {
		t.Errorf("expected LevelInfo for capitalized 'Debug', got %d", got)
	}
}

// ---------------------------------------------------------------------------
// parseTraceConfig (run.go) — additional cases
// ---------------------------------------------------------------------------

func TestParseTraceConfig_SummaryFromConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "summary"},
	}
	trace, err := parseTraceConfig(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Mode != "summary" {
		t.Errorf("expected mode summary, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_FieldPassthrough(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{
			Mode:            "full",
			Dir:             "/tmp/traces",
			SchemaVersion:   2,
			Redact:          true,
			RedactPatterns:  []string{"secret.*"},
			RedactAllowlist: []string{"safe_field"},
			MaxBytes:        1024,
			TotalMaxBytes:   4096,
			MaxFiles:        10,
			IncludePhases:   []string{"analyze", "plan"},
		},
	}
	trace, err := parseTraceConfig(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Dir != "/tmp/traces" {
		t.Errorf("expected Dir '/tmp/traces', got %s", trace.Dir)
	}
	if trace.SchemaVersion != 2 {
		t.Errorf("expected SchemaVersion 2, got %d", trace.SchemaVersion)
	}
	if !trace.Redact {
		t.Error("expected Redact=true")
	}
	if len(trace.RedactPatterns) != 1 || trace.RedactPatterns[0] != "secret.*" {
		t.Errorf("unexpected RedactPatterns: %v", trace.RedactPatterns)
	}
	if len(trace.RedactAllowlist) != 1 || trace.RedactAllowlist[0] != "safe_field" {
		t.Errorf("unexpected RedactAllowlist: %v", trace.RedactAllowlist)
	}
	if trace.MaxBytes != 1024 {
		t.Errorf("expected MaxBytes 1024, got %d", trace.MaxBytes)
	}
	if trace.TotalMaxBytes != 4096 {
		t.Errorf("expected TotalMaxBytes 4096, got %d", trace.TotalMaxBytes)
	}
	if trace.MaxFiles != 10 {
		t.Errorf("expected MaxFiles 10, got %d", trace.MaxFiles)
	}
	if len(trace.IncludePhases) != 2 {
		t.Errorf("expected 2 IncludePhases, got %d", len(trace.IncludePhases))
	}
}

func TestParseTraceConfig_OffExplicit(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}
	trace, err := parseTraceConfig(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Mode != "off" {
		t.Errorf("expected mode off, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_OverrideSummaryToFull(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "summary"},
	}
	trace, err := parseTraceConfig(cfg, "full")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Mode != "full" {
		t.Errorf("expected override to full, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_OverrideToSummary(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}
	trace, err := parseTraceConfig(cfg, "summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Mode != "summary" {
		t.Errorf("expected override to summary, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_InvalidModeFromConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "verbose"},
	}
	_, err := parseTraceConfig(cfg, "")
	if err == nil {
		t.Fatal("expected error for invalid mode from config")
	}
}

// ---------------------------------------------------------------------------
// handleTUICompletion (run.go) — additional edge cases
// ---------------------------------------------------------------------------

func TestHandleTUICompletion_ChannelNilTUIErr_WorkflowErr(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1)
	ch <- nil // TUI completed without error
	wfErr := errors.New("workflow failed")
	err := handleTUICompletion(ch, wfErr)
	if err != wfErr {
		t.Errorf("expected workflow error, got %v", err)
	}
}

func TestHandleTUICompletion_ChannelNilTUIErr_NilWorkflowErr(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1)
	ch <- nil // TUI completed without error
	err := handleTUICompletion(ch, nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestHandleTUICompletion_ChannelNoReady_WorkflowErr(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1) // empty -- nothing ready
	wfErr := errors.New("workflow fail")
	err := handleTUICompletion(ch, wfErr)
	if err != wfErr {
		t.Errorf("expected workflow error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatStatus (workflows.go) — table-driven variant
// ---------------------------------------------------------------------------

func TestFormatStatus_AllCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    core.WorkflowStatus
		expected string
	}{
		{core.WorkflowStatusPending, "pending"},
		{core.WorkflowStatusRunning, "running"},
		{core.WorkflowStatusCompleted, "completed"},
		{core.WorkflowStatusFailed, "failed"},
		{core.WorkflowStatus("xyz"), "xyz"},
	}
	for _, tc := range cases {
		if got := formatStatus(tc.input); got != tc.expected {
			t.Errorf("formatStatus(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// formatPhase (workflows.go) — table-driven variant
// ---------------------------------------------------------------------------

func TestFormatPhase_AllCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    core.Phase
		expected string
	}{
		{core.PhaseRefine, "refine"},
		{core.PhaseAnalyze, "analyze"},
		{core.PhasePlan, "plan"},
		{core.PhaseExecute, "execute"},
		{core.Phase("other"), "other"},
	}
	for _, tc := range cases {
		if got := formatPhase(tc.input); got != tc.expected {
			t.Errorf("formatPhase(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// formatWorkflowTime (workflows.go) — additional edge case
// ---------------------------------------------------------------------------

func TestFormatWorkflowTime_SpecificDate(t *testing.T) {
	t.Parallel()
	tm := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC)
	expected := "2025-12-31 23:59"
	if got := formatWorkflowTime(tm); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// ---------------------------------------------------------------------------
// truncateString (workflows.go) — boundary and edge cases
// ---------------------------------------------------------------------------

func TestTruncateString_BoundaryExact(t *testing.T) {
	t.Parallel()
	// String of length exactly 4 with maxLen=4 -- should NOT truncate
	if got := truncateString("abcd", 4); got != "abcd" {
		t.Errorf("expected 'abcd', got %q", got)
	}
}

func TestTruncateString_OnePastLimit(t *testing.T) {
	t.Parallel()
	// String of length 5 with maxLen=4 -- should truncate to 1 char + "..."
	got := truncateString("abcde", 4)
	if got != "a..." {
		t.Errorf("expected 'a...', got %q", got)
	}
}

func TestTruncateString_NewlinesAndTruncation(t *testing.T) {
	t.Parallel()
	// Newlines are replaced first, then truncation happens
	got := truncateString("a\nb\nc\nd\ne\nf\ng", 8)
	// After newline replacement: "a b c d e f g" (len=13)
	// Truncated to 8-3=5 chars + "..."
	if got != "a b c..." {
		t.Errorf("expected 'a b c...', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// parseDurationDefault (common.go) — zero duration edge case
// ---------------------------------------------------------------------------

func TestParseDurationDefault_ZeroDuration(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("0s", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestParseDurationDefault_NanosecondPrecision(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("100ns", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 100*time.Nanosecond {
		t.Errorf("expected 100ns, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// phaseTimeoutValue (common.go) — verify empty phase config returns ""
// ---------------------------------------------------------------------------

func TestPhaseTimeoutValue_EmptyConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{}
	for _, phase := range []core.Phase{core.PhaseAnalyze, core.PhasePlan, core.PhaseExecute} {
		if got := phaseTimeoutValue(cfg, phase); got != "" {
			t.Errorf("phaseTimeoutValue(empty, %s) = %q; want empty", phase, got)
		}
	}
}

// ---------------------------------------------------------------------------
// buildBlueprint (common.go) — minimal/empty config
// ---------------------------------------------------------------------------

func TestBuildBlueprint_Defaults(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{}
	bp := buildBlueprint(cfg)
	if bp.ExecutionMode != "multi_agent" {
		t.Errorf("expected multi_agent for disabled single-agent, got %s", bp.ExecutionMode)
	}
	if bp.MaxRetries != 0 {
		t.Errorf("expected 0 max retries for zero-value config, got %d", bp.MaxRetries)
	}
	if bp.DryRun {
		t.Error("expected DryRun=false for zero-value config")
	}
	if bp.Timeout != 0 {
		t.Errorf("expected zero timeout for zero-value config, got %v", bp.Timeout)
	}
}

func TestBuildBlueprint_SingleAgentWithReasoningEffort(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{
		SingleAgent: workflow.SingleAgentConfig{
			Enabled:         true,
			Agent:           "codex",
			Model:           "codex-mini",
			ReasoningEffort: "high",
		},
	}
	bp := buildBlueprint(cfg)
	if bp.ExecutionMode != "single_agent" {
		t.Errorf("expected single_agent, got %s", bp.ExecutionMode)
	}
	if bp.SingleAgent.ReasoningEffort != "high" {
		t.Errorf("expected reasoning_effort=high, got %s", bp.SingleAgent.ReasoningEffort)
	}
}

func TestBuildBlueprint_ModeratorFields(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{
		Moderator: workflow.ModeratorConfig{
			Enabled:             true,
			Agent:               "claude",
			Threshold:           0.75,
			MinRounds:           2,
			MaxRounds:           5,
			WarningThreshold:    0.5,
			StagnationThreshold: 0.3,
		},
	}
	bp := buildBlueprint(cfg)
	if !bp.Consensus.Enabled {
		t.Error("expected consensus.enabled=true")
	}
	if bp.Consensus.MinRounds != 2 {
		t.Errorf("expected min_rounds=2, got %d", bp.Consensus.MinRounds)
	}
	if bp.Consensus.MaxRounds != 5 {
		t.Errorf("expected max_rounds=5, got %d", bp.Consensus.MaxRounds)
	}
	if bp.Consensus.WarningThreshold != 0.5 {
		t.Errorf("expected warning_threshold=0.5, got %f", bp.Consensus.WarningThreshold)
	}
	if bp.Consensus.StagnationThreshold != 0.3 {
		t.Errorf("expected stagnation_threshold=0.3, got %f", bp.Consensus.StagnationThreshold)
	}
}

// ---------------------------------------------------------------------------
// buildSingleAgentConfig (common.go) — additional config fallback cases
// ---------------------------------------------------------------------------

func TestBuildSingleAgentConfig_ConfigEnabled(t *testing.T) {
	// When singleAgent flag is false, config values pass through including Enabled.
	oldSA := singleAgent
	oldAN := agentName
	oldAM := agentModel
	defer func() {
		singleAgent = oldSA
		agentName = oldAN
		agentModel = oldAM
	}()
	singleAgent = false
	agentName = ""
	agentModel = ""

	cfg := &config.Config{
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				SingleAgent: config.SingleAgentConfig{
					Enabled: true,
					Agent:   "gemini",
					Model:   "gemini-2.5-flash",
				},
			},
		},
	}

	result := buildSingleAgentConfig(cfg)
	if !result.Enabled {
		t.Error("expected Enabled=true from config fallback")
	}
	if result.Agent != "gemini" {
		t.Errorf("expected agent 'gemini', got %q", result.Agent)
	}
	if result.Model != "gemini-2.5-flash" {
		t.Errorf("expected model 'gemini-2.5-flash', got %q", result.Model)
	}
}

func TestBuildSingleAgentConfig_FlagOverridesConfigEnabled(t *testing.T) {
	// Even if config has enabled=true, the CLI flag wins.
	oldSA := singleAgent
	oldAN := agentName
	oldAM := agentModel
	defer func() {
		singleAgent = oldSA
		agentName = oldAN
		agentModel = oldAM
	}()
	singleAgent = true
	agentName = "codex"
	agentModel = "codex-mini"

	cfg := &config.Config{
		Phases: config.PhasesConfig{
			Analyze: config.AnalyzePhaseConfig{
				SingleAgent: config.SingleAgentConfig{
					Enabled: true,
					Agent:   "gemini",
					Model:   "gemini-pro",
				},
			},
		},
	}

	result := buildSingleAgentConfig(cfg)
	if !result.Enabled {
		t.Error("expected Enabled=true")
	}
	if result.Agent != "codex" {
		t.Errorf("expected agent 'codex' from flag, got %q", result.Agent)
	}
	if result.Model != "codex-mini" {
		t.Errorf("expected model 'codex-mini' from flag, got %q", result.Model)
	}
}

// ---------------------------------------------------------------------------
// countCompletedTasks (execute.go) — additional edge cases
// ---------------------------------------------------------------------------

func TestCountCompletedTasks_AllCompleted(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Status: core.TaskStatusCompleted},
				"t2": {ID: "t2", Status: core.TaskStatusCompleted},
			},
		},
	}
	if got := countCompletedTasks(state); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestCountCompletedTasks_NoneCompleted(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Status: core.TaskStatusPending},
				"t2": {ID: "t2", Status: core.TaskStatusFailed},
				"t3": {ID: "t3", Status: core.TaskStatusSkipped},
			},
		},
	}
	if got := countCompletedTasks(state); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// getPrompt (run.go) — additional edge cases
// ---------------------------------------------------------------------------

func TestGetPrompt_FilePrecedence(t *testing.T) {
	t.Parallel()
	// When both args and file are empty, error is returned
	_, err := getPrompt(nil, "")
	if err == nil {
		t.Fatal("expected error for nil args and empty file")
	}
}

func TestGetPrompt_EmptyArgs(t *testing.T) {
	t.Parallel()
	_, err := getPrompt([]string{}, "")
	if err == nil {
		t.Fatal("expected error for empty args slice")
	}
}

// ---------------------------------------------------------------------------
// validateSingleAgentFlags (common.go) — ensure reset behavior is correct
// ---------------------------------------------------------------------------

func TestValidateSingleAgentFlags_AllEmpty(t *testing.T) {
	oldSA := singleAgent
	oldAN := agentName
	oldAM := agentModel
	defer func() {
		singleAgent = oldSA
		agentName = oldAN
		agentModel = oldAM
	}()
	singleAgent = false
	agentName = ""
	agentModel = ""

	if err := validateSingleAgentFlags(); err != nil {
		t.Errorf("expected no error for all-empty flags, got: %v", err)
	}
}
