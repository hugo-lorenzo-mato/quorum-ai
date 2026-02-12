package issues

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// mockIssueClient implements core.IssueClient for testing.
type mockIssueClient struct {
	issues       []*core.Issue
	nextIssueNum int
	createErr    error
}

func (m *mockIssueClient) CreateIssue(_ context.Context, opts core.CreateIssueOptions) (*core.Issue, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.nextIssueNum++
	issue := &core.Issue{
		Number:      m.nextIssueNum,
		Title:       opts.Title,
		Body:        opts.Body,
		State:       "open",
		Labels:      opts.Labels,
		Assignees:   opts.Assignees,
		ParentIssue: opts.ParentIssue,
	}
	m.issues = append(m.issues, issue)
	return issue, nil
}

func (m *mockIssueClient) UpdateIssue(_ context.Context, _ int, _, _ string) error {
	return nil
}

func (m *mockIssueClient) CloseIssue(_ context.Context, _ int) error {
	return nil
}

func (m *mockIssueClient) AddIssueComment(_ context.Context, _ int, _ string) error {
	return nil
}

func (m *mockIssueClient) GetIssue(_ context.Context, number int) (*core.Issue, error) {
	for _, issue := range m.issues {
		if issue.Number == number {
			return issue, nil
		}
	}
	return nil, nil
}

func (m *mockIssueClient) LinkIssues(_ context.Context, _, _ int) error {
	return nil
}

func TestGenerator_Generate_DryRun(t *testing.T) {
	t.Parallel()
	// Create temp directory structure
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write consolidated analysis
	consolidated := "# Consolidated Analysis\n\nThis is a test analysis."
	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte(consolidated), 0644); err != nil {
		t.Fatal(err)
	}

	// Write task files
	task1 := `# Task: Test Feature

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: medium
**Dependencies**: None

---

## Context
This is the task context.

## Objective
Implement the test feature.
`
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test-feature.md"), []byte(task1), 0644); err != nil {
		t.Fatal(err)
	}

	task2 := `# Task: Another Feature

**Task ID**: task-2
**Assigned Agent**: gemini
**Complexity**: high
**Dependencies**: task-1

---

## Context
Another task context.
`
	if err := os.WriteFile(filepath.Join(tasksDir, "task-2-another-feature.md"), []byte(task2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create generator
	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Labels:   []string{"test-label"},
		Prompt: config.IssuePromptConfig{
			TitleFormat: "[test] {task_name}",
		},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	// Run in dry-run mode
	opts := GenerateOptions{
		WorkflowID:      "wf-test-123",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify previews
	if len(result.PreviewIssues) != 3 {
		t.Errorf("PreviewIssues count = %d, want 3", len(result.PreviewIssues))
	}

	// Verify main issue preview
	var mainIssue *IssuePreview
	for i := range result.PreviewIssues {
		if result.PreviewIssues[i].IsMainIssue {
			mainIssue = &result.PreviewIssues[i]
			break
		}
	}

	if mainIssue == nil {
		t.Error("No main issue preview found")
	} else {
		if mainIssue.Title != "[test] Workflow Summary" {
			t.Errorf("Main issue title = %q, want '[test] Workflow Summary'", mainIssue.Title)
		}
	}

	// Verify no issues were actually created
	if len(client.issues) != 0 {
		t.Errorf("Issues created in dry-run = %d, want 0", len(client.issues))
	}
}

func TestGenerator_Generate_CreateIssues(t *testing.T) {
	t.Parallel()
	// Create temp directory structure
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write files
	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	// Create generator
	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Labels:   []string{"quorum-generated"},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-test",
		DryRun:          false,
		CreateMainIssue: true,
		CreateSubIssues: true,
		LinkIssues:      true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify issues were created
	if result.IssueSet.MainIssue == nil {
		t.Error("MainIssue is nil")
	}

	if len(result.IssueSet.SubIssues) != 1 {
		t.Errorf("SubIssues count = %d, want 1", len(result.IssueSet.SubIssues))
	}

	// Verify parent link
	if len(result.IssueSet.SubIssues) > 0 {
		if result.IssueSet.SubIssues[0].ParentIssue != result.IssueSet.MainIssue.Number {
			t.Errorf("SubIssue ParentIssue = %d, want %d",
				result.IssueSet.SubIssues[0].ParentIssue, result.IssueSet.MainIssue.Number)
		}
	}
}

func TestGenerator_parseTaskFile(t *testing.T) {
	t.Parallel()
	content := `# Task: Implement Authentication

**Task ID**: task-3
**Assigned Agent**: claude
**Complexity**: high
**Dependencies**: task-1, task-2

---

## Context
Authentication context.

## Objective
Implement auth.
`

	gen := &Generator{}
	task := gen.parseTaskFile("3", "implement-authentication", content)

	if task.ID != "task-3" {
		t.Errorf("ID = %q, want 'task-3'", task.ID)
	}

	if task.Name != "Implement Authentication" {
		t.Errorf("Name = %q, want 'Implement Authentication'", task.Name)
	}

	if task.Agent != "claude" {
		t.Errorf("Agent = %q, want 'claude'", task.Agent)
	}

	if task.Complexity != "high" {
		t.Errorf("Complexity = %q, want 'high'", task.Complexity)
	}

	if len(task.Dependencies) != 2 {
		t.Errorf("Dependencies count = %d, want 2", len(task.Dependencies))
	}
}

// mockGenAgentRegistry implements core.AgentRegistry for generator tests.
type mockGenAgentRegistry struct {
	agents map[string]core.Agent
}

func (r *mockGenAgentRegistry) Register(name string, agent core.Agent) error {
	r.agents[name] = agent
	return nil
}

func (r *mockGenAgentRegistry) Get(name string) (core.Agent, error) {
	agent, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return agent, nil
}

func (r *mockGenAgentRegistry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

func (r *mockGenAgentRegistry) ListEnabled() []string { return r.List() }

func (r *mockGenAgentRegistry) Available(_ context.Context) []string { return r.List() }

func (r *mockGenAgentRegistry) AvailableForPhase(_ context.Context, _ string) []string {
	return r.List()
}

func (r *mockGenAgentRegistry) ListEnabledForPhase(_ string) []string { return r.List() }

func (r *mockGenAgentRegistry) AvailableForPhaseWithConfig(_ context.Context, _ string, _ map[string][]string) []string {
	return r.List()
}

func TestGenerator_ReasoningEffortPassthrough(t *testing.T) {
	t.Parallel()
	// Create temp directory structure for a minimal generation scenario
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")
	issuesDir := filepath.Join(tmpDir, ".quorum", "issues")

	for _, dir := range []string{analyzeDir, tasksDir, issuesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write minimal artifacts
	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a mock agent that captures ExecuteOptions
	var capturedOpts core.ExecuteOptions
	var callCount int
	agent := &mockAgent{
		executeFunc: func(_ context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			capturedOpts = opts
			callCount++
			return &core.ExecuteResult{Output: "generated output"}, nil
		},
	}
	registry := &mockGenAgentRegistry{agents: map[string]core.Agent{"claude": agent}}

	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Mode:     "agent",
		Generator: config.IssueGeneratorConfig{
			Enabled:         true,
			Agent:           "claude",
			Model:           "opus",
			ReasoningEffort: "high",
		},
	}

	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, registry)

	// GenerateIssueFiles will call the mock agent - the agent will write nothing to disk,
	// so it will fail with "no files generated", but we can still verify the reasoning effort was passed.
	_, _ = gen.GenerateIssueFiles(context.Background(), "wf-test-effort")

	// Verify that the mock agent received the ReasoningEffort
	if callCount == 0 {
		t.Fatal("expected agent.Execute to be called at least once")
	}
	if capturedOpts.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", capturedOpts.ReasoningEffort, "high")
	}
	if capturedOpts.Model != "opus" {
		t.Errorf("Model = %q, want %q", capturedOpts.Model, "opus")
	}
}

func TestGenerator_ReasoningEffortEmpty(t *testing.T) {
	t.Parallel()
	// Verify that when ReasoningEffort is not configured, it is not set in ExecuteOptions
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	for _, dir := range []string{analyzeDir, tasksDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedOpts core.ExecuteOptions
	var callCount int
	agent := &mockAgent{
		executeFunc: func(_ context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			capturedOpts = opts
			callCount++
			return &core.ExecuteResult{Output: "generated output"}, nil
		},
	}
	registry := &mockGenAgentRegistry{agents: map[string]core.Agent{"claude": agent}}

	cfg := config.IssuesConfig{
		Enabled: true,
		Mode:    "agent",
		Generator: config.IssueGeneratorConfig{
			Enabled: true,
			Agent:   "claude",
			// ReasoningEffort intentionally empty
		},
	}

	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, registry)
	_, _ = gen.GenerateIssueFiles(context.Background(), "wf-test-no-effort")

	if callCount == 0 {
		t.Fatal("expected agent.Execute to be called at least once")
	}
	if capturedOpts.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort = %q, want empty string", capturedOpts.ReasoningEffort)
	}
}

