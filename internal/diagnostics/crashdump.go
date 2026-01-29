package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"
)

// CrashDump contains all information captured during a crash.
type CrashDump struct {
	// Metadata
	Timestamp time.Time `json:"timestamp"`
	ProcessID int       `json:"process_id"`
	GoVersion string    `json:"go_version"`
	GOOS      string    `json:"goos"`
	GOARCH    string    `json:"goarch"`

	// Panic information
	PanicValue    string `json:"panic_value"`
	PanicLocation string `json:"panic_location,omitempty"`
	StackTrace    string `json:"stack_trace,omitempty"`

	// System state at crash
	ResourceState   ResourceSnapshot   `json:"resource_state"`
	ResourceHistory []ResourceSnapshot `json:"resource_history,omitempty"`

	// Execution context
	CurrentPhase string   `json:"current_phase,omitempty"`
	CurrentTask  string   `json:"current_task,omitempty"`
	CommandPath  string   `json:"command_path,omitempty"`
	CommandArgs  []string `json:"command_args,omitempty"`
	WorkDir      string   `json:"work_dir,omitempty"`

	// Environment (redacted)
	RedactedEnv map[string]string `json:"redacted_env,omitempty"`
}

// CommandContext captures command execution context.
type CommandContext struct {
	Path    string
	Args    []string
	WorkDir string
	Started time.Time
}

// CrashDumpWriter handles crash dump generation and persistence.
type CrashDumpWriter struct {
	dir          string
	maxFiles     int
	includeStack bool
	includeEnv   bool
	logger       *slog.Logger
	monitor      *ResourceMonitor

	// Context for current execution (updated atomically)
	currentPhase atomic.Value // string
	currentTask  atomic.Value // string
	currentCmd   atomic.Value // *CommandContext

	mu sync.Mutex // Protects file operations
}

// NewCrashDumpWriter creates a crash dump writer.
func NewCrashDumpWriter(
	dir string,
	maxFiles int,
	includeStack bool,
	includeEnv bool,
	logger *slog.Logger,
	monitor *ResourceMonitor,
) *CrashDumpWriter {
	if maxFiles <= 0 {
		maxFiles = 10
	}
	if dir == "" {
		dir = ".quorum/crashdumps"
	}

	w := &CrashDumpWriter{
		dir:          dir,
		maxFiles:     maxFiles,
		includeStack: includeStack,
		includeEnv:   includeEnv,
		logger:       logger,
		monitor:      monitor,
	}
	w.currentPhase.Store("")
	w.currentTask.Store("")
	w.currentCmd.Store((*CommandContext)(nil))
	return w
}

// SetCurrentContext updates the execution context for crash dumps.
func (w *CrashDumpWriter) SetCurrentContext(phase, task string) {
	w.currentPhase.Store(phase)
	w.currentTask.Store(task)
}

// SetCurrentCommand updates the current command being executed.
func (w *CrashDumpWriter) SetCurrentCommand(ctx *CommandContext) {
	w.currentCmd.Store(ctx)
}

// ClearCurrentCommand clears the current command context.
func (w *CrashDumpWriter) ClearCurrentCommand() {
	w.currentCmd.Store((*CommandContext)(nil))
}

// WriteCrashDump generates and writes a crash dump.
func (w *CrashDumpWriter) WriteCrashDump(panicValue interface{}) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	dump := CrashDump{
		Timestamp:  time.Now().UTC(),
		ProcessID:  os.Getpid(),
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		PanicValue: fmt.Sprintf("%v", panicValue),
	}

	// Include stack trace if configured
	if w.includeStack {
		dump.StackTrace = string(debug.Stack())
	}

	// Get resource state
	if w.monitor != nil {
		dump.ResourceState = w.monitor.TakeSnapshot()
		dump.ResourceHistory = w.monitor.GetHistory()
	}

	// Get execution context
	if phase, ok := w.currentPhase.Load().(string); ok {
		dump.CurrentPhase = phase
	}
	if task, ok := w.currentTask.Load().(string); ok {
		dump.CurrentTask = task
	}
	if cmd := w.currentCmd.Load(); cmd != nil {
		if cmdCtx, ok := cmd.(*CommandContext); ok && cmdCtx != nil {
			dump.CommandPath = cmdCtx.Path
			dump.CommandArgs = cmdCtx.Args
			dump.WorkDir = cmdCtx.WorkDir
		}
	}

	// Include redacted environment if configured
	if w.includeEnv {
		dump.RedactedEnv = w.redactEnvironment()
	}

	// Ensure directory exists
	if err := os.MkdirAll(w.dir, 0o750); err != nil {
		return "", fmt.Errorf("creating crash dump dir: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("crash-%s.json",
		dump.Timestamp.Format("2006-01-02T15-04-05"))
	path := filepath.Join(w.dir, filename)

	// Write dump
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling crash dump: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing crash dump: %w", err)
	}

	// Cleanup old dumps
	_ = w.cleanupOldDumps()

	return path, nil
}

