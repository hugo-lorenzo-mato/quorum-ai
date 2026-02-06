-- Migration 011: Rename config column to blueprint
-- The former WorkflowConfig is now Blueprint with richer structure.
ALTER TABLE workflows RENAME COLUMN config TO blueprint;

INSERT INTO schema_migrations (version, description) VALUES (11, 'Rename config to blueprint');
