package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_FromArgs(t *testing.T) {
	prompt, err := getPrompt([]string{"test prompt"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "test prompt" {
		t.Errorf("expected 'test prompt', got '%s'", prompt)
	}
}

func TestGetPrompt_FromFile(t *testing.T) {
	// Create temp file with prompt
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("file prompt content"), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	prompt, err := getPrompt([]string{}, promptFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "file prompt content" {
		t.Errorf("expected 'file prompt content', got '%s'", prompt)
	}
}

func TestGetPrompt_FileNotFound(t *testing.T) {
	_, err := getPrompt([]string{}, "/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestGetPrompt_NoPrompt(t *testing.T) {
	_, err := getPrompt([]string{}, "")
	if err == nil {
		t.Error("expected error when no prompt provided")
	}
}

func TestRootCmd_Structure(t *testing.T) {
	// Test that root command is properly configured
	if rootCmd.Use != "quorum" {
		t.Errorf("expected 'quorum', got '%s'", rootCmd.Use)
	}
	if rootCmd.Short == "" {
		t.Error("expected non-empty short description")
	}
}

func TestVersionCmd_Structure(t *testing.T) {
	// Test that version command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("version command not registered")
	}
}

func TestDoctorCmd_Structure(t *testing.T) {
	// Test that doctor command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("doctor command not registered")
	}
}

func TestInitCmd_Structure(t *testing.T) {
	// Test that init command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("init command not registered")
	}
}

func TestRunCmd_Structure(t *testing.T) {
	// Test that run command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "run [prompt]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("run command not registered")
	}
}

func TestStatusCmd_Structure(t *testing.T) {
	// Test that status command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("status command not registered")
	}
}

func TestSetVersion(t *testing.T) {
	SetVersion("1.0.0", "abc123", "2024-01-01")
	if appVersion != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", appVersion)
	}
	if appCommit != "abc123" {
		t.Errorf("expected commit 'abc123', got '%s'", appCommit)
	}
	if appDate != "2024-01-01" {
		t.Errorf("expected date '2024-01-01', got '%s'", appDate)
	}
}

func TestCheckCommand(t *testing.T) {
	// Test checkCommand with a known command
	if !checkCommand("ls", []string{}) {
		t.Error("expected 'ls' to be available")
	}
}

func TestCheckCommand_NotFound(t *testing.T) {
	// Test checkCommand with unknown command
	if checkCommand("nonexistent_command_xyz", []string{}) {
		t.Error("expected 'nonexistent_command_xyz' to not be available")
	}
}
