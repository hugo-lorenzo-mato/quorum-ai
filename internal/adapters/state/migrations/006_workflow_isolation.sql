-- Migration 006: Workflow-level Git isolation
--
-- This migration enables concurrent workflow execution by:
-- 1. Adding Git branch tracking columns to workflows table
-- 2. Creating running_workflows table for concurrent execution tracking
-- 3. Creating workflow_locks table for per-workflow locking
-- 4. Adding indexes for efficient queries
--
-- NOTE: The active_workflow table is NOT dropped for backward compatibility.
-- New code should use running_workflows instead.

-- Add Git isolation columns to workflows table
-- workflow_branch: The Git branch created for this workflow
ALTER TABLE workflows ADD COLUMN workflow_branch TEXT;

-- base_branch: The branch from which workflow branch was created
ALTER TABLE workflows ADD COLUMN base_branch TEXT DEFAULT 'main';

-- merge_strategy: How task branches merge to workflow branch
-- Values: 'sequential', 'parallel', 'rebase'
ALTER TABLE workflows ADD COLUMN merge_strategy TEXT DEFAULT 'sequential';

-- worktree_root: Path to the worktree directory for this workflow
ALTER TABLE workflows ADD COLUMN worktree_root TEXT;

-- Create multi-workflow running tracking table
-- Replaces the singleton active_workflow pattern
CREATE TABLE IF NOT EXISTS running_workflows (
    workflow_id TEXT PRIMARY KEY,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lock_holder_pid INTEGER,
    lock_holder_host TEXT,
    heartbeat_at DATETIME,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Index for efficient heartbeat queries (zombie detection)
CREATE INDEX IF NOT EXISTS idx_running_workflows_heartbeat
ON running_workflows(heartbeat_at);

-- Create per-workflow lock table
CREATE TABLE IF NOT EXISTS workflow_locks (
    workflow_id TEXT PRIMARY KEY,
    acquired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    holder_pid INTEGER NOT NULL,
    holder_host TEXT NOT NULL,
    expires_at DATETIME,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Update tasks table for workflow-scoped branches
-- Branch naming will include workflow ID
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_branch
ON tasks(workflow_id, branch);

-- Insert migration record (idempotent)
INSERT INTO schema_migrations (version, description, applied_at)
SELECT 6, 'Workflow-level Git isolation', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 6);
