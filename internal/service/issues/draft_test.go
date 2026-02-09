package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func newTestGenerator(t *testing.T) (*Generator, string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.IssuesConfig{
		Enabled:  true,
		Provider: "github",
		Labels:   []string{"test"},
	}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)
	return gen, tmpDir
}

func TestRenderDraftContent(t *testing.T) {
	fm := DraftFrontmatter{
		Title:       "Test Issue",
		Labels:      []string{"bug", "enhancement"},
		Assignees:   []string{"user1"},
		IsMainIssue: false,
		TaskID:      "task-1",
		SourcePath:  "path/to/source.md",
		Status:      "draft",
	}

	content := renderDraftContent(fm, "## Summary\n\nThis is the body.")

	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter delimiters")
	}
	if !strings.Contains(content, "title: Test Issue") && !strings.Contains(content, "title: \"Test Issue\"") {
		t.Error("expected title in frontmatter")
	}
	if !strings.Contains(content, "task_id: task-1") && !strings.Contains(content, "task_id: \"task-1\"") {
		t.Error("expected task_id in frontmatter")
	}
	if !strings.Contains(content, "## Summary") {
		t.Error("expected body content")
	}
}

func TestRenderDraftContent_EmptyBody(t *testing.T) {
	fm := DraftFrontmatter{
		Title:  "Title Only",
		Status: "draft",
	}

	content := renderDraftContent(fm, "")

	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter delimiters")
	}
	if !strings.Contains(content, "title:") {
		t.Error("expected title in frontmatter")
	}
}

func TestRenderDraftContent_EmptyLabelsAndAssignees(t *testing.T) {
	fm := DraftFrontmatter{
		Title:     "Minimal Issue",
		Labels:    nil,
		Assignees: nil,
		Status:    "draft",
	}

	content := renderDraftContent(fm, "Body text")

	// Should still produce valid content
	if !strings.Contains(content, "Body text") {
		t.Error("expected body content")
	}
}

func TestParseDraftContent_Valid(t *testing.T) {
	content := `---
title: Test Issue
labels:
  - bug
  - enhancement
assignees:
  - user1
is_main_issue: false
task_id: task-1
source_path: path/to/source.md
status: draft
---
## Summary

This is the body.`

	fm, body, err := parseDraftContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm.Title != "Test Issue" {
		t.Errorf("Title = %q, want 'Test Issue'", fm.Title)
	}
	if len(fm.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(fm.Labels))
	}
	if fm.Labels[0] != "bug" {
		t.Errorf("Labels[0] = %q, want 'bug'", fm.Labels[0])
	}
	if len(fm.Assignees) != 1 || fm.Assignees[0] != "user1" {
		t.Errorf("Assignees = %v, want [user1]", fm.Assignees)
	}
	if fm.IsMainIssue {
		t.Error("expected IsMainIssue=false")
	}
	if fm.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want 'task-1'", fm.TaskID)
	}
	if fm.SourcePath != "path/to/source.md" {
		t.Errorf("SourcePath = %q, want 'path/to/source.md'", fm.SourcePath)
	}
	if fm.Status != "draft" {
		t.Errorf("Status = %q, want 'draft'", fm.Status)
	}
	if !strings.Contains(body, "## Summary") {
		t.Error("expected body to contain '## Summary'")
	}
}

func TestParseDraftContent_NoFrontmatter(t *testing.T) {
	content := "# Title\n\nBody content"

	_, _, err := parseDraftContent(content)
	if err == nil {
		t.Error("expected error for content without frontmatter")
	}
}

func TestParseDraftContent_InvalidYAML(t *testing.T) {
	content := "---\n: invalid: yaml: [\n---\nBody"

	_, _, err := parseDraftContent(content)
	if err == nil {
		t.Error("expected error for invalid YAML in frontmatter")
	}
}

