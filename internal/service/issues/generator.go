package issues

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// Error format strings for duplicated messages (S1192).
const (
	errResolvingProjectRoot = "resolving project root: %w"
	errGettingProjectRoot   = "getting project root: %w"
	errResolvingDraftDir    = "resolving draft directory: %w"
)

// IssueInput represents a single issue to be created (from frontend edits).
type IssueInput struct {
	Title       string
	Body        string
	Labels      []string
	Assignees   []string
	IsMainIssue bool
	TaskID      string
	FilePath    string
}

// Generator creates GitHub/GitLab issues from workflow artifacts.
type Generator struct {
	client      core.IssueClient
	config      config.IssuesConfig
	projectRoot string                  // Project root directory (replaces os.Getwd)
	reportDir   string
	agents      core.AgentRegistry      // Optional: for LLM-based generation
	prompts     *service.PromptRenderer // Lazy-initialized prompt renderer

	// LLM generation cache - prevents regenerating files within same workflow
	llmGenerationCache map[string][]IssuePreview // workflowID -> generated issues

	progress ProgressReporter // Optional: progress reporting for SSE/UI
}

// NewGenerator creates a new issue generator.
// projectRoot is the project root directory; if empty, falls back to os.Getwd().
// agents can be nil if LLM-based generation is not needed.
func NewGenerator(client core.IssueClient, cfg config.IssuesConfig, projectRoot, reportDir string, agents core.AgentRegistry) *Generator {
	return &Generator{
		client:             client,
		config:             cfg,
		projectRoot:        projectRoot,
		reportDir:          reportDir,
		agents:             agents,
		llmGenerationCache: make(map[string][]IssuePreview),
	}
}

// SetProgressReporter sets an optional progress reporter for generation/publishing progress.
func (g *Generator) SetProgressReporter(r ProgressReporter) {
	g.progress = r
}

func (g *Generator) emitIssuesGenerationProgress(workflowID, stage string, current, total int, issue *ProgressIssue, message string) {
	if g.progress == nil {
		return
	}
	g.progress.OnIssuesGenerationProgress(workflowID, stage, current, total, issue, message)
}

// PublishingProgressParams holds parameters for emitting a publishing progress event.
type PublishingProgressParams struct {
	WorkflowID  string
	Stage       string
	Current     int
	Total       int
	Issue       *ProgressIssue
	IssueNumber int
	DryRun      bool
	Message     string
}

func (g *Generator) emitIssuesPublishingProgress(p PublishingProgressParams) {
	if g.progress == nil {
		return
	}
	g.progress.OnIssuesPublishingProgress(p)
}

// getProjectRoot returns the project root, falling back to os.Getwd() if not set.
func (g *Generator) getProjectRoot() (string, error) {
	if g.projectRoot != "" {
		return g.projectRoot, nil
	}
	return os.Getwd()
}

// resolveDraftDir returns the absolute path to the draft directory for a workflow.
// Uses DraftDirectory from config if set, otherwise defaults to .quorum/issues/.
func (g *Generator) resolveDraftDir(workflowID string) (string, error) {
	if err := ValidateWorkflowID(workflowID); err != nil {
		return "", err
	}
	root, err := g.getProjectRoot()
	if err != nil {
		return "", fmt.Errorf(errResolvingProjectRoot, err)
	}
	baseDir := ".quorum/issues"
	if g.config.DraftDirectory != "" {
		baseDir = filepath.Clean(g.config.DraftDirectory)
	}
	result := filepath.Join(root, baseDir, workflowID, "draft")
	if err := validatePathUnderRoot(result, root); err != nil {
		return "", err
	}
	return result, nil
}

// resolvePublishedDir returns the absolute path to the published directory for a workflow.
func (g *Generator) resolvePublishedDir(workflowID string) (string, error) {
	if err := ValidateWorkflowID(workflowID); err != nil {
		return "", err
	}
	root, err := g.getProjectRoot()
	if err != nil {
		return "", fmt.Errorf(errResolvingProjectRoot, err)
	}
	baseDir := ".quorum/issues"
	if g.config.DraftDirectory != "" {
		baseDir = filepath.Clean(g.config.DraftDirectory)
	}
	result := filepath.Join(root, baseDir, workflowID, "published")
	if err := validatePathUnderRoot(result, root); err != nil {
		return "", err
	}
	return result, nil
}

// resolveIssuesBaseDir returns the relative base directory for issues (without workflow subdirectory).
func (g *Generator) resolveIssuesBaseDir() string {
	if g.config.DraftDirectory != "" {
		return g.config.DraftDirectory
	}
	return filepath.Join(".quorum", "issues")
}

// getPromptRenderer returns the prompt renderer, initializing it lazily if needed.
func (g *Generator) getPromptRenderer() (*service.PromptRenderer, error) {
	if g.prompts != nil {
		return g.prompts, nil
	}

	renderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, fmt.Errorf("creating prompt renderer: %w", err)
	}
	g.prompts = renderer
	return renderer, nil
}

// GenerateOptions configures issue generation.
type GenerateOptions struct {
	// WorkflowID is the workflow identifier.
	WorkflowID string

	// DryRun previews issues without creating them.
	DryRun bool

	// CreateMainIssue creates a parent issue from consolidated analysis.
	CreateMainIssue bool

	// CreateSubIssues creates child issues for each task.
	CreateSubIssues bool

	// LinkIssues links sub-issues to the main issue.
	LinkIssues bool

	// CustomLabels overrides default labels.
	CustomLabels []string

	// CustomAssignees overrides default assignees.
	CustomAssignees []string
}

// DefaultGenerateOptions returns sensible defaults.
func DefaultGenerateOptions(workflowID string) GenerateOptions {
	return GenerateOptions{
		WorkflowID:      workflowID,
		DryRun:          false,
		CreateMainIssue: true,
		CreateSubIssues: true,
		LinkIssues:      true,
	}
}

// GenerateResult contains the result of issue generation.
type GenerateResult struct {
	// IssueSet contains created issues.
	IssueSet *core.IssueSet

	// PreviewIssues contains issue previews (dry-run mode).
	PreviewIssues []IssuePreview

	// Errors contains non-fatal errors during generation.
	Errors []error

	// AIUsed indicates whether AI generation was attempted and succeeded.
	AIUsed bool

	// AIErrors contains AI-specific errors (helps debugging).
	AIErrors []string
}

// IssuePreview represents an issue that would be created.
type IssuePreview struct {
	Title       string
	Body        string
	Labels      []string
	Assignees   []string
	IsMainIssue bool
	TaskID      string
	FilePath    string
}

type expectedIssueFile struct {
	FileName string
	TaskID   string
	IsMain   bool
	Task     *service.IssueTaskFile
}

// GenerationTracker tracks expected vs actual generated files to avoid race conditions.
// This replaces the fragile timestamp-based detection.
type GenerationTracker struct {
	// ExpectedFiles maps expected filename to task info.
	ExpectedFiles map[string]string // filename -> taskID or "main"

	// GeneratedFiles maps filename to generation time.
	GeneratedFiles map[string]time.Time

	// StartTime is when generation began.
	StartTime time.Time

	// WorkflowID identifies the workflow being processed.
	WorkflowID string
}

// NewGenerationTracker creates a new tracker for a generation run.
func NewGenerationTracker(workflowID string) *GenerationTracker {
	return &GenerationTracker{
		ExpectedFiles:  make(map[string]string),
		GeneratedFiles: make(map[string]time.Time),
		StartTime:      time.Now(),
		WorkflowID:     workflowID,
	}
}

// AddExpected registers an expected file.
func (t *GenerationTracker) AddExpected(filename, taskID string) {
	t.ExpectedFiles[filename] = taskID
}

// MarkGenerated marks a file as generated.
func (t *GenerationTracker) MarkGenerated(filename string, modTime time.Time) {
	t.GeneratedFiles[filename] = modTime
}

// GetMissingFiles returns files that were expected but not generated.
func (t *GenerationTracker) GetMissingFiles() []string {
	var missing []string
	for expected := range t.ExpectedFiles {
		if _, found := t.GeneratedFiles[expected]; !found {
			missing = append(missing, expected)
		}
	}
	return missing
}

