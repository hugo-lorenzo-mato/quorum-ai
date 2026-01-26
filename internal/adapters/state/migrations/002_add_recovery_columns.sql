-- Migration 002: Add recovery metadata columns
-- Adds fields for workflow resume and task recovery capabilities

-- Add report_path to workflows table for resume support
ALTER TABLE workflows ADD COLUMN report_path TEXT;

-- Add recovery metadata columns to tasks table
-- These columns enable task-level resume and git operation tracking
ALTER TABLE tasks ADD COLUMN last_commit TEXT;
ALTER TABLE tasks ADD COLUMN files_modified TEXT;  -- JSON array of file paths
ALTER TABLE tasks ADD COLUMN branch TEXT;
ALTER TABLE tasks ADD COLUMN resumable INTEGER NOT NULL DEFAULT 0;  -- SQLite boolean (0=false, 1=true)
ALTER TABLE tasks ADD COLUMN resume_hint TEXT;

-- Add partial index for efficiently finding resumable tasks
CREATE INDEX IF NOT EXISTS idx_tasks_resumable ON tasks(resumable) WHERE resumable = 1;

-- Add partial index for branch-based queries (only index non-null values)
CREATE INDEX IF NOT EXISTS idx_tasks_branch ON tasks(branch) WHERE branch IS NOT NULL;

-- Record migration
INSERT INTO schema_migrations (version, description) VALUES (2, 'Add recovery metadata columns');
