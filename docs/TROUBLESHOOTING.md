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
cat .quorum/state/state.json.lock

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
rm .quorum/state/state.json.lock
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
cat .quorum.yaml | grep -A5 "agents:"
```

**Solution:**

1. Ensure at least one agent CLI is installed:
   ```bash
   # Check if Claude is available
   which claude && claude --version

   # Check if Gemini is available
   which gemini && gemini --version
   ```

2. Verify agent configuration in `.quorum.yaml`:
   ```yaml
   agents:
     claude:
       enabled: true
       path: claude  # or full path: /usr/local/bin/claude
     gemini:
       enabled: true
       path: gemini
   ```

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

# Or check state file directly
cat .quorum/state/state.json | jq '.status'
```

**Solution:**

Starting from recent versions, quorum automatically recovers zombie workflows on server startup, marking them as failed with an appropriate message.

If manual intervention is needed:

1. Stop any running quorum processes:
   ```bash
   pkill -f "quorum"
   ```

2. Edit the state file to fix the status:
   ```bash
   # Backup first
   cp .quorum/state/state.json .quorum/state/state.json.backup

   # Edit status (using jq)
   jq '.status = "failed" | .error = "Workflow interrupted (manual recovery)"' \
     .quorum/state/state.json > /tmp/state.json && \
     mv /tmp/state.json .quorum/state/state.json
   ```

3. Remove any stale locks:
   ```bash
   rm -f .quorum/state/state.json.lock
   ```

4. Restart quorum:
   ```bash
   quorum serve  # or quorum status to verify
   ```

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
