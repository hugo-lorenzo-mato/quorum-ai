-- ============================================
-- Cleanup Ghost Workflows
-- WARNING: This script modifies data
-- Execute: sqlite3 .quorum/state/state.db < scripts/cleanup-ghost-workflows.sql
-- ============================================

-- ALWAYS backup first!
-- cp .quorum/state/state.db .quorum/state/state.db.backup.$(date +%Y%m%d%H%M%S)

.mode column
.headers on

-- Show what will be cleaned BEFORE cleaning
SELECT '=== ACTIVE_WORKFLOW TO CLEAN ===' AS action;
SELECT aw.workflow_id, w.status
FROM active_workflow aw
LEFT JOIN workflows w ON aw.workflow_id = w.id
WHERE w.status IN ('failed', 'completed') OR w.id IS NULL;

-- Clean active_workflow if it points to terminal or non-existent workflow
DELETE FROM active_workflow
WHERE workflow_id IN (
    SELECT aw.workflow_id
    FROM active_workflow aw
    LEFT JOIN workflows w ON aw.workflow_id = w.id
    WHERE w.status IN ('failed', 'completed') OR w.id IS NULL
);

SELECT '=== CLEANUP COMPLETED ===' AS result;
SELECT changes() AS rows_deleted;

-- Verify result
SELECT '=== POST-CLEANUP VERIFICATION ===' AS verification;
SELECT COUNT(*) AS active_workflows_remaining FROM active_workflow;

SELECT '=== CURRENT ACTIVE WORKFLOW ===' AS current_state;
SELECT * FROM active_workflow;
