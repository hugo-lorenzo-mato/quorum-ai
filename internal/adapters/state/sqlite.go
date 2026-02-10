package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/fsutil"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial_schema.sql
var migrationV1 string

//go:embed migrations/002_add_recovery_columns.sql
var migrationV2 string

//go:embed migrations/003_add_title_column.sql
var migrationV3 string

//go:embed migrations/004_add_heartbeat_columns.sql
var migrationV4 string

//go:embed migrations/005_add_agent_events.sql
var migrationV5 string

//go:embed migrations/006_workflow_isolation.sql
var migrationV6 string

//go:embed migrations/007_task_merge_fields.sql
var migrationV7 string

//go:embed migrations/008_kanban_support.sql
var migrationV8 string

//go:embed migrations/009_prompt_hash.sql
var migrationV9 string

//go:embed migrations/010_add_task_description.sql
var migrationV10 string

//go:embed migrations/011_blueprint.sql
var migrationV11 string

// SQLiteStateManager implements StateManager with SQLite storage.
type SQLiteStateManager struct {
	dbPath     string
	backupPath string
	lockPath   string
	lockTTL    time.Duration
	db         *sql.DB // Write connection
	readDB     *sql.DB // Read-only connection for non-blocking reads
	mu         sync.RWMutex

	// Retry configuration
	maxRetries    int
	baseRetryWait time.Duration
}

// SQLiteStateManagerOption configures the manager.
type SQLiteStateManagerOption func(*SQLiteStateManager)

// NewSQLiteStateManager creates a new SQLite state manager.
func NewSQLiteStateManager(dbPath string, opts ...SQLiteStateManagerOption) (*SQLiteStateManager, error) {
	m := &SQLiteStateManager{
		dbPath:        dbPath,
		backupPath:    dbPath + ".bak",
		lockPath:      dbPath + ".lock",
		lockTTL:       time.Hour,
		maxRetries:    5,
		baseRetryWait: 100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(m)
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	// Open write connection with WAL mode for better concurrency
	// busy_timeout=5000 means wait up to 5 seconds for locks before returning SQLITE_BUSY
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening write database: %w", err)
	}

	// Configure write connection pool - SQLite only supports one writer at a time
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Don't close idle connections
	m.db = db

	// Open read-only connection for non-blocking reads
	// mode=ro ensures this connection cannot write, avoiding lock contention
	readDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&mode=ro&_pragma=busy_timeout(1000)")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("opening read database: %w", err)
	}

	// Configure read connection pool - can have multiple readers
	readDB.SetMaxOpenConns(10)
	readDB.SetMaxIdleConns(5)
	readDB.SetConnMaxLifetime(5 * time.Minute)
	m.readDB = readDB

	// Run migrations (uses write connection)
	if err := m.migrate(); err != nil {
		_ = db.Close()
		_ = readDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	// Cleanup ghost workflows on startup
	// This fixes any inconsistencies from previous crashes or bugs
	if err := m.cleanupOnStartup(context.Background()); err != nil {
		// Log but don't fail startup - cleanup errors shouldn't prevent operation
		fmt.Printf("WARN: startup cleanup failed: %v\n", err)
	}

	return m, nil
}

// WithSQLiteBackupPath sets the backup file path.
func WithSQLiteBackupPath(path string) SQLiteStateManagerOption {
	return func(m *SQLiteStateManager) {
		m.backupPath = path
	}
}

// WithSQLiteLockTTL sets the lock TTL used to break stale locks.
func WithSQLiteLockTTL(ttl time.Duration) SQLiteStateManagerOption {
	return func(m *SQLiteStateManager) {
		m.lockTTL = ttl
	}
}

// Close closes both database connections.
func (m *SQLiteStateManager) Close() error {
	var errs []error
	if m.readDB != nil {
		if err := m.readDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing read connection: %w", err))
		}
	}
	if m.db != nil {
		if err := m.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing write connection: %w", err))
		}
	}
	if len(errs) > 0 {
		return errs[0] // Return first error
	}
	return nil
}

// retryWrite executes a write operation with exponential backoff retry.
// It specifically handles SQLITE_BUSY errors by retrying.
func (m *SQLiteStateManager) retryWrite(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		if err := fn(); err != nil {
			// Check if it's a busy/locked error
			if isSQLiteBusy(err) {
				lastErr = err
				if attempt < m.maxRetries {
					// Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
					wait := m.baseRetryWait * time.Duration(1<<attempt)
					select {
					case <-ctx.Done():
						return fmt.Errorf("%s: %w (last error: %v)", operation, ctx.Err(), lastErr)
					case <-time.After(wait):
						continue
					}
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("%s: max retries exceeded: %w", operation, lastErr)
}

// isSQLiteBusy checks if an error is a SQLite busy/locked error.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY") ||
		strings.Contains(errStr, "SQLITE_LOCKED")
}

// migrate runs pending migrations.
//
//nolint:gocyclo // Migration stepper is intentionally verbose for clarity.
func (m *SQLiteStateManager) migrate() error {
	// Check current schema version
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist yet, run initial migration
		version = 0
	}

	// Run migrations based on current version
	if version < 1 {
		if _, err := m.db.Exec(migrationV1); err != nil {
			return fmt.Errorf("applying migration v1: %w", err)
		}
	}

	if version < 2 {
		if _, err := m.db.Exec(migrationV2); err != nil {
			return fmt.Errorf("applying migration v2: %w", err)
		}
	}

	if version < 3 {
		// Migration V3 adds title column - ignore error if column already exists
		_, err := m.db.Exec(migrationV3)
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v3: %w", err)
		}
	}

	if version < 4 {
		// Migration V4 adds heartbeat columns - ignore error if columns already exist
		_, err := m.db.Exec(migrationV4)
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v4: %w", err)
		}
	}

	if version < 5 {
		// Migration V5 adds agent_events column for UI reload recovery
		_, err := m.db.Exec(migrationV5)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("applying migration v5: %w", err)
		}
	}

	if version < 6 {
		// Migration V6 adds workflow isolation tables
		_, err := m.db.Exec(migrationV6)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v6: %w", err)
		}
	}

	if version < 7 {
		// Migration V7 adds merge tracking fields to tasks table
		_, err := m.db.Exec(migrationV7)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v7: %w", err)
		}
	}

	if version < 8 {
		// Migration V8 adds Kanban board support
		_, err := m.db.Exec(migrationV8)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v8: %w", err)
		}
	}

	if version < 9 {
		// Migration V9 adds prompt_hash for duplicate detection
		_, err := m.db.Exec(migrationV9)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v9: %w", err)
		}
	}

	if version < 10 {
		// Migration V10 adds description column to tasks table
		_, err := m.db.Exec(migrationV10)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("applying migration v10: %w", err)
		}
	}

	if version < 11 {
		// Migration V11 renames config column to blueprint
		_, err := m.db.Exec(migrationV11)
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "no such column") {
			return fmt.Errorf("applying migration v11: %w", err)
		}
	}

	return nil
}

type saveOptions struct {
	preserveUpdatedAt bool
	disableAutoKanban bool
	setActiveWorkflow bool
}

// Save persists workflow state atomically and sets it as the active workflow.
func (m *SQLiteStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	return m.saveWithOptions(ctx, state, saveOptions{
		preserveUpdatedAt: false,
		disableAutoKanban: false,
		setActiveWorkflow: true,
	})
}

