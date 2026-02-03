package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

const mainIssueFilename = "00-consolidated-analysis.md"

// WriteIssuesToDisk writes issues to markdown files under .quorum/issues/{workflowID}.
// It returns previews that include the file paths written.
func (g *Generator) WriteIssuesToDisk(workflowID string, inputs []IssueInput) ([]IssuePreview, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflowID is required")
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no issues provided")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	issuesDirRel := filepath.Join(".quorum", "issues", workflowID)
	issuesDirAbs := filepath.Join(cwd, issuesDirRel)
	if err := os.MkdirAll(issuesDirAbs, 0755); err != nil {
		return nil, fmt.Errorf("creating issues directory: %w", err)
	}

	// Build task index for stable filenames
	taskFiles, _ := g.getTaskFilePaths()
	tasksByID := make(map[string]service.IssueTaskFile, len(taskFiles))
	maxIndex := 0
	for _, task := range taskFiles {
		tasksByID[task.ID] = task
		if task.Index > maxIndex {
			maxIndex = task.Index
		}
	}

	used := make(map[string]bool)

	// Pre-seed used filenames from explicit file paths
	for _, input := range inputs {
		if input.FilePath != "" {
			name := filepath.Base(input.FilePath)
			if name != "" {
				used[name] = true
				if idx := extractFileNumber(name); idx > maxIndex {
					maxIndex = idx
				}
			}
		}
	}

	previews := make([]IssuePreview, 0, len(inputs))
	for _, input := range inputs {
		fileName := g.resolveIssueFilename(input, tasksByID, used, &maxIndex)
		filePath, err := ValidateOutputPath(issuesDirAbs, fileName)
		if err != nil {
			return nil, fmt.Errorf("validating issue file path %q: %w", fileName, err)
		}

		content := buildIssueMarkdown(input.Title, input.Body)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("writing issue file %s: %w", filePath, err)
		}

		labels := input.Labels
		if input.IsMainIssue {
			labels = ensureEpicLabel(labels)
		}

		previews = append(previews, IssuePreview{
			Title:       input.Title,
			Body:        input.Body,
			Labels:      labels,
			Assignees:   input.Assignees,
			IsMainIssue: input.IsMainIssue,
			TaskID:      input.TaskID,
			FilePath:    filepath.Join(issuesDirRel, fileName),
		})
	}

	return previews, nil
}

