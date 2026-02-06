package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/spf13/cobra"
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

func TestAnalyzeCmd_Structure(t *testing.T) {
	// Test that analyze command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "analyze [prompt]" {
			found = true
			if cmd.Short == "" {
				t.Error("analyze command missing short description")
			}
			if cmd.Long == "" {
				t.Error("analyze command missing long description")
			}
			break
		}
	}
	if !found {
		t.Error("analyze command not registered")
	}
}

func TestPlanCmd_Structure(t *testing.T) {
	// Test that plan command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "plan" {
			found = true
			if cmd.Short == "" {
				t.Error("plan command missing short description")
			}
			if cmd.Long == "" {
				t.Error("plan command missing long description")
			}
			break
		}
	}
	if !found {
		t.Error("plan command not registered")
	}
}

func TestExecuteCmd_Structure(t *testing.T) {
	// Test that execute command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "execute" {
			found = true
			if cmd.Short == "" {
				t.Error("execute command missing short description")
			}
			if cmd.Long == "" {
				t.Error("execute command missing long description")
			}
			break
		}
	}
	if !found {
		t.Error("execute command not registered")
	}
}

func TestAnalyzeCmd_Flags(t *testing.T) {
	// Find analyze command
	var analyzeCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "analyze [prompt]" {
			analyzeCmd = cmd
			break
		}
	}
	if analyzeCmd == nil {
		t.Fatal("analyze command not found")
	}

	// Test flags exist
	flags := []string{"file", "dry-run", "max-retries", "output", "single-agent", "agent", "model"}
	for _, flagName := range flags {
		if analyzeCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("analyze command missing flag: %s", flagName)
		}
	}
}

func TestPlanCmd_Flags(t *testing.T) {
	// Find plan command
	var planCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "plan" {
			planCmd = cmd
			break
		}
	}
	if planCmd == nil {
		t.Fatal("plan command not found")
	}

	// Test flags exist
	flags := []string{"dry-run", "max-retries", "output", "single-agent", "agent", "model"}
	for _, flagName := range flags {
		if planCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("plan command missing flag: %s", flagName)
		}
	}
}

func TestExecuteCmd_Flags(t *testing.T) {
	// Find execute command
	var executeCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "execute" {
			executeCmd = cmd
			break
		}
	}
	if executeCmd == nil {
		t.Fatal("execute command not found")
	}

	// Test flags exist
	flags := []string{"dry-run", "max-retries", "output", "sandbox", "single-agent", "agent", "model"}
	for _, flagName := range flags {
		if executeCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("execute command missing flag: %s", flagName)
		}
	}
}

func TestCountCompletedTasks(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusCompleted},
				"task-2": {ID: "task-2", Status: core.TaskStatusPending},
				"task-3": {ID: "task-3", Status: core.TaskStatusCompleted},
				"task-4": {ID: "task-4", Status: core.TaskStatusFailed},
			},
		},
	}

	count := countCompletedTasks(state)
	if count != 2 {
		t.Errorf("expected 2 completed tasks, got %d", count)
	}
}

func TestCountCompletedTasks_Empty(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{},
		},
	}

	count := countCompletedTasks(state)
	if count != 0 {
		t.Errorf("expected 0 completed tasks, got %d", count)
	}
}

func TestInitializeWorkflowState(t *testing.T) {
	prompt := "test prompt"
	bp := &core.Blueprint{
		ExecutionMode: "single_agent",
		SingleAgent:   core.BlueprintSingleAgent{Agent: "claude"},
	}
	state := InitializeWorkflowState(prompt, bp)

	if state.Prompt != prompt {
		t.Errorf("expected prompt '%s', got '%s'", prompt, state.Prompt)
	}
	if state.CurrentPhase != core.PhaseRefine {
		t.Errorf("expected phase 'refine', got '%s'", state.CurrentPhase)
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("expected status 'running', got '%s'", state.Status)
	}
	if state.WorkflowID == "" {
		t.Error("expected non-empty workflow ID")
	}
	if state.Version != core.CurrentStateVersion {
		t.Errorf("expected version %d, got %d", core.CurrentStateVersion, state.Version)
	}
	if state.Blueprint != bp {
		t.Errorf("expected blueprint %v, got %v", bp, state.Blueprint)
	}
}

func TestGenerateCmdWorkflowID(t *testing.T) {
	id1 := generateCmdWorkflowID()
	id2 := generateCmdWorkflowID()

	if id1 == "" {
		t.Error("expected non-empty workflow ID")
	}
	if id1 == id2 {
		t.Error("expected unique workflow IDs")
	}
	if len(id1) < 10 {
		t.Errorf("workflow ID too short: %s", id1)
	}
}
