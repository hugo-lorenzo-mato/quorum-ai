package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- checkAgentConfigs ---

func TestCheckAgentConfigs_NoGeminiConfig(t *testing.T) {
	// Test with empty directory (no .gemini)
	tmpDir := t.TempDir()
	issues := checkAgentConfigsInDir(tmpDir)
	// No .gemini directory means no issues
	if len(issues) != 0 {
		t.Errorf("expected no issues when .gemini doesn't exist, got: %v", issues)
	}
}

func TestCheckAgentConfigs_WithDisabledGeminiConfig(t *testing.T) {
	// Create a temp home dir and set HOME to it
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		// t.Setenv handles cleanup, but also restore for safety
		_ = os.Setenv("HOME", origHome)
	}()

	// Create ~/.gemini/settings.json with "disabled": true
	geminiDir := filepath.Join(tmpDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	config := map[string]interface{}{
		"disabled": true,
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	issues := checkAgentConfigsInDir(tmpDir)
	found := false
	for _, issue := range issues {
		if issue == "Gemini config contains 'disabled: true' which causes 'NO_AGENTS' error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about 'disabled: true', got issues: %v", issues)
	}
}

func TestCheckAgentConfigs_WithValidGeminiConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		_ = os.Setenv("HOME", origHome)
	}()

	// Create valid ~/.gemini/settings.json
	geminiDir := filepath.Join(tmpDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	config := map[string]interface{}{
		"ui": map[string]interface{}{
			"theme": "Default",
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	issues := checkAgentConfigsInDir(tmpDir)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestCheckAgentConfigs_WithInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() {
		_ = os.Setenv("HOME", origHome)
	}()

	geminiDir := filepath.Join(tmpDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	issues := checkAgentConfigsInDir(tmpDir)
	found := false
	for _, issue := range issues {
		if issue == "Gemini config contains invalid JSON" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid JSON warning, got: %v", issues)
	}
}

func TestCheckAgentConfigs_NoHomeDir(t *testing.T) {
	// Empty directory without .gemini
	tmpDir := t.TempDir()

	// No .gemini directory
	issues := checkAgentConfigsInDir(tmpDir)
	if len(issues) != 0 {
		t.Errorf("expected no issues when .gemini doesn't exist, got: %v", issues)
	}
}

// --- checkCommand ---

func TestCheckCommand_KnownCommand(t *testing.T) {
	t.Parallel()
	if !checkCommand("true", []string{}) {
		t.Error("expected 'true' command to be available")
	}
}

func TestCheckCommand_UnknownCommand(t *testing.T) {
	t.Parallel()
	if checkCommand("this_command_definitely_does_not_exist_xyz_12345", []string{}) {
		t.Error("expected unknown command to return false")
	}
}

func TestCheckCommand_CommandWithArgs(t *testing.T) {
	t.Parallel()
	if !checkCommand("echo", []string{"hello"}) {
		t.Error("expected 'echo hello' to succeed")
	}
}

func TestCheckCommand_CommandThatFails(t *testing.T) {
	t.Parallel()
	// 'false' is a shell command that always returns exit code 1
	if checkCommand("false", []string{}) {
		t.Error("expected 'false' command to return false")
	}
}

// --- resolveTraceDir (from trace.go) ---

func TestResolveTraceDir_WithOverride(t *testing.T) {
	t.Parallel()
	got := resolveTraceDir("/custom/trace/dir")
	if got != "/custom/trace/dir" {
		t.Errorf("expected /custom/trace/dir, got %s", got)
	}
}

func TestResolveTraceDir_EmptyOverride(t *testing.T) {
	t.Parallel()
	got := resolveTraceDir("")
	// Should fall back to viper config or default
	if got == "" {
		t.Error("expected non-empty trace dir")
	}
}

func TestResolveTraceDir_WhitespaceOverride(t *testing.T) {
	t.Parallel()
	got := resolveTraceDir("   ")
	// Whitespace-only should fall back to default
	if got == "   " {
		t.Error("expected whitespace to be treated as empty")
	}
}

// --- listTraceEntries ---

func TestListTraceEntries_NonexistentDir(t *testing.T) {
	t.Parallel()
	entries, err := listTraceEntries("/nonexistent/dir/that/does/not/exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil && len(entries) != 0 {
		t.Errorf("expected nil or empty entries, got %d", len(entries))
	}
}

func TestListTraceEntries_EmptyDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	entries, err := listTraceEntries(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestListTraceEntries_WithNonDirFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create a regular file (not a directory) - should be skipped
	if err := os.WriteFile(filepath.Join(tmpDir, "some-file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	entries, err := listTraceEntries(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (only files, no dirs), got %d", len(entries))
	}
}

func TestListTraceEntries_DirWithoutManifest(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create a directory without run.json - should be skipped
	if err := os.MkdirAll(filepath.Join(tmpDir, "run-1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entries, err := listTraceEntries(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (no manifest), got %d", len(entries))
	}
}

// --- SetVersion / GetVersion ---

func TestSetVersion_GetVersion(t *testing.T) {
	origV, origC, origD := appVersion, appCommit, appDate
	defer func() {
		appVersion, appCommit, appDate = origV, origC, origD
	}()

	SetVersion("2.0.0", "def456", "2025-06-01")
	if got := GetVersion(); got != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got)
	}
	if appCommit != "def456" {
		t.Errorf("expected commit def456, got %s", appCommit)
	}
	if appDate != "2025-06-01" {
		t.Errorf("expected date 2025-06-01, got %s", appDate)
	}
}

func TestGetVersion_EmptyDefault(t *testing.T) {
	origV := appVersion
	defer func() { appVersion = origV }()

	appVersion = ""
	if got := GetVersion(); got != "" {
		t.Errorf("expected empty version, got %s", got)
	}
}
