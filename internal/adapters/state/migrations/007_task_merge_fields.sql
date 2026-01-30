-- Migration 007: Add merge tracking fields to tasks table
--
-- These fields support workflow-level Git isolation by tracking:
-- 1. merge_pending: Whether a task's merge to workflow branch failed (conflict)
-- 2. merge_commit: The commit hash after successful merge to workflow branch
--
-- This enables proper recovery after crashes/restarts when using workflow isolation.

-- Add merge tracking columns to tasks table
ALTER TABLE tasks ADD COLUMN merge_pending INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN merge_commit TEXT;

-- Insert migration record (idempotent)
INSERT INTO schema_migrations (version, description, applied_at)
SELECT 7, 'Task merge tracking fields', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 7);
