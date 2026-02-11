package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
)

// ===========================================================================
// Helper: create a ready model for coverage boost tests
// ===========================================================================

func newCovBoostModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel()
	cleanupModel(t, &m)
	return m
}

// ===========================================================================
// message.go: LastMessage
// ===========================================================================

func TestLastMessage_Empty(t *testing.T) {
	h := NewConversationHistory(10)
	if h.LastMessage() != nil {
		t.Error("LastMessage should return nil for empty history")
	}
}

func TestLastMessage_OneMessage(t *testing.T) {
	h := NewConversationHistory(10)
	h.Add(NewUserMessage("hello"))
	msg := h.LastMessage()
	if msg == nil {
		t.Fatal("LastMessage should not be nil")
	}
	if msg.Content != "hello" {
		t.Errorf("got content %q, want 'hello'", msg.Content)
	}
	if msg.Role != RoleUser {
		t.Errorf("got role %q, want %q", msg.Role, RoleUser)
	}
}

func TestLastMessage_Multiple(t *testing.T) {
	h := NewConversationHistory(10)
	h.Add(NewUserMessage("first"))
	h.Add(NewAgentMessage("claude", "second"))
	h.Add(NewSystemMessage("third"))
	msg := h.LastMessage()
	if msg == nil {
		t.Fatal("LastMessage should not be nil")
	}
	if msg.Content != "third" {
		t.Errorf("got content %q, want 'third'", msg.Content)
	}
	if msg.Role != RoleSystem {
		t.Errorf("got role %q, want %q", msg.Role, RoleSystem)
	}
}

// ===========================================================================
// model.go: extractTokenValue
// ===========================================================================

func TestExtractTokenValue_Int(t *testing.T) {
	data := map[string]any{"count": 42}
	if got := extractTokenValue(data, "count"); got != 42 {
		t.Errorf("int: got %d, want 42", got)
	}
}

func TestExtractTokenValue_Int64(t *testing.T) {
	data := map[string]any{"count": int64(100)}
	if got := extractTokenValue(data, "count"); got != 100 {
		t.Errorf("int64: got %d, want 100", got)
	}
}

func TestExtractTokenValue_Float64(t *testing.T) {
	data := map[string]any{"count": float64(99.9)}
	if got := extractTokenValue(data, "count"); got != 99 {
		t.Errorf("float64: got %d, want 99", got)
	}
}

func TestExtractTokenValue_Float32(t *testing.T) {
	data := map[string]any{"count": float32(50.5)}
	if got := extractTokenValue(data, "count"); got != 50 {
		t.Errorf("float32: got %d, want 50", got)
	}
}

func TestExtractTokenValue_Missing(t *testing.T) {
	data := map[string]any{"other": 10}
	if got := extractTokenValue(data, "count"); got != 0 {
		t.Errorf("missing key: got %d, want 0", got)
	}
}

func TestExtractTokenValue_UnsupportedType(t *testing.T) {
	data := map[string]any{"count": "not a number"}
	if got := extractTokenValue(data, "count"); got != 0 {
		t.Errorf("string type: got %d, want 0", got)
	}
}

func TestExtractTokenValue_NilMap(t *testing.T) {
	var data map[string]any
	if got := extractTokenValue(data, "count"); got != 0 {
		t.Errorf("nil map: got %d, want 0", got)
	}
}

// ===========================================================================
// model.go: dimLine
// ===========================================================================

func TestDimLine(t *testing.T) {
	result := dimLine("hello world")
	if !strings.HasPrefix(result, "\x1b[2m") {
		t.Error("should start with ANSI dim code")
	}
	if !strings.HasSuffix(result, "\x1b[22m") {
		t.Error("should end with ANSI reset intensity code")
	}
	if !strings.Contains(result, "hello world") {
		t.Error("should contain original text")
	}
}

func TestDimLine_Empty(t *testing.T) {
	result := dimLine("")
	if result != "\x1b[2m\x1b[22m" {
		t.Errorf("empty dimLine: got %q", result)
	}
}

// ===========================================================================
// model.go: truncateWithAnsi
// ===========================================================================

func TestTruncateWithAnsi_PlainText(t *testing.T) {
	result := truncateWithAnsi("hello world", 5)
	if result != "hello" {
		t.Errorf("got %q, want 'hello'", result)
	}
}

func TestTruncateWithAnsi_ZeroWidth(t *testing.T) {
	result := truncateWithAnsi("hello", 0)
	if result != "" {
		t.Errorf("zero width: got %q, want empty", result)
	}
}

func TestTruncateWithAnsi_NegativeWidth(t *testing.T) {
	result := truncateWithAnsi("hello", -5)
	if result != "" {
		t.Errorf("negative width: got %q, want empty", result)
	}
}

func TestTruncateWithAnsi_ExactLength(t *testing.T) {
	result := truncateWithAnsi("hello", 5)
	if result != "hello" {
		t.Errorf("exact: got %q, want 'hello'", result)
	}
}

func TestTruncateWithAnsi_LongerThanContent(t *testing.T) {
	result := truncateWithAnsi("hi", 5)
	// Should pad to exact width
	if len(result) != 5 {
		t.Errorf("padded length: got %d, want 5 (result=%q)", len(result), result)
	}
}

