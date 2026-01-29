-- Migration V5: Add columns for workflow isolation and task merge status

-- Add columns to workflows table
ALTER TABLE workflows ADD COLUMN workflow_branch TEXT;
ALTER TABLE workflows ADD COLUMN base_branch TEXT;
ALTER TABLE workflows ADD COLUMN merge_strategy TEXT;
ALTER TABLE workflows ADD COLUMN worktree_root TEXT;

-- Add column to tasks table
ALTER TABLE tasks ADD COLUMN merge_pending INTEGER DEFAULT 0;
