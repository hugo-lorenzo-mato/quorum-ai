package chat

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial_schema.sql
var chatMigrationV1 string

//go:embed migrations/002_add_title.sql
var chatMigrationV2 string

// SQLiteChatStore implements ChatStore with SQLite storage.
type SQLiteChatStore struct {
	dbPath string
	db     *sql.DB // Write connection
	readDB *sql.DB // Read-only connection
	mu     sync.RWMutex

	// Retry configuration
	maxRetries    int
	baseRetryWait time.Duration
}

// SQLiteChatStoreOption configures the store.
type SQLiteChatStoreOption func(*SQLiteChatStore)

// NewSQLiteChatStore creates a new SQLite-based chat store.
func NewSQLiteChatStore(dbPath string, opts ...SQLiteChatStoreOption) (*SQLiteChatStore, error) {
	s := &SQLiteChatStore{
		dbPath:        dbPath,
		maxRetries:    5,
		baseRetryWait: 100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(s)
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating chat directory: %w", err)
	}

	// Open write connection with WAL mode
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening write database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	s.db = db

	// Open read-only connection
	readDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&mode=ro&_pragma=busy_timeout(1000)")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("opening read database: %w", err)
	}
	readDB.SetMaxOpenConns(10)
	readDB.SetMaxIdleConns(5)
	readDB.SetConnMaxLifetime(5 * time.Minute)
	s.readDB = readDB

	// Run migrations
	if err := s.migrate(); err != nil {
		_ = db.Close()
		_ = readDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// migrate runs database migrations.
func (s *SQLiteChatStore) migrate() error {
	// Create migrations table if needed
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS chat_schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Check current version
	var currentVersion int
	row := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM chat_schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("checking schema version: %w", err)
	}

	// Apply pending migrations
	migrations := []string{chatMigrationV1, chatMigrationV2}
	for i, migration := range migrations {
		version := i + 1
		if version <= currentVersion {
			continue
		}

		// Execute migration in a transaction
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration transaction: %w", err)
		}

		// Split and execute each statement
		for _, stmt := range splitStatements(migration) {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("executing migration v%d: %w", version, err)
			}
		}

		// Record migration
		if _, err := tx.Exec(
			"INSERT INTO chat_schema_migrations (version, applied_at) VALUES (?, ?)",
			version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration v%d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration v%d: %w", version, err)
		}
	}

	return nil
}

// splitStatements splits a SQL script into individual statements.
func splitStatements(script string) []string {
	var statements []string
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Remove leading comment lines, keeping the actual SQL
		lines := strings.Split(stmt, "\n")
		var sqlLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
				sqlLines = append(sqlLines, line)
			}
		}
		if len(sqlLines) > 0 {
			statements = append(statements, strings.Join(sqlLines, "\n"))
		}
	}
	return statements
}

// retryWrite executes a write operation with retry logic.
func (s *SQLiteChatStore) retryWrite(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if err := fn(); err != nil {
			if isSQLiteBusy(err) {
				lastErr = err
				wait := s.baseRetryWait * time.Duration(1<<attempt)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d retries: %w", operation, s.maxRetries, lastErr)
}

// isSQLiteBusy checks if an error is a SQLite busy/locked error.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "SQLITE_LOCKED")
}

// SaveSession persists a chat session.
func (s *SQLiteChatStore) SaveSession(ctx context.Context, session *core.ChatSessionState) error {
	return s.retryWrite(ctx, "SaveSession", func() error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO chat_sessions (id, title, created_at, updated_at, agent, model, project_root)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				updated_at = excluded.updated_at,
				agent = excluded.agent,
				model = excluded.model,
				project_root = excluded.project_root
		`,
			session.ID,
			session.Title,
			session.CreatedAt.UTC().Format(time.RFC3339Nano),
			session.UpdatedAt.UTC().Format(time.RFC3339Nano),
			session.Agent,
			session.Model,
			session.ProjectRoot,
		)
		return err
	})
}

// LoadSession retrieves a chat session by ID.
func (s *SQLiteChatStore) LoadSession(ctx context.Context, id string) (*core.ChatSessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.readDB.QueryRowContext(ctx, `
		SELECT id, title, created_at, updated_at, agent, model, project_root
		FROM chat_sessions WHERE id = ?
	`, id)

	var session core.ChatSessionState
	var createdAt, updatedAt string
	var title, model, projectRoot sql.NullString

	err := row.Scan(&session.ID, &title, &createdAt, &updatedAt, &session.Agent, &model, &projectRoot)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning session: %w", err)
	}

	session.Title = title.String
	session.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	session.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	session.Model = model.String
	session.ProjectRoot = projectRoot.String

	return &session, nil
}

// ListSessions returns all chat sessions.
func (s *SQLiteChatStore) ListSessions(ctx context.Context) ([]*core.ChatSessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.readDB.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at, agent, model, project_root
		FROM chat_sessions
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*core.ChatSessionState
	for rows.Next() {
		var session core.ChatSessionState
		var createdAt, updatedAt string
		var title, model, projectRoot sql.NullString

		if err := rows.Scan(&session.ID, &title, &createdAt, &updatedAt, &session.Agent, &model, &projectRoot); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}

		session.Title = title.String
		session.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		session.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		session.Model = model.String
		session.ProjectRoot = projectRoot.String

		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// DeleteSession removes a chat session and all its messages.
func (s *SQLiteChatStore) DeleteSession(ctx context.Context, id string) error {
	return s.retryWrite(ctx, "DeleteSession", func() error {
		_, err := s.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE id = ?", id)
		return err
	})
}

// SaveMessage adds a message to a session.
func (s *SQLiteChatStore) SaveMessage(ctx context.Context, msg *core.ChatMessageState) error {
	return s.retryWrite(ctx, "SaveMessage", func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		// Insert message
		_, err = tx.ExecContext(ctx, `
			INSERT INTO chat_messages (id, session_id, role, agent, content, timestamp, tokens_in, tokens_out, cost_usd)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			msg.ID,
			msg.SessionID,
			msg.Role,
			msg.Agent,
			msg.Content,
			msg.Timestamp.UTC().Format(time.RFC3339Nano),
			msg.TokensIn,
			msg.TokensOut,
			msg.CostUSD,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		// Update session timestamp
		_, err = tx.ExecContext(ctx, `
			UPDATE chat_sessions SET updated_at = ? WHERE id = ?
		`, msg.Timestamp.UTC().Format(time.RFC3339Nano), msg.SessionID)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		return tx.Commit()
	})
}

// LoadMessages retrieves all messages for a session.
func (s *SQLiteChatStore) LoadMessages(ctx context.Context, sessionID string) ([]*core.ChatMessageState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.readDB.QueryContext(ctx, `
		SELECT id, session_id, role, agent, content, timestamp, tokens_in, tokens_out, cost_usd
		FROM chat_messages
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var messages []*core.ChatMessageState
	for rows.Next() {
		var msg core.ChatMessageState
		var timestamp string
		var agent sql.NullString

		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &agent, &msg.Content, &timestamp, &msg.TokensIn, &msg.TokensOut, &msg.CostUSD); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}

		msg.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
		msg.Agent = agent.String

		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

// Close closes both database connections.
func (s *SQLiteChatStore) Close() error {
	var errs []error
	if s.readDB != nil {
		if err := s.readDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing read connection: %w", err))
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing write connection: %w", err))
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
