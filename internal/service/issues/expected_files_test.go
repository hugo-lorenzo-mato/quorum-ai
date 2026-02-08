package issues

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func TestBuildExpectedIssueFiles_WithConsolidatedAndTasks(t *testing.T) {
	gen := &Generator{}

	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth", Index: 1},
		{ID: "task-2", Slug: "add-tests", Index: 2},
	}

	expected := gen.buildExpectedIssueFiles("/path/to/consolidated.md", taskFiles)

	// Should include consolidated + 2 tasks = 3
	if len(expected) != 3 {
		t.Fatalf("expected 3 files, got %d", len(expected))
	}

	// First should be main issue
	if !expected[0].IsMain {
		t.Error("expected first entry to be main issue")
	}
	if expected[0].TaskID != "main" {
		t.Errorf("main issue TaskID = %q, want 'main'", expected[0].TaskID)
	}
	if expected[0].FileName != mainIssueFilename {
		t.Errorf("main issue FileName = %q, want %q", expected[0].FileName, mainIssueFilename)
	}

	// Second should be task-1
	if expected[1].TaskID != "task-1" {
		t.Errorf("second entry TaskID = %q, want 'task-1'", expected[1].TaskID)
	}
	if expected[1].IsMain {
		t.Error("expected second entry to not be main issue")
	}
	if expected[1].FileName != "01-implement-auth.md" {
		t.Errorf("second entry FileName = %q, want '01-implement-auth.md'", expected[1].FileName)
	}
	if expected[1].Task == nil {
		t.Error("expected second entry to have Task reference")
	}

	// Third should be task-2
	if expected[2].TaskID != "task-2" {
		t.Errorf("third entry TaskID = %q, want 'task-2'", expected[2].TaskID)
	}
	if expected[2].FileName != "02-add-tests.md" {
		t.Errorf("third entry FileName = %q, want '02-add-tests.md'", expected[2].FileName)
	}
}

func TestBuildExpectedIssueFiles_NoConsolidated(t *testing.T) {
	gen := &Generator{}

	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "single-task", Index: 1},
	}

	expected := gen.buildExpectedIssueFiles("", taskFiles)

	// Without consolidated, should only have task files
	if len(expected) != 1 {
		t.Fatalf("expected 1 file, got %d", len(expected))
	}

	if expected[0].IsMain {
		t.Error("expected no main issue when consolidated path is empty")
	}
	if expected[0].TaskID != "task-1" {
		t.Errorf("TaskID = %q, want 'task-1'", expected[0].TaskID)
	}
}

func TestBuildExpectedIssueFiles_EmptyTaskFiles(t *testing.T) {
	gen := &Generator{}

	expected := gen.buildExpectedIssueFiles("/path/to/consolidated.md", nil)

	// Should only have consolidated
	if len(expected) != 1 {
		t.Fatalf("expected 1 file, got %d", len(expected))
	}

	if !expected[0].IsMain {
		t.Error("expected main issue for consolidated")
	}
}

func TestBuildExpectedIssueFiles_NeitherConsolidatedNorTasks(t *testing.T) {
	gen := &Generator{}

	expected := gen.buildExpectedIssueFiles("", nil)

	if len(expected) != 0 {
		t.Errorf("expected 0 files, got %d", len(expected))
	}
}

func TestBuildExpectedIssueFiles_TaskReferenceIndependence(t *testing.T) {
	gen := &Generator{}

	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "auth", Index: 1},
	}

	expected := gen.buildExpectedIssueFiles("", taskFiles)

	// Modifying the original task should not affect the expected file's Task pointer
	// (the function takes a copy via `t := task`)
	taskFiles[0].Slug = "modified"

	if expected[0].Task.Slug != "auth" {
		t.Error("expected Task reference to be independent of original slice")
	}
}

func TestExpectedIssueFile_Fields(t *testing.T) {
	task := &service.IssueTaskFile{ID: "task-1", Slug: "test", Index: 1}
	eif := expectedIssueFile{
		FileName: "01-test.md",
		TaskID:   "task-1",
		IsMain:   false,
		Task:     task,
	}

	if eif.FileName != "01-test.md" {
		t.Errorf("FileName = %q, want '01-test.md'", eif.FileName)
	}
	if eif.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want 'task-1'", eif.TaskID)
	}
	if eif.IsMain {
		t.Error("expected IsMain=false")
	}
	if eif.Task != task {
		t.Error("expected Task pointer to match")
	}
}

func TestBuildExpectedIssueFiles_ManyTasks(t *testing.T) {
	gen := &Generator{}

	taskFiles := make([]service.IssueTaskFile, 15)
	for i := range taskFiles {
		taskFiles[i] = service.IssueTaskFile{
			ID:    "task-" + string(rune('a'+i)),
			Slug:  "task-slug",
			Index: i + 1,
		}
	}

	expected := gen.buildExpectedIssueFiles("/consolidated.md", taskFiles)

	// 1 consolidated + 15 tasks = 16
	if len(expected) != 16 {
		t.Errorf("expected 16 files, got %d", len(expected))
	}

	// Verify file naming for double-digit indices
	for _, ef := range expected[1:] {
		if ef.FileName == "" {
			t.Error("expected non-empty filename for all tasks")
		}
	}
}
