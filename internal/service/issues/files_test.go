package issues

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func TestWriteIssuesToDisk_Basic(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-write-basic"

	// Create task dir so getTaskFilePaths can work
	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}

	inputs := []IssueInput{
		{
			Title:       "Main Issue",
			Body:        "Main body",
			Labels:      []string{"epic"},
			IsMainIssue: true,
			TaskID:      "main",
		},
		{
			Title:  "Sub Issue 1",
			Body:   "Sub body 1",
			Labels: []string{"feature"},
			TaskID: "task-1",
		},
	}

	previews, err := gen.WriteIssuesToDisk(workflowID, inputs)
	if err != nil {
		t.Fatalf("WriteIssuesToDisk() error = %v", err)
	}

	if len(previews) != 2 {
		t.Fatalf("expected 2 previews, got %d", len(previews))
	}

	// Verify main issue
	if previews[0].Title != "Main Issue" {
		t.Errorf("previews[0].Title = %q, want 'Main Issue'", previews[0].Title)
	}
	if !previews[0].IsMainIssue {
		t.Error("expected first preview to be main issue")
	}

	// Verify files exist on disk
	draftDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "draft")
	entries, err := os.ReadDir(draftDir)
	if err != nil {
		t.Fatalf("reading draft dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 files in draft dir, got %d", len(entries))
	}
}

func TestWriteIssuesToDisk_EmptyWorkflowID(t *testing.T) {
	gen, _ := newTestGenerator(t)

	_, err := gen.WriteIssuesToDisk("", []IssueInput{{Title: "Test"}})
	if err == nil {
		t.Error("expected error for empty workflowID")
	}
}

func TestWriteIssuesToDisk_NoInputs(t *testing.T) {
	gen, _ := newTestGenerator(t)

	_, err := gen.WriteIssuesToDisk("wf-test", nil)
	if err == nil {
		t.Error("expected error for empty inputs")
	}
}

func TestWriteIssuesToDisk_EnsuresEpicLabel(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-epic"

	tasksDir := filepath.Join(tmpDir, "plan-phase", "tasks")
	os.MkdirAll(tasksDir, 0755)

	inputs := []IssueInput{
		{
			Title:       "Main Without Epic",
			Body:        "Body",
			Labels:      []string{"feature"},
			IsMainIssue: true,
		},
	}

	previews, err := gen.WriteIssuesToDisk(workflowID, inputs)
	if err != nil {
		t.Fatalf("WriteIssuesToDisk() error = %v", err)
	}

	// Main issue should have "epic" label added
	hasEpic := false
	for _, label := range previews[0].Labels {
		if label == "epic" {
			hasEpic = true
			break
		}
	}
	if !hasEpic {
		t.Errorf("expected 'epic' label on main issue, got labels: %v", previews[0].Labels)
	}
}

func TestBuildIssueMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  string
	}{
		{
			name:  "normal",
			title: "Test Title",
			body:  "Test body",
			want:  "# Test Title\n\nTest body\n",
		},
		{
			name:  "empty body",
			title: "Title Only",
			body:  "",
			want:  "# Title Only\n",
		},
		{
			name:  "empty title",
			title: "",
			body:  "Some body",
			want:  "# Untitled Issue\n\nSome body\n",
		},
		{
			name:  "whitespace title and body",
			title: "  Trimmed  ",
			body:  "  Body  ",
			want:  "# Trimmed\n\nBody\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildIssueMarkdown(tc.title, tc.body)
			if got != tc.want {
				t.Errorf("buildIssueMarkdown(%q, %q) = %q, want %q", tc.title, tc.body, got, tc.want)
			}
		})
	}
}

func TestUniqueIssueFilename(t *testing.T) {
	used := make(map[string]bool)

	// First use should return as-is
	name1 := uniqueIssueFilename("01-test.md", used)
	if name1 != "01-test.md" {
		t.Errorf("expected '01-test.md', got %q", name1)
	}

	// Second use should append suffix
	name2 := uniqueIssueFilename("01-test.md", used)
	if name2 != "01-test-2.md" {
		t.Errorf("expected '01-test-2.md', got %q", name2)
	}

	// Third use should increment
	name3 := uniqueIssueFilename("01-test.md", used)
	if name3 != "01-test-3.md" {
		t.Errorf("expected '01-test-3.md', got %q", name3)
	}
}

