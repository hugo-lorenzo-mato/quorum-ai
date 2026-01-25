package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTaskFile_ValidFile(t *testing.T) {
	content := `# Task: Create Web Server

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: medium
**Dependencies**: None

---

## Context
This is the context section.
`
	tmpFile := createTempTaskFile(t, "task-1-create-web-server.md", content)

	item, err := parseTaskFile(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "task-1", item.ID)
	assert.Equal(t, "Create Web Server", item.Name)
	assert.Equal(t, "claude", item.CLI)
	assert.Equal(t, "medium", item.Complexity)
	assert.Empty(t, item.Dependencies)
	assert.Equal(t, tmpFile, item.File)
}

func TestParseTaskFile_WithDependencies(t *testing.T) {
	content := `# Task: Update Database

**Task ID**: task-5
**Assigned Agent**: gemini
**Complexity**: high
**Dependencies**: task-3, task-4

---
`
	tmpFile := createTempTaskFile(t, "task-5-update-database.md", content)

	item, err := parseTaskFile(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "task-5", item.ID)
	assert.Equal(t, "gemini", item.CLI)
	assert.Equal(t, "high", item.Complexity)
	assert.Equal(t, []string{"task-3", "task-4"}, item.Dependencies)
}

func TestParseTaskFile_MissingTaskID(t *testing.T) {
	content := `# Task: No ID Task

**Assigned Agent**: claude
**Complexity**: low

---
`
	tmpFile := createTempTaskFile(t, "task-1-no-id.md", content)

	_, err := parseTaskFile(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Task ID")
}

func TestParseTaskFile_FallbackNameFromFilename(t *testing.T) {
	content := `**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: low
**Dependencies**: None

---
`
	tmpFile := createTempTaskFile(t, "task-1-implement-feature.md", content)

	item, err := parseTaskFile(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "task-1", item.ID)
	assert.Equal(t, "Implement Feature", item.Name) // Extracted from filename
}

func TestParseTaskFile_DefaultComplexity(t *testing.T) {
	content := `# Task: Simple Task

**Task ID**: task-1
**Assigned Agent**: claude
**Dependencies**: None

---
`
	tmpFile := createTempTaskFile(t, "task-1-simple.md", content)

	item, err := parseTaskFile(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "medium", item.Complexity) // Default value
}

func TestParseDependencies_None(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"None", []string{}},
		{"none", []string{}},
		{"NONE", []string{}},
		{"", []string{}},
		{"  None  ", []string{}},
	}

	for _, tc := range tests {
		result := parseDependencies(tc.input)
		assert.Equal(t, tc.expected, result, "input: %q", tc.input)
	}
}

func TestParseDependencies_Multiple(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"task-1", []string{"task-1"}},
		{"task-1, task-2", []string{"task-1", "task-2"}},
		{"task-1,task-2,task-3", []string{"task-1", "task-2", "task-3"}},
		{"  task-1 ,  task-2  ", []string{"task-1", "task-2"}},
	}

	for _, tc := range tests {
		result := parseDependencies(tc.input)
		assert.Equal(t, tc.expected, result, "input: %q", tc.input)
	}
}

func TestExtractTaskNameFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"task-1-create-web-server.md", "Create Web Server"},
		{"task-10-implement-api.md", "Implement Api"},
		{"task-1-a.md", "A"},
		{"task-99-very-long-task-name.md", "Very Long Task Name"},
	}

	for _, tc := range tests {
		result := extractTaskNameFromFilename(tc.filename)
		assert.Equal(t, tc.expected, result, "filename: %s", tc.filename)
	}
}

func TestComputeExecutionLevels_NoDependencies(t *testing.T) {
	tasks := []TaskManifestItem{
		{ID: "task-1", Dependencies: []string{}},
		{ID: "task-2", Dependencies: []string{}},
		{ID: "task-3", Dependencies: []string{}},
	}

	levels, err := computeExecutionLevels(tasks)
	require.NoError(t, err)

	// All tasks should be in level 0 (can run in parallel)
	assert.Len(t, levels, 1)
	assert.Len(t, levels[0], 3)
	assert.Contains(t, levels[0], "task-1")
	assert.Contains(t, levels[0], "task-2")
	assert.Contains(t, levels[0], "task-3")
}

func TestComputeExecutionLevels_LinearDependencies(t *testing.T) {
	tasks := []TaskManifestItem{
		{ID: "task-1", Dependencies: []string{}},
		{ID: "task-2", Dependencies: []string{"task-1"}},
		{ID: "task-3", Dependencies: []string{"task-2"}},
	}

	levels, err := computeExecutionLevels(tasks)
	require.NoError(t, err)

	// Should have 3 levels (sequential chain)
	assert.Len(t, levels, 3)
	assert.Equal(t, []string{"task-1"}, levels[0])
	assert.Equal(t, []string{"task-2"}, levels[1])
	assert.Equal(t, []string{"task-3"}, levels[2])
}

