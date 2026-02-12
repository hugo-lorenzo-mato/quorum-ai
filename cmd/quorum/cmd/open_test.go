package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

func TestRunOpen(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	t.Run("open current directory", func(t *testing.T) {
		// Create test directory
		testDir := filepath.Join(tmpDir, "test-project")
		err := os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		oldDir, _ := os.Getwd()
		defer os.Chdir(oldDir)

		err = os.Chdir(testDir)
		require.NoError(t, err)

		openForce = false
		openInheritGlobal = false
		quiet = false
		defer func() {
			quiet = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runOpen(openCmd, []string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Initialized")

		// Verify .quorum directory was created
		quorumDir := filepath.Join(testDir, ".quorum")
		stat, err := os.Stat(quorumDir)
		assert.NoError(t, err)
		assert.True(t, stat.IsDir())
	})

	t.Run("open specific directory", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "specific-project")
		err := os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		openForce = false
		openInheritGlobal = false
		quiet = false

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runOpen(openCmd, []string{testDir})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		assert.NoError(t, err)

		// Verify .quorum directory was created
		quorumDir := filepath.Join(testDir, ".quorum")
		stat, err := os.Stat(quorumDir)
		assert.NoError(t, err)
		assert.True(t, stat.IsDir())
	})

	t.Run("open non-existent directory", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "does-not-exist")

		err := runOpen(openCmd, []string{nonExistent})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("open file instead of directory", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "testfile.txt")
		err := os.WriteFile(testFile, []byte("test"), 0600)
		require.NoError(t, err)

		err = runOpen(openCmd, []string{testFile})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("open with custom name", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "named-project")
		err := os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		openProjectName = "My Custom Project"
		openForce = false
		openInheritGlobal = false
		quiet = true
		defer func() {
			openProjectName = ""
			openInheritGlobal = false
			quiet = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runOpen(openCmd, []string{testDir})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)

		assert.NoError(t, err)
	})

	t.Run("open already initialized", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "already-init")
		err := os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		// Create .quorum directory first
		quorumDir := filepath.Join(testDir, ".quorum")
		err = os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte(config.DefaultConfigYAML), 0600)
		require.NoError(t, err)

		openForce = false
		openInheritGlobal = false
		quiet = false
		defer func() {
			quiet = false
		}()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runOpen(openCmd, []string{testDir})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "already initialized")
	})

	t.Run("open with inherit-global", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "inherit-global")
		err := os.MkdirAll(testDir, 0o755)
		require.NoError(t, err)

		openInheritGlobal = true
		openForce = true
		quiet = true
		defer func() {
			openInheritGlobal = false
			openForce = false
			quiet = false
		}()

		err = runOpen(openCmd, []string{testDir})
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(testDir, ".quorum"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(testDir, ".quorum", "config.yaml"))
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		globalPath, err := config.GlobalConfigPath()
		require.NoError(t, err)
		_, err = os.Stat(globalPath)
		require.NoError(t, err)

		registry, err := project.NewFileRegistry()
		require.NoError(t, err)
		defer registry.Close()

		p, err := registry.GetProjectByPath(t.Context(), testDir)
		require.NoError(t, err)
		assert.Equal(t, project.ConfigModeInheritGlobal, p.ConfigMode)
	})
}

func TestInitializeProject(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("initialize new project", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "new-project")
		err := os.MkdirAll(projectDir, 0755)
		require.NoError(t, err)

		err = initializeProject(projectDir, false)
		assert.NoError(t, err)

		// Verify structure
		quorumDir := filepath.Join(projectDir, ".quorum")
		stat, err := os.Stat(quorumDir)
		assert.NoError(t, err)
		assert.True(t, stat.IsDir())

		configPath := filepath.Join(quorumDir, "config.yaml")
		_, err = os.Stat(configPath)
		assert.NoError(t, err)

		subdirs := []string{"state", "logs", "runs"}
		for _, subdir := range subdirs {
			dirPath := filepath.Join(quorumDir, subdir)
			stat, err := os.Stat(dirPath)
			assert.NoError(t, err, "subdir %s should exist", subdir)
			assert.True(t, stat.IsDir(), "subdir %s should be a directory", subdir)
		}
	})

	t.Run("initialize already exists without force", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "exists-no-force")
		quorumDir := filepath.Join(projectDir, ".quorum")
		err := os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("existing"), 0600)
		require.NoError(t, err)

		err = initializeProject(projectDir, false)
		assert.NoError(t, err)

		// Config should not be overwritten
		content, err := os.ReadFile(configPath)
		assert.NoError(t, err)
		assert.Equal(t, "existing", string(content))
	})

	t.Run("initialize with force overwrites", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "exists-force")
		quorumDir := filepath.Join(projectDir, ".quorum")
		err := os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("old content"), 0600)
		require.NoError(t, err)

		err = initializeProject(projectDir, true)
		assert.NoError(t, err)

		// Config should be overwritten
		content, err := os.ReadFile(configPath)
		assert.NoError(t, err)
		assert.NotEqual(t, "old content", string(content))
	})
}

func TestOpenCommand(t *testing.T) {
	assert.NotNil(t, openCmd)
	assert.Equal(t, "open [path]", openCmd.Use)

	// Verify flags
	flags := []string{"name", "color", "default", "force", "inherit-global"}
	for _, flagName := range flags {
		flag := openCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "flag %s should exist", flagName)
	}
}
