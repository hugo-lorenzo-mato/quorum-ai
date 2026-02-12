package issues

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// GenerationState tracks the state of issue generation for idempotency.
type GenerationState struct {
	// WorkflowID identifies the workflow.
	WorkflowID string `json:"workflow_id"`

	// InputChecksum is a hash of all input files (consolidated + tasks).
	InputChecksum string `json:"input_checksum"`

	// StartedAt is when generation started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when generation completed (nil if incomplete).
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// GeneratedFiles tracks files that were generated.
	GeneratedFiles []GeneratedFileInfo `json:"generated_files,omitempty"`

	// CreatedIssues tracks issues that were created in GitHub.
	CreatedIssues []CreatedIssueInfo `json:"created_issues,omitempty"`

	// ErrorMessage contains any error that occurred.
	ErrorMessage string `json:"error_message,omitempty"`
}

// GeneratedFileInfo contains info about a generated file.
type GeneratedFileInfo struct {
	Filename  string    `json:"filename"`
	TaskID    string    `json:"task_id,omitempty"`
	IsMain    bool      `json:"is_main,omitempty"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatedIssueInfo contains info about a created GitHub issue.
type CreatedIssueInfo struct {
	Number    int       `json:"number"`
	URL       string    `json:"url"`
	TaskID    string    `json:"task_id,omitempty"`
	IsMain    bool      `json:"is_main,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// IsComplete returns true if generation completed successfully.
func (s *GenerationState) IsComplete() bool {
	return s.CompletedAt != nil && s.ErrorMessage == ""
}

// GetGeneratedFilePaths returns paths to all generated files.
func (s *GenerationState) GetGeneratedFilePaths(baseDir string) []string {
	paths := make([]string, len(s.GeneratedFiles))
	for i, f := range s.GeneratedFiles {
		paths[i] = filepath.Join(baseDir, f.Filename)
	}
	return paths
}

// Deduplicator manages generation state for idempotency.
type Deduplicator struct {
	stateDir string
}

// NewDeduplicator creates a new deduplicator.
// stateDir is where state files are stored (typically .quorum/issues/{workflowID}/).
func NewDeduplicator(stateDir string) *Deduplicator {
	return &Deduplicator{
		stateDir: stateDir,
	}
}

// GetOrCreateState retrieves existing state or creates new state for a workflow.
// Returns the state and a bool indicating if it was existing (true) or new (false).
func (d *Deduplicator) GetOrCreateState(workflowID, inputChecksum string) (*GenerationState, bool, error) {
	statePath := d.getStatePath()

	// Try to load existing state
	state, err := d.loadState(statePath)
	if err == nil && state != nil {
		// Check if checksum matches (same input)
		if state.InputChecksum == inputChecksum {
			slog.Info("found existing generation state with matching checksum",
				"workflow_id", workflowID,
				"completed", state.IsComplete())
			return state, true, nil
		}
		// Checksum mismatch - input changed, need to regenerate
		slog.Info("input checksum changed, will regenerate",
			"workflow_id", workflowID,
			"old_checksum", truncateChecksum(state.InputChecksum),
			"new_checksum", truncateChecksum(inputChecksum))
	}

	// Create new state
	newState := &GenerationState{
		WorkflowID:    workflowID,
		InputChecksum: inputChecksum,
		StartedAt:     time.Now(),
	}

	return newState, false, nil
}

// MarkFileGenerated records that a file was generated.
func (d *Deduplicator) MarkFileGenerated(state *GenerationState, filename, taskID string, isMain bool, content []byte) {
	checksum := checksumBytes(content)
	state.GeneratedFiles = append(state.GeneratedFiles, GeneratedFileInfo{
		Filename:  filename,
		TaskID:    taskID,
		IsMain:    isMain,
		Checksum:  checksum,
		CreatedAt: time.Now(),
	})
}

// MarkIssueCreated records that an issue was created in GitHub.
func (d *Deduplicator) MarkIssueCreated(state *GenerationState, number int, url, taskID string, isMain bool) {
	state.CreatedIssues = append(state.CreatedIssues, CreatedIssueInfo{
		Number:    number,
		URL:       url,
		TaskID:    taskID,
		IsMain:    isMain,
		CreatedAt: time.Now(),
	})
}

// MarkComplete marks generation as complete.
func (d *Deduplicator) MarkComplete(state *GenerationState) {
	now := time.Now()
	state.CompletedAt = &now
	state.ErrorMessage = ""
}

// MarkFailed marks generation as failed with an error.
func (d *Deduplicator) MarkFailed(state *GenerationState, err error) {
	state.ErrorMessage = err.Error()
}

// Save persists the state to disk.
func (d *Deduplicator) Save(state *GenerationState) error {
	statePath := d.getStatePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	slog.Debug("saved generation state",
		"workflow_id", state.WorkflowID,
		"path", statePath,
		"files", len(state.GeneratedFiles),
		"issues", len(state.CreatedIssues))

	return nil
}

// Delete removes the state file for a workflow.
func (d *Deduplicator) Delete(workflowID string) error {
	statePath := d.getStatePath()
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing state file for workflow %s: %w", workflowID, err)
	}
	return nil
}

