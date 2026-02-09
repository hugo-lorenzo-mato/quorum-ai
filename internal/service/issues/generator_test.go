package issues

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
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
		Template: config.IssueTemplateConfig{
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
	gen := &Generator{
		config: config.IssuesConfig{
			Template: config.IssueTemplateConfig{
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
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)
	if gen.progress != nil {
		t.Error("progress should be nil initially")
	}
	gen.SetProgressReporter(nil)
	gen.emitIssuesGenerationProgress("wf", "stage", 0, 0, nil, "msg")
	gen.emitIssuesPublishingProgress("wf", "stage", 0, 0, nil, 0, false, "msg")
}

func TestGenerator_GetIssueSet(t *testing.T) {
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
	gen := NewGenerator(nil, config.IssuesConfig{}, "", t.TempDir(), nil)
	_, err := gen.ReadGeneratedIssues("../../../etc")
	if err == nil {
		t.Error("ReadGeneratedIssues should reject path traversal in workflowID")
	}
}

func TestGenerator_ReadGeneratedIssues_EmptyDrafts(t *testing.T) {
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
	gen := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), t.TempDir(), nil)
	if err := gen.cleanIssuesDirectory("wf-nonexistent"); err != nil {
		t.Errorf("cleanIssuesDirectory() for non-existent dir should not error: %v", err)
	}
}
