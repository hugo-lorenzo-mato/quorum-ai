package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Loader handles configuration loading from multiple sources.
type Loader struct {
	v          *viper.Viper
	configFile string
	envPrefix  string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{
		v:         viper.New(),
		envPrefix: "QUORUM",
	}
}

// NewLoaderWithViper creates a loader using an existing viper instance.
// This allows integration with CLI flag bindings.
func NewLoaderWithViper(v *viper.Viper) *Loader {
	return &Loader{
		v:         v,
		envPrefix: "QUORUM",
	}
}

// WithConfigFile sets an explicit config file path.
func (l *Loader) WithConfigFile(path string) *Loader {
	l.configFile = path
	return l
}

// WithEnvPrefix sets the environment variable prefix.
func (l *Loader) WithEnvPrefix(prefix string) *Loader {
	l.envPrefix = prefix
	return l
}

// Viper returns the underlying viper instance for flag binding.
func (l *Loader) Viper() *viper.Viper {
	return l.v
}

// Load loads configuration from all sources.
// Precedence (highest to lowest):
// 1. CLI flags (set via viper.BindPFlag)
// 2. Environment variables (QUORUM_*)
// 3. Project config (.quorum.yaml in current directory)
// 4. User config (~/.config/quorum/config.yaml)
// 5. Defaults
func (l *Loader) Load() (*Config, error) {
	// Set defaults first
	l.setDefaults()

	// Configure environment variable reading
	l.v.SetEnvPrefix(l.envPrefix)
	l.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	l.v.AutomaticEnv()

	// Config file setup
	if l.configFile != "" {
		l.v.SetConfigFile(l.configFile)
	} else {
		l.v.SetConfigName(".quorum")
		l.v.SetConfigType("yaml")

		// Add search paths in precedence order (first found wins)
		// Project config takes precedence over user config
		l.v.AddConfigPath(".")
		if home, err := os.UserHomeDir(); err == nil {
			l.v.AddConfigPath(filepath.Join(home, ".config", "quorum"))
		}
	}

	// Read config file (ignore not found)
	if err := l.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := l.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// setDefaults configures default values.
func (l *Loader) setDefaults() {
	// Log defaults
	l.v.SetDefault("log.level", "info")
	l.v.SetDefault("log.format", "auto")

	// Trace defaults
	l.v.SetDefault("trace.mode", "off")
	l.v.SetDefault("trace.dir", ".quorum/traces")
	l.v.SetDefault("trace.schema_version", 1)
	l.v.SetDefault("trace.redact", true)
	l.v.SetDefault("trace.redact_patterns", []string{})
	l.v.SetDefault("trace.redact_allowlist", []string{})
	l.v.SetDefault("trace.max_bytes", 262144)
	l.v.SetDefault("trace.total_max_bytes", 10485760)
	l.v.SetDefault("trace.max_files", 500)
	l.v.SetDefault("trace.include_phases", []string{"analyze", "consensus", "plan", "execute"})

	// Workflow defaults
	l.v.SetDefault("workflow.timeout", "2h")
	l.v.SetDefault("workflow.max_retries", 3)
	l.v.SetDefault("workflow.dry_run", false)
	l.v.SetDefault("workflow.sandbox", false)

	// Agent defaults
	l.v.SetDefault("agents.default", "claude")
	l.v.SetDefault("agents.claude.enabled", true)
	l.v.SetDefault("agents.claude.path", "claude")
	l.v.SetDefault("agents.claude.model", "claude-sonnet-4-20250514")
	l.v.SetDefault("agents.claude.max_tokens", 4096)
	l.v.SetDefault("agents.claude.temperature", 0.7)
	l.v.SetDefault("agents.gemini.enabled", true)
	l.v.SetDefault("agents.gemini.path", "gemini")
	l.v.SetDefault("agents.gemini.model", "gemini-2.5-flash")
	l.v.SetDefault("agents.gemini.max_tokens", 4096)
	l.v.SetDefault("agents.gemini.temperature", 0.7)
	l.v.SetDefault("agents.codex.enabled", false) // Optional: enable explicitly in config
	l.v.SetDefault("agents.codex.path", "codex")
	l.v.SetDefault("agents.codex.model", "gpt-5.1-codex")
	l.v.SetDefault("agents.codex.max_tokens", 4096)
	l.v.SetDefault("agents.codex.temperature", 0.7)
	l.v.SetDefault("agents.copilot.enabled", false)
	l.v.SetDefault("agents.copilot.path", "gh copilot")
	l.v.SetDefault("agents.copilot.max_tokens", 4096)
	l.v.SetDefault("agents.copilot.temperature", 0.7)
	l.v.SetDefault("agents.aider.enabled", false)
	l.v.SetDefault("agents.aider.path", "aider")
	l.v.SetDefault("agents.aider.model", "gpt-4")
	l.v.SetDefault("agents.aider.max_tokens", 4096)
	l.v.SetDefault("agents.aider.temperature", 0.7)

	// State defaults (unified under .quorum/)
	l.v.SetDefault("state.path", ".quorum/state/state.json")
	l.v.SetDefault("state.backup_path", ".quorum/state/state.json.bak")
	l.v.SetDefault("state.lock_ttl", "1h")

	// Git defaults
	l.v.SetDefault("git.worktree_dir", ".worktrees")
	l.v.SetDefault("git.auto_clean", true)
	l.v.SetDefault("git.worktree_mode", "always")

	// GitHub defaults
	l.v.SetDefault("github.remote", "origin")

	// Consensus defaults (80/60/50 escalation policy)
	l.v.SetDefault("consensus.threshold", 0.80)
	l.v.SetDefault("consensus.v2_threshold", 0.60)
	l.v.SetDefault("consensus.human_threshold", 0.50)
	l.v.SetDefault("consensus.weights.claims", 0.40)
	l.v.SetDefault("consensus.weights.risks", 0.30)
	l.v.SetDefault("consensus.weights.recommendations", 0.30)

	// Costs defaults
	l.v.SetDefault("costs.max_per_workflow", 10.0)
	l.v.SetDefault("costs.max_per_task", 2.0)
	l.v.SetDefault("costs.alert_threshold", 0.80)
}

// ConfigFile returns the config file path if one was used.
func (l *Loader) ConfigFile() string {
	return l.v.ConfigFileUsed()
}

// Get returns a configuration value by key.
func (l *Loader) Get(key string) interface{} {
	return l.v.Get(key)
}

// Set sets a configuration value.
func (l *Loader) Set(key string, value interface{}) {
	l.v.Set(key, value)
}

// IsSet checks if a key has been set.
func (l *Loader) IsSet(key string) bool {
	return l.v.IsSet(key)
}

// AllSettings returns all settings as a map.
func (l *Loader) AllSettings() map[string]interface{} {
	return l.v.AllSettings()
}