// CreateIssuesFromFiles creates issues by reading markdown files from disk.
// If inputs is empty, it reads from the generated issues directory.
func (g *Generator) CreateIssuesFromFiles(ctx context.Context, workflowID string, inputs []IssueInput, dryRun, linkIssues bool, defaultLabels, defaultAssignees []string) (*GenerateResult, error) {
	result := &GenerateResult{
		IssueSet: &core.IssueSet{
			WorkflowID:  workflowID,
			GeneratedAt: time.Now(),
		},
	}

	if workflowID == "" {
		return nil, fmt.Errorf("workflowID is required")
	}

	if len(inputs) == 0 {
		previews, err := g.ReadGeneratedIssues(workflowID)
		if err != nil {
			return nil, fmt.Errorf("reading generated issues: %w", err)
		}
		for _, preview := range previews {
			inputs = append(inputs, IssueInput{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      defaultLabels,
				Assignees:   defaultAssignees,
				IsMainIssue: preview.IsMainIssue,
				TaskID:      preview.TaskID,
				FilePath:    preview.FilePath,
			})
		}
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("no issues available to create")
	}

	// Separate main issue from sub-issues
	var mainInput *IssueInput
	var subInputs []IssueInput
	for i := range inputs {
		if inputs[i].IsMainIssue {
			mainInput = &inputs[i]
		} else {
			subInputs = append(subInputs, inputs[i])
		}
	}

	var mainIssue *core.Issue
	var mappingEntries []issueMappingEntry

	// Create main issue
	if mainInput != nil {
		title, body, filePath, err := g.readIssueFile(workflowID, *mainInput)
		if err != nil {
			return nil, fmt.Errorf("reading main issue file: %w", err)
		}

		labels := mainInput.Labels
		if len(labels) == 0 {
			labels = defaultLabels
		}
		labels = ensureEpicLabel(labels)

		assignees := mainInput.Assignees
		if len(assignees) == 0 {
			assignees = defaultAssignees
		}

		if dryRun {
			result.PreviewIssues = append(result.PreviewIssues, IssuePreview{
				Title:       title,
				Body:        body,
				Labels:      labels,
				Assignees:   assignees,
				IsMainIssue: true,
				TaskID:      mainInput.TaskID,
				FilePath:    filePath,
			})
		} else {
			issue, err := g.client.CreateIssue(ctx, core.CreateIssueOptions{
				Title:     title,
				Body:      body,
				Labels:    labels,
				Assignees: assignees,
			})
			if err != nil {
				return nil, fmt.Errorf("creating main issue: %w", err)
			}
			mainIssue = issue
			result.IssueSet.MainIssue = mainIssue
			mappingEntries = append(mappingEntries, issueMappingEntry{
				TaskID:      mainInput.TaskID,
				FilePath:    filePath,
				IssueNumber: issue.Number,
				IssueID:     issue.ID,
				IsMain:      true,
				ParentIssue: 0,
			})
		}
	}

	// Create sub-issues
	for _, input := range subInputs {
		title, body, filePath, err := g.readIssueFile(workflowID, input)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("reading issue file for %s: %w", input.TaskID, err))
			continue
		}

		labels := input.Labels
		if len(labels) == 0 {
			labels = defaultLabels
		}
		assignees := input.Assignees
		if len(assignees) == 0 {
			assignees = defaultAssignees
		}

		if dryRun {
			result.PreviewIssues = append(result.PreviewIssues, IssuePreview{
				Title:       title,
				Body:        body,
				Labels:      labels,
				Assignees:   assignees,
				IsMainIssue: false,
				TaskID:      input.TaskID,
				FilePath:    filePath,
			})
			continue
		}

		parentNum := 0
		if linkIssues && mainIssue != nil {
			parentNum = mainIssue.Number
		}

		issue, err := g.client.CreateIssue(ctx, core.CreateIssueOptions{
			Title:       title,
			Body:        body,
			Labels:      labels,
			Assignees:   assignees,
			ParentIssue: parentNum,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("creating issue '%s': %w", title, err))
			continue
		}
		result.IssueSet.SubIssues = append(result.IssueSet.SubIssues, issue)
		if parentNum > 0 && issue.ParentIssue == 0 {
			result.Errors = append(result.Errors, fmt.Errorf("linking sub-issue #%d to parent #%d failed", issue.Number, parentNum))
		}
		mappingEntries = append(mappingEntries, issueMappingEntry{
			TaskID:      input.TaskID,
			FilePath:    filePath,
			IssueNumber: issue.Number,
			IssueID:     issue.ID,
			IsMain:      false,
			ParentIssue: issue.ParentIssue,
		})
	}

	if !dryRun {
		if err := g.writeIssueMappingFile(workflowID, mappingEntries); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("writing issue mapping: %w", err))
		}
	}

	return result, nil
}

func (g *Generator) resolveIssueFilename(input IssueInput, tasksByID map[string]service.IssueTaskFile, used map[string]bool, maxIndex *int) string {
	if input.FilePath != "" {
		name := filepath.Base(input.FilePath)
		if name != "" {
			return uniqueIssueFilename(name, used)
		}
	}

	if input.IsMainIssue {
		return uniqueIssueFilename(mainIssueFilename, used)
	}

	if input.TaskID != "" {
		if task, ok := tasksByID[input.TaskID]; ok {
			name := issueFilenameForTask(task)
			if idx := extractFileNumber(name); idx > *maxIndex {
				*maxIndex = idx
			}
			return uniqueIssueFilename(name, used)
		}
	}

	slug := sanitizeFilename(input.Title)
	if slug == "" {
		slug = "issue"
	}
	*maxIndex = *maxIndex + 1
	name := fmt.Sprintf("%02d-%s.md", *maxIndex, slug)
	return uniqueIssueFilename(name, used)
}

