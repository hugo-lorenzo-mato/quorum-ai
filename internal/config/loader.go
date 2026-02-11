package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Loader handles configuration loading from multiple sources.
type Loader struct {
	v              *viper.Viper
	configFile     string
	envPrefix      string
	projectDir     string     // Resolved project root directory (set by Load)
	projectDirHint string     // Optional: override project root directory for path resolution
	resolvePaths   bool       // Whether to resolve relative paths to absolute on Load
	mu             sync.Mutex // Protects concurrent access to viper operations
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{
		v:            viper.New(),
		envPrefix:    "QUORUM",
		resolvePaths: true,
	}
}

// NewLoaderWithViper creates a loader using an existing viper instance.
// This allows integration with CLI flag bindings.
func NewLoaderWithViper(v *viper.Viper) *Loader {
	return &Loader{
		v:            v,
		envPrefix:    "QUORUM",
		resolvePaths: true,
	}
}

// WithConfigFile sets an explicit config file path.
func (l *Loader) WithConfigFile(path string) *Loader {
	l.configFile = path
	return l
}

// WithProjectDir provides a project root directory hint for resolving relative paths.
// This is required for scenarios where the config file is not located under the project
// root (e.g. a global config shared by many projects).
func (l *Loader) WithProjectDir(path string) *Loader {
	l.projectDirHint = path
	return l
}