func TestParseDraftContent_MainIssue(t *testing.T) {
	content := `---
title: Consolidated Analysis
is_main_issue: true
task_id: main
status: draft
---
Main issue body.`

	fm, body, err := parseDraftContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fm.IsMainIssue {
		t.Error("expected IsMainIssue=true")
	}
	if fm.TaskID != "main" {
		t.Errorf("TaskID = %q, want 'main'", fm.TaskID)
	}
	if !strings.Contains(body, "Main issue body") {
		t.Error("expected body content")
	}
}

func TestExtractFrontmatter_Valid(t *testing.T) {
	content := "---\ntitle: Test\n---\nBody"

	fm, body, ok := extractFrontmatter(content)
	if !ok {
		t.Error("expected frontmatter to be found")
	}
	if !strings.Contains(fm, "title: Test") {
		t.Errorf("frontmatter = %q, expected to contain 'title: Test'", fm)
	}
	if !strings.Contains(body, "Body") {
		t.Errorf("body = %q, expected to contain 'Body'", body)
	}
}

func TestExtractFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Title\n\nBody content without frontmatter"

	_, body, ok := extractFrontmatter(content)
	if ok {
		t.Error("expected no frontmatter found")
	}
	if body != content {
		t.Errorf("body should be original content when no frontmatter")
	}
}

func TestExtractFrontmatter_EmptyContent(t *testing.T) {
	_, _, ok := extractFrontmatter("")
	if ok {
		t.Error("expected no frontmatter for empty content")
	}
}

func TestExtractFrontmatter_IncompleteDelimiters(t *testing.T) {
	content := "---\ntitle: Test\nNo closing delimiter"

	_, _, ok := extractFrontmatter(content)
	if ok {
		t.Error("expected no frontmatter for incomplete delimiters")
	}
}

func TestWriteDraftFile(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-test-123"

	fm := DraftFrontmatter{
		Title:       "Test Draft",
		Labels:      []string{"bug"},
		Assignees:   []string{"dev1"},
		IsMainIssue: false,
		TaskID:      "task-1",
		Status:      "draft",
	}

	path, err := gen.WriteDraftFile(workflowID, "01-test-draft.md", fm, "Draft body content")
	if err != nil {
		t.Fatalf("WriteDraftFile() error = %v", err)
	}

	// Verify file was written
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Fatalf("draft file not created at %s", path)
	}

	// Verify file is under draft directory
	expectedDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "draft")
	if !strings.HasPrefix(path, expectedDir) {
		t.Errorf("file path %q not under expected directory %q", path, expectedDir)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading draft file: %v", err)
	}
	if !strings.Contains(string(content), "Draft body content") {
		t.Error("draft file should contain body content")
	}
	if !strings.Contains(string(content), "---") {
		t.Error("draft file should contain frontmatter")
	}
}

func TestWriteDraftFile_PathTraversal(t *testing.T) {
	gen, _ := newTestGenerator(t)

	fm := DraftFrontmatter{Title: "Malicious", Status: "draft"}
	_, err := gen.WriteDraftFile("wf-test", "../../../etc/passwd", fm, "evil")
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

func TestWriteDraftFile_CustomDraftDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.IssuesConfig{
		Enabled:        true,
		DraftDirectory: "custom/issues",
	}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)

	fm := DraftFrontmatter{Title: "Custom Dir", Status: "draft"}
	path, err := gen.WriteDraftFile("wf-custom", "test.md", fm, "body")
	if err != nil {
		t.Fatalf("WriteDraftFile() error = %v", err)
	}

	expectedDir := filepath.Join(tmpDir, "custom", "issues", "wf-custom", "draft")
	if !strings.HasPrefix(path, expectedDir) {
		t.Errorf("file path %q not under custom directory %q", path, expectedDir)
	}
}

