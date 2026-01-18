package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// UIState represents the persisted UI state.
type UIState struct {
	Version       int       `json:"version"`
	SelectedTask  int       `json:"selected_task"`
	ShowLogs      bool      `json:"show_logs"`
	LogScrollPos  int       `json:"log_scroll_pos"`
	TaskScrollPos int       `json:"task_scroll_pos"`
	ExpandedTasks []string  `json:"expanded_tasks,omitempty"`
	LastWorkflow  string    `json:"last_workflow,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CurrentUIStateVersion is the schema version for UI state.
const CurrentUIStateVersion = 1

// DefaultUIState returns the default UI state.
func DefaultUIState() *UIState {
	return &UIState{
		Version:      CurrentUIStateVersion,
		SelectedTask: 0,
		ShowLogs:     false,
		UpdatedAt:    time.Now(),
	}
}

// UIStateManager handles UI state persistence.
type UIStateManager struct {
	mu      sync.RWMutex
	path    string
	state   *UIState
	dirty   bool
	saveCh  chan struct{}
	closeCh chan struct{}
	closeWg sync.WaitGroup
}

// NewUIStateManager creates a new UI state manager.
func NewUIStateManager(baseDir string) *UIStateManager {
	path := filepath.Join(baseDir, "ui-state.json")

	mgr := &UIStateManager{
		path:    path,
		state:   DefaultUIState(),
		saveCh:  make(chan struct{}, 1),
		closeCh: make(chan struct{}),
	}

	// Start background saver
	mgr.closeWg.Add(1)
	go mgr.backgroundSaver()

	return mgr
}

// Load loads the UI state from disk.
func (m *UIStateManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		// No state file yet, use defaults
		return nil
	}
	if err != nil {
		return err
	}

	var state UIState
	if err := json.Unmarshal(data, &state); err != nil {
		// Invalid state file, use defaults
		return nil
	}

	// Version migration if needed
	if state.Version < CurrentUIStateVersion {
		state = m.migrateState(state)
	}

	m.state = &state
	return nil
}

// Save saves the UI state to disk.
func (m *UIStateManager) Save() error {
	m.mu.RLock()
	state := *m.state
	m.mu.RUnlock()

	state.UpdatedAt = time.Now()

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically
	tmpPath := m.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tmpPath, m.path)
}

// Get returns the current UI state.
func (m *UIStateManager) Get() UIState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.state
}

// Update updates the UI state.
func (m *UIStateManager) Update(fn func(*UIState)) {
	m.mu.Lock()
	fn(m.state)
	m.dirty = true
	m.mu.Unlock()

	// Signal background saver
	select {
	case m.saveCh <- struct{}{}:
	default:
	}
}

// SetSelectedTask updates the selected task index.
func (m *UIStateManager) SetSelectedTask(idx int) {
	m.Update(func(s *UIState) {
		s.SelectedTask = idx
	})
}

// SetShowLogs updates the log panel visibility.
func (m *UIStateManager) SetShowLogs(show bool) {
	m.Update(func(s *UIState) {
		s.ShowLogs = show
	})
}

// SetLastWorkflow updates the last workflow ID.
func (m *UIStateManager) SetLastWorkflow(workflowID core.WorkflowID) {
	m.Update(func(s *UIState) {
		s.LastWorkflow = string(workflowID)
	})
}

// Close shuts down the state manager.
func (m *UIStateManager) Close() error {
	close(m.closeCh)
	m.closeWg.Wait()

	// Final save
	if m.dirty {
		return m.Save()
	}
	return nil
}

// backgroundSaver saves state periodically when dirty.
func (m *UIStateManager) backgroundSaver() {
	defer m.closeWg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.closeCh:
			return
		case <-m.saveCh:
			// Debounce - wait for more updates
			time.Sleep(500 * time.Millisecond)
		case <-ticker.C:
		}

		m.mu.Lock()
		dirty := m.dirty
		m.dirty = false
		m.mu.Unlock()

		if dirty {
			_ = m.Save() // Ignore errors in background
		}
	}
}

// migrateState migrates old state versions.
func (m *UIStateManager) migrateState(old UIState) UIState {
	// Add migration logic as versions evolve
	migrated := old
	migrated.Version = CurrentUIStateVersion
	return migrated
}