func (m *SQLiteStateManager) saveWithOptions(ctx context.Context, state *core.WorkflowState, opts saveOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update timestamp
	if opts.preserveUpdatedAt {
		if state.UpdatedAt.IsZero() {
			state.UpdatedAt = time.Now()
		}
	} else {
		state.UpdatedAt = time.Now()
	}

	// Auto-move completed workflows to "to_verify" column if not already in to_verify or done
	if !opts.disableAutoKanban && state.Status == core.WorkflowStatusCompleted {
		if state.KanbanColumn != "to_verify" && state.KanbanColumn != "done" {
			state.KanbanColumn = "to_verify"
			now := time.Now()
			state.KanbanCompletedAt = &now
		}
	}

	// Calculate checksum
	state.Checksum = ""
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling state for checksum: %w", err)
	}
	hash := sha256.Sum256(stateBytes)
	checksum := hex.EncodeToString(hash[:])

	// Persist each Save() atomically. AcquireLock() provides external mutual exclusion,
	// but we always commit per call so state is durable even if the process terminates.
	tx, txErr := m.db.BeginTx(ctx, nil)
	if txErr != nil {
		return fmt.Errorf("beginning transaction: %w", txErr)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize JSON fields
	taskOrderJSON, err := json.Marshal(state.TaskOrder)
	if err != nil {
		return fmt.Errorf("marshaling task order: %w", err)
	}

	var blueprintJSON, metricsJSON []byte
	if state.Blueprint != nil {
		blueprintJSON, err = json.Marshal(state.Blueprint)
		if err != nil {
			return fmt.Errorf("marshaling blueprint: %w", err)
		}
	}
	if state.Metrics != nil {
		metricsJSON, err = json.Marshal(state.Metrics)
		if err != nil {
			return fmt.Errorf("marshaling metrics: %w", err)
		}
	}

	// Serialize agent events
	var agentEventsJSON []byte
	if len(state.AgentEvents) > 0 {
		agentEventsJSON, err = json.Marshal(state.AgentEvents)
		if err != nil {
			return fmt.Errorf("marshaling agent events: %w", err)
		}
	}

	// Calculate prompt hash for duplicate detection
	promptHash := ""
	if state.Prompt != "" {
		h := sha256.Sum256([]byte(state.Prompt))
		promptHash = hex.EncodeToString(h[:])
	}

	// Upsert workflow
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflows (
			id, version, title, status, current_phase, prompt, optimized_prompt,
			task_order, blueprint, metrics, checksum, created_at, updated_at, report_path,
			agent_events, workflow_branch,
			kanban_column, kanban_position, pr_url, pr_number,
			kanban_started_at, kanban_completed_at, kanban_execution_count, kanban_last_error,
			prompt_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			title = excluded.title,
			status = excluded.status,
			current_phase = excluded.current_phase,
			prompt = excluded.prompt,
			optimized_prompt = excluded.optimized_prompt,
			task_order = excluded.task_order,
			blueprint = excluded.blueprint,
			metrics = excluded.metrics,
			checksum = excluded.checksum,
			updated_at = excluded.updated_at,
			report_path = excluded.report_path,
			agent_events = excluded.agent_events,
			workflow_branch = excluded.workflow_branch,
			kanban_column = excluded.kanban_column,
			kanban_position = excluded.kanban_position,
			pr_url = excluded.pr_url,
			pr_number = excluded.pr_number,
			kanban_started_at = excluded.kanban_started_at,
			kanban_completed_at = excluded.kanban_completed_at,
			kanban_execution_count = excluded.kanban_execution_count,
			kanban_last_error = excluded.kanban_last_error,
			prompt_hash = excluded.prompt_hash
	`,
		state.WorkflowID, state.Version, state.Title, state.Status, state.CurrentPhase,
		state.Prompt, state.OptimizedPrompt, string(taskOrderJSON),
		nullableString(blueprintJSON), nullableString(metricsJSON),
		checksum, state.CreatedAt, state.UpdatedAt,
		nullableString([]byte(state.ReportPath)),
		nullableString(agentEventsJSON),
		nullableString([]byte(state.WorkflowBranch)),
		nullableString([]byte(state.KanbanColumn)), state.KanbanPosition,
		nullableString([]byte(state.PRURL)), state.PRNumber,
		nullableTime(state.KanbanStartedAt), nullableTime(state.KanbanCompletedAt),
		state.KanbanExecutionCount, nullableString([]byte(state.KanbanLastError)),
		nullableString([]byte(promptHash)),
	)
	if err != nil {
		return fmt.Errorf("upserting workflow: %w", err)
	}

	// Delete existing tasks for this workflow (will re-insert)
	_, err = tx.ExecContext(ctx, "DELETE FROM tasks WHERE workflow_id = ?", state.WorkflowID)
	if err != nil {
		return fmt.Errorf("deleting existing tasks: %w", err)
	}

	// Insert tasks
	for _, task := range state.Tasks {
		if err := m.insertTask(ctx, tx, state.WorkflowID, task); err != nil {
			return fmt.Errorf("inserting task %s: %w", task.ID, err)
		}
	}

	// Delete existing checkpoints (will re-insert)
	_, err = tx.ExecContext(ctx, "DELETE FROM checkpoints WHERE workflow_id = ?", state.WorkflowID)
	if err != nil {
		return fmt.Errorf("deleting existing checkpoints: %w", err)
	}

	// Insert checkpoints
	for _, cp := range state.Checkpoints {
		if err := m.insertCheckpoint(ctx, tx, state.WorkflowID, &cp); err != nil {
			return fmt.Errorf("inserting checkpoint %s: %w", cp.ID, err)
		}
	}

	// Set as active workflow
	if opts.setActiveWorkflow {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO active_workflow (id, workflow_id, updated_at)
			VALUES (1, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				workflow_id = excluded.workflow_id,
				updated_at = excluded.updated_at
		`, state.WorkflowID, time.Now())
		if err != nil {
			return fmt.Errorf("setting active workflow: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (m *SQLiteStateManager) insertTask(ctx context.Context, tx *sql.Tx, workflowID core.WorkflowID, task *core.TaskState) error {
	depsJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("marshaling dependencies: %w", err)
	}

	var toolCallsJSON []byte
	if len(task.ToolCalls) > 0 {
		toolCallsJSON, err = json.Marshal(task.ToolCalls)
		if err != nil {
			return fmt.Errorf("marshaling tool calls: %w", err)
		}
	}

	var filesModifiedJSON []byte
	if len(task.FilesModified) > 0 {
		filesModifiedJSON, err = json.Marshal(task.FilesModified)
		if err != nil {
			return fmt.Errorf("marshaling files modified: %w", err)
		}
	}

	// Convert bools to SQLite integer (0/1)
	resumableInt := 0
	if task.Resumable {
		resumableInt = 1
	}
	mergePendingInt := 0
	if task.MergePending {
		mergePendingInt = 1
	}

	_, err = tx.ExecContext(ctx, `
			INSERT INTO tasks (
				id, workflow_id, phase, name, description, status, cli, model,
				dependencies, tokens_in, tokens_out, retries,
				error, worktree_path, started_at, completed_at,
				output, output_file, model_used, finish_reason, tool_calls,
				last_commit, files_modified, branch, resumable, resume_hint,
				merge_pending, merge_commit
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		task.ID, workflowID, task.Phase, task.Name, nullableString([]byte(task.Description)), task.Status,
		task.CLI, task.Model, string(depsJSON),
		task.TokensIn, task.TokensOut, task.Retries,
		nullableString([]byte(task.Error)), nullableString([]byte(task.WorktreePath)),
		nullableTime(task.StartedAt), nullableTime(task.CompletedAt),
		nullableString([]byte(task.Output)), nullableString([]byte(task.OutputFile)),
		nullableString([]byte(task.ModelUsed)), nullableString([]byte(task.FinishReason)),
		nullableString(toolCallsJSON),
		nullableString([]byte(task.LastCommit)), nullableString(filesModifiedJSON),
		nullableString([]byte(task.Branch)), resumableInt, nullableString([]byte(task.ResumeHint)),
		mergePendingInt, nullableString([]byte(task.MergeCommit)),
	)
	return err
}

func (m *SQLiteStateManager) insertCheckpoint(ctx context.Context, tx *sql.Tx, workflowID core.WorkflowID, cp *core.Checkpoint) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO checkpoints (
			id, workflow_id, type, phase, task_id, timestamp, message, data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		cp.ID, workflowID, cp.Type, cp.Phase, nullableString([]byte(string(cp.TaskID))),
		cp.Timestamp, nullableString([]byte(cp.Message)), cp.Data,
	)
	return err
}

// Load retrieves the active workflow state.
func (m *SQLiteStateManager) Load(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get active workflow ID using read connection
	var activeID string
	err := m.readDB.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)
	if err == sql.ErrNoRows {
		return nil, nil // No active workflow
	}
	if err != nil {
		return nil, fmt.Errorf("getting active workflow ID: %w", err)
	}

	return m.loadWorkflowByID(ctx, core.WorkflowID(activeID))
}

// LoadByID retrieves a specific workflow state by its ID.
func (m *SQLiteStateManager) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadWorkflowByID(ctx, id)
}

//nolint:gocyclo // Multi-table load keeps logic centralized for consistency.
func (m *SQLiteStateManager) loadWorkflowByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	// Load workflow using read connection
	var state core.WorkflowState
	var taskOrderJSON, blueprintJSON, metricsJSON sql.NullString
	var optimizedPrompt sql.NullString
	var checksum sql.NullString
	var reportPath sql.NullString
	var title sql.NullString
	var agentEventsJSON sql.NullString
	var workflowBranch sql.NullString
	// Kanban fields
	var kanbanColumn, prURL, kanbanLastError sql.NullString
	var kanbanPosition, prNumber, kanbanExecutionCount sql.NullInt64
	var kanbanStartedAt, kanbanCompletedAt sql.NullTime

	err := m.readDB.QueryRowContext(ctx, `
		SELECT id, version, title, status, current_phase, prompt, optimized_prompt,
		       task_order, blueprint, metrics, checksum, created_at, updated_at, report_path,
		       agent_events, workflow_branch,
		       kanban_column, kanban_position, pr_url, pr_number,
		       kanban_started_at, kanban_completed_at, kanban_execution_count, kanban_last_error
		FROM workflows WHERE id = ?
	`, id).Scan(
		&state.WorkflowID, &state.Version, &title, &state.Status, &state.CurrentPhase,
		&state.Prompt, &optimizedPrompt, &taskOrderJSON, &blueprintJSON, &metricsJSON,
		&checksum, &state.CreatedAt, &state.UpdatedAt, &reportPath, &agentEventsJSON, &workflowBranch,
		&kanbanColumn, &kanbanPosition, &prURL, &prNumber,
		&kanbanStartedAt, &kanbanCompletedAt, &kanbanExecutionCount, &kanbanLastError,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Workflow doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("loading workflow: %w", err)
	}

	if title.Valid {
		state.Title = title.String
	}
	if optimizedPrompt.Valid {
		state.OptimizedPrompt = optimizedPrompt.String
	}
	if checksum.Valid {
		state.Checksum = checksum.String
	}
	if reportPath.Valid {
		state.ReportPath = reportPath.String
	}
	if workflowBranch.Valid {
		state.WorkflowBranch = workflowBranch.String
	}
	// Kanban fields
	if kanbanColumn.Valid {
		state.KanbanColumn = kanbanColumn.String
	}
	state.KanbanPosition = int(kanbanPosition.Int64)
	if prURL.Valid {
		state.PRURL = prURL.String
	}
	state.PRNumber = int(prNumber.Int64)
	if kanbanStartedAt.Valid {
		state.KanbanStartedAt = &kanbanStartedAt.Time
	}
	if kanbanCompletedAt.Valid {
		state.KanbanCompletedAt = &kanbanCompletedAt.Time
	}
	state.KanbanExecutionCount = int(kanbanExecutionCount.Int64)
	if kanbanLastError.Valid {
		state.KanbanLastError = kanbanLastError.String
	}

	// Parse JSON fields
	if taskOrderJSON.Valid {
		if err := json.Unmarshal([]byte(taskOrderJSON.String), &state.TaskOrder); err != nil {
			return nil, fmt.Errorf("unmarshaling task order: %w", err)
		}
	}
	if blueprintJSON.Valid && blueprintJSON.String != "" {
		state.Blueprint = &core.Blueprint{}
		if err := json.Unmarshal([]byte(blueprintJSON.String), state.Blueprint); err != nil {
			return nil, fmt.Errorf("unmarshaling blueprint: %w", err)
		}
	}
	if metricsJSON.Valid && metricsJSON.String != "" {
		state.Metrics = &core.StateMetrics{}
		if err := json.Unmarshal([]byte(metricsJSON.String), state.Metrics); err != nil {
			return nil, fmt.Errorf("unmarshaling metrics: %w", err)
		}
	}
	if agentEventsJSON.Valid && agentEventsJSON.String != "" {
		if err := json.Unmarshal([]byte(agentEventsJSON.String), &state.AgentEvents); err != nil {
			return nil, fmt.Errorf("unmarshaling agent events: %w", err)
		}
	}

	// Load tasks using read connection
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	rows, err := m.readDB.QueryContext(ctx, `
			SELECT id, phase, name, description, status, cli, model, dependencies,
			       tokens_in, tokens_out, retries, error,
			       worktree_path, started_at, completed_at, output,
			       output_file, model_used, finish_reason, tool_calls,
			       last_commit, files_modified, branch, resumable, resume_hint,
			       merge_pending, merge_commit
			FROM tasks WHERE workflow_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("loading tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		task, err := m.scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning task: %w", err)
		}
		state.Tasks[task.ID] = task
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tasks: %w", err)
	}

	// Load checkpoints using read connection
	cpRows, err := m.readDB.QueryContext(ctx, `
		SELECT id, type, phase, task_id, timestamp, message, data
		FROM checkpoints WHERE workflow_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoints: %w", err)
	}
	defer cpRows.Close()

	for cpRows.Next() {
		cp, err := m.scanCheckpoint(cpRows)
		if err != nil {
			return nil, fmt.Errorf("scanning checkpoint: %w", err)
		}
		state.Checkpoints = append(state.Checkpoints, *cp)
	}
	if err := cpRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checkpoints: %w", err)
	}

	return &state, nil
}

func (m *SQLiteStateManager) scanTask(rows *sql.Rows) (*core.TaskState, error) {
	var task core.TaskState
	var depsJSON, toolCallsJSON sql.NullString
	var description, cli, model, errorStr, worktreePath sql.NullString
	var startedAt, completedAt sql.NullTime
	var output, outputFile, modelUsed, finishReason sql.NullString
	var lastCommit, filesModifiedJSON, branch, resumeHint sql.NullString
	var resumable int
	var mergePending sql.NullInt64
	var mergeCommit sql.NullString

	err := rows.Scan(
		&task.ID, &task.Phase, &task.Name, &description, &task.Status,
		&cli, &model, &depsJSON,
		&task.TokensIn, &task.TokensOut, &task.Retries,
		&errorStr, &worktreePath, &startedAt, &completedAt,
		&output, &outputFile, &modelUsed, &finishReason, &toolCallsJSON,
		&lastCommit, &filesModifiedJSON, &branch, &resumable, &resumeHint,
		&mergePending, &mergeCommit,
	)
	if err != nil {
		return nil, err
	}

	if description.Valid {
		task.Description = description.String
	}
	if cli.Valid {
		task.CLI = cli.String
	}
	if model.Valid {
		task.Model = model.String
	}
	if errorStr.Valid {
		task.Error = errorStr.String
	}
	if worktreePath.Valid {
		task.WorktreePath = worktreePath.String
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if output.Valid {
		task.Output = output.String
	}
	if outputFile.Valid {
		task.OutputFile = outputFile.String
	}
	if modelUsed.Valid {
		task.ModelUsed = modelUsed.String
	}
	if finishReason.Valid {
		task.FinishReason = finishReason.String
	}
	if lastCommit.Valid {
		task.LastCommit = lastCommit.String
	}
	if branch.Valid {
		task.Branch = branch.String
	}
	if resumeHint.Valid {
		task.ResumeHint = resumeHint.String
	}
	task.Resumable = resumable != 0
	if mergePending.Valid {
		task.MergePending = mergePending.Int64 != 0
	}
	if mergeCommit.Valid {
		task.MergeCommit = mergeCommit.String
	}

	if depsJSON.Valid && depsJSON.String != "" {
		if err := json.Unmarshal([]byte(depsJSON.String), &task.Dependencies); err != nil {
			return nil, fmt.Errorf("unmarshaling dependencies: %w", err)
		}
	}
	if toolCallsJSON.Valid && toolCallsJSON.String != "" {
		if err := json.Unmarshal([]byte(toolCallsJSON.String), &task.ToolCalls); err != nil {
			return nil, fmt.Errorf("unmarshaling tool calls: %w", err)
		}
	}
	if filesModifiedJSON.Valid && filesModifiedJSON.String != "" {
		if err := json.Unmarshal([]byte(filesModifiedJSON.String), &task.FilesModified); err != nil {
			return nil, fmt.Errorf("unmarshaling files modified: %w", err)
		}
	}

	return &task, nil
}

func (m *SQLiteStateManager) scanCheckpoint(rows *sql.Rows) (*core.Checkpoint, error) {
	var cp core.Checkpoint
	var taskID, message sql.NullString
	var data []byte

	err := rows.Scan(&cp.ID, &cp.Type, &cp.Phase, &taskID, &cp.Timestamp, &message, &data)
	if err != nil {
		return nil, err
	}

	if taskID.Valid {
		cp.TaskID = core.TaskID(taskID.String)
	}
	if message.Valid {
		cp.Message = message.String
	}
	cp.Data = data

	return &cp, nil
}

// ListWorkflows returns summaries of all available workflows.
func (m *SQLiteStateManager) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get active workflow ID using read connection
	var activeID sql.NullString
	_ = m.readDB.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)

	rows, err := m.readDB.QueryContext(ctx, `
		SELECT id, title, status, current_phase, prompt, created_at, updated_at
		FROM workflows
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}
	defer rows.Close()

	var summaries []core.WorkflowSummary
	for rows.Next() {
		var s core.WorkflowSummary
		var title sql.NullString
		err := rows.Scan(&s.WorkflowID, &title, &s.Status, &s.CurrentPhase, &s.Prompt, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning workflow summary: %w", err)
		}
		if title.Valid {
			s.Title = title.String
		}

		// Truncate prompt for display
		if len(s.Prompt) > 100 {
			s.Prompt = s.Prompt[:100] + "..."
		}

		// Mark active workflow
		if activeID.Valid && string(s.WorkflowID) == activeID.String {
			s.IsActive = true
		}

		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workflow summaries: %w", err)
	}

	return summaries, nil
}

