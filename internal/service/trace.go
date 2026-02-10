package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// TraceConfig configures trace output for a workflow run.
type TraceConfig struct {
	Mode            string
	Dir             string
	SchemaVersion   int
	Redact          bool
	RedactPatterns  []string
	RedactAllowlist []string
	MaxBytes        int64
	TotalMaxBytes   int64
	MaxFiles        int
	IncludePhases   []string
}

// TraceRunInfo describes run-level metadata for traces.
type TraceRunInfo struct {
	RunID        string
	WorkflowID   string
	PromptLength int
	StartedAt    time.Time
	AppVersion   string
	AppCommit    string
	AppDate      string
	GitCommit    string
	GitDirty     bool
	Config       TraceConfig
}

// TraceRunSummary captures end-of-run stats.
type TraceRunSummary struct {
	RunID          string
	EndedAt        time.Time
	TotalEvents    int
	TotalPrompts   int
	TotalTokensIn  int
	TotalTokensOut int
	TotalFiles     int
	TotalBytes     int64
	Dir            string
}

// TraceEvent is a single trace record.
type TraceEvent struct {
	Phase     string
	Step      string
	EventType string
	Agent     string
	Model     string
	TaskID    string
	TaskName  string
	FileExt   string
	Content   []byte
	TokensIn  int
	TokensOut int
	Metadata  map[string]interface{}
}

// TraceWriter records trace events.
type TraceWriter interface {
	Enabled() bool
	StartRun(ctx context.Context, info TraceRunInfo) error
	Record(ctx context.Context, event TraceEvent) error
	EndRun(ctx context.Context) TraceRunSummary
	RunID() string
	Dir() string
}

// NewTraceWriter creates a trace writer based on config.
func NewTraceWriter(cfg TraceConfig, logger *logging.Logger) TraceWriter {
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = "off"
	}
	if mode == "off" {
		return &noopTraceWriter{}
	}

	writer := &fileTraceWriter{
		cfg:     normalizeTraceConfig(cfg),
		logger:  logger,
		enabled: true,
	}
	writer.compileRedactors()

	return writer
}

type noopTraceWriter struct{}

func (n *noopTraceWriter) Enabled() bool { return false }
func (n *noopTraceWriter) StartRun(_ context.Context, _ TraceRunInfo) error {
	return nil
}
func (n *noopTraceWriter) Record(_ context.Context, _ TraceEvent) error { return nil }
func (n *noopTraceWriter) EndRun(_ context.Context) TraceRunSummary     { return TraceRunSummary{} }
func (n *noopTraceWriter) RunID() string                                { return "" }
func (n *noopTraceWriter) Dir() string                                  { return "" }

type fileTraceWriter struct {
	cfg     TraceConfig
	logger  *logging.Logger
	enabled bool

	mu        sync.Mutex
	seq       int
	runID     string
	dir       string
	jsonlPath string
	runInfo   TraceRunInfo

	warned         bool
	redactPatterns []*regexp.Regexp
	allowPatterns  []*regexp.Regexp
	totalBytes     int64
	fileCount      int
	totalEvents    int
	totalPrompts   int
	totalTokensIn  int
	totalTokensOut int
}

func (w *fileTraceWriter) Enabled() bool { return w.enabled }
func (w *fileTraceWriter) RunID() string { return w.runID }
func (w *fileTraceWriter) Dir() string   { return w.dir }

func (w *fileTraceWriter) StartRun(_ context.Context, info TraceRunInfo) error {
	if !w.enabled {
		return nil
	}

	runID := sanitizeTraceID(info.RunID)
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().Unix())
	}

	w.runID = runID
	w.dir = filepath.Join(w.cfg.Dir, runID)
	w.jsonlPath = filepath.Join(w.dir, "trace.jsonl")
	info.RunID = runID
	info.Config = w.cfg
	w.runInfo = info

	if err := os.MkdirAll(w.dir, 0o750); err != nil {
		w.disableWithWarning(fmt.Errorf("creating trace dir: %w", err))
		return err
	}

	if err := w.writeManifest(info, TraceRunSummary{}); err != nil {
		w.disableWithWarning(fmt.Errorf("writing trace manifest: %w", err))
		return err
	}

	return nil
}

