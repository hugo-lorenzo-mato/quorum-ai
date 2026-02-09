package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUIStateManager_SaveLoad(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create manager and update state
	mgr1 := NewUIStateManager(tmpDir)
	mgr1.SetSelectedTask(5)
	mgr1.SetShowLogs(true)

	// Save
	if err := mgr1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	mgr1.Close()

	// Load in new manager
	mgr2 := NewUIStateManager(tmpDir)
	if err := mgr2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer mgr2.Close()

	state := mgr2.Get()
	if state.SelectedTask != 5 {
		t.Errorf("Expected SelectedTask=5, got %d", state.SelectedTask)
	}
	if !state.ShowLogs {
		t.Error("Expected ShowLogs=true")
	}
}

func TestUIStateManager_NoFileUsesDefaults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mgr := NewUIStateManager(tmpDir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer mgr.Close()

	state := mgr.Get()
	if state.SelectedTask != 0 {
		t.Errorf("Expected default SelectedTask=0, got %d", state.SelectedTask)
	}
	if state.ShowLogs {
		t.Error("Expected default ShowLogs=false")
	}
}

func TestUIStateManager_CorruptedFileUsesDefaults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "ui-state.json")

	// Write corrupted file
	os.WriteFile(statePath, []byte("not json"), 0644)

	mgr := NewUIStateManager(tmpDir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer mgr.Close()

	// Should use defaults
	state := mgr.Get()
	if state.Version != CurrentUIStateVersion {
		t.Errorf("Expected current version, got %d", state.Version)
	}
}

func TestUIStateManager_AtomicWrite(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "ui-state.json")

	mgr := NewUIStateManager(tmpDir)
	mgr.SetSelectedTask(3)

	if err := mgr.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	mgr.Close()

	// Verify file exists and no temp file left
	if _, err := os.Stat(statePath); err != nil {
		t.Error("State file should exist")
	}
	if _, err := os.Stat(statePath + ".tmp"); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after save")
	}
}
