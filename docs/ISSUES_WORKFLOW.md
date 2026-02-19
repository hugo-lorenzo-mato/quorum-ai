# Issues Workflow Documentation

## Overview

Quorum AI can automatically generate GitHub/GitLab issues from workflow analysis. The issue generation supports two modes:

1. **AI Generation Mode** (`mode: "agent"`): Uses LLM to create polished, well-structured issues
2. **Fast Mode** (`mode: "direct"`): Direct copy of task files (instant, no AI processing)

After generation, issues can be reviewed, edited as drafts, and published directly through the UI or API.

## Workflow Steps

### 1. Generate Issues (Preview)

After completing a workflow analysis, navigate to the workflow detail page and click **"Generate Issues"**.

**Options:**
- **Fast Preview** (instant): Converts task files directly to issues
- **AI Generation** (60-120s): Uses LLM to create polished, professional issues

The system will:
- Read the consolidated analysis (main issue)
- Process each task file (sub-issues)
- Generate markdown files in `.quorum/issues/{workflowID}/draft/`

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

The draft-based workflow also supports editing via the API (see [Draft Management Endpoints](#draft-management-endpoints) below).

### 3. Publish Issues

When satisfied with the issues:

1. Click **"Create Issues"** in the bottom action bar (or use the publish API endpoint)
2. System sends edited issues to backend
3. Backend creates issues via GitHub/GitLab API
4. Issues are linked: main issue references all sub-issues

**Result:**
- 1 main issue (consolidated analysis)
- N sub-issues (one per task)
- Sub-issues linked to main issue (if `link_issues: true`)

## API Endpoints

All issue endpoints are under the `/api/v1` prefix and require a valid workflow ID.

### POST `/api/v1/workflows/{workflowID}/issues`

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
      "task_id": "main",
      "file_path": ""
    },
    {
      "title": "Sub-issue Title",
      "body": "Sub-issue body",
      "labels": ["task"],
      "assignees": [],
      "is_main_issue": false,
      "task_id": "task-1",
      "file_path": ""
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
  "message": "Created 6 issues",
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
  ],
  "errors": [],
  "ai_used": true,
  "ai_errors": []
}
```

### GET `/api/v1/workflows/{workflowID}/issues/preview`

Generates issue previews without creating them.

**Query Parameters:**
- `fast=true`: Fast mode (no AI, instant)
- `fast=false`: AI mode (60-120s)

Note: AI mode can take significant time for large workflows. Progress is reported via SSE events (see [Real-Time Events](#real-time-events-sse)).

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
      "task_id": "main",
      "file_path": ".quorum/issues/wf-xxx/draft/00-consolidated-analysis.md"
    }
  ],
  "errors": [],
  "ai_errors": []
}
```

### POST `/api/v1/workflows/{workflowID}/issues/single`

Creates a single issue directly.

**Request Body:**
```json
{
  "title": "Issue Title",
  "body": "Issue body",
  "labels": ["bug"],
  "assignees": ["developer"],
  "is_main_issue": false,
  "task_id": "task-1",
  "file_path": ""
}
```

**Response:**
```json
{
  "success": true,
  "issue": {
    "number": 125,
    "title": "Issue Title",
    "url": "https://github.com/owner/repo/issues/125",
    "state": "open",
    "labels": ["bug"]
  }
}
```

### POST `/api/v1/workflows/{workflowID}/issues/files`

Saves issues to markdown files on disk without creating them on the provider. Useful for persisting frontend edits to the draft directory.

**Request Body:**
```json
{
  "issues": [
    {
      "title": "Issue Title",
      "body": "Issue body",
      "labels": ["enhancement"],
      "assignees": ["developer"],
      "is_main_issue": true,
      "task_id": "main",
      "file_path": ""
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "message": "Saved 5 issue file(s)",
  "issues": [
    {
      "title": "Issue Title",
      "body": "Issue body",
      "labels": ["enhancement"],
      "assignees": ["developer"],
      "is_main_issue": true,
      "task_id": "main",
      "file_path": ".quorum/issues/wf-xxx/draft/00-consolidated-analysis.md"
    }
  ]
}
```

### Draft Management Endpoints

These endpoints support the draft-based workflow for reviewing and editing issues before publishing.

#### GET `/api/v1/workflows/{workflowID}/issues/drafts`

Lists all current draft files for a workflow.

**Response:**
```json
{
  "workflow_id": "wf-20260202-122523-h00sj",
  "drafts": [
    {
      "title": "Consolidated Analysis",
      "body": "Main issue body...",
      "labels": ["enhancement"],
      "assignees": [],
      "is_main_issue": true,
      "task_id": "main",
      "file_path": ".quorum/issues/wf-xxx/draft/00-consolidated-analysis.md"
    },
    {
      "title": "Task 1: Implement Feature",
      "body": "Sub-issue body...",
      "labels": ["task"],
      "assignees": [],
      "is_main_issue": false,
      "task_id": "task-1",
      "file_path": ".quorum/issues/wf-xxx/draft/01-implement-feature.md"
    }
  ]
}
```