// IsValidFile checks if a file should be considered as generated
// (created after start time and either expected or matching pattern).
func (t *GenerationTracker) IsValidFile(filename string, modTime time.Time) bool {
	// Must be created/modified after generation started
	if modTime.Before(t.StartTime) {
		return false
	}

	// If we have expected files, check against them
	if len(t.ExpectedFiles) > 0 {
		// Direct match
		if _, ok := t.ExpectedFiles[filename]; ok {
			return true
		}
		// Fuzzy match for variations
		for expected := range t.ExpectedFiles {
			if fuzzyMatchFilename(filename, expected) {
				return true
			}
		}
	}

	// If no expected files defined, accept any .md file created after start
	return true
}

// fuzzyMatchFilename checks if two filenames are similar enough to be considered the same.
func fuzzyMatchFilename(actual, expected string) bool {
	// Remove extension
	actualBase := strings.TrimSuffix(actual, ".md")
	expectedBase := strings.TrimSuffix(expected, ".md")

	// Exact match
	if actualBase == expectedBase {
		return true
	}

	// Check if one contains the other (handles prefix/suffix variations)
	if strings.Contains(actualBase, expectedBase) || strings.Contains(expectedBase, actualBase) {
		return true
	}

	// Check numeric prefix match (01-foo vs 1-foo)
	actualNum := extractLeadingNumber(actualBase)
	expectedNum := extractLeadingNumber(expectedBase)
	if actualNum == expectedNum && actualNum != "" {
		// Same number, check rest
		actualRest := strings.TrimLeft(actualBase, "0123456789-")
		expectedRest := strings.TrimLeft(expectedBase, "0123456789-")
		if actualRest == expectedRest {
			return true
		}
	}

	return false
}

// extractLeadingNumber extracts leading digits from a string.
func extractLeadingNumber(s string) string {
	re := regexp.MustCompile(`^(\d+)`)
	match := re.FindString(s)
	return match
}

// Generate creates issues from workflow artifacts.
func (g *Generator) Generate(ctx context.Context, opts GenerateOptions) (*GenerateResult, error) {
	result := &GenerateResult{
		IssueSet: &core.IssueSet{
			WorkflowID:  opts.WorkflowID,
			GeneratedAt: time.Now(),
		},
	}

	// Determine labels and assignees
	labels := g.config.Labels
	if len(opts.CustomLabels) > 0 {
		labels = opts.CustomLabels
	}

	assignees := g.config.Assignees
	if len(opts.CustomAssignees) > 0 {
		assignees = opts.CustomAssignees
	}
	mainLabels := ensureEpicLabel(labels)

	createdCount := 0
	totalToPublish := 0

	// Pre-read task files so we can emit deterministic publishing progress.
	var tasks []TaskInfo
	if opts.CreateSubIssues {
		var err error
		tasks, err = g.readTaskFiles()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("reading task files: %w", err))
		}
		totalToPublish += len(tasks)
	}

	// Read consolidated analysis for main issue
	var consolidatedContent string
	var mainIssue *core.Issue

	if opts.CreateMainIssue {
		var err error
		consolidatedContent, err = g.readConsolidatedAnalysis()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("reading consolidated analysis: %w", err))
		}

		if consolidatedContent != "" {
			totalToPublish++
		}

		// Emit a "started" publishing event once we know the total.
		if totalToPublish > 0 {
			g.emitIssuesPublishingProgress(PublishingProgressParams{
				WorkflowID: opts.WorkflowID, Stage: "started", Total: totalToPublish,
				DryRun: opts.DryRun, Message: "issue publishing started",
			})
		}

		if consolidatedContent != "" {
			var mainTitle, mainBody string
			var aiSucceeded bool

			// Try LLM-based generation if enabled
			if g.config.Generator.Enabled {
				slog.Info("attempting LLM generation for main issue",
					"agent", g.config.Generator.Agent,
					"model", g.config.Generator.Model)
				mainTitle, mainBody, err = g.generateWithLLM(ctx, consolidatedContent, "", opts.WorkflowID, true)
				if err != nil {
					aiErr := fmt.Sprintf("main issue: %v", err)
					result.AIErrors = append(result.AIErrors, aiErr)
					slog.Warn("LLM generation failed, falling back to direct copy",
						"error", err)
				} else {
					aiSucceeded = true
				}
			}

			// Fallback to direct copy if LLM failed or disabled
			if mainTitle == "" || mainBody == "" {
				mainTitle = g.formatTitle(opts.WorkflowID, "", true)
				mainBody = g.formatMainIssueBody(consolidatedContent, opts.WorkflowID)
			} else if aiSucceeded {
				result.AIUsed = true
			}

			if opts.DryRun {
				result.PreviewIssues = append(result.PreviewIssues, IssuePreview{
					Title:       mainTitle,
					Body:        mainBody,
					Labels:      mainLabels,
					Assignees:   assignees,
					IsMainIssue: true,
				})
				createdCount++
				g.emitIssuesPublishingProgress(PublishingProgressParams{
					WorkflowID: opts.WorkflowID, Stage: "progress", Current: createdCount, Total: totalToPublish,
					Issue: &ProgressIssue{Title: mainTitle, TaskID: "main", IsMainIssue: true}, DryRun: true,
				})
			} else {
				mainIssue, err = g.client.CreateIssue(ctx, core.CreateIssueOptions{
					Title:     mainTitle,
					Body:      mainBody,
					Labels:    mainLabels,
					Assignees: assignees,
				})
				if err != nil {
					return nil, fmt.Errorf("creating main issue: %w", err)
				}
				result.IssueSet.MainIssue = mainIssue
				createdCount++
				g.emitIssuesPublishingProgress(PublishingProgressParams{
					WorkflowID: opts.WorkflowID, Stage: "progress", Current: createdCount, Total: totalToPublish,
					Issue: &ProgressIssue{Title: mainIssue.Title, TaskID: "main", IsMainIssue: true}, IssueNumber: mainIssue.Number,
				})
			}
		}
	} else if totalToPublish > 0 {
		// No main issue: still emit a started event for sub-issue publishing.
		g.emitIssuesPublishingProgress(PublishingProgressParams{
			WorkflowID: opts.WorkflowID, Stage: "started", Total: totalToPublish,
			DryRun: opts.DryRun, Message: "issue publishing started",
		})
	}

	// Read and create sub-issues from task files
	if opts.CreateSubIssues {
		for _, task := range tasks {
			var taskTitle, taskBody string
			var taskAISucceeded bool
			var err error

			// Try LLM-based generation if enabled
			if g.config.Generator.Enabled {
				taskTitle, taskBody, err = g.generateWithLLM(ctx, task.Content, task.ID, opts.WorkflowID, false)
				if err != nil {
					aiErr := fmt.Sprintf("task %s: %v", task.ID, err)
					result.AIErrors = append(result.AIErrors, aiErr)
					slog.Warn("LLM generation failed for task, falling back to direct copy",
						"task_id", task.ID,
						"error", err)
				} else {
					taskAISucceeded = true
				}
			}

			// Fallback to direct copy if LLM failed or disabled
			if taskTitle == "" || taskBody == "" {
				taskTitle = g.formatTitle(task.ID, task.Name, false)
				taskBody = g.formatTaskIssueBody(task)
			} else if taskAISucceeded {
				result.AIUsed = true
			}

			if opts.DryRun {
				result.PreviewIssues = append(result.PreviewIssues, IssuePreview{
					Title:       taskTitle,
					Body:        taskBody,
					Labels:      labels,
					Assignees:   assignees,
					IsMainIssue: false,
					TaskID:      task.ID,
				})
				createdCount++
				g.emitIssuesPublishingProgress(PublishingProgressParams{
					WorkflowID: opts.WorkflowID, Stage: "progress", Current: createdCount, Total: totalToPublish,
					Issue: &ProgressIssue{Title: taskTitle, TaskID: task.ID}, DryRun: true,
				})
			} else {
				parentNum := 0
				if opts.LinkIssues && mainIssue != nil {
					parentNum = mainIssue.Number
				}

				subIssue, err := g.client.CreateIssue(ctx, core.CreateIssueOptions{
					Title:       taskTitle,
					Body:        taskBody,
					Labels:      labels,
					Assignees:   assignees,
					ParentIssue: parentNum,
				})
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("creating issue for %s: %w", task.ID, err))
					continue
				}
				result.IssueSet.SubIssues = append(result.IssueSet.SubIssues, subIssue)
				createdCount++
				g.emitIssuesPublishingProgress(PublishingProgressParams{
					WorkflowID: opts.WorkflowID, Stage: "progress", Current: createdCount, Total: totalToPublish,
					Issue: &ProgressIssue{Title: subIssue.Title, TaskID: task.ID}, IssueNumber: subIssue.Number,
				})
			}
		}
	}

	// Make sure we always end with a "completed" progress update when we had a defined total.
	if totalToPublish > 0 {
		g.emitIssuesPublishingProgress(PublishingProgressParams{
			WorkflowID: opts.WorkflowID, Stage: "completed", Current: createdCount, Total: totalToPublish,
			DryRun: opts.DryRun, Message: "issue publishing completed",
		})
	}

	return result, nil
}

