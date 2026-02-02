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

// Generator creates GitHub/GitLab issues from workflow artifacts.
type Generator struct {
	client    core.IssueClient
	config    config.IssuesConfig
	reportDir string
	agents    core.AgentRegistry          // Optional: for LLM-based generation
	prompts   *service.PromptRenderer     // Lazy-initialized prompt renderer
}

// NewGenerator creates a new issue generator.
// agents can be nil if LLM-based generation is not needed.
func NewGenerator(client core.IssueClient, cfg config.IssuesConfig, reportDir string, agents core.AgentRegistry) *Generator {
	return &Generator{
		client:    client,
		config:    cfg,
		reportDir: reportDir,
		agents:    agents,
	}
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
					Labels:      labels,
					Assignees:   assignees,
					IsMainIssue: true,
				})
			} else {
				mainIssue, err = g.client.CreateIssue(ctx, core.CreateIssueOptions{
					Title:     mainTitle,
					Body:      mainBody,
					Labels:    labels,
					Assignees: assignees,
				})
				if err != nil {
					return nil, fmt.Errorf("creating main issue: %w", err)
				}
				result.IssueSet.MainIssue = mainIssue
			}
		}
	}

	// Read and create sub-issues from task files
	if opts.CreateSubIssues {
		tasks, err := g.readTaskFiles()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("reading task files: %w", err))
		}

		for _, task := range tasks {
			var taskTitle, taskBody string
			var taskAISucceeded bool

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
			}
		}
	}

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
	if strings.Contains(g.reportDir, ".quorum/runs/") {
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
		if line == "---" && len(task.Agent) > 0 {
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
func (g *Generator) GetIssueSet(ctx context.Context, workflowID string) (*core.IssueSet, error) {
	// This would typically query the state store for persisted issue references
	// For now, return nil indicating no existing issues
	return nil, nil
}

// =============================================================================
// LLM-based Issue Generation (Path-Based Approach)
// =============================================================================

// GenerateIssueFiles generates markdown files for all issues using AI.
// This uses a path-based approach where Claude reads source files and writes
// issue files directly to disk, avoiding embedding large content in the prompt.
// Files are saved to .quorum/issues/{workflowID}/ directory.
// Returns the list of generated file paths.
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

	// Create output directory with absolute path
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	issuesDir := filepath.Join(cwd, ".quorum", "issues", workflowID)
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating issues directory: %w", err)
	}

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

	// Build template parameters
	cfg := g.config.Template
	params := service.IssueGenerateParams{
		ConsolidatedAnalysisPath: consolidatedPath,
		TaskFiles:                taskFiles,
		IssuesDir:                issuesDir,
		Language:                 cfg.Language,
		Tone:                     cfg.Tone,
		Summarize:                g.config.Generator.Summarize,
		IncludeDiagrams:          cfg.IncludeDiagrams,
		IncludeTestingSection:    true, // Always include testing section
		CustomInstructions:       cfg.CustomInstructions,
		Convention:               cfg.Convention,
	}

	// Render the prompt using the template
	prompt, err := prompts.RenderIssueGenerate(params)
	if err != nil {
		return nil, fmt.Errorf("rendering issue generation prompt: %w", err)
	}

	// Debug: Log prompt size and first/last 200 chars
	slog.Info("rendered issue generation prompt",
		"prompt_size_bytes", len(prompt),
		"consolidated_path", consolidatedPath,
		"task_count", len(taskFiles),
		"issues_dir", issuesDir,
	)
	if len(prompt) > 400 {
		slog.Debug("prompt preview",
			"first_200", prompt[:200],
			"last_200", prompt[len(prompt)-200:],
		)
	}

	slog.Info("starting AI issue generation",
		"agent", g.config.Generator.Agent,
		"model", g.config.Generator.Model,
		"output_dir", issuesDir,
		"task_count", len(taskFiles),
		"prompt_size", len(prompt))

	// Execute the agent - Claude will read source files and write issue files directly
	result, err := agent.Execute(ctx, core.ExecuteOptions{
		Prompt:  prompt,
		Model:   g.config.Generator.Model,
		Format:  core.OutputFormatText,
		Timeout: 10 * time.Minute,
		Sandbox: false,
		WorkDir: cwd, // Work from project root so paths resolve correctly
	})
	if err != nil {
		return nil, fmt.Errorf("executing agent: %w", err)
	}

	slog.Info("AI issue generation completed", "output_length", len(result.Output))

	// Scan the filesystem for generated issue files
	// (Claude wrote them directly, so we just need to find them)
	files, err := g.scanGeneratedIssueFiles(issuesDir)
	if err != nil {
		return nil, fmt.Errorf("scanning generated issue files: %w", err)
	}

	if len(files) == 0 {
		// Fallback: try the legacy parsing approach in case Claude didn't use Write tool
		slog.Warn("no files found in issues directory, trying output parsing fallback")
		files, err = g.parseAndWriteIssueFiles(result.Output, issuesDir)
		if err != nil {
			return nil, fmt.Errorf("parsing AI output (fallback): %w", err)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("AI did not generate any issue files")
	}

	slog.Info("found generated issue files", "count", len(files))

	return files, nil
}

// getConsolidatedAnalysisPath returns the absolute path to the consolidated analysis file.
func (g *Generator) getConsolidatedAnalysisPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	// Try analyze-phase/consolidated.md first
	path := filepath.Join(g.reportDir, "analyze-phase", "consolidated.md")
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(cwd, path)
	}
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}

	// Fallback to consensus directory
	consensusPath := filepath.Join(g.reportDir, "analyze-phase", "consensus", "consolidated.md")
	absConsensusPath := consensusPath
	if !filepath.IsAbs(consensusPath) {
		absConsensusPath = filepath.Join(cwd, consensusPath)
	}
	if _, err := os.Stat(absConsensusPath); err == nil {
		return absConsensusPath, nil
	}

	return "", fmt.Errorf("consolidated analysis not found at %s or %s", absPath, absConsensusPath)
}