#### PUT `/api/v1/workflows/{workflowID}/issues/drafts/{taskId}`

Edits a specific draft file. Only non-null fields in the request body are applied (partial update).

**Request Body:**
```json
{
  "title": "Updated Title",
  "body": "Updated body content",
  "labels": ["enhancement", "priority-high"],
  "assignees": ["developer"]
}
```

All fields are optional (use JSON null to skip a field). For example, to update only the title:
```json
{
  "title": "New Title"
}
```

**Response:**
```json
{
  "title": "Updated Title",
  "body": "Updated body content",
  "labels": ["enhancement", "priority-high"],
  "assignees": ["developer"],
  "is_main_issue": false,
  "task_id": "task-1",
  "file_path": ".quorum/issues/wf-xxx/draft/01-implement-feature.md"
}
```

#### POST `/api/v1/workflows/{workflowID}/issues/publish`

Publishes draft issues to GitHub/GitLab. Optionally filter which drafts to publish by task ID.

**Request Body:**
```json
{
  "dry_run": false,
  "link_issues": true,
  "task_ids": ["main", "task-1", "task-2"]
}
```

- `dry_run`: Preview what would be created without actually creating issues
- `link_issues`: Link sub-issues to the main issue
- `task_ids`: Publish only specific drafts (empty array publishes all drafts)

**Response:**
```json
{
  "workflow_id": "wf-20260202-122523-h00sj",
  "published": [
    {
      "task_id": "",
      "file_path": "",
      "issue_number": 123,
      "issue_url": "https://github.com/owner/repo/issues/123",
      "is_main_issue": true
    },
    {
      "task_id": "",
      "file_path": "",
      "issue_number": 124,
      "issue_url": "https://github.com/owner/repo/issues/124",
      "is_main_issue": false
    }
  ]
}
```

#### GET `/api/v1/workflows/{workflowID}/issues/status`

Returns the current status of issue drafts and published issues for a workflow.

**Response:**
```json
{
  "workflow_id": "wf-20260202-122523-h00sj",
  "has_drafts": true,
  "draft_count": 6,
  "has_published": true,
  "published_count": 6
}
```

### GET `/api/v1/config/issues`

Returns the current issues configuration for the active project.

**Response:** The full `IssuesConfig` object as defined in the configuration section below.

## Real-Time Events (SSE)

Issue generation emits Server-Sent Events (SSE) for real-time progress tracking in the UI. Connect to the SSE endpoint to receive these events:

- `GET /api/v1/events` -- primary SSE endpoint
- `GET /api/v1/sse/events` -- alias for frontend compatibility

### Event Type: `issues_generation_progress`

Emitted during AI-based issue file generation.

**Payload:**
```json
{
  "type": "issues_generation_progress",
  "workflow_id": "wf-20260202-122523-h00sj",
  "project_id": "proj-1",
  "stage": "generating",
  "current": 3,
  "total": 12,
  "message": "Generating issue for task-3",
  "file_name": "03-implement-feature.md",
  "title": "Implement Feature X",
  "task_id": "task-3",
  "is_main_issue": false
}
```

### Event Type: `issues_publishing_progress`

Emitted during issue creation/publishing to the provider.

**Payload:**
```json
{
  "type": "issues_publishing_progress",
  "workflow_id": "wf-20260202-122523-h00sj",
  "project_id": "proj-1",
  "stage": "creating",
  "current": 2,
  "total": 6,
  "message": "Creating issue on GitHub",
  "title": "Task 2: Refactor Service",
  "task_id": "task-2",
  "is_main_issue": false,
  "issue_number": 125,
  "dry_run": false
}
```

### Event Type: Agent Stream Events

During AI generation, real-time agent streaming events are also emitted with types including: `started`, `thinking`, `tool_use`, `chunk`, `progress`, `completed`, and `error`. These provide granular visibility into the LLM generation process.

## Configuration

### `config.yaml`

The issues configuration block supports the following fields. For the full configuration reference, see [CONFIGURATION.md](CONFIGURATION.md).

