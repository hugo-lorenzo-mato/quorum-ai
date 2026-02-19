# Troubleshooting

This document covers common issues and their solutions when using quorum-ai.

---

## Table of Contents

- [State Lock Issues](#state-lock-issues)
  - [Lock Acquire Failed](#lock-acquire-failed)
- [Agent Issues](#agent-issues)
  - [No Agents Available](#no-agents-available)
- [Workflow Issues](#workflow-issues)
  - [Workflow Stuck in Running State](#workflow-stuck-in-running-state)
- [OpenCode / Ollama Issues](#opencode--ollama-issues)
  - [Truncated Output or Missing Context](#truncated-output-or-missing-context)
  - [OpenCode Connection Refused](#opencode-connection-refused)
  - [Model Not Found](#model-not-found)
  - [Slow OpenCode Response](#slow-opencode-response)
- [WebUI / Server Issues](#webui--server-issues)
  - [Server Fails to Start](#server-fails-to-start)
  - [Chat Session Errors](#chat-session-errors)
- [Snapshot Issues](#snapshot-issues)
  - [Snapshot Export Fails](#snapshot-export-fails)
  - [Snapshot Import Conflicts](#snapshot-import-conflicts)
- [Issue Generation Issues](#issue-generation-issues)
  - [More Issues Created Than Tasks](#more-issues-created-than-tasks)
  - [Issue Edits Not Applied](#issue-edits-not-applied)
  - [Duplicate Issues Warning in Logs](#duplicate-issues-warning-in-logs)
  - [AI Generation Timeout](#ai-generation-timeout)
  - [Issues Missing Task Information](#issues-missing-task-information)
- [Getting Help](#getting-help)

---

## State Lock Issues

### Lock Acquire Failed

**Symptoms:**

```
Workflow failed: acquiring lock: [state] LOCK_ACQUIRE_FAILED: lock held by PID 1320641 since 2026-01-27 04:36:36
```

**Cause:**

The state manager uses file-based locking to prevent concurrent modifications. This error occurs when:

1. Another quorum process (CLI, TUI, or web server) is already running
2. A previous process crashed without releasing the lock
3. The lock file was left behind after an unexpected termination

**Diagnosis:**

Check the lock file contents and verify if the holding process is still alive:

```bash
# View lock holder information
cat .quorum/state/state.db.lock

# Example output:
# {"pid":1320641,"hostname":"myhost","acquired_at":"2026-01-27T04:36:36.992Z"}

# Check if the process is still running
ps aux | grep <PID>
```

**Solution:**

If the process is still running and you need to use it:
```bash
# Option 1: Use the existing process (recommended)
# If it's the web server, access via http://localhost:8080
# If it's the TUI, switch to that terminal

# Option 2: Stop the existing process gracefully
kill <PID>
```

If the process is dead but the lock remains (stale lock):
```bash
# Remove the stale lock file
rm .quorum/state/state.db.lock
```

**Prevention:**

- Avoid running multiple quorum instances simultaneously on the same project
- Use `quorum serve` for the web interface instead of running CLI commands while the server is active
- If running in scripts, ensure proper signal handling to clean up locks on termination

---

## Agent Issues

### No Agents Available

**Symptoms:**

```
Workflow failed: [validation] NO_AGENTS: no agents available for analyze phase
```

**Cause:**

This error occurs when quorum cannot find any working agents for the requested phase. Common causes:

1. No agents are enabled in the configuration
2. Agent CLI tools are not installed or not in PATH
3. Agent health checks are failing (timeout, authentication issues)
4. Context timeout expired before agents could respond

**Diagnosis:**

```bash
# Verify agent installation and configuration
quorum doctor

# Check which agents are configured
# Primary location: .quorum/config.yaml
# Legacy fallback: .quorum.yaml (supported for backward compatibility)
cat .quorum/config.yaml | grep -A10 "agents:"
```

**Solution:**

1. Ensure at least one agent CLI is installed:
   ```bash
   # Check if Claude is available
   which claude && claude --version

   # Check if Gemini is available
   which gemini && gemini --version
   ```

2. Verify agent configuration in `.quorum/config.yaml`:
   ```yaml
   agents:
     claude:
       enabled: true
       path: claude  # or full path: /usr/local/bin/claude
       model: claude-opus-4-6
       phases:
         analyze: true
         plan: true
         execute: true
     gemini:
       enabled: true
       path: gemini
       model: gemini-2.5-pro
       phases:
         analyze: true
         plan: true
         execute: true
   ```

   Note: The `phases` map is required. Without it, agents are enabled for no phases due to the strict opt-in model. The `model` field specifies which model variant the agent should use.

3. Test agent connectivity manually:
   ```bash
   # Test Claude
   echo "Hello" | claude --print

   # Test Gemini
   echo "Hello" | gemini
   ```

4. Check for timeout issues - increase the workflow timeout if agents are slow to respond:
   ```yaml
   workflow:
     timeout: "30m"  # Increase from default if needed
   ```

---

## Workflow Issues

### Workflow Stuck in Running State

**Symptoms:**

- Workflow shows "running" status but no progress is being made
- UI or CLI shows the workflow as active but agents are not executing
- After server restart, workflow still shows "running"

**Cause:**

This can happen when:

1. The server or CLI crashed during workflow execution
2. The process was killed without proper cleanup
3. Network issues interrupted agent communication

**Diagnosis:**

```bash
# Check workflow status via API (if server is running)
curl -s http://localhost:8080/api/v1/workflows | jq '.[] | select(.status == "running")'

# Or check the state DB directly (requires sqlite3)
sqlite3 .quorum/state/state.db "select id,status,current_phase,updated_at from workflows where status = 'running';"
```

**Solution:**

Starting from recent versions, quorum automatically recovers zombie workflows on server startup, marking them as failed with an appropriate message.

If manual intervention is needed:

1. Stop any running quorum processes:
   ```bash
   pkill -f "quorum"
   ```

2. Clear the active workflow (keeps history, just deactivates):
   ```bash
   quorum new
   ```

3. Remove any stale locks (if present):
   ```bash
   rm -f .quorum/state/state.db.lock
   ```

4. Restart quorum:
   ```bash
   quorum serve  # or quorum status to verify
   ```

---

## OpenCode / Ollama Issues

### Truncated Output or Missing Context

**Symptoms:**

- Agent responses are cut off mid-sentence
- Code analysis misses important context
- "Context too long" errors in Ollama logs

**Cause:**

Ollama defaults to 2048-4096 tokens context window, insufficient for code-intensive tasks. OpenCode requires 64K+ tokens for effective operation.

**Diagnosis:**

```bash
# Check current context allocation
ollama ps

# Look for CONTEXT column - should show your configured value
# NAME                 ID              SIZE     PROCESSOR   CONTEXT
# qwen2.5-coder:32b    b92d6a0bd47e    27 GB    100% GPU    2048     <- TOO LOW
```

**Solution:**

Configure Ollama context window globally:

```bash
# Create systemd override
sudo systemctl edit ollama.service

# Add:
[Service]
Environment="OLLAMA_CONTEXT_LENGTH=32768"

# Apply changes
sudo systemctl daemon-reload
sudo systemctl restart ollama

# Verify
ollama ps
```

See [Ollama Integration Guide](OLLAMA.md#context-window-configuration) for detailed instructions.

---

### OpenCode Connection Refused

**Symptoms:**

```
Error: connect ECONNREFUSED 127.0.0.1:11434
```

**Cause:**

Ollama server not running or bound to different address.

**Solution:**

```bash
# Check Ollama status
systemctl status ollama

# Start if not running
sudo systemctl start ollama

# Verify connectivity
curl http://localhost:11434/api/tags
```

---

### Model Not Found

**Symptoms:**

```
Error: model "qwen2.5-coder" not found
```

**Solution:**

```bash
# List available models
ollama list

# Pull required model
ollama pull qwen2.5-coder:32b

# Update quorum config to match exact model tag
```

---

### Slow OpenCode Response

**Symptoms:**

- Long delays (30s+) before output starts
- Timeouts during execution

**Cause:**

1. Model cold start (first load after server restart)
2. Context window too large for available VRAM
3. CPU fallback due to insufficient GPU memory

**Solution:**

```bash
# Pre-load model to avoid cold start
ollama run qwen2.5-coder:32b --keepalive 1h

# Or reduce context if VRAM constrained
sudo systemctl edit ollama.service
# Set: Environment="OLLAMA_CONTEXT_LENGTH=16384"
```

---

## WebUI / Server Issues

### Server Fails to Start

**Symptoms:**

```
Error: listen tcp :8080: bind: address already in use
```

**Cause:**

Another process is already bound to the configured port, or a previous `quorum serve` instance was not fully terminated.

**Diagnosis:**

```bash
# Check which process is using port 8080
lsof -i :8080

# Check for running quorum processes
pgrep -af quorum
```

**Solution:**

1. Stop the conflicting process, or use a different port:
   ```yaml
   server:
     port: 9090  # Use a different port
   ```

2. If a previous quorum server is still running:
   ```bash
   pkill -f "quorum serve"
   ```

3. Restart the server:
   ```bash
   quorum serve
   ```

### Chat Session Errors

**Symptoms:**

- Chat messages fail to send with HTTP 500 errors
- Agent selection does not persist between messages
- Attachments fail to upload

**Cause:**

Chat sessions depend on the server event bus and agent registry being correctly initialized. Common causes include:

1. Selected agent is not enabled or not installed
2. Session state was lost after server restart
3. File attachments exceed configured limits

**Solution:**

1. Verify agent availability:
   ```bash
   quorum doctor
   ```

2. Create a new chat session if the current one is in an invalid state. Chat sessions do not persist across server restarts.

3. For attachment issues, verify that the file exists and is readable by the quorum process.

---

## Snapshot Issues

### Snapshot Export Fails

**Symptoms:**

```
Error: snapshot export failed: archive creation error
```

**Cause:**

1. Insufficient disk space for the archive
2. Corrupted state database
3. Stale lock file preventing state access

**Solution:**

1. Ensure sufficient disk space:
   ```bash
   df -h .
   ```

2. Remove stale locks before export:
   ```bash
   rm -f .quorum/state/state.db.lock
   ```

3. Validate state before export:
   ```bash
   quorum snapshot validate
   ```

### Snapshot Import Conflicts

**Symptoms:**

```
Error: snapshot import failed: project already exists
```

**Cause:**

The snapshot contains projects or workflows that conflict with existing data in the target environment.

**Solution:**

1. Use the validate command to preview what would be imported:
   ```bash
   quorum snapshot validate --file snapshot.tar.gz
   ```

2. If conflicts are expected, back up existing state before importing.

3. Import the snapshot into a clean environment if possible.

---

## Issue Generation Issues

### More Issues Created Than Tasks

**Symptoms:**

```
Expected 12 issues, but 15 were created in GitHub
```

**Cause:**

In older versions without auto-cleanup, issue generation files accumulated in `.quorum/issues/{workflowID}/draft/` without cleanup, causing duplicates when regenerating issues.

**Solution:**

In current versions, this is fixed automatically. The system cleans the draft directory before each generation.

If you encounter this on an older installation, manually clean the directory:

```bash
rm -rf .quorum/issues/{workflowID}/
```

Then regenerate issues.

### Issue Edits Not Applied

**Symptoms:**

```
Edited issue titles/bodies in the UI, but created issues have original content
```

**Cause:**

In older versions, the backend ignored the edited issues sent from the frontend and re-read files from disk.

**Solution:**

In current versions, this is fixed automatically. The backend now uses edited content from the frontend.

**Verification:** Check backend logs for:
```
INFO creating issues from frontend input count=12
```

For older installations, upgrade to the latest version, or edit the generated markdown files directly in `.quorum/issues/{workflowID}/draft/`.

### Duplicate Issues Warning in Logs

**Symptoms:**

```
WARN duplicate issue file detected file=issue-1-task.md task_id=task-1
```

**Cause:**

Multiple files exist with the same task ID (e.g., `01-task.md` and `issue-1-task.md`).

**Solution:**

This is informational only. The system automatically deduplicates and uses the first file found (sorted numerically).

To prevent in future:
1. Ensure AI generation completes successfully (no partial runs)
2. The system auto-cleans the draft directory before generation

### AI Generation Timeout

**Symptoms:**

```
Error: issue generation failed: context deadline exceeded
```

**Cause:**

Large workflows with 50+ tasks exceed the configured timeout.

**Solution:**

**Option 1:** Increase timeout in config:
```yaml
issues:
  timeout: "10m"  # Increase from default 5m
```

**Option 2:** Use fast mode (no AI processing) via the API:
```bash
curl http://localhost:8080/api/v1/workflows/$WF_ID/issues/preview?fast=true
```

**Option 3:** Process tasks in smaller batches by splitting the workflow.

### Issues Missing Task Information

**Symptoms:**

```
Created issues don't include task details, acceptance criteria, or proper structure
```

**Cause:**

Fast mode was used (direct copy) instead of AI mode.

**Solution:**

Use AI generation for better quality:

1. In UI: Select "AI Generation" instead of "Fast Preview"
2. Via API: Call `/api/v1/workflows/{workflowID}/issues/preview` without `?fast=true`
3. Ensure `generator.enabled: true` in config

**Trade-off:** AI mode takes 60-120s vs instant for fast mode.

---

## Getting Help

If you encounter an issue not covered here:

1. Check the [GitHub Issues](https://github.com/hugo-lorenzo-mato/quorum-ai/issues) for similar problems
2. Run `quorum doctor` to diagnose common configuration issues
3. Enable debug logging for more details:
   ```yaml
   log:
     level: debug
   ```
4. Open a new issue with:
   - quorum version (`quorum version`)
   - Operating system and architecture
   - Relevant configuration (redact sensitive data)
   - Full error message and stack trace if available
   - Steps to reproduce the issue

---

## See Also

- [Issues Workflow Documentation](ISSUES_WORKFLOW.md) for complete guide
- [Configuration Reference](CONFIGURATION.md) for issue generation settings