// GetActiveWorkflowID returns the ID of the currently active workflow.
// It validates that the workflow exists and is not in a terminal state (failed/completed).
// If the active workflow is inconsistent, it is automatically cleaned up and empty is returned.
func (m *SQLiteStateManager) GetActiveWorkflowID(ctx context.Context) (core.WorkflowID, error) {
	m.mu.RLock()

	// Use read connection for non-blocking reads
	var id string
	err := m.readDB.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&id)
	if err == sql.ErrNoRows {
		m.mu.RUnlock()
		return "", nil
	}
	if err != nil {
		m.mu.RUnlock()
		return "", fmt.Errorf("getting active workflow ID: %w", err)
	}

	// Validate that the workflow exists and is not in a terminal state
	if id != "" {
		var status string
		err := m.readDB.QueryRowContext(ctx,
			"SELECT status FROM workflows WHERE id = ?", id).Scan(&status)

		if err == sql.ErrNoRows {
			// Workflow doesn't exist - inconsistency detected
			m.mu.RUnlock()
			m.autoCleanupActiveWorkflow(ctx, id, "workflow not found in database")
			return "", nil
		}
		if err != nil {
			// Query error - return the ID anyway (don't break on transient errors)
			m.mu.RUnlock()
			//nolint:nilerr // Status lookup failure shouldn't block access to the active workflow ID.
			return core.WorkflowID(id), nil
		}

		// Check for terminal states: failed and completed workflows should not remain active
		if status == string(core.WorkflowStatusFailed) || status == string(core.WorkflowStatusCompleted) {
			m.mu.RUnlock()
			m.autoCleanupActiveWorkflow(ctx, id, fmt.Sprintf("workflow in terminal state: %s", status))
			return "", nil
		}
	}

	m.mu.RUnlock()
	return core.WorkflowID(id), nil
}

