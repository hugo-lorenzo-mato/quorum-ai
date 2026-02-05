package issues

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func TestMatchFile_ConsolidatedVariousFormats(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-tests"},
	}

	matcher := NewFileMatcher(taskFiles)

	tests := []struct {
		filename     string
		wantMatched  bool
		wantConsolid bool
	}{
		{"00-consolidated-analysis.md", true, true},
		{"0-consolidated.md", true, true},
		{"consolidated.md", true, true},
		{"main-issue.md", true, true},
		{"mainissue.md", true, true},
		{"summary.md", true, true},
		{"overview.md", true, true},
		{"00-overview.md", true, true},
		{"000-consolidated-report.md", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := matcher.MatchFile(tc.filename)

			if result.Matched != tc.wantMatched {
				t.Errorf("MatchFile(%q).Matched = %v, want %v",
					tc.filename, result.Matched, tc.wantMatched)
			}
			if result.IsConsolidated != tc.wantConsolid {
				t.Errorf("MatchFile(%q).IsConsolidated = %v, want %v",
					tc.filename, result.IsConsolidated, tc.wantConsolid)
			}
		})
	}
}

func TestMatchFile_TaskExactName(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-unit-tests"},
		{ID: "task-3", Slug: "refactor-api"},
	}

	matcher := NewFileMatcher(taskFiles)

	tests := []struct {
		filename   string
		wantTaskID string
	}{
		{"01-implement-auth.md", "task-1"},
		{"task-1-implement-auth.md", "task-1"},
		{"02-add-unit-tests.md", "task-2"},
		{"03-refactor-api.md", "task-3"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := matcher.MatchFile(tc.filename)

			if !result.Matched {
				t.Errorf("expected file %q to be matched", tc.filename)
				return
			}
			if result.TaskID != tc.wantTaskID {
				t.Errorf("MatchFile(%q).TaskID = %q, want %q",
					tc.filename, result.TaskID, tc.wantTaskID)
			}
			if result.IsConsolidated {
				t.Errorf("expected file %q to not be consolidated", tc.filename)
			}
		})
	}
}

func TestMatchFile_TaskVariations(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-login"},
	}

	matcher := NewFileMatcher(taskFiles)

	tests := []struct {
		filename    string
		wantMatched bool
		wantTaskID  string
		wantConfMin int // minimum confidence
	}{
		// High confidence matches
		{"01-implement-login.md", true, "task-1", 90},
		{"task-1-implement-login.md", true, "task-1", 80}, // Pattern 4 match
		{"issue-1-implement-login.md", true, "task-1", 80},

		// Medium confidence matches
		{"some-prefix-implement-login.md", true, "task-1", 50},
		{"implement-login.md", true, "task-1", 50},

		// Number fallback (low confidence)
		{"01-something-else.md", true, "task-1", 50},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := matcher.MatchFile(tc.filename)

			if result.Matched != tc.wantMatched {
				t.Errorf("MatchFile(%q).Matched = %v, want %v",
					tc.filename, result.Matched, tc.wantMatched)
				return
			}

			if result.Matched {
				if result.TaskID != tc.wantTaskID {
					t.Errorf("MatchFile(%q).TaskID = %q, want %q",
						tc.filename, result.TaskID, tc.wantTaskID)
				}
				if result.Confidence < tc.wantConfMin {
					t.Errorf("MatchFile(%q).Confidence = %d, want >= %d",
						tc.filename, result.Confidence, tc.wantConfMin)
				}
			}
		})
	}
}

func TestMatchFile_NoMatch(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-tests"},
	}

	matcher := NewFileMatcher(taskFiles)

	tests := []string{
		"random-file.md",
		"notes.txt",
		"readme.md",
		"05-unknown-task.md", // task-5 doesn't exist
		"completely-different.md",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			result := matcher.MatchFile(filename)

			if result.Matched {
				t.Errorf("MatchFile(%q) unexpectedly matched with TaskID=%q, Consolidated=%v",
					filename, result.TaskID, result.IsConsolidated)
			}
		})
	}
}

func TestMatchAll(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-tests"},
		{ID: "task-3", Slug: "update-docs"},
	}

	matcher := NewFileMatcher(taskFiles)

	filenames := []string{
		"00-consolidated.md",
		"01-implement-auth.md",
		"02-add-tests.md",
		"03-update-docs.md",
		"random-file.md",
	}

	consolidated, tasks, unmatched := matcher.MatchAll(filenames)

	// Check consolidated
	if consolidated != "00-consolidated.md" {
		t.Errorf("expected consolidated=%q, got %q", "00-consolidated.md", consolidated)
	}

	// Check tasks
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks["task-1"] != "01-implement-auth.md" {
		t.Errorf("expected task-1=%q, got %q", "01-implement-auth.md", tasks["task-1"])
	}
	if tasks["task-2"] != "02-add-tests.md" {
		t.Errorf("expected task-2=%q, got %q", "02-add-tests.md", tasks["task-2"])
	}
	if tasks["task-3"] != "03-update-docs.md" {
		t.Errorf("expected task-3=%q, got %q", "03-update-docs.md", tasks["task-3"])
	}

	// Check unmatched
	if len(unmatched) != 1 || unmatched[0] != "random-file.md" {
		t.Errorf("expected unmatched=[random-file.md], got %v", unmatched)
	}
}

