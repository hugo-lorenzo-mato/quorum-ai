package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestRunStatus(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("no active workflow", func(t *testing.T) {
		// Create state manager with non-existent state
		statePath := filepath.Join(tmpDir, "no-workflow.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		statusJSON = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runStatus(statusCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "No active workflow")
	})

	t.Run("with active workflow text output", func(t *testing.T) {
		// Create state manager and workflow
		statePath := filepath.Join(tmpDir, "workflow.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowID := core.WorkflowID("test-workflow")

		// Create workflow state
		now := time.Now()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: workflowID,
				Prompt:     "test workflow",
				CreatedAt:  now,
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Status:       core.WorkflowStatusRunning,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}

		// Add task
		startedAt := now.Add(-5 * time.Minute)
		completedAt := now.Add(-2 * time.Minute)
		workflowState.WorkflowRun.Tasks["task-1"] = &core.TaskState{
			ID:          "task-1",
			Name:        "Test Task 1",
			Phase:       core.PhaseAnalyze,
			Status:      core.TaskStatusCompleted,
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
		}

		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		statusJSON = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runStatus(statusCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "test-workflow")
		assert.Contains(t, output, "ANALYZE")
		assert.Contains(t, output, "IN_PROGRESS")
	})

	t.Run("json output", func(t *testing.T) {
		// Create state manager and workflow
		statePath := filepath.Join(tmpDir, "workflow-json.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowID := core.WorkflowID("json-workflow")

		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: workflowID,
				Prompt:     "json workflow",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhasePlan,
				Status:       core.WorkflowStatusCompleted,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}

		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		statusJSON = true
		defer func() { statusJSON = false }()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runStatus(statusCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "json-workflow")
		assert.Contains(t, output, "PLAN")
		assert.Contains(t, output, "COMPLETED")
	})

	t.Run("default state path", func(t *testing.T) {
		// Reset viper to use default path
		viper.Reset()
		viper.Set("state.path", "")

		tmpDir2 := t.TempDir()
		oldDir, _ := os.Getwd()
		defer os.Chdir(oldDir)

		err := os.Chdir(tmpDir2)
		require.NoError(t, err)

		// Create .quorum directory
		quorumDir := filepath.Join(tmpDir2, ".quorum", "state")
		err = os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		statusJSON = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runStatus(statusCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		assert.NoError(t, err)
	})
}

func TestOutputJSON(t *testing.T) {
	t.Run("output simple map", func(t *testing.T) {
		data := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputJSON(data)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		assert.NoError(t, err)

		// Read output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "key1")
		assert.Contains(t, output, "value1")
		assert.Contains(t, output, "key2")
		assert.Contains(t, output, "42")
	})

	t.Run("output workflow state", func(t *testing.T) {
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "test-workflow",
				Prompt:     "test",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Status:       core.WorkflowStatusRunning,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputJSON(workflowState)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		assert.NoError(t, err)

		// Read output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "test-workflow")
	})
}

func TestStatusCommand(t *testing.T) {
	assert.NotNil(t, statusCmd)
	assert.Equal(t, "status", statusCmd.Use)

	// Verify flags
	flag := statusCmd.Flags().Lookup("json")
	assert.NotNil(t, flag)
	assert.Equal(t, "json", flag.Name)
}