// CreateIssuesFromInput creates issues directly from frontend input (edited issues).
// This bypasses filesystem reading and uses the provided issue data.
func (g *Generator) CreateIssuesFromInput(ctx context.Context, inputs []IssueInput, dryRun, linkIssues bool, defaultLabels, defaultAssignees []string) (*GenerateResult, error) {
	result := &GenerateResult{
		IssueSet: &core.IssueSet{
			GeneratedAt: time.Now(),
		},
	}

	var mainIssue *core.Issue

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

	// Create main issue if present
	if mainInput != nil {
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
				Title:       mainInput.Title,
				Body:        mainInput.Body,
				Labels:      labels,
				Assignees:   assignees,
				IsMainIssue: true,
				FilePath:    mainInput.FilePath,
			})
		} else {
			issue, err := g.client.CreateIssue(ctx, core.CreateIssueOptions{
				Title:     mainInput.Title,
				Body:      mainInput.Body,
				Labels:    labels,
				Assignees: assignees,
			})
			if err != nil {
				return nil, fmt.Errorf("creating main issue: %w", err)
			}
			mainIssue = issue
			result.IssueSet.MainIssue = mainIssue
			slog.Info("created main issue", "number", issue.Number, "title", issue.Title)
		}
	}

	// Create sub-issues
	for _, input := range subInputs {
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
				Title:       input.Title,
				Body:        input.Body,
				Labels:      labels,
				Assignees:   assignees,
				IsMainIssue: false,
				TaskID:      input.TaskID,
				FilePath:    input.FilePath,
			})
		} else {
			parentNum := 0
			if linkIssues && mainIssue != nil {
				parentNum = mainIssue.Number
			}

			issue, err := g.client.CreateIssue(ctx, core.CreateIssueOptions{
				Title:       input.Title,
				Body:        input.Body,
				Labels:      labels,
				Assignees:   assignees,
				ParentIssue: parentNum,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("creating issue '%s': %w", input.Title, err))
				continue
			}
			result.IssueSet.SubIssues = append(result.IssueSet.SubIssues, issue)
			slog.Info("created sub-issue", "number", issue.Number, "title", issue.Title, "task_id", input.TaskID)
		}
	}

	slog.Info("created issues from frontend input",
		"total", len(result.IssueSet.SubIssues)+1,
		"main", mainIssue != nil,
		"sub_issues", len(result.IssueSet.SubIssues))

	return result, nil
}

// TaskInfo contains parsed task information.
type TaskInfo struct {
	ID           string
	Name         string
	Agent        string
	Complexity   string
	Dependencies []string
	Content      string
}

// readConsolidatedAnalysis reads the consolidated analysis file.
func (g *Generator) readConsolidatedAnalysis() (string, error) {
	// Try analyze-phase/consolidated.md first
	path := filepath.Join(g.reportDir, "analyze-phase", "consolidated.md")
	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}

	// Fallback to consensus directory
	consensusPath := filepath.Join(g.reportDir, "analyze-phase", "consensus", "consolidated.md")
	content, err = os.ReadFile(consensusPath)
	if err != nil {
		return "", fmt.Errorf("consolidated analysis not found at %s or %s", path, consensusPath)
	}

	return string(content), nil
}

// readTaskFiles reads all task files from multiple locations.
// It checks: 1) {reportDir}/plan-phase/tasks/, 2) .quorum/tasks/ (global)
func (g *Generator) readTaskFiles() ([]TaskInfo, error) {
	var tasks []TaskInfo
	taskPattern := regexp.MustCompile(`^task-(\d+)-(.+)\.md$`)

	// Directories to search for task files
	tasksDirs := []string{
		filepath.Join(g.reportDir, "plan-phase", "tasks"),
	}

	// Also check global .quorum/tasks directory (derive from reportDir)
	// reportDir is typically .quorum/runs/{workflowID}, so .quorum/tasks is ../../../.quorum/tasks
	quorumRunsSegment := ".quorum" + string(filepath.Separator) + "runs" + string(filepath.Separator)
	if strings.Contains(g.reportDir, quorumRunsSegment) {
		quorumRoot := filepath.Dir(filepath.Dir(g.reportDir)) // Go up to .quorum
		globalTasksDir := filepath.Join(quorumRoot, "tasks")
		tasksDirs = append(tasksDirs, globalTasksDir)
	}

	seen := make(map[string]bool) // Deduplicate by task ID

	for _, tasksDir := range tasksDirs {
		entries, err := os.ReadDir(tasksDir)
		if err != nil {
			continue // Skip non-existent directories
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			matches := taskPattern.FindStringSubmatch(entry.Name())
			if matches == nil {
				continue
			}

			taskID := fmt.Sprintf("task-%s", matches[1])
			if seen[taskID] {
				continue // Skip duplicates
			}

			content, err := os.ReadFile(filepath.Join(tasksDir, entry.Name()))
			if err != nil {
				continue
			}

			task := g.parseTaskFile(matches[1], matches[2], string(content))
			tasks = append(tasks, task)
			seen[taskID] = true
		}
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task files found in any location")
	}

	// Sort by task ID
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// parseTaskFile extracts task information from file content.
func (g *Generator) parseTaskFile(num, slug, content string) TaskInfo {
	task := TaskInfo{
		ID:      fmt.Sprintf("task-%s", num),
		Name:    strings.ReplaceAll(slug, "-", " "),
		Content: content,
	}

	// Extract metadata from frontmatter
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "**Task ID**:") {
			task.ID = strings.TrimSpace(strings.TrimPrefix(line, "**Task ID**:"))
		} else if strings.HasPrefix(line, "**Assigned Agent**:") {
			task.Agent = strings.TrimSpace(strings.TrimPrefix(line, "**Assigned Agent**:"))
		} else if strings.HasPrefix(line, "**Complexity**:") {
			task.Complexity = strings.TrimSpace(strings.TrimPrefix(line, "**Complexity**:"))
		} else if strings.HasPrefix(line, "**Dependencies**:") {
			deps := strings.TrimSpace(strings.TrimPrefix(line, "**Dependencies**:"))
			if deps != "None" && deps != "" {
				task.Dependencies = strings.Split(deps, ", ")
			}
		}

		// Stop after frontmatter
		if line == "---" && task.Agent != "" {
			break
		}
	}

	// Extract title from first heading
	for _, line := range lines {
		if strings.HasPrefix(line, "# Task:") {
			task.Name = strings.TrimSpace(strings.TrimPrefix(line, "# Task:"))
			break
		} else if strings.HasPrefix(line, "# ") {
			task.Name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	return task
}

// formatTitle formats an issue title.
func (g *Generator) formatTitle(id, name string, isMain bool) string {
	format := g.config.Template.TitleFormat
	if format == "" {
		format = "[quorum] {task_name}"
	}

	if isMain {
		return strings.ReplaceAll(format, "{task_name}", "Workflow Summary")
	}

	title := format
	title = strings.ReplaceAll(title, "{task_name}", name)
	title = strings.ReplaceAll(title, "{task_id}", id)

	return title
}

// formatMainIssueBody formats the main issue body from consolidated analysis.
func (g *Generator) formatMainIssueBody(consolidated, workflowID string) string {
	var sb strings.Builder

	sb.WriteString("## Workflow Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Workflow ID**: `%s`\n\n", workflowID))
	sb.WriteString("---\n\n")

	// Include consolidated analysis (truncated if too long)
	if len(consolidated) > 50000 {
		consolidated = consolidated[:50000] + "\n\n*[Content truncated...]*"
	}
	sb.WriteString(consolidated)

	sb.WriteString("\n\n---\n\n")
	sb.WriteString("## Sub-Issues\n\n")
	sb.WriteString("*Sub-issues will be linked below as they are created.*\n")

	// Add footer
	sb.WriteString("\n\n---\n")
	sb.WriteString("*Generated by [Quorum AI](https://github.com/hugo-lorenzo-mato/quorum-ai)*\n")

	return sb.String()
}

