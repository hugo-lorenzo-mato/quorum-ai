-- Migration 009: Add prompt_hash column for duplicate detection
-- This allows efficient detection of workflows with identical prompts

-- Add prompt_hash column to workflows table
ALTER TABLE workflows ADD COLUMN prompt_hash TEXT;

-- Create index for efficient lookup by prompt hash
CREATE INDEX IF NOT EXISTS idx_workflows_prompt_hash ON workflows(prompt_hash);