// getStatePath returns the path to the state file for a workflow.
func (d *Deduplicator) getStatePath() string {
	return filepath.Join(d.stateDir, ".generation-state.json")
}

// loadState loads state from a file.
func (d *Deduplicator) loadState(path string) (*GenerationState, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path constructed from internal project/report directory
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state GenerationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshaling state: %w", err)
	}

	return &state, nil
}

// CalculateInputChecksum computes a checksum of all input files.
// This is used to detect if inputs have changed and regeneration is needed.
func CalculateInputChecksum(consolidatedPath string, taskPaths []string) (string, error) {
	h := sha256.New()

	// Hash consolidated file
	if consolidatedPath != "" {
		if err := hashFile(h, consolidatedPath); err != nil {
			return "", fmt.Errorf("hashing consolidated: %w", err)
		}
	}

	// Sort task paths for deterministic ordering
	sortedPaths := make([]string, len(taskPaths))
	copy(sortedPaths, taskPaths)
	sort.Strings(sortedPaths)

	// Hash each task file
	for _, path := range sortedPaths {
		if err := hashFile(h, path); err != nil {
			return "", fmt.Errorf("hashing task %s: %w", path, err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFile adds a file's contents to a hash.
func hashFile(h io.Writer, path string) error {
	f, err := os.Open(path) // #nosec G304 -- path constructed from internal project/report directory
	if err != nil {
		return err
	}
	defer f.Close()

	// Write path as separator
	if _, err := h.Write([]byte(path)); err != nil {
		return err
	}
	if _, err := h.Write([]byte{0}); err != nil {
		return err
	}

	// Write content
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	return nil
}

// checksumBytes returns a hex-encoded SHA256 checksum of the data.
func checksumBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// truncateChecksum safely truncates a checksum for logging purposes.
func truncateChecksum(checksum string) string {
	if len(checksum) <= 16 {
		return checksum
	}
	return checksum[:16] + "..."
}

// HasExistingIssues checks if issues have already been created for a workflow.
func (d *Deduplicator) HasExistingIssues(workflowID string) (hasIssues bool, issueCount int) {
	statePath := d.getStatePath()
	state, err := d.loadState(statePath)
	if err != nil || state == nil {
		return false, 0
	}
	if state.WorkflowID != workflowID {
		return false, 0
	}
	return len(state.CreatedIssues) > 0, len(state.CreatedIssues)
}

// GetExistingIssueNumbers returns the numbers of already-created issues.
func (d *Deduplicator) GetExistingIssueNumbers(workflowID string) []int {
	statePath := d.getStatePath()
	state, err := d.loadState(statePath)
	if err != nil || state == nil {
		return nil
	}
	if state.WorkflowID != workflowID {
		return nil
	}

	numbers := make([]int, len(state.CreatedIssues))
	for i, issue := range state.CreatedIssues {
		numbers[i] = issue.Number
	}
	return numbers
}
