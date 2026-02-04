-- Migration 010: Add description column to tasks table
-- Stores detailed task description for agent execution context

ALTER TABLE tasks ADD COLUMN description TEXT;

-- Insert migration record
INSERT INTO schema_migrations (version, description) VALUES (10, 'Add task description column');
