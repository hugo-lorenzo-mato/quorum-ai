package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// =============================================================================
// Checkpoint coverage: ErrorCheckpointWithContext, resume points for moderator/analysis
// =============================================================================

func TestErrorCheckpointWithContext_AllFields(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	details := ErrorCheckpointDetails{
		Agent:             "claude",
		Model:             "opus-4",
		Round:             3,
		Attempt:           2,
		DurationMS:        1500,
		TokensIn:          200,
		TokensOut:         150,
		OutputSample:      "partial output text",
		IsTransient:       true,
		IsValidationError: false,
		FallbacksTried:    []string{"gemini", "codex"},
		Extra:             map[string]string{"context_key": "context_value"},
	}

	err := mgr.ErrorCheckpointWithContext(context.Background(), state, errForTest("test error"), details)
	if err != nil {
		t.Fatalf("ErrorCheckpointWithContext() error = %v", err)
	}

	if len(state.Checkpoints) == 0 {
		t.Fatal("expected at least one checkpoint")
	}

	last := state.Checkpoints[len(state.Checkpoints)-1]
	if last.Type != string(CheckpointError) {
		t.Errorf("expected checkpoint type %q, got %q", CheckpointError, last.Type)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(last.Data, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	assertMetaString(t, metadata, "agent", "claude")
	assertMetaString(t, metadata, "model", "opus-4")
	assertMetaFloat(t, metadata, "round", 3)
	assertMetaFloat(t, metadata, "attempt", 2)
	assertMetaFloat(t, metadata, "duration_ms", 1500)
	assertMetaFloat(t, metadata, "tokens_in", 200)
	assertMetaFloat(t, metadata, "tokens_out", 150)
	assertMetaString(t, metadata, "output_sample", "partial output text")
	assertMetaBool(t, metadata, "is_transient", true)
	assertMetaString(t, metadata, "context_key", "context_value")

	if _, ok := metadata["is_validation_error"]; ok {
		t.Error("is_validation_error should not be present when false")
	}
}

func TestErrorCheckpointWithContext_LongOutputSampleTruncated(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	longOutput := strings.Repeat("x", 600)
	details := ErrorCheckpointDetails{
		OutputSample: longOutput,
	}

	err := mgr.ErrorCheckpointWithContext(context.Background(), state, errForTest("err"), details)
	if err != nil {
		t.Fatalf("ErrorCheckpointWithContext() error = %v", err)
	}

	last := state.Checkpoints[len(state.Checkpoints)-1]
	var metadata map[string]interface{}
	if err := json.Unmarshal(last.Data, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	sample, ok := metadata["output_sample"].(string)
	if !ok {
		t.Fatal("output_sample not found in metadata")
	}
	if !strings.HasSuffix(sample, "...[truncated]") {
		t.Error("long output sample should be truncated")
	}
	if len(sample) > 520 {
		t.Errorf("truncated sample too long: %d chars", len(sample))
	}
}

func TestErrorCheckpointWithContext_ValidationError(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	details := ErrorCheckpointDetails{
		IsValidationError: true,
	}

	err := mgr.ErrorCheckpointWithContext(context.Background(), state, errForTest("validation err"), details)
	if err != nil {
		t.Fatalf("ErrorCheckpointWithContext() error = %v", err)
	}

	last := state.Checkpoints[len(state.Checkpoints)-1]
	var metadata map[string]interface{}
	if err := json.Unmarshal(last.Data, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	assertMetaBool(t, metadata, "is_validation_error", true)
}

func TestGetResumePoint_ModeratorRound(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	metadata := map[string]interface{}{
		"round":   float64(2),
		"outputs": "serialized-outputs",
	}
	data, _ := json.Marshal(metadata)

	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        "cp-mod",
		Type:      string(CheckpointModeratorRound),
		Phase:     core.PhaseAnalyze,
		Timestamp: time.Now(),
		Data:      data,
	})

	rp, err := mgr.GetResumePoint(state)
	if err != nil {
		t.Fatalf("GetResumePoint() error = %v", err)
	}

	if rp.ModeratorRound != 2 {
		t.Errorf("expected moderator round 2, got %d", rp.ModeratorRound)
	}
	if rp.SavedOutputs != "serialized-outputs" {
		t.Errorf("expected saved outputs, got %q", rp.SavedOutputs)
	}
	if rp.RestartPhase {
		t.Error("moderator round should not restart phase")
	}
}

func TestGetResumePoint_AnalysisRound(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	metadata := map[string]interface{}{
		"round":   float64(3),
		"outputs": "analysis-outputs",
	}
	data, _ := json.Marshal(metadata)

	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        "cp-analysis",
		Type:      string(CheckpointAnalysisRound),
		Phase:     core.PhaseAnalyze,
		Timestamp: time.Now(),
		Data:      data,
	})

	rp, err := mgr.GetResumePoint(state)
	if err != nil {
		t.Fatalf("GetResumePoint() error = %v", err)
	}

	if rp.ModeratorRound != 3 {
		t.Errorf("expected analysis round 3, got %d", rp.ModeratorRound)
	}
	if rp.SavedOutputs != "analysis-outputs" {
		t.Errorf("expected saved outputs, got %q", rp.SavedOutputs)
	}
	if rp.RestartPhase {
		t.Error("analysis round should not restart phase")
	}
}