// autoCleanupActiveWorkflow cleans up the active_workflow table when inconsistency is detected.
// This is a defensive measure to prevent ghost workflows from blocking the system.
func (m *SQLiteStateManager) autoCleanupActiveWorkflow(ctx context.Context, workflowID, reason string) {
	// Log the inconsistency (using fmt since there's no logger on struct)
	fmt.Printf("WARN: auto-cleaning inconsistent active_workflow: %s (reason: %s)\n", workflowID, reason)

	// Attempt to clean up
	if err := m.DeactivateWorkflow(ctx); err != nil {
		fmt.Printf("ERROR: failed to auto-cleanup active_workflow %s: %v\n", workflowID, err)
	}
}

// cleanupOnStartup performs consistency checks and fixes on startup.
// This cleans up ghost workflows that may have been left from crashes or bugs.
func (m *SQLiteStateManager) cleanupOnStartup(ctx context.Context) error {
	// Check if active_workflow points to a terminal workflow
	var activeID string
	err := m.readDB.QueryRowContext(ctx,
		"SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)

	if err == sql.ErrNoRows {
		return nil // No active workflow, nothing to clean
	}
	if err != nil {
		return fmt.Errorf("checking active workflow: %w", err)
	}

	// Check workflow status
	var status string
	err = m.readDB.QueryRowContext(ctx,
		"SELECT status FROM workflows WHERE id = ?", activeID).Scan(&status)

	if err == sql.ErrNoRows {
		// Workflow doesn't exist - clean up orphan reference
		fmt.Printf("INFO: cleaning up orphan active_workflow on startup (workflow %s not found)\n", activeID)
		return m.DeactivateWorkflow(ctx)
	}
	if err != nil {
		return fmt.Errorf("checking workflow status: %w", err)
	}

	// If terminal status, clean up
	if status == string(core.WorkflowStatusFailed) || status == string(core.WorkflowStatusCompleted) {
		fmt.Printf("INFO: cleaning up ghost active_workflow on startup (workflow %s in %s state)\n", activeID, status)
		return m.DeactivateWorkflow(ctx)
	}

	return nil
}

// FindWorkflowsByPrompt finds workflows with the same prompt hash.
// Returns workflows that match the given prompt, useful for duplicate detection.
func (m *SQLiteStateManager) FindWorkflowsByPrompt(ctx context.Context, prompt string) ([]core.DuplicateWorkflowInfo, error) {
	if prompt == "" {
		return nil, nil
	}

	// Calculate prompt hash
	h := sha256.Sum256([]byte(prompt))
	promptHash := hex.EncodeToString(h[:])

	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.readDB.QueryContext(ctx, `
		SELECT id, status, created_at, COALESCE(title, '') as title
		FROM workflows
		WHERE prompt_hash = ?
		ORDER BY created_at DESC
	`, promptHash)
	if err != nil {
		return nil, fmt.Errorf("querying workflows by prompt hash: %w", err)
	}
	defer rows.Close()

	var results []core.DuplicateWorkflowInfo
	for rows.Next() {
		var info core.DuplicateWorkflowInfo
		var status string
		if err := rows.Scan(&info.WorkflowID, &status, &info.CreatedAt, &info.Title); err != nil {
			return nil, fmt.Errorf("scanning workflow row: %w", err)
		}
		info.Status = core.WorkflowStatus(status)
		results = append(results, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workflow rows: %w", err)
	}

	return results, nil
}

// SetActiveWorkflowID sets the active workflow ID.
func (m *SQLiteStateManager) SetActiveWorkflowID(ctx context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "setting active workflow ID", func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO active_workflow (id, workflow_id, updated_at)
			VALUES (1, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				workflow_id = excluded.workflow_id,
				updated_at = excluded.updated_at
		`, id, time.Now())
		return err
	})
}

// AcquireLock obtains an exclusive lock on the state.
// Uses SQLite's built-in locking with a mutex for in-process coordination.
// The transaction is stored and reused by Save() to avoid nested transactions.
func (m *SQLiteStateManager) AcquireLock(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureWithinStateDir(m.lockPath); err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(m.lockPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	// Check for existing lock
	if data, err := fsutil.ReadFileScoped(m.lockPath); err == nil {
		var info lockInfo
		if err := json.Unmarshal(data, &info); err == nil {
			// Check if lock is stale
			if time.Since(info.AcquiredAt) < m.lockTTL {
				// Check if process is still alive
				if processExists(info.PID) {
					return core.ErrState("LOCK_ACQUIRE_FAILED",
						fmt.Sprintf("lock held by PID %d since %s", info.PID, info.AcquiredAt))
				}
			}
			// Stale lock, remove it
			if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing stale lock: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading lock file: %w", err)
	}

	// Create lock file
	hostname, _ := os.Hostname()
	info := lockInfo{
		PID:        os.Getpid(),
		Hostname:   hostname,
		AcquiredAt: time.Now(),
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshaling lock info: %w", err)
	}

	// Write lock file exclusively
	// #nosec G304 -- lockPath validated to be within state directory
	f, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return core.ErrState("LOCK_ACQUIRE_FAILED", "lock file created by another process")
		}
		return fmt.Errorf("creating lock file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		if rmErr := os.Remove(m.lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("writing lock file: %w (cleanup failed: %v)", err, rmErr)
		}
		return fmt.Errorf("writing lock file: %w", err)
	}
	if err := f.Close(); err != nil {
		if rmErr := os.Remove(m.lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("closing lock file: %w (cleanup failed: %v)", err, rmErr)
		}
		return fmt.Errorf("closing lock file: %w", err)
	}

	return nil
}

// ReleaseLock releases the exclusive lock.
func (m *SQLiteStateManager) ReleaseLock(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureWithinStateDir(m.lockPath); err != nil {
		return err
	}

	// Verify we own the lock
	data, err := fsutil.ReadFileScoped(m.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already released
		}
		return fmt.Errorf("reading lock file: %w", err)
	}

	var info lockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("parsing lock info: %w", err)
	}

	if info.PID != os.Getpid() {
		return core.ErrState("LOCK_RELEASE_FAILED", "lock owned by different process")
	}

	if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing lock file: %w", err)
	}

	return nil
}

// AcquireWorkflowLock acquires an exclusive lock for a specific workflow.
// Uses database-backed locking with stale lock detection.
func (m *SQLiteStateManager) AcquireWorkflowLock(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid := os.Getpid()
	hostname, _ := os.Hostname()
	now := time.Now().UTC()
	expiresAt := now.Add(m.lockTTL)

	return m.retryWrite(ctx, "acquire_workflow_lock", func() error {
		// Check for existing lock
		var existingPID sql.NullInt64
		var existingHost sql.NullString
		var existingExpires sql.NullTime

		err := m.db.QueryRowContext(ctx, `
			SELECT holder_pid, holder_host, expires_at
			FROM workflow_locks
			WHERE workflow_id = ?
		`, string(workflowID)).Scan(&existingPID, &existingHost, &existingExpires)

		if err == nil {
			// Lock exists - check if stale
			if existingExpires.Valid && existingExpires.Time.Before(now) {
				// Lock expired - delete it
				_, _ = m.db.ExecContext(ctx, `
					DELETE FROM workflow_locks WHERE workflow_id = ?
				`, string(workflowID))
			} else if existingPID.Valid {
				// Check if process still alive
				if sqliteProcessExists(int(existingPID.Int64)) {
					return core.ErrState("WORKFLOW_LOCK_HELD",
						fmt.Sprintf("workflow %s locked by process %d on %s",
							workflowID, existingPID.Int64, existingHost.String))
				}
				// Process dead - delete stale lock
				_, _ = m.db.ExecContext(ctx, `
					DELETE FROM workflow_locks WHERE workflow_id = ?
				`, string(workflowID))
			}
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("checking existing lock: %w", err)
		}

		// Try to acquire lock (PRIMARY KEY constraint handles race conditions)
		_, err = m.db.ExecContext(ctx, `
			INSERT INTO workflow_locks (workflow_id, holder_pid, holder_host, acquired_at, expires_at)
			VALUES (?, ?, ?, ?, ?)
		`, string(workflowID), pid, hostname, now, expiresAt)

		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
				strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
				return core.ErrState("WORKFLOW_LOCK_HELD",
					fmt.Sprintf("workflow %s is already locked", workflowID))
			}
			return fmt.Errorf("inserting lock: %w", err)
		}

		return nil
	})
}

// ReleaseWorkflowLock releases the lock for a specific workflow.
// Only releases if the current process holds the lock.
func (m *SQLiteStateManager) ReleaseWorkflowLock(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid := os.Getpid()

	return m.retryWrite(ctx, "release_workflow_lock", func() error {
		result, err := m.db.ExecContext(ctx, `
			DELETE FROM workflow_locks
			WHERE workflow_id = ? AND holder_pid = ?
		`, string(workflowID), pid)

		if err != nil {
			return fmt.Errorf("deleting lock: %w", err)
		}

		rows, _ := result.RowsAffected()
		_ = rows // Lock didn't exist or wasn't ours; ignore.

		return nil
	})
}

// RefreshWorkflowLock extends the lock expiration time for a workflow.
// Returns error if the lock is not held by this process.
func (m *SQLiteStateManager) RefreshWorkflowLock(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid := os.Getpid()
	now := time.Now().UTC()
	expiresAt := now.Add(m.lockTTL)

	return m.retryWrite(ctx, "refresh_workflow_lock", func() error {
		result, err := m.db.ExecContext(ctx, `
			UPDATE workflow_locks
			SET expires_at = ?, acquired_at = ?
			WHERE workflow_id = ? AND holder_pid = ?
		`, expiresAt, now, string(workflowID), pid)

		if err != nil {
			return fmt.Errorf("refreshing lock: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return core.ErrState("LOCK_NOT_HELD",
				fmt.Sprintf("workflow %s lock not held by this process", workflowID))
		}

		return nil
	})
}

// SetWorkflowRunning marks a workflow as currently executing.
func (m *SQLiteStateManager) SetWorkflowRunning(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid := os.Getpid()
	hostname, _ := os.Hostname()
	now := time.Now().UTC()

	return m.retryWrite(ctx, "set_workflow_running", func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO running_workflows
			(workflow_id, started_at, lock_holder_pid, lock_holder_host, heartbeat_at)
			VALUES (?, ?, ?, ?, ?)
		`, string(workflowID), now, pid, hostname, now)

		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
				strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
				return core.ErrState("WORKFLOW_ALREADY_RUNNING",
					fmt.Sprintf("workflow %s is already marked as running", workflowID)).WithCause(err)
			}
			return fmt.Errorf("setting workflow running: %w", err)
		}

		return nil
	})
}