func (w *fileTraceWriter) Record(_ context.Context, event TraceEvent) error {
	if !w.enabled {
		return nil
	}
	if !w.phaseIncluded(event.Phase) {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.seq++
	seq := w.seq
	w.totalEvents++
	if event.EventType == "prompt" {
		w.totalPrompts++
	}
	w.totalTokensIn += event.TokensIn
	w.totalTokensOut += event.TokensOut

	record := traceRecord{
		Seq:       seq,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: event.EventType,
		Phase:     event.Phase,
		Step:      event.Step,
		Agent:     event.Agent,
		Model:     event.Model,
		TaskID:    event.TaskID,
		TaskName:  event.TaskName,
		TokensIn:  event.TokensIn,
		TokensOut: event.TokensOut,
		Metadata:  event.Metadata,
	}

	content := event.Content
	if len(content) > 0 {
		record.HashRaw = hashContent(content)
		stored, redacted := w.redact(content)
		stored, truncated := w.truncate(stored)
		record.HashStored = hashContent(stored)
		record.ContentRedacted = redacted
		record.ContentTruncated = truncated

		if w.cfg.Mode == "full" {
			filename := w.buildFilename(seq, event)
			if filename != "" {
				filePath := filepath.Join(w.dir, filename)
				if w.canWriteFile(int64(len(stored))) {
					if err := os.WriteFile(filePath, stored, 0o600); err != nil {
						w.disableWithWarning(fmt.Errorf("writing trace file: %w", err))
					} else {
						record.File = filename
						w.totalBytes += int64(len(stored))
						w.fileCount++
					}
				} else {
					record.ContentDropped = true
				}
			} else {
				record.ContentDropped = true
			}
		}
	}

	if err := w.appendRecord(record); err != nil {
		w.disableWithWarning(fmt.Errorf("writing trace record: %w", err))
		return err
	}

	return nil
}

func (w *fileTraceWriter) EndRun(_ context.Context) TraceRunSummary {
	if !w.enabled {
		return TraceRunSummary{}
	}

	summary := TraceRunSummary{
		RunID:          w.runID,
		EndedAt:        time.Now().UTC(),
		TotalEvents:    w.totalEvents,
		TotalPrompts:   w.totalPrompts,
		TotalTokensIn:  w.totalTokensIn,
		TotalTokensOut: w.totalTokensOut,
		TotalFiles:     w.fileCount,
		TotalBytes:     w.totalBytes,
		Dir:            w.dir,
	}

	if err := w.writeManifest(w.runInfo, summary); err != nil {
		w.disableWithWarning(fmt.Errorf("updating trace manifest: %w", err))
	}

	return summary
}

func (w *fileTraceWriter) appendRecord(record traceRecord) error {
	if !w.enabled {
		return nil
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	file, err := os.OpenFile(w.jsonlPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}

	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (w *fileTraceWriter) buildFilename(seq int, event TraceEvent) string {
	base := strings.Trim(strings.Join([]string{
		event.Phase,
		event.Step,
		event.Agent,
		event.TaskID,
		event.EventType,
	}, "-"), "-")
	base = sanitizeFileComponent(base)
	if base == "" {
		base = fmt.Sprintf("event-%d", seq)
	}

	ext := strings.TrimPrefix(event.FileExt, ".")
	if ext == "" {
		ext = "txt"
	}

	return fmt.Sprintf("%04d-%s.%s", seq, base, ext)
}

func (w *fileTraceWriter) phaseIncluded(phase string) bool {
	if len(w.cfg.IncludePhases) == 0 {
		return true
	}
	for _, p := range w.cfg.IncludePhases {
		if p == phase {
			return true
		}
	}
	return false
}

func (w *fileTraceWriter) redact(content []byte) ([]byte, bool) {
	if !w.cfg.Redact || len(w.redactPatterns) == 0 {
		return content, false
	}

	text := string(content)
	redacted := false
	for _, re := range w.redactPatterns {
		text = re.ReplaceAllStringFunc(text, func(match string) string {
			for _, allow := range w.allowPatterns {
				if allow.MatchString(match) {
					return match
				}
			}
			redacted = true
			return "[REDACTED]"
		})
	}

	return []byte(text), redacted
}

func (w *fileTraceWriter) truncate(content []byte) ([]byte, bool) {
	if w.cfg.MaxBytes <= 0 || int64(len(content)) <= w.cfg.MaxBytes {
		return content, false
	}

	marker := []byte("\n[trace truncated]\n")
	limit := w.cfg.MaxBytes - int64(len(marker))
	if limit <= 0 {
		return content[:int(w.cfg.MaxBytes)], true
	}

	result := content[:int(limit)]
	result = append(result, marker...)
	return result, true
}

func (w *fileTraceWriter) canWriteFile(size int64) bool {
	if w.cfg.TotalMaxBytes <= 0 || w.cfg.MaxFiles <= 0 {
		return false
	}
	if w.fileCount+1 > w.cfg.MaxFiles {
		return false
	}
	return w.totalBytes+size <= w.cfg.TotalMaxBytes
}

func (w *fileTraceWriter) writeManifest(info TraceRunInfo, summary TraceRunSummary) error {
	if !w.enabled {
		return nil
	}

	manifest := traceManifest{
		SchemaVersion: w.cfg.SchemaVersion,
		RunID:         w.runID,
		WorkflowID:    info.WorkflowID,
		PromptLength:  info.PromptLength,
		StartedAt:     info.StartedAt,
		EndedAt:       summary.EndedAt,
		AppVersion:    info.AppVersion,
		AppCommit:     info.AppCommit,
		AppDate:       info.AppDate,
		GitCommit:     info.GitCommit,
		GitDirty:      info.GitDirty,
		Config:        w.cfg,
		Summary:       summary,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(w.dir, "run.json")
	return os.WriteFile(path, data, 0o600)
}

func (w *fileTraceWriter) disableWithWarning(err error) {
	if !w.enabled {
		return
	}
	w.enabled = false
	if w.warned {
		return
	}
	w.warned = true
	if w.logger != nil {
		w.logger.Warn("trace disabled", "error", err)
	}
}

func (w *fileTraceWriter) compileRedactors() {
	patterns := w.cfg.RedactPatterns
	if len(patterns) == 0 {
		patterns = defaultRedactPatterns()
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("invalid redact pattern", "pattern", pattern, "error", err)
			}
			continue
		}
		w.redactPatterns = append(w.redactPatterns, re)
	}

	for _, pattern := range w.cfg.RedactAllowlist {
		re, err := regexp.Compile(pattern)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("invalid redact allowlist pattern", "pattern", pattern, "error", err)
			}
			continue
		}
		w.allowPatterns = append(w.allowPatterns, re)
	}
}