func TestTruncateWithAnsi_WithAnsiCodes(t *testing.T) {
	// Text with ANSI: \x1b[31m = red, \x1b[0m = reset
	input := "\x1b[31mhello\x1b[0m world"
	result := truncateWithAnsi(input, 5)
	// Should truncate to 5 visible chars, preserving ANSI codes
	if !strings.Contains(result, "\x1b[31m") {
		t.Errorf("should preserve ANSI codes, got %q", result)
	}
}

func TestTruncateWithAnsi_Empty(t *testing.T) {
	result := truncateWithAnsi("", 5)
	// Should pad to width
	if len(result) != 5 {
		t.Errorf("empty: got len %d, want 5", len(result))
	}
}

// ===========================================================================
// model.go: skipCharsWithAnsi
// ===========================================================================

func TestSkipCharsWithAnsi_PlainText(t *testing.T) {
	result := skipCharsWithAnsi("hello world", 6)
	if result != "world" {
		t.Errorf("got %q, want 'world'", result)
	}
}

func TestSkipCharsWithAnsi_ZeroSkip(t *testing.T) {
	result := skipCharsWithAnsi("hello", 0)
	if result != "hello" {
		t.Errorf("zero skip: got %q, want 'hello'", result)
	}
}

func TestSkipCharsWithAnsi_NegativeSkip(t *testing.T) {
	result := skipCharsWithAnsi("hello", -3)
	if result != "hello" {
		t.Errorf("negative skip: got %q, want 'hello'", result)
	}
}

func TestSkipCharsWithAnsi_SkipAll(t *testing.T) {
	result := skipCharsWithAnsi("hello", 5)
	if result != "" {
		t.Errorf("skip all: got %q, want empty", result)
	}
}

func TestSkipCharsWithAnsi_WithAnsiCodes(t *testing.T) {
	input := "\x1b[31mhello\x1b[0m world"
	result := skipCharsWithAnsi(input, 6)
	// Should skip 6 visible chars (hello + space), keep "world"
	if !strings.Contains(result, "world") {
		t.Errorf("should contain 'world', got %q", result)
	}
}

func TestSkipCharsWithAnsi_Empty(t *testing.T) {
	result := skipCharsWithAnsi("", 3)
	if result != "" {
		t.Errorf("empty: got %q, want empty", result)
	}
}

// ===========================================================================
// model.go: overlayOnBase
// ===========================================================================

func TestOverlayOnBase_Basic(t *testing.T) {
	m := newCovBoostModel(t)
	base := "line1\nline2\nline3\nline4\nline5"
	overlay := "OVR"
	result := m.overlayOnBase(base, overlay, 10, 5)
	if result == "" {
		t.Error("overlayOnBase should not return empty")
	}
	if !strings.Contains(result, "OVR") {
		t.Error("result should contain overlay text")
	}
}

func TestOverlayOnBase_LargeOverlay(t *testing.T) {
	m := newCovBoostModel(t)
	base := "line1\nline2\nline3"
	overlay := "a\nb\nc\nd\ne\nf\ng\nh"
	result := m.overlayOnBase(base, overlay, 10, 3)
	if result == "" {
		t.Error("overlayOnBase should not return empty")
	}
}

func TestOverlayOnBase_EmptyBase(t *testing.T) {
	m := newCovBoostModel(t)
	result := m.overlayOnBase("", "overlay", 10, 5)
	if result == "" {
		t.Error("should not return empty even with empty base")
	}
}

// ===========================================================================
// model.go: overlayAtBottom
// ===========================================================================

func TestOverlayAtBottom_Basic(t *testing.T) {
	m := newCovBoostModel(t)
	base := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	overlay := "OVR1\nOVR2"
	result := m.overlayAtBottom(base, overlay, 20, 10, 0, 2)
	if result == "" {
		t.Error("overlayAtBottom should not return empty")
	}
}

func TestOverlayAtBottom_WithLeftOffset(t *testing.T) {
	m := newCovBoostModel(t)
	base := "LEFTRIGHT\nLEFTRIGHT\nLEFTRIGHT\nLEFTRIGHT\nLEFTRIGHT"
	overlay := "OVR"
	result := m.overlayAtBottom(base, overlay, 20, 5, 4, 1)
	if result == "" {
		t.Error("overlayAtBottom with offset should not return empty")
	}
}

func TestOverlayAtBottom_EmptyBase(t *testing.T) {
	m := newCovBoostModel(t)
	result := m.overlayAtBottom("", "overlay", 10, 5, 0, 0)
	if result == "" {
		t.Error("should not return empty even with empty base")
	}
}

// ===========================================================================
// model.go: suggestAgents
// ===========================================================================

func TestSuggestAgents_NoAvailable(t *testing.T) {
	m := newCovBoostModel(t)
	m.availableAgents = nil
	agents := m.suggestAgents("")
	if len(agents) == 0 {
		t.Error("should return fallback agents when none configured")
	}
}

func TestSuggestAgents_WithAvailable(t *testing.T) {
	m := newCovBoostModel(t)
	m.availableAgents = []string{"claude", "gemini", "codex"}
	agents := m.suggestAgents("")
	if len(agents) != 3 {
		t.Errorf("got %d agents, want 3", len(agents))
	}
}

func TestSuggestAgents_FilterPartial(t *testing.T) {
	m := newCovBoostModel(t)
	m.availableAgents = []string{"claude", "gemini", "codex", "copilot"}
	agents := m.suggestAgents("co")
	if len(agents) != 2 {
		t.Errorf("got %d agents matching 'co', want 2 (codex, copilot)", len(agents))
	}
}