// getTaskFilePaths returns information about task files (paths, not content).
func (g *Generator) getTaskFilePaths() ([]service.IssueTaskFile, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	var taskFiles []service.IssueTaskFile
	taskPattern := regexp.MustCompile(`^task-(\d+)-(.+)\.md$`)

	// Directories to search for task files
	tasksDirs := []string{
		filepath.Join(g.reportDir, "plan-phase", "tasks"),
	}

	// Also check global .quorum/tasks directory
	if strings.Contains(g.reportDir, ".quorum/runs/") {
		quorumRoot := filepath.Dir(filepath.Dir(g.reportDir))
		globalTasksDir := filepath.Join(quorumRoot, "tasks")
		tasksDirs = append(tasksDirs, globalTasksDir)
	}

	seen := make(map[string]bool)

	for _, tasksDir := range tasksDirs {
		absTasksDir := tasksDir
		if !filepath.IsAbs(tasksDir) {
			absTasksDir = filepath.Join(cwd, tasksDir)
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

			absPath := filepath.Join(absTasksDir, entry.Name())
			taskName := strings.ReplaceAll(matches[2], "-", " ")
			slug := matches[2] // Already kebab-case

			taskFiles = append(taskFiles, service.IssueTaskFile{
				Path: absPath,
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

	// Sort by task ID
	sort.Slice(taskFiles, func(i, j int) bool {
		return taskFiles[i].ID < taskFiles[j].ID
	})

	return taskFiles, nil
}

// scanGeneratedIssueFiles scans the issues directory for generated markdown files.
func (g *Generator) scanGeneratedIssueFiles(issuesDir string) ([]string, error) {
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

		// Only consider recently created/modified files (within last 10 minutes)
		if time.Since(info.ModTime()) < 10*time.Minute {
			files = append(files, filePath)
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
func (g *Generator) parseAndWriteIssueFiles(output, issuesDir string) ([]string, error) {
	var files []string

	// Pattern to match file markers: <!-- FILE: filename.md -->
	fileMarkerRe := regexp.MustCompile(`(?m)^<!--\s*FILE:\s*([^\s]+\.md)\s*-->`)
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

		// Write the file
		filePath := filepath.Join(issuesDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
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

		// Write the file
		filePath := filepath.Join(issuesDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
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
func (g *Generator) buildMasterPrompt(consolidated string, tasks []TaskInfo, workflowID, outputDir string) string {
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
func (g *Generator) ReadGeneratedIssues(workflowID string) ([]IssuePreview, error) {
	issuesDir := filepath.Join(".quorum", "issues", workflowID)

	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No issues generated yet
		}
		return nil, fmt.Errorf("reading issues directory: %w", err)
	}

	var previews []IssuePreview

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

		title, body := parseIssueMarkdown(string(content))

		// Determine if it's the consolidated analysis (task_id will be empty)
		taskID := ""
		if !strings.Contains(entry.Name(), "consolidated") {
			// Extract task ID from filename pattern: XX-task-name.md
			re := regexp.MustCompile(`^\d+-(.+)\.md$`)
			if match := re.FindStringSubmatch(entry.Name()); len(match) > 1 {
				taskID = "task-" + strings.Split(match[1], "-")[0]
			}
		}

		previews = append(previews, IssuePreview{
			Title:       title,
			Body:        body,
			Labels:      g.config.Labels,
			Assignees:   g.config.Assignees,
			IsMainIssue: false, // All are sub-issues per user request
			TaskID:      taskID,
		})
	}

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

// generateWithLLM is kept for backwards compatibility but redirects to file-based approach.
// Returns empty strings to trigger fallback (the new approach is GenerateIssueFiles).
func (g *Generator) generateWithLLM(ctx context.Context, content, taskID, workflowID string, isMain bool) (title, body string, err error) {
	// This method is deprecated - use GenerateIssueFiles instead
	return "", "", fmt.Errorf("deprecated: use GenerateIssueFiles for AI-based generation")
}