func TestIssueFilenameForTask(t *testing.T) {
	task := service.IssueTaskFile{
		ID:    "task-1",
		Slug:  "implement-auth",
		Index: 1,
	}

	name := issueFilenameForTask(task)
	if name != "01-implement-auth.md" {
		t.Errorf("expected '01-implement-auth.md', got %q", name)
	}

	task2 := service.IssueTaskFile{
		ID:    "task-10",
		Slug:  "large-index",
		Index: 10,
	}
	name2 := issueFilenameForTask(task2)
	if name2 != "10-large-index.md" {
		t.Errorf("expected '10-large-index.md', got %q", name2)
	}
}

func TestEnsureEpicLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   int // expected length
	}{
		{
			name:   "adds epic",
			labels: []string{"bug"},
			want:   2,
		},
		{
			name:   "already has epic",
			labels: []string{"bug", "epic"},
			want:   2,
		},
		{
			name:   "case insensitive",
			labels: []string{"EPIC"},
			want:   1,
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   1,
		},
		{
			name:   "empty labels",
			labels: []string{},
			want:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ensureEpicLabel(tc.labels)
			if len(result) != tc.want {
				t.Errorf("ensureEpicLabel(%v) length = %d, want %d (result=%v)", tc.labels, len(result), tc.want, result)
			}
			// Verify epic exists in result
			found := false
			for _, l := range result {
				if strings.EqualFold(l, "epic") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected 'epic' label in result: %v", result)
			}
		})
	}
}

func TestEnsureLabel(t *testing.T) {
	labels := []string{"bug", "priority:high"}

	result := ensureLabel(labels, "bug")
	if len(result) != 2 {
		t.Error("should not duplicate existing label")
	}

	result2 := ensureLabel(labels, "new-label")
	if len(result2) != 3 {
		t.Error("should add new label")
	}

	// Verify original slice not modified
	if len(labels) != 2 {
		t.Error("original labels should not be modified")
	}
}

