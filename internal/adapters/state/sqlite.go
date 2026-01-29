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
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/fsutil"
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

	return nil
}

// Save persists workflow state atomically and sets it as the active workflow.
func (m *SQLiteStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update timestamp
	state.UpdatedAt = time.Now()

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

	var configJSON, metricsJSON []byte
	if state.Config != nil {
		configJSON, err = json.Marshal(state.Config)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
	}
	if state.Metrics != nil {
		metricsJSON, err = json.Marshal(state.Metrics)
		if err != nil {
			return fmt.Errorf("marshaling metrics: %w", err)
		}
	}

	// Upsert workflow
	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflows (
			id, version, title, status, current_phase, prompt, optimized_prompt,
			task_order, config, metrics, checksum, created_at, updated_at, report_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			title = excluded.title,
			status = excluded.status,
			current_phase = excluded.current_phase,
			prompt = excluded.prompt,
			optimized_prompt = excluded.optimized_prompt,
			task_order = excluded.task_order,
			config = excluded.config,
			metrics = excluded.metrics,
			checksum = excluded.checksum,
			updated_at = excluded.updated_at,
			report_path = excluded.report_path
	`,
		state.WorkflowID, state.Version, state.Title, state.Status, state.CurrentPhase,
		state.Prompt, state.OptimizedPrompt, string(taskOrderJSON),
		nullableString(configJSON), nullableString(metricsJSON),
		checksum, state.CreatedAt, state.UpdatedAt,
		nullableString([]byte(state.ReportPath)),
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

	// Convert bool to SQLite integer (0/1)
	resumableInt := 0
	if task.Resumable {
		resumableInt = 1
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (
			id, workflow_id, phase, name, status, cli, model,
			dependencies, tokens_in, tokens_out, cost_usd, retries,
			error, worktree_path, started_at, completed_at,
			output, output_file, model_used, finish_reason, tool_calls,
			last_commit, files_modified, branch, resumable, resume_hint
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		task.ID, workflowID, task.Phase, task.Name, task.Status,
		task.CLI, task.Model, string(depsJSON),
		task.TokensIn, task.TokensOut, task.CostUSD, task.Retries,
		nullableString([]byte(task.Error)), nullableString([]byte(task.WorktreePath)),
		nullableTime(task.StartedAt), nullableTime(task.CompletedAt),
		nullableString([]byte(task.Output)), nullableString([]byte(task.OutputFile)),
		nullableString([]byte(task.ModelUsed)), nullableString([]byte(task.FinishReason)),
		nullableString(toolCallsJSON),
		nullableString([]byte(task.LastCommit)), nullableString(filesModifiedJSON),
		nullableString([]byte(task.Branch)), resumableInt, nullableString([]byte(task.ResumeHint)),
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

func (m *SQLiteStateManager) loadWorkflowByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	// Load workflow using read connection
	var state core.WorkflowState
	var taskOrderJSON, configJSON, metricsJSON sql.NullString
	var optimizedPrompt sql.NullString
	var checksum sql.NullString
	var reportPath sql.NullString
	var title sql.NullString

	err := m.readDB.QueryRowContext(ctx, `
		SELECT id, version, title, status, current_phase, prompt, optimized_prompt,
		       task_order, config, metrics, checksum, created_at, updated_at, report_path
		FROM workflows WHERE id = ?
	`, id).Scan(
		&state.WorkflowID, &state.Version, &title, &state.Status, &state.CurrentPhase,
		&state.Prompt, &optimizedPrompt, &taskOrderJSON, &configJSON, &metricsJSON,
		&checksum, &state.CreatedAt, &state.UpdatedAt, &reportPath,
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

	// Parse JSON fields
	if taskOrderJSON.Valid {
		if err := json.Unmarshal([]byte(taskOrderJSON.String), &state.TaskOrder); err != nil {
			return nil, fmt.Errorf("unmarshaling task order: %w", err)
		}
	}
	if configJSON.Valid && configJSON.String != "" {
		state.Config = &core.WorkflowConfig{}
		if err := json.Unmarshal([]byte(configJSON.String), state.Config); err != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", err)
		}
	}
	if metricsJSON.Valid && metricsJSON.String != "" {
		state.Metrics = &core.StateMetrics{}
		if err := json.Unmarshal([]byte(metricsJSON.String), state.Metrics); err != nil {
			return nil, fmt.Errorf("unmarshaling metrics: %w", err)
		}
	}

	// Load tasks using read connection
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	rows, err := m.readDB.QueryContext(ctx, `
		SELECT id, phase, name, status, cli, model, dependencies,
		       tokens_in, tokens_out, cost_usd, retries, error,
		       worktree_path, started_at, completed_at, output,
		       output_file, model_used, finish_reason, tool_calls,
		       last_commit, files_modified, branch, resumable, resume_hint
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
	var cli, model, errorStr, worktreePath sql.NullString
	var startedAt, completedAt sql.NullTime
	var output, outputFile, modelUsed, finishReason sql.NullString
	var lastCommit, filesModifiedJSON, branch, resumeHint sql.NullString
	var resumable int

	err := rows.Scan(
		&task.ID, &task.Phase, &task.Name, &task.Status,
		&cli, &model, &depsJSON,
		&task.TokensIn, &task.TokensOut, &task.CostUSD, &task.Retries,
		&errorStr, &worktreePath, &startedAt, &completedAt,
		&output, &outputFile, &modelUsed, &finishReason, &toolCallsJSON,
		&lastCommit, &filesModifiedJSON, &branch, &resumable, &resumeHint,
	)
	if err != nil {
		return nil, err
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
func (m *SQLiteStateManager) GetActiveWorkflowID(ctx context.Context) (core.WorkflowID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use read connection for non-blocking reads
	var id string
	err := m.readDB.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting active workflow ID: %w", err)
	}
	return core.WorkflowID(id), nil
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
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		if rmErr := os.Remove(m.lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("writing lock file: %w (cleanup failed: %v)", err, rmErr)
		}
		return fmt.Errorf("writing lock file: %w", err)
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

	// Use SQLite's backup API via VACUUM INTO
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", m.backupPath))
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

	// Load active workflow
	m.mu.Unlock() // Temporarily unlock for Load
	defer m.mu.Lock()
	return m.Load(ctx)
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
// This mirrors the JSON backend semantics: "archive" is non-destructive (data is retained),
// while PurgeAllWorkflows() is the destructive operation.
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

	// Ensure archive directory exists (next to the DB, same as JSON backend).
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
func (m *SQLiteStateManager) deleteReportDirectory(reportPath, workflowID string) {
	// Try stored report path first
	if reportPath != "" {
		if err := os.RemoveAll(reportPath); err == nil {
			return
		}
	}

	// Fall back to default location: .quorum/runs/{workflow-id}
	defaultPath := filepath.Join(".quorum", "runs", workflowID)
	_ = os.RemoveAll(defaultPath)
}

// UpdateHeartbeat updates the heartbeat timestamp for a running workflow.
// This is used for zombie detection - workflows with stale heartbeats are considered dead.
func (m *SQLiteStateManager) UpdateHeartbeat(ctx context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.retryWrite(ctx, "updating heartbeat", func() error {
		result, err := m.db.ExecContext(ctx,
			`UPDATE workflows SET heartbeat_at = ? WHERE id = ? AND status = 'running'`,
			time.Now().UTC(), string(id))
		if err != nil {
			return fmt.Errorf("updating heartbeat: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return fmt.Errorf("workflow not found or not running: %s", id)
		}
		return nil
	})
}

// FindZombieWorkflows returns workflows with status "running" but stale heartbeats.
// A workflow is considered a zombie if its heartbeat is older than the threshold.
func (m *SQLiteStateManager) FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().UTC().Add(-staleThreshold)

	// Find workflows that are "running" but have stale heartbeats
	// Also include workflows with NULL heartbeat_at (never had a heartbeat)
	rows, err := m.readDB.QueryContext(ctx, `
		SELECT id FROM workflows
		WHERE status = 'running'
		AND (heartbeat_at IS NULL OR heartbeat_at < ?)
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