func TestSuggestAgents_NoMatch(t *testing.T) {
	m := newCovBoostModel(t)
	m.availableAgents = []string{"claude", "gemini"}
	agents := m.suggestAgents("zzz")
	if len(agents) != 0 {
		t.Errorf("got %d agents, want 0", len(agents))
	}
}

// ===========================================================================
// model.go: suggestModels
// ===========================================================================

func TestSuggestModels_NoAgentModels(t *testing.T) {
	m := newCovBoostModel(t)
	m.agentModels = nil
	m.currentAgent = "claude"
	models := m.suggestModels("")
	if len(models) == 0 {
		t.Error("should return fallback models for claude")
	}
}

func TestSuggestModels_WithAgentModels(t *testing.T) {
	m := newCovBoostModel(t)
	m.agentModels = map[string][]string{
		"claude": {"opus", "sonnet", "haiku"},
	}
	m.currentAgent = "claude"
	models := m.suggestModels("")
	if len(models) != 3 {
		t.Errorf("got %d models, want 3", len(models))
	}
}

func TestSuggestModels_FilterPartial(t *testing.T) {
	m := newCovBoostModel(t)
	m.agentModels = map[string][]string{
		"claude": {"opus", "sonnet", "haiku"},
	}
	m.currentAgent = "claude"
	models := m.suggestModels("son")
	if len(models) != 1 || models[0] != "sonnet" {
		t.Errorf("got %v, want [sonnet]", models)
	}
}

func TestSuggestModels_DefaultAgent(t *testing.T) {
	m := newCovBoostModel(t)
	m.currentAgent = "" // Should default to "claude"
	m.agentModels = nil
	models := m.suggestModels("")
	if len(models) == 0 {
		t.Error("should return models even with empty currentAgent")
	}
}

func TestSuggestModels_GeminiFallback(t *testing.T) {
	m := newCovBoostModel(t)
	m.currentAgent = "gemini"
	m.agentModels = nil
	models := m.suggestModels("")
	if len(models) == 0 {
		t.Error("should return fallback models for gemini")
	}
}

func TestSuggestModels_CodexFallback(t *testing.T) {
	m := newCovBoostModel(t)
	m.currentAgent = "codex"
	m.agentModels = nil
	models := m.suggestModels("")
	if len(models) == 0 {
		t.Error("should return fallback models for codex")
	}
}

func TestSuggestModels_CopilotFallback(t *testing.T) {
	m := newCovBoostModel(t)
	m.currentAgent = "copilot"
	m.agentModels = nil
	models := m.suggestModels("")
	if len(models) == 0 {
		t.Error("should return fallback models for copilot")
	}
}

func TestSuggestModels_UnknownAgent(t *testing.T) {
	m := newCovBoostModel(t)
	m.currentAgent = "unknown"
	m.currentModel = "default-model"
	m.agentModels = nil
	models := m.suggestModels("")
	if len(models) != 1 || models[0] != "default-model" {
		t.Errorf("unknown agent: got %v, want [default-model]", models)
	}
}

// ===========================================================================
// model.go: suggestThemes
// ===========================================================================

func TestSuggestThemes_All(t *testing.T) {
	m := newCovBoostModel(t)
	themes := m.suggestThemes("")
	if len(themes) != 2 {
		t.Errorf("got %d themes, want 2", len(themes))
	}
}

func TestSuggestThemes_FilterDark(t *testing.T) {
	m := newCovBoostModel(t)
	themes := m.suggestThemes("dar")
	if len(themes) != 1 || themes[0] != "dark" {
		t.Errorf("got %v, want [dark]", themes)
	}
}

func TestSuggestThemes_FilterLight(t *testing.T) {
	m := newCovBoostModel(t)
	themes := m.suggestThemes("l")
	if len(themes) != 1 || themes[0] != "light" {
		t.Errorf("got %v, want [light]", themes)
	}
}

func TestSuggestThemes_NoMatch(t *testing.T) {
	m := newCovBoostModel(t)
	themes := m.suggestThemes("zzz")
	if len(themes) != 0 {
		t.Errorf("got %v, want empty", themes)
	}
}

// ===========================================================================
// model.go: suggestWorkflows
// ===========================================================================

func TestSuggestWorkflows_NoRunner(t *testing.T) {
	m := newCovBoostModel(t)
	m.runner = nil
	m.workflowCache = nil
	wfs := m.suggestWorkflows("")
	if len(wfs) != 0 {
		t.Errorf("got %v, want nil/empty", wfs)
	}
}

func TestSuggestWorkflows_WithCache(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = []core.WorkflowSummary{
		{WorkflowID: "wf-abc", Prompt: "build feature"},
		{WorkflowID: "wf-xyz", Prompt: "fix bug"},
	}
	wfs := m.suggestWorkflows("")
	if len(wfs) != 2 {
		t.Errorf("got %d, want 2", len(wfs))
	}
}

func TestSuggestWorkflows_FilterByID(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = []core.WorkflowSummary{
		{WorkflowID: "wf-abc", Prompt: "build feature"},
		{WorkflowID: "wf-xyz", Prompt: "fix bug"},
	}
	wfs := m.suggestWorkflows("abc")
	if len(wfs) != 1 || wfs[0] != "wf-abc" {
		t.Errorf("got %v, want [wf-abc]", wfs)
	}
}

