package issues

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// =============================================================================
// Generator: resolveDraftDir / resolvePublishedDir with invalid workflow IDs
// =============================================================================

func TestResolveDraftDir_InvalidWorkflowID(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)
	_, err := g.resolveDraftDir("")
	if err == nil {
		t.Error("expected error for empty workflow ID")
	}

	_, err = g.resolveDraftDir("../escape")
	if err == nil {
		t.Error("expected error for path traversal workflow ID")
	}
}

func TestResolvePublishedDir_InvalidWorkflowID(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)
	_, err := g.resolvePublishedDir("")
	if err == nil {
		t.Error("expected error for empty workflow ID")
	}

	_, err = g.resolvePublishedDir("wf/../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal workflow ID")
	}
}

func TestResolveDraftDir_CustomDraftDirectory(t *testing.T) {
	root := t.TempDir()
	cfg := config.IssuesConfig{
		DraftDirectory: "custom/issues",
	}
	g := NewGenerator(nil, cfg, root, "", nil)

	dir, err := g.resolveDraftDir("wf-abc")
	if err != nil {
		t.Fatalf("resolveDraftDir() error = %v", err)
	}

	// Normalize path for cross-platform comparison
	normalizedDir := filepath.ToSlash(dir)
	if !strings.Contains(normalizedDir, "custom/issues") {
		t.Errorf("expected custom directory in path, got %q", dir)
	}
	if !strings.HasSuffix(dir, filepath.Join("wf-abc", "draft")) {
		t.Errorf("expected wf-abc/draft suffix, got %q", dir)
	}
}

func TestResolvePublishedDir_CustomDraftDirectory(t *testing.T) {
	root := t.TempDir()
	cfg := config.IssuesConfig{
		DraftDirectory: "custom/issues",
	}
	g := NewGenerator(nil, cfg, root, "", nil)

	dir, err := g.resolvePublishedDir("wf-abc")
	if err != nil {
		t.Fatalf("resolvePublishedDir() error = %v", err)
	}

	// Normalize path for cross-platform comparison
	normalizedDir := filepath.ToSlash(dir)
	if !strings.Contains(normalizedDir, "custom/issues") {
		t.Errorf("expected custom directory in path, got %q", dir)
	}
	if !strings.HasSuffix(dir, filepath.Join("wf-abc", "published")) {
		t.Errorf("expected wf-abc/published suffix, got %q", dir)
	}
}

// =============================================================================
// Generator: getPromptRenderer (lazy initialization)
// =============================================================================

func TestGetPromptRenderer_LazyInit(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	// First call should lazily initialize
	r1, err := g.getPromptRenderer()
	if err != nil {
		t.Fatalf("getPromptRenderer() error = %v", err)
	}
	if r1 == nil {
		t.Fatal("expected non-nil renderer")
	}

	// Second call should return the same instance
	r2, err := g.getPromptRenderer()
	if err != nil {
		t.Fatalf("getPromptRenderer() second call error = %v", err)
	}
	if r1 != r2 {
		t.Error("expected same renderer instance on subsequent calls")
	}
}

// =============================================================================
// Generator: buildMasterPrompt coverage
// =============================================================================

func TestBuildMasterPrompt_BasicContent(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{
			Language: "english",
			Tone:     "professional",
		},
	}, t.TempDir(), "", nil)

	tasks := []TaskInfo{
		{ID: "task-1", Name: "Setup project", Content: "Task 1 content"},
		{ID: "task-2", Name: "Add tests", Content: "Task 2 content"},
	}

	result := g.buildMasterPrompt("Consolidated analysis content", tasks, "", "/output/issues")

	if !strings.Contains(result, "Generate GitHub Issue Markdown Files") {
		t.Error("should contain task description")
	}
	if !strings.Contains(result, "/output/issues") {
		t.Error("should contain output directory")
	}
	if !strings.Contains(result, "01-setup-project.md") {
		t.Error("should contain expected filename for task-1")
	}
	if !strings.Contains(result, "02-add-tests.md") {
		t.Error("should contain expected filename for task-2")
	}
	if !strings.Contains(result, "English") || !strings.Contains(result, "professional") {
		t.Error("should contain language/tone instructions")
	}
}

func TestBuildMasterPrompt_SpanishLanguage(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Language: "spanish"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "Spanish") {
		t.Error("should contain Spanish language instruction")
	}
}