// formatTaskIssueBody formats a task issue body.
func (g *Generator) formatTaskIssueBody(task TaskInfo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: %s\n\n", task.Name))
	sb.WriteString(fmt.Sprintf("**Task ID**: `%s`\n", task.ID))

	if task.Agent != "" {
		sb.WriteString(fmt.Sprintf("**Assigned Agent**: %s\n", task.Agent))
	}
	if task.Complexity != "" {
		sb.WriteString(fmt.Sprintf("**Complexity**: %s\n", task.Complexity))
	}
	if len(task.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("**Dependencies**: %s\n", strings.Join(task.Dependencies, ", ")))
	}

	sb.WriteString("\n---\n\n")

	// Include task content (extract relevant sections)
	content := g.extractTaskContent(task.Content)
	if len(content) > 40000 {
		content = content[:40000] + "\n\n*[Content truncated...]*"
	}
	sb.WriteString(content)

	// Add footer
	sb.WriteString("\n\n---\n")
	sb.WriteString("*Generated by [Quorum AI](https://github.com/hugo-lorenzo-mato/quorum-ai)*\n")

	return sb.String()
}

// extractTaskContent extracts relevant sections from task file content.
func (g *Generator) extractTaskContent(content string) string {
	// Skip the frontmatter, include from first section after ---
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[2])
	}

	// Fallback: skip first heading and metadata
	lines := strings.Split(content, "\n")
	var result []string
	inFrontmatter := true

	for _, line := range lines {
		if inFrontmatter {
			if strings.HasPrefix(line, "## ") {
				inFrontmatter = false
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// GetIssueSet retrieves an existing issue set for a workflow (if any).
func (g *Generator) GetIssueSet(_ context.Context, _ string) (*core.IssueSet, error) {
	// This would typically query the state store for persisted issue references
	// For now, return nil indicating no existing issues
	return nil, nil
}

// cleanIssuesDirectory removes only the draft/ subdirectory to prevent duplicates
// from accumulating across multiple generations. The published/ directory is preserved.
func (g *Generator) cleanIssuesDirectory(workflowID string) error {
	draftDir, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return fmt.Errorf(errResolvingDraftDir, err)
	}

	// Check if directory exists
	if _, err := os.Stat(draftDir); os.IsNotExist(err) {
		slog.Debug("draft directory does not exist, skipping cleanup", "dir", draftDir)
		return nil
	}

	// Remove only the draft directory, preserving published/
	if err := os.RemoveAll(draftDir); err != nil {
		return fmt.Errorf("removing draft directory: %w", err)
	}

	slog.Info("cleaned draft directory to prevent duplicates", "dir", draftDir)
	return nil
}

// =============================================================================
// LLM-based Issue Generation (Path-Based Approach)
// =============================================================================

// GenerateIssueFiles generates markdown files for all issues using AI.
// This uses a path-based approach where Claude reads source files and writes
// issue files directly to disk, avoiding embedding large content in the prompt.
// Files are saved to .quorum/issues/{workflowID}/ directory.
// Returns the list of generated file paths.
// maxTasksPerBatch is the maximum number of tasks to process in a single Claude CLI call.
// Claude CLI has context limits, and with large task files (~30-50KB each), 8-10 tasks
// is the practical maximum before hitting "Prompt is too long" errors.
const maxTasksPerBatch = 8

//nolint:gocyclo // Orchestrates prompt generation, file IO, and validations.
func (g *Generator) GenerateIssueFiles(ctx context.Context, workflowID string) ([]string, error) {
	if g.agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	agent, err := g.agents.Get(g.config.Generator.Agent)
	if err != nil {
		return nil, fmt.Errorf("getting agent %s: %w", g.config.Generator.Agent, err)
	}

	// Get prompt renderer
	prompts, err := g.getPromptRenderer()
	if err != nil {
		return nil, fmt.Errorf("getting prompt renderer: %w", err)
	}

	logger, closeLogger, err := g.openIssuesLogger(workflowID)
	if err != nil {
		slog.Warn("failed to open issues log file", "error", err)
	} else {
		defer func() {
			if closeErr := closeLogger(); closeErr != nil {
				slog.Warn("failed to close issues log file", "error", closeErr)
			}
		}()
	}

	// Create output directory
	root, err := g.getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf(errGettingProjectRoot, err)
	}

	// Clean the draft directory before generation to avoid duplicates
	if err := g.cleanIssuesDirectory(workflowID); err != nil {
		return nil, fmt.Errorf("cleaning draft directory: %w", err)
	}

	draftDirAbs, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return nil, fmt.Errorf(errResolvingDraftDir, err)
	}
	if err := os.MkdirAll(draftDirAbs, 0o755); err != nil {
		return nil, fmt.Errorf("creating draft directory: %w", err)
	}

	// Use relative path for shorter prompt (Claude will resolve it from cwd)
	baseDir := g.resolveIssuesBaseDir()
	issuesDirRel := filepath.Join(baseDir, workflowID, "draft")
	issuesDirAbs := draftDirAbs
	cwd := root

	// Get consolidated analysis path (not content)
	consolidatedPath, err := g.getConsolidatedAnalysisPath()
	if err != nil {
		return nil, fmt.Errorf("finding consolidated analysis: %w", err)
	}

	// Get task file paths (not content)
	taskFiles, err := g.getTaskFilePaths()
	if err != nil {
		return nil, fmt.Errorf("getting task file paths: %w", err)
	}

	expected := g.buildExpectedIssueFiles(consolidatedPath, taskFiles)
	expectedByName := make(map[string]expectedIssueFile, len(expected))
	tracker := NewGenerationTracker(workflowID)
	for _, exp := range expected {
		expectedByName[exp.FileName] = exp
		tracker.AddExpected(exp.FileName, exp.TaskID)
	}
	totalExpected := len(tracker.ExpectedFiles)
	emitted := make(map[string]bool, totalExpected)
	estimatedProgress := 0

	parseProgressIssue := func(fileName string) *ProgressIssue {
		filePath := filepath.Join(issuesDirAbs, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return &ProgressIssue{Title: fileName, FileName: fileName}
		}

		if fm, _, fmErr := parseDraftContent(string(content)); fmErr == nil && fm != nil {
			return &ProgressIssue{
				Title:       fm.Title,
				TaskID:      fm.TaskID,
				FileName:    fileName,
				IsMainIssue: fm.IsMainIssue,
			}
		}

		title, _ := parseIssueMarkdown(string(content))
		return &ProgressIssue{
			Title:       title,
			TaskID:      "",
			FileName:    fileName,
			IsMainIssue: strings.Contains(fileName, "consolidated") || strings.HasPrefix(fileName, "00-"),
		}
	}

	emitNewFiles := func() {
		names := make([]string, 0, len(tracker.GeneratedFiles))
		for name := range tracker.GeneratedFiles {
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool {
			return extractFileNumber(names[i]) < extractFileNumber(names[j])
		})
		for _, name := range names {
			if emitted[name] {
				continue
			}
			emitted[name] = true
			cur := len(emitted)
			if estimatedProgress > cur {
				cur = estimatedProgress
			}
			g.emitIssuesGenerationProgress(workflowID, "file_generated", cur, totalExpected, parseProgressIssue(name), "")
		}
		// Always emit an aggregate progress update (helps UIs that don't track per-file events).
		cur := len(emitted)
		if estimatedProgress > cur {
			cur = estimatedProgress
		}
		g.emitIssuesGenerationProgress(workflowID, "progress", cur, totalExpected, nil, "")
	}

	resilienceCfg := g.buildLLMResilienceConfig(g.config.Generator.Resilience, logger)
	executor := NewResilientLLMExecutor(agent, resilienceCfg)
	maxAttempts := 1
	if resilienceCfg.Enabled && resilienceCfg.MaxRetries > 0 {
		maxAttempts = resilienceCfg.MaxRetries + 1
	}

	slog.Info("starting AI issue generation",
		"agent", g.config.Generator.Agent,
		"model", g.config.Generator.Model,
		"output_dir", issuesDirRel,
		"total_tasks", len(taskFiles),
		"max_attempts", maxAttempts)

	g.emitIssuesGenerationProgress(workflowID, "started", 0, totalExpected, nil, "issue generation started")

	if logger != nil {
		deadline, hasDeadline := ctx.Deadline()
		logger.Info("issue generation started",
			"workflow_id", workflowID,
			"agent", g.config.Generator.Agent,
			"model", g.config.Generator.Model,
			"output_dir", issuesDirRel,
			"total_tasks", len(taskFiles),
			"max_attempts", maxAttempts,
			"has_deadline", hasDeadline,
			"deadline", deadline)
	}

	cfg := g.config.Template
	batchSize := maxTasksPerBatch
	missing := expected
	var batchErrors []error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if len(missing) == 0 {
			break
		}

		// Baseline estimate: everything not in "missing" is considered already generated.
		// This helps keep progress monotonic across retries.
		if totalExpected > 0 {
			estimatedProgress = totalExpected - len(missing)
			if estimatedProgress < 0 {
				estimatedProgress = 0
			}
		}

		var tasksToGenerate []service.IssueTaskFile
		includeMain := false
		for _, exp := range missing {
			if exp.IsMain {
				includeMain = true
				continue
			}
			if exp.Task != nil {
				tasksToGenerate = append(tasksToGenerate, *exp.Task)
			}
		}

		batches := g.splitIntoBatches(tasksToGenerate, batchSize)
		totalBatches := len(batches)

		if logger != nil {
			logger.Info("starting generation attempt",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"batch_size", batchSize,
				"tasks_to_generate", len(tasksToGenerate),
				"include_main", includeMain,
				"batches", totalBatches)
		}

		for batchNum, batch := range batches {
			var consolidatedForBatch string
			if includeMain && batchNum == 0 {
				consolidatedForBatch = consolidatedPath
			}

			params := service.IssueGenerateParams{
				ConsolidatedAnalysisPath: consolidatedForBatch,
				TaskFiles:                batch,
				IssuesDir:                issuesDirRel,
				Language:                 cfg.Language,
				Tone:                     cfg.Tone,
				Summarize:                g.config.Generator.Summarize,
				IncludeDiagrams:          cfg.IncludeDiagrams,
				IncludeTestingSection:    true,
				CustomInstructions:       cfg.CustomInstructions,
				Convention:               cfg.Convention,
			}

			prompt, err := prompts.RenderIssueGenerate(params)
			if err != nil {
				return nil, fmt.Errorf("rendering issue generation prompt (batch %d, attempt %d): %w", batchNum+1, attempt, err)
			}

			slog.Info("processing issue generation batch",
				"batch", batchNum+1,
				"total_batches", totalBatches,
				"attempt", attempt,
				"tasks_in_batch", len(batch),
				"prompt_size", len(prompt),
				"includes_consolidated", consolidatedForBatch != "")
			cur := len(emitted)
			if estimatedProgress > cur {
				cur = estimatedProgress
			}
			g.emitIssuesGenerationProgress(workflowID, "batch_started", cur, totalExpected, nil,
				fmt.Sprintf("batch %d/%d (attempt %d)", batchNum+1, totalBatches, attempt))

			if logger != nil {
				taskIDs := make([]string, 0, len(batch))
				for _, task := range batch {
					taskIDs = append(taskIDs, task.ID)
				}
				logger.Info("executing batch",
					"attempt", attempt,
					"batch", batchNum+1,
					"total_batches", totalBatches,
					"tasks", taskIDs,
					"prompt_size", len(prompt),
					"includes_consolidated", consolidatedForBatch != "")
			}

			deadline, hasDeadline := ctx.Deadline()
			timeout := resolveExecuteTimeout(deadline, hasDeadline, 10*time.Minute)

			result, err := executor.Execute(ctx, core.ExecuteOptions{
				Prompt:          prompt,
				Model:           g.config.Generator.Model,
				Format:          core.OutputFormatText,
				Timeout:         timeout,
				WorkDir:         cwd,
				ReasoningEffort: g.config.Generator.ReasoningEffort,
			})
			if err != nil {
				batchErrors = append(batchErrors, fmt.Errorf("batch %d/%d attempt %d: %w", batchNum+1, totalBatches, attempt, err))
				if logger != nil {
					logger.Error("batch failed",
						"attempt", attempt,
						"batch", batchNum+1,
						"error", err)
				}
				cur := len(emitted)
				if estimatedProgress > cur {
					cur = estimatedProgress
				}
				g.emitIssuesGenerationProgress(workflowID, "batch_failed", cur, totalExpected, nil,
					fmt.Sprintf("batch %d/%d failed (attempt %d)", batchNum+1, totalBatches, attempt))
				continue
			}

			slog.Info("batch completed",
				"batch", batchNum+1,
				"total_batches", totalBatches,
				"attempt", attempt,
				"output_length", len(result.Output))
			if logger != nil {
				logger.Info("batch completed",
					"attempt", attempt,
					"batch", batchNum+1,
					"output_length", len(result.Output))
			}
			// Update an estimated progress count so the UI can show incremental movement even before we scan the filesystem.
			estimatedProgress += len(batch)
			if includeMain && batchNum == 0 {
				estimatedProgress++
			}
			if estimatedProgress > totalExpected && totalExpected > 0 {
				estimatedProgress = totalExpected
			}
			g.emitIssuesGenerationProgress(workflowID, "batch_completed", estimatedProgress, totalExpected, nil,
				fmt.Sprintf("batch %d/%d completed (attempt %d)", batchNum+1, totalBatches, attempt))
		}

		if _, err := g.scanGeneratedIssueFilesWithTracker(issuesDirAbs, tracker); err != nil {
			return nil, fmt.Errorf("scanning generated issue files: %w", err)
		}
		emitNewFiles()

		missingNames := tracker.GetMissingFiles()
		sort.Strings(missingNames)
		if len(missingNames) == 0 {
			break
		}

		missing = missing[:0]
		for _, name := range missingNames {
			if exp, ok := expectedByName[name]; ok {
				missing = append(missing, exp)
			}
		}

		if logger != nil {
			logger.Warn("missing issue files after attempt",
				"attempt", attempt,
				"missing", missingNames)
		}

		if attempt < maxAttempts && batchSize > 1 {
			batchSize = (batchSize + 1) / 2
		}
	}

	files, err := g.scanGeneratedIssueFilesWithTracker(issuesDirAbs, tracker)
	if err != nil {
		return nil, fmt.Errorf("scanning generated issue files: %w", err)
	}
	emitNewFiles()

	if len(files) == 0 {
		return nil, fmt.Errorf("AI did not generate any issue files (no files found in %s)", issuesDirAbs)
	}

	if missingNames := tracker.GetMissingFiles(); len(missingNames) > 0 {
		sort.Strings(missingNames)
		errMsg := fmt.Sprintf("issue generation incomplete: missing %d file(s): %s", len(missingNames), strings.Join(missingNames, ", "))
		if len(batchErrors) > 0 {
			errMsg = fmt.Sprintf("%s; batch errors: %v", errMsg, batchErrors)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	slog.Info("AI issue generation completed", "total_files", len(files))
	if logger != nil {
		logger.Info("issue generation completed", "total_files", len(files))
	}

	g.emitIssuesGenerationProgress(workflowID, "completed", len(emitted), totalExpected, nil, "issue generation completed")
	return files, nil
}

// splitIntoBatches divides a slice of task files into batches of the specified size.
func (g *Generator) splitIntoBatches(tasks []service.IssueTaskFile, batchSize int) [][]service.IssueTaskFile {
	if batchSize <= 0 {
		batchSize = maxTasksPerBatch
	}

	var batches [][]service.IssueTaskFile
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		batches = append(batches, tasks[i:end])
	}

	// If no tasks, return single empty batch to still process consolidated analysis
	if len(batches) == 0 {
		batches = append(batches, []service.IssueTaskFile{})
	}

	return batches
}