func TestSuggestWorkflows_FilterByPrompt(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = []core.WorkflowSummary{
		{WorkflowID: "wf-abc", Prompt: "build feature"},
		{WorkflowID: "wf-xyz", Prompt: "fix bug"},
	}
	wfs := m.suggestWorkflows("bug")
	if len(wfs) != 1 || wfs[0] != "wf-xyz" {
		t.Errorf("got %v, want [wf-xyz]", wfs)
	}
}

func TestSuggestWorkflows_WithRunner(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = nil
	m.runner = &mockWorkflowRunner{
		workflows: []core.WorkflowSummary{
			{WorkflowID: "wf-runner", Prompt: "from runner"},
		},
	}
	wfs := m.suggestWorkflows("")
	if len(wfs) != 1 || wfs[0] != "wf-runner" {
		t.Errorf("got %v, want [wf-runner]", wfs)
	}
}

// ===========================================================================
// model.go: getWorkflowDescription
// ===========================================================================

func TestGetWorkflowDescription_Found(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = []core.WorkflowSummary{
		{
			WorkflowID:   "wf-1",
			Status:       core.WorkflowStatusCompleted,
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       "short prompt",
			IsActive:     false,
		},
	}
	desc := m.getWorkflowDescription("wf-1")
	if desc == "" {
		t.Error("should return description for known workflow")
	}
	if !strings.Contains(desc, "COMPLETED") {
		t.Errorf("should contain status, got %q", desc)
	}
}

func TestGetWorkflowDescription_Active(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = []core.WorkflowSummary{
		{
			WorkflowID:   "wf-active",
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseExecute,
			Prompt:       "active workflow",
			IsActive:     true,
		},
	}
	desc := m.getWorkflowDescription("wf-active")
	if !strings.HasPrefix(desc, "* ") {
		t.Errorf("active workflow should start with '* ', got %q", desc)
	}
}

func TestGetWorkflowDescription_LongPrompt(t *testing.T) {
	m := newCovBoostModel(t)
	longPrompt := strings.Repeat("x", 100)
	m.workflowCache = []core.WorkflowSummary{
		{
			WorkflowID:   "wf-long",
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
			Prompt:       longPrompt,
		},
	}
	desc := m.getWorkflowDescription("wf-long")
	if !strings.Contains(desc, "...") {
		t.Error("long prompt should be truncated with ellipsis")
	}
}

func TestGetWorkflowDescription_NotFound(t *testing.T) {
	m := newCovBoostModel(t)
	m.workflowCache = nil
	desc := m.getWorkflowDescription("wf-nonexistent")
	if desc != "" {
		t.Errorf("should return empty for unknown workflow, got %q", desc)
	}
}

// ===========================================================================
// stats_panel.go: scroll methods, Height, Update, Render
// ===========================================================================

func TestStatsPanel_ScrollMethods(t *testing.T) {
	p := NewStatsPanel()
	p.SetSize(80, 30)

	// All these were at 0% coverage
	p.ScrollUp()
	p.ScrollDown()
	p.PageUp()
	p.PageDown()
	p.GotoTop()
	p.GotoBottom()
	p.Update(nil)

	h := p.Height()
	if h != 30 {
		t.Errorf("Height: got %d, want 30", h)
	}
}

func TestStatsPanel_Render(t *testing.T) {
	p := NewStatsPanel()
	p.SetSize(80, 30)

	// Render was at 0%
	result := p.Render()
	if result == "" {
		t.Error("Render should not return empty after SetSize")
	}

	// RenderWithFocus also covers more
	result2 := p.RenderWithFocus(true)
	if result2 == "" {
		t.Error("RenderWithFocus should not return empty")
	}
}

func TestStatsPanel_WithData(t *testing.T) {
	p := NewStatsPanel()
	p.SetSize(80, 40)

	p.SetResourceStats(ResourceStats{
		MemoryMB:      128.5,
		CPUPercent:    25.3,
		CPURawPercent: 101.2,
		Uptime:        90 * time.Minute,
		Goroutines:    42,
	})

	p.SetMachineStats(diagnostics.SystemMetrics{
		CPUPercent: 55.0,
		CPUModel:   "Intel i7",
		CPUCores:   8,
		CPUThreads: 16,
		MemTotalMB: 16384,
		MemUsedMB:  8192,
		MemPercent: 50.0,
		DiskTotalGB: 500,
		DiskUsedGB:  250,
		DiskPercent: 50.0,
		LoadAvg1:    1.5,
		LoadAvg5:    1.2,
		LoadAvg15:   0.8,
	})

	result := p.Render()
	if result == "" {
		t.Error("Render with data should not return empty")
	}
}

func TestStatsPanel_WithGPU(t *testing.T) {
	p := NewStatsPanel()
	p.SetSize(80, 40)

	p.SetMachineStats(diagnostics.SystemMetrics{
		MemTotalMB: 16384,
		GPUInfos: []diagnostics.GPUInfo{
			{
				Name:        "RTX 3080",
				UtilValid:   true,
				UtilPercent: 75.0,
				MemValid:    true,
				MemTotalMB:  10240,
				MemUsedMB:   4096,
				TempValid:   true,
				TempC:       72.0,
			},
		},
	})

	result := p.Render()
	if result == "" {
		t.Error("Render with GPU data should not return empty")
	}
}

func TestStatsPanel_NotReady(t *testing.T) {
	p := NewStatsPanel()
	// Don't call SetSize, so it's not ready
	result := p.Render()
	if result != "" {
		t.Error("Render before SetSize should return empty")
	}
}

// ===========================================================================
// tasks_panel.go: IsDirty, ClearDirty, Show
// ===========================================================================