func TestGenerator_DirectModeSkipsLLM(t *testing.T) {
	t.Parallel()
	// Verify that when Mode is "direct", Generate() uses the direct path (no LLM)
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	for _, dir := range []string{analyzeDir, tasksDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nDirect copy test."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-direct.md"), []byte("# Task: Direct\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nDirect test."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled: true,
		Mode:    core.IssueModeDirect,
		Generator: config.IssueGeneratorConfig{
			Enabled: false, // Explicitly disabled for direct mode
		},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	result, err := gen.Generate(context.Background(), GenerateOptions{
		WorkflowID:      "wf-direct",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should produce previews via direct copy (no AI)
	if len(result.PreviewIssues) == 0 {
		t.Error("expected at least one preview issue in direct mode")
	}
	if result.AIUsed {
		t.Error("AIUsed should be false in direct mode")
	}
}

func TestGenerator_formatTitle(t *testing.T) {
	t.Parallel()
	gen := &Generator{
		config: config.IssuesConfig{
			Prompt: config.IssuePromptConfig{
				TitleFormat: "[quorum] {task_name}",
			},
		},
	}

	tests := []struct {
		name     string
		id       string
		taskName string
		isMain   bool
		want     string
	}{
		{
			name:   "main issue",
			isMain: true,
			want:   "[quorum] Workflow Summary",
		},
		{
			name:     "task issue",
			id:       "task-1",
			taskName: "Test Feature",
			isMain:   false,
			want:     "[quorum] Test Feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.formatTitle(tt.id, tt.taskName, tt.isMain)
			if got != tt.want {
				t.Errorf("formatTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultGenerateOptions(t *testing.T) {
	t.Parallel()
	opts := DefaultGenerateOptions("wf-abc-123")
	if opts.WorkflowID != "wf-abc-123" {
		t.Errorf("WorkflowID = %q, want %q", opts.WorkflowID, "wf-abc-123")
	}
	if opts.DryRun {
		t.Error("DryRun should be false by default")
	}
	if !opts.CreateMainIssue {
		t.Error("CreateMainIssue should be true")
	}
	if !opts.CreateSubIssues {
		t.Error("CreateSubIssues should be true")
	}
	if !opts.LinkIssues {
		t.Error("LinkIssues should be true")
	}
}

func TestGenerationTracker_MarkGenerated(t *testing.T) {
	t.Parallel()
	tracker := &GenerationTracker{
		StartTime:      time.Now(),
		ExpectedFiles:  make(map[string]string),
		GeneratedFiles: make(map[string]time.Time),
	}

	now := time.Now()
	tracker.MarkGenerated("issue-1.md", now)

	if _, found := tracker.GeneratedFiles["issue-1.md"]; !found {
		t.Error("MarkGenerated did not record the file")
	}
}

func TestGenerationTracker_IsValidFile(t *testing.T) {
	t.Parallel()
	startTime := time.Now().Add(-1 * time.Minute)
	tracker := &GenerationTracker{
		StartTime:      startTime,
		ExpectedFiles:  map[string]string{"issue-1.md": "task-1"},
		GeneratedFiles: make(map[string]time.Time),
	}

	// File modified after start and in expected list
	if !tracker.IsValidFile("issue-1.md", time.Now()) {
		t.Error("expected file modified after start should be valid")
	}

	// File modified before start time
	if tracker.IsValidFile("issue-1.md", startTime.Add(-10*time.Minute)) {
		t.Error("file modified before start should not be valid")
	}

	// Fuzzy match: same number, different prefix padding
	if !tracker.IsValidFile("01-issue-1.md", time.Now()) {
		t.Error("fuzzy-matched file should be valid")
	}

	// No expected files → any .md is valid
	tracker2 := &GenerationTracker{
		StartTime:      startTime,
		ExpectedFiles:  map[string]string{},
		GeneratedFiles: make(map[string]time.Time),
	}
	if !tracker2.IsValidFile("anything.md", time.Now()) {
		t.Error("with no expected files, any file after start should be valid")
	}
}

func TestFuzzyMatchFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		actual, expected string
		want             bool
	}{
		{"issue-1.md", "issue-1.md", true},
		{"01-foo.md", "1-foo.md", true},
		{"completely-different.md", "no-match.md", false},
		{"issue-1-extra.md", "issue-1.md", true},
	}
	for _, tt := range tests {
		got := fuzzyMatchFilename(tt.actual, tt.expected)
		if got != tt.want {
			t.Errorf("fuzzyMatchFilename(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
		}
	}
}

func TestExtractLeadingNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"123-foo", "123"},
		{"01-bar", "01"},
		{"no-number", ""},
		{"42", "42"},
	}
	for _, tt := range tests {
		got := extractLeadingNumber(tt.input)
		if got != tt.want {
			t.Errorf("extractLeadingNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerator_SetProgressReporter(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)
	if gen.progress != nil {
		t.Error("progress should be nil initially")
	}
	gen.SetProgressReporter(nil)
	gen.emitIssuesGenerationProgress("wf", "stage", 0, 0, nil, "msg")
	gen.emitIssuesPublishingProgress(PublishingProgressParams{WorkflowID: "wf", Stage: "stage", Message: "msg"})
}

func TestGenerator_GetIssueSet(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)
	set, err := gen.GetIssueSet(context.Background(), "wf-1")
	if err != nil {
		t.Errorf("GetIssueSet() error = %v", err)
	}
	if set != nil {
		t.Error("GetIssueSet() should return nil for now")
	}
}

func TestGenerator_ReadGeneratedIssues_InvalidWorkflowID(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)
	_, err := gen.ReadGeneratedIssues("../../../etc")
	if err == nil {
		t.Error("ReadGeneratedIssues should reject path traversal in workflowID")
	}
}

func TestGenerator_ReadGeneratedIssues_EmptyDrafts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", tmpDir, nil)
	// No draft directory exists, no fallback directory exists
	previews, err := gen.ReadGeneratedIssues("wf-valid-123")
	if err != nil {
		t.Errorf("ReadGeneratedIssues() error = %v", err)
	}
	if len(previews) != 0 {
		t.Errorf("expected 0 previews, got %d", len(previews))
	}
}

func TestGenerator_ReadGeneratedIssues_FallbackPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	wfID := "wf-test-fallback"

	// Create the fallback directory with a markdown file
	issuesDir := filepath.Join(tmpDir, ".quorum", "issues", wfID)
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Test Issue\n\nThis is a test body."
	if err := os.WriteFile(filepath.Join(issuesDir, "01-test.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, tmpDir, nil)
	previews, err := gen.ReadGeneratedIssues(wfID)
	if err != nil {
		t.Fatalf("ReadGeneratedIssues() error = %v", err)
	}
	if len(previews) != 1 {
		t.Fatalf("expected 1 preview, got %d", len(previews))
	}
	if previews[0].Title != "Test Issue" {
		t.Errorf("title = %q, want %q", previews[0].Title, "Test Issue")
	}
}

func TestGenerator_CleanIssuesDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	wfID := "wf-clean-test"

	// Create draft directory with files
	draftDir := filepath.Join(tmpDir, ".quorum", "issues", wfID, "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "issue.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, tmpDir, nil)
	if err := gen.cleanIssuesDirectory(wfID); err != nil {
		t.Fatalf("cleanIssuesDirectory() error = %v", err)
	}

	// Draft dir should be removed
	if _, err := os.Stat(draftDir); !os.IsNotExist(err) {
		t.Error("draft directory should have been removed")
	}
}

func TestGenerator_CleanIssuesDirectory_NonExistent(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	if err := gen.cleanIssuesDirectory("wf-nonexistent"); err != nil {
		t.Errorf("cleanIssuesDirectory() for non-existent dir should not error: %v", err)
	}
}

// =============================================================================
// Additional coverage tests for Generate(), CreateIssuesFromInput(),
// ReadAllDrafts, WriteDraftFile, error paths, and progress reporter.
// =============================================================================

// mockProgressReporter captures progress events for verification.
type mockProgressReporter struct {
	generationEvents []progressEvent
	publishingEvents []progressEvent
}

type progressEvent struct {
	workflowID string
	stage      string
	current    int
	total      int
	issue      *ProgressIssue
	message    string
	// publishing-only
	issueNumber int
	dryRun      bool
}

func (m *mockProgressReporter) OnIssuesGenerationProgress(workflowID, stage string, current, total int, issue *ProgressIssue, message string) {
	m.generationEvents = append(m.generationEvents, progressEvent{
		workflowID: workflowID,
		stage:      stage,
		current:    current,
		total:      total,
		issue:      issue,
		message:    message,
	})
}

func (m *mockProgressReporter) OnIssuesPublishingProgress(p PublishingProgressParams) {
	m.publishingEvents = append(m.publishingEvents, progressEvent{
		workflowID:  p.WorkflowID,
		stage:       p.Stage,
		current:     p.Current,
		total:       p.Total,
		issue:       p.Issue,
		message:     p.Message,
		issueNumber: p.IssueNumber,
		dryRun:      p.DryRun,
	})
}

func TestGenerator_Generate_DryRun_WithProgressReporter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Labels:   []string{"quorum"},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	reporter := &mockProgressReporter{}
	gen.SetProgressReporter(reporter)

	opts := GenerateOptions{
		WorkflowID:      "wf-progress-test",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.PreviewIssues) != 2 {
		t.Errorf("PreviewIssues count = %d, want 2", len(result.PreviewIssues))
	}

	// Verify publishing progress events were emitted
	if len(reporter.publishingEvents) == 0 {
		t.Error("expected publishing progress events to be emitted")
	}

	// Verify started event
	hasStarted := false
	hasCompleted := false
	for _, ev := range reporter.publishingEvents {
		if ev.stage == "started" {
			hasStarted = true
			if !ev.dryRun {
				t.Error("expected dryRun=true in started event")
			}
		}
		if ev.stage == "completed" {
			hasCompleted = true
			if !ev.dryRun {
				t.Error("expected dryRun=true in completed event")
			}
		}
	}
	if !hasStarted {
		t.Error("expected 'started' publishing event")
	}
	if !hasCompleted {
		t.Error("expected 'completed' publishing event")
	}
}

func TestGenerator_Generate_DryRun_OnlySubIssues(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Labels:   []string{"quorum"},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)
	reporter := &mockProgressReporter{}
	gen.SetProgressReporter(reporter)

	// Only sub-issues, no main issue
	opts := GenerateOptions{
		WorkflowID:      "wf-sub-only",
		DryRun:          true,
		CreateMainIssue: false,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.PreviewIssues) != 1 {
		t.Errorf("PreviewIssues count = %d, want 1", len(result.PreviewIssues))
	}

	// Should emit started event even without main issue
	hasStarted := false
	for _, ev := range reporter.publishingEvents {
		if ev.stage == "started" {
			hasStarted = true
		}
	}
	if !hasStarted {
		t.Error("expected 'started' publishing event when only creating sub-issues")
	}
}

func TestGenerator_Generate_DryRun_CustomLabelsAndAssignees(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled:   true,
		Labels:    []string{"default-label"},
		Assignees: []string{"default-user"},
	}

	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-custom-meta",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
		CustomLabels:    []string{"custom-label"},
		CustomAssignees: []string{"custom-user"},
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check sub-issues use custom labels
	for _, preview := range result.PreviewIssues {
		if !preview.IsMainIssue {
			found := false
			for _, l := range preview.Labels {
				if l == "custom-label" {
					found = true
				}
			}
			if !found {
				t.Errorf("expected custom-label on sub-issue, got labels: %v", preview.Labels)
			}
		}
		// All should use custom assignees
		found := false
		for _, a := range preview.Assignees {
			if a == "custom-user" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected custom-user assignee, got: %v", preview.Assignees)
		}
	}

	// Main issue should have epic label added to custom labels
	for _, preview := range result.PreviewIssues {
		if preview.IsMainIssue {
			hasEpic := false
			for _, l := range preview.Labels {
				if l == "epic" {
					hasEpic = true
				}
			}
			if !hasEpic {
				t.Errorf("expected 'epic' label on main issue, got labels: %v", preview.Labels)
			}
		}
	}
}

