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
	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial_schema.sql
var migrationV1 string

//go:embed migrations/002_add_recovery_columns.sql
var migrationV2 string

// SQLiteStateManager implements StateManager with SQLite storage.
type SQLiteStateManager struct {
	dbPath     string
	backupPath string
	db         *sql.DB
	mu         sync.RWMutex
	lockHeld   bool
	lockPID    int
}

// SQLiteStateManagerOption configures the manager.
type SQLiteStateManagerOption func(*SQLiteStateManager)

// NewSQLiteStateManager creates a new SQLite state manager.
func NewSQLiteStateManager(dbPath string, opts ...SQLiteStateManagerOption) (*SQLiteStateManager, error) {
	m := &SQLiteStateManager{
		dbPath:     dbPath,
		backupPath: dbPath + ".bak",
	}
	for _, opt := range opts {
		opt(m)
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	// Open database with WAL mode for better concurrency
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	m.db = db

	// Run migrations
	if err := m.migrate(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("running migrations: %w (close error: %v)", err, closeErr)
		}
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

// Close closes the database connection.
func (m *SQLiteStateManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
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

	// Begin transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
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
			id, version, status, current_phase, prompt, optimized_prompt,
			task_order, config, metrics, checksum, created_at, updated_at, report_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
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
		state.WorkflowID, state.Version, state.Status, state.CurrentPhase,
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

	// Commit transaction
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

	// Get active workflow ID
	var activeID string
	err := m.db.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)
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
	// Load workflow
	var state core.WorkflowState
	var taskOrderJSON, configJSON, metricsJSON sql.NullString
	var optimizedPrompt sql.NullString
	var checksum sql.NullString
	var reportPath sql.NullString

	err := m.db.QueryRowContext(ctx, `
		SELECT id, version, status, current_phase, prompt, optimized_prompt,
		       task_order, config, metrics, checksum, created_at, updated_at, report_path
		FROM workflows WHERE id = ?
	`, id).Scan(
		&state.WorkflowID, &state.Version, &state.Status, &state.CurrentPhase,
		&state.Prompt, &optimizedPrompt, &taskOrderJSON, &configJSON, &metricsJSON,
		&checksum, &state.CreatedAt, &state.UpdatedAt, &reportPath,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Workflow doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("loading workflow: %w", err)
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

	// Load tasks
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	rows, err := m.db.QueryContext(ctx, `
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

	// Load checkpoints
	cpRows, err := m.db.QueryContext(ctx, `
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

	// Get active workflow ID
	var activeID sql.NullString
	_ = m.db.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)

	rows, err := m.db.QueryContext(ctx, `
		SELECT id, status, current_phase, prompt, created_at, updated_at
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
		err := rows.Scan(&s.WorkflowID, &s.Status, &s.CurrentPhase, &s.Prompt, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning workflow summary: %w", err)
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

	var id string
	err := m.db.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&id)
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

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO active_workflow (id, workflow_id, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			workflow_id = excluded.workflow_id,
			updated_at = excluded.updated_at
	`, id, time.Now())
	if err != nil {
		return fmt.Errorf("setting active workflow ID: %w", err)
	}
	return nil
}

// AcquireLock obtains an exclusive lock on the state.
// Uses SQLite's built-in locking with a mutex for in-process coordination.
func (m *SQLiteStateManager) AcquireLock(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.lockHeld {
		return fmt.Errorf("lock already held by this process")
	}

	// SQLite handles file-level locking automatically with WAL mode
	// We use an exclusive transaction to ensure atomicity
	_, err := m.db.ExecContext(ctx, "BEGIN EXCLUSIVE")
	if err != nil {
		return fmt.Errorf("acquiring exclusive lock: %w", err)
	}

	m.lockHeld = true
	m.lockPID = os.Getpid()
	return nil
}

// ReleaseLock releases the exclusive lock.
func (m *SQLiteStateManager) ReleaseLock(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.lockHeld {
		return nil // No lock to release
	}

	_, err := m.db.ExecContext(ctx, "COMMIT")
	if err != nil {
		// Try rollback if commit fails (ignore rollback error)
		_, _ = m.db.ExecContext(ctx, "ROLLBACK")
	}

	m.lockHeld = false
	m.lockPID = 0
	return err
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

	// Close current database
	if err := m.db.Close(); err != nil {
		return nil, fmt.Errorf("closing database: %w", err)
	}

	// Copy backup to main database
	if err := m.copyFile(m.backupPath, m.dbPath); err != nil {
		return nil, fmt.Errorf("restoring backup: %w", err)
	}

	// Reopen database
	db, err := sql.Open("sqlite", m.dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("reopening database: %w", err)
	}
	m.db = db

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

	_, err := m.db.ExecContext(ctx, "DELETE FROM active_workflow WHERE id = 1")
	if err != nil {
		return fmt.Errorf("deactivating workflow: %w", err)
	}
	return nil
}

// ArchiveWorkflows moves completed workflows to an archived state.
// For SQLite, we use a status flag since we can't move rows to another database easily.
// Returns the number of workflows archived.
func (m *SQLiteStateManager) ArchiveWorkflows(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get active workflow ID to avoid archiving it
	var activeID sql.NullString
	_ = m.db.QueryRowContext(ctx, "SELECT workflow_id FROM active_workflow WHERE id = 1").Scan(&activeID)

	// Count workflows that will be archived
	var count int
	query := `
		SELECT COUNT(*) FROM workflows
		WHERE (status = 'completed' OR status = 'failed')
	`
	args := []interface{}{}

	if activeID.Valid {
		query += " AND id != ?"
		args = append(args, activeID.String)
	}

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting workflows to archive: %w", err)
	}

	if count == 0 {
		return 0, nil
	}

	// Delete archived workflows and their related data
	// First delete tasks and checkpoints
	deleteQuery := `
		DELETE FROM tasks WHERE workflow_id IN (
			SELECT id FROM workflows
			WHERE (status = 'completed' OR status = 'failed')
	`
	if activeID.Valid {
		deleteQuery += " AND id != ?"
	}
	deleteQuery += ")"

	if activeID.Valid {
		_, err = m.db.ExecContext(ctx, deleteQuery, activeID.String)
	} else {
		_, err = m.db.ExecContext(ctx, deleteQuery)
	}
	if err != nil {
		return 0, fmt.Errorf("deleting archived tasks: %w", err)
	}

	// Delete checkpoints
	checkpointsQuery := `
		DELETE FROM checkpoints WHERE workflow_id IN (
			SELECT id FROM workflows
			WHERE (status = 'completed' OR status = 'failed')
	`
	if activeID.Valid {
		checkpointsQuery += " AND id != ?"
	}
	checkpointsQuery += ")"

	if activeID.Valid {
		_, err = m.db.ExecContext(ctx, checkpointsQuery, activeID.String)
	} else {
		_, err = m.db.ExecContext(ctx, checkpointsQuery)
	}
	if err != nil {
		return 0, fmt.Errorf("deleting archived checkpoints: %w", err)
	}

	// Delete workflows
	workflowsQuery := `
		DELETE FROM workflows
		WHERE (status = 'completed' OR status = 'failed')
	`
	if activeID.Valid {
		workflowsQuery += " AND id != ?"
		_, err = m.db.ExecContext(ctx, workflowsQuery, activeID.String)
	} else {
		_, err = m.db.ExecContext(ctx, workflowsQuery)
	}
	if err != nil {
		return 0, fmt.Errorf("deleting archived workflows: %w", err)
	}

	return count, nil
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

// Verify that SQLiteStateManager implements core.StateManager.
var _ core.StateManager = (*SQLiteStateManager)(nil)