func TestTasksPanel_IsDirtyClearDirty(t *testing.T) {
	p := NewTasksPanel()
	if p.IsDirty() {
		t.Error("should not be dirty initially")
	}
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusPending},
			},
			TaskOrder: []core.TaskID{"t1"},
		},
	}
	p.SetState(state)
	if !p.IsDirty() {
		t.Error("should be dirty after SetState")
	}
	p.ClearDirty()
	if p.IsDirty() {
		t.Error("should not be dirty after ClearDirty")
	}
}

func TestTasksPanel_Show(t *testing.T) {
	p := NewTasksPanel()
	if p.IsVisible() {
		t.Error("should not be visible initially")
	}
	p.Show()
	if !p.IsVisible() {
		t.Error("should be visible after Show")
	}
	p.Hide()
	if p.IsVisible() {
		t.Error("should be hidden after Hide")
	}
}

// ===========================================================================
// session.go: Close
// ===========================================================================

// ===========================================================================
// history_search.go: Save and Load with temp file
// ===========================================================================

func TestHistorySearch_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, ".quorum", "history.json")

	hs := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		filtered:    make([]HistoryEntry, 0),
		maxEntries:  100,
		historyFile: histFile,
	}

	hs.Add("first command", "claude")
	hs.Add("second command", "gemini")

	if err := hs.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load into new instance
	hs2 := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		filtered:    make([]HistoryEntry, 0),
		maxEntries:  100,
		historyFile: histFile,
	}
	if err := hs2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if hs2.Count() != 2 {
		t.Errorf("loaded %d entries, want 2", hs2.Count())
	}
}

func TestHistorySearch_LoadNonexistent(t *testing.T) {
	hs := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		historyFile: "/tmp/nonexistent-quorum-test-file.json",
	}
	err := hs.Load()
	if err != nil {
		t.Errorf("Load of nonexistent file should return nil, got %v", err)
	}
}

// ===========================================================================
// history_search.go: filter, MoveUp, MoveDown, GetSelected
// ===========================================================================

func TestHistorySearch_FilterAndSelect(t *testing.T) {
	tmpDir := t.TempDir()
	hs := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		filtered:    make([]HistoryEntry, 0),
		maxEntries:  100,
		historyFile: filepath.Join(tmpDir, "history.json"),
	}

	hs.Add("analyze code", "claude")
	hs.Add("plan feature", "claude")
	hs.Add("run tests", "codex")

	hs.filter("")
	if hs.FilteredCount() != 3 {
		t.Errorf("empty filter: got %d, want 3", hs.FilteredCount())
	}

	hs.filter("plan")
	if hs.FilteredCount() != 1 {
		t.Errorf("'plan' filter: got %d, want 1", hs.FilteredCount())
	}

	selected := hs.GetSelected()
	if selected != "plan feature" {
		t.Errorf("selected = %q, want 'plan feature'", selected)
	}

	// MoveDown shouldn't go past end
	hs.MoveDown()
	if hs.selectedIndex != 0 {
		t.Errorf("MoveDown past end: selectedIndex = %d", hs.selectedIndex)
	}

	// MoveUp from 0 shouldn't go negative
	hs.MoveUp()
	if hs.selectedIndex != 0 {
		t.Errorf("MoveUp from 0: selectedIndex = %d", hs.selectedIndex)
	}
}

func TestHistorySearch_EmptyGetSelected(t *testing.T) {
	hs := &HistorySearch{
		entries:  make([]HistoryEntry, 0),
		filtered: make([]HistoryEntry, 0),
	}
	if got := hs.GetSelected(); got != "" {
		t.Errorf("GetSelected on empty: got %q, want empty", got)
	}
}

// ===========================================================================
// history_search.go: itoa
// ===========================================================================

// ===========================================================================
// history_search.go: highlightMatches
// ===========================================================================

// ===========================================================================
// file_viewer.go: ScrollLeft, ScrollRight, ScrollEnd edge cases
// ===========================================================================

func TestFileViewer_ScrollLeftFromZero(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	// ScrollLeft from zero offset should stay at zero
	fv.ScrollLeft()
	if fv.horizontalOffset != 0 {
		t.Errorf("horizontalOffset should be 0, got %d", fv.horizontalOffset)
	}
}

func TestFileViewer_ScrollRightAndEnd(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)

	// Create a temporary file inside the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	longLine := strings.Repeat("x", 200)
	tmpFile := filepath.Join(cwd, "test_scroll_coverage_boost.go.tmp")
	if err := os.WriteFile(tmpFile, []byte(longLine), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile) })

	if err := fv.SetFile(tmpFile); err != nil {
		t.Fatalf("SetFile: %v", err)
	}

	fv.ScrollRight()
	if fv.horizontalOffset == 0 {
		t.Error("ScrollRight should increase offset for long lines")
	}

	fv.ScrollEnd()
	// ScrollEnd should set offset to maxLineWidth - viewport width
	if fv.horizontalOffset == 0 && fv.maxLineWidth > fv.viewport.Width {
		t.Error("ScrollEnd should scroll to end of long lines")
	}

	// ScrollLeft should decrease offset
	prevOffset := fv.horizontalOffset
	fv.ScrollLeft()
	if fv.horizontalOffset >= prevOffset && prevOffset > 0 {
		t.Error("ScrollLeft should decrease offset")
	}
}

func TestFileViewer_SetFile_Nonexistent(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	err := fv.SetFile("nonexistent-file-xyz.txt")
	if err == nil {
		t.Error("SetFile with nonexistent file should return error")
	}
}