```yaml
issues:
  enabled: true
  provider: "github"  # or "gitlab"
  mode: "agent"       # "direct" (fast copy) or "agent" (LLM-based generation)
  auto_generate: false # Automatically generate issues after planning phase
  timeout: "5m"       # Timeout for generation operations
  repository: ""      # Override auto-detected repository (format: "owner/repo")
  draft_directory: "" # Custom draft directory (default: .quorum/issues/)

  # Default labels and assignees
  labels:
    - "quorum-ai"
    - "automated"
  assignees:
    - "project-lead"

  # Parent issue prompt preset
  parent_prompt: ""  # Optional prompt preset for parent issues

  # Issue prompt settings
  prompt:
    language: "english"
    tone: "professional"    # formal, informal, technical, friendly, professional
    include_diagrams: true
    title_format: ""        # Pattern with variables: {workflow_id}, {task_name}, etc.
    body_prompt_file: ""    # Path to custom body prompt file
    convention: ""          # Style reference: "conventional-commits", "angular", etc.
    custom_instructions: |
      - Focus on actionable tasks
      - Include acceptance criteria
      - Reference relevant code files

  # AI generation settings
  generator:
    enabled: true
    agent: "claude"
    model: "claude-sonnet-4.5"
    summarize: true
    max_body_length: 0       # 0 = unlimited
    reasoning_effort: ""     # Agent-specific reasoning effort
    instructions: ""         # Custom instructions for body generation
    title_instructions: ""   # Custom instructions for title generation
    resilience:
      enabled: false
      max_retries: 3
      initial_backoff: "1s"
      max_backoff: "30s"
      backoff_multiplier: 2.0
      failure_threshold: 5
      reset_timeout: "60s"
    validation:
      enabled: false
      sanitize_forbidden: false
      required_sections: []
      forbidden_patterns: []
    rate_limit:
      enabled: false
      max_per_minute: 30

  # GitLab-specific options (only when provider: "gitlab")
  gitlab:
    use_epics: false
    project_id: ""
```

## File Structure

Generated issues are stored in `.quorum/issues/{workflowID}/` with separate subdirectories for drafts and published issues:

```
.quorum/issues/wf-20260202-122523-h00sj/
    draft/
        00-consolidated-analysis.md      # Main issue draft
        01-project-registry-service.md    # Task 1 draft
        02-project-context-encapsulation.md  # Task 2 draft
        03-project-state-pool.md          # Task 3 draft
        ...
    published/
        # Files moved here after successful publishing
        ...
```

**Naming Convention:**
- `00-consolidated-analysis.md`: Main issue
- `01-{task-slug}.md`, `02-{task-slug}.md`, etc.: Sub-issues

**Cleanup Behavior:**
- The draft directory is **cleaned before each generation** to prevent duplicates
- Old files are removed automatically
- No manual cleanup required

## Deduplication

The system includes multiple deduplication safeguards:

1. **Pre-generation cleanup**: Removes old files before AI generation
2. **Filename pattern detection**: Extracts task IDs from multiple formats
3. **Task ID tracking**: Skips duplicate task IDs when reading files
4. **Logging**: Warns about duplicates in logs

**Supported Filename Patterns:**
- `00-consolidated-analysis.md` -- Main issue
- `01-task-name.md` -- task-1
- `02-another-task.md` -- task-2
- Legacy: `issue-1-task.md`, `issue-task-1.md` (for compatibility)

## Troubleshooting

### Issue: More issues created than tasks

**Cause:** Old files accumulated from previous generations.

**Solution:** Fixed in current versions. System now auto-cleans before generation.

**Manual Cleanup:**
```bash
rm -rf .quorum/issues/{workflowID}/
```

### Issue: Edits not applied when creating issues

**Cause:** Old versions used filesystem instead of frontend data.

**Solution:** Fixed in current versions. Backend now reads from `issues` array in request.

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

1. **Review before publishing**: Always review generated issues in the editor or via the drafts API
2. **Use AI mode for polish**: AI-generated issues are more professional
3. **Customize labels**: Set appropriate labels for your workflow
4. **Link issues**: Keep `link_issues: true` for traceability
5. **Fast mode for testing**: Use fast preview during development
6. **Use the draft workflow**: Generate drafts, review and edit, then publish for maximum control

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

Issue operations are currently API-only. Use `curl` or similar HTTP clients in CI/CD pipelines:

```bash
# Start the quorum server
quorum serve &
sleep 5

# Generate issue previews (fast mode) via API
curl -s "http://localhost:8080/api/v1/workflows/$WORKFLOW_ID/issues/preview?fast=true"

# Publish drafts to GitHub via API
curl -s -X POST "http://localhost:8080/api/v1/workflows/$WORKFLOW_ID/issues/publish" \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false, "link_issues": true}'
```

## FAQ

**Q: Can I edit issues after creation?**
A: Yes, on GitHub/GitLab directly. The editor and draft API are for pre-creation review only.

**Q: What happens to generated files after creation?**
A: Draft files remain in `.quorum/issues/{workflowID}/draft/` for reference. Published records are tracked separately.

**Q: Can I use both AI and fast mode?**
A: Yes. Preview in fast mode, then regenerate with AI if needed.

**Q: How to disable issue generation?**
A: Set `issues.enabled: false` in config.

**Q: Can I customize issue format?**
A: Yes. Prefer using `issues.prompt.custom_instructions`. If you're developing quorum-ai itself, edit the embedded system prompt in `internal/service/prompts/issue-generate.md.tmpl`.

**Q: What is the difference between "direct" and "agent" mode?**
A: Direct mode copies task artifacts as-is into issues (instant). Agent mode uses an LLM to rewrite and polish the content into well-structured GitHub issues (takes 60-120s).

## See Also

- [Configuration Guide](CONFIGURATION.md)
- [Workflow Reports](WORKFLOW_REPORTS.md)
- [Troubleshooting](TROUBLESHOOTING.md)