// ClearWorkflowRunning removes a workflow from the running state.
func (m *SQLiteStateManager) ClearWorkflowRunning(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "clear_workflow_running", func() error {
		_, err := m.db.ExecContext(ctx, `
			DELETE FROM running_workflows WHERE workflow_id = ?
		`, string(workflowID))

		if err != nil {
			return fmt.Errorf("clearing workflow running: %w", err)
		}

		return nil
	})
}

// ListRunningWorkflows returns IDs of all currently executing workflows.
func (m *SQLiteStateManager) ListRunningWorkflows(ctx context.Context) ([]core.WorkflowID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.readDB.QueryContext(ctx, `
		SELECT workflow_id FROM running_workflows ORDER BY started_at
	`)
	if err != nil {
		return nil, fmt.Errorf("querying running workflows: %w", err)
	}
	defer rows.Close()

	var ids []core.WorkflowID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning workflow id: %w", err)
		}
		ids = append(ids, core.WorkflowID(id))
	}

	return ids, rows.Err()
}

// GetRunningWorkflowRecord returns the running_workflows row for a given workflow ID.
// Returns (nil, nil) when the workflow is not marked as running.
func (m *SQLiteStateManager) GetRunningWorkflowRecord(ctx context.Context, workflowID core.WorkflowID) (*core.RunningWorkflowRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var id string
	var startedAt sql.NullTime
	var lockPID sql.NullInt64
	var lockHost sql.NullString
	var heartbeatAt sql.NullTime

	err := m.readDB.QueryRowContext(ctx, `
		SELECT workflow_id, started_at, lock_holder_pid, lock_holder_host, heartbeat_at
		FROM running_workflows
		WHERE workflow_id = ?
	`, string(workflowID)).Scan(&id, &startedAt, &lockPID, &lockHost, &heartbeatAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying running_workflows: %w", err)
	}

	var pidPtr *int
	if lockPID.Valid {
		p := int(lockPID.Int64)
		pidPtr = &p
	}

	var heartbeatPtr *time.Time
	if heartbeatAt.Valid {
		hb := heartbeatAt.Time
		heartbeatPtr = &hb
	}

	record := &core.RunningWorkflowRecord{
		WorkflowID:     core.WorkflowID(id),
		LockHolderPID:  pidPtr,
		LockHolderHost: lockHost.String,
		HeartbeatAt:    heartbeatPtr,
	}
	if startedAt.Valid {
		record.StartedAt = startedAt.Time
	}

	return record, nil
}