// getConsolidatedAnalysisPath returns the RELATIVE path to the consolidated analysis file.
// Using relative paths keeps the prompt shorter and avoids Claude CLI prompt size limits.
func (g *Generator) getConsolidatedAnalysisPath() (string, error) {
	root, err := g.getProjectRoot()
	if err != nil {
		return "", fmt.Errorf(errGettingProjectRoot, err)
	}

	// Get relative reportDir (in case it's absolute)
	reportDirRel := g.reportDir
	if filepath.IsAbs(g.reportDir) {
		if rel, err := filepath.Rel(root, g.reportDir); err == nil {
			reportDirRel = rel
		}
	}

	// Try analyze-phase/consolidated.md first
	relPath := filepath.Join(reportDirRel, "analyze-phase", "consolidated.md")
	absPath := filepath.Join(root, relPath)
	if _, err := os.Stat(absPath); err == nil {
		return relPath, nil
	}

	// Fallback to consensus directory
	consensusRelPath := filepath.Join(reportDirRel, "analyze-phase", "consensus", "consolidated.md")
	absConsensusPath := filepath.Join(root, consensusRelPath)
	if _, err := os.Stat(absConsensusPath); err == nil {
		return consensusRelPath, nil
	}

	return "", fmt.Errorf("consolidated analysis not found at %s or %s", absPath, absConsensusPath)
}

