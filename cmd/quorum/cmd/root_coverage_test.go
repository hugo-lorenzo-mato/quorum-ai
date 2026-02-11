package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	// Save and restore flags
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test basic execution with help flag
	os.Args = []string{"quorum", "--help"}
	err := Execute()
	// Help returns nil, but cobra handles the output
	assert.NoError(t, err)
}

func TestGetVersionFunction(t *testing.T) {
	// Set a known version
	SetVersion("test-version-func", "test-commit", "test-date")

	version := GetVersion()
	assert.Equal(t, "test-version-func", version)
}

func TestInitConfig(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	t.Run("no config file", func(t *testing.T) {
		// Reset viper
		viper.Reset()
		cfgFile = ""
		projectID = ""

		err := os.Chdir(tmpDir)
		require.NoError(t, err)

		err = initConfig()
		// Should succeed even without config file
		assert.NoError(t, err)
	})

	t.Run("with config file", func(t *testing.T) {
		viper.Reset()

		// Create a config file
		quorumDir := filepath.Join(tmpDir, ".quorum")
		err := os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("log:\n  level: debug\n"), 0600)
		require.NoError(t, err)

		cfgFile = configPath
		err = initConfig()
		assert.NoError(t, err)

		// Verify config was loaded
		level := viper.GetString("log.level")
		assert.Equal(t, "debug", level)
	})

	t.Run("with project flag", func(t *testing.T) {
		viper.Reset()
		cfgFile = ""

		// Create project directory with config
		projectDir := filepath.Join(tmpDir, "test-project")
		quorumDir := filepath.Join(projectDir, ".quorum")
		err := os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("log:\n  level: info\n"), 0600)
		require.NoError(t, err)

		// Set project flag to absolute path
		projectID = projectDir

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		err = initConfig()
		assert.NoError(t, err)

		projectID = ""
	})

	t.Run("invalid config file", func(t *testing.T) {
		viper.Reset()

		invalidPath := filepath.Join(tmpDir, "invalid.yaml")
		err := os.WriteFile(invalidPath, []byte("invalid: yaml: [[["), 0600)
		require.NoError(t, err)

		cfgFile = invalidPath
		err = initConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reading config")
	})
}

func TestTryResolveProjectPath(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "absolute path",
			value:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			value:    "./relative/path",
			expected: "./relative/path",
		},
		{
			name:     "project name without path",
			value:    "my-project",
			expected: "",
		},
		{
			name:     "empty value",
			value:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tryResolveProjectPath(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRootCommand(t *testing.T) {
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "quorum", rootCmd.Use)
	assert.True(t, rootCmd.SilenceUsage)
	assert.True(t, rootCmd.SilenceErrors)
}

func TestRootCommandFlags(t *testing.T) {
	// Test that persistent flags are registered
	flag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, flag)
	assert.Equal(t, "config", flag.Name)

	flag = rootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, flag)
	assert.Equal(t, "log-level", flag.Name)

	flag = rootCmd.PersistentFlags().Lookup("log-format")
	assert.NotNil(t, flag)
	assert.Equal(t, "log-format", flag.Name)

	flag = rootCmd.PersistentFlags().Lookup("no-color")
	assert.NotNil(t, flag)
	assert.Equal(t, "no-color", flag.Name)

	flag = rootCmd.PersistentFlags().Lookup("quiet")
	assert.NotNil(t, flag)
	assert.Equal(t, "quiet", flag.Name)
	assert.Equal(t, "q", flag.Shorthand)

	flag = rootCmd.PersistentFlags().Lookup("project")
	assert.NotNil(t, flag)
	assert.Equal(t, "project", flag.Name)
}

func TestRootCommandPersistentPreRunE(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Reset viper and config
	viper.Reset()
	cfgFile = ""
	projectID = ""

	// Test PersistentPreRunE
	err = rootCmd.PersistentPreRunE(rootCmd, []string{})
	assert.NoError(t, err)
}