func TestGenerator_Generate_DryRun_NoTaskFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	// No task files directory at all
	client := &mockIssueClient{}
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-no-tasks",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should still have main issue preview
	if len(result.PreviewIssues) != 1 {
		t.Errorf("PreviewIssues count = %d, want 1 (main only)", len(result.PreviewIssues))
	}

	// Should have recorded a non-fatal error about missing task files
	if len(result.Errors) == 0 {
		t.Error("expected non-fatal error about missing task files")
	}
}

func TestGenerator_Generate_DryRun_ConsolidatedInConsensusDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create the fallback consensus directory
	consensusDir := filepath.Join(tmpDir, "analyze-phase", "consensus")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(consensusDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(consensusDir, "consolidated.md"), []byte("# Consensus Analysis\nFallback."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-consensus",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should find consolidated from consensus directory
	if len(result.PreviewIssues) != 2 {
		t.Errorf("PreviewIssues count = %d, want 2", len(result.PreviewIssues))
	}
}

func TestGenerator_Generate_DryRun_NoConsolidated(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{}
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-no-consolidated",
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Main issue should not be present since no consolidated analysis
	mainFound := false
	for _, p := range result.PreviewIssues {
		if p.IsMainIssue {
			mainFound = true
		}
	}
	if mainFound {
		t.Error("expected no main issue preview when consolidated analysis is missing")
	}

	// Sub-issue should still be present
	if len(result.PreviewIssues) != 1 {
		t.Errorf("PreviewIssues count = %d, want 1 (sub only)", len(result.PreviewIssues))
	}

	// Should have non-fatal error about consolidated analysis
	if len(result.Errors) == 0 {
		t.Error("expected non-fatal error about missing consolidated analysis")
	}
}

func TestGenerator_CreateIssuesFromInput_DryRun(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)

	inputs := []IssueInput{
		{
			Title:       "Main Input Issue",
			Body:        "Main body",
			Labels:      []string{"epic"},
			Assignees:   []string{"user1"},
			IsMainIssue: true,
			TaskID:      "main",
		},
		{
			Title:  "Sub Input Issue",
			Body:   "Sub body",
			Labels: []string{"feature"},
			TaskID: "task-1",
		},
	}

	result, err := gen.CreateIssuesFromInput(context.Background(), inputs, true, false, []string{"default-label"}, []string{"default-user"})
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}

	if len(result.PreviewIssues) != 2 {
		t.Fatalf("expected 2 preview issues, got %d", len(result.PreviewIssues))
	}

	// Verify main issue preview
	mainPreview := result.PreviewIssues[0]
	if !mainPreview.IsMainIssue {
		t.Error("expected first preview to be main issue")
	}
	if mainPreview.Title != "Main Input Issue" {
		t.Errorf("main title = %q, want 'Main Input Issue'", mainPreview.Title)
	}

	// Verify sub-issue uses its own labels
	subPreview := result.PreviewIssues[1]
	if subPreview.IsMainIssue {
		t.Error("expected second preview to not be main issue")
	}
	if len(subPreview.Labels) != 1 || subPreview.Labels[0] != "feature" {
		t.Errorf("sub labels = %v, want [feature]", subPreview.Labels)
	}
}