func TestGetResumePoint_PhaseComplete_AdvancesPhase(t *testing.T) {
	mock := &mockStateManager{state: newTestWorkflowState()}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	state := mock.state

	data, _ := json.Marshal(map[string]interface{}{"phase": "analyze"})

	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        "cp-phase-done",
		Type:      string(CheckpointPhaseComplete),
		Phase:     core.PhaseAnalyze,
		Timestamp: time.Now(),
		Data:      data,
	})

	rp, err := mgr.GetResumePoint(state)
	if err != nil {
		t.Fatalf("GetResumePoint() error = %v", err)
	}

	// After analyze completes, should advance to plan
	if rp.Phase != core.PhasePlan {
		t.Errorf("expected phase %q after analyze complete, got %q", core.PhasePlan, rp.Phase)
	}
}

func TestNextPhase(t *testing.T) {
	tests := []struct {
		current  core.Phase
		expected core.Phase
	}{
		{core.PhaseRefine, core.PhaseAnalyze},
		{core.PhaseAnalyze, core.PhasePlan},
		{core.PhasePlan, core.PhaseExecute},
		{core.PhaseExecute, core.PhaseExecute}, // No next phase
	}

	for _, tt := range tests {
		got := nextPhase(tt.current)
		if got != tt.expected {
			t.Errorf("nextPhase(%q) = %q, want %q", tt.current, got, tt.expected)
		}
	}
}

func TestPhaseCheckpoint_SaveError(t *testing.T) {
	mock := &mockStateManager{
		state:     newTestWorkflowState(),
		saveError: errForTest("save failed"),
	}
	mgr := NewCheckpointManager(mock, logging.NewNop())

	err := mgr.PhaseCheckpoint(context.Background(), mock.state, core.PhaseAnalyze, false)
	if err == nil {
		t.Error("expected error when save fails")
	}
}

func TestTaskCheckpoint_SaveError(t *testing.T) {
	mock := &mockStateManager{
		state:     newTestWorkflowState(),
		saveError: errForTest("save failed"),
	}
	mgr := NewCheckpointManager(mock, logging.NewNop())
	task := &core.Task{ID: "task-1", Name: "test", Status: core.TaskStatusRunning}

	err := mgr.TaskCheckpoint(context.Background(), mock.state, task, false)
	if err == nil {
		t.Error("expected error when save fails")
	}
}

// =============================================================================
// DAG coverage: Clear, GetDependencies/GetDependents for nil/non-existent
// =============================================================================

func TestDAGBuilder_Clear(t *testing.T) {
	builder := NewDAGBuilder()
	_ = builder.AddTask(&core.Task{ID: "task-1", Name: "task-1"})
	_ = builder.AddTask(&core.Task{ID: "task-2", Name: "task-2"})
	_ = builder.AddDependency("task-2", "task-1")

	if builder.TaskCount() != 2 {
		t.Errorf("expected 2 tasks before clear, got %d", builder.TaskCount())
	}

	builder.Clear()

	if builder.TaskCount() != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", builder.TaskCount())
	}
}