func TestResolveIssueFilename(t *testing.T) {
	gen, _ := newTestGenerator(t)

	tasksByID := map[string]service.IssueTaskFile{
		"task-1": {ID: "task-1", Slug: "implement-auth", Index: 1},
		"task-2": {ID: "task-2", Slug: "add-tests", Index: 2},
	}

	tests := []struct {
		name  string
		input IssueInput
		want  string
	}{
		{
			name:  "explicit file path",
			input: IssueInput{FilePath: "custom/path/01-auth.md"},
			want:  "01-auth.md",
		},
		{
			name:  "main issue",
			input: IssueInput{IsMainIssue: true},
			want:  mainIssueFilename,
		},
		{
			name:  "known task",
			input: IssueInput{TaskID: "task-1"},
			want:  "01-implement-auth.md",
		},
		{
			name:  "unknown task with title",
			input: IssueInput{TaskID: "task-99", Title: "New Feature"},
			want:  "03-new-feature.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			used := make(map[string]bool)
			maxIndex := 2
			got := gen.resolveIssueFilename(tc.input, tasksByID, used, &maxIndex)
			if got != tc.want {
				t.Errorf("resolveIssueFilename() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCreateIssuesFromFiles_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.IssuesConfig{
		Enabled: true,
		Labels:  []string{"quorum"},
	}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-create-dry"

	// Write draft files
	fm1 := DraftFrontmatter{Title: "Main Issue", IsMainIssue: true, TaskID: "main", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "00-consolidated.md", fm1, "Main body content"); err != nil {
		t.Fatal(err)
	}
	fm2 := DraftFrontmatter{Title: "Sub Issue", IsMainIssue: false, TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "01-sub.md", fm2, "Sub body content"); err != nil {
		t.Fatal(err)
	}

	baseDir := filepath.Join(".quorum", "issues")
	inputs := []IssueInput{
		{
			Title:       "Main Issue",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
			FilePath:    filepath.Join(baseDir, workflowID, "draft", "00-consolidated.md"),
		},
		{
			Title:    "Sub Issue",
			Body:     "Sub body",
			TaskID:   "task-1",
			FilePath: filepath.Join(baseDir, workflowID, "draft", "01-sub.md"),
		},
	}

	result, err := gen.CreateIssuesFromFiles(context.Background(), workflowID, inputs, true, false, []string{"quorum"}, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromFiles() error = %v", err)
	}

	if len(result.PreviewIssues) != 2 {
		t.Errorf("expected 2 preview issues, got %d", len(result.PreviewIssues))
	}
}

func TestCreateIssuesFromFiles_EmptyWorkflowID(t *testing.T) {
	gen, _ := newTestGenerator(t)

	_, err := gen.CreateIssuesFromFiles(context.Background(), "", nil, false, false, nil, nil)
	if err == nil {
		t.Error("expected error for empty workflowID")
	}
}

func TestCreateIssuesFromFiles_CreateWithMock(t *testing.T) {
	tmpDir := t.TempDir()
	client := &mockIssueClient{}
	cfg := config.IssuesConfig{
		Enabled: true,
		Labels:  []string{"quorum"},
	}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-create-real"

	// Write draft files
	fm1 := DraftFrontmatter{Title: "Main Issue", IsMainIssue: true, TaskID: "main", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "00-main.md", fm1, "Main body"); err != nil {
		t.Fatal(err)
	}
	fm2 := DraftFrontmatter{Title: "Sub Issue", IsMainIssue: false, TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "01-sub.md", fm2, "Sub body"); err != nil {
		t.Fatal(err)
	}

	baseDir := filepath.Join(".quorum", "issues")
	inputs := []IssueInput{
		{
			Title:       "Main Issue",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
			FilePath:    filepath.Join(baseDir, workflowID, "draft", "00-main.md"),
		},
		{
			Title:    "Sub Issue",
			Body:     "Sub body",
			TaskID:   "task-1",
			FilePath: filepath.Join(baseDir, workflowID, "draft", "01-sub.md"),
		},
	}

	result, err := gen.CreateIssuesFromFiles(context.Background(), workflowID, inputs, false, true, []string{"quorum"}, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromFiles() error = %v", err)
	}

	// Verify issues were created
	if result.IssueSet.MainIssue == nil {
		t.Error("expected main issue to be created")
	}
	if len(result.IssueSet.SubIssues) != 1 {
		t.Errorf("expected 1 sub-issue, got %d", len(result.IssueSet.SubIssues))
	}

	// Verify linking
	if result.IssueSet.SubIssues[0].ParentIssue != result.IssueSet.MainIssue.Number {
		t.Error("expected sub-issue to be linked to main issue")
	}

	// Verify mapping file was written
	publishedDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "published")
	mappingPath := filepath.Join(publishedDir, "mapping.json")
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		t.Error("expected mapping.json to be created")
	}
}

func TestCreateIssuesFromFiles_CreateError(t *testing.T) {
	tmpDir := t.TempDir()
	client := &mockIssueClient{
		createErr: &testError{msg: "API error"},
	}
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-error"

	fm := DraftFrontmatter{Title: "Will Fail", IsMainIssue: true, TaskID: "main", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "00-fail.md", fm, "body"); err != nil {
		t.Fatal(err)
	}

	baseDir := filepath.Join(".quorum", "issues")
	inputs := []IssueInput{
		{
			Title:       "Will Fail",
			Body:        "body",
			IsMainIssue: true,
			TaskID:      "main",
			FilePath:    filepath.Join(baseDir, workflowID, "draft", "00-fail.md"),
		},
	}

	_, err := gen.CreateIssuesFromFiles(context.Background(), workflowID, inputs, false, false, nil, nil)
	if err == nil {
		t.Error("expected error when issue creation fails")
	}
}

func TestWriteIssueMappingFile(t *testing.T) {
	gen, tmpDir := newTestGenerator(t)
	workflowID := "wf-mapping-write"

	entries := []IssueMappingEntry{
		{TaskID: "main", FilePath: "00-main.md", IssueNumber: 1, IsMain: true},
		{TaskID: "task-1", FilePath: "01-task.md", IssueNumber: 2, ParentIssue: 1},
	}

	err := gen.writeIssueMappingFile(workflowID, entries)
	if err != nil {
		t.Fatalf("writeIssueMappingFile() error = %v", err)
	}

	// Verify file exists
	mappingPath := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "published", "mapping.json")
	if _, statErr := os.Stat(mappingPath); os.IsNotExist(statErr) {
		t.Error("expected mapping.json to be created")
	}

	// Read and verify
	mapping, err := gen.ReadIssueMapping(workflowID)
	if err != nil {
		t.Fatalf("ReadIssueMapping() error = %v", err)
	}
	if mapping.WorkflowID != workflowID {
		t.Errorf("WorkflowID = %q, want %q", mapping.WorkflowID, workflowID)
	}
	if len(mapping.Issues) != 2 {
		t.Errorf("Issues count = %d, want 2", len(mapping.Issues))
	}
}

func TestResolveDraftDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		draftDir string
		wantSub  string
	}{
		{
			name:     "default",
			draftDir: "",
			wantSub:  ".quorum/issues/wf-test/draft",
		},
		{
			name:     "custom",
			draftDir: "my-issues",
			wantSub:  "my-issues/wf-test/draft",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.IssuesConfig{DraftDirectory: tc.draftDir}
			gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)

			dir, err := gen.resolveDraftDir("wf-test")
			if err != nil {
				t.Fatalf("resolveDraftDir() error = %v", err)
			}

			expected := filepath.Join(tmpDir, tc.wantSub)
			if dir != expected {
				t.Errorf("resolveDraftDir() = %q, want %q", dir, expected)
			}
		})
	}
}

func TestResolvePublishedDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.IssuesConfig{}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)

	dir, err := gen.resolvePublishedDir("wf-test")
	if err != nil {
		t.Fatalf("resolvePublishedDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, ".quorum", "issues", "wf-test", "published")
	if dir != expected {
		t.Errorf("resolvePublishedDir() = %q, want %q", dir, expected)
	}
}

func TestResolveIssuesBaseDir(t *testing.T) {
	tests := []struct {
		name     string
		draftDir string
		want     string
	}{
		{
			name: "default",
			want: filepath.Join(".quorum", "issues"),
		},
		{
			name:     "custom",
			draftDir: "custom/dir",
			want:     "custom/dir",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.IssuesConfig{DraftDirectory: tc.draftDir}
			gen := NewGenerator(nil, cfg, "", "", nil)

			got := gen.resolveIssuesBaseDir()
			if got != tc.want {
				t.Errorf("resolveIssuesBaseDir() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadIssueFile_WithFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-read-fm"

	// Write a draft with frontmatter
	fm := DraftFrontmatter{Title: "From Frontmatter", TaskID: "task-1", Status: "draft"}
	if _, err := gen.WriteDraftFile(workflowID, "01-task.md", fm, "Body from frontmatter"); err != nil {
		t.Fatal(err)
	}

	input := IssueInput{
		TaskID:   "task-1",
		FilePath: "01-task.md",
	}

	title, body, _, err := gen.readIssueFile(workflowID, input)
	if err != nil {
		t.Fatalf("readIssueFile() error = %v", err)
	}

	if title != "From Frontmatter" {
		t.Errorf("title = %q, want 'From Frontmatter'", title)
	}
	if !strings.Contains(body, "Body from frontmatter") {
		t.Errorf("body = %q, expected to contain 'Body from frontmatter'", body)
	}
}

func TestReadIssueFile_PlainMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(nil, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-read-plain"

	// Write a plain markdown file (no frontmatter) in the draft directory
	draftDir := filepath.Join(tmpDir, ".quorum", "issues", workflowID, "draft")
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		t.Fatal(err)
	}
	plain := "# Plain Title\n\nPlain body content."
	if err := os.WriteFile(filepath.Join(draftDir, "01-plain.md"), []byte(plain), 0644); err != nil {
		t.Fatal(err)
	}

	input := IssueInput{
		TaskID:   "task-1",
		FilePath: "01-plain.md",
	}

	title, body, _, err := gen.readIssueFile(workflowID, input)
	if err != nil {
		t.Fatalf("readIssueFile() error = %v", err)
	}

	if title != "Plain Title" {
		t.Errorf("title = %q, want 'Plain Title'", title)
	}
	if !strings.Contains(body, "Plain body content") {
		t.Errorf("body = %q, expected to contain 'Plain body content'", body)
	}
}

func TestReadIssueFile_MissingFilePath(t *testing.T) {
	gen, _ := newTestGenerator(t)

	input := IssueInput{TaskID: "task-1"} // No FilePath

	_, _, _, err := gen.readIssueFile("wf-test", input)
	if err == nil {
		t.Error("expected error for missing file path")
	}
}

func TestReadIssueFile_PathTraversal(t *testing.T) {
	gen, _ := newTestGenerator(t)

	input := IssueInput{
		TaskID:   "task-1",
		FilePath: "../../etc/passwd",
	}

	_, _, _, err := gen.readIssueFile("wf-test", input)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestParseIssueMarkdown(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "standard",
			content:   "# Issue Title\n\n## Summary\n\nBody content.",
			wantTitle: "Issue Title",
			wantBody:  "## Summary\n\nBody content.",
		},
		{
			name:      "no title",
			content:   "## Summary\n\nNo H1 heading.",
			wantTitle: "Untitled Issue",
			wantBody:  "## Summary\n\nNo H1 heading.",
		},
		{
			name:      "title only",
			content:   "# Just a Title",
			wantTitle: "Just a Title",
			wantBody:  "",
		},
		{
			name:      "empty",
			content:   "",
			wantTitle: "Untitled Issue",
			wantBody:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, body := parseIssueMarkdown(tc.content)
			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}
			if body != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

func TestExtractFileNumber(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"00-consolidated.md", 0},
		{"01-task.md", 1},
		{"10-large.md", 10},
		{"readme.md", 9999},
		{"no-number.md", 9999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFileNumber(tc.name)
			if got != tc.want {
				t.Errorf("extractFileNumber(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestSanitizeFilename_Generator(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple Title", "simple-title"},
		{"With Special!@# Chars", "with-special-chars"},
		{"multiple---dashes", "multiple-dashes"},
		{"UPPERCASE", "uppercase"},
		{"", "issue"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIssueMappingEntry_Fields(t *testing.T) {
	entry := IssueMappingEntry{
		TaskID:      "task-1",
		FilePath:    "01-task.md",
		IssueNumber: 42,
		IssueID:     12345,
		IsMain:      false,
		ParentIssue: 1,
	}

	if entry.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want 'task-1'", entry.TaskID)
	}
	if entry.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d, want 42", entry.IssueNumber)
	}
	if entry.ParentIssue != 1 {
		t.Errorf("ParentIssue = %d, want 1", entry.ParentIssue)
	}
}

func TestIssueMapping_Fields(t *testing.T) {
	mapping := IssueMapping{
		WorkflowID: "wf-test",
		Issues: []IssueMappingEntry{
			{TaskID: "main", IssueNumber: 1, IsMain: true},
		},
	}

	if mapping.WorkflowID != "wf-test" {
		t.Errorf("WorkflowID = %q, want 'wf-test'", mapping.WorkflowID)
	}
	if len(mapping.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1", len(mapping.Issues))
	}
}

func TestCreateIssuesFromFiles_SubIssueError(t *testing.T) {
	tmpDir := t.TempDir()
	callCount := 0
	client := &mockIssueClient{
		// Will succeed for main, but let's test sub-issue error path
	}
	// Override CreateIssue to fail on second call
	origCreate := client.CreateIssue
	_ = origCreate

	cfg := config.IssuesConfig{Enabled: true}
	gen := NewGenerator(client, cfg, tmpDir, tmpDir, nil)
	workflowID := "wf-sub-err"

	// Write main draft
	fm1 := DraftFrontmatter{Title: "Main", IsMainIssue: true, TaskID: "main", Status: "draft"}
	gen.WriteDraftFile(workflowID, "00-main.md", fm1, "Main body")

	// Create an input that points to a non-existent file (will cause sub-issue error)
	baseDir := filepath.Join(".quorum", "issues")
	inputs := []IssueInput{
		{
			Title:       "Main",
			Body:        "Main body",
			IsMainIssue: true,
			TaskID:      "main",
			FilePath:    filepath.Join(baseDir, workflowID, "draft", "00-main.md"),
		},
		{
			Title:    "Sub Issue",
			Body:     "Sub body",
			TaskID:   "task-1",
			FilePath: filepath.Join(baseDir, workflowID, "draft", "nonexistent.md"),
		},
	}

	result, err := gen.CreateIssuesFromFiles(context.Background(), workflowID, inputs, false, false, nil, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromFiles() should not fatally error: %v", err)
	}

	// Should have non-fatal error for the sub-issue
	if len(result.Errors) == 0 {
		t.Error("expected at least one non-fatal error for missing sub-issue file")
	}

	_ = callCount
}

func TestGetProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// With explicit project root
	gen := NewGenerator(nil, config.IssuesConfig{}, tmpDir, tmpDir, nil)
	root, err := gen.getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot() error = %v", err)
	}
	if root != tmpDir {
		t.Errorf("getProjectRoot() = %q, want %q", root, tmpDir)
	}

	// Without explicit project root (falls back to os.Getwd)
	gen2 := NewGenerator(nil, config.IssuesConfig{}, "", "", nil)
	root2, err := gen2.getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot() without explicit root: error = %v", err)
	}
	if root2 == "" {
		t.Error("expected non-empty project root from os.Getwd()")
	}
}

func TestGenerateResult_Fields(t *testing.T) {
	result := GenerateResult{
		IssueSet: &core.IssueSet{
			MainIssue: &core.Issue{Number: 1},
			SubIssues: []*core.Issue{{Number: 2}},
		},
		PreviewIssues: []IssuePreview{
			{Title: "Preview", IsMainIssue: true},
		},
		AIUsed: true,
	}

	if result.IssueSet.MainIssue.Number != 1 {
		t.Error("expected MainIssue.Number = 1")
	}
	if !result.AIUsed {
		t.Error("expected AIUsed = true")
	}
	if len(result.PreviewIssues) != 1 {
		t.Error("expected 1 preview issue")
	}
}