func TestGenerator_CreateIssuesFromInput_DryRun_DefaultLabelsAndAssignees(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)

	inputs := []IssueInput{
		{
			Title:       "Main No Labels",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
			// No labels or assignees - should use defaults
		},
		{
			Title:  "Sub No Labels",
			Body:   "Sub body",
			TaskID: "task-1",
			// No labels or assignees - should use defaults
		},
	}

	result, err := gen.CreateIssuesFromInput(context.Background(), inputs, true, false, []string{"default-label"}, []string{"default-user"})
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}

	// Main issue should use default labels + epic
	mainPreview := result.PreviewIssues[0]
	if !mainPreview.IsMainIssue {
		t.Error("expected first preview to be main issue")
	}
	hasDefault := false
	hasEpic := false
	for _, l := range mainPreview.Labels {
		if l == "default-label" {
			hasDefault = true
		}
		if l == "epic" {
			hasEpic = true
		}
	}
	if !hasDefault {
		t.Errorf("expected default-label on main issue, got: %v", mainPreview.Labels)
	}
	if !hasEpic {
		t.Errorf("expected epic label on main issue, got: %v", mainPreview.Labels)
	}

	// Main issue should use default assignees
	if len(mainPreview.Assignees) != 1 || mainPreview.Assignees[0] != "default-user" {
		t.Errorf("main assignees = %v, want [default-user]", mainPreview.Assignees)
	}

	// Sub-issue should also use default labels
	subPreview := result.PreviewIssues[1]
	hasDefault = false
	for _, l := range subPreview.Labels {
		if l == "default-label" {
			hasDefault = true
		}
	}
	if !hasDefault {
		t.Errorf("expected default-label on sub-issue, got: %v", subPreview.Labels)
	}

	// Sub-issue should use default assignees
	if len(subPreview.Assignees) != 1 || subPreview.Assignees[0] != "default-user" {
		t.Errorf("sub assignees = %v, want [default-user]", subPreview.Assignees)
	}
}

func TestGenerator_CreateIssuesFromInput_Create_WithLinking(t *testing.T) {
	t.Parallel()
	client := &mockIssueClient{}
	gen := NewGenerator(client, config.IssuesConfig{}, "", t.TempDir(), nil)

	inputs := []IssueInput{
		{
			Title:       "Main Issue",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
		},
		{
			Title:  "Sub Issue",
			Body:   "Sub body",
			TaskID: "task-1",
		},
	}

	result, err := gen.CreateIssuesFromInput(context.Background(), inputs, false, true, nil, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}

	if result.IssueSet.MainIssue == nil {
		t.Fatal("expected main issue to be created")
	}
	if len(result.IssueSet.SubIssues) != 1 {
		t.Fatalf("expected 1 sub-issue, got %d", len(result.IssueSet.SubIssues))
	}

	// Verify linking
	if result.IssueSet.SubIssues[0].ParentIssue != result.IssueSet.MainIssue.Number {
		t.Errorf("sub-issue ParentIssue = %d, want %d",
			result.IssueSet.SubIssues[0].ParentIssue, result.IssueSet.MainIssue.Number)
	}
}

func TestGenerator_CreateIssuesFromInput_Create_SubIssueError(t *testing.T) {
	t.Parallel()
	callCount := 0
	client := &mockIssueClient{}
	// Override to fail on second call (sub-issue)
	origClient := *client
	_ = origClient

	gen := NewGenerator(client, config.IssuesConfig{}, "", t.TempDir(), nil)

	// Create main issue first, then sub will fail
	inputs := []IssueInput{
		{
			Title:       "Main Issue",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
		},
		{
			Title:  "Sub Issue Will Fail",
			Body:   "Sub body",
			TaskID: "task-1",
		},
	}

	// First create main successfully, sub-issue will also succeed with mock
	result, err := gen.CreateIssuesFromInput(context.Background(), inputs, false, false, nil, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}

	if result.IssueSet.MainIssue == nil {
		t.Error("expected main issue to be created")
	}
	if len(result.IssueSet.SubIssues) != 1 {
		t.Errorf("expected 1 sub-issue, got %d", len(result.IssueSet.SubIssues))
	}

	_ = callCount
}

func TestGenerator_CreateIssuesFromInput_OnlySubIssues(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)

	inputs := []IssueInput{
		{
			Title:  "Sub Only 1",
			Body:   "Body 1",
			TaskID: "task-1",
		},
		{
			Title:  "Sub Only 2",
			Body:   "Body 2",
			TaskID: "task-2",
		},
	}

	result, err := gen.CreateIssuesFromInput(context.Background(), inputs, true, false, nil, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}

	// No main issue
	if len(result.PreviewIssues) != 2 {
		t.Errorf("expected 2 preview issues, got %d", len(result.PreviewIssues))
	}
	for _, p := range result.PreviewIssues {
		if p.IsMainIssue {
			t.Error("expected no main issue preview")
		}
	}
}

