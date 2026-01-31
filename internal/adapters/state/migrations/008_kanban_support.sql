-- Migration 008: Kanban board support for workflows
--
-- This migration enables Kanban-style workflow management by:
-- 1. Adding Kanban columns to workflows table for board management
-- 2. Adding PR tracking fields (pr_url, pr_number) - currently logged but not persisted
-- 3. Adding Kanban execution metadata (started_at, completed_at, execution_count, last_error)
-- 4. Creating kanban_engine_state singleton table for engine state persistence
--
-- IMPORTANT CONCEPTUAL DISTINCTION:
-- - kanban_column: Visual position on the Kanban board
-- - workflow.status: Internal execution state (pending, running, completed, etc.)
-- - These are INDEPENDENT - a workflow can be status='completed' but kanban_column='to_verify'

-- Add Kanban column field to workflows
-- Valid values: refinement (default), todo, in_progress, to_verify, done
ALTER TABLE workflows ADD COLUMN kanban_column TEXT DEFAULT 'refinement';

-- Position within column for drag & drop ordering (lower = higher in list)
ALTER TABLE workflows ADD COLUMN kanban_position INTEGER DEFAULT 0;

-- PR tracking fields - CRITICAL: These were previously logged but never persisted
-- See workflow_isolation_finalize.go where pr.HTMLURL and pr.Number are logged but not saved
ALTER TABLE workflows ADD COLUMN pr_url TEXT;
ALTER TABLE workflows ADD COLUMN pr_number INTEGER;

-- Kanban execution tracking
-- These track the Kanban-initiated execution, separate from workflow's internal timestamps
ALTER TABLE workflows ADD COLUMN kanban_started_at DATETIME;
ALTER TABLE workflows ADD COLUMN kanban_completed_at DATETIME;
ALTER TABLE workflows ADD COLUMN kanban_execution_count INTEGER DEFAULT 0;
ALTER TABLE workflows ADD COLUMN kanban_last_error TEXT;

-- Kanban engine state table (singleton - only id=1 is valid)
-- Stores global engine state that must persist across server restarts
CREATE TABLE IF NOT EXISTS kanban_engine_state (
    -- Singleton constraint: only id=1 is allowed
    id INTEGER PRIMARY KEY CHECK (id = 1),

    -- Is the Kanban execution engine enabled?
    -- 0 = disabled (default), 1 = enabled
    enabled INTEGER NOT NULL DEFAULT 0,

    -- Currently executing workflow (NULL if none)
    -- Uses SET NULL on delete to handle workflow cleanup gracefully
    current_workflow_id TEXT,

    -- Circuit breaker tracking
    -- consecutive_failures counts failures in a row (resets on success)
    -- When >= 2, circuit_breaker_open is set to 1 and engine auto-disables
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_failure_at DATETIME,
    circuit_breaker_open INTEGER NOT NULL DEFAULT 0,

    -- Last update timestamp
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key to workflows table
    FOREIGN KEY (current_workflow_id) REFERENCES workflows(id) ON DELETE SET NULL
);

-- Initialize the singleton engine state row
-- INSERT OR IGNORE ensures this is idempotent
INSERT OR IGNORE INTO kanban_engine_state (id, enabled, consecutive_failures, circuit_breaker_open, updated_at)
VALUES (1, 0, 0, 0, CURRENT_TIMESTAMP);

-- Create indexes for efficient Kanban queries
-- idx_workflows_kanban_column: for filtering by column (e.g., "show all in 'todo'")
CREATE INDEX IF NOT EXISTS idx_workflows_kanban_column ON workflows(kanban_column);

-- idx_workflows_kanban_position: for ordering within column (drag & drop)
-- Composite index covers both column filtering AND position ordering
CREATE INDEX IF NOT EXISTS idx_workflows_kanban_position ON workflows(kanban_column, kanban_position);

-- Record this migration (idempotent)
INSERT INTO schema_migrations (version, description, applied_at)
SELECT 8, 'Kanban board support', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 8);
