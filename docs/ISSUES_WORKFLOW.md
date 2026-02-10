# Issues Workflow Documentation

## Overview

Quorum AI can automatically generate GitHub/GitLab issues from workflow analysis. The issue generation supports two modes:

1. **AI Generation Mode**: Uses LLM to create polished, well-structured issues
2. **Fast Mode**: Direct copy of task files (instant, no AI processing)

After generation, issues can be reviewed, edited, and submitted directly through the UI.

## Workflow Steps

### 1. Generate Issues (Preview)

After completing a workflow analysis, navigate to the workflow detail page and click **"Generate Issues"**.

**Options:**
- **Fast Preview** (instant): Converts task files directly to issues
- **AI Generation** (60-120s): Uses LLM to create polished, professional issues

The system will:
- Read the consolidated analysis (main issue)
- Process each task file (sub-issues)
- Generate markdown files in `.quorum/issues/{workflowID}/`

### 2. Review & Edit

The Issues Editor provides:
- **Sidebar**: List of all generated issues
- **Editor Panel**: Edit title, body, labels, assignees
- **Preview**: See rendered markdown
- **Auto-save**: Changes saved to localStorage

**Edit Features:**
- Markdown formatting with syntax highlighting
- Real-time preview
- Individual issue editing
- Bulk label/assignee management

### 3. Create Issues

When satisfied with the issues:

1. Click **"Create Issues"** in the bottom action bar
2. System sends edited issues to backend
3. Backend creates issues via GitHub/GitLab API
4. Issues are linked: main issue references all sub-issues

**Result:**
- 1 main issue (consolidated analysis)
- N sub-issues (one per task)
- Sub-issues linked to main issue (if `link_issues: true`)

## API Endpoints

### POST `/api/workflows/{workflowID}/issues`

Creates issues from workflow artifacts or frontend-edited data.

**Request Body:**
```json
{
  "dry_run": false,
  "create_main_issue": true,
  "create_sub_issues": true,
  "link_issues": true,
  "labels": ["enhancement", "ai-generated"],
  "assignees": ["username"],
  "issues": [
    {
      "title": "Issue Title",
      "body": "Issue body in markdown",
      "labels": ["bug"],
      "assignees": ["developer"],
      "is_main_issue": true,
      "task_id": "main"
    },
    {
      "title": "Sub-issue Title",
      "body": "Sub-issue body",
      "labels": ["task"],
      "assignees": [],
      "is_main_issue": false,
      "task_id": "task-1"
    }
  ]
}
```

**Behavior:**
- If `issues` array provided: Creates issues from edited data (frontend flow)
- If `issues` empty/missing: Reads from filesystem (classic flow)
- Supports both dry-run (preview) and actual creation

**Response:**
```json
{
  "success": true,
  "message": "Created 1 main issue and 5 sub-issues",
  "main_issue": {
    "number": 123,
    "title": "Main Issue Title",
    "url": "https://github.com/owner/repo/issues/123",
    "state": "open",
    "labels": ["enhancement"]
  },
  "sub_issues": [
    {
      "number": 124,
      "title": "Task 1",
      "url": "https://github.com/owner/repo/issues/124",
      "state": "open",
      "labels": ["task"],
      "parent_issue": 123
    }
  ]
}
```

### GET `/api/workflows/{workflowID}/issues/preview`

Generates issue previews without creating them.

**Query Parameters:**
- `fast=true`: Fast mode (no AI, instant)
- `fast=false`: AI mode (60-120s)

**Response:**
```json
{
  "success": true,
  "message": "Preview: 12 issues (AI generated)",
  "ai_used": true,
  "preview_issues": [
    {
      "title": "Issue Title",
      "body": "Issue body",
      "labels": ["enhancement"],
      "assignees": [],
      "is_main_issue": true,
      "task_id": "main"
    }
  ]
}
```

### POST `/api/workflows/{workflowID}/issues/single`

Creates a single issue directly (bypass workflow).

**Request Body:**
```json
{
  "title": "Issue Title",
  "body": "Issue body",
  "labels": ["bug"],
  "assignees": ["developer"]
}
```

## Configuration

### `config.yaml`

```yaml
issues:
  enabled: true
  provider: "github"  # or "gitlab"
  
  # Default labels and assignees
  labels:
    - "quorum-ai"
    - "automated"
  assignees:
    - "project-lead"
  
  # AI generation settings
  generator:
    enabled: true
    agent: "claude"
    model: "claude-sonnet-4.5"
    summarize: true
  
  # Issue prompt settings
  parent_prompt: ""  # Optional prompt preset for parent issues
  prompt:
    language: "english"
    tone: "professional"
    include_diagrams: true
    custom_instructions: |
      - Focus on actionable tasks
      - Include acceptance criteria
      - Reference relevant code files
  
  # Timeout for generation
  timeout: "5m"
```

