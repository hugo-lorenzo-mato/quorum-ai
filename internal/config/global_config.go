package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// GlobalConfigPath returns the default global configuration path used by the WebUI server.
//
// This is separate from per-project `.quorum/config.yaml` files and acts as the default
// configuration for projects whose config_mode is "inherit_global".
func GlobalConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	registryDir := filepath.Join(homeDir, ".quorum-registry")
	return filepath.Join(registryDir, "global-config.yaml"), nil
}

// EnsureGlobalConfigFile ensures the global configuration file exists on disk.
// If it does not exist, it is created using DefaultConfigYAML.
func EnsureGlobalConfigFile() (string, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return "", err
	}

	if _, statErr := os.Stat(path); statErr == nil {
		return path, nil
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("checking global config: %w", statErr)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("creating global config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(DefaultConfigYAML), 0o600); err != nil {
		return "", fmt.Errorf("creating global config: %w", err)
	}

	return path, nil
}
