package issues

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeduplicator_GetOrCreateState_New(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	state, exists, err := dedup.GetOrCreateState("workflow-123", "checksum-abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected new state, got existing")
	}
	if state.WorkflowID != "workflow-123" {
		t.Errorf("expected WorkflowID='workflow-123', got '%s'", state.WorkflowID)
	}
	if state.InputChecksum != "checksum-abc" {
		t.Errorf("expected InputChecksum='checksum-abc', got '%s'", state.InputChecksum)
	}
	if state.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestDeduplicator_GetOrCreateState_Existing(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// Create initial state
	state1, _, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	dedup.MarkComplete(state1)
	if err := dedup.Save(state1); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Get existing state
	state2, exists, err := dedup.GetOrCreateState("workflow-123", "checksum-abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected existing state")
	}
	if !state2.IsComplete() {
		t.Error("expected state to be complete")
	}
}

func TestDeduplicator_GetOrCreateState_ChecksumMismatch(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// Create initial state with one checksum
	state1, _, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	dedup.MarkComplete(state1)
	if err := dedup.Save(state1); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Try to get with different checksum
	state2, exists, err := dedup.GetOrCreateState("workflow-123", "checksum-xyz")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return new state since checksum changed
	if exists {
		t.Error("expected new state due to checksum mismatch")
	}
	if state2.InputChecksum != "checksum-xyz" {
		t.Errorf("expected new checksum, got '%s'", state2.InputChecksum)
	}
}