// getTaskFilePaths returns information about task files (RELATIVE paths, not content).
// Using relative paths keeps the prompt shorter and avoids Claude CLI prompt size limits.
func (g *Generator) getTaskFilePaths() ([]service.IssueTaskFile, error) {
	root, err := g.getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf(errGettingProjectRoot, err)
	}

	// Get relative reportDir (in case it's absolute)
	reportDirRel := g.reportDir
	if filepath.IsAbs(g.reportDir) {
		if rel, err := filepath.Rel(root, g.reportDir); err == nil {
			reportDirRel = rel
		}
	}

	var taskFiles []service.IssueTaskFile
	taskPattern := regexp.MustCompile(`^task-(\d+)-(.+)\.md$`)

	// Directories to search for task files (relative paths)
	tasksDirs := []string{
		filepath.Join(reportDirRel, "plan-phase", "tasks"),
	}

	// Also check global .quorum/tasks directory (always relative)
	if strings.Contains(g.reportDir, ".quorum/runs/") || strings.Contains(reportDirRel, ".quorum/runs/") {
		// Global tasks directory is always .quorum/tasks (relative)
		globalTasksDir := ".quorum/tasks"
		tasksDirs = append(tasksDirs, globalTasksDir)
	}

	seen := make(map[string]bool)

	for _, tasksDir := range tasksDirs {
		absTasksDir := tasksDir
		if !filepath.IsAbs(tasksDir) {
			absTasksDir = filepath.Join(root, tasksDir)
		}

		entries, err := os.ReadDir(absTasksDir)
		if err != nil {
			continue // Skip non-existent directories
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			matches := taskPattern.FindStringSubmatch(entry.Name())
			if matches == nil {
				continue
			}

			taskID := fmt.Sprintf("task-%s", matches[1])
			if seen[taskID] {
				continue
			}

			// Use RELATIVE path for shorter prompt
			relPath := filepath.Join(tasksDir, entry.Name())
			taskName := strings.ReplaceAll(matches[2], "-", " ")
			slug := matches[2] // Already kebab-case

			taskFiles = append(taskFiles, service.IssueTaskFile{
				Path: relPath,
				ID:   taskID,
				Name: taskName,
				Slug: slug,
			})
			seen[taskID] = true
		}
	}

	if len(taskFiles) == 0 {
		return nil, fmt.Errorf("no task files found in any location")
	}

	// Sort by numeric task ID (task-1, task-2, task-10, ...)
	sort.Slice(taskFiles, func(i, j int) bool {
		return extractTaskNumber(taskFiles[i].ID) < extractTaskNumber(taskFiles[j].ID)
	})

	// Assign global 1-based index for deterministic file naming
	for i := range taskFiles {
		taskFiles[i].Index = i + 1
	}

	return taskFiles, nil
}

// scanGeneratedIssueFiles scans the issues directory for generated markdown files.
// If a tracker is provided, it uses start-time based validation instead of a fixed window.
//
//nolint:unused // Reserved for future refactors that scan without tracker.
func (g *Generator) scanGeneratedIssueFiles(issuesDir string) ([]string, error) {
	return g.scanGeneratedIssueFilesWithTracker(issuesDir, nil)
}

// scanGeneratedIssueFilesWithTracker scans with an optional tracker for better accuracy.
func (g *Generator) scanGeneratedIssueFilesWithTracker(issuesDir string, tracker *GenerationTracker) ([]string, error) {
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist yet
		}
		return nil, fmt.Errorf("reading issues directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(issuesDir, entry.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			slog.Warn("skipping empty issue file", "file", entry.Name(), "path", filePath)
			continue
		}

		// Use tracker if available for more accurate detection
		if tracker != nil {
			if tracker.IsValidFile(entry.Name(), info.ModTime()) {
				files = append(files, filePath)
				tracker.MarkGenerated(entry.Name(), info.ModTime())
			}
		} else {
			// Fallback: accept any .md file modified in the last 30 minutes
			// (increased from 10 minutes to handle longer batch processing)
			if time.Since(info.ModTime()) < 30*time.Minute {
				files = append(files, filePath)
			}
		}
	}

	// Log any missing expected files
	if tracker != nil {
		missing := tracker.GetMissingFiles()
		if len(missing) > 0 {
			slog.Warn("expected files not found in generation output",
				"missing", missing,
				"workflow_id", tracker.WorkflowID)
		}
	}

	// Sort files numerically
	sort.Slice(files, func(i, j int) bool {
		return extractFileNumber(filepath.Base(files[i])) < extractFileNumber(filepath.Base(files[j]))
	})

	return files, nil
}

// parseAndWriteIssueFiles parses the AI output and writes markdown files.
// Expected format: <!-- FILE: filename.md --> followed by content until the next marker.
//
//nolint:unused // Reserved for alternative parsing mode.
func (g *Generator) parseAndWriteIssueFiles(output, issuesDir string) ([]string, error) {
	var files []string

	// Pattern to match file markers: <!-- FILE: filename.md -->
	fileMarkerRe := regexp.MustCompile(`(?m)^<!--\s*FILE:\s*(\S+\.md)\s*-->`)
	matches := fileMarkerRe.FindAllStringSubmatchIndex(output, -1)

	if len(matches) == 0 {
		slog.Warn("no file markers found in AI output, attempting alternative parsing")
		// Try alternative format: code blocks with filenames
		return g.parseCodeBlockFiles(output, issuesDir)
	}

	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		filename := output[match[2]:match[3]]
		contentStart := match[1] // End of the marker

		// Content ends at the next marker or end of output
		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(output)
		}

		content := strings.TrimSpace(output[contentStart:contentEnd])

		// Skip empty content
		if content == "" {
			continue
		}

		// SECURITY: Validate the output path to prevent path traversal attacks
		filePath, err := ValidateOutputPath(issuesDir, filename)
		if err != nil {
			slog.Error("path validation failed, skipping file",
				"filename", filename,
				"error", err)
			continue
		}

		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			slog.Warn("failed to write issue file", "path", filePath, "error", err)
			continue
		}

		files = append(files, filePath)
		slog.Debug("wrote issue file", "path", filePath, "size", len(content))
	}

	// Sort files numerically
	sort.Slice(files, func(i, j int) bool {
		return extractFileNumber(filepath.Base(files[i])) < extractFileNumber(filepath.Base(files[j]))
	})

	return files, nil
}