func TestBuildMasterPrompt_FrenchLanguage(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Language: "french"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "French") {
		t.Error("should contain French language instruction")
	}
}

func TestBuildMasterPrompt_GermanLanguage(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Language: "german"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "German") {
		t.Error("should contain German language instruction")
	}
}

func TestBuildMasterPrompt_PortugueseLanguage(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Language: "portuguese"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "Portuguese") {
		t.Error("should contain Portuguese language instruction")
	}
}

func TestBuildMasterPrompt_TechnicalTone(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Tone: "technical"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "technical") {
		t.Error("should contain technical tone instruction")
	}
}

func TestBuildMasterPrompt_ConciseTone(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Tone: "concise"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "concise") {
		t.Error("should contain concise tone instruction")
	}
}

func TestBuildMasterPrompt_CasualTone(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Tone: "casual"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "casual") {
		t.Error("should contain casual tone instruction")
	}
}

func TestBuildMasterPrompt_WithDiagrams(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{IncludeDiagrams: true},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "Mermaid") {
		t.Error("should contain Mermaid diagram instruction")
	}
}

func TestBuildMasterPrompt_WithSummarize(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Generator: config.IssueGeneratorConfig{Summarize: true},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "Summarize") {
		t.Error("should contain summarize instruction")
	}
}

func TestBuildMasterPrompt_CustomInstructions(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{CustomInstructions: "Focus on security"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "Focus on security") {
		t.Error("should contain custom instructions")
	}
}

func TestBuildMasterPrompt_WithConvention(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{Convention: "angular commit style"},
	}, t.TempDir(), "", nil)

	result := g.buildMasterPrompt("Analysis", nil, "", "/output")
	if !strings.Contains(result, "angular commit style") {
		t.Error("should contain convention")
	}
}

func TestBuildMasterPrompt_LongConsolidated(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	longContent := strings.Repeat("x", 35000)
	result := g.buildMasterPrompt(longContent, nil, "", "/output")
	if !strings.Contains(result, "[Truncated...]") {
		t.Error("long consolidated content should be truncated")
	}
}

func TestBuildMasterPrompt_LongTaskContent(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	tasks := []TaskInfo{
		{ID: "task-1", Name: "big task", Content: strings.Repeat("y", 20000)},
	}
	result := g.buildMasterPrompt("Analysis", tasks, "", "/output")
	if !strings.Contains(result, "[Truncated...]") {
		t.Error("long task content should be truncated")
	}
}

// =============================================================================
// Generator: parseAndWriteIssueFiles / parseCodeBlockFiles
// =============================================================================

func TestParseAndWriteIssueFiles_FileMarkers(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	output := "<!-- FILE: 00-consolidated.md -->\n# Main Issue\n\nBody content\n\n<!-- FILE: 01-task-setup.md -->\n# Setup Task\n\nSetup content\n"

	files, err := g.parseAndWriteIssueFiles(output, issuesDir)
	if err != nil {
		t.Fatalf("parseAndWriteIssueFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Verify files were written
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading file %s: %v", f, err)
		}
		if len(content) == 0 {
			t.Errorf("file %s is empty", f)
		}
	}
}