// RecoverAndDump is a defer-compatible function for panic recovery with dump.
// Usage: defer writer.RecoverAndDump()
func (w *CrashDumpWriter) RecoverAndDump() {
	if r := recover(); r != nil {
		path, err := w.WriteCrashDump(r)
		if err != nil {
			if w.logger != nil {
				w.logger.Error("failed to write crash dump",
					"error", err,
					"panic", r,
				)
			}
		} else {
			if w.logger != nil {
				w.logger.Error("crash dump written",
					"path", path,
					"panic", r,
				)
			}
		}
		panic(r) // Re-panic after dump
	}
}

// RecoverAndReturn recovers from panic, writes dump, and returns error instead of re-panicking.
// Usage: defer writer.RecoverAndReturn(&err)
//
//nolint:gocritic // ptrToRefParam: errPtr must be a pointer to modify the caller's error variable
func (w *CrashDumpWriter) RecoverAndReturn(errPtr *error) {
	if r := recover(); r != nil {
		path, dumpErr := w.WriteCrashDump(r)
		if dumpErr != nil {
			if w.logger != nil {
				w.logger.Error("failed to write crash dump",
					"error", dumpErr,
					"panic", r,
				)
			}
		} else {
			if w.logger != nil {
				w.logger.Error("crash dump written after panic",
					"path", path,
					"panic", r,
				)
			}
		}
		*errPtr = fmt.Errorf("command execution panicked: %v (dump: %s)", r, path)
	}
}

// cleanupOldDumps removes crash dumps exceeding MaxFiles.
func (w *CrashDumpWriter) cleanupOldDumps() error {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}

	// Filter to crash dumps only
	var dumps []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
			dumps = append(dumps, e)
		}
	}

	// Sort by modification time (oldest first)
	sort.Slice(dumps, func(i, j int) bool {
		infoI, errI := dumps[i].Info()
		infoJ, errJ := dumps[j].Info()
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// Remove oldest dumps exceeding limit
	for len(dumps) > w.maxFiles {
		path := filepath.Join(w.dir, dumps[0].Name())
		if err := os.Remove(path); err != nil {
			if w.logger != nil {
				w.logger.Warn("failed to remove old crash dump",
					"path", path,
					"error", err,
				)
			}
		}
		dumps = dumps[1:]
	}

	return nil
}

func (w *CrashDumpWriter) redactEnvironment() map[string]string {
	result := make(map[string]string)
	sensitiveSubstrings := []string{
		"TOKEN", "KEY", "SECRET", "PASSWORD", "CREDENTIAL",
		"AUTH", "PRIVATE", "API_KEY", "APIKEY",
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		// Check if key contains sensitive substring
		isSensitive := false
		keyUpper := strings.ToUpper(key)
		for _, sensitive := range sensitiveSubstrings {
			if strings.Contains(keyUpper, sensitive) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			result[key] = "[REDACTED]"
		} else {
			result[key] = parts[1]
		}
	}

	return result
}

// LoadLatestCrashDump loads the most recent crash dump from the directory.
func LoadLatestCrashDump(dir string) (*CrashDump, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading crash dump dir: %w", err)
	}

	// Filter to crash dumps and find newest
	var newest os.DirEntry
	var newestTime time.Time

	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "crash-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == nil || info.ModTime().After(newestTime) {
			newest = e
			newestTime = info.ModTime()
		}
	}

	if newest == nil {
		return nil, fmt.Errorf("no crash dumps found")
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening crash dump dir: %w", err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(newest.Name())
	if err != nil {
		return nil, fmt.Errorf("reading crash dump: %w", err)
	}

	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parsing crash dump: %w", err)
	}

	return &dump, nil
}
