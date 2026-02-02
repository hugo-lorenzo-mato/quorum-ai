package issues

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

	gen := NewGenerator(client, cfg, tmpDir, nil)

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

	gen := NewGenerator(client, cfg, tmpDir, nil)

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
			name:     "main issue",
			isMain:   true,
			want:     "[quorum] Workflow Summary",
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