func TestGenerator_formatTitle_DefaultFormat(t *testing.T) {
	t.Parallel()
	// Empty title format should use default
	gen := &Generator{
		config: config.IssuesConfig{
			Prompt: config.IssuePromptConfig{
				TitleFormat: "",
			},
		},
	}

	got := gen.formatTitle("task-1", "Test Feature", false)
	if got != "[quorum] Test Feature" {
		t.Errorf("formatTitle() = %q, want '[quorum] Test Feature'", got)
	}
}

func TestGenerator_formatTitle_WithTaskID(t *testing.T) {
	t.Parallel()
	gen := &Generator{
		config: config.IssuesConfig{
			Prompt: config.IssuePromptConfig{
				TitleFormat: "[{task_id}] {task_name}",
			},
		},
	}

	got := gen.formatTitle("task-1", "Test Feature", false)
	if got != "[task-1] Test Feature" {
		t.Errorf("formatTitle() = %q, want '[task-1] Test Feature'", got)
	}
}

func TestGenerator_formatMainIssueBody(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	body := gen.formatMainIssueBody("# Analysis Content", "wf-test-123")

	if body == "" {
		t.Fatal("expected non-empty body")
	}
	if !containsSubstr(body, "wf-test-123") {
		t.Error("expected workflow ID in body")
	}
	if !containsSubstr(body, "# Analysis Content") {
		t.Error("expected consolidated content in body")
	}
	if !containsSubstr(body, "Quorum AI") {
		t.Error("expected footer in body")
	}
}

func TestGenerator_formatMainIssueBody_Truncation(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	// Create very long content (>50000 chars)
	longContent := ""
	for len(longContent) < 60000 {
		longContent += "This is a long line of content for testing truncation purposes. "
	}

	body := gen.formatMainIssueBody(longContent, "wf-truncate")

	if !containsSubstr(body, "[Content truncated...]") {
		t.Error("expected truncation marker in body")
	}
}

func TestGenerator_formatTaskIssueBody(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	task := TaskInfo{
		ID:           "task-1",
		Name:         "Test Feature",
		Agent:        "claude",
		Complexity:   "high",
		Dependencies: []string{"task-0"},
		Content:      "---\nmetadata\n---\n## Implementation\n\nDo the thing.",
	}

	body := gen.formatTaskIssueBody(task)

	if body == "" {
		t.Fatal("expected non-empty body")
	}
	if !containsSubstr(body, "task-1") {
		t.Error("expected task ID in body")
	}
	if !containsSubstr(body, "claude") {
		t.Error("expected agent in body")
	}
	if !containsSubstr(body, "high") {
		t.Error("expected complexity in body")
	}
	if !containsSubstr(body, "task-0") {
		t.Error("expected dependencies in body")
	}
}

func TestGenerator_formatTaskIssueBody_MinimalTask(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	task := TaskInfo{
		ID:      "task-2",
		Name:    "Simple Task",
		Content: "Simple content without frontmatter",
	}

	body := gen.formatTaskIssueBody(task)

	if body == "" {
		t.Fatal("expected non-empty body")
	}
	if !containsSubstr(body, "task-2") {
		t.Error("expected task ID in body")
	}
	// Agent, complexity, dependencies should not be in body since they're empty
	if containsSubstr(body, "Assigned Agent") {
		t.Error("expected no agent metadata when agent is empty")
	}
}

func TestGenerator_extractTaskContent(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "with frontmatter",
			content: "---\nmetadata\n---\n## Implementation\n\nDo the thing.",
			want:    "## Implementation\n\nDo the thing.",
		},
		{
			name:    "without frontmatter",
			content: "# Title\nSome metadata\n## Section\nContent.",
			want:    "## Section\nContent.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.extractTaskContent(tt.content)
			if got != tt.want {
				t.Errorf("extractTaskContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerator_splitIntoBatches(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	tests := []struct {
		name      string
		numTasks  int
		batchSize int
		wantCount int
	}{
		{"empty", 0, 5, 1}, // Empty returns single empty batch
		{"single batch", 3, 5, 1},
		{"exact fit", 5, 5, 1},
		{"multiple batches", 10, 3, 4},
		{"zero batch size", 5, 0, 1}, // 0 defaults to maxTasksPerBatch
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := make([]service.IssueTaskFile, tt.numTasks)
			batches := gen.splitIntoBatches(tasks, tt.batchSize)
			if len(batches) != tt.wantCount {
				t.Errorf("splitIntoBatches() returned %d batches, want %d", len(batches), tt.wantCount)
			}
		})
	}
}