// IsWorkflowRunning checks if a specific workflow is currently executing.
func (m *SQLiteStateManager) IsWorkflowRunning(ctx context.Context, workflowID core.WorkflowID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	err := m.readDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM running_workflows WHERE workflow_id = ?
	`, string(workflowID)).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("checking if workflow running: %w", err)
	}

	return count > 0, nil
}

// UpdateWorkflowHeartbeat updates the heartbeat timestamp for a running workflow.
func (m *SQLiteStateManager) UpdateWorkflowHeartbeat(ctx context.Context, workflowID core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	return m.retryWrite(ctx, "update_workflow_heartbeat", func() error {
		result, err := m.db.ExecContext(ctx, `
			UPDATE running_workflows
			SET heartbeat_at = ?
			WHERE workflow_id = ?
		`, now, string(workflowID))

		if err != nil {
			return fmt.Errorf("updating heartbeat: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			// Workflow not in running_workflows, add it
			pid := os.Getpid()
			hostname, _ := os.Hostname()
			_, err := m.db.ExecContext(ctx, `
				INSERT INTO running_workflows
				(workflow_id, started_at, lock_holder_pid, lock_holder_host, heartbeat_at)
				VALUES (?, ?, ?, ?, ?)
			`, string(workflowID), now, pid, hostname, now)
			if err != nil {
				return fmt.Errorf("inserting running workflow: %w", err)
			}
		}

		return nil
	})
}

// sqliteProcessExists checks if a process is running.
// This is a local copy for SQLite state manager to avoid import cycles.
func sqliteProcessExists(pid int) bool {
	// Windows reports no access when signaling the current process; treat that as existing.
	if runtime.GOOS == "windows" && pid == os.Getpid() {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Exists checks if the database file exists and has data.
func (m *SQLiteStateManager) Exists() bool {
	info, err := os.Stat(m.dbPath)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// Backup creates a backup of the current database.
func (m *SQLiteStateManager) Backup(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureWithinStateDir(m.backupPath); err != nil {
		return err
	}

	// Use SQLite's backup API via VACUUM INTO
	// Prefer parameterized SQL to avoid building SQL with string formatting.
	_, err := m.db.ExecContext(ctx, "VACUUM INTO ?", m.backupPath)
	if err != nil {
		// Fallback to file copy if VACUUM INTO not supported
		return m.copyFile(m.dbPath, m.backupPath)
	}
	return nil
}

func (m *SQLiteStateManager) copyFile(src, dst string) error {
	if err := m.ensureWithinStateDir(src); err != nil {
		return err
	}
	if err := m.ensureWithinStateDir(dst); err != nil {
		return err
	}
	// #nosec G304 -- src path validated to be within state directory
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	// #nosec G304 -- dst path validated to be within state directory
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return dstFile.Sync()
}

func (m *SQLiteStateManager) ensureWithinStateDir(path string) error {
	baseDir := filepath.Dir(m.dbPath)
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolving state directory: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes state directory")
	}
	return nil
}

// Restore restores from the most recent backup.
func (m *SQLiteStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if backup exists
	if _, err := os.Stat(m.backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no backup file found at %s", m.backupPath)
	}

	// Close current database connections
	if m.readDB != nil {
		if err := m.readDB.Close(); err != nil {
			return nil, fmt.Errorf("closing read database: %w", err)
		}
	}
	if err := m.db.Close(); err != nil {
		return nil, fmt.Errorf("closing database: %w", err)
	}

	// Copy backup to main database
	if err := m.copyFile(m.backupPath, m.dbPath); err != nil {
		return nil, fmt.Errorf("restoring backup: %w", err)
	}

	// Reopen write connection with same settings
	db, err := sql.Open("sqlite", m.dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("reopening write database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	m.db = db

	// Reopen read connection
	readDB, err := sql.Open("sqlite", m.dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&mode=ro&_pragma=busy_timeout(1000)")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("reopening read database: %w", err)
	}
	readDB.SetMaxOpenConns(10)
	readDB.SetMaxIdleConns(5)
	readDB.SetConnMaxLifetime(5 * time.Minute)
	m.readDB = readDB

	// Load active workflow - query active ID and load directly without lock dance
	// (we already hold the write lock, so calling Load() would deadlock on RLock)
	var activeID string
	err = m.readDB.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)
	if err == sql.ErrNoRows {
		return nil, nil // No active workflow
	}
	if err != nil {
		return nil, fmt.Errorf("getting active workflow ID after restore: %w", err)
	}

	return m.loadWorkflowByID(ctx, core.WorkflowID(activeID))
}

// Helper functions for nullable values

func nullableString(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func nullableTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// DeactivateWorkflow clears the active workflow without deleting any data.
func (m *SQLiteStateManager) DeactivateWorkflow(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "deactivating workflow", func() error {
		_, err := m.db.ExecContext(ctx, "DELETE FROM active_workflow WHERE id = 1")
		return err
	})
}

// ArchiveWorkflows preserves completed/failed workflows by exporting them to a JSON archive
// and then removing them from the SQLite database (so they no longer appear in lists).
//
// "archive" is non-destructive (data is retained on disk), while PurgeAllWorkflows()
// is the destructive operation.
//
// Returns the number of workflows archived.
func (m *SQLiteStateManager) ArchiveWorkflows(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get active workflow ID to avoid archiving it
	var activeID sql.NullString
	_ = m.db.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)

	// Determine which workflows will be archived.
	query := `SELECT id FROM workflows WHERE (status = 'completed' OR status = 'failed')`
	args := []interface{}{}
	if activeID.Valid {
		query += " AND id != ?"
		args = append(args, activeID.String)
	}

	rows, err := m.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("listing workflows to archive: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scanning workflow id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating workflows to archive: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// Ensure archive directory exists (next to the DB).
	archiveDir := filepath.Join(filepath.Dir(m.dbPath), "archive")
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return 0, fmt.Errorf("creating archive directory: %w", err)
	}

	// Export workflows to JSON files first. Only delete from DB if all exports succeed.
	for _, id := range ids {
		state, err := m.loadWorkflowByID(ctx, core.WorkflowID(id))
		if err != nil {
			return 0, fmt.Errorf("loading workflow %s for archive: %w", id, err)
		}
		if state == nil {
			continue
		}

		// Clear checksum for checksum computation and consistent envelope.
		state.Checksum = ""
		stateBytes, err := json.Marshal(state)
		if err != nil {
			return 0, fmt.Errorf("marshaling workflow %s for archive checksum: %w", id, err)
		}
		hash := sha256.Sum256(stateBytes)
		checksum := hex.EncodeToString(hash[:])

		envelope := stateEnvelope{
			Version:   1,
			Checksum:  checksum,
			UpdatedAt: time.Now(),
			State:     state,
		}

		data, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return 0, fmt.Errorf("marshaling workflow %s archive envelope: %w", id, err)
		}

		archivePath := filepath.Join(archiveDir, id+".json")
		if err := atomicWriteFile(archivePath, data, 0o600); err != nil {
			return 0, fmt.Errorf("writing workflow %s archive file: %w", id, err)
		}
	}

	// Delete archived workflows and their related data from the DB.
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning archive delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, "DELETE FROM tasks WHERE workflow_id = ?", id); err != nil {
			return 0, fmt.Errorf("deleting archived tasks for workflow %s: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM checkpoints WHERE workflow_id = ?", id); err != nil {
			return 0, fmt.Errorf("deleting archived checkpoints for workflow %s: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM workflows WHERE id = ?", id); err != nil {
			return 0, fmt.Errorf("deleting archived workflow %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing archive delete transaction: %w", err)
	}

	return len(ids), nil
}

// PurgeAllWorkflows deletes all workflow data permanently.
// Returns the number of workflows deleted.
func (m *SQLiteStateManager) PurgeAllWorkflows(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Count workflows before deletion
	var count int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflows").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting workflows: %w", err)
	}

	// Delete in order due to foreign keys
	if _, err := m.db.ExecContext(ctx, "DELETE FROM checkpoints"); err != nil {
		return 0, fmt.Errorf("deleting checkpoints: %w", err)
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM tasks"); err != nil {
		return 0, fmt.Errorf("deleting tasks: %w", err)
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM workflows"); err != nil {
		return 0, fmt.Errorf("deleting workflows: %w", err)
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM active_workflow"); err != nil {
		return 0, fmt.Errorf("deleting active workflow: %w", err)
	}

	return count, nil
}

// DeleteWorkflow deletes a single workflow by ID.
// Returns error if workflow does not exist.
func (m *SQLiteStateManager) DeleteWorkflow(ctx context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get report_path before deleting (for cleanup)
	var reportPath sql.NullString
	_ = m.db.QueryRowContext(ctx, "SELECT report_path FROM workflows WHERE id = ?", string(id)).Scan(&reportPath)

	err := m.retryWrite(ctx, "deleting workflow", func() error {
		// Check workflow exists
		var exists int
		err := m.db.QueryRowContext(ctx, "SELECT 1 FROM workflows WHERE id = ?", string(id)).Scan(&exists)
		if err != nil {
			return fmt.Errorf("workflow not found: %s", id)
		}

		// Delete checkpoints for tasks belonging to this workflow
		_, err = m.db.ExecContext(ctx, `
			DELETE FROM checkpoints
			WHERE task_id IN (SELECT id FROM tasks WHERE workflow_id = ?)
		`, string(id))
		if err != nil {
			return fmt.Errorf("deleting checkpoints: %w", err)
		}

		// Delete tasks
		_, err = m.db.ExecContext(ctx, "DELETE FROM tasks WHERE workflow_id = ?", string(id))
		if err != nil {
			return fmt.Errorf("deleting tasks: %w", err)
		}

		// Delete workflow
		_, err = m.db.ExecContext(ctx, "DELETE FROM workflows WHERE id = ?", string(id))
		if err != nil {
			return fmt.Errorf("deleting workflow: %w", err)
		}

		// Clear active workflow if it matches
		_, err = m.db.ExecContext(ctx, "DELETE FROM active_workflow WHERE workflow_id = ?", string(id))
		if err != nil {
			return fmt.Errorf("clearing active workflow: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Delete report directory (best effort, don't fail if it doesn't exist)
	m.deleteReportDirectory(reportPath.String, string(id))

	return nil
}

// deleteReportDirectory removes the workflow's report directory.
// It tries the stored reportPath first, then falls back to default location.
// Paths are sanitized to prevent traversal outside .quorum/.
func (m *SQLiteStateManager) deleteReportDirectory(reportPath, workflowID string) {
	// Try stored report path first (sanitized)
	if reportPath != "" {
		if strings.HasPrefix(filepath.Clean(reportPath), ".quorum"+string(filepath.Separator)) {
			if err := os.RemoveAll(filepath.Clean(reportPath)); err == nil {
				return
			}
		}
	}

	// Fall back to default location: .quorum/runs/{workflow-id}
	// Use filepath.Base to strip any traversal from the ID
	safeID := filepath.Base(workflowID)
	defaultPath := filepath.Join(".quorum", "runs", safeID)
	_ = os.RemoveAll(defaultPath)
}

// UpdateHeartbeat updates the heartbeat timestamp for a running workflow.
// This is used for zombie detection - workflows with stale heartbeats are considered dead.
// IMPORTANT: Updates running_workflows.heartbeat_at (which FindZombieWorkflows queries),
// and also workflows.heartbeat_at for compatibility.
func (m *SQLiteStateManager) UpdateHeartbeat(ctx context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	return m.retryWrite(ctx, "updating heartbeat", func() error {
		// Update running_workflows (the table that FindZombieWorkflows queries)
		result, err := m.db.ExecContext(ctx,
			`UPDATE running_workflows SET heartbeat_at = ? WHERE workflow_id = ?`,
			now, string(id))
		if err != nil {
			return fmt.Errorf("updating heartbeat: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return fmt.Errorf("workflow not in running_workflows: %s", id)
		}

		// Also update workflows.heartbeat_at for backward compatibility
		_, _ = m.db.ExecContext(ctx,
			`UPDATE workflows SET heartbeat_at = ? WHERE id = ?`,
			now, string(id))

		return nil
	})
}

// FindZombieWorkflows returns workflows with status "running" but stale heartbeats.
// A workflow is considered a zombie if its heartbeat is older than the threshold.
func (m *SQLiteStateManager) FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().UTC().Add(-staleThreshold)

	// Find workflows that are in running_workflows table but have stale heartbeats.
	// This uses the new running_workflows table for tracking active workflows.
	// Also include workflows with NULL heartbeat_at (never had a heartbeat).
	rows, err := m.readDB.QueryContext(ctx, `
		SELECT w.id
		FROM workflows w
		INNER JOIN running_workflows rw ON w.id = rw.workflow_id
		WHERE (rw.heartbeat_at IS NULL OR rw.heartbeat_at < ?)
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying zombie workflows: %w", err)
	}
	defer rows.Close()

	var zombies []*core.WorkflowState
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning workflow id: %w", err)
		}

		// Load full workflow state
		state, err := m.loadWorkflowByID(ctx, core.WorkflowID(id))
		if err != nil {
			continue // Skip workflows that fail to load
		}
		if state != nil {
			zombies = append(zombies, state)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating zombie workflows: %w", err)
	}

	return zombies, nil
}

// Verify that SQLiteStateManager implements core.StateManager.
var _ core.StateManager = (*SQLiteStateManager)(nil)

// ============================================================================
// Kanban State Management Methods
// ============================================================================

// GetNextKanbanWorkflow returns the next workflow from the "todo" column
// ordered by position (lowest first). Returns nil if no workflows in todo.
func (m *SQLiteStateManager) GetNextKanbanWorkflow(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get the workflow ID with the lowest position in the "todo" column
	var workflowID string
	err := m.readDB.QueryRowContext(ctx, `
		SELECT id FROM workflows
		WHERE kanban_column = 'todo'
		ORDER BY kanban_position ASC, created_at ASC
		LIMIT 1
	`).Scan(&workflowID)

	if err == sql.ErrNoRows {
		return nil, nil // No workflows in todo
	}
	if err != nil {
		return nil, fmt.Errorf("querying next kanban workflow: %w", err)
	}

	return m.loadWorkflowByID(ctx, core.WorkflowID(workflowID))
}

