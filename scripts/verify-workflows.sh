#!/bin/bash
# Verify consistency between database and filesystem

DB_PATH="${QUORUM_DB:-.quorum/state/state.db}"
RUNS_DIR="${QUORUM_RUNS:-.quorum/runs}"

echo "=== Workflow Verification ==="
echo "Database: $DB_PATH"
echo "Runs directory: $RUNS_DIR"
echo ""

# Check database exists
if [ ! -f "$DB_PATH" ]; then
    echo "ERROR: Database not found at $DB_PATH"
    exit 1
fi

# 1. Workflows in DB with missing directories
echo "=== Workflows in DB with missing directories ==="
has_missing=false
sqlite3 "$DB_PATH" "SELECT id, status, report_path FROM workflows WHERE report_path != '' AND report_path IS NOT NULL" | while IFS='|' read -r id status path; do
    if [ -n "$path" ] && [ ! -d "$path" ]; then
        echo "MISSING: $id ($status) -> $path"
        has_missing=true
    fi
done

if [ "$has_missing" = false ]; then
    echo "  (none)"
fi

# 2. Directories without workflow in DB
echo ""
echo "=== Orphan directories (not in DB) ==="
has_orphan=false
if [ -d "$RUNS_DIR" ]; then
    for dir in "$RUNS_DIR"/wf-*; do
        if [ -d "$dir" ]; then
            wf_id=$(basename "$dir")
            count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM workflows WHERE id='$wf_id'")
            if [ "$count" -eq 0 ]; then
                echo "ORPHAN DIR: $dir"
                has_orphan=true
            fi
        fi
    done
fi

if [ "$has_orphan" = false ]; then
    echo "  (none)"
fi

# 3. Active workflow consistency
echo ""
echo "=== Active Workflow ==="
active_id=$(sqlite3 "$DB_PATH" "SELECT workflow_id FROM active_workflow WHERE id=1")
if [ -n "$active_id" ]; then
    status=$(sqlite3 "$DB_PATH" "SELECT status FROM workflows WHERE id='$active_id'")
    echo "Active: $active_id (status: $status)"
    if [ "$status" = "failed" ] || [ "$status" = "completed" ]; then
        echo "WARNING: Active workflow is in terminal state!"
        echo "Run: sqlite3 $DB_PATH 'DELETE FROM active_workflow WHERE id=1;'"
    fi
else
    echo "No active workflow"
fi

# 4. Summary
echo ""
echo "=== Status Summary ==="
sqlite3 "$DB_PATH" "SELECT status, COUNT(*) as count FROM workflows GROUP BY status"

echo ""
echo "=== Verification Complete ==="