func TestMatchAll_HigherConfidencePrevails(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-login"},
	}

	matcher := NewFileMatcher(taskFiles)

	// Provide multiple files that could match task-1
	filenames := []string{
		"01-implement-login.md", // Higher confidence (slug match)
		"01-something-else.md",  // Lower confidence (number only)
	}

	_, tasks, _ := matcher.MatchAll(filenames)

	// Should keep the higher confidence match
	if tasks["task-1"] != "01-implement-login.md" {
		t.Errorf("expected higher confidence match, got %q", tasks["task-1"])
	}
}

func TestGetMissingTasks(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-tests"},
		{ID: "task-3", Slug: "update-docs"},
	}

	matcher := NewFileMatcher(taskFiles)

	// Only task-1 and task-3 have files
	matchedTasks := map[string]string{
		"task-1": "01-implement-auth.md",
		"task-3": "03-update-docs.md",
	}

	missing := matcher.GetMissingTasks(matchedTasks)

	if len(missing) != 1 {
		t.Fatalf("expected 1 missing task, got %d: %v", len(missing), missing)
	}
	if missing[0] != "task-2" {
		t.Errorf("expected missing task-2, got %s", missing[0])
	}
}

func TestGetMissingTasks_AllMatched(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
		{ID: "task-2", Slug: "add-tests"},
	}

	matcher := NewFileMatcher(taskFiles)

	matchedTasks := map[string]string{
		"task-1": "01-implement-auth.md",
		"task-2": "02-add-tests.md",
	}

	missing := matcher.GetMissingTasks(matchedTasks)

	if len(missing) != 0 {
		t.Errorf("expected no missing tasks, got %v", missing)
	}
}

func TestExtractTaskNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"01-implement-feature.md", 1},
		{"02-add-tests.md", 2},
		{"10-big-task.md", 10},
		{"task-3-something.md", 3},
		{"issue-5.md", 5},
		{"feature-7.md", 7},
		{"3.md", 3},
		{"00-consolidated.md", 0}, // 0 is skipped
		{"readme.md", 0},          // No number
		{"file.txt", 0},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := extractTaskNumber(tc.filename)
			if result != tc.expected {
				t.Errorf("extractTaskNumber(%q) = %d, want %d",
					tc.filename, result, tc.expected)
			}
		})
	}
}

func TestNormalizeTaskSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Implement User Auth", "implement-user-auth"},
		{"add_tests_now", "add-tests-now"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars", "specialchars"},
		{"multiple---dashes", "multiple-dashes"},
		{"-leading-dash", "leading-dash"},
		{"trailing-dash-", "trailing-dash"},
		{"mix Of_Everything!Here", "mix-of-everythinghere"}, // ! is removed, not replaced
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeTaskSlug(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeTaskSlug(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestMatchResult_Fields(t *testing.T) {
	// Test MatchResult struct fields
	result := MatchResult{
		Matched:        true,
		IsConsolidated: false,
		TaskID:         "task-1",
		Confidence:     90,
	}

	if !result.Matched {
		t.Error("expected Matched to be true")
	}
	if result.IsConsolidated {
		t.Error("expected IsConsolidated to be false")
	}
	if result.TaskID != "task-1" {
		t.Errorf("expected TaskID='task-1', got '%s'", result.TaskID)
	}
	if result.Confidence != 90 {
		t.Errorf("expected Confidence=90, got %d", result.Confidence)
	}
}

func TestFileMatcher_CaseInsensitive(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "implement-auth"},
	}

	matcher := NewFileMatcher(taskFiles)

	tests := []struct {
		filename    string
		wantMatched bool
	}{
		{"01-implement-auth.md", true},
		{"01-IMPLEMENT-AUTH.md", true},
		{"01-Implement-Auth.md", true},
		{"CONSOLIDATED.md", true},
		{"Consolidated.md", true},
		{"MAIN-ISSUE.md", true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := matcher.MatchFile(tc.filename)
			if result.Matched != tc.wantMatched {
				t.Errorf("MatchFile(%q).Matched = %v, want %v",
					tc.filename, result.Matched, tc.wantMatched)
			}
		})
	}
}

func TestNewFileMatcher_EmptyTasks(t *testing.T) {
	matcher := NewFileMatcher(nil)

	// Should still match consolidated patterns
	result := matcher.MatchFile("00-consolidated.md")
	if !result.Matched || !result.IsConsolidated {
		t.Error("expected consolidated file to still match with empty tasks")
	}

	// Task files should not match
	result = matcher.MatchFile("01-some-task.md")
	if result.Matched {
		t.Error("expected no match for task file with empty tasks")
	}
}

func TestMatchAll_MultipleConsolidated(t *testing.T) {
	taskFiles := []service.IssueTaskFile{
		{ID: "task-1", Slug: "feature"},
	}

	matcher := NewFileMatcher(taskFiles)

	// Multiple files that could be consolidated
	filenames := []string{
		"summary.md",
		"00-consolidated.md", // Higher confidence
	}

	consolidated, _, _ := matcher.MatchAll(filenames)

	// Should prefer the higher confidence match
	if consolidated != "00-consolidated.md" && consolidated != "summary.md" {
		t.Errorf("expected a consolidated file, got %q", consolidated)
	}
}
