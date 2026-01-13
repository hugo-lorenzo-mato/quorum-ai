package config

// Config holds all application configuration.
type Config struct {
	Log       LogConfig       `mapstructure:"log"`
	Trace     TraceConfig     `mapstructure:"trace"`
	Workflow  WorkflowConfig  `mapstructure:"workflow"`
	Agents    AgentsConfig    `mapstructure:"agents"`
	State     StateConfig     `mapstructure:"state"`
	Git       GitConfig       `mapstructure:"git"`
	GitHub    GitHubConfig    `mapstructure:"github"`
	Consensus ConsensusConfig `mapstructure:"consensus"`
	Costs     CostsConfig     `mapstructure:"costs"`
}

// LogConfig configures logging behavior.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	File   string `mapstructure:"file"`
}

// TraceConfig configures trace mode output.
type TraceConfig struct {
	Mode            string   `mapstructure:"mode"`
	Dir             string   `mapstructure:"dir"`
	SchemaVersion   int      `mapstructure:"schema_version"`
	Redact          bool     `mapstructure:"redact"`
	RedactPatterns  []string `mapstructure:"redact_patterns"`
	RedactAllowlist []string `mapstructure:"redact_allowlist"`
	MaxBytes        int64    `mapstructure:"max_bytes"`
	TotalMaxBytes   int64    `mapstructure:"total_max_bytes"`
	MaxFiles        int      `mapstructure:"max_files"`
	IncludePhases   []string `mapstructure:"include_phases"`
}

// WorkflowConfig configures workflow execution.
type WorkflowConfig struct {
	Timeout    string   `mapstructure:"timeout"`
	MaxRetries int      `mapstructure:"max_retries"`
	DryRun     bool     `mapstructure:"dry_run"`
	Sandbox    bool     `mapstructure:"sandbox"`
	DenyTools  []string `mapstructure:"deny_tools"`
}

// AgentsConfig configures available AI agents.
type AgentsConfig struct {
	Default string      `mapstructure:"default"`
	Claude  AgentConfig `mapstructure:"claude"`
	Gemini  AgentConfig `mapstructure:"gemini"`
	Codex   AgentConfig `mapstructure:"codex"`
	Copilot AgentConfig `mapstructure:"copilot"`
	Aider   AgentConfig `mapstructure:"aider"`
}

// AgentConfig configures a single AI agent.
type AgentConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Path        string            `mapstructure:"path"`
	Model       string            `mapstructure:"model"`
	PhaseModels map[string]string `mapstructure:"phase_models"`
	MaxTokens   int               `mapstructure:"max_tokens"`
	Temperature float64           `mapstructure:"temperature"`
}

// StateConfig configures state persistence.
type StateConfig struct {
	Path       string `mapstructure:"path"`
	BackupPath string `mapstructure:"backup_path"`
	LockTTL    string `mapstructure:"lock_ttl"`
}

// GitConfig configures git operations.
type GitConfig struct {
	WorktreeDir string `mapstructure:"worktree_dir"`
	AutoClean   bool   `mapstructure:"auto_clean"`
}

// GitHubConfig configures GitHub integration.
type GitHubConfig struct {
	Token  string `mapstructure:"token"`
	Remote string `mapstructure:"remote"`
}

// ConsensusConfig configures consensus calculation.
type ConsensusConfig struct {
	Threshold float64         `mapstructure:"threshold"`
	Weights   ConsensusWeight `mapstructure:"weights"`
}

// ConsensusWeight configures component weights for consensus.
type ConsensusWeight struct {
	Claims          float64 `mapstructure:"claims"`
	Risks           float64 `mapstructure:"risks"`
	Recommendations float64 `mapstructure:"recommendations"`
}

// CostsConfig configures cost limits and alerts.
type CostsConfig struct {
	MaxPerWorkflow float64 `mapstructure:"max_per_workflow"`
	MaxPerTask     float64 `mapstructure:"max_per_task"`
	AlertThreshold float64 `mapstructure:"alert_threshold"`
}
