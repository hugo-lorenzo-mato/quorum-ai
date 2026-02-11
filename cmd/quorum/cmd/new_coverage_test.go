package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestRunNew(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("no workflows with force", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "empty.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		newPurge = true
		newForce = true
		defer func() {
			newPurge = false
			newForce = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runNew(newCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "No workflows to purge")
	})

	t.Run("purge with confirmation no", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "purge-no.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		// Create workflow
		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "test-workflow",
				Prompt:     "test prompt",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Status:       core.WorkflowStatusCompleted,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}
		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		newPurge = true
		newForce = false
		defer func() {
			newPurge = false
			newForce = false
		}()

		// Simulate user input "no"
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			w.Write([]byte("n\n"))
			w.Close()
		}()
		defer func() { os.Stdin = oldStdin }()

		// Capture stdout
		oldStdout := os.Stdout
		rOut, wOut, _ := os.Pipe()
		os.Stdout = wOut

		err = runNew(newCmd, []string{})

		wOut.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(rOut)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Aborted")
	})

	t.Run("purge with force flag", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "purge-force.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		// Create workflow
		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "test-workflow-2",
				Prompt:     "test prompt 2",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Status:       core.WorkflowStatusFailed,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}
		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		newPurge = true
		newForce = true
		defer func() {
			newPurge = false
			newForce = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runNew(newCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Purged")
		assert.Contains(t, output, "workflow")
	})

	t.Run("archive workflows", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "archive.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		// Create completed workflow
		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "completed-workflow",
				Prompt:     "completed prompt",
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseExecute,
				Status:       core.WorkflowStatusCompleted,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}
		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		newArchive = true
		defer func() { newArchive = false }()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runNew(newCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Ready for new task")
	})

	t.Run("deactivate with active workflow", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "deactivate.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		// Create active workflow
		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "active-workflow",
				Prompt:     "active prompt",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Status:       core.WorkflowStatusRunning,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}
		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		// Set as active (simulate by saving with active workflow)
		// Note: StateManager doesn't expose SetActiveWorkflow directly

		// Reset flags
		newArchive = false
		newPurge = false
		newForce = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runNew(newCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Workflow deactivated")
		assert.Contains(t, output, "Ready for new task")
	})

	t.Run("deactivate with no active workflow", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "no-active.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		newArchive = false
		newPurge = false
		newForce = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runNew(newCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "No active workflow")
	})

	t.Run("purge with yes confirmation", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, "purge-yes.db")
		viper.Set("state.path", statePath)
		defer viper.Reset()

		// Create workflow
		sm, err := state.NewStateManager(statePath)
		require.NoError(t, err)
		defer state.CloseStateManager(sm)

		ctx := context.Background()
		workflowState := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "to-purge",
				Prompt:     "purge prompt",
				CreatedAt:  time.Now(),
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Status:       core.WorkflowStatusCompleted,
				Tasks:        make(map[core.TaskID]*core.TaskState),
			},
		}
		err = sm.Save(ctx, workflowState)
		require.NoError(t, err)

		newPurge = true
		newForce = false
		defer func() {
			newPurge = false
			newForce = false
		}()

		// Simulate user input "yes"
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			w.Write([]byte("yes\n"))
			w.Close()
		}()
		defer func() { os.Stdin = oldStdin }()

		// Capture stdout
		oldStdout := os.Stdout
		rOut, wOut, _ := os.Pipe()
		os.Stdout = wOut

		err = runNew(newCmd, []string{})

		wOut.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(rOut)
		output := buf.String()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(output, "Purged") || strings.Contains(output, "workflow"))
	})
}

func TestNewCommand(t *testing.T) {
	assert.NotNil(t, newCmd)
	assert.Equal(t, "new", newCmd.Use)

	// Verify flags
	flag := newCmd.Flags().Lookup("archive")
	assert.NotNil(t, flag)

	flag = newCmd.Flags().Lookup("purge")
	assert.NotNil(t, flag)

	flag = newCmd.Flags().Lookup("force")
	assert.NotNil(t, flag)
	assert.Equal(t, "f", flag.Shorthand)
}
