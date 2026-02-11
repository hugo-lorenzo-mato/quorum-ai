package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTraceCmd(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("list with no traces", func(t *testing.T) {
		traceDir := filepath.Join(tmpDir, "empty-traces")
		err := os.MkdirAll(traceDir, 0755)
		require.NoError(t, err)

		traceDirFlag = traceDir
		traceList = true
		traceLimit = 10
		defer func() {
			traceDirFlag = ""
			traceList = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTraceCmd(traceCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "No traces found")
	})

	t.Run("show trace summary", func(t *testing.T) {
		// Create trace directory with manifest
		traceDir := filepath.Join(tmpDir, "with-traces")
		runID := "test-run-123"
		runDir := filepath.Join(traceDir, runID)
		err := os.MkdirAll(runDir, 0755)
		require.NoError(t, err)

		// Create manifest
		manifest := traceManifestView{
			RunID:      runID,
			WorkflowID: "test-workflow",
			StartedAt:  time.Now().Add(-time.Hour),
			EndedAt:    time.Now(),
			AppVersion: "v1.0.0",
			AppCommit:  "abc123",
			AppDate:    "2024-01-01",
			GitCommit:  "def456",
			GitDirty:   false,
			Config: traceConfigView{
				Mode: "full",
				Dir:  traceDir,
			},
			Summary: traceSummaryView{
				TotalPrompts:   5,
				TotalTokensIn:  1000,
				TotalTokensOut: 2000,
				TotalFiles:     10,
				TotalBytes:     50000,
			},
		}

		data, err := json.Marshal(manifest)
		require.NoError(t, err)

		manifestPath := filepath.Join(runDir, "run.json")
		err = os.WriteFile(manifestPath, data, 0600)
		require.NoError(t, err)

		traceDirFlag = traceDir
		traceRunID = runID
		traceJSON = false
		defer func() {
			traceDirFlag = ""
			traceRunID = ""
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTraceCmd(traceCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, runID)
		assert.Contains(t, output, "test-workflow")
		assert.Contains(t, output, "v1.0.0")
		assert.Contains(t, output, "abc123")
	})

	t.Run("json output", func(t *testing.T) {
		// Create trace directory with manifest
		traceDir := filepath.Join(tmpDir, "json-traces")
		runID := "test-run-json"
		runDir := filepath.Join(traceDir, runID)
		err := os.MkdirAll(runDir, 0755)
		require.NoError(t, err)

		manifest := traceManifestView{
			RunID:      runID,
			WorkflowID: "json-workflow",
			StartedAt:  time.Now(),
			EndedAt:    time.Now(),
		}

		data, err := json.Marshal(manifest)
		require.NoError(t, err)

		manifestPath := filepath.Join(runDir, "run.json")
		err = os.WriteFile(manifestPath, data, 0600)
		require.NoError(t, err)

		traceDirFlag = traceDir
		traceRunID = runID
		traceJSON = true
		defer func() {
			traceDirFlag = ""
			traceRunID = ""
			traceJSON = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTraceCmd(traceCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "json-workflow")
	})

	t.Run("list with limit", func(t *testing.T) {
		// Create multiple traces
		traceDir := filepath.Join(tmpDir, "multi-traces")
		err := os.MkdirAll(traceDir, 0755)
		require.NoError(t, err)

		// Create 5 traces
		for i := 0; i < 5; i++ {
			runID := time.Now().Add(time.Duration(i) * time.Second).Format("20060102-150405")
			runDir := filepath.Join(traceDir, runID)
			err := os.MkdirAll(runDir, 0755)
			require.NoError(t, err)

			manifest := traceManifestView{
				RunID:      runID,
				WorkflowID: "workflow-" + runID,
				StartedAt:  time.Now().Add(time.Duration(i) * time.Hour),
				EndedAt:    time.Now(),
			}

			data, err := json.Marshal(manifest)
			require.NoError(t, err)

			manifestPath := filepath.Join(runDir, "run.json")
			err = os.WriteFile(manifestPath, data, 0600)
			require.NoError(t, err)
		}

		traceDirFlag = traceDir
		traceList = true
		traceLimit = 3
		defer func() {
			traceDirFlag = ""
			traceList = false
			traceLimit = 10
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTraceCmd(traceCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		assert.NoError(t, err)
	})
}

func TestResolveTraceDir(t *testing.T) {
	tests := []struct {
		name     string
		override string
		config   string
		expected string
	}{
		{
			name:     "with override",
			override: "/custom/trace/dir",
			expected: "/custom/trace/dir",
		},
		{
			name:     "empty override uses viper",
			override: "",
			config:   "/config/trace/dir",
			expected: "/config/trace/dir",
		},
		{
			name:     "both empty uses default",
			override: "",
			config:   "",
			expected: ".quorum/traces",
		},
		{
			name:     "whitespace override ignored",
			override: "   ",
			config:   "/config/dir",
			expected: "/config/dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			if tt.config != "" {
				viper.Set("trace.dir", tt.config)
			}
			defer viper.Reset()

			result := resolveTraceDir(tt.override)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "-",
		},
		{
			name:     "valid time",
			input:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2024-01-01T12:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTraceCommand(t *testing.T) {
	assert.NotNil(t, traceCmd)
	assert.Equal(t, "trace", traceCmd.Use)

	// Verify flags
	flags := []string{"dir", "run-id", "list", "limit", "json"}
	for _, flagName := range flags {
		flag := traceCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "flag %s should exist", flagName)
	}
}

func TestListTraceEntries(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("non-existent directory", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "does-not-exist")
		entries, err := listTraceEntries(nonExistent)
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("directory with traces", func(t *testing.T) {
		traceDir := filepath.Join(tmpDir, "traces")
		err := os.MkdirAll(traceDir, 0755)
		require.NoError(t, err)

		// Create a valid trace
		runID := "test-trace"
		runDir := filepath.Join(traceDir, runID)
		err = os.MkdirAll(runDir, 0755)
		require.NoError(t, err)

		manifest := traceManifestView{
			RunID:      runID,
			WorkflowID: "test",
			StartedAt:  time.Now(),
		}

		data, err := json.Marshal(manifest)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(runDir, "run.json"), data, 0600)
		require.NoError(t, err)

		entries, err := listTraceEntries(traceDir)
		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, runID, entries[0].RunID)
	})

	t.Run("ignores files", func(t *testing.T) {
		traceDir := filepath.Join(tmpDir, "mixed")
		err := os.MkdirAll(traceDir, 0755)
		require.NoError(t, err)

		// Create a file (should be ignored)
		err = os.WriteFile(filepath.Join(traceDir, "somefile.txt"), []byte("test"), 0600)
		require.NoError(t, err)

		entries, err := listTraceEntries(traceDir)
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})
}