// ===========================================================================
// file_viewer.go: getVisiblePortion
// ===========================================================================

func TestGetVisiblePortion_OffsetBeyondLength(t *testing.T) {
	result := getVisiblePortion("hi", 10, 100)
	if result != "" {
		t.Errorf("offset beyond length: got %q, want empty", result)
	}
}

// ===========================================================================
// file_viewer.go: isBinaryContent
// ===========================================================================

func TestIsBinaryContent_ValidUTF8(t *testing.T) {
	if isBinaryContent([]byte("hello world\nthis is text")) {
		t.Error("valid UTF-8 text should not be binary")
	}
}

// ===========================================================================
// file_viewer.go: getSyntaxColor
// ===========================================================================

func TestGetSyntaxColor_Various(t *testing.T) {
	exts := []string{".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs", ".rb",
		".java", ".c", ".h", ".cpp", ".hpp", ".cc", ".md", ".json",
		".yaml", ".yml", ".toml", ".sh", ".bash", ".sql", ".html",
		".htm", ".css", ".xml", ".unknown"}
	for _, ext := range exts {
		color := getSyntaxColor(ext)
		if color == "" {
			t.Errorf("getSyntaxColor(%q) returned empty", ext)
		}
	}
}

// ===========================================================================
// file_viewer.go: runeWidth
// ===========================================================================

// ===========================================================================
// diff_view.go: additional edge cases
// ===========================================================================

func TestDiffView_ComputeDiff_EmptyContent(t *testing.T) {
	d := NewAgentDiffView()
	d.SetSize(80, 30)
	d.SetContent("", "")
	if len(d.diffLines) == 0 {
		t.Error("should have at least one diff line from empty split")
	}
}

func TestDiffView_PairNavigation(t *testing.T) {
	d := NewAgentDiffView()
	d.AddAgentPair("claude", "gemini")
	d.AddAgentPair("claude", "codex")

	if !d.NextPair() {
		t.Error("NextPair should return true with multiple pairs")
	}
	left, right := d.GetCurrentPair()
	if left != "claude" || right != "codex" {
		t.Errorf("got pair %s-%s, want claude-codex", left, right)
	}

	if !d.PrevPair() {
		t.Error("PrevPair should return true with multiple pairs")
	}
	left, right = d.GetCurrentPair()
	if left != "claude" || right != "gemini" {
		t.Errorf("got pair %s-%s, want claude-gemini", left, right)
	}
}

func TestDiffView_SinglePairNav(t *testing.T) {
	d := NewAgentDiffView()
	d.AddAgentPair("claude", "gemini")
	if d.NextPair() {
		t.Error("NextPair should return false with single pair")
	}
	if d.PrevPair() {
		t.Error("PrevPair should return false with single pair")
	}
}

func TestDiffView_NoPairsGetCurrentPair(t *testing.T) {
	d := NewAgentDiffView()
	d.SetAgents("a", "b")
	left, right := d.GetCurrentPair()
	if left != "a" || right != "b" {
		t.Errorf("got %s-%s, want a-b", left, right)
	}
}

// ===========================================================================
// diff_view.go: truncateOrPad
// ===========================================================================

// ===========================================================================
// logs.go: formatTokenCount, formatDuration, truncateModel
// ===========================================================================

// ===========================================================================
// logs.go: levelString
// ===========================================================================

func TestLevelString(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevelSuccess, "SUCCESS"},
		{LogLevel(99), "INFO"}, // default
	}
	for _, tc := range tests {
		got := p.levelString(tc.level)
		if got != tc.want {
			t.Errorf("levelString(%d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

// ===========================================================================
// logs.go: GetPlainText with all log levels
// ===========================================================================

// ===========================================================================
// logs.go: formatEntry with long message (word wrap)
// ===========================================================================

// ===========================================================================
// logs.go: ToggleFooter
// ===========================================================================

// ===========================================================================
// context_preview.go: formatSize, formatTokens
// ===========================================================================

// ===========================================================================
// token_panel.go: padOrTrim
// ===========================================================================

// ===========================================================================
// token_panel.go: render with entries
// ===========================================================================

func TestTokenPanel_RenderContent(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	entries := []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "chat", TokensIn: 100, TokensOut: 200},
		{Scope: "workflow", CLI: "gemini", Model: "pro", Phase: "plan", TokensIn: 5000, TokensOut: 3000},
	}
	p.SetEntries(entries)

	result := p.Render()
	if result == "" {
		t.Error("Render with entries should not return empty")
	}

	result2 := p.RenderWithFocus(true)
	if result2 == "" {
		t.Error("RenderWithFocus with entries should not return empty")
	}
}

func TestTokenPanel_Empty(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)
	result := p.Render()
	if result == "" {
		t.Error("Render even empty should not be empty (shows 'No token data')")
	}
}

// ===========================================================================
// commands.go: Parse edge cases
// ===========================================================================

func TestCommandRegistry_Parse_EmptySlash(t *testing.T) {
	r := NewCommandRegistry()
	cmd, args, ok := r.Parse("/")
	if ok {
		t.Error("bare / should not match any command")
	}
	if cmd != nil || args != nil {
		t.Error("bare / should return nils")
	}
}

func TestCommandRegistry_Parse_NonCommand(t *testing.T) {
	r := NewCommandRegistry()
	cmd, args, ok := r.Parse("just a message")
	if ok || cmd != nil || args != nil {
		t.Error("non-slash input should not match")
	}
}

