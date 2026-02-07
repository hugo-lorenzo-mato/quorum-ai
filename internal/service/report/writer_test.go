package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowReportWriter_TaskPlanPath(t *testing.T) {
	cfg := Config{
		BaseDir: "/tmp/quorum-test",
		Enabled: true,
	}
	writer := NewWorkflowReportWriter(cfg, "wf-test-123")

	tests := []struct {
		name     string
		taskID   string
		taskName string
		wantEnd  string
	}{
		{
			name:     "simple task",
			taskID:   "task-1",
			taskName: "Create web server",
			wantEnd:  "tasks/task-1-create-web-server.md",
		},
		{
			name:     "task with special chars",
			taskID:   "task-2",
			taskName: "Fix bug/issue #123",
			wantEnd:  "tasks/task-2-fix-bug-issue-123.md",
		},
		{
			name:     "task with spaces",
			taskID:   "task-3",
			taskName: "Update   multiple   spaces",
			wantEnd:  "tasks/task-3-update-multiple-spaces.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := writer.TaskPlanPath(tt.taskID, tt.taskName)

			if !strings.HasSuffix(got, tt.wantEnd) {
				t.Errorf("TaskPlanPath() = %q, want suffix %q", got, tt.wantEnd)
			}

			// Should be under plan-phase directory
			if !strings.Contains(got, "plan-phase") {
				t.Error("TaskPlanPath() should be under plan-phase directory")
			}
		})
	}
}

func TestWorkflowReportWriter_EnsureTasksDir(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "quorum-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		BaseDir: tmpDir,
		Enabled: true,
	}
	writer := NewWorkflowReportWriter(cfg, "wf-test-123")

	// EnsureTasksDir should create the directory
	err = writer.EnsureTasksDir()
	if err != nil {
		t.Fatalf("EnsureTasksDir() error = %v", err)
	}

	// Check that the directory was created
	tasksDir := filepath.Join(writer.PlanPhasePath(), "tasks")
	info, err := os.Stat(tasksDir)
	if err != nil {
		t.Fatalf("tasks directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("tasks path should be a directory")
	}

	// Calling again should be idempotent
	err = writer.EnsureTasksDir()
	if err != nil {
		t.Fatalf("EnsureTasksDir() second call error = %v", err)
	}
}

func TestWorkflowReportWriter_EnsureTasksDir_Disabled(t *testing.T) {
	cfg := Config{
		BaseDir: "/nonexistent/path",
		Enabled: false, // Disabled
	}
	writer := NewWorkflowReportWriter(cfg, "wf-test-123")

	// Should return nil without creating anything
	err := writer.EnsureTasksDir()
	if err != nil {
		t.Fatalf("EnsureTasksDir() with disabled config should not error: %v", err)
	}
}

func TestWorkflowReportWriter_Resume_CreatesMissingDirsOnFirstWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "quorum-test-resume-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		BaseDir: tmpDir,
		Enabled: true,
	}
	workflowID := "wf-test-resume-123"

	// Simulate "API created only the execution directory" scenario.
	executionDir := filepath.Join(tmpDir, workflowID)
	if err := os.MkdirAll(executionDir, 0o755); err != nil {
		t.Fatalf("failed to create execution dir: %v", err)
	}

	writer := ResumeWorkflowReportWriter(cfg, workflowID, executionDir)

	if err := writer.WriteOriginalPrompt("hello"); err != nil {
		t.Fatalf("WriteOriginalPrompt() error = %v", err)
	}

	path := filepath.Join(writer.AnalyzePhasePath(), "00-original-prompt.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected original prompt report to exist at %s: %v", path, err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple name", "simple-name"},
		{"with/slash", "with-slash"},
		{"with\\backslash", "withbackslash"}, // backslash is dropped
		{"with:colon", "with-colon"},
		{"UPPERCASE", "uppercase"},
		{"multiple   spaces", "multiple-spaces"},  // consecutive spaces collapse to single dash
		{"--leading-dashes", "-leading-dashes"},   // consecutive dashes collapse
		{"trailing-dashes--", "trailing-dashes-"}, // consecutive dashes collapse
		{"file.name.txt", "file.name.txt"},        // dots are preserved
		{"under_score", "under_score"},            // underscores are preserved
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
