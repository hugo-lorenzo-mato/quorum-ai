-- Migration 005: Add agent_events column for UI reload recovery
-- Stores serialized agent events (JSON array) for persisting streaming activity

ALTER TABLE workflows ADD COLUMN agent_events TEXT;

-- Insert migration record
INSERT INTO schema_migrations (version, description) VALUES (5, 'Add agent_events column');
