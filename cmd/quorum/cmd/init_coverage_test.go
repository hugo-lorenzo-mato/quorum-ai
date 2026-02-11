package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInit(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	t.Run("successful initialization", func(t *testing.T) {
		// Change to temp directory
		err := os.Chdir(tmpDir)
		require.NoError(t, err)

		// Reset force flag
		initForce = false

		// Run init
		err = runInit(initCmd, []string{})
		assert.NoError(t, err)

		// Verify .quorum directory was created
		quorumDir := filepath.Join(tmpDir, ".quorum")
		stat, err := os.Stat(quorumDir)
		assert.NoError(t, err)
		assert.True(t, stat.IsDir())

		// Verify config file was created
		configPath := filepath.Join(quorumDir, "config.yaml")
		stat, err = os.Stat(configPath)
		assert.NoError(t, err)
		assert.False(t, stat.IsDir())

		// Verify subdirectories were created
		subdirs := []string{"state", "logs", "runs"}
		for _, subdir := range subdirs {
			dirPath := filepath.Join(quorumDir, subdir)
			stat, err := os.Stat(dirPath)
			assert.NoError(t, err, "subdir %s should exist", subdir)
			assert.True(t, stat.IsDir(), "subdir %s should be a directory", subdir)
		}
	})

	t.Run("config already exists without force", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		err := os.Chdir(tmpDir2)
		require.NoError(t, err)

		// Create existing config
		quorumDir := filepath.Join(tmpDir2, ".quorum")
		err = os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("existing config"), 0600)
		require.NoError(t, err)

		initForce = false

		// Run init should fail
		err = runInit(initCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("config exists with force flag", func(t *testing.T) {
		tmpDir3 := t.TempDir()
		err := os.Chdir(tmpDir3)
		require.NoError(t, err)

		// Create existing config
		quorumDir := filepath.Join(tmpDir3, ".quorum")
		err = os.MkdirAll(quorumDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(quorumDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("old config"), 0600)
		require.NoError(t, err)

		initForce = true
		defer func() { initForce = false }()

		// Run init should succeed and overwrite
		err = runInit(initCmd, []string{})
		assert.NoError(t, err)

		// Verify config was overwritten
		content, err := os.ReadFile(configPath)
		assert.NoError(t, err)
		assert.NotEqual(t, "old config", string(content))
	})

	t.Run("legacy config warning", func(t *testing.T) {
		tmpDir4 := t.TempDir()
		err := os.Chdir(tmpDir4)
		require.NoError(t, err)

		// Create legacy config file
		legacyPath := filepath.Join(tmpDir4, ".quorum.yaml")
		err = os.WriteFile(legacyPath, []byte("legacy: true"), 0600)
		require.NoError(t, err)

		initForce = false

		// Run init - should succeed and print warning
		err = runInit(initCmd, []string{})
		assert.NoError(t, err)
	})
}

func TestInitializeAgentConfigs(t *testing.T) {
	// Create temporary home directory
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	t.Run("creates gemini config when missing", func(t *testing.T) {
		err := initializeAgentConfigs()
		assert.NoError(t, err)

		// Verify .gemini directory was created
		geminiDir := filepath.Join(tmpHome, ".gemini")
		stat, err := os.Stat(geminiDir)
		assert.NoError(t, err)
		assert.True(t, stat.IsDir())

		// Verify settings.json was created
		configPath := filepath.Join(geminiDir, "settings.json")
		stat, err = os.Stat(configPath)
		assert.NoError(t, err)

		// Verify config content
		data, err := os.ReadFile(configPath)
		assert.NoError(t, err)

		var config map[string]interface{}
		err = json.Unmarshal(data, &config)
		assert.NoError(t, err)
		assert.Contains(t, config, "security")
		assert.Contains(t, config, "ui")
		assert.Contains(t, config, "general")
	})

	t.Run("removes disabled flag from existing config", func(t *testing.T) {
		tmpHome2 := t.TempDir()
		os.Setenv("HOME", tmpHome2)

		// Create gemini config with "disabled": true
		geminiDir := filepath.Join(tmpHome2, ".gemini")
		err := os.MkdirAll(geminiDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(geminiDir, "settings.json")
		disabledConfig := map[string]interface{}{
			"disabled": true,
			"ui": map[string]interface{}{
				"theme": "dark",
			},
		}
		data, err := json.MarshalIndent(disabledConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, data, 0600)
		require.NoError(t, err)

		// Run initializeAgentConfigs
		err = initializeAgentConfigs()
		assert.NoError(t, err)

		// Verify "disabled" was removed
		data, err = os.ReadFile(configPath)
		assert.NoError(t, err)

		var config map[string]interface{}
		err = json.Unmarshal(data, &config)
		assert.NoError(t, err)
		_, hasDisabled := config["disabled"]
		assert.False(t, hasDisabled, "disabled flag should be removed")
		assert.Contains(t, config, "ui")
	})

	t.Run("preserves existing valid config", func(t *testing.T) {
		tmpHome3 := t.TempDir()
		os.Setenv("HOME", tmpHome3)

		// Create valid gemini config
		geminiDir := filepath.Join(tmpHome3, ".gemini")
		err := os.MkdirAll(geminiDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(geminiDir, "settings.json")
		validConfig := map[string]interface{}{
			"security": map[string]interface{}{
				"auth": map[string]interface{}{
					"selectedType": "custom",
				},
			},
			"custom": "value",
		}
		data, err := json.MarshalIndent(validConfig, "", "  ")
		require.NoError(t, err)

		err = os.WriteFile(configPath, data, 0600)
		require.NoError(t, err)

		// Run initializeAgentConfigs
		err = initializeAgentConfigs()
		assert.NoError(t, err)

		// Verify config was not changed
		data, err = os.ReadFile(configPath)
		assert.NoError(t, err)

		var config map[string]interface{}
		err = json.Unmarshal(data, &config)
		assert.NoError(t, err)
		assert.Equal(t, "value", config["custom"])
	})

	t.Run("handles invalid json in existing config", func(t *testing.T) {
		tmpHome4 := t.TempDir()
		os.Setenv("HOME", tmpHome4)

		// Create invalid gemini config
		geminiDir := filepath.Join(tmpHome4, ".gemini")
		err := os.MkdirAll(geminiDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(geminiDir, "settings.json")
		err = os.WriteFile(configPath, []byte("invalid json {{{"), 0600)
		require.NoError(t, err)

		// Should return error
		err = initializeAgentConfigs()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing existing gemini config")
	})
}

func TestInitCommand(t *testing.T) {
	assert.NotNil(t, initCmd)
	assert.Equal(t, "init", initCmd.Use)

	// Verify flags
	flag := initCmd.Flags().Lookup("force")
	assert.NotNil(t, flag)
	assert.Equal(t, "force", flag.Name)
}