func issueFilenameForTask(task service.IssueTaskFile) string {
	return fmt.Sprintf("%02d-%s.md", task.Index, task.Slug)
}

func uniqueIssueFilename(name string, used map[string]bool) string {
	if !used[name] {
		used[name] = true
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

func buildIssueMarkdown(title, body string) string {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		title = "Untitled Issue"
	}
	if body == "" {
		return fmt.Sprintf("# %s\n", title)
	}
	return fmt.Sprintf("# %s\n\n%s\n", title, body)
}

func (g *Generator) readIssueFile(workflowID string, input IssueInput) (title, body, relPath string, err error) {
	issuesDirRel := filepath.Join(".quorum", "issues", workflowID)
	issuesDirAbs := issuesDirRel
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		issuesDirAbs = filepath.Join(cwd, issuesDirRel)
	}

	path := input.FilePath
	if path == "" {
		return "", "", "", fmt.Errorf("issue file path is required")
	}

	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
		if cwdErr == nil {
			relPath, _ = filepath.Rel(cwd, path)
		} else {
			relPath = path
		}
	} else if strings.ContainsAny(path, "/\\") || strings.HasPrefix(path, ".quorum") {
		if cwdErr == nil {
			absPath = filepath.Join(cwd, path)
		} else {
			absPath = path
		}
		relPath = path
	} else {
		absPath = filepath.Join(issuesDirAbs, path)
		relPath = filepath.Join(issuesDirRel, path)
	}

	absIssuesDir, err := filepath.Abs(issuesDirAbs)
	if err != nil {
		return "", "", "", fmt.Errorf("resolving issues directory: %w", err)
	}
	absResolved, err := filepath.Abs(absPath)
	if err != nil {
		return "", "", "", fmt.Errorf("resolving issue file path: %w", err)
	}
	if !strings.HasPrefix(absResolved, absIssuesDir+string(filepath.Separator)) && absResolved != absIssuesDir {
		return "", "", "", fmt.Errorf("issue file is outside issues dir: %s", absResolved)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", "", fmt.Errorf("reading issue file %s: %w", absPath, err)
	}

	title, body = parseIssueMarkdown(string(content))
	return title, body, relPath, nil
}

type issueMapping struct {
	WorkflowID  string              `json:"workflow_id"`
	GeneratedAt time.Time           `json:"generated_at"`
	Issues      []issueMappingEntry `json:"issues"`
}

type issueMappingEntry struct {
	TaskID      string `json:"task_id,omitempty"`
	FilePath    string `json:"file_path"`
	IssueNumber int    `json:"issue_number"`
	IssueID     int64  `json:"issue_id,omitempty"`
	IsMain      bool   `json:"is_main_issue"`
	ParentIssue int    `json:"parent_issue,omitempty"`
}

func (g *Generator) writeIssueMappingFile(workflowID string, entries []issueMappingEntry) error {
	mapping := issueMapping{
		WorkflowID:  workflowID,
		GeneratedAt: time.Now(),
	}
	mapping.Issues = entries

	issuesDir := filepath.Join(".quorum", "issues", workflowID)
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		return fmt.Errorf("creating issues directory: %w", err)
	}

	mappingPath := filepath.Join(issuesDir, "mapping.json")
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mapping: %w", err)
	}
	return os.WriteFile(mappingPath, data, 0644)
}

func ensureEpicLabel(labels []string) []string {
	return ensureLabel(labels, "epic")
}

func ensureLabel(labels []string, target string) []string {
	for _, label := range labels {
		if strings.EqualFold(label, target) {
			return labels
		}
	}
	return append(append([]string{}, labels...), target)
}