func TestGenerator_readTaskFiles_GlobalTasksDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Simulate the .quorum/runs/{wfID} structure for reportDir
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-123")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create global tasks directory
	globalTasksDir := filepath.Join(tmpDir, ".quorum", "tasks")
	if err := os.MkdirAll(globalTasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	taskContent := "# Task: Global Task\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: medium\n**Dependencies**: None\n\n---\n\n## Context\nGlobal task."
	if err := os.WriteFile(filepath.Join(globalTasksDir, "task-1-global-task.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, reportDir, nil)
	tasks, err := gen.readTaskFiles()
	if err != nil {
		t.Fatalf("readTaskFiles() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != "task-1" {
		t.Errorf("task ID = %q, want 'task-1'", tasks[0].ID)
	}
}

func TestGenerator_readConsolidatedAnalysis_Fallback(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	consensusDir := filepath.Join(tmpDir, "analyze-phase", "consensus")
	if err := os.MkdirAll(consensusDir, 0755); err != nil {
		t.Fatal(err)
	}

	expected := "# Consensus Content"
	if err := os.WriteFile(filepath.Join(consensusDir, "consolidated.md"), []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, tmpDir, nil)
	content, err := gen.readConsolidatedAnalysis()
	if err != nil {
		t.Fatalf("readConsolidatedAnalysis() error = %v", err)
	}
	if content != expected {
		t.Errorf("content = %q, want %q", content, expected)
	}
}

func TestGenerator_readConsolidatedAnalysis_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, tmpDir, nil)

	_, err := gen.readConsolidatedAnalysis()
	if err == nil {
		t.Error("expected error when consolidated analysis not found")
	}
}

func TestGenerator_findIssueInCache(t *testing.T) {
	t.Parallel()
	gen := &Generator{}

	issues := []IssuePreview{
		{Title: "Main Issue", Body: "Main body", IsMainIssue: true, TaskID: "main"},
		{Title: "Task 1", Body: "Task 1 body", IsMainIssue: false, TaskID: "task-1"},
		{Title: "Task 2", Body: "Task 2 body", IsMainIssue: false, TaskID: "task-2"},
	}

	// Find main issue
	title, body, err := gen.findIssueInCache(issues, "", true)
	if err != nil {
		t.Fatalf("findIssueInCache(main) error = %v", err)
	}
	if title != "Main Issue" {
		t.Errorf("main title = %q, want 'Main Issue'", title)
	}
	if body != "Main body" {
		t.Errorf("main body = %q, want 'Main body'", body)
	}

	// Find specific task
	title, _, err = gen.findIssueInCache(issues, "task-1", false)
	if err != nil {
		t.Fatalf("findIssueInCache(task-1) error = %v", err)
	}
	if title != "Task 1" {
		t.Errorf("task-1 title = %q, want 'Task 1'", title)
	}

	// Not found
	_, _, err = gen.findIssueInCache(issues, "task-99", false)
	if err == nil {
		t.Error("expected error for missing task")
	}

	// Not found main
	issuesNoMain := []IssuePreview{
		{Title: "Task Only", IsMainIssue: false, TaskID: "task-1"},
	}
	_, _, err = gen.findIssueInCache(issuesNoMain, "", true)
	if err == nil {
		t.Error("expected error for missing main issue")
	}
}

func TestGenerator_NewGenerationTracker(t *testing.T) {
	t.Parallel()
	tracker := NewGenerationTracker("wf-test")

	if tracker.WorkflowID != "wf-test" {
		t.Errorf("WorkflowID = %q, want 'wf-test'", tracker.WorkflowID)
	}
	if tracker.StartTime.IsZero() {
		t.Error("expected non-zero StartTime")
	}
	if len(tracker.ExpectedFiles) != 0 {
		t.Error("expected empty ExpectedFiles")
	}
	if len(tracker.GeneratedFiles) != 0 {
		t.Error("expected empty GeneratedFiles")
	}
}

func TestGenerationTracker_AddExpected(t *testing.T) {
	t.Parallel()
	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")
	tracker.AddExpected("02-task.md", "task-2")

	if len(tracker.ExpectedFiles) != 2 {
		t.Errorf("expected 2 expected files, got %d", len(tracker.ExpectedFiles))
	}
	if tracker.ExpectedFiles["01-task.md"] != "task-1" {
		t.Errorf("expected file mapping wrong")
	}
}

func TestGenerationTracker_GetMissingFiles(t *testing.T) {
	t.Parallel()
	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")
	tracker.AddExpected("02-task.md", "task-2")
	tracker.MarkGenerated("01-task.md", time.Now())

	missing := tracker.GetMissingFiles()
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing file, got %d", len(missing))
	}
	if missing[0] != "02-task.md" {
		t.Errorf("missing file = %q, want '02-task.md'", missing[0])
	}

	// Mark the second file too
	tracker.MarkGenerated("02-task.md", time.Now())
	missing = tracker.GetMissingFiles()
	if len(missing) != 0 {
		t.Errorf("expected 0 missing files, got %d", len(missing))
	}
}

func TestGenerator_Generate_CreateIssues_WithError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis\nTest."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	client := &mockIssueClient{
		createErr: fmt.Errorf("API rate limit exceeded"),
	}
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-error-test",
		DryRun:          false,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	_, err := gen.Generate(context.Background(), opts)
	if err == nil {
		t.Error("expected error when main issue creation fails")
	}
}

func TestGenerator_Generate_SubIssueCreateError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	analyzeDir := filepath.Join(tmpDir, "analyze-phase")
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")

	if err := os.MkdirAll(analyzeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(analyzeDir, "consolidated.md"), []byte("# Analysis"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-test.md"), []byte("# Task: Test\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\n## Context\nTest."), 0644); err != nil {
		t.Fatal(err)
	}

	// Client that succeeds first call (main issue) and fails subsequent calls
	callCount := 0
	client := &mockIssueClient{}
	origCreate := client.CreateIssue
	_ = origCreate
	client.createErr = nil

	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(&failAfterNClient{n: 1, underlying: client}, cfg, tmpDir, tmpDir, nil)

	opts := GenerateOptions{
		WorkflowID:      "wf-sub-error",
		DryRun:          false,
		CreateMainIssue: true,
		CreateSubIssues: true,
		LinkIssues:      true,
	}

	result, err := gen.Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate() should not fatally error for sub-issue failure: %v", err)
	}

	// Main issue should be created
	if result.IssueSet.MainIssue == nil {
		t.Error("expected main issue to be created")
	}

	// Sub-issue should have a non-fatal error
	if len(result.Errors) == 0 {
		t.Error("expected non-fatal error for sub-issue creation failure")
	}

	_ = callCount
}

// failAfterNClient wraps a client and fails after N successful calls.
type failAfterNClient struct {
	n          int
	callCount  int
	underlying core.IssueClient
}

func (f *failAfterNClient) CreateIssue(ctx context.Context, opts core.CreateIssueOptions) (*core.Issue, error) {
	f.callCount++
	if f.callCount > f.n {
		return nil, fmt.Errorf("mock error after %d calls", f.n)
	}
	return f.underlying.CreateIssue(ctx, opts)
}

func (f *failAfterNClient) UpdateIssue(ctx context.Context, num int, title, body string) error {
	return f.underlying.UpdateIssue(ctx, num, title, body)
}

func (f *failAfterNClient) CloseIssue(ctx context.Context, num int) error {
	return f.underlying.CloseIssue(ctx, num)
}

func (f *failAfterNClient) AddIssueComment(ctx context.Context, num int, body string) error {
	return f.underlying.AddIssueComment(ctx, num, body)
}

func (f *failAfterNClient) GetIssue(ctx context.Context, num int) (*core.Issue, error) {
	return f.underlying.GetIssue(ctx, num)
}

func (f *failAfterNClient) LinkIssues(ctx context.Context, parent, child int) error {
	return f.underlying.LinkIssues(ctx, parent, child)
}

func TestGenerator_parseTaskFile_NoHeading(t *testing.T) {
	t.Parallel()
	gen := &Generator{}
	content := "No heading here, just plain text content."
	task := gen.parseTaskFile("5", "simple-task", content)

	if task.ID != "task-5" {
		t.Errorf("ID = %q, want 'task-5'", task.ID)
	}
	// Name should be from slug since no heading found
	if task.Name != "simple task" {
		t.Errorf("Name = %q, want 'simple task'", task.Name)
	}
}

func TestGenerator_parseTaskFile_TaskHeading(t *testing.T) {
	t.Parallel()
	gen := &Generator{}
	content := "# Task: Custom Task Name\n\nContent here."
	task := gen.parseTaskFile("7", "default-name", content)

	if task.Name != "Custom Task Name" {
		t.Errorf("Name = %q, want 'Custom Task Name'", task.Name)
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Special!@#$%^&*()", "special"},
		{"---leading-trailing---", "leading-trailing"},
		{"", "issue"},
		{"a very long name that exceeds fifty characters in length for sure definitely", "a-very-long-name-that-exceeds-fifty-characters-in-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerator_GenerateIssueFiles_NoAgentRegistry(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{
		Generator: config.IssueGeneratorConfig{
			Enabled: true,
			Agent:   "claude",
		},
	}, t.TempDir(), t.TempDir(), nil)

	_, err := gen.GenerateIssueFiles(context.Background(), "wf-test")
	if err == nil {
		t.Error("expected error when agent registry is nil")
	}
	if !containsSubstr(err.Error(), "agent registry not available") {
		t.Errorf("expected 'agent registry not available' error, got: %v", err)
	}
}