func TestDAGBuilder_GetDependencies_NonExistentTask(t *testing.T) {
	builder := NewDAGBuilder()
	_ = builder.AddTask(&core.Task{ID: "task-1", Name: "task-1"})

	deps := builder.GetDependencies("nonexistent")
	if deps != nil {
		t.Errorf("expected nil dependencies for nonexistent task, got %v", deps)
	}
}

func TestDAGBuilder_GetDependents_NonExistentTask(t *testing.T) {
	builder := NewDAGBuilder()
	_ = builder.AddTask(&core.Task{ID: "task-1", Name: "task-1"})

	deps := builder.GetDependents("nonexistent")
	if deps != nil {
		t.Errorf("expected nil dependents for nonexistent task, got %v", deps)
	}
}

// =============================================================================
// Metrics coverage: EndTask for non-existent, GetTaskMetrics for non-existent
// =============================================================================

func TestMetrics_EndTask_NonExistentTask(t *testing.T) {
	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// End a task that was never started - should not panic
	result := &core.ExecuteResult{Output: "test"}
	collector.EndTask("nonexistent-task", result, nil)

	metrics := collector.GetWorkflowMetrics()
	if metrics.TasksCompleted != 0 {
		t.Errorf("expected 0 completed tasks, got %d", metrics.TasksCompleted)
	}
}

func TestMetrics_GetTaskMetrics_NonExistentTask(t *testing.T) {
	collector := NewMetricsCollector()
	collector.StartWorkflow()

	tm, ok := collector.GetTaskMetrics("nonexistent-task")
	if ok {
		t.Error("expected ok=false for nonexistent task metrics")
	}
	if tm != nil {
		t.Error("expected nil for nonexistent task metrics")
	}
}

func TestMetrics_RecordRetry_NonExistentTask(t *testing.T) {
	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// Recording retry for non-existent task should not panic
	collector.RecordRetry("nonexistent-task")

	metrics := collector.GetWorkflowMetrics()
	if metrics.RetriesTotal != 1 {
		t.Errorf("expected 1 retry total, got %d", metrics.RetriesTotal)
	}
}

// =============================================================================
// Prompt coverage: uncovered Render methods
// =============================================================================

func TestPromptRenderer_RenderRefinePrompt(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := RefinePromptParams{
		OriginalPrompt: "Add a caching layer",
		Template:       "refine-prompt-v2",
	}

	result, err := renderer.RenderRefinePrompt(params)
	if err != nil {
		t.Fatalf("RenderRefinePrompt() error = %v", err)
	}

	if !strings.Contains(result, "caching layer") {
		t.Error("result should contain the original prompt")
	}
}

func TestPromptRenderer_RenderSynthesizeAnalysis(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := SynthesizeAnalysisParams{
		Prompt: "Add authentication",
		Analyses: []AnalysisOutput{
			{
				AgentName:       "claude",
				RawOutput:       "Analysis from Claude",
				Claims:          []string{"JWT is recommended"},
				Risks:           []string{"Token expiry"},
				Recommendations: []string{"Use refresh tokens"},
			},
			{
				AgentName:       "gemini",
				RawOutput:       "Analysis from Gemini",
				Claims:          []string{"OAuth2 is standard"},
				Risks:           []string{"Complexity"},
				Recommendations: []string{"Use existing library"},
			},
		},
	}

	result, err := renderer.RenderSynthesizeAnalysis(params)
	if err != nil {
		t.Fatalf("RenderSynthesizeAnalysis() error = %v", err)
	}

	if !strings.Contains(result, "authentication") {
		t.Error("result should contain prompt")
	}
}

