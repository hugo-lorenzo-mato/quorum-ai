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

// WriteIssuesToDisk writes issues to markdown files under the draft directory.
// Files include YAML frontmatter with structured metadata.
// It returns previews that include the file paths written.
func (g *Generator) WriteIssuesToDisk(workflowID string, inputs []IssueInput) ([]IssuePreview, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflowID is required")
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no issues provided")
	}

	draftDirAbs, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return nil, fmt.Errorf("resolving draft directory: %w", err)
	}
	if err := os.MkdirAll(draftDirAbs, 0o755); err != nil {
		return nil, fmt.Errorf("creating draft directory: %w", err)
	}

	baseDir := g.resolveIssuesBaseDir()
	draftDirRel := filepath.Join(baseDir, workflowID, "draft")

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

		labels := input.Labels
		if input.IsMainIssue {
			labels = ensureEpicLabel(labels)
		}

		fm := DraftFrontmatter{
			Title:       input.Title,
			Labels:      labels,
			Assignees:   input.Assignees,
			IsMainIssue: input.IsMainIssue,
			TaskID:      input.TaskID,
			SourcePath:  input.FilePath,
			Status:      "draft",
		}

		absPath, err := g.WriteDraftFile(workflowID, fileName, fm, input.Body)
		if err != nil {
			return nil, fmt.Errorf("writing draft file %q: %w", fileName, err)
		}
		_ = absPath

		previews = append(previews, IssuePreview{
			Title:       input.Title,
			Body:        input.Body,
			Labels:      labels,
			Assignees:   input.Assignees,
			IsMainIssue: input.IsMainIssue,
			TaskID:      input.TaskID,
			FilePath:    filepath.Join(draftDirRel, fileName),
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

	totalToPublish := len(subInputs)
	if mainInput != nil {
		totalToPublish++
	}
	createdCount := 0
	if totalToPublish > 0 {
		g.emitIssuesPublishingProgress(workflowID, "started", 0, totalToPublish, nil, 0, dryRun, "issue publishing started")
	}

	var mainIssue *core.Issue
	var mappingEntries []IssueMappingEntry

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
			createdCount++
			g.emitIssuesPublishingProgress(workflowID, "progress", createdCount, totalToPublish, &ProgressIssue{
				Title:       title,
				TaskID:      mainInput.TaskID,
				IsMainIssue: true,
				FileName:    filepath.Base(filePath),
			}, 0, true, "")
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
			createdCount++
			g.emitIssuesPublishingProgress(workflowID, "progress", createdCount, totalToPublish, &ProgressIssue{
				Title:       issue.Title,
				TaskID:      mainInput.TaskID,
				IsMainIssue: true,
				FileName:    filepath.Base(filePath),
			}, issue.Number, false, "")
			mappingEntries = append(mappingEntries, IssueMappingEntry{
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
			createdCount++
			g.emitIssuesPublishingProgress(workflowID, "progress", createdCount, totalToPublish, &ProgressIssue{
				Title:       title,
				TaskID:      input.TaskID,
				IsMainIssue: false,
				FileName:    filepath.Base(filePath),
			}, 0, true, "")
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
		createdCount++
		g.emitIssuesPublishingProgress(workflowID, "progress", createdCount, totalToPublish, &ProgressIssue{
			Title:       issue.Title,
			TaskID:      input.TaskID,
			IsMainIssue: false,
			FileName:    filepath.Base(filePath),
		}, issue.Number, false, "")
		if parentNum > 0 && issue.ParentIssue == 0 {
			result.Errors = append(result.Errors, fmt.Errorf("linking sub-issue #%d to parent #%d failed", issue.Number, parentNum))
		}
		mappingEntries = append(mappingEntries, IssueMappingEntry{
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

	if totalToPublish > 0 {
		g.emitIssuesPublishingProgress(workflowID, "completed", createdCount, totalToPublish, nil, 0, dryRun, "issue publishing completed")
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
	(*maxIndex)++
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
	if err := ValidateWorkflowID(workflowID); err != nil {
		return "", "", "", err
	}

	root, err := g.getProjectRoot()
	if err != nil {
		return "", "", "", fmt.Errorf("resolving project root: %w", err)
	}

	baseDir := g.resolveIssuesBaseDir()
	issuesDirRel := filepath.Join(baseDir, workflowID)
	issuesDirAbs := filepath.Join(root, issuesDirRel)

	path := input.FilePath
	if path == "" {
		return "", "", "", fmt.Errorf("issue file path is required")
	}

	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
		relPath, _ = filepath.Rel(root, path)
	} else if strings.ContainsAny(path, "/\\") || strings.HasPrefix(path, ".quorum") || strings.HasPrefix(path, baseDir) {
		absPath = filepath.Join(root, path)
		relPath = path
	} else {
		// Bare filename: look in draft/ subdirectory first, then fallback to base
		draftPath := filepath.Join(issuesDirAbs, "draft", path)
		if _, statErr := os.Stat(draftPath); statErr == nil {
			absPath = draftPath
			relPath = filepath.Join(issuesDirRel, "draft", path)
		} else {
			absPath = filepath.Join(issuesDirAbs, path)
			relPath = filepath.Join(issuesDirRel, path)
		}
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

	// Try parsing frontmatter first, fallback to plain markdown
	fm, fmBody, fmErr := parseDraftContent(string(content))
	if fmErr == nil && fm != nil {
		return fm.Title, fmBody, relPath, nil
	}

	title, body = parseIssueMarkdown(string(content))
	return title, body, relPath, nil
}

// IssueMapping holds the mapping between draft files and published issues.
type IssueMapping struct {
	WorkflowID  string              `json:"workflow_id"`
	GeneratedAt time.Time           `json:"generated_at"`
	Issues      []IssueMappingEntry `json:"issues"`
}

// IssueMappingEntry records a single published issue mapping.
type IssueMappingEntry struct {
	TaskID      string `json:"task_id,omitempty"`
	FilePath    string `json:"file_path"`
	IssueNumber int    `json:"issue_number"`
	IssueID     int64  `json:"issue_id,omitempty"`
	IsMain      bool   `json:"is_main_issue"`
	ParentIssue int    `json:"parent_issue,omitempty"`
}

func (g *Generator) writeIssueMappingFile(workflowID string, entries []IssueMappingEntry) error {
	mapping := IssueMapping{
		WorkflowID:  workflowID,
		GeneratedAt: time.Now(),
	}
	mapping.Issues = entries

	publishedDir, err := g.resolvePublishedDir(workflowID)
	if err != nil {
		return fmt.Errorf("resolving published directory: %w", err)
	}
	if err := os.MkdirAll(publishedDir, 0o755); err != nil {
		return fmt.Errorf("creating published directory: %w", err)
	}

	mappingPath := filepath.Join(publishedDir, "mapping.json")
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mapping: %w", err)
	}
	return os.WriteFile(mappingPath, data, 0o600)
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
