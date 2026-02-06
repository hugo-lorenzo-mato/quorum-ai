-- Migration 001: Initial schema
-- Creates the base tables for workflow state persistence

-- Schema migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-- Active workflow tracking
CREATE TABLE IF NOT EXISTS active_workflow (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    workflow_id TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    title TEXT,
    status TEXT NOT NULL,
    current_phase TEXT NOT NULL,
    prompt TEXT NOT NULL,
    optimized_prompt TEXT,
    task_order TEXT NOT NULL, -- JSON array of task IDs
    config TEXT,              -- JSON serialized WorkflowConfig
    metrics TEXT,             -- JSON serialized StateMetrics
    checksum TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT NOT NULL,
    workflow_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    cli TEXT,
    model TEXT,
    dependencies TEXT,       -- JSON array of task IDs
    tokens_in INTEGER NOT NULL DEFAULT 0,
    tokens_out INTEGER NOT NULL DEFAULT 0,
    retries INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    worktree_path TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    output TEXT,
    output_file TEXT,
    model_used TEXT,
    finish_reason TEXT,
    tool_calls TEXT,         -- JSON array of ToolCall
    PRIMARY KEY (id, workflow_id),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Checkpoints table
CREATE TABLE IF NOT EXISTS checkpoints (
    id TEXT NOT NULL,
    workflow_id TEXT NOT NULL,
    type TEXT NOT NULL,
    phase TEXT NOT NULL,
    task_id TEXT,
    timestamp DATETIME NOT NULL,
    message TEXT,
    data BLOB,
    PRIMARY KEY (id, workflow_id),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflows_updated_at ON workflows(updated_at);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow_id ON tasks(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_checkpoints_workflow_id ON checkpoints(workflow_id);

-- Insert migration record
INSERT INTO schema_migrations (version, description) VALUES (1, 'Initial schema');