func TestComputeExecutionLevels_DiamondDependency(t *testing.T) {
	// Diamond pattern:
	//      task-1
	//      /    \
	//   task-2  task-3
	//      \    /
	//      task-4
	tasks := []TaskManifestItem{
		{ID: "task-1", Dependencies: []string{}},
		{ID: "task-2", Dependencies: []string{"task-1"}},
		{ID: "task-3", Dependencies: []string{"task-1"}},
		{ID: "task-4", Dependencies: []string{"task-2", "task-3"}},
	}

	levels, err := computeExecutionLevels(tasks)
	require.NoError(t, err)

	assert.Len(t, levels, 3)
	assert.Equal(t, []string{"task-1"}, levels[0])
	assert.Len(t, levels[1], 2) // task-2 and task-3 can run in parallel
	assert.Contains(t, levels[1], "task-2")
	assert.Contains(t, levels[1], "task-3")
	assert.Equal(t, []string{"task-4"}, levels[2])
}

func TestComputeExecutionLevels_CircularDependency(t *testing.T) {
	tasks := []TaskManifestItem{
		{ID: "task-1", Dependencies: []string{"task-3"}},
		{ID: "task-2", Dependencies: []string{"task-1"}},
		{ID: "task-3", Dependencies: []string{"task-2"}},
	}

	_, err := computeExecutionLevels(tasks)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestComputeExecutionLevels_MissingDependency(t *testing.T) {
	// task-2 depends on task-99 which doesn't exist
	tasks := []TaskManifestItem{
		{ID: "task-1", Dependencies: []string{}},
		{ID: "task-2", Dependencies: []string{"task-99"}},
	}

	// Should succeed but ignore the missing dependency
	levels, err := computeExecutionLevels(tasks)
	require.NoError(t, err)

	// Both should be in level 0 since task-99 is ignored
	assert.Len(t, levels, 1)
	assert.Len(t, levels[0], 2)
}

func TestComputeExecutionLevels_Empty(t *testing.T) {
	levels, err := computeExecutionLevels([]TaskManifestItem{})
	require.NoError(t, err)
	assert.Nil(t, levels)
}

func TestGenerateManifestFromFilesystem_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid task files
	createTaskFileInDir(t, tmpDir, "task-1-setup.md", `# Task: Setup

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: low
**Dependencies**: None

---
`)

	createTaskFileInDir(t, tmpDir, "task-2-implement.md", `# Task: Implement

**Task ID**: task-2
**Assigned Agent**: gemini
**Complexity**: high
**Dependencies**: task-1

---
`)

	manifest, err := generateManifestFromFilesystem(tmpDir)
	require.NoError(t, err)

	assert.Len(t, manifest.Tasks, 2)
	assert.Len(t, manifest.ExecutionLevels, 2)

	// Verify task order and content
	assert.Equal(t, "task-1", manifest.Tasks[0].ID)
	assert.Equal(t, "task-2", manifest.Tasks[1].ID)

	// Verify execution levels
	assert.Equal(t, []string{"task-1"}, manifest.ExecutionLevels[0])
	assert.Equal(t, []string{"task-2"}, manifest.ExecutionLevels[1])
}

func TestGenerateManifestFromFilesystem_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := generateManifestFromFilesystem(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no task files found")
}

func TestGenerateManifestFromFilesystem_PartialSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one valid and one invalid task file
	createTaskFileInDir(t, tmpDir, "task-1-valid.md", `# Task: Valid

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: low
**Dependencies**: None

---
`)

	createTaskFileInDir(t, tmpDir, "task-2-invalid.md", `# Task: Invalid

Missing task ID header here

---
`)

	manifest, err := generateManifestFromFilesystem(tmpDir)
	require.NoError(t, err)

	// Should have only the valid task
	assert.Len(t, manifest.Tasks, 1)
	assert.Equal(t, "task-1", manifest.Tasks[0].ID)
}

func TestGenerateManifestFromFilesystem_AllInvalid(t *testing.T) {
	tmpDir := t.TempDir()

	createTaskFileInDir(t, tmpDir, "task-1-invalid.md", `# Task: Invalid
No task ID
---
`)

	createTaskFileInDir(t, tmpDir, "task-2-invalid.md", `# Task: Also Invalid
Still no task ID
---
`)

	_, err := generateManifestFromFilesystem(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all task files failed to parse")
}

func TestGenerateManifestFromFilesystem_CircularDeps(t *testing.T) {
	tmpDir := t.TempDir()

	createTaskFileInDir(t, tmpDir, "task-1-a.md", `# Task: A

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: low
**Dependencies**: task-2

---
`)

	createTaskFileInDir(t, tmpDir, "task-2-b.md", `# Task: B

**Task ID**: task-2
**Assigned Agent**: claude
**Complexity**: low
**Dependencies**: task-1

---
`)

	_, err := generateManifestFromFilesystem(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// Helper functions

func createTempTaskFile(t *testing.T, name, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, name)
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
	return path
}

func createTaskFileInDir(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
}
