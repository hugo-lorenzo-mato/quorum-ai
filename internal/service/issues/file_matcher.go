package issues

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// FileMatcher provides flexible matching of generated issue files to expected tasks.
// This handles variations in how the LLM might name files while still correctly
// identifying which task each file corresponds to.
type FileMatcher struct {
	taskFiles            []service.IssueTaskFile
	consolidatedPatterns []*regexp.Regexp
	taskPatterns         map[string][]*regexp.Regexp // taskID -> patterns
}

// NewFileMatcher creates a new file matcher for the given task files.
func NewFileMatcher(taskFiles []service.IssueTaskFile) *FileMatcher {
	m := &FileMatcher{
		taskFiles:    taskFiles,
		taskPatterns: make(map[string][]*regexp.Regexp),
	}

	// Compile consolidated patterns
	consolidatedStrs := []string{
		`(?i)^0*-?consolidated`,
		`(?i)^main-?issue`,
		`(?i)^summary`,
		`(?i)^overview`,
		`(?i)^00-`,
	}
	for _, p := range consolidatedStrs {
		if re, err := regexp.Compile(p); err == nil {
			m.consolidatedPatterns = append(m.consolidatedPatterns, re)
		}
	}

	// Compile task-specific patterns
	for _, task := range taskFiles {
		var patterns []*regexp.Regexp

		// Pattern 1: NN-slug.md (e.g., 01-implement-login.md)
		if re, err := regexp.Compile(`(?i)^\d+-.*` + regexp.QuoteMeta(task.Slug)); err == nil {
			patterns = append(patterns, re)
		}

		// Pattern 2: issue-NN-slug.md
		if re, err := regexp.Compile(`(?i)^issue-\d+-.*` + regexp.QuoteMeta(task.Slug)); err == nil {
			patterns = append(patterns, re)
		}

		// Pattern 3: task-N-slug.md
		if re, err := regexp.Compile(`(?i)^task-\d+-.*` + regexp.QuoteMeta(task.Slug)); err == nil {
			patterns = append(patterns, re)
		}

		// Pattern 4: Exact task ID (e.g., task-1-*.md)
		if re, err := regexp.Compile(`(?i)^` + regexp.QuoteMeta(task.ID) + `-`); err == nil {
			patterns = append(patterns, re)
		}

		// Pattern 5: Slug at end (e.g., *-implement-login.md)
		if re, err := regexp.Compile(`(?i)-` + regexp.QuoteMeta(task.Slug) + `\.md$`); err == nil {
			patterns = append(patterns, re)
		}

		// Pattern 6: Just the slug (e.g., implement-login.md)
		if re, err := regexp.Compile(`(?i)^` + regexp.QuoteMeta(task.Slug) + `\.md$`); err == nil {
			patterns = append(patterns, re)
		}

		m.taskPatterns[task.ID] = patterns
	}

	return m
}

// MatchResult contains the result of matching a file.
type MatchResult struct {
	// Matched indicates if the file was successfully matched.
	Matched bool

	// IsConsolidated indicates if the file is the consolidated/main issue.
	IsConsolidated bool

	// TaskID is the matched task ID (empty if IsConsolidated or not matched).
	TaskID string

	// Confidence indicates match confidence (higher is better).
	// 100 = exact match, 50-99 = partial match, 0 = no match
	Confidence int
}

// MatchFile attempts to match a filename to a task or consolidated issue.
func (m *FileMatcher) MatchFile(filename string) MatchResult {
	// Normalize filename
	filename = strings.ToLower(filename)

	// Check for consolidated patterns first
	for _, re := range m.consolidatedPatterns {
		if re.MatchString(filename) {
			return MatchResult{
				Matched:        true,
				IsConsolidated: true,
				Confidence:     100,
			}
		}
	}

	// Check task patterns
	var bestMatch MatchResult
	for taskID, patterns := range m.taskPatterns {
		for i, re := range patterns {
			if re.MatchString(filename) {
				// Earlier patterns have higher confidence
				confidence := 100 - (i * 10)
				if confidence > bestMatch.Confidence {
					bestMatch = MatchResult{
						Matched:    true,
						TaskID:     taskID,
						Confidence: confidence,
					}
				}
			}
		}
	}

	if bestMatch.Matched {
		return bestMatch
	}

	// Fallback: try to extract task number from filename
	taskNum := extractTaskNumber(filename)
	if taskNum > 0 {
		taskID := "task-" + strconv.Itoa(taskNum)
		// Verify this task exists
		for _, task := range m.taskFiles {
			if task.ID == taskID {
				return MatchResult{
					Matched:    true,
					TaskID:     taskID,
					Confidence: 50, // Low confidence for number-only match
				}
			}
		}
	}

	return MatchResult{Matched: false}
}

// MatchAll matches all provided filenames and returns a map of taskID -> filename.
// Returns separate results for consolidated and task files.
func (m *FileMatcher) MatchAll(filenames []string) (consolidated string, tasks map[string]string, unmatched []string) {
	tasks = make(map[string]string)

	for _, filename := range filenames {
		result := m.MatchFile(filename)
		if !result.Matched {
			unmatched = append(unmatched, filename)
			continue
		}

		if result.IsConsolidated {
			if consolidated == "" || result.Confidence > 50 {
				consolidated = filename
			}
		} else {
			// Keep higher confidence matches
			existing, exists := tasks[result.TaskID]
			if !exists {
				tasks[result.TaskID] = filename
			} else {
				// Compare confidences (prefer current if file looks more specific)
				existingResult := m.MatchFile(existing)
				if result.Confidence > existingResult.Confidence {
					tasks[result.TaskID] = filename
				}
			}
		}
	}

	return consolidated, tasks, unmatched
}

// GetMissingTasks returns task IDs that don't have matching files.
func (m *FileMatcher) GetMissingTasks(matchedTasks map[string]string) []string {
	var missing []string
	for _, task := range m.taskFiles {
		if _, found := matchedTasks[task.ID]; !found {
			missing = append(missing, task.ID)
		}
	}
	return missing
}

// extractTaskNumber extracts a task number from a filename.
// Handles formats like: 01-foo.md, task-1-foo.md, issue-1.md
func extractTaskNumber(filename string) int {
	// Try different patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^(\d+)-`),           // 01-foo.md
		regexp.MustCompile(`task-(\d+)`),        // task-1-foo.md
		regexp.MustCompile(`issue-(\d+)`),       // issue-1.md
		regexp.MustCompile(`-(\d+)\.md$`),       // foo-1.md
		regexp.MustCompile(`(\d+)\.md$`),        // 1.md
	}

	for _, re := range patterns {
		if match := re.FindStringSubmatch(filename); len(match) > 1 {
			if num, err := strconv.Atoi(match[1]); err == nil {
				// Skip 0 as that's usually consolidated
				if num > 0 {
					return num
				}
			}
		}
	}

	return 0
}

// NormalizeTaskSlug converts a task name to a slug format.
func NormalizeTaskSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and underscores with dashes
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove non-alphanumeric characters except dashes
	re := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = re.ReplaceAllString(slug, "")

	// Collapse multiple dashes
	re = regexp.MustCompile(`-+`)
	slug = re.ReplaceAllString(slug, "-")

	// Trim dashes from ends
	slug = strings.Trim(slug, "-")

	return slug
}