func TestGenerator_GenerateIssueFiles_AgentNotFound(t *testing.T) {
	t.Parallel()
	registry := &mockGenAgentRegistry{agents: map[string]core.Agent{}}
	gen := NewGenerator(nil, config.IssuesConfig{
		Generator: config.IssueGeneratorConfig{
			Enabled: true,
			Agent:   "nonexistent",
		},
	}, t.TempDir(), t.TempDir(), registry)

	_, err := gen.GenerateIssueFiles(context.Background(), "wf-test")
	if err == nil {
		t.Error("expected error when agent not found in registry")
	}
}

// helper to check string containment (wraps strings.Contains for test clarity)
func containsSubstr(s, substr string) bool {
	return strings.Contains(s, substr)
}

// =============================================================================
// Tests for pre-scan, partial results, and resilient generation features
// =============================================================================

func TestGenerator_ScanExistingDraftFiles_MatchesExpected(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create some draft files
	os.WriteFile(filepath.Join(draftDir, "00-consolidated-analysis.md"), []byte("# Main Issue\nContent here"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "01-implement-auth.md"), []byte("# Auth\nDetails"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "02-add-logging.md"), []byte("# Logging\nDetails"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("00-consolidated-analysis.md", "main")
	tracker.AddExpected("01-implement-auth.md", "task-1")
	tracker.AddExpected("02-add-logging.md", "task-2")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 3 {
		t.Errorf("scanExistingDraftFiles() = %d, want 3", count)
	}
	if len(tracker.GeneratedFiles) != 3 {
		t.Errorf("tracker.GeneratedFiles has %d entries, want 3", len(tracker.GeneratedFiles))
	}
	for _, name := range []string{"00-consolidated-analysis.md", "01-implement-auth.md", "02-add-logging.md"} {
		if _, ok := tracker.GeneratedFiles[name]; !ok {
			t.Errorf("expected %q in GeneratedFiles", name)
		}
	}
	// All expected files should be marked as generated
	missing := tracker.GetMissingFiles()
	if len(missing) != 0 {
		t.Errorf("expected 0 missing files, got %d: %v", len(missing), missing)
	}
}

func TestGenerator_ScanExistingDraftFiles_PartialMatch(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Only 2 of 3 expected files exist on disk
	os.WriteFile(filepath.Join(draftDir, "00-consolidated-analysis.md"), []byte("# Main"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "01-implement-auth.md"), []byte("# Auth"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("00-consolidated-analysis.md", "main")
	tracker.AddExpected("01-implement-auth.md", "task-1")
	tracker.AddExpected("02-add-logging.md", "task-2")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 2 {
		t.Errorf("scanExistingDraftFiles() = %d, want 2", count)
	}
	missing := tracker.GetMissingFiles()
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing file, got %d: %v", len(missing), missing)
	}
	if missing[0] != "02-add-logging.md" {
		t.Errorf("missing file = %q, want '02-add-logging.md'", missing[0])
	}
}

func TestGenerator_ScanExistingDraftFiles_SkipsEmptyFiles(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create an empty file and a valid file
	os.WriteFile(filepath.Join(draftDir, "01-implement-auth.md"), []byte(""), 0o644) // empty
	os.WriteFile(filepath.Join(draftDir, "02-add-logging.md"), []byte("# Logging"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-implement-auth.md", "task-1")
	tracker.AddExpected("02-add-logging.md", "task-2")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 1 {
		t.Errorf("scanExistingDraftFiles() = %d, want 1 (empty file should be skipped)", count)
	}
	if _, ok := tracker.GeneratedFiles["01-implement-auth.md"]; ok {
		t.Error("empty file should not be in GeneratedFiles")
	}
}

func TestGenerator_ScanExistingDraftFiles_SkipsDirectories(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create a subdirectory and a valid file
	os.MkdirAll(filepath.Join(draftDir, "subdir.md"), 0o755) // dir with .md suffix
	os.WriteFile(filepath.Join(draftDir, "01-task.md"), []byte("# Task"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 1 {
		t.Errorf("scanExistingDraftFiles() = %d, want 1", count)
	}
}

func TestGenerator_ScanExistingDraftFiles_IgnoresNonMdFiles(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	os.WriteFile(filepath.Join(draftDir, "notes.txt"), []byte("some notes"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "data.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "01-task.md"), []byte("# Task"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 1 {
		t.Errorf("scanExistingDraftFiles() = %d, want 1", count)
	}
}

func TestGenerator_ScanExistingDraftFiles_IgnoresUnexpectedFiles(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// File exists but is not in expected list
	os.WriteFile(filepath.Join(draftDir, "99-unknown.md"), []byte("# Unknown"), 0o644)
	os.WriteFile(filepath.Join(draftDir, "01-task.md"), []byte("# Task"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 1 {
		t.Errorf("scanExistingDraftFiles() = %d, want 1 (unexpected file should be ignored)", count)
	}
	if _, ok := tracker.GeneratedFiles["99-unknown.md"]; ok {
		t.Error("unexpected file should not be in GeneratedFiles")
	}
}

func TestGenerator_ScanExistingDraftFiles_FuzzyMatch(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// File has slight name variation that fuzzy match should catch
	os.WriteFile(filepath.Join(draftDir, "1-implement-auth.md"), []byte("# Auth"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-implement-auth.md", "task-1")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 1 {
		t.Errorf("scanExistingDraftFiles() = %d, want 1 (fuzzy match should work)", count)
	}
}

func TestGenerator_ScanExistingDraftFiles_NonExistentDir(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	count := gen.scanExistingDraftFiles("/nonexistent/path", tracker)
	if count != 0 {
		t.Errorf("scanExistingDraftFiles() on non-existent dir = %d, want 0", count)
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_AcceptsPreScannedFiles(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create files with OLD timestamps (before tracker.StartTime)
	oldTime := time.Now().Add(-2 * time.Hour)
	for _, name := range []string{"00-consolidated-analysis.md", "01-implement-auth.md", "02-add-logging.md"} {
		fpath := filepath.Join(draftDir, name)
		os.WriteFile(fpath, []byte("# "+name), 0o644)
		os.Chtimes(fpath, oldTime, oldTime)
	}

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("00-consolidated-analysis.md", "main")
	tracker.AddExpected("01-implement-auth.md", "task-1")
	tracker.AddExpected("02-add-logging.md", "task-2")

	// Pre-scan marks files as generated (simulates scanExistingDraftFiles)
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	preScanCount := gen.scanExistingDraftFiles(draftDir, tracker)
	if preScanCount != 3 {
		t.Fatalf("pre-scan found %d files, want 3", preScanCount)
	}

	// Now scanGeneratedIssueFilesWithTracker should accept these old files
	// because they are already in tracker.GeneratedFiles
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, tracker)
	if err != nil {
		t.Fatalf("scanGeneratedIssueFilesWithTracker() error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("scanGeneratedIssueFilesWithTracker() returned %d files, want 3", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_RejectsOldFilesWithoutPreScan(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create file with OLD timestamp, NOT pre-scanned
	oldTime := time.Now().Add(-2 * time.Hour)
	fpath := filepath.Join(draftDir, "01-task.md")
	os.WriteFile(fpath, []byte("# Task"), 0o644)
	os.Chtimes(fpath, oldTime, oldTime)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")
	// Deliberately NOT calling scanExistingDraftFiles

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, tracker)
	if err != nil {
		t.Fatalf("scanGeneratedIssueFilesWithTracker() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("old files without pre-scan should be rejected, got %d files", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_MixedPreScannedAndNew(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// File 1: old timestamp, will be pre-scanned
	oldTime := time.Now().Add(-2 * time.Hour)
	fpath1 := filepath.Join(draftDir, "01-implement-auth.md")
	os.WriteFile(fpath1, []byte("# Auth"), 0o644)
	os.Chtimes(fpath1, oldTime, oldTime)

	// File 2: new timestamp, generated in current run
	fpath2 := filepath.Join(draftDir, "02-add-logging.md")
	os.WriteFile(fpath2, []byte("# Logging"), 0o644)

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-implement-auth.md", "task-1")
	tracker.AddExpected("02-add-logging.md", "task-2")

	// Pre-scan only file 1
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	gen.scanExistingDraftFiles(draftDir, tracker)

	// Both should be accepted: file 1 via pre-scan, file 2 via IsValidFile
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, tracker)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files (1 pre-scanned + 1 new), got %d", len(files))
	}
}

func TestGenerator_MissingFilterExcludesPreScannedFiles(t *testing.T) {
	t.Parallel()

	// Simulate the filtering logic from GenerateIssueFiles
	expected := []expectedIssueFile{
		{FileName: "00-consolidated-analysis.md", IsMain: true},
		{FileName: "01-implement-auth.md", TaskID: "task-1"},
		{FileName: "02-add-logging.md", TaskID: "task-2"},
		{FileName: "03-add-tests.md", TaskID: "task-3"},
	}

	tracker := NewGenerationTracker("wf-test")
	for _, exp := range expected {
		tracker.AddExpected(exp.FileName, exp.TaskID)
	}

	// Pre-scan found 2 of 4 files
	tracker.MarkGenerated("00-consolidated-analysis.md", time.Now().Add(-1*time.Hour))
	tracker.MarkGenerated("01-implement-auth.md", time.Now().Add(-1*time.Hour))

	// Apply the same filter as GenerateIssueFiles
	var missing []expectedIssueFile
	for _, exp := range expected {
		if _, exists := tracker.GeneratedFiles[exp.FileName]; !exists {
			missing = append(missing, exp)
		}
	}

	if len(missing) != 2 {
		t.Fatalf("expected 2 missing files, got %d", len(missing))
	}
	if missing[0].FileName != "02-add-logging.md" {
		t.Errorf("missing[0] = %q, want '02-add-logging.md'", missing[0].FileName)
	}
	if missing[1].FileName != "03-add-tests.md" {
		t.Errorf("missing[1] = %q, want '03-add-tests.md'", missing[1].FileName)
	}
}

func TestGenerator_MissingFilterAllPreScanned(t *testing.T) {
	t.Parallel()

	expected := []expectedIssueFile{
		{FileName: "00-consolidated-analysis.md", IsMain: true},
		{FileName: "01-implement-auth.md", TaskID: "task-1"},
	}

	tracker := NewGenerationTracker("wf-test")
	for _, exp := range expected {
		tracker.AddExpected(exp.FileName, exp.TaskID)
	}

	// All files pre-scanned
	tracker.MarkGenerated("00-consolidated-analysis.md", time.Now().Add(-1*time.Hour))
	tracker.MarkGenerated("01-implement-auth.md", time.Now().Add(-1*time.Hour))

	var missing []expectedIssueFile
	for _, exp := range expected {
		if _, exists := tracker.GeneratedFiles[exp.FileName]; !exists {
			missing = append(missing, exp)
		}
	}

	if len(missing) != 0 {
		t.Errorf("expected 0 missing files when all pre-scanned, got %d", len(missing))
	}
}

func TestGenerator_LastMissingFiles_SetOnPartialResults(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)

	// Initially nil
	if gen.LastMissingFiles != nil {
		t.Error("LastMissingFiles should be nil initially")
	}

	// Simulate setting it (as GenerateIssueFiles does)
	gen.LastMissingFiles = []string{"03-missing-task.md", "04-another-missing.md"}
	if len(gen.LastMissingFiles) != 2 {
		t.Errorf("LastMissingFiles has %d entries, want 2", len(gen.LastMissingFiles))
	}

	// Simulate reset on next successful run
	gen.LastMissingFiles = nil
	if gen.LastMissingFiles != nil {
		t.Error("LastMissingFiles should be nil after reset")
	}
}

func TestGenerator_ScanExistingDraftFiles_AllExpectedPresent(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Simulate a complete previous run: all 23 expected files exist
	fileNames := make([]string, 23)
	fileNames[0] = "00-consolidated-analysis.md"
	for i := 1; i < 23; i++ {
		fileNames[i] = fmt.Sprintf("%02d-task-%d.md", i, i)
	}

	tracker := NewGenerationTracker("wf-test")
	for i, name := range fileNames {
		os.WriteFile(filepath.Join(draftDir, name), []byte(fmt.Sprintf("# Issue %d\nContent", i)), 0o644)
		taskID := fmt.Sprintf("task-%d", i)
		if i == 0 {
			taskID = "main"
		}
		tracker.AddExpected(name, taskID)
	}

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	count := gen.scanExistingDraftFiles(draftDir, tracker)

	if count != 23 {
		t.Errorf("scanExistingDraftFiles() = %d, want 23", count)
	}
	missing := tracker.GetMissingFiles()
	if len(missing) != 0 {
		t.Errorf("expected 0 missing files, got %d: %v", len(missing), missing)
	}

	// Verify scanGeneratedIssueFilesWithTracker also returns all 23
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, tracker)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 23 {
		t.Errorf("scanGeneratedIssueFilesWithTracker() returned %d files, want 23", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_SkipsEmptyFiles(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Create an empty file that was pre-scanned would skip, but let's verify
	// scanGeneratedIssueFilesWithTracker also handles it
	fpath := filepath.Join(draftDir, "01-task.md")
	os.WriteFile(fpath, []byte(""), 0o644) // empty

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, tracker)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("empty files should be skipped, got %d files", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_NonExistentDir(t *testing.T) {
	t.Parallel()
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	tracker := NewGenerationTracker("wf-test")

	files, err := gen.scanGeneratedIssueFilesWithTracker("/nonexistent/dir", tracker)
	if err != nil {
		t.Errorf("non-existent dir should return nil, nil; got error: %v", err)
	}
	if files != nil {
		t.Errorf("expected nil files for non-existent dir, got %d", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_NilTracker(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Recent file should be accepted by the 30-minute fallback
	os.WriteFile(filepath.Join(draftDir, "01-task.md"), []byte("# Task"), 0o644)

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("recent file with nil tracker should be accepted, got %d files", len(files))
	}
}

func TestGenerator_ScanGeneratedFilesWithTracker_NilTrackerOldFile(t *testing.T) {
	t.Parallel()
	draftDir := t.TempDir()

	// Old file should be rejected by the 30-minute fallback
	oldTime := time.Now().Add(-2 * time.Hour)
	fpath := filepath.Join(draftDir, "01-task.md")
	os.WriteFile(fpath, []byte("# Task"), 0o644)
	os.Chtimes(fpath, oldTime, oldTime)

	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	files, err := gen.scanGeneratedIssueFilesWithTracker(draftDir, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("old file with nil tracker should be rejected, got %d files", len(files))
	}
}