// MoveWorkflow moves a workflow to a new column with a new position.
func (m *SQLiteStateManager) MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "move_workflow", func() error {
		// Update the workflow's kanban column and position
		result, err := m.db.ExecContext(ctx, `
			UPDATE workflows
			SET kanban_column = ?, kanban_position = ?, updated_at = ?
			WHERE id = ?
		`, toColumn, position, time.Now(), workflowID)

		if err != nil {
			return fmt.Errorf("moving workflow: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}

		return nil
	})
}

// UpdateKanbanStatus updates the Kanban-specific fields on a workflow.
func (m *SQLiteStateManager) UpdateKanbanStatus(ctx context.Context, workflowID, column, prURL string, prNumber int, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "update_kanban_status", func() error {
		now := time.Now()

		// Build the update query dynamically based on which fields are set
		result, err := m.db.ExecContext(ctx, `
			UPDATE workflows
			SET kanban_column = ?,
			    pr_url = CASE WHEN ? != '' THEN ? ELSE pr_url END,
			    pr_number = CASE WHEN ? > 0 THEN ? ELSE pr_number END,
			    kanban_last_error = ?,
			    kanban_completed_at = CASE WHEN ? IN ('to_verify', 'done') THEN ? ELSE kanban_completed_at END,
			    kanban_execution_count = kanban_execution_count + CASE WHEN ? IN ('to_verify', 'refinement') THEN 1 ELSE 0 END,
			    updated_at = ?
			WHERE id = ?
		`, column, prURL, prURL, prNumber, prNumber, lastError, column, now, column, now, workflowID)

		if err != nil {
			return fmt.Errorf("updating kanban status: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}

		return nil
	})
}

// GetKanbanEngineState retrieves the persisted engine state from the singleton table.
func (m *SQLiteStateManager) GetKanbanEngineState(ctx context.Context) (*kanban.KanbanEngineState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enabled, circuitBreakerOpen int
	var currentWorkflowID sql.NullString
	var consecutiveFailures int
	var lastFailureAt sql.NullTime

	err := m.readDB.QueryRowContext(ctx, `
		SELECT enabled, current_workflow_id, consecutive_failures, last_failure_at, circuit_breaker_open
		FROM kanban_engine_state
		WHERE id = 1
	`).Scan(&enabled, &currentWorkflowID, &consecutiveFailures, &lastFailureAt, &circuitBreakerOpen)

	if err == sql.ErrNoRows {
		// No state exists yet, return nil (use defaults)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying kanban engine state: %w", err)
	}

	state := &kanban.KanbanEngineState{
		Enabled:             enabled == 1,
		ConsecutiveFailures: consecutiveFailures,
		CircuitBreakerOpen:  circuitBreakerOpen == 1,
	}

	if currentWorkflowID.Valid {
		state.CurrentWorkflowID = &currentWorkflowID.String
	}
	if lastFailureAt.Valid {
		state.LastFailureAt = &lastFailureAt.Time
	}

	return state, nil
}

// SaveKanbanEngineState persists the engine state to the singleton table.
func (m *SQLiteStateManager) SaveKanbanEngineState(ctx context.Context, state *kanban.KanbanEngineState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "save_kanban_engine_state", func() error {
		enabled := 0
		if state.Enabled {
			enabled = 1
		}

		circuitBreakerOpen := 0
		if state.CircuitBreakerOpen {
			circuitBreakerOpen = 1
		}

		_, err := m.db.ExecContext(ctx, `
			INSERT INTO kanban_engine_state (id, enabled, current_workflow_id, consecutive_failures, last_failure_at, circuit_breaker_open, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				enabled = excluded.enabled,
				current_workflow_id = excluded.current_workflow_id,
				consecutive_failures = excluded.consecutive_failures,
				last_failure_at = excluded.last_failure_at,
				circuit_breaker_open = excluded.circuit_breaker_open,
				updated_at = excluded.updated_at
		`, enabled, nullableString([]byte(ptrToString(state.CurrentWorkflowID))), state.ConsecutiveFailures,
			nullableTime(state.LastFailureAt), circuitBreakerOpen, time.Now())

		if err != nil {
			return fmt.Errorf("saving kanban engine state: %w", err)
		}

		return nil
	})
}

// ListWorkflowsByKanbanColumn returns all workflows in a specific Kanban column, ordered by position.
func (m *SQLiteStateManager) ListWorkflowsByKanbanColumn(ctx context.Context, column string) ([]*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.readDB.QueryContext(ctx, `
		SELECT id FROM workflows
		WHERE kanban_column = ?
		ORDER BY kanban_position ASC, created_at ASC
	`, column)
	if err != nil {
		return nil, fmt.Errorf("querying workflows by kanban column: %w", err)
	}
	defer rows.Close()

	var workflows []*core.WorkflowState
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning workflow id: %w", err)
		}

		wf, err := m.loadWorkflowByID(ctx, core.WorkflowID(id))
		if err != nil {
			return nil, fmt.Errorf("loading workflow %s: %w", id, err)
		}
		if wf != nil {
			workflows = append(workflows, wf)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workflows: %w", err)
	}

	return workflows, nil
}

// GetKanbanBoard returns all workflows grouped by their Kanban column.
func (m *SQLiteStateManager) GetKanbanBoard(ctx context.Context) (map[string][]*core.WorkflowState, error) {
	columns := []string{"refinement", "todo", "in_progress", "to_verify", "done"}
	board := make(map[string][]*core.WorkflowState)

	for _, col := range columns {
		workflows, err := m.ListWorkflowsByKanbanColumn(ctx, col)
		if err != nil {
			return nil, fmt.Errorf("getting workflows for column %s: %w", col, err)
		}
		board[col] = workflows
	}

	return board, nil
}

// ptrToString safely converts a string pointer to string (empty if nil).
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ============================================================================
// Atomic Transaction Support
// ============================================================================

// sqliteAtomicContext implements core.AtomicStateContext for SQLite transactions.
type sqliteAtomicContext struct {
	tx  *sql.Tx
	ctx context.Context
	m   *SQLiteStateManager
}