func TestReadDraftFile(t *testing.T) {
	gen, _ := newTestGenerator(t)
	workflowID := "wf-read-test"

	// Write a draft first
	fm := DraftFrontmatter{
		Title:       "Readable Draft",
		Labels:      []string{"feature"},
		Assignees:   []string{"reader"},
		IsMainIssue: true,
		TaskID:      "main",
		Status:      "draft",
	}
	_, err := gen.WriteDraftFile(workflowID, "00-consolidated.md", fm, "Readable body")
	if err != nil {
		t.Fatalf("WriteDraftFile() error = %v", err)
	}

	// Read it back
	readFM, body, err := gen.ReadDraftFile(workflowID, "00-consolidated.md")
	if err != nil {
		t.Fatalf("ReadDraftFile() error = %v", err)
	}

	if readFM.Title != "Readable Draft" {
		t.Errorf("Title = %q, want 'Readable Draft'", readFM.Title)
	}
	if !readFM.IsMainIssue {
		t.Error("expected IsMainIssue=true")
	}
	if readFM.TaskID != "main" {
		t.Errorf("TaskID = %q, want 'main'", readFM.TaskID)
	}
	if !strings.Contains(body, "Readable body") {
		t.Errorf("body = %q, expected to contain 'Readable body'", body)
	}
}

func TestReadDraftFile_NonExistent(t *testing.T) {
	gen, _ := newTestGenerator(t)

	_, _, err := gen.ReadDraftFile("wf-nonexistent", "missing.md")
	if err == nil {
		t.Error("expected error for non-existent draft file")
	}
}

func TestReadAllDrafts(t *testing.T) {
	gen, _ := newTestGenerator(t)
	workflowID := "wf-all-drafts"

	// Write multiple drafts
	drafts := []struct {
		fileName string
		fm       DraftFrontmatter
		body     string
	}{
		{
			"00-consolidated-analysis.md",
			DraftFrontmatter{Title: "Main Issue", IsMainIssue: true, TaskID: "main", Status: "draft"},
			"Main body",
		},
		{
			"01-implement-auth.md",
			DraftFrontmatter{Title: "Implement Auth", TaskID: "task-1", Labels: []string{"feature"}, Status: "draft"},
			"Auth body",
		},
		{
			"02-add-tests.md",
			DraftFrontmatter{Title: "Add Tests", TaskID: "task-2", Labels: []string{"testing"}, Status: "draft"},
			"Tests body",
		},
	}

	for _, d := range drafts {
		if _, err := gen.WriteDraftFile(workflowID, d.fileName, d.fm, d.body); err != nil {
			t.Fatalf("WriteDraftFile(%q) error = %v", d.fileName, err)
		}
	}

	// Read all drafts
	previews, err := gen.ReadAllDrafts(workflowID)
	if err != nil {
		t.Fatalf("ReadAllDrafts() error = %v", err)
	}

	if len(previews) != 3 {
		t.Fatalf("expected 3 previews, got %d", len(previews))
	}

	// Verify ordering (should be sorted by file number)
	if previews[0].Title != "Main Issue" {
		t.Errorf("previews[0].Title = %q, want 'Main Issue'", previews[0].Title)
	}
	if previews[1].Title != "Implement Auth" {
		t.Errorf("previews[1].Title = %q, want 'Implement Auth'", previews[1].Title)
	}
	if previews[2].Title != "Add Tests" {
		t.Errorf("previews[2].Title = %q, want 'Add Tests'", previews[2].Title)
	}

	// Verify main issue flag
	if !previews[0].IsMainIssue {
		t.Error("expected first preview to be main issue")
	}
	if previews[1].IsMainIssue {
		t.Error("expected second preview to not be main issue")
	}
}

func TestReadAllDrafts_EmptyDirectory(t *testing.T) {
	gen, _ := newTestGenerator(t)

	previews, err := gen.ReadAllDrafts("wf-empty")
	if err != nil {
		t.Fatalf("ReadAllDrafts() error = %v", err)
	}
	if previews != nil {
		t.Errorf("expected nil for empty/non-existent directory, got %v", previews)
	}
}