func TestGenerationState_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		state    GenerationState
		expected bool
	}{
		{
			name: "complete with no error",
			state: GenerationState{
				CompletedAt:  func() *time.Time { t := time.Now(); return &t }(),
				ErrorMessage: "",
			},
			expected: true,
		},
		{
			name: "not completed",
			state: GenerationState{
				CompletedAt:  nil,
				ErrorMessage: "",
			},
			expected: false,
		},
		{
			name: "completed with error",
			state: GenerationState{
				CompletedAt:  func() *time.Time { t := time.Now(); return &t }(),
				ErrorMessage: "some error",
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.state.IsComplete()
			if result != tc.expected {
				t.Errorf("IsComplete() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestGenerationState_GetGeneratedFilePaths(t *testing.T) {
	state := &GenerationState{
		GeneratedFiles: []GeneratedFileInfo{
			{Filename: "00-consolidated.md"},
			{Filename: "01-task-1.md"},
			{Filename: "02-task-2.md"},
		},
	}

	paths := state.GetGeneratedFilePaths("/base/dir")

	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}

	expected := []string{
		filepath.Join("/base/dir", "00-consolidated.md"),
		filepath.Join("/base/dir", "01-task-1.md"),
		filepath.Join("/base/dir", "02-task-2.md"),
	}

	for i, path := range paths {
		if path != expected[i] {
			t.Errorf("path[%d] = %q, want %q", i, path, expected[i])
		}
	}
}

func TestDeduplicator_MarkFileGenerated(t *testing.T) {
	dedup := NewDeduplicator(t.TempDir())
	state := &GenerationState{WorkflowID: "test"}

	dedup.MarkFileGenerated(state, "test.md", "task-1", false, []byte("content"))

	if len(state.GeneratedFiles) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(state.GeneratedFiles))
	}

	file := state.GeneratedFiles[0]
	if file.Filename != "test.md" {
		t.Errorf("expected Filename='test.md', got '%s'", file.Filename)
	}
	if file.TaskID != "task-1" {
		t.Errorf("expected TaskID='task-1', got '%s'", file.TaskID)
	}
	if file.IsMain {
		t.Error("expected IsMain=false")
	}
	if file.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
}

func TestDeduplicator_MarkIssueCreated(t *testing.T) {
	dedup := NewDeduplicator(t.TempDir())
	state := &GenerationState{WorkflowID: "test"}

	dedup.MarkIssueCreated(state, 42, "https://github.com/org/repo/issues/42", "task-1", false)

	if len(state.CreatedIssues) != 1 {
		t.Fatalf("expected 1 created issue, got %d", len(state.CreatedIssues))
	}

	issue := state.CreatedIssues[0]
	if issue.Number != 42 {
		t.Errorf("expected Number=42, got %d", issue.Number)
	}
	if issue.URL != "https://github.com/org/repo/issues/42" {
		t.Errorf("unexpected URL: %s", issue.URL)
	}
	if issue.TaskID != "task-1" {
		t.Errorf("expected TaskID='task-1', got '%s'", issue.TaskID)
	}
}

func TestDeduplicator_MarkComplete(t *testing.T) {
	dedup := NewDeduplicator(t.TempDir())
	state := &GenerationState{
		WorkflowID:   "test",
		ErrorMessage: "previous error",
	}

	dedup.MarkComplete(state)

	if state.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if state.ErrorMessage != "" {
		t.Error("expected ErrorMessage to be cleared")
	}
	if !state.IsComplete() {
		t.Error("expected IsComplete() to return true")
	}
}

func TestDeduplicator_MarkFailed(t *testing.T) {
	dedup := NewDeduplicator(t.TempDir())
	state := &GenerationState{WorkflowID: "test"}

	testErr := &testError{msg: "generation failed"}
	dedup.MarkFailed(state, testErr)

	if state.ErrorMessage != "generation failed" {
		t.Errorf("expected ErrorMessage='generation failed', got '%s'", state.ErrorMessage)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestDeduplicator_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// Create state with data
	state := &GenerationState{
		WorkflowID:    "workflow-123",
		InputChecksum: "checksum-abc",
		StartedAt:     time.Now(),
		GeneratedFiles: []GeneratedFileInfo{
			{Filename: "test.md", TaskID: "task-1", Checksum: "abc123"},
		},
		CreatedIssues: []CreatedIssueInfo{
			{Number: 42, URL: "https://example.com/42", TaskID: "task-1"},
		},
	}

	// Save
	if err := dedup.Save(state); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load
	loaded, exists, err := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if !exists {
		t.Error("expected state to exist")
	}

	// Verify loaded state
	if loaded.WorkflowID != state.WorkflowID {
		t.Errorf("WorkflowID mismatch")
	}
	if len(loaded.GeneratedFiles) != 1 {
		t.Errorf("expected 1 generated file, got %d", len(loaded.GeneratedFiles))
	}
	if len(loaded.CreatedIssues) != 1 {
		t.Errorf("expected 1 created issue, got %d", len(loaded.CreatedIssues))
	}
}

func TestDeduplicator_Delete(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// Create and save state
	state, _, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	if err := dedup.Save(state); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Delete
	if err := dedup.Delete("workflow-123"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify deleted
	_, exists, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	if exists {
		t.Error("expected state to be deleted")
	}
}

func TestDeduplicator_Delete_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// Delete non-existent should not error
	if err := dedup.Delete("non-existent"); err != nil {
		t.Errorf("delete non-existent should not error, got: %v", err)
	}
}

func TestDeduplicator_HasExistingIssues(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// No state yet
	has, count := dedup.HasExistingIssues("workflow-123")
	if has || count != 0 {
		t.Error("expected no existing issues for non-existent workflow")
	}

	// Create state with issues
	state, _, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	dedup.MarkIssueCreated(state, 1, "url1", "task-1", false)
	dedup.MarkIssueCreated(state, 2, "url2", "task-2", false)
	dedup.Save(state)

	has, count = dedup.HasExistingIssues("workflow-123")
	if !has {
		t.Error("expected existing issues")
	}
	if count != 2 {
		t.Errorf("expected 2 issues, got %d", count)
	}
}

func TestDeduplicator_GetExistingIssueNumbers(t *testing.T) {
	tempDir := t.TempDir()
	dedup := NewDeduplicator(tempDir)

	// No state yet
	numbers := dedup.GetExistingIssueNumbers("workflow-123")
	if numbers != nil {
		t.Error("expected nil for non-existent workflow")
	}

	// Create state with issues
	state, _, _ := dedup.GetOrCreateState("workflow-123", "checksum-abc")
	dedup.MarkIssueCreated(state, 42, "url1", "task-1", false)
	dedup.MarkIssueCreated(state, 43, "url2", "task-2", false)
	dedup.Save(state)

	numbers = dedup.GetExistingIssueNumbers("workflow-123")
	if len(numbers) != 2 {
		t.Fatalf("expected 2 numbers, got %d", len(numbers))
	}
	if numbers[0] != 42 || numbers[1] != 43 {
		t.Errorf("unexpected numbers: %v", numbers)
	}
}

func TestCalculateInputChecksum(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	consolidatedPath := filepath.Join(tempDir, "consolidated.md")
	task1Path := filepath.Join(tempDir, "task1.md")
	task2Path := filepath.Join(tempDir, "task2.md")

	os.WriteFile(consolidatedPath, []byte("consolidated content"), 0644)
	os.WriteFile(task1Path, []byte("task 1 content"), 0644)
	os.WriteFile(task2Path, []byte("task 2 content"), 0644)

	// Calculate checksum
	checksum, err := CalculateInputChecksum(consolidatedPath, []string{task1Path, task2Path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// Same input should give same checksum
	checksum2, _ := CalculateInputChecksum(consolidatedPath, []string{task1Path, task2Path})
	if checksum != checksum2 {
		t.Error("expected same checksum for same input")
	}

	// Different order should give same checksum (sorted)
	checksum3, _ := CalculateInputChecksum(consolidatedPath, []string{task2Path, task1Path})
	if checksum != checksum3 {
		t.Error("expected same checksum regardless of task order")
	}
}

func TestCalculateInputChecksum_DifferentContent(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	consolidatedPath := filepath.Join(tempDir, "consolidated.md")
	task1Path := filepath.Join(tempDir, "task1.md")

	os.WriteFile(consolidatedPath, []byte("content v1"), 0644)
	os.WriteFile(task1Path, []byte("task content"), 0644)

	checksum1, _ := CalculateInputChecksum(consolidatedPath, []string{task1Path})

	// Change content
	os.WriteFile(consolidatedPath, []byte("content v2"), 0644)

	checksum2, _ := CalculateInputChecksum(consolidatedPath, []string{task1Path})

	if checksum1 == checksum2 {
		t.Error("expected different checksums for different content")
	}
}

func TestCalculateInputChecksum_EmptyConsolidated(t *testing.T) {
	tempDir := t.TempDir()

	taskPath := filepath.Join(tempDir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	// Empty consolidated path should work
	checksum, err := CalculateInputChecksum("", []string{taskPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checksum == "" {
		t.Error("expected non-empty checksum")
	}
}

func TestCalculateInputChecksum_FileNotFound(t *testing.T) {
	_, err := CalculateInputChecksum("/nonexistent/file.md", nil)

	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestChecksumBytes(t *testing.T) {
	data := []byte("test content")

	checksum := checksumBytes(data)

	if checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if len(checksum) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected 64 char checksum, got %d", len(checksum))
	}

	// Same data should give same checksum
	checksum2 := checksumBytes(data)
	if checksum != checksum2 {
		t.Error("expected same checksum for same data")
	}

	// Different data should give different checksum
	checksum3 := checksumBytes([]byte("different content"))
	if checksum == checksum3 {
		t.Error("expected different checksum for different data")
	}
}