// parseCodeBlockFiles attempts to parse files from code blocks with filename headers.
// Format: ### filename.md followed by a code block
//
//nolint:unused // Reserved for alternative parsing mode.
func (g *Generator) parseCodeBlockFiles(output, issuesDir string) ([]string, error) {
	var files []string

	// Pattern: ### XX-filename.md or #### XX-filename.md followed by ```markdown or ```
	headerRe := regexp.MustCompile(`(?m)^#{2,4}\s*(\d+-[^\n]+\.md)\s*\n`)
	codeBlockRe := regexp.MustCompile("```(?:markdown)?\\n([\\s\\S]*?)```")

	headerMatches := headerRe.FindAllStringSubmatchIndex(output, -1)

	for i, match := range headerMatches {
		if len(match) < 4 {
			continue
		}

		filename := output[match[2]:match[3]]

		// Find content after the header (look for code block)
		searchStart := match[1]
		var searchEnd int
		if i+1 < len(headerMatches) {
			searchEnd = headerMatches[i+1][0]
		} else {
			searchEnd = len(output)
		}

		section := output[searchStart:searchEnd]
		codeMatch := codeBlockRe.FindStringSubmatch(section)

		var content string
		if codeMatch != nil && len(codeMatch) > 1 {
			content = strings.TrimSpace(codeMatch[1])
		} else {
			// No code block, use the section content directly
			content = strings.TrimSpace(section)
		}

		if content == "" {
			continue
		}

		// SECURITY: Validate the output path to prevent path traversal attacks
		filePath, err := ValidateOutputPath(issuesDir, filename)
		if err != nil {
			slog.Error("path validation failed, skipping file",
				"filename", filename,
				"error", err)
			continue
		}

		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			slog.Warn("failed to write issue file", "path", filePath, "error", err)
			continue
		}

		files = append(files, filePath)
	}

	// Sort files numerically
	sort.Slice(files, func(i, j int) bool {
		return extractFileNumber(filepath.Base(files[i])) < extractFileNumber(filepath.Base(files[j]))
	})

	return files, nil
}

// extractFileNumber extracts the leading number from a filename (e.g., "01-title.md" -> 1)
func extractFileNumber(name string) int {
	re := regexp.MustCompile(`^(\d+)`)
	match := re.FindStringSubmatch(name)
	if len(match) > 1 {
		num, _ := strconv.Atoi(match[1])
		return num
	}
	return 9999
}

// buildMasterPrompt creates the prompt for AI to generate all issue markdown files.
//
//nolint:unused // Reserved for future prompt consolidation.
func (g *Generator) buildMasterPrompt(consolidated string, tasks []TaskInfo, _, outputDir string) string {
	cfg := g.config.Template

	var sb strings.Builder

	// Main instruction
	sb.WriteString("# Task: Generate GitHub Issue Markdown Files\n\n")
	sb.WriteString("You must create markdown files for GitHub/GitLab issues based on the provided analysis.\n\n")

	// Output specification
	sb.WriteString("## Output Requirements\n\n")
	sb.WriteString(fmt.Sprintf("Create markdown files in the directory: `%s`\n\n", outputDir))
	sb.WriteString("**File naming convention:**\n")
	sb.WriteString("- `00-consolidated-analysis.md` - Issue from the consolidated analysis\n")
	for i, task := range tasks {
		safeName := sanitizeFilename(task.Name)
		sb.WriteString(fmt.Sprintf("- `%02d-%s.md` - Issue for %s\n", i+1, safeName, task.ID))
	}
	sb.WriteString("\n")

	// Content rules
	sb.WriteString("## Content Rules (MUST follow)\n\n")
	sb.WriteString("1. **All issues are SUB-ISSUES** - Do NOT create parent/epic issues\n")
	sb.WriteString("2. **Clean content only** - Do NOT include:\n")
	sb.WriteString("   - Model names or versions\n")
	sb.WriteString("   - Agent names (Claude, Gemini, etc.)\n")
	sb.WriteString("   - Generation timestamps\n")
	sb.WriteString("   - Internal IDs or workflow IDs\n")
	sb.WriteString("   - Technical metadata about the generation process\n")
	sb.WriteString("3. **Each file must have:**\n")
	sb.WriteString("   - First line: `# Issue Title` (will be used as GitHub issue title)\n")
	sb.WriteString("   - Body: Clean markdown suitable for GitHub issues\n")
	sb.WriteString("4. **Focus on actionable content:**\n")
	sb.WriteString("   - What needs to be done\n")
	sb.WriteString("   - Acceptance criteria\n")
	sb.WriteString("   - Technical details relevant to implementation\n")
	sb.WriteString("\n")

	// Language and tone
	sb.WriteString("## Style Requirements\n\n")
	switch cfg.Language {
	case "spanish":
		sb.WriteString("- Write ALL content in **Spanish**\n")
	case "french":
		sb.WriteString("- Write ALL content in **French**\n")
	case "german":
		sb.WriteString("- Write ALL content in **German**\n")
	case "portuguese":
		sb.WriteString("- Write ALL content in **Portuguese**\n")
	default:
		sb.WriteString("- Write in **English**\n")
	}

	switch cfg.Tone {
	case "technical":
		sb.WriteString("- Use technical, detailed tone with precise terminology\n")
	case "concise":
		sb.WriteString("- Be extremely concise, use bullet points\n")
	case "casual":
		sb.WriteString("- Use casual, friendly tone\n")
	default:
		sb.WriteString("- Use professional tone\n")
	}

	if cfg.IncludeDiagrams {
		sb.WriteString("- Include Mermaid diagrams where helpful for architecture/flow\n")
	}

	if g.config.Generator.Summarize {
		sb.WriteString("- Summarize content concisely, keeping key technical details\n")
	}

	// Custom instructions from user (highest priority)
	if cfg.CustomInstructions != "" {
		sb.WriteString(fmt.Sprintf("\n## Custom Instructions (HIGH PRIORITY)\n\n%s\n", cfg.CustomInstructions))
	}

	// Convention
	if cfg.Convention != "" {
		sb.WriteString(fmt.Sprintf("\n## Convention to Follow\n\n%s\n", cfg.Convention))
	}

	// Source content
	sb.WriteString("\n---\n\n")
	sb.WriteString("# Source Content\n\n")

	// Consolidated analysis
	sb.WriteString("## Consolidated Analysis\n\n")
	if len(consolidated) > 30000 {
		consolidated = consolidated[:30000] + "\n\n*[Truncated...]*"
	}
	sb.WriteString("```markdown\n")
	sb.WriteString(consolidated)
	sb.WriteString("\n```\n\n")

	// Task files
	sb.WriteString("## Tasks\n\n")
	for _, task := range tasks {
		sb.WriteString(fmt.Sprintf("### %s: %s\n\n", task.ID, task.Name))
		content := task.Content
		if len(content) > 15000 {
			content = content[:15000] + "\n\n*[Truncated...]*"
		}
		sb.WriteString("```markdown\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}

	// Output format specification
	sb.WriteString("\n---\n\n")
	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Output each file using this EXACT format with file markers:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("<!-- FILE: 00-consolidated-analysis.md -->\n")
	sb.WriteString("# Issue Title Here\n\n")
	sb.WriteString("Issue body content...\n\n")
	sb.WriteString("<!-- FILE: 01-task-name.md -->\n")
	sb.WriteString("# Another Issue Title\n\n")
	sb.WriteString("Issue body content...\n")
	sb.WriteString("```\n\n")
	sb.WriteString("**IMPORTANT:** Each file MUST start with `<!-- FILE: filename.md -->` marker on its own line.\n")
	sb.WriteString("The first line AFTER the marker should be the issue title as an H1 heading (`# Title`).\n\n")

	// Final instruction
	sb.WriteString("**Now generate all the markdown content.**\n")
	sb.WriteString("Start with `00-consolidated-analysis.md`, then create one file per task.\n")
	sb.WriteString("Output the complete content - do not truncate or summarize excessively.\n")

	return sb.String()
}

// sanitizeFilename converts a string to a safe filename
func sanitizeFilename(name string) string {
	// Convert to lowercase, replace spaces with dashes
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	// Remove unsafe characters
	re := regexp.MustCompile(`[^a-z0-9\-]`)
	name = re.ReplaceAllString(name, "")

	// Collapse multiple dashes
	re = regexp.MustCompile(`-+`)
	name = re.ReplaceAllString(name, "-")

	// Trim dashes from ends
	name = strings.Trim(name, "-")

	// Limit length
	if len(name) > 50 {
		name = name[:50]
	}

	if name == "" {
		name = "issue"
	}

	return name
}

// ReadGeneratedIssues reads the generated issue markdown files and returns IssuePreview objects.
// It first looks in the draft/ subdirectory; if empty, falls back to the base directory for backward compat.
// Deduplicates by task ID to prevent duplicate issues.
func (g *Generator) ReadGeneratedIssues(workflowID string) ([]IssuePreview, error) {
	if err := ValidateWorkflowID(workflowID); err != nil {
		return nil, err
	}

	// Try reading from draft/ subdirectory first (new layout)
	previews, err := g.ReadAllDrafts(workflowID)
	if err != nil {
		return nil, err
	}
	if len(previews) > 0 {
		slog.Info("read generated issues from draft directory",
			"unique_issues", len(previews))
		return previews, nil
	}

	// Fallback: read from the flat base directory (backward compat with old layout)
	root, err := g.getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf(errResolvingProjectRoot, err)
	}
	baseDir := g.resolveIssuesBaseDir()
	issuesDir := filepath.Join(root, baseDir, workflowID)
	if err := validatePathUnderRoot(issuesDir, root); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No issues generated yet
		}
		return nil, fmt.Errorf("reading issues directory: %w", err)
	}

	seen := make(map[string]bool) // Track by taskID to deduplicate
	taskIndexMap := make(map[int]string)
	if taskFiles, err := g.getTaskFilePaths(); err == nil {
		for _, task := range taskFiles {
			taskIndexMap[task.Index] = task.ID
		}
	}

	// Sort entries numerically
	var files []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return extractFileNumber(files[i].Name()) < extractFileNumber(files[j].Name())
	})

	for _, entry := range files {
		filePath := filepath.Join(issuesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Try parsing frontmatter first
		fm, fmBody, fmErr := parseDraftContent(string(content))
		if fmErr == nil && fm != nil {
			if fm.TaskID != "" && seen[fm.TaskID] {
				continue
			}
			if fm.TaskID != "" {
				seen[fm.TaskID] = true
			}
			previews = append(previews, IssuePreview{
				Title:       fm.Title,
				Body:        fmBody,
				Labels:      fm.Labels,
				Assignees:   fm.Assignees,
				IsMainIssue: fm.IsMainIssue,
				TaskID:      fm.TaskID,
				FilePath:    filepath.Join(issuesDir, entry.Name()),
			})
			continue
		}

		// Fallback: plain markdown parsing
		title, body := parseIssueMarkdown(string(content))

		// Determine if it's the consolidated analysis (task_id will be empty)
		taskID := ""
		isMainIssue := false
		if strings.Contains(entry.Name(), "consolidated") || strings.HasPrefix(entry.Name(), "00-") {
			isMainIssue = true
			taskID = "main"
		} else {
			// Extract task ID from filename pattern: XX-task-name.md or issue-N-task-name.md
			re := regexp.MustCompile(`^(\d+)-(.+)\.md$`)
			if match := re.FindStringSubmatch(entry.Name()); len(match) > 1 {
				taskNum := match[1]
				if taskNum != "00" {
					if num, err := strconv.Atoi(taskNum); err == nil {
						if mapped, ok := taskIndexMap[num]; ok {
							taskID = mapped
						} else {
							taskID = fmt.Sprintf("task-%d", num)
						}
					} else {
						taskID = "task-" + taskNum
					}
				}
			} else {
				re = regexp.MustCompile(`issue-(?:task-)?(\d+)`)
				if match := re.FindStringSubmatch(entry.Name()); len(match) > 1 {
					if num, err := strconv.Atoi(match[1]); err == nil {
						if mapped, ok := taskIndexMap[num]; ok {
							taskID = mapped
						} else {
							taskID = fmt.Sprintf("task-%d", num)
						}
					} else {
						taskID = "task-" + match[1]
					}
				}
			}
		}

		// Deduplicate: skip if we've already seen this task ID
		if taskID != "" && seen[taskID] {
			slog.Warn("duplicate issue file detected, skipping",
				"file", entry.Name(),
				"task_id", taskID)
			continue
		}
		if taskID != "" {
			seen[taskID] = true
		}

		previews = append(previews, IssuePreview{
			Title:       title,
			Body:        body,
			Labels:      g.config.Labels,
			Assignees:   g.config.Assignees,
			IsMainIssue: isMainIssue,
			TaskID:      taskID,
			FilePath:    filepath.Join(issuesDir, entry.Name()),
		})
	}

	slog.Info("read generated issues",
		"total_files", len(files),
		"unique_issues", len(previews),
		"duplicates_skipped", len(files)-len(previews))

	return previews, nil
}