func TestParseAndWriteIssueFiles_NoMarkers_FallsBackToCodeBlocks(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	output := "### 01-setup.md\n\n```markdown\n# Setup Issue\n\nBody here\n```\n"

	files, err := g.parseAndWriteIssueFiles(output, issuesDir)
	if err != nil {
		t.Fatalf("parseAndWriteIssueFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestParseCodeBlockFiles_WithCodeBlocks(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	output := "### 01-first.md\n\n```markdown\n# First Issue\n\nFirst body\n```\n\n### 02-second.md\n\n```\n# Second Issue\n\nSecond body\n```\n"

	files, err := g.parseCodeBlockFiles(output, issuesDir)
	if err != nil {
		t.Fatalf("parseCodeBlockFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestParseCodeBlockFiles_WithoutCodeBlocks(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	output := "### 01-first.md\n\n# First Issue\n\nDirect content without code block\n"

	files, err := g.parseCodeBlockFiles(output, issuesDir)
	if err != nil {
		t.Fatalf("parseCodeBlockFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

// =============================================================================
// Generator: scanGeneratedIssueFilesWithTracker
// =============================================================================

func TestScanGeneratedIssueFilesWithTracker(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")
	tracker.AddExpected("02-task.md", "task-2")
	// Ensure files written in the test are considered "after generation started".
	tracker.StartTime = time.Now().Add(-time.Second)

	// Create some files
	covWriteFile(t, filepath.Join(issuesDir, "01-task.md"), "# Issue 1\n\nBody")
	covWriteFile(t, filepath.Join(issuesDir, "02-task.md"), "# Issue 2\n\nBody")
	covWriteFile(t, filepath.Join(issuesDir, "empty.md"), "") // Empty file should be skipped
	covWriteFile(t, filepath.Join(issuesDir, "not-md.txt"), "not markdown")

	files, err := g.scanGeneratedIssueFilesWithTracker(issuesDir, tracker)
	if err != nil {
		t.Fatalf("scanGeneratedIssueFilesWithTracker() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	missing := tracker.GetMissingFiles()
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %v", missing)
	}
}

func TestScanGeneratedIssueFilesWithTracker_NonExistentDir(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	files, err := g.scanGeneratedIssueFilesWithTracker("/nonexistent/path", nil)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if files != nil {
		t.Error("expected nil files for nonexistent dir")
	}
}

func TestScanGeneratedIssueFilesWithTracker_NoTracker(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	issuesDir := filepath.Join(root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	covWriteFile(t, filepath.Join(issuesDir, "01-task.md"), "# Issue 1\n\nBody")

	files, err := g.scanGeneratedIssueFilesWithTracker(issuesDir, nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

// =============================================================================
// Generator: formatTaskIssueBody with truncation
// =============================================================================

func TestFormatTaskIssueBody_LongContentTruncated(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	task := TaskInfo{
		ID:           "task-1",
		Name:         "Big Task",
		Agent:        "claude",
		Complexity:   "high",
		Dependencies: []string{"task-0"},
		Content:      "---\n**Task ID**: task-1\n---\n" + strings.Repeat("x", 45000),
	}

	body := g.formatTaskIssueBody(task)

	if !strings.Contains(body, "[Content truncated...]") {
		t.Error("long content should be truncated")
	}
	if !strings.Contains(body, "claude") {
		t.Error("should contain agent name")
	}
	if !strings.Contains(body, "high") {
		t.Error("should contain complexity")
	}
	if !strings.Contains(body, "task-0") {
		t.Error("should contain dependencies")
	}
}

// =============================================================================
// Generator: formatMainIssueBody with truncation
// =============================================================================

func TestFormatMainIssueBody_LongContentTruncated(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	longContent := strings.Repeat("y", 55000)
	body := g.formatMainIssueBody(longContent, "wf-123")

	if !strings.Contains(body, "[Content truncated...]") {
		t.Error("long content should be truncated")
	}
	if !strings.Contains(body, "wf-123") {
		t.Error("should contain workflow ID")
	}
}

// =============================================================================
// Generator: extractTaskContent edge cases
// =============================================================================

func TestExtractTaskContent_NoFrontmatter(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	content := "# Task Title\n\nSome metadata\n\n## Implementation\n\nActual content here"
	result := g.extractTaskContent(content)

	if !strings.Contains(result, "## Implementation") {
		t.Error("should extract from first ## heading when no frontmatter")
	}
}

func TestExtractTaskContent_WithFrontmatter(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	content := "---\nid: task-1\n---\n## Implementation\n\nContent after frontmatter"
	result := g.extractTaskContent(content)

	if !strings.Contains(result, "## Implementation") {
		t.Error("should extract content after frontmatter closing delimiter")
	}
}

// =============================================================================
// Generator: emitIssuesGenerationProgress / emitIssuesPublishingProgress
// =============================================================================

type recordingProgressReporter struct {
	genCalls []string
	pubCalls []string
}

func (r *recordingProgressReporter) OnIssuesGenerationProgress(_, stage string, _, _ int, _ *ProgressIssue, _ string) {
	r.genCalls = append(r.genCalls, stage)
}

func (r *recordingProgressReporter) OnIssuesPublishingProgress(p PublishingProgressParams) {
	r.pubCalls = append(r.pubCalls, p.Stage)
}

func TestEmitProgressEvents(t *testing.T) {
	reporter := &recordingProgressReporter{}
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)
	g.SetProgressReporter(reporter)

	g.emitIssuesGenerationProgress("wf-1", "started", 0, 5, nil, "starting")
	g.emitIssuesPublishingProgress(PublishingProgressParams{
		WorkflowID: "wf-1", Stage: "started", Total: 3,
	})

	if len(reporter.genCalls) != 1 || reporter.genCalls[0] != "started" {
		t.Errorf("expected gen calls ['started'], got %v", reporter.genCalls)
	}
	if len(reporter.pubCalls) != 1 || reporter.pubCalls[0] != "started" {
		t.Errorf("expected pub calls ['started'], got %v", reporter.pubCalls)
	}
}

func TestEmitProgressEvents_NilReporter(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	// Should not panic with nil reporter
	g.emitIssuesGenerationProgress("wf-1", "started", 0, 5, nil, "")
	g.emitIssuesPublishingProgress(PublishingProgressParams{
		WorkflowID: "wf-1", Stage: "started", Total: 3,
	})
}

// =============================================================================
// Generator: splitIntoBatches
// =============================================================================

func TestSplitIntoBatches_ZeroBatchSize(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	tasks := []service.IssueTaskFile{
		{ID: "task-1"}, {ID: "task-2"}, {ID: "task-3"},
	}

	batches := g.splitIntoBatches(tasks, 0)
	// Should use default batch size
	if len(batches) != 1 {
		t.Errorf("expected 1 batch with default size, got %d", len(batches))
	}
}

func TestSplitIntoBatches_EmptyTasks(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	batches := g.splitIntoBatches(nil, 5)
	if len(batches) != 1 {
		t.Errorf("expected 1 empty batch, got %d", len(batches))
	}
	if len(batches[0]) != 0 {
		t.Error("empty batch should have 0 tasks")
	}
}

func TestSplitIntoBatches_MultipleBatches(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	tasks := make([]service.IssueTaskFile, 10)
	for i := range tasks {
		tasks[i] = service.IssueTaskFile{ID: "task-" + string(rune('0'+i))}
	}

	batches := g.splitIntoBatches(tasks, 3)
	if len(batches) != 4 { // 3+3+3+1
		t.Errorf("expected 4 batches, got %d", len(batches))
	}
	if len(batches[3]) != 1 {
		t.Errorf("last batch should have 1 task, got %d", len(batches[3]))
	}
}

// =============================================================================
// Generator: fuzzyMatchFilename
// =============================================================================

func TestFuzzyMatchFilename_Coverage(t *testing.T) {
	tests := []struct {
		actual   string
		expected string
		match    bool
	}{
		{"01-setup.md", "01-setup.md", true},
		{"1-setup.md", "01-setup.md", true},
		{"01-setup.md", "1-setup.md", true},
		{"01-setup-project.md", "01-setup.md", true},
		{"totally-different.md", "01-setup.md", false},
	}

	for _, tt := range tests {
		got := fuzzyMatchFilename(tt.actual, tt.expected)
		if got != tt.match {
			t.Errorf("fuzzyMatchFilename(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.match)
		}
	}
}

// =============================================================================
// Generator: GenerationTracker
// =============================================================================

func TestGenerationTracker_IsValidFile_Coverage(t *testing.T) {
	tracker := NewGenerationTracker("wf-test")
	tracker.AddExpected("01-task.md", "task-1")

	// File before start time
	if tracker.IsValidFile("01-task.md", tracker.StartTime.Add(-time.Second)) {
		t.Error("file before start time should be invalid")
	}

	// File after start time matching expected
	if !tracker.IsValidFile("01-task.md", tracker.StartTime.Add(time.Second)) {
		t.Error("expected file after start time should be valid")
	}

	// File after start time not matching expected
	if tracker.IsValidFile("unexpected.md", tracker.StartTime.Add(time.Second)) {
		t.Error("unexpected file should be invalid when expected files are defined")
	}
}

func TestGenerationTracker_NoExpectedFiles(t *testing.T) {
	tracker := NewGenerationTracker("wf-test")
	// No expected files added

	// Any file after start time should be valid
	if !tracker.IsValidFile("anything.md", tracker.StartTime.Add(time.Second)) {
		t.Error("any file should be valid when no expected files defined")
	}
}

// =============================================================================
// Generator: cleanIssuesDirectory
// =============================================================================

func TestCleanIssuesDirectory_NonExistentDir(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	err := g.cleanIssuesDirectory("wf-nonexistent")
	if err != nil {
		t.Errorf("expected no error for nonexistent dir, got %v", err)
	}
}

func TestCleanIssuesDirectory_ExistingDir(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	draftDir := filepath.Join(root, ".quorum", "issues", "wf-test", "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	covWriteFile(t, filepath.Join(draftDir, "test.md"), "content")

	err := g.cleanIssuesDirectory("wf-test")
	if err != nil {
		t.Fatalf("cleanIssuesDirectory() error = %v", err)
	}

	if _, err := os.Stat(draftDir); !os.IsNotExist(err) {
		t.Error("draft dir should have been removed")
	}
}

func TestCleanIssuesDirectory_InvalidWorkflowID(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	err := g.cleanIssuesDirectory("")
	if err == nil {
		t.Error("expected error for empty workflow ID")
	}
}

// =============================================================================
// Generator: GenerateIssueFiles error paths
// =============================================================================

func TestGenerateIssueFiles_NoAgentRegistry(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	_, err := g.GenerateIssueFiles(context.Background(), "wf-test")
	if err == nil {
		t.Error("expected error when agent registry is nil")
	}
	if !strings.Contains(err.Error(), "agent registry") {
		t.Errorf("error should mention agent registry, got: %v", err)
	}
}

// =============================================================================
// Generator: findIssueInCache
// =============================================================================

func TestFindIssueInCache_MainIssue(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cache := []IssuePreview{
		{Title: "Main", Body: "Main body", IsMainIssue: true},
		{Title: "Task 1", Body: "Task body", TaskID: "task-1"},
	}

	title, body, err := g.findIssueInCache(cache, "", true)
	if err != nil {
		t.Fatalf("findIssueInCache() error = %v", err)
	}
	if title != "Main" {
		t.Errorf("expected title 'Main', got %q", title)
	}
	if body != "Main body" {
		t.Errorf("expected body 'Main body', got %q", body)
	}
}

func TestFindIssueInCache_TaskIssue(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cache := []IssuePreview{
		{Title: "Main", Body: "Main body", IsMainIssue: true},
		{Title: "Task 1", Body: "Task body", TaskID: "task-1"},
	}

	title, _, err := g.findIssueInCache(cache, "task-1", false)
	if err != nil {
		t.Fatalf("findIssueInCache() error = %v", err)
	}
	if title != "Task 1" {
		t.Errorf("expected title 'Task 1', got %q", title)
	}
}

func TestFindIssueInCache_PartialTaskMatch(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cache := []IssuePreview{
		{Title: "Task 1", Body: "Body", TaskID: "task-1-setup-project"},
	}

	title, _, err := g.findIssueInCache(cache, "task-1", false)
	if err != nil {
		t.Fatalf("findIssueInCache() error = %v", err)
	}
	if title != "Task 1" {
		t.Errorf("expected partial match, got %q", title)
	}
}

func TestFindIssueInCache_MainNotFound(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cache := []IssuePreview{
		{Title: "Task 1", Body: "Body", TaskID: "task-1"},
	}

	_, _, err := g.findIssueInCache(cache, "", true)
	if err == nil {
		t.Error("expected error when main issue not found")
	}
}

func TestFindIssueInCache_TaskNotFound(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cache := []IssuePreview{
		{Title: "Main", Body: "Body", IsMainIssue: true},
	}

	_, _, err := g.findIssueInCache(cache, "task-99", false)
	if err == nil {
		t.Error("expected error when task not found")
	}
}

// =============================================================================
// Generator: ReadGeneratedIssues with draft files
// =============================================================================

func TestReadGeneratedIssues_DraftDir(t *testing.T) {
	root := t.TempDir()
	g := NewGenerator(nil, config.IssuesConfig{}, root, "", nil)

	draftDir := filepath.Join(root, ".quorum", "issues", "wf-test", "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a draft with frontmatter
	draftContent := "---\ntitle: Test Issue\ntask_id: task-1\nis_main_issue: false\nlabels:\n  - bug\nassignees:\n  - user1\n---\n\n# Test Issue\n\nBody content"
	covWriteFile(t, filepath.Join(draftDir, "01-test.md"), draftContent)

	issues, err := g.ReadGeneratedIssues("wf-test")
	if err != nil {
		t.Fatalf("ReadGeneratedIssues() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Title != "Test Issue" {
		t.Errorf("expected title 'Test Issue', got %q", issues[0].Title)
	}
}

func TestReadGeneratedIssues_InvalidWorkflowID(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	_, err := g.ReadGeneratedIssues("")
	if err == nil {
		t.Error("expected error for empty workflow ID")
	}
}

// =============================================================================
// Generator: resolveIssuesBaseDir
// =============================================================================

func TestResolveIssuesBaseDir_Default(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)
	result := g.resolveIssuesBaseDir()
	if result != filepath.Join(".quorum", "issues") {
		t.Errorf("expected default base dir, got %q", result)
	}
}

func TestResolveIssuesBaseDir_Custom(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{DraftDirectory: "custom/dir"}, t.TempDir(), "", nil)
	result := g.resolveIssuesBaseDir()
	if result != "custom/dir" {
		t.Errorf("expected custom dir, got %q", result)
	}
}

// =============================================================================
// buildLLMResilienceConfig coverage
// =============================================================================

func TestBuildLLMResilienceConfig_AllFields(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cfg := config.LLMResilienceConfig{
		Enabled:           true,
		MaxRetries:        5,
		InitialBackoff:    "2s",
		MaxBackoff:        "1m",
		BackoffMultiplier: 3.0,
		FailureThreshold:  10,
		ResetTimeout:      "10m",
	}

	result := g.buildLLMResilienceConfig(cfg, nil)

	if !result.Enabled {
		t.Error("expected enabled")
	}
	if result.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", result.MaxRetries)
	}
	if result.InitialBackoff != 2*time.Second {
		t.Errorf("InitialBackoff = %v, want 2s", result.InitialBackoff)
	}
	if result.MaxBackoff != time.Minute {
		t.Errorf("MaxBackoff = %v, want 1m", result.MaxBackoff)
	}
	if result.BackoffMultiplier != 3.0 {
		t.Errorf("BackoffMultiplier = %v, want 3.0", result.BackoffMultiplier)
	}
	if result.FailureThreshold != 10 {
		t.Errorf("FailureThreshold = %d, want 10", result.FailureThreshold)
	}
	if result.ResetTimeout != 10*time.Minute {
		t.Errorf("ResetTimeout = %v, want 10m", result.ResetTimeout)
	}
}

func TestBuildLLMResilienceConfig_ZeroConfig(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	cfg := config.LLMResilienceConfig{} // All zero values
	result := g.buildLLMResilienceConfig(cfg, nil)

	// Should return defaults
	defaults := DefaultLLMResilienceConfig()
	if result.MaxRetries != defaults.MaxRetries {
		t.Errorf("expected default MaxRetries, got %d", result.MaxRetries)
	}
}

// =============================================================================
// Generator: parseTaskFile edge cases
// =============================================================================

func TestParseTaskFile_WithHeading(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	content := "# Task: Custom Task Name\n\n**Task ID**: task-5\n**Assigned Agent**: gemini\n**Complexity**: medium\n**Dependencies**: task-1, task-2\n---\n\n## Implementation\n\nSteps here"

	task := g.parseTaskFile("5", "custom-task-name", content)

	if task.ID != "task-5" {
		t.Errorf("expected task-5, got %q", task.ID)
	}
	if task.Name != "Custom Task Name" {
		t.Errorf("expected 'Custom Task Name', got %q", task.Name)
	}
	if task.Agent != "gemini" {
		t.Errorf("expected gemini, got %q", task.Agent)
	}
	if task.Complexity != "medium" {
		t.Errorf("expected medium, got %q", task.Complexity)
	}
	if len(task.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(task.Dependencies))
	}
}

func TestParseTaskFile_NoDependencies(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	content := "# Initial Setup\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Dependencies**: None\n---\n"

	task := g.parseTaskFile("1", "initial-setup", content)

	if len(task.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies for 'None', got %d", len(task.Dependencies))
	}
}

// =============================================================================
// Generator: formatTitle
// =============================================================================

func TestFormatTitle_DefaultFormat(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, t.TempDir(), "", nil)

	mainTitle := g.formatTitle("", "", true)
	if mainTitle != "[quorum] Workflow Summary" {
		t.Errorf("expected '[quorum] Workflow Summary', got %q", mainTitle)
	}

	taskTitle := g.formatTitle("task-1", "setup project", false)
	if taskTitle != "[quorum] setup project" {
		t.Errorf("expected '[quorum] setup project', got %q", taskTitle)
	}
}

func TestFormatTitle_CustomFormat(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{
		Prompt: config.IssuePromptConfig{
			TitleFormat: "[{task_id}] {task_name}",
		},
	}, t.TempDir(), "", nil)

	title := g.formatTitle("task-1", "implement auth", false)
	if title != "[task-1] implement auth" {
		t.Errorf("expected '[task-1] implement auth', got %q", title)
	}
}

// =============================================================================
// Generator: getProjectRoot
// =============================================================================

func TestGetProjectRoot_ExplicitRoot(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, "/explicit/root", "", nil)

	root, err := g.getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot() error = %v", err)
	}
	if root != "/explicit/root" {
		t.Errorf("expected '/explicit/root', got %q", root)
	}
}

func TestGetProjectRoot_FallbackToGetwd(t *testing.T) {
	g := NewGenerator(nil, config.IssuesConfig{}, "", "", nil)

	root, err := g.getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot() error = %v", err)
	}
	if root == "" {
		t.Error("fallback root should not be empty")
	}
}

// =============================================================================
// CreateIssuesFromInput coverage: sub-issues with errors
// =============================================================================

func TestCreateIssuesFromInput_SubIssueCreateError(t *testing.T) {
	client := &mockIssueClient{
		createErr: errors.New("create failed"),
	}
	g := NewGenerator(client, config.IssuesConfig{}, t.TempDir(), "", nil)

	inputs := []IssueInput{
		{Title: "Sub 1", Body: "Body", TaskID: "task-1"},
	}

	result, err := g.CreateIssuesFromInput(context.Background(), inputs, false, false, nil, nil)
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected non-fatal error for sub-issue creation failure")
	}
}

func TestCreateIssuesFromInput_WithDefaultLabelsAndAssignees(t *testing.T) {
	client := &mockIssueClient{}
	g := NewGenerator(client, config.IssuesConfig{}, t.TempDir(), "", nil)

	inputs := []IssueInput{
		{Title: "Main Issue", Body: "Body", IsMainIssue: true},
		{Title: "Sub 1", Body: "Body", TaskID: "task-1"},
	}

	result, err := g.CreateIssuesFromInput(context.Background(), inputs, true, true,
		[]string{"default-label"}, []string{"default-assignee"})
	if err != nil {
		t.Fatalf("CreateIssuesFromInput() error = %v", err)
	}
	if len(result.PreviewIssues) != 2 {
		t.Errorf("expected 2 preview issues, got %d", len(result.PreviewIssues))
	}

	// Check that defaults were applied
	for _, issue := range result.PreviewIssues {
		if len(issue.Labels) == 0 {
			t.Error("expected labels to be set")
		}
		if len(issue.Assignees) == 0 {
			t.Error("expected assignees to be set")
		}
	}
}

// =============================================================================
// parseIssueMarkdown
// =============================================================================

func TestParseIssueMarkdown_NoH1(t *testing.T) {
	title, body := parseIssueMarkdown("Just some text\nwithout heading")
	if title != "Untitled Issue" {
		t.Errorf("expected 'Untitled Issue', got %q", title)
	}
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestParseIssueMarkdown_WithH1(t *testing.T) {
	title, body := parseIssueMarkdown("# My Issue\n\nSome body content")
	if title != "My Issue" {
		t.Errorf("expected 'My Issue', got %q", title)
	}
	if !strings.Contains(body, "Some body content") {
		t.Errorf("body should contain content, got %q", body)
	}
}

// =============================================================================
// sanitizeFilename coverage (unique test name to avoid conflict with generator_test.go)
// =============================================================================

func TestSanitizeFilename_Coverage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"task@#$%name", "taskname"},
		{"a--b--c", "a-b-c"},
		{"", "issue"},
		{strings.Repeat("a", 60), strings.Repeat("a", 50)},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// =============================================================================
// extractFileNumber coverage (unique test name to avoid conflict with files_test.go)
// =============================================================================

func TestExtractFileNumber_Coverage(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"01-task.md", 1},
		{"10-setup.md", 10},
		{"no-number.md", 9999},
	}

	for _, tt := range tests {
		got := extractFileNumber(tt.name)
		if got != tt.expected {
			t.Errorf("extractFileNumber(%q) = %d, want %d", tt.name, got, tt.expected)
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

// covWriteFile is a test helper that writes content to a file.
// Named to avoid conflict with helpers in other test files.
func covWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}