func TestReadAllDrafts_DeduplicatesByTaskID(t *testing.T) {
	gen, _ := newTestGenerator(t)
	workflowID := "wf-dedup"

	// Write two files with same task ID but different file numbers
	fm := DraftFrontmatter{Title: "First Version", TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "01-task-a.md", fm, "First body"); err != nil {
		t.Fatal(err)
	}
	fm2 := DraftFrontmatter{Title: "Duplicate Version", TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "02-task-b.md", fm2, "Duplicate body"); err != nil {
		t.Fatal(err)
	}

	previews, err := gen.ReadAllDrafts(workflowID)
	if err != nil {
		t.Fatalf("ReadAllDrafts() error = %v", err)
	}

	// Should deduplicate - only first (by sort order) is kept
	if len(previews) != 1 {
		t.Errorf("expected 1 preview after dedup, got %d", len(previews))
	}
	if previews[0].Title != "First Version" {
		t.Errorf("expected first version to be kept, got %q", previews[0].Title)
	}
}

func TestReadAllDrafts_SkipsNonMarkdownFiles(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-nonmd"

	draftDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a markdown file
	fm := DraftFrontmatter{Title: "Valid Draft", TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "01-valid.md", fm, "Valid body"); err != nil {
		t.Fatal(err)
	}

	// Write a non-markdown file
	if err := os.WriteFile(filepath.Join(draftDir, "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	previews, err := gen.ReadAllDrafts(workflowID)
	if err != nil {
		t.Fatalf("ReadAllDrafts() error = %v", err)
	}

	if len(previews) != 1 {
		t.Errorf("expected 1 preview (markdown only), got %d", len(previews))
	}
}

func TestReadAllDrafts_FallbackPlainMarkdown(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-plain"

	draftDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a plain markdown file (no frontmatter)
	plain := "# Plain Issue Title\n\n## Summary\n\nPlain body content.\n"
	if err := os.WriteFile(filepath.Join(draftDir, "00-consolidated.md"), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}

	previews, err := gen.ReadAllDrafts(workflowID)
	if err != nil {
		t.Fatalf("ReadAllDrafts() error = %v", err)
	}

	if len(previews) != 1 {
		t.Fatalf("expected 1 preview, got %d", len(previews))
	}

	if previews[0].Title != "Plain Issue Title" {
		t.Errorf("Title = %q, want 'Plain Issue Title'", previews[0].Title)
	}
	if !previews[0].IsMainIssue {
		t.Error("expected 00-consolidated.md to be treated as main issue")
	}
}

func TestReadIssueMapping_NonExistent(t *testing.T) {
	gen, _ := newTestGenerator(t)

	mapping, err := gen.ReadIssueMapping("wf-no-mapping")
	if err != nil {
		t.Fatalf("ReadIssueMapping() error = %v", err)
	}
	if mapping != nil {
		t.Error("expected nil mapping for non-existent file")
	}
}

func TestReadIssueMapping_Valid(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-mapping"

	publishedDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "published")
	if err := os.MkdirAll(publishedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mappingJSON := `{
		"workflow_id": "wf-mapping",
		"generated_at": "2024-01-15T10:00:00Z",
		"issues": [
			{
				"task_id": "main",
				"file_path": ".quorum/issues/wf-mapping/draft/00-consolidated.md",
				"issue_number": 1,
				"is_main_issue": true
			},
			{
				"task_id": "task-1",
				"file_path": ".quorum/issues/wf-mapping/draft/01-auth.md",
				"issue_number": 2,
				"is_main_issue": false,
				"parent_issue": 1
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(publishedDir, "mapping.json"), []byte(mappingJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	mapping, err := gen.ReadIssueMapping(workflowID)
	if err != nil {
		t.Fatalf("ReadIssueMapping() error = %v", err)
	}

	if mapping == nil {
		t.Fatal("expected non-nil mapping")
	}
	if mapping.WorkflowID != "wf-mapping" {
		t.Errorf("WorkflowID = %q, want 'wf-mapping'", mapping.WorkflowID)
	}
	if len(mapping.Issues) != 2 {
		t.Fatalf("Issues count = %d, want 2", len(mapping.Issues))
	}
	if mapping.Issues[0].IssueNumber != 1 || !mapping.Issues[0].IsMain {
		t.Error("expected first issue to be main issue #1")
	}
	if mapping.Issues[1].IssueNumber != 2 || mapping.Issues[1].ParentIssue != 1 {
		t.Error("expected second issue to be #2 with parent #1")
	}
}

func TestReadIssueMapping_InvalidJSON(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-badjson"

	publishedDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "published")
	if err := os.MkdirAll(publishedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(publishedDir, "mapping.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := gen.ReadIssueMapping(workflowID)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRoundTrip_WriteThenRead(t *testing.T) {
	gen, _ := newTestGenerator(t)
	workflowID := "wf-roundtrip"

	originalFM := DraftFrontmatter{
		Title:       "Round Trip Issue",
		Labels:      []string{"bug", "p1"},
		Assignees:   []string{"alice", "bob"},
		IsMainIssue: false,
		TaskID:      "task-42",
		SourcePath:  "tasks/task-42.md",
		Status:      "draft",
	}
	originalBody := "## Description\n\nThis is a round-trip test.\n\n## Acceptance Criteria\n\n- Works correctly"

	_, err := gen.WriteDraftFile(workflowID, "42-round-trip.md", originalFM, originalBody)
	if err != nil {
		t.Fatalf("WriteDraftFile() error = %v", err)
	}

	readFM, readBody, err := gen.ReadDraftFile(workflowID, "42-round-trip.md")
	if err != nil {
		t.Fatalf("ReadDraftFile() error = %v", err)
	}

	if readFM.Title != originalFM.Title {
		t.Errorf("Title = %q, want %q", readFM.Title, originalFM.Title)
	}
	if len(readFM.Labels) != len(originalFM.Labels) {
		t.Errorf("Labels count = %d, want %d", len(readFM.Labels), len(originalFM.Labels))
	}
	for i, label := range readFM.Labels {
		if label != originalFM.Labels[i] {
			t.Errorf("Labels[%d] = %q, want %q", i, label, originalFM.Labels[i])
		}
	}
	if len(readFM.Assignees) != len(originalFM.Assignees) {
		t.Errorf("Assignees count = %d, want %d", len(readFM.Assignees), len(originalFM.Assignees))
	}
	if readFM.IsMainIssue != originalFM.IsMainIssue {
		t.Errorf("IsMainIssue = %v, want %v", readFM.IsMainIssue, originalFM.IsMainIssue)
	}
	if readFM.TaskID != originalFM.TaskID {
		t.Errorf("TaskID = %q, want %q", readFM.TaskID, originalFM.TaskID)
	}
	if readFM.SourcePath != originalFM.SourcePath {
		t.Errorf("SourcePath = %q, want %q", readFM.SourcePath, originalFM.SourcePath)
	}
	if readFM.Status != originalFM.Status {
		t.Errorf("Status = %q, want %q", readFM.Status, originalFM.Status)
	}
	if !strings.Contains(readBody, "round-trip test") {
		t.Errorf("body = %q, expected to contain 'round-trip test'", readBody)
	}
}

func TestDraftFrontmatter_ZeroValue(t *testing.T) {
	fm := DraftFrontmatter{}
	content := renderDraftContent(fm, "")

	// Should produce valid output even with zero values
	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter delimiters for zero-value struct")
	}

	// Parse back - should work
	parsed, _, err := parseDraftContent(content)
	if err != nil {
		t.Fatalf("unexpected error parsing zero-value frontmatter: %v", err)
	}
	if parsed.Title != "" {
		t.Errorf("expected empty title, got %q", parsed.Title)
	}
	if parsed.IsMainIssue {
		t.Error("expected IsMainIssue=false for zero value")
	}
}