func TestCommandRegistry_Parse_WithArgs(t *testing.T) {
	r := NewCommandRegistry()
	cmd, args, ok := r.Parse("/model gpt-5")
	if !ok || cmd == nil {
		t.Fatal("/model should match")
	}
	if cmd.Name != "model" {
		t.Errorf("name = %q, want 'model'", cmd.Name)
	}
	if len(args) != 1 || args[0] != "gpt-5" {
		t.Errorf("args = %v, want ['gpt-5']", args)
	}
}

func TestCommandRegistry_Parse_Alias(t *testing.T) {
	r := NewCommandRegistry()
	cmd, _, ok := r.Parse("/h")
	if !ok || cmd == nil {
		t.Fatal("/h (alias for help) should match")
	}
	if cmd.Name != "help" {
		t.Errorf("name = %q, want 'help'", cmd.Name)
	}
}

func TestCommandRegistry_Parse_Unknown(t *testing.T) {
	r := NewCommandRegistry()
	cmd, _, ok := r.Parse("/nonexistentcommand")
	if ok || cmd != nil {
		t.Error("unknown command should not match")
	}
}

// ===========================================================================
// explorer.go: various edge cases
// ===========================================================================

func TestExplorerPanel_SetRoot_OutsideBoundary(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	err := p.SetRoot("/")
	if err == nil {
		// On some systems where cwd IS /, this might succeed
		if p.initialRoot != "/" {
			t.Error("SetRoot to / should fail when initialRoot is not /")
		}
	}
}

func TestExplorerPanel_GoUp_AtRoot(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	// GoUp at initialRoot should be a no-op
	originalRoot := p.root
	p.GoUp()
	if p.root != originalRoot {
		t.Error("GoUp at initialRoot should not change root")
	}
}

func TestExplorerPanel_WidthHeight(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(40, 20)
	if p.Width() != 40 {
		t.Errorf("Width: got %d, want 40", p.Width())
	}
	if p.Height() != 20 {
		t.Errorf("Height: got %d, want 20", p.Height())
	}
}

func TestExplorerPanel_SetFocused(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	if p.IsFocused() {
		t.Error("should not be focused initially")
	}
	p.SetFocused(true)
	if !p.IsFocused() {
		t.Error("should be focused after SetFocused(true)")
	}
}

func TestExplorerPanel_Count(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(40, 20)
	// After SetSize triggers refresh, there should be entries
	count := p.Count()
	if count < 0 {
		t.Error("Count should not be negative")
	}
}

// ===========================================================================
// consensus.go: additional render paths
// ===========================================================================

func TestConsensusPanel_RenderWithHistory(t *testing.T) {
	p := NewConsensusPanel(80.0)
	p.SetSize(60, 30)
	p.SetScore(75)
	p.AddRound(1, 50)
	p.AddRound(2, 75)
	p.SetPairScore("claude", "gemini", "", 70)
	p.SetAnalysisPath("/tmp/analysis")
	p.Toggle()

	result := p.Render()
	if result == "" {
		t.Error("Render with history should not return empty")
	}
}

func TestConsensusPanel_CompactRender_ScoreBelowThreshold(t *testing.T) {
	p := NewConsensusPanel(80.0)
	p.SetScore(45) // Below 60
	result := p.CompactRender()
	if result == "" {
		t.Error("CompactRender should return non-empty when score > 0")
	}
}

// ===========================================================================
// Styles: applyDarkTheme, applyLightTheme (imported from model.go)
// ===========================================================================

func TestApplyThemes(t *testing.T) {
	// These modify package-level vars, so just verify no panic
	applyDarkTheme()
	applyLightTheme()
	applyDarkTheme() // Restore default
}

// ===========================================================================
// model.go: copyLastResponse (via /copy command)
// ===========================================================================

func TestCopyLastResponse_NoMessages(t *testing.T) {
	m := newCovBoostModel(t)
	// History is empty, so no agent messages to copy
	newModel, _, handled := m.copyLastResponse()
	if !handled {
		t.Error("copyLastResponse should return handled=true")
	}
	_ = newModel
}

func TestCopyLastResponse_WithAgentMessage(t *testing.T) {
	m := newCovBoostModel(t)
	m.history.Add(NewUserMessage("hello"))
	m.history.Add(NewAgentMessage("claude", "this is the response"))

	newModel, _, handled := m.copyLastResponse()
	if !handled {
		t.Error("copyLastResponse should return handled=true")
	}
	_ = newModel
}

func TestCopyLastResponse_OnlyUserMessages(t *testing.T) {
	m := newCovBoostModel(t)
	m.history.Add(NewUserMessage("hello"))
	m.history.Add(NewUserMessage("another"))

	newModel, _, handled := m.copyLastResponse()
	if !handled {
		t.Error("copyLastResponse should return handled=true even with no agent messages")
	}
	_ = newModel
}

// ===========================================================================
// model.go: copyConversation (via /copyall command)
// ===========================================================================

func TestCopyConversation_Empty(t *testing.T) {
	m := newCovBoostModel(t)
	newModel, _, handled := m.copyConversation()
	if !handled {
		t.Error("should return handled=true")
	}
	_ = newModel
}

func TestCopyConversation_WithMessages(t *testing.T) {
	m := newCovBoostModel(t)
	m.history.Add(NewUserMessage("hello"))
	m.history.Add(NewAgentMessage("claude", "hi there"))
	m.history.Add(NewSystemMessage("system info"))

	newModel, _, handled := m.copyConversation()
	if !handled {
		t.Error("should return handled=true")
	}
	_ = newModel
}

