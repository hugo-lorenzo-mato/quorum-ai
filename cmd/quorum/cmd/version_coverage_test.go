package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	// Set version info
	SetVersion("v1.2.3", "abc123def", "2024-01-15")

	t.Run("version command output", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run version command
		versionCmd.Run(versionCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		var buf bytes.Buffer
		_, err := buf.ReadFrom(r)
		require.NoError(t, err)

		output := buf.String()

		// Verify output contains version info
		assert.Contains(t, output, "v1.2.3")
		assert.Contains(t, output, "abc123def")
		assert.Contains(t, output, "2024-01-15")
		assert.Contains(t, output, "quorum-ai")
		assert.Contains(t, output, "commit:")
		assert.Contains(t, output, "built:")
	})

	t.Run("version command properties", func(t *testing.T) {
		assert.NotNil(t, versionCmd)
		assert.Equal(t, "version", versionCmd.Use)
		assert.Equal(t, "Print version information", versionCmd.Short)
		assert.NotNil(t, versionCmd.Run)
	})

	t.Run("version with empty values", func(t *testing.T) {
		SetVersion("", "", "")

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run version command
		versionCmd.Run(versionCmd, []string{})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		var buf bytes.Buffer
		_, err := buf.ReadFrom(r)
		require.NoError(t, err)

		output := buf.String()

		// Should still have output structure even with empty values
		assert.Contains(t, output, "quorum-ai")
		assert.Contains(t, output, "commit:")
		assert.Contains(t, output, "built:")
	})
}

func TestVersionCommandRegistered(t *testing.T) {
	// Verify version command is registered as subcommand
	commands := rootCmd.Commands()
	var found bool
	for _, cmd := range commands {
		if cmd.Use == "version" {
			found = true
			break
		}
	}
	assert.True(t, found, "version command should be registered with root command")
}