// WithResolvePaths controls whether relative paths are resolved to absolute paths on Load().
// For API editing endpoints, you typically want resolvePaths=false to preserve relative values.
func (l *Loader) WithResolvePaths(resolve bool) *Loader {
	l.resolvePaths = resolve
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
// 3. Project config (.quorum/config.yaml - new location)
// 4. Legacy project config (.quorum.yaml - for backwards compatibility)
// 5. User config (~/.config/quorum/config.yaml)
// 6. Defaults
func (l *Loader) Load() (*Config, error) {
	// Lock to prevent concurrent map writes in viper
	l.mu.Lock()
	defer l.mu.Unlock()

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
		// Try new location first: .quorum/config.yaml
		newConfigPath := filepath.Join(".quorum", "config.yaml")
		if _, err := os.Stat(newConfigPath); err == nil {
			l.v.SetConfigFile(newConfigPath)
		} else {
			// Fall back to legacy location: .quorum.yaml
			l.v.SetConfigName(".quorum")
			l.v.SetConfigType("yaml")

			// Add search paths in precedence order (first found wins)
			// Project config takes precedence over user config
			l.v.AddConfigPath(".")
			if home, err := os.UserHomeDir(); err == nil {
				l.v.AddConfigPath(filepath.Join(home, ".config", "quorum"))
			}
		}
	}

	// Read config file (ignore not found)
	if err := l.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// ignore
		} else if errors.Is(err, os.ErrNotExist) {
			// Explicit config file path does not exist: treat as "no config file" and fall back to defaults.
		} else {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	// Normalize legacy keys from config file (e.g., maxretries -> max_retries)
	if configPath := l.v.ConfigFileUsed(); configPath != "" {
		// If we were given an explicit config file path that doesn't exist, viper may still
		// report it as "used". Skip normalization in that case.
		if _, err := os.Stat(configPath); err == nil {
			normalized, err := loadNormalizedConfigMap(configPath)
			if err != nil {
				return nil, fmt.Errorf("normalizing config: %w", err)
			}
			if len(normalized) > 0 {
				if err := l.v.MergeConfigMap(normalized); err != nil {
					return nil, fmt.Errorf("merging normalized config: %w", err)
				}
			}
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := l.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Resolve all relative paths to absolute paths
	// Use the project root (parent of .quorum/) as the base for relative paths
	projectDir := ""
	if configPath := l.v.ConfigFileUsed(); configPath != "" {
		absConfigPath, err := filepath.Abs(configPath)
		if err == nil {
			configDir := filepath.Dir(absConfigPath)
			// If config is in .quorum/ directory, use its parent as project root
			// e.g., /project/.quorum/config.yaml -> /project/
			if filepath.Base(configDir) == ".quorum" {
				projectDir = filepath.Dir(configDir)
			} else {
				// Legacy .quorum.yaml in project root
				projectDir = configDir
			}
		}
	}
	// If no config file found, fall back to current working directory
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	// Override project dir when caller provides a hint (e.g. global config shared by many projects).
	if strings.TrimSpace(l.projectDirHint) != "" {
		projectDir = l.projectDirHint
	}
	l.projectDir = projectDir
	if l.resolvePaths {
		l.resolveAbsolutePaths(&cfg, projectDir)
	}

	return &cfg, nil
}

// ProjectDir returns the resolved project root directory.
// This is the directory containing the .quorum/ config folder (or CWD as fallback).
// Available after Load() has been called.
func (l *Loader) ProjectDir() string {
	return l.projectDir
}

// resolveAbsolutePaths converts all relative paths in the config to absolute paths.
// Relative paths are resolved relative to baseDir (typically the config file's directory).
// This prevents issues when quorum is executed from different working directories.
func (l *Loader) resolveAbsolutePaths(cfg *Config, baseDir string) {
	// State paths
	if cfg.State.Path != "" {
		cfg.State.Path = resolvePathRelativeTo(cfg.State.Path, baseDir)
	}
	if cfg.State.BackupPath != "" {
		cfg.State.BackupPath = resolvePathRelativeTo(cfg.State.BackupPath, baseDir)
	}

	// Trace directory
	if cfg.Trace.Dir != "" {
		cfg.Trace.Dir = resolvePathRelativeTo(cfg.Trace.Dir, baseDir)
	}

	// Report base directory
	if cfg.Report.BaseDir != "" {
		cfg.Report.BaseDir = resolvePathRelativeTo(cfg.Report.BaseDir, baseDir)
	}

	// Diagnostics crash dump directory
	if cfg.Diagnostics.CrashDump.Dir != "" {
		cfg.Diagnostics.CrashDump.Dir = resolvePathRelativeTo(cfg.Diagnostics.CrashDump.Dir, baseDir)
	}

	// Git worktree directory
	if cfg.Git.Worktree.Dir != "" {
		cfg.Git.Worktree.Dir = resolvePathRelativeTo(cfg.Git.Worktree.Dir, baseDir)
	}
}

// resolvePathRelativeTo converts a relative path to an absolute path using baseDir as the base.
// If the path is already absolute, it is returned unchanged.
// Example: resolvePathRelativeTo(".quorum/state.db", "/home/user/project")
//
//	→ "/home/user/project/.quorum/state.db"
func resolvePathRelativeTo(path, baseDir string) string {
	// Check for absolute paths (including Unix-style paths on Windows)
	if filepath.IsAbs(path) {
		return path
	}
	// On Windows, filepath.IsAbs("/unix/path") returns false
	// But such paths should be treated as absolute
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return path
	}
	return filepath.Join(baseDir, path)
}

func loadNormalizedConfigMap(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	normalizeLegacyConfigMap(raw)
	return raw, nil
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
	l.v.SetDefault("trace.include_phases", []string{"analyze", "plan", "execute"})

	// Workflow defaults
	l.v.SetDefault("workflow.timeout", "16h")
	l.v.SetDefault("workflow.max_retries", 3)
	l.v.SetDefault("workflow.dry_run", false)
	l.v.SetDefault("workflow.heartbeat.enabled", true) // Always true; kept for backwards compat
	l.v.SetDefault("workflow.heartbeat.interval", "30s")
	l.v.SetDefault("workflow.heartbeat.stale_threshold", "2m")
	l.v.SetDefault("workflow.heartbeat.check_interval", "60s")
	l.v.SetDefault("workflow.heartbeat.auto_resume", true)
	l.v.SetDefault("workflow.heartbeat.max_resumes", 1)

	// Phase defaults
	// Analyze phase
	l.v.SetDefault("phases.analyze.timeout", "8h")
	l.v.SetDefault("phases.analyze.refiner.enabled", true)
	l.v.SetDefault("phases.analyze.refiner.agent", "")
	l.v.SetDefault("phases.analyze.refiner.template", "refine-prompt-v2")
	l.v.SetDefault("phases.analyze.moderator.enabled", true)
	l.v.SetDefault("phases.analyze.moderator.agent", "")
	l.v.SetDefault("phases.analyze.moderator.threshold", 0.80)
	l.v.SetDefault("phases.analyze.moderator.min_successful_agents", 2)
	l.v.SetDefault("phases.analyze.moderator.min_rounds", 2)
	l.v.SetDefault("phases.analyze.moderator.max_rounds", 3)
	l.v.SetDefault("phases.analyze.moderator.warning_threshold", 0.30)
	l.v.SetDefault("phases.analyze.moderator.stagnation_threshold", 0.02)
	l.v.SetDefault("phases.analyze.synthesizer.agent", "")
	// Single-agent mode defaults (bypasses multi-agent consensus)
	l.v.SetDefault("phases.analyze.single_agent.enabled", false)
	l.v.SetDefault("phases.analyze.single_agent.agent", "")
	l.v.SetDefault("phases.analyze.single_agent.model", "")

	// Plan phase
	l.v.SetDefault("phases.plan.timeout", "1h")
	l.v.SetDefault("phases.plan.synthesizer.enabled", false)
	l.v.SetDefault("phases.plan.synthesizer.agent", "")

	// Execute phase
	l.v.SetDefault("phases.execute.timeout", "2h")

	// Agent defaults
	// NOTE: agents.default has NO default - user must explicitly configure it
	// NOTE: agent models have NO defaults - user must explicitly configure them
	l.v.SetDefault("agents.default", "")
	l.v.SetDefault("agents.claude.enabled", false)
	l.v.SetDefault("agents.claude.path", "claude")
	l.v.SetDefault("agents.claude.model", "")
	l.v.SetDefault("agents.claude.max_tokens", 4096)
	l.v.SetDefault("agents.claude.temperature", 0.7)
	l.v.SetDefault("agents.gemini.enabled", false)
	l.v.SetDefault("agents.gemini.path", "gemini")
	l.v.SetDefault("agents.gemini.model", "")
	l.v.SetDefault("agents.gemini.max_tokens", 4096)
	l.v.SetDefault("agents.gemini.temperature", 0.7)
	l.v.SetDefault("agents.codex.enabled", false)
	l.v.SetDefault("agents.codex.path", "codex")
	l.v.SetDefault("agents.codex.model", "")
	l.v.SetDefault("agents.codex.max_tokens", 4096)
	l.v.SetDefault("agents.codex.temperature", 0.7)
	l.v.SetDefault("agents.copilot.enabled", false)
	l.v.SetDefault("agents.copilot.path", "copilot")
	l.v.SetDefault("agents.copilot.model", "")
	l.v.SetDefault("agents.copilot.max_tokens", 16384)
	l.v.SetDefault("agents.copilot.temperature", 0.7)
	l.v.SetDefault("agents.opencode.enabled", false)
	l.v.SetDefault("agents.opencode.path", "opencode")
	l.v.SetDefault("agents.opencode.model", "")
	l.v.SetDefault("agents.opencode.max_tokens", 16384)
	l.v.SetDefault("agents.opencode.temperature", 0.7)

	// State defaults
	l.v.SetDefault("state.path", ".quorum/state/state.db")
	l.v.SetDefault("state.backup_path", ".quorum/state/state.db.bak")
	l.v.SetDefault("state.lock_ttl", "1h")

	// Git defaults
	l.v.SetDefault("git.worktree.dir", ".worktrees")
	l.v.SetDefault("git.worktree.auto_clean", false) // Must be false when task.auto_commit is false to preserve changes
	l.v.SetDefault("git.worktree.mode", "always")
	l.v.SetDefault("git.task.auto_commit", true) // Commit changes after task completion

	// GitHub defaults
	l.v.SetDefault("github.remote", "origin")

	// Chat defaults (TUI interactive chat)
	l.v.SetDefault("chat.timeout", "3m")
	l.v.SetDefault("chat.progress_interval", "15s")
	l.v.SetDefault("chat.editor", "vim")

	// Report defaults (markdown report generation)
	l.v.SetDefault("report.enabled", true)
	l.v.SetDefault("report.base_dir", ".quorum/runs")
	l.v.SetDefault("report.use_utc", true)
	l.v.SetDefault("report.include_raw", true)

	// Diagnostics defaults (system monitoring and crash recovery)
	l.v.SetDefault("diagnostics.enabled", true)
	l.v.SetDefault("diagnostics.resource_monitoring.interval", "30s")
	l.v.SetDefault("diagnostics.resource_monitoring.fd_threshold_percent", 80)
	l.v.SetDefault("diagnostics.resource_monitoring.goroutine_threshold", 10000)
	l.v.SetDefault("diagnostics.resource_monitoring.memory_threshold_mb", 4096)
	l.v.SetDefault("diagnostics.resource_monitoring.history_size", 120) // 1 hour at 30s intervals
	l.v.SetDefault("diagnostics.crash_dump.dir", ".quorum/crashdumps")
	l.v.SetDefault("diagnostics.crash_dump.max_files", 10)
	l.v.SetDefault("diagnostics.crash_dump.include_stack", true)
	l.v.SetDefault("diagnostics.crash_dump.include_env", false)
	l.v.SetDefault("diagnostics.preflight_checks.enabled", true)
	l.v.SetDefault("diagnostics.preflight_checks.min_free_fd_percent", 20)
	l.v.SetDefault("diagnostics.preflight_checks.min_free_memory_mb", 256)

	// Issue generation defaults
	l.v.SetDefault("issues.enabled", true)
	l.v.SetDefault("issues.provider", "github")
	l.v.SetDefault("issues.auto_generate", false)
	l.v.SetDefault("issues.mode", "direct")
	l.v.SetDefault("issues.draft_directory", "")
	l.v.SetDefault("issues.repository", "")
	l.v.SetDefault("issues.parent_prompt", "")
	l.v.SetDefault("issues.prompt.language", "english")
	l.v.SetDefault("issues.prompt.tone", "professional")
	l.v.SetDefault("issues.prompt.include_diagrams", true)
	l.v.SetDefault("issues.prompt.title_format", "[quorum] {task_name}")
	l.v.SetDefault("issues.prompt.body_prompt_file", "")
	l.v.SetDefault("issues.prompt.convention", "")
	l.v.SetDefault("issues.prompt.custom_instructions", "")
	l.v.SetDefault("issues.labels", []string{"quorum-generated"})
	l.v.SetDefault("issues.assignees", []string{})
	l.v.SetDefault("issues.gitlab.use_epics", false)
	l.v.SetDefault("issues.gitlab.project_id", "")
	l.v.SetDefault("issues.generator.reasoning_effort", "")
	l.v.SetDefault("issues.generator.instructions", "")
	l.v.SetDefault("issues.generator.title_instructions", "")
	l.v.SetDefault("issues.generator.resilience.enabled", true)
	l.v.SetDefault("issues.generator.resilience.max_retries", 3)
	l.v.SetDefault("issues.generator.resilience.initial_backoff", "1s")
	l.v.SetDefault("issues.generator.resilience.max_backoff", "30s")
	l.v.SetDefault("issues.generator.resilience.backoff_multiplier", 2.0)
	l.v.SetDefault("issues.generator.resilience.failure_threshold", 3)
	l.v.SetDefault("issues.generator.resilience.reset_timeout", "30s")
}

// ConfigFile returns the config file path if one was used.
func (l *Loader) ConfigFile() string {
	if l.configFile != "" {
		return l.configFile
	}
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

// Validate checks configuration consistency and returns an error if invalid.
// This provides fail-fast validation for agent references and model configuration.
//
//nolint:gocyclo // Validation is intentionally explicit for clearer errors.
func Validate(cfg *Config) error {
	// Validate default agent
	if cfg.Agents.Default == "" {
		return fmt.Errorf("agents.default is required")
	}
	defaultAgent := cfg.Agents.GetAgentConfig(cfg.Agents.Default)
	if defaultAgent == nil {
		return fmt.Errorf("agents.default references unknown agent %q", cfg.Agents.Default)
	}
	if !defaultAgent.Enabled {
		return fmt.Errorf("agents.default references disabled agent %q", cfg.Agents.Default)
	}

	// Validate phases.analyze.refiner if enabled
	if cfg.Phases.Analyze.Refiner.Enabled {
		if cfg.Phases.Analyze.Refiner.Agent == "" {
			return fmt.Errorf("phases.analyze.refiner.agent is required when refiner is enabled")
		}
		agent := cfg.Agents.GetAgentConfig(cfg.Phases.Analyze.Refiner.Agent)
		if agent == nil {
			return fmt.Errorf("phases.analyze.refiner.agent references unknown agent %q", cfg.Phases.Analyze.Refiner.Agent)
		}
		if !agent.Enabled {
			return fmt.Errorf("phases.analyze.refiner.agent references disabled agent %q", cfg.Phases.Analyze.Refiner.Agent)
		}
		if agent.GetModelForPhase("optimize") == "" {
			return fmt.Errorf("phases.analyze.refiner.agent %q has no model configured for optimize phase", cfg.Phases.Analyze.Refiner.Agent)
		}
	}

	// Validate phases.analyze.synthesizer
	if cfg.Phases.Analyze.Synthesizer.Agent == "" {
		return fmt.Errorf("phases.analyze.synthesizer.agent is required")
	}
	synthAgent := cfg.Agents.GetAgentConfig(cfg.Phases.Analyze.Synthesizer.Agent)
	if synthAgent == nil {
		return fmt.Errorf("phases.analyze.synthesizer.agent references unknown agent %q", cfg.Phases.Analyze.Synthesizer.Agent)
	}
	if !synthAgent.Enabled {
		return fmt.Errorf("phases.analyze.synthesizer.agent references disabled agent %q", cfg.Phases.Analyze.Synthesizer.Agent)
	}
	if synthAgent.GetModelForPhase("analyze") == "" {
		return fmt.Errorf("phases.analyze.synthesizer.agent %q has no model configured for analyze phase", cfg.Phases.Analyze.Synthesizer.Agent)
	}

	// Validate phases.analyze.moderator if enabled
	if cfg.Phases.Analyze.Moderator.Enabled {
		if cfg.Phases.Analyze.Moderator.Agent == "" {
			return fmt.Errorf("phases.analyze.moderator.agent is required when moderator is enabled")
		}
		agent := cfg.Agents.GetAgentConfig(cfg.Phases.Analyze.Moderator.Agent)
		if agent == nil {
			return fmt.Errorf("phases.analyze.moderator.agent references unknown agent %q", cfg.Phases.Analyze.Moderator.Agent)
		}
		if !agent.Enabled {
			return fmt.Errorf("phases.analyze.moderator.agent references disabled agent %q", cfg.Phases.Analyze.Moderator.Agent)
		}
		if agent.GetModelForPhase("analyze") == "" {
			return fmt.Errorf("phases.analyze.moderator.agent %q has no model configured for analyze phase", cfg.Phases.Analyze.Moderator.Agent)
		}
	}

	// Validate mutual exclusivity: single_agent and moderator cannot both be enabled
	if cfg.Phases.Analyze.SingleAgent.Enabled && cfg.Phases.Analyze.Moderator.Enabled {
		return fmt.Errorf("phases.analyze.single_agent.enabled and phases.analyze.moderator.enabled cannot both be true; " +
			"single-agent mode bypasses consensus, disable moderator when using single_agent")
	}

	// Validate phases.analyze.single_agent if enabled
	if cfg.Phases.Analyze.SingleAgent.Enabled {
		if cfg.Phases.Analyze.SingleAgent.Agent == "" {
			return fmt.Errorf("phases.analyze.single_agent.agent is required when single_agent is enabled")
		}
		agent := cfg.Agents.GetAgentConfig(cfg.Phases.Analyze.SingleAgent.Agent)
		if agent == nil {
			return fmt.Errorf("phases.analyze.single_agent.agent references unknown agent %q", cfg.Phases.Analyze.SingleAgent.Agent)
		}
		if !agent.Enabled {
			return fmt.Errorf("phases.analyze.single_agent.agent references disabled agent %q", cfg.Phases.Analyze.SingleAgent.Agent)
		}
		if agent.GetModelForPhase("analyze") == "" {
			return fmt.Errorf("phases.analyze.single_agent.agent %q has no model configured for analyze phase", cfg.Phases.Analyze.SingleAgent.Agent)
		}
	}

	// Validate phases.plan.synthesizer if enabled
	if cfg.Phases.Plan.Synthesizer.Enabled {
		if cfg.Phases.Plan.Synthesizer.Agent == "" {
			return fmt.Errorf("phases.plan.synthesizer.agent is required when synthesizer is enabled")
		}
		agent := cfg.Agents.GetAgentConfig(cfg.Phases.Plan.Synthesizer.Agent)
		if agent == nil {
			return fmt.Errorf("phases.plan.synthesizer.agent references unknown agent %q", cfg.Phases.Plan.Synthesizer.Agent)
		}
		if !agent.Enabled {
			return fmt.Errorf("phases.plan.synthesizer.agent references disabled agent %q", cfg.Phases.Plan.Synthesizer.Agent)
		}
		if agent.GetModelForPhase("plan") == "" {
			return fmt.Errorf("phases.plan.synthesizer.agent %q has no model configured for plan phase", cfg.Phases.Plan.Synthesizer.Agent)
		}
	}

	// Validate issues configuration
	if err := cfg.Issues.Validate(); err != nil {
		return err
	}

	return nil
}
