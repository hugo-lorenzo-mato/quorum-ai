package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Generator creates GitHub/GitLab issues from workflow artifacts.
type Generator struct {
	client    core.IssueClient
	config    config.IssuesConfig
	reportDir string
	agents    core.AgentRegistry // Optional: for LLM-based generation
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

			// Try LLM-based generation if enabled
			if g.config.Generator.Enabled {
				mainTitle, mainBody, err = g.generateWithLLM(ctx, consolidatedContent, "", opts.WorkflowID, true)
				if err != nil {
					slog.Warn("LLM generation failed, falling back to direct copy",
						"error", err)
				}
			}

			// Fallback to direct copy if LLM failed or disabled
			if mainTitle == "" || mainBody == "" {
				mainTitle = g.formatTitle(opts.WorkflowID, "", true)
				mainBody = g.formatMainIssueBody(consolidatedContent, opts.WorkflowID)
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

			// Try LLM-based generation if enabled
			if g.config.Generator.Enabled {
				taskTitle, taskBody, err = g.generateWithLLM(ctx, task.Content, task.ID, opts.WorkflowID, false)
				if err != nil {
					slog.Warn("LLM generation failed for task, falling back to direct copy",
						"task_id", task.ID,
						"error", err)
				}
			}

			// Fallback to direct copy if LLM failed or disabled
			if taskTitle == "" || taskBody == "" {
				taskTitle = g.formatTitle(task.ID, task.Name, false)
				taskBody = g.formatTaskIssueBody(task)
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

// readTaskFiles reads all task files from the plan phase.
func (g *Generator) readTaskFiles() ([]TaskInfo, error) {
	tasksDir := filepath.Join(g.reportDir, "plan-phase", "tasks")

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("reading tasks directory: %w", err)
	}

	var tasks []TaskInfo
	taskPattern := regexp.MustCompile(`^task-(\d+)-(.+)\.md$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := taskPattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		content, err := os.ReadFile(filepath.Join(tasksDir, entry.Name()))
		if err != nil {
			continue
		}

		task := g.parseTaskFile(matches[1], matches[2], string(content))
		tasks = append(tasks, task)
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
// LLM-based Issue Generation
// =============================================================================

// generateWithLLM uses an LLM agent to generate issue title and body.
// Returns empty strings if LLM is not available or fails (caller should fallback).
func (g *Generator) generateWithLLM(ctx context.Context, content, taskID, workflowID string, isMain bool) (title, body string, err error) {
	if g.agents == nil {
		return "", "", fmt.Errorf("agent registry not available")
	}

	agent, err := g.agents.Get(g.config.Generator.Agent)
	if err != nil {
		return "", "", fmt.Errorf("getting agent %s: %w", g.config.Generator.Agent, err)
	}

	prompt := g.buildGeneratorPrompt(content, taskID, workflowID, isMain)

	result, err := agent.Execute(ctx, core.ExecuteOptions{
		Prompt:  prompt,
		Model:   g.config.Generator.Model,
		Format:  core.OutputFormatJSON,
		Timeout: 2 * time.Minute,
		Sandbox: true,
	})
	if err != nil {
		return "", "", fmt.Errorf("executing agent: %w", err)
	}

	title, body, err = g.parseGeneratorResponse(result.Output)
	if err != nil {
		return "", "", fmt.Errorf("parsing response: %w", err)
	}

	return title, body, nil
}

// generatorResponse is the expected JSON response from the LLM.
type generatorResponse struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// parseGeneratorResponse extracts title and body from LLM JSON output.
func (g *Generator) parseGeneratorResponse(output string) (title, body string, err error) {
	// Clean up output - LLM might include markdown code blocks
	output = strings.TrimSpace(output)
	if strings.HasPrefix(output, "```json") {
		output = strings.TrimPrefix(output, "```json")
		output = strings.TrimSuffix(output, "```")
		output = strings.TrimSpace(output)
	} else if strings.HasPrefix(output, "```") {
		output = strings.TrimPrefix(output, "```")
		output = strings.TrimSuffix(output, "```")
		output = strings.TrimSpace(output)
	}

	var resp generatorResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	if resp.Title == "" || resp.Body == "" {
		return "", "", fmt.Errorf("empty title or body in response")
	}

	return resp.Title, resp.Body, nil
}

// buildGeneratorPrompt creates the prompt for LLM-based issue generation.
// It applies ALL template configuration options (language, tone, etc.).
func (g *Generator) buildGeneratorPrompt(content, taskID, workflowID string, isMain bool) string {
	cfg := g.config.Template

	var sb strings.Builder
	sb.WriteString("Generate a GitHub/GitLab issue from the following content.\n\n")
	sb.WriteString("## Instructions (MUST follow all):\n\n")

	// Language - MANDATORY
	switch cfg.Language {
	case "spanish":
		sb.WriteString("- Write ALL content in Spanish language\n")
	case "french":
		sb.WriteString("- Write ALL content in French language\n")
	case "german":
		sb.WriteString("- Write ALL content in German language\n")
	case "portuguese":
		sb.WriteString("- Write ALL content in Portuguese language\n")
	case "chinese":
		sb.WriteString("- Write ALL content in Chinese language\n")
	case "japanese":
		sb.WriteString("- Write ALL content in Japanese language\n")
	default:
		sb.WriteString("- Write in English\n")
	}

	// Tone - MANDATORY
	switch cfg.Tone {
	case "professional":
		sb.WriteString("- Use professional, formal tone\n")
	case "casual":
		sb.WriteString("- Use casual, friendly tone\n")
	case "technical":
		sb.WriteString("- Use technical, detailed tone with precise terminology\n")
	case "concise":
		sb.WriteString("- Be extremely concise, bullet points preferred\n")
	default:
		sb.WriteString("- Use professional tone\n")
	}

	// Title format - apply configured format
	if cfg.TitleFormat != "" {
		sb.WriteString(fmt.Sprintf("- Title format: %s\n", cfg.TitleFormat))
		sb.WriteString("  (Replace {task_name} with descriptive name, {task_id} with task ID, {workflow_id} with workflow ID)\n")
	}

	// Include diagrams
	if cfg.IncludeDiagrams {
		sb.WriteString("- Include Mermaid diagrams if the content has architecture/flow information\n")
	} else {
		sb.WriteString("- Do NOT include diagrams\n")
	}

	// Summarize vs verbatim
	if g.config.Generator.Summarize {
		sb.WriteString("- Summarize content concisely, keeping key technical details\n")
		sb.WriteString(fmt.Sprintf("- Maximum body length: %d characters\n", g.config.Generator.MaxBodyLength))
	} else {
		sb.WriteString("- Include full content, only format for readability\n")
	}

	// Convention
	if cfg.Convention != "" {
		sb.WriteString(fmt.Sprintf("- Follow %s convention for formatting\n", cfg.Convention))
	}

	// Custom instructions - ALWAYS at the end, highest priority
	if cfg.CustomInstructions != "" {
		sb.WriteString(fmt.Sprintf("\n## Custom Instructions (HIGH PRIORITY):\n%s\n", cfg.CustomInstructions))
	}

	// Output format
	sb.WriteString("\n## Output:\n")
	sb.WriteString("Respond with valid JSON only:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\"title\": \"issue title here\", \"body\": \"issue body in markdown\"}\n")
	sb.WriteString("```\n\n")

	// Issue type context
	if isMain {
		sb.WriteString(fmt.Sprintf("## Context: This is the MAIN issue (parent/epic) summarizing workflow %s.\n\n", workflowID))
	} else {
		sb.WriteString(fmt.Sprintf("## Context: This is a SUB-ISSUE for task %s in workflow %s.\n\n", taskID, workflowID))
	}

	// Content to process - truncate if too long
	if len(content) > 50000 {
		content = content[:50000] + "\n\n*[Content truncated for processing...]*"
	}
	sb.WriteString("## Source Content:\n")
	sb.WriteString(content)

	// Footer instruction
	sb.WriteString("\n\n## Additional Requirements:\n")
	sb.WriteString("- Add footer: '*Generated by [Quorum AI](https://github.com/hugo-lorenzo-mato/quorum-ai)*'\n")

	return sb.String()
}