## File Structure

Generated issues are stored in `.quorum/issues/{workflowID}/`:

```
.quorum/issues/wf-20260202-122523-h00sj/
├── 00-consolidated-analysis.md      # Main issue
├── 01-project-registry-service.md    # Task 1
├── 02-project-context-encapsulation.md  # Task 2
├── 03-project-state-pool.md          # Task 3
└── ...
```

**Naming Convention:**
- `00-consolidated-analysis.md`: Main issue
- `01-{task-slug}.md`, `02-{task-slug}.md`, etc.: Sub-issues

**Cleanup Behavior:**
- Directory is **cleaned before each generation** to prevent duplicates
- Old files are removed automatically
- No manual cleanup required

## Deduplication

The system includes multiple deduplication safeguards:

1. **Pre-generation cleanup**: Removes old files before AI generation
2. **Filename pattern detection**: Extracts task IDs from multiple formats
3. **Task ID tracking**: Skips duplicate task IDs when reading files
4. **Logging**: Warns about duplicates in logs

**Supported Filename Patterns:**
- `00-consolidated-analysis.md` → Main issue
- `01-task-name.md` → task-1
- `02-another-task.md` → task-2
- Legacy: `issue-1-task.md`, `issue-task-1.md` (for compatibility)

## Troubleshooting

### Issue: More issues created than tasks

**Cause:** Old files accumulated from previous generations.

**Solution:** Fixed in v1.1.0+. System now auto-cleans before generation.

**Manual Cleanup:**
```bash
rm -rf .quorum/issues/{workflowID}/
```

### Issue: Edits not applied when creating issues

**Cause:** Old versions used filesystem instead of frontend data.

**Solution:** Fixed in v1.1.0+. Backend now reads from `issues` array in request.

**Verification:**
Check backend logs for: `"creating issues from frontend input"`

### Issue: AI generation timeout

**Cause:** Large workflows with 50+ tasks exceed timeout.

**Solution:**
1. Increase timeout in config: `issues.timeout: "10m"`
2. Use fast mode instead of AI mode
3. Split workflow into smaller batches

### Issue: Duplicate issues detected

**Cause:** Multiple files with same task ID.

**Solution:** System auto-deduplicates and logs warnings. Check logs:
```
WARN duplicate issue file detected file=issue-1-*.md task_id=task-1
```

## Best Practices

1. **Review before creating**: Always review generated issues in the editor
2. **Use AI mode for polish**: AI-generated issues are more professional
3. **Customize labels**: Set appropriate labels for your workflow
4. **Link issues**: Keep `link_issues: true` for traceability
5. **Fast mode for testing**: Use fast preview during development

## Advanced Usage

### Custom Issue Prompts

Use `issues.prompt.custom_instructions` to customize AI behavior:

```yaml
issues:
  prompt:
    custom_instructions: |
      - Include performance benchmarks
      - Reference Jira tickets
      - Add time estimates
```

If you're developing quorum-ai itself, you can also modify the embedded system prompt in `internal/service/prompts/issue-generate.md.tmpl`.

### Batch Processing

For large workflows (20+ tasks), AI generation uses batching:
- Max 8 tasks per batch (configurable)
- Each batch processes independently
- Files written to same directory

### Integration with CI/CD

```bash
# Generate issues in CI pipeline
quorum workflow run --config config.yaml
quorum issues generate --workflow-id $WORKFLOW_ID --fast

# Create issues automatically
quorum issues create --workflow-id $WORKFLOW_ID
```

## Migration Guide

### From v1.0.x to v1.1.0+

**Breaking Changes:** None

**New Features:**
- Frontend edit support
- Auto-cleanup
- Deduplication

**Action Required:** None. Existing workflows continue to work.

**Recommended:**
1. Update config to use new `generator.enabled: true`
2. Test issue editing in UI
3. Verify cleanup works (check `.quorum/issues/`)

## FAQ

**Q: Can I edit issues after creation?**  
A: Yes, on GitHub/GitLab directly. The editor is for pre-creation only.

**Q: What happens to generated files after creation?**  
A: They remain in `.quorum/issues/{workflowID}/` for reference.

**Q: Can I use both AI and fast mode?**  
A: Yes. Preview in fast mode, then regenerate with AI if needed.

**Q: How to disable issue generation?**  
A: Set `issues.enabled: false` in config.

**Q: Can I customize issue format?**  
A: Yes. Prefer using `issues.prompt.custom_instructions`. If you're developing quorum-ai itself, edit the embedded system prompt in `internal/service/prompts/issue-generate.md.tmpl`.

## See Also

- [Configuration Guide](CONFIGURATION.md)
- [Workflow Reports](WORKFLOW_REPORTS.md)
- [Troubleshooting](TROUBLESHOOTING.md)