func normalizeTraceConfig(cfg TraceConfig) TraceConfig {
	if cfg.SchemaVersion <= 0 {
		cfg.SchemaVersion = 1
	}
	if cfg.Dir == "" {
		cfg.Dir = ".quorum/traces"
	}
	if cfg.Redact && len(cfg.RedactPatterns) == 0 {
		cfg.RedactPatterns = defaultRedactPatterns()
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 262144
	}
	if cfg.TotalMaxBytes <= 0 {
		cfg.TotalMaxBytes = 10485760
	}
	if cfg.TotalMaxBytes < cfg.MaxBytes {
		cfg.TotalMaxBytes = cfg.MaxBytes
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 500
	}
	return cfg
}

func defaultRedactPatterns() []string {
	return []string{
		`(?i)authorization\s*:\s*bearer\s+[A-Za-z0-9._\-]+`,
		`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|secret)\b\s*[:=]\s*[^\s"']+`,
		`\bsk-[A-Za-z0-9]{16,}\b`,
		`\bgh[pousr]_[A-Za-z0-9]{36,}\b`,
		`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`,
	}
}

func sanitizeTraceID(input string) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func sanitizeFileComponent(input string) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range input {
		if r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func hashContent(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

type traceRecord struct {
	Seq              int                    `json:"seq"`
	Timestamp        string                 `json:"ts"`
	EventType        string                 `json:"event_type"`
	Phase            string                 `json:"phase"`
	Step             string                 `json:"step,omitempty"`
	Agent            string                 `json:"agent,omitempty"`
	Model            string                 `json:"model,omitempty"`
	TaskID           string                 `json:"task_id,omitempty"`
	TaskName         string                 `json:"task_name,omitempty"`
	TokensIn         int                    `json:"tokens_in,omitempty"`
	TokensOut        int                    `json:"tokens_out,omitempty"`
	File             string                 `json:"file,omitempty"`
	HashRaw          string                 `json:"hash_raw,omitempty"`
	HashStored       string                 `json:"hash_stored,omitempty"`
	ContentRedacted  bool                   `json:"content_redacted,omitempty"`
	ContentTruncated bool                   `json:"content_truncated,omitempty"`
	ContentDropped   bool                   `json:"content_dropped,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type traceManifest struct {
	SchemaVersion int             `json:"schema_version"`
	RunID         string          `json:"run_id"`
	WorkflowID    string          `json:"workflow_id"`
	PromptLength  int             `json:"prompt_length"`
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       time.Time       `json:"ended_at,omitempty"`
	AppVersion    string          `json:"app_version,omitempty"`
	AppCommit     string          `json:"app_commit,omitempty"`
	AppDate       string          `json:"app_date,omitempty"`
	GitCommit     string          `json:"git_commit,omitempty"`
	GitDirty      bool            `json:"git_dirty"`
	Config        TraceConfig     `json:"config"`
	Summary       TraceRunSummary `json:"summary,omitempty"`
}

// TraceOutputNotifierDelegate defines the delegate interface for output notifications.
// This matches workflow.OutputNotifier but is defined here to avoid circular imports.
type TraceOutputNotifierDelegate interface {
	PhaseStarted(phase string)
	TaskStarted(taskID, taskName, cli string)
	TaskCompleted(taskID, taskName string, duration time.Duration, tokensIn, tokensOut int)
	TaskFailed(taskID, taskName string, err error)
	WorkflowStateUpdated(status string, totalTasks int)
}

// TraceOutputNotifier wraps a TraceWriter to emit trace events for workflow notifications.
// It can be composed with the existing OutputNotifier to add tracing without modifying
// the existing notification flow.
type TraceOutputNotifier struct {
	writer TraceWriter
	ctx    context.Context
}

// NewTraceOutputNotifier creates a new trace output notifier.
func NewTraceOutputNotifier(writer TraceWriter) *TraceOutputNotifier {
	return &TraceOutputNotifier{
		writer: writer,
		ctx:    context.Background(),
	}
}

// PhaseStarted records a phase started event.
func (t *TraceOutputNotifier) PhaseStarted(phase string) {
	if t.writer == nil || !t.writer.Enabled() {
		return
	}
	_ = t.writer.Record(t.ctx, TraceEvent{
		Phase:     phase,
		EventType: "phase_started",
	})
}

// TaskStarted records a task started event.
func (t *TraceOutputNotifier) TaskStarted(taskID, taskName, cli string) {
	if t.writer == nil || !t.writer.Enabled() {
		return
	}
	_ = t.writer.Record(t.ctx, TraceEvent{
		Phase:     "",
		EventType: "task_started",
		TaskID:    taskID,
		TaskName:  taskName,
		Agent:     cli,
		Metadata: map[string]interface{}{
			"task_id":   taskID,
			"task_name": taskName,
			"cli":       cli,
		},
	})
}

// TaskCompleted records a task completed event.
func (t *TraceOutputNotifier) TaskCompleted(taskID, taskName string, duration time.Duration, tokensIn, tokensOut int) {
	if t.writer == nil || !t.writer.Enabled() {
		return
	}
	_ = t.writer.Record(t.ctx, TraceEvent{
		Phase:     "",
		EventType: "task_completed",
		TaskID:    taskID,
		TaskName:  taskName,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Metadata: map[string]interface{}{
			"duration": duration.String(),
		},
	})
}

// TaskFailed records a task failed event.
func (t *TraceOutputNotifier) TaskFailed(taskID, taskName string, err error) {
	if t.writer == nil || !t.writer.Enabled() {
		return
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	_ = t.writer.Record(t.ctx, TraceEvent{
		Phase:     "",
		EventType: "task_failed",
		TaskID:    taskID,
		TaskName:  taskName,
		Metadata: map[string]interface{}{
			"error": errStr,
		},
	})
}

// WorkflowStateUpdated records a workflow state updated event.
func (t *TraceOutputNotifier) WorkflowStateUpdated(status string, totalTasks int) {
	if t.writer == nil || !t.writer.Enabled() {
		return
	}
	_ = t.writer.Record(t.ctx, TraceEvent{
		Phase:     "",
		EventType: "workflow_state_updated",
		Metadata: map[string]interface{}{
			"status":      status,
			"total_tasks": totalTasks,
		},
	})
}

// Close closes the underlying trace writer.
func (t *TraceOutputNotifier) Close() error {
	if t.writer != nil && t.writer.Enabled() {
		t.writer.EndRun(t.ctx)
	}
	return nil
}
