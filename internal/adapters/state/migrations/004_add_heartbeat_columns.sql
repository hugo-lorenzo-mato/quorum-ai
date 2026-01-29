-- Migration 004: Add heartbeat columns for zombie detection and auto-resume
-- Adds columns to track workflow heartbeats and resume attempts

-- Add heartbeat_at column to track last heartbeat timestamp
ALTER TABLE workflows ADD COLUMN heartbeat_at DATETIME;

-- Add resume_count column to track number of auto-resumes
ALTER TABLE workflows ADD COLUMN resume_count INTEGER DEFAULT 0;

-- Add max_resumes column to limit auto-resume attempts
ALTER TABLE workflows ADD COLUMN max_resumes INTEGER DEFAULT 3;

-- Create index for efficient zombie detection queries
CREATE INDEX IF NOT EXISTS idx_workflows_zombie_detection ON workflows(status, heartbeat_at);

-- Record migration
INSERT INTO schema_migrations (version, description) VALUES (4, 'Add heartbeat columns for zombie detection');