// ExecuteAtomically runs operations atomically within a database transaction.
func (m *SQLiteStateManager) ExecuteAtomically(ctx context.Context, fn func(core.AtomicStateContext) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	atomicCtx := &sqliteAtomicContext{
		tx:  tx,
		ctx: ctx,
		m:   m,
	}

	if err := fn(atomicCtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// LoadByID retrieves a workflow state within the transaction.
//
//nolint:gocyclo // Complex aggregation mirrors database schema.
func (a *sqliteAtomicContext) LoadByID(id core.WorkflowID) (*core.WorkflowState, error) {
	var state core.WorkflowState
	var taskOrderJSON, blueprintJSON, metricsJSON sql.NullString
	var optimizedPrompt sql.NullString
	var checksum sql.NullString
	var reportPath sql.NullString
	var title sql.NullString
	var agentEventsJSON sql.NullString
	var workflowBranch sql.NullString
	var kanbanColumn, prURL, kanbanLastError sql.NullString
	var kanbanPosition, prNumber, kanbanExecutionCount sql.NullInt64
	var kanbanStartedAt, kanbanCompletedAt sql.NullTime

	err := a.tx.QueryRowContext(a.ctx, `
		SELECT id, version, title, status, current_phase, prompt, optimized_prompt,
		       task_order, blueprint, metrics, checksum, created_at, updated_at, report_path,
		       agent_events, workflow_branch,
		       kanban_column, kanban_position, pr_url, pr_number,
		       kanban_started_at, kanban_completed_at, kanban_execution_count, kanban_last_error
		FROM workflows WHERE id = ?
	`, id).Scan(
		&state.WorkflowID, &state.Version, &title, &state.Status, &state.CurrentPhase,
		&state.Prompt, &optimizedPrompt, &taskOrderJSON, &blueprintJSON, &metricsJSON,
		&checksum, &state.CreatedAt, &state.UpdatedAt, &reportPath, &agentEventsJSON, &workflowBranch,
		&kanbanColumn, &kanbanPosition, &prURL, &prNumber,
		&kanbanStartedAt, &kanbanCompletedAt, &kanbanExecutionCount, &kanbanLastError,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading workflow: %w", err)
	}

	if title.Valid {
		state.Title = title.String
	}
	if optimizedPrompt.Valid {
		state.OptimizedPrompt = optimizedPrompt.String
	}
	if checksum.Valid {
		state.Checksum = checksum.String
	}
	if reportPath.Valid {
		state.ReportPath = reportPath.String
	}
	if workflowBranch.Valid {
		state.WorkflowBranch = workflowBranch.String
	}
	if kanbanColumn.Valid {
		state.KanbanColumn = kanbanColumn.String
	}
	state.KanbanPosition = int(kanbanPosition.Int64)
	if prURL.Valid {
		state.PRURL = prURL.String
	}
	state.PRNumber = int(prNumber.Int64)
	if kanbanStartedAt.Valid {
		state.KanbanStartedAt = &kanbanStartedAt.Time
	}
	if kanbanCompletedAt.Valid {
		state.KanbanCompletedAt = &kanbanCompletedAt.Time
	}
	state.KanbanExecutionCount = int(kanbanExecutionCount.Int64)
	if kanbanLastError.Valid {
		state.KanbanLastError = kanbanLastError.String
	}

	if taskOrderJSON.Valid {
		if err := json.Unmarshal([]byte(taskOrderJSON.String), &state.TaskOrder); err != nil {
			return nil, fmt.Errorf("unmarshaling task order: %w", err)
		}
	}
	if blueprintJSON.Valid && blueprintJSON.String != "" {
		state.Blueprint = &core.Blueprint{}
		if err := json.Unmarshal([]byte(blueprintJSON.String), state.Blueprint); err != nil {
			return nil, fmt.Errorf("unmarshaling blueprint: %w", err)
		}
	}
	if metricsJSON.Valid && metricsJSON.String != "" {
		state.Metrics = &core.StateMetrics{}
		if err := json.Unmarshal([]byte(metricsJSON.String), state.Metrics); err != nil {
			return nil, fmt.Errorf("unmarshaling metrics: %w", err)
		}
	}
	if agentEventsJSON.Valid && agentEventsJSON.String != "" {
		if err := json.Unmarshal([]byte(agentEventsJSON.String), &state.AgentEvents); err != nil {
			return nil, fmt.Errorf("unmarshaling agent events: %w", err)
		}
	}

	// Load tasks within transaction
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	rows, err := a.tx.QueryContext(a.ctx, `
			SELECT id, phase, name, description, status, cli, model, dependencies,
			       tokens_in, tokens_out, retries, error,
			       worktree_path, started_at, completed_at, output,
			       output_file, model_used, finish_reason, tool_calls,
			       last_commit, files_modified, branch, resumable, resume_hint,
			       merge_pending, merge_commit
			FROM tasks WHERE workflow_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("loading tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		task, err := a.m.scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning task: %w", err)
		}
		state.Tasks[task.ID] = task
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tasks: %w", err)
	}

	// Load checkpoints within transaction
	cpRows, err := a.tx.QueryContext(a.ctx, `
		SELECT id, type, phase, task_id, timestamp, message, data
		FROM checkpoints WHERE workflow_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoints: %w", err)
	}
	defer cpRows.Close()

	for cpRows.Next() {
		cp, err := a.m.scanCheckpoint(cpRows)
		if err != nil {
			return nil, fmt.Errorf("scanning checkpoint: %w", err)
		}
		state.Checkpoints = append(state.Checkpoints, *cp)
	}
	if err := cpRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checkpoints: %w", err)
	}

	return &state, nil
}

// Save persists workflow state within the transaction.
func (a *sqliteAtomicContext) Save(state *core.WorkflowState) error {
	state.UpdatedAt = time.Now()

	// Calculate checksum
	state.Checksum = ""
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling state for checksum: %w", err)
	}
	hash := sha256.Sum256(stateBytes)
	checksum := hex.EncodeToString(hash[:])

	taskOrderJSON, err := json.Marshal(state.TaskOrder)
	if err != nil {
		return fmt.Errorf("marshaling task order: %w", err)
	}

	var blueprintJSON, metricsJSON []byte
	if state.Blueprint != nil {
		blueprintJSON, err = json.Marshal(state.Blueprint)
		if err != nil {
			return fmt.Errorf("marshaling blueprint: %w", err)
		}
	}
	if state.Metrics != nil {
		metricsJSON, err = json.Marshal(state.Metrics)
		if err != nil {
			return fmt.Errorf("marshaling metrics: %w", err)
		}
	}

	var agentEventsJSON []byte
	if len(state.AgentEvents) > 0 {
		agentEventsJSON, err = json.Marshal(state.AgentEvents)
		if err != nil {
			return fmt.Errorf("marshaling agent events: %w", err)
		}
	}

	_, err = a.tx.ExecContext(a.ctx, `
		INSERT INTO workflows (
			id, version, title, status, current_phase, prompt, optimized_prompt,
			task_order, blueprint, metrics, checksum, created_at, updated_at, report_path,
			agent_events, workflow_branch,
			kanban_column, kanban_position, pr_url, pr_number,
			kanban_started_at, kanban_completed_at, kanban_execution_count, kanban_last_error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			title = excluded.title,
			status = excluded.status,
			current_phase = excluded.current_phase,
			prompt = excluded.prompt,
			optimized_prompt = excluded.optimized_prompt,
			task_order = excluded.task_order,
			blueprint = excluded.blueprint,
			metrics = excluded.metrics,
			checksum = excluded.checksum,
			updated_at = excluded.updated_at,
			report_path = excluded.report_path,
			agent_events = excluded.agent_events,
			workflow_branch = excluded.workflow_branch,
			kanban_column = excluded.kanban_column,
			kanban_position = excluded.kanban_position,
			pr_url = excluded.pr_url,
			pr_number = excluded.pr_number,
			kanban_started_at = excluded.kanban_started_at,
			kanban_completed_at = excluded.kanban_completed_at,
			kanban_execution_count = excluded.kanban_execution_count,
			kanban_last_error = excluded.kanban_last_error
	`,
		state.WorkflowID, state.Version, state.Title, state.Status, state.CurrentPhase,
		state.Prompt, state.OptimizedPrompt, string(taskOrderJSON),
		nullableString(blueprintJSON), nullableString(metricsJSON),
		checksum, state.CreatedAt, state.UpdatedAt,
		nullableString([]byte(state.ReportPath)),
		nullableString(agentEventsJSON),
		nullableString([]byte(state.WorkflowBranch)),
		nullableString([]byte(state.KanbanColumn)), state.KanbanPosition,
		nullableString([]byte(state.PRURL)), state.PRNumber,
		nullableTime(state.KanbanStartedAt), nullableTime(state.KanbanCompletedAt),
		state.KanbanExecutionCount, nullableString([]byte(state.KanbanLastError)),
	)
	if err != nil {
		return fmt.Errorf("upserting workflow: %w", err)
	}

	// Delete existing tasks
	_, err = a.tx.ExecContext(a.ctx, "DELETE FROM tasks WHERE workflow_id = ?", state.WorkflowID)
	if err != nil {
		return fmt.Errorf("deleting existing tasks: %w", err)
	}

	// Insert tasks
	for _, task := range state.Tasks {
		if err := a.m.insertTask(a.ctx, a.tx, state.WorkflowID, task); err != nil {
			return fmt.Errorf("inserting task %s: %w", task.ID, err)
		}
	}

	// Delete existing checkpoints
	_, err = a.tx.ExecContext(a.ctx, "DELETE FROM checkpoints WHERE workflow_id = ?", state.WorkflowID)
	if err != nil {
		return fmt.Errorf("deleting existing checkpoints: %w", err)
	}

	// Insert checkpoints
	for _, cp := range state.Checkpoints {
		if err := a.m.insertCheckpoint(a.ctx, a.tx, state.WorkflowID, &cp); err != nil {
			return fmt.Errorf("inserting checkpoint %s: %w", cp.ID, err)
		}
	}

	// Set as active workflow
	_, err = a.tx.ExecContext(a.ctx, `
		INSERT INTO active_workflow (id, workflow_id, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			workflow_id = excluded.workflow_id,
			updated_at = excluded.updated_at
	`, state.WorkflowID, time.Now())
	if err != nil {
		return fmt.Errorf("setting active workflow: %w", err)
	}

	return nil
}

// SetWorkflowRunning marks a workflow as running within the transaction.
func (a *sqliteAtomicContext) SetWorkflowRunning(workflowID core.WorkflowID) error {
	pid := os.Getpid()
	hostname, _ := os.Hostname()
	now := time.Now().UTC()

	_, err := a.tx.ExecContext(a.ctx, `
		INSERT INTO running_workflows
		(workflow_id, started_at, lock_holder_pid, lock_holder_host, heartbeat_at)
		VALUES (?, ?, ?, ?, ?)
	`, string(workflowID), now, pid, hostname, now)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
			return core.ErrState("WORKFLOW_ALREADY_RUNNING",
				fmt.Sprintf("workflow %s is already marked as running", workflowID)).WithCause(err)
		}
		return fmt.Errorf("setting workflow running: %w", err)
	}

	return nil
}

// ClearWorkflowRunning removes a workflow from running state within the transaction.
func (a *sqliteAtomicContext) ClearWorkflowRunning(workflowID core.WorkflowID) error {
	_, err := a.tx.ExecContext(a.ctx, `
		DELETE FROM running_workflows WHERE workflow_id = ?
	`, string(workflowID))

	if err != nil {
		return fmt.Errorf("clearing workflow running: %w", err)
	}

	return nil
}

// IsWorkflowRunning checks if a workflow is running within the transaction.
func (a *sqliteAtomicContext) IsWorkflowRunning(workflowID core.WorkflowID) (bool, error) {
	var count int
	err := a.tx.QueryRowContext(a.ctx, `
		SELECT COUNT(*) FROM running_workflows WHERE workflow_id = ?
	`, string(workflowID)).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("checking if workflow running: %w", err)
	}

	return count > 0, nil
}