// parseIssueMarkdown extracts title and body from a markdown file.
// Title is the first H1 heading, body is everything after.
func parseIssueMarkdown(content string) (title, body string) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimPrefix(line, "# ")
			title = strings.TrimSpace(title)
			// Body is everything after the title line
			if i+1 < len(lines) {
				body = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			}
			return title, body
		}
	}

	// No H1 found, use filename as title and full content as body
	return "Untitled Issue", strings.TrimSpace(content)
}

// generateWithLLM generates issue content using LLM by delegating to GenerateIssueFiles.
// It caches results to avoid regenerating files for each task in the same workflow.
// Returns title and body for the requested issue (main or specific task).
func (g *Generator) generateWithLLM(ctx context.Context, _, taskID, workflowID string, isMain bool) (title, body string, err error) {
	// Check if we already have cached results for this workflow
	if cachedIssues, ok := g.llmGenerationCache[workflowID]; ok {
		return g.findIssueInCache(cachedIssues, taskID, isMain)
	}

	// Verify we have agent registry
	if g.agents == nil {
		return "", "", fmt.Errorf("LLM generation requires agent registry")
	}

	// Generate all issue files using the path-based approach
	slog.Info("generating all issue files with LLM",
		"workflow_id", workflowID,
		"agent", g.config.Generator.Agent)

	files, err := g.GenerateIssueFiles(ctx, workflowID)
	if err != nil {
		return "", "", fmt.Errorf("LLM generation failed: %w", err)
	}

	if len(files) == 0 {
		return "", "", fmt.Errorf("LLM generated no issue files")
	}

	// Read all generated files into cache
	issues, err := g.ReadGeneratedIssues(workflowID)
	if err != nil {
		return "", "", fmt.Errorf("reading generated issues: %w", err)
	}

	// Store in cache for subsequent calls
	g.llmGenerationCache[workflowID] = issues

	slog.Info("cached LLM-generated issues",
		"workflow_id", workflowID,
		"count", len(issues))

	// Find and return the requested issue
	return g.findIssueInCache(issues, taskID, isMain)
}

// findIssueInCache searches the cached issues for a specific task or main issue.
func (g *Generator) findIssueInCache(issues []IssuePreview, taskID string, isMain bool) (title, body string, err error) {
	for _, issue := range issues {
		if isMain && issue.IsMainIssue {
			return issue.Title, issue.Body, nil
		}
		if !isMain && issue.TaskID == taskID {
			return issue.Title, issue.Body, nil
		}
		// Fallback: match by partial taskID (e.g., "task-1" matches "task-1-something")
		if !isMain && taskID != "" && strings.Contains(issue.TaskID, taskID) {
			return issue.Title, issue.Body, nil
		}
	}

	// If not found, return error (will trigger fallback to direct copy)
	if isMain {
		return "", "", fmt.Errorf("main issue not found in LLM output")
	}
	return "", "", fmt.Errorf("task %s not found in LLM output", taskID)
}
