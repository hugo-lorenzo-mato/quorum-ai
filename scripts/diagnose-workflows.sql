-- ============================================
-- Diagnose Workflows - Ghost Workflow Detection
-- Execute: sqlite3 .quorum/state/state.db < scripts/diagnose-workflows.sql
-- ============================================

.mode column
.headers on

-- 1. Check active_workflow consistency
SELECT '=== ACTIVE WORKFLOW ===' AS section;
SELECT
    aw.workflow_id,
    w.status,
    w.report_path,
    CASE
        WHEN w.status IN ('failed', 'completed') THEN 'INCONSISTENT - Should be deactivated'
        WHEN w.id IS NULL THEN 'INCONSISTENT - Workflow not found'
        ELSE 'OK'
    END AS diagnosis
FROM active_workflow aw
LEFT JOIN workflows w ON aw.workflow_id = w.id;

-- 2. Workflows without report_path
SELECT '=== WORKFLOWS WITHOUT REPORT_PATH ===' AS section;
SELECT
    id,
    status,
    current_phase,
    created_at,
    'Missing report_path' AS issue
FROM workflows
WHERE (report_path IS NULL OR report_path = '')
AND status != 'pending';

-- 3. Failed workflows that are still active (ghost workflows)
SELECT '=== GHOST WORKFLOWS (failed but active) ===' AS section;
SELECT
    w.id,
    w.status,
    w.error,
    w.updated_at,
    CASE
        WHEN aw.workflow_id IS NOT NULL THEN 'ACTIVE (should not be)'
        ELSE 'Not active'
    END AS active_status
FROM workflows w
LEFT JOIN active_workflow aw ON w.id = aw.workflow_id
WHERE w.status = 'failed';

-- 4. Status summary
SELECT '=== STATUS SUMMARY ===' AS section;
SELECT
    status,
    COUNT(*) as count
FROM workflows
GROUP BY status;

-- 5. Recent workflows (last 7 days)
SELECT '=== RECENT WORKFLOWS (last 7 days) ===' AS section;
SELECT
    id,
    status,
    current_phase,
    SUBSTR(prompt, 1, 50) || '...' AS prompt_preview,
    created_at
FROM workflows
WHERE created_at >= datetime('now', '-7 days')
ORDER BY created_at DESC
LIMIT 10;
