-- Migration 003: Add title column to workflows
-- Adds missing title column for workflows that were created before it was added

-- Add title column if it doesn't exist
-- SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so this will fail silently if the column exists
-- The migration system should handle this gracefully
ALTER TABLE workflows ADD COLUMN title TEXT;

-- Record migration
INSERT INTO schema_migrations (version, description) VALUES (3, 'Add title column to workflows');