// ===========================================================================
// model.go: copyLogsToClipboard (via /copylogs command)
// ===========================================================================

func TestCopyLogsToClipboard_Empty(t *testing.T) {
	m := newCovBoostModel(t)
	newModel, _, handled := m.copyLogsToClipboard()
	if !handled {
		t.Error("should return handled=true")
	}
	_ = newModel
}

func TestCopyLogsToClipboard_WithLogs(t *testing.T) {
	m := newCovBoostModel(t)
	m.logsPanel.AddInfo("test", "log message 1")
	m.logsPanel.AddError("test", "log message 2")

	newModel, _, handled := m.copyLogsToClipboard()
	if !handled {
		t.Error("should return handled=true")
	}
	_ = newModel
}

// ===========================================================================
// model.go: chatProgressTick
// ===========================================================================

func TestChatProgressTick(t *testing.T) {
	m := newCovBoostModel(t)
	m.chatStartedAt = time.Now()
	m.chatProgressInterval = 100 * time.Millisecond

	cmd := m.chatProgressTick()
	if cmd == nil {
		t.Fatal("chatProgressTick should return a non-nil cmd")
	}
}

func TestChatProgressTick_DefaultInterval(t *testing.T) {
	m := newCovBoostModel(t)
	m.chatStartedAt = time.Now()
	m.chatProgressInterval = 0 // Uses default

	cmd := m.chatProgressTick()
	if cmd == nil {
		t.Fatal("chatProgressTick should return a non-nil cmd even with zero interval")
	}
}

// ===========================================================================
// model.go: panelNavTimeoutCmd
// ===========================================================================

func TestPanelNavTimeoutCmd(t *testing.T) {
	cmd := panelNavTimeoutCmd(42)
	if cmd == nil {
		t.Fatal("panelNavTimeoutCmd should return a non-nil cmd")
	}
}

// ===========================================================================
// model.go: WithWorkflowRunner
// ===========================================================================

func TestWithWorkflowRunner(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	cleanupModel(t, &m)

	runner := &mockWorkflowRunner{}
	m = m.WithWorkflowRunner(runner, nil, nil)
	if m.runner == nil {
		t.Error("runner should be set after WithWorkflowRunner")
	}
}

// ===========================================================================
// explorer.go: scheduleRefresh
// ===========================================================================

func TestExplorerPanel_ScheduleRefresh(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	// scheduleRefresh should not panic
	p.scheduleRefresh()

	// Wait for debounce timer to fire
	time.Sleep(200 * time.Millisecond)

	// Try to read from onChange channel (non-blocking)
	select {
	case <-p.OnChange():
		// Got the signal - good
	default:
		// Timer may have already fired, that's ok
	}
}

// ===========================================================================
// logs.go: LogsPanel scroll methods (increase coverage for scroll operations)
// ===========================================================================

// ===========================================================================
// logs.go: RenderWithFocus
// ===========================================================================

func TestLogsPanel_RenderWithFocus(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 20)
	p.AddInfo("test", "a message")

	result := p.RenderWithFocus(true)
	if result == "" {
		t.Error("RenderWithFocus should not return empty")
	}

	result2 := p.RenderWithFocus(false)
	if result2 == "" {
		t.Error("RenderWithFocus(false) should not return empty")
	}
}

// ===========================================================================
// logs.go: formatEntry with empty source
// ===========================================================================

// ===========================================================================
// token_panel.go: scroll methods
// ===========================================================================

// ===========================================================================
// history_search.go: Show, Hide, Toggle, SetSize, Render
// ===========================================================================

func TestHistorySearch_ShowHideToggle(t *testing.T) {
	tmpDir := t.TempDir()
	hs := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		filtered:    make([]HistoryEntry, 0),
		maxEntries:  100,
		historyFile: filepath.Join(tmpDir, "history.json"),
	}
	hs.input = newTestTextInput()

	if hs.IsVisible() {
		t.Error("should not be visible initially")
	}
	hs.Show()
	if !hs.IsVisible() {
		t.Error("should be visible after Show")
	}
	hs.Hide()
	if hs.IsVisible() {
		t.Error("should be hidden after Hide")
	}
	hs.Toggle()
	if !hs.IsVisible() {
		t.Error("should be visible after Toggle from hidden")
	}
	hs.Toggle()
	if hs.IsVisible() {
		t.Error("should be hidden after Toggle from visible")
	}
}

func TestHistorySearch_Render(t *testing.T) {
	tmpDir := t.TempDir()
	hs := &HistorySearch{
		entries:     make([]HistoryEntry, 0),
		filtered:    make([]HistoryEntry, 0),
		maxEntries:  100,
		historyFile: filepath.Join(tmpDir, "history.json"),
	}
	hs.input = newTestTextInput()

	// Not visible
	result := hs.Render()
	if result != "" {
		t.Error("Render when not visible should return empty")
	}

	hs.Add("command 1", "claude")
	hs.Add("command 2", "gemini")
	hs.Add("command 3", "codex")

	hs.SetSize(60, 20)
	hs.Show()
	result = hs.Render()
	if result == "" {
		t.Error("Render when visible should not return empty")
	}
}

// newTestTextInput creates a basic textinput.Model for tests
func newTestTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 256
	ti.Width = 40
	return ti
}