func TestPromptRenderer_RenderPlanManifest(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := PlanParams{
		Prompt:               "Build API server",
		ConsolidatedAnalysis: "Analysis suggests REST architecture",
		MaxTasks:             5,
	}

	result, err := renderer.RenderPlanManifest(params)
	if err != nil {
		t.Fatalf("RenderPlanManifest() error = %v", err)
	}

	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestPromptRenderer_RenderSynthesizePlans(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := SynthesizePlansParams{
		Prompt:   "Build API",
		Analysis: "Comprehensive analysis",
		Plans: []PlanProposal{
			{AgentName: "claude", Model: "opus-4", Content: "Plan from Claude"},
			{AgentName: "gemini", Model: "pro", Content: "Plan from Gemini"},
		},
		MaxTasks: 5,
	}

	result, err := renderer.RenderSynthesizePlans(params)
	if err != nil {
		t.Fatalf("RenderSynthesizePlans() error = %v", err)
	}

	if !strings.Contains(result, "API") {
		t.Error("result should contain prompt")
	}
}

func TestPromptRenderer_RenderPlanComprehensive(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := ComprehensivePlanParams{
		Prompt:               "Build a web server",
		ConsolidatedAnalysis: "Analysis content",
		AvailableAgents: []AgentInfo{
			{Name: "claude", Model: "opus-4", Strengths: "Code generation", Capabilities: "JSON, streaming"},
		},
		TasksDir:         "/tmp/tasks",
		NamingConvention: "{id}-{name}.md",
	}

	result, err := renderer.RenderPlanComprehensive(params)
	if err != nil {
		t.Fatalf("RenderPlanComprehensive() error = %v", err)
	}

	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestPromptRenderer_RenderIssueGenerate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := IssueGenerateParams{
		ConsolidatedAnalysisPath: ".quorum/runs/wf-1/analyze-phase/consolidated.md",
		TaskFiles: []IssueTaskFile{
			{Path: ".quorum/tasks/task-1-setup.md", ID: "task-1", Name: "setup", Slug: "setup", Index: 1},
		},
		IssuesDir:             ".quorum/issues/wf-1/draft",
		Language:              "english",
		Tone:                  "professional",
		Summarize:             false,
		IncludeDiagrams:       true,
		IncludeTestingSection: true,
		CustomInstructions:    "Focus on testing",
		Convention:            "angular",
	}

	result, err := renderer.RenderIssueGenerate(params)
	if err != nil {
		t.Fatalf("RenderIssueGenerate() error = %v", err)
	}

	if result == "" {
		t.Error("result should not be empty")
	}
}

// =============================================================================
// RateLimiter coverage: GetStatus, AcquireN context cancellation
// =============================================================================

func TestRateLimiterRegistry_GetStatus(t *testing.T) {
	registry := NewRateLimiterRegistry()
	registry.Get("claude")
	registry.Get("gemini")

	status := registry.GetStatus()
	if len(status) != 2 {
		t.Errorf("len(GetStatus) = %d, want 2", len(status))
	}
}

func TestRateLimiter_AcquireN_ContextCancelled(t *testing.T) {
	cfg := RateLimiterConfig{
		MaxTokens:  1,
		RefillRate: 0.01,
	}
	limiter := NewRateLimiter(cfg)
	limiter.TryAcquire() // Drain

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.AcquireN(ctx, 3)
	if err == nil {
		t.Error("expected error when context is cancelled during AcquireN")
	}
}

// =============================================================================
// Report coverage: writeConsensusSummary, truncate edge case
// =============================================================================

func TestTruncate_ShortString(t *testing.T) {
	result := truncate("hi", 10)
	if result != "hi" {
		t.Errorf("expected 'hi', got %q", result)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	result := truncate("hello", 5)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncate_LongString(t *testing.T) {
	result := truncate("hello world", 8)
	if result != "hello..." {
		t.Errorf("expected 'hello...', got %q", result)
	}
}

// =============================================================================
// System prompts coverage: validation edge cases, GetSystemPrompt error paths
// =============================================================================

func TestValidateSystemPromptMeta_Errors(t *testing.T) {
	tests := []struct {
		name    string
		meta    systemPromptFrontmatter
		id      string
		wantErr string
	}{
		{
			name:    "empty id",
			meta:    systemPromptFrontmatter{ID: ""},
			id:      "test",
			wantErr: "id is required",
		},
		{
			name:    "id mismatch",
			meta:    systemPromptFrontmatter{ID: "foo"},
			id:      "bar",
			wantErr: "does not match filename",
		},
		{
			name:    "empty title",
			meta:    systemPromptFrontmatter{ID: "test", Title: ""},
			id:      "test",
			wantErr: "title is required",
		},
		{
			name:    "invalid workflow_phase",
			meta:    systemPromptFrontmatter{ID: "test", Title: "Test", WorkflowPhase: "invalid"},
			id:      "test",
			wantErr: "invalid workflow_phase",
		},
		{
			name:    "empty step",
			meta:    systemPromptFrontmatter{ID: "test", Title: "Test", WorkflowPhase: "analyze", Step: ""},
			id:      "test",
			wantErr: "step is required",
		},
		{
			name:    "invalid status",
			meta:    systemPromptFrontmatter{ID: "test", Title: "Test", WorkflowPhase: "analyze", Step: "v1", Status: "invalid"},
			id:      "test",
			wantErr: "invalid status",
		},
		{
			name:    "empty used_by",
			meta:    systemPromptFrontmatter{ID: "test", Title: "Test", WorkflowPhase: "analyze", Step: "v1", Status: "active"},
			id:      "test",
			wantErr: "used_by must not be empty",
		},
		{
			name:    "invalid used_by value",
			meta:    systemPromptFrontmatter{ID: "test", Title: "Test", WorkflowPhase: "analyze", Step: "v1", Status: "active", UsedBy: []string{"invalid"}},
			id:      "test",
			wantErr: "invalid used_by value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSystemPromptMeta(tt.meta, tt.id)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetSystemPrompt_EmptyID(t *testing.T) {
	_, err := GetSystemPrompt("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestGetSystemPrompt_NotFound(t *testing.T) {
	_, err := GetSystemPrompt("nonexistent-prompt-id")
	if err == nil {
		t.Error("expected error for nonexistent prompt")
	}
}

func TestSplitSystemPromptFrontmatter_NoPrefix(t *testing.T) {
	_, body, ok := splitSystemPromptFrontmatter("no frontmatter here")
	if ok {
		t.Error("expected ok=false for no frontmatter prefix")
	}
	if body != "no frontmatter here" {
		t.Errorf("expected original content as body, got %q", body)
	}
}

func TestSplitSystemPromptFrontmatter_NoClosingDelimiter(t *testing.T) {
	content := "---\nid: test\ntitle: Test\n"
	_, body, ok := splitSystemPromptFrontmatter(content)
	if ok {
		t.Error("expected ok=false for no closing delimiter")
	}
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestSplitSystemPromptFrontmatter_WindowsLineEndings(t *testing.T) {
	content := "---\r\nid: test\r\n---\r\nBody content\r\n"
	fm, body, ok := splitSystemPromptFrontmatter(content)
	if !ok {
		t.Error("expected ok=true for valid frontmatter with CRLF")
	}
	if fm != "id: test" {
		t.Errorf("expected 'id: test' frontmatter, got %q", fm)
	}
	if !strings.Contains(body, "Body content") {
		t.Errorf("body should contain 'Body content', got %q", body)
	}
}

// =============================================================================
// Trace coverage: noopTraceWriter methods, TraceOutputNotifier, sanitize helpers
// =============================================================================

func TestNoopTraceWriter(t *testing.T) {
	w := &noopTraceWriter{}

	if w.Enabled() {
		t.Error("noop writer should not be enabled")
	}
	if w.RunID() != "" {
		t.Error("noop writer should return empty run ID")
	}
	if w.Dir() != "" {
		t.Error("noop writer should return empty dir")
	}
	if err := w.Record(context.Background(), TraceEvent{}); err != nil {
		t.Errorf("noop Record should return nil, got %v", err)
	}
	summary := w.EndRun(context.Background())
	if summary.RunID != "" {
		t.Error("noop EndRun should return empty summary")
	}
}

func TestTraceOutputNotifier_NilWriter(t *testing.T) {
	notifier := &TraceOutputNotifier{writer: nil}

	// All methods should be no-ops with nil writer
	notifier.PhaseStarted("analyze")
	notifier.TaskStarted("task-1", "Test Task", "claude")
	notifier.TaskCompleted("task-1", "Test Task", time.Second, 100, 50)
	notifier.TaskFailed("task-1", "Test Task", errForTest("test"))
	notifier.WorkflowStateUpdated("running", 5)
	if err := notifier.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestTraceOutputNotifier_DisabledWriter(t *testing.T) {
	w := &noopTraceWriter{} // Enabled() returns false
	notifier := NewTraceOutputNotifier(w)

	// All methods should be no-ops when writer is disabled
	notifier.PhaseStarted("analyze")
	notifier.TaskStarted("task-1", "Test Task", "claude")
	notifier.TaskCompleted("task-1", "Test Task", time.Second, 100, 50)
	notifier.TaskFailed("task-1", "Test Task", errForTest("test"))
	notifier.WorkflowStateUpdated("running", 5)
	if err := notifier.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestTraceOutputNotifier_EnabledWriter(t *testing.T) {
	dir := t.TempDir()
	w := startTraceWriter(t, TraceConfig{
		Mode:          "summary",
		Dir:           dir,
		MaxBytes:      4096,
		TotalMaxBytes: 8192,
		MaxFiles:      100,
	})

	notifier := NewTraceOutputNotifier(w)

	notifier.PhaseStarted("analyze")
	notifier.TaskStarted("task-1", "Test Task", "claude")
	notifier.TaskCompleted("task-1", "Test Task", time.Second, 100, 50)
	notifier.TaskFailed("task-2", "Failed Task", errForTest("boom"))
	notifier.TaskFailed("task-3", "No Error Task", nil)
	notifier.WorkflowStateUpdated("running", 5)

	if err := notifier.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	records := readTraceRecords(t, w.Dir())
	if len(records) < 5 {
		t.Errorf("expected at least 5 records, got %d", len(records))
	}
}

func TestSanitizeTraceID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc-123_XYZ", "abc-123_XYZ"},
		{"hello world!", "hello_world_"},
		{"run@2024/01", "run_2024_01"},
	}

	for _, tt := range tests {
		got := sanitizeTraceID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeTraceID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeFileComponent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc-def_123", "abc-def_123"},
		{"hello world!@#", "helloworld"},
	}

	for _, tt := range tests {
		got := sanitizeFileComponent(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFileComponent(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestHashContent(t *testing.T) {
	// Empty content returns empty string
	result := hashContent(nil)
	if result != "" {
		t.Errorf("hashContent(nil) = %q, want empty", result)
	}

	result = hashContent([]byte{})
	if result != "" {
		t.Errorf("hashContent(empty) = %q, want empty", result)
	}

	// Non-empty content returns sha256 hex
	result = hashContent([]byte("hello"))
	if len(result) != 64 {
		t.Errorf("expected 64-char hash, got %d chars", len(result))
	}
}

func TestNormalizeTraceConfig(t *testing.T) {
	// Zero values should be set to defaults
	cfg := normalizeTraceConfig(TraceConfig{
		Redact: true,
	})

	if cfg.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", cfg.SchemaVersion)
	}
	if cfg.Dir != ".quorum/traces" {
		t.Errorf("Dir = %q, want .quorum/traces", cfg.Dir)
	}
	if cfg.MaxBytes != 262144 {
		t.Errorf("MaxBytes = %d, want 262144", cfg.MaxBytes)
	}
	if cfg.TotalMaxBytes != 10485760 {
		t.Errorf("TotalMaxBytes = %d, want 10485760", cfg.TotalMaxBytes)
	}
	if cfg.MaxFiles != 500 {
		t.Errorf("MaxFiles = %d, want 500", cfg.MaxFiles)
	}
	if len(cfg.RedactPatterns) == 0 {
		t.Error("Redact=true should populate default redact patterns")
	}

	// TotalMaxBytes < MaxBytes should be corrected
	cfg2 := normalizeTraceConfig(TraceConfig{
		MaxBytes:      1000,
		TotalMaxBytes: 500,
	})
	if cfg2.TotalMaxBytes < cfg2.MaxBytes {
		t.Errorf("TotalMaxBytes (%d) should be >= MaxBytes (%d)", cfg2.TotalMaxBytes, cfg2.MaxBytes)
	}
}

func TestPhaseIncluded(t *testing.T) {
	w := &fileTraceWriter{
		cfg: TraceConfig{
			IncludePhases: []string{"analyze", "plan"},
		},
	}

	if !w.phaseIncluded("analyze") {
		t.Error("analyze should be included")
	}
	if !w.phaseIncluded("plan") {
		t.Error("plan should be included")
	}
	if w.phaseIncluded("execute") {
		t.Error("execute should not be included")
	}

	// Empty IncludePhases means include all
	w2 := &fileTraceWriter{cfg: TraceConfig{}}
	if !w2.phaseIncluded("anything") {
		t.Error("empty include phases should include everything")
	}
}

func TestBuildFilename_EmptyBase(t *testing.T) {
	w := &fileTraceWriter{}

	// All empty fields should produce event-N fallback
	result := w.buildFilename(1, TraceEvent{})
	if !strings.Contains(result, "event-1") {
		t.Errorf("expected event-1 fallback, got %q", result)
	}

	// With fields
	result = w.buildFilename(5, TraceEvent{
		Phase:     "analyze",
		Step:      "v1",
		Agent:     "claude",
		EventType: "prompt",
		FileExt:   ".md",
	})
	if !strings.HasPrefix(result, "0005-") {
		t.Errorf("expected 0005- prefix, got %q", result)
	}
	if !strings.HasSuffix(result, ".md") {
		t.Errorf("expected .md suffix, got %q", result)
	}
}

// =============================================================================
// Modes coverage: IsDryRunBlocked for non-dry-run error, case-insensitive tool deny
// =============================================================================

func TestIsDryRunBlocked_NonDryRunError(t *testing.T) {
	err := errForTest("not a dry run error")
	if IsDryRunBlocked(err) {
		t.Error("IsDryRunBlocked should return false for non-dry-run errors")
	}
}

func TestModeEnforcer_DeniedToolCaseInsensitive(t *testing.T) {
	mode := ExecutionMode{
		DeniedTools: []string{"DANGEROUS-TOOL"},
	}
	enforcer := NewModeEnforcer(mode)

	op := Operation{
		Name: "use-tool",
		Type: OpTypeLLM,
		Tool: "dangerous-tool", // lowercase
	}

	err := enforcer.CanExecute(context.Background(), op)
	if err == nil {
		t.Error("tool matching should be case-insensitive")
	}
}

func TestDefaultMode(t *testing.T) {
	mode := DefaultMode()
	if mode.DryRun {
		t.Error("default mode should not be dry-run")
	}
	if mode.Yolo {
		t.Error("default mode should not be yolo")
	}
	if !mode.Interactive {
		t.Error("default mode should be interactive")
	}
	if mode.DeniedTools != nil {
		t.Error("default mode should have nil denied tools")
	}
}

// =============================================================================
// Helper functions
// =============================================================================

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string { return e.msg }

func errForTest(msg string) error {
	return &simpleError{msg: msg}
}

func assertMetaString(t *testing.T, meta map[string]interface{}, key, expected string) {
	t.Helper()
	val, ok := meta[key].(string)
	if !ok {
		t.Errorf("metadata key %q not found or not string", key)
		return
	}
	if val != expected {
		t.Errorf("metadata[%q] = %q, want %q", key, val, expected)
	}
}

func assertMetaFloat(t *testing.T, meta map[string]interface{}, key string, expected float64) {
	t.Helper()
	val, ok := meta[key].(float64)
	if !ok {
		t.Errorf("metadata key %q not found or not float64", key)
		return
	}
	if val != expected {
		t.Errorf("metadata[%q] = %v, want %v", key, val, expected)
	}
}

func assertMetaBool(t *testing.T, meta map[string]interface{}, key string, expected bool) {
	t.Helper()
	val, ok := meta[key].(bool)
	if !ok {
		t.Errorf("metadata key %q not found or not bool", key)
		return
	}
	if val != expected {
		t.Errorf("metadata[%q] = %v, want %v", key, val, expected)
	}
}
