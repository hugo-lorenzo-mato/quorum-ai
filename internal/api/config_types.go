package api

// ============================================================================
// RESPONSE DTOs (for GET /api/v1/config)
// ============================================================================

// ConfigResponseWithMeta wraps config response with metadata.
type ConfigResponseWithMeta struct {
	Config FullConfigResponse `json:"config"`
	Meta   ConfigMeta         `json:"_meta"`
}

// ConfigMeta contains configuration metadata for sync/conflict detection.
type ConfigMeta struct {
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified,omitempty"`
	Source       string `json:"source"` // "file", "default"
	// Scope indicates which configuration scope was loaded.
	// Values: "global" | "project"
	Scope string `json:"scope,omitempty"`
	// ProjectConfigMode indicates whether the current project inherits the global config
	// or uses a project-specific config file.
	// Values: "inherit_global" | "custom"
	ProjectConfigMode string `json:"project_config_mode,omitempty"`

	// Runtime apply info (best-effort): whether this config was applied to the server runtime.
	RuntimeApplyStatus string `json:"runtime_apply_status,omitempty"` // "applied" | "failed"
	RuntimeAppliedAt   string `json:"runtime_applied_at,omitempty"`   // RFC3339
	RuntimeApplyError  string `json:"runtime_apply_error,omitempty"`
}

// FullConfigResponse represents the complete configuration response.
type FullConfigResponse struct {
	Log         LogConfigResponse         `json:"log"`
	Trace       TraceConfigResponse       `json:"trace"`
	Workflow    WorkflowConfigResponse    `json:"workflow"`
	Phases      PhasesConfigResponse      `json:"phases"`
	Agents      AgentsConfigResponse      `json:"agents"`
	State       StateConfigResponse       `json:"state"`
	Git         GitConfigResponse         `json:"git"`
	GitHub      GitHubConfigResponse      `json:"github"`
	Chat        ChatConfigResponse        `json:"chat"`
	Report      ReportConfigResponse      `json:"report"`
	Diagnostics DiagnosticsConfigResponse `json:"diagnostics"`
	Issues      IssuesConfigResponse      `json:"issues"`
}

// LogConfigResponse represents logging configuration.
type LogConfigResponse struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// TraceConfigResponse represents trace configuration.
type TraceConfigResponse struct {
	Mode            string   `json:"mode"`
	Dir             string   `json:"dir"`
	SchemaVersion   int      `json:"schema_version"`
	Redact          bool     `json:"redact"`
	RedactPatterns  []string `json:"redact_patterns"`
	RedactAllowlist []string `json:"redact_allowlist"`
	MaxBytes        int64    `json:"max_bytes"`
	TotalMaxBytes   int64    `json:"total_max_bytes"`
	MaxFiles        int      `json:"max_files"`
	IncludePhases   []string `json:"include_phases"`
}

// WorkflowConfigResponse represents workflow configuration.
type WorkflowConfigResponse struct {
	Timeout    string                  `json:"timeout"`
	MaxRetries int                     `json:"max_retries"`
	DryRun     bool                    `json:"dry_run"`
	Sandbox    bool                    `json:"sandbox"`
	DenyTools  []string                `json:"deny_tools"`
	Heartbeat  HeartbeatConfigResponse `json:"heartbeat"`
}

// HeartbeatConfigResponse represents heartbeat configuration for zombie workflow detection.
type HeartbeatConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	Interval       string `json:"interval"`
	StaleThreshold string `json:"stale_threshold"`
	CheckInterval  string `json:"check_interval"`
	AutoResume     bool   `json:"auto_resume"`
	MaxResumes     int    `json:"max_resumes"`
}

// PhasesConfigResponse represents all phase configurations.
type PhasesConfigResponse struct {
	Analyze AnalyzePhaseConfigResponse `json:"analyze"`
	Plan    PlanPhaseConfigResponse    `json:"plan"`
	Execute ExecutePhaseConfigResponse `json:"execute"`
}

// AnalyzePhaseConfigResponse represents analyze phase configuration.
type AnalyzePhaseConfigResponse struct {
	Timeout     string                    `json:"timeout"`
	Refiner     RefinerConfigResponse     `json:"refiner"`
	Moderator   ModeratorConfigResponse   `json:"moderator"`
	Synthesizer SynthesizerConfigResponse `json:"synthesizer"`
	SingleAgent SingleAgentConfigResponse `json:"single_agent"`
}

// RefinerConfigResponse represents refiner configuration.
type RefinerConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Agent   string `json:"agent"`
}

// ModeratorConfigResponse represents moderator configuration.
type ModeratorConfigResponse struct {
	Enabled             bool    `json:"enabled"`
	Agent               string  `json:"agent"`
	Threshold           float64 `json:"threshold"`
	MinRounds           int     `json:"min_rounds"`
	MaxRounds           int     `json:"max_rounds"`
	WarningThreshold    float64 `json:"warning_threshold"`
	StagnationThreshold float64 `json:"stagnation_threshold"`
}

// SynthesizerConfigResponse represents synthesizer configuration.
type SynthesizerConfigResponse struct {
	Agent string `json:"agent"`
}

// SingleAgentConfigResponse represents single-agent mode configuration.
type SingleAgentConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Agent   string `json:"agent"`
	Model   string `json:"model"`
}

// PlanPhaseConfigResponse represents plan phase configuration.
type PlanPhaseConfigResponse struct {
	Timeout     string                        `json:"timeout"`
	Synthesizer PlanSynthesizerConfigResponse `json:"synthesizer"`
}

// PlanSynthesizerConfigResponse represents plan synthesizer configuration.
type PlanSynthesizerConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Agent   string `json:"agent"`
}

// ExecutePhaseConfigResponse represents execute phase configuration.
type ExecutePhaseConfigResponse struct {
	Timeout string `json:"timeout"`
}

// AgentsConfigResponse represents all agent configurations.
type AgentsConfigResponse struct {
	Default  string                  `json:"default"`
	Claude   FullAgentConfigResponse `json:"claude"`
	Gemini   FullAgentConfigResponse `json:"gemini"`
	Codex    FullAgentConfigResponse `json:"codex"`
	Copilot  FullAgentConfigResponse `json:"copilot"`
	OpenCode FullAgentConfigResponse `json:"opencode"`
}

// FullAgentConfigResponse represents complete agent configuration.
type FullAgentConfigResponse struct {
	Enabled                   bool              `json:"enabled"`
	Path                      string            `json:"path"`
	Model                     string            `json:"model"`
	PhaseModels               map[string]string `json:"phase_models"`
	Phases                    map[string]bool   `json:"phases"`
	ReasoningEffort           string            `json:"reasoning_effort"`
	ReasoningEffortPhases     map[string]string `json:"reasoning_effort_phases"`
	TokenDiscrepancyThreshold float64           `json:"token_discrepancy_threshold"`
}

// StateConfigResponse represents state persistence configuration.
type StateConfigResponse struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path"`
	LockTTL    string `json:"lock_ttl"`
}

// GitConfigResponse represents git configuration with semantic grouping.
type GitConfigResponse struct {
	Worktree     WorktreeConfigResponse        `json:"worktree"`
	Task         GitTaskConfigResponse         `json:"task"`
	Finalization GitFinalizationConfigResponse `json:"finalization"`
}

// WorktreeConfigResponse represents worktree management configuration.
type WorktreeConfigResponse struct {
	Dir       string `json:"dir"`
	Mode      string `json:"mode"`
	AutoClean bool   `json:"auto_clean"`
}

// GitTaskConfigResponse represents per-task progress configuration.
type GitTaskConfigResponse struct {
	AutoCommit bool `json:"auto_commit"`
}

// GitFinalizationConfigResponse represents workflow finalization configuration.
type GitFinalizationConfigResponse struct {
	AutoPush      bool   `json:"auto_push"`
	AutoPR        bool   `json:"auto_pr"`
	AutoMerge     bool   `json:"auto_merge"`
	PRBaseBranch  string `json:"pr_base_branch"`
	MergeStrategy string `json:"merge_strategy"`
}

// GitHubConfigResponse represents GitHub configuration.
type GitHubConfigResponse struct {
	Remote string `json:"remote"`
}

// ChatConfigResponse represents chat configuration.
type ChatConfigResponse struct {
	Timeout          string `json:"timeout"`
	ProgressInterval string `json:"progress_interval"`
	Editor           string `json:"editor"`
}

// ReportConfigResponse represents report configuration.
type ReportConfigResponse struct {
	Enabled    bool   `json:"enabled"`
	BaseDir    string `json:"base_dir"`
	UseUTC     bool   `json:"use_utc"`
	IncludeRaw bool   `json:"include_raw"`
}

// DiagnosticsConfigResponse represents diagnostics configuration.
type DiagnosticsConfigResponse struct {
	Enabled            bool                             `json:"enabled"`
	ResourceMonitoring ResourceMonitoringConfigResponse `json:"resource_monitoring"`
	CrashDump          CrashDumpConfigResponse          `json:"crash_dump"`
	PreflightChecks    PreflightConfigResponse          `json:"preflight_checks"`
}

// ResourceMonitoringConfigResponse represents resource monitoring configuration.
type ResourceMonitoringConfigResponse struct {
	Interval           string `json:"interval"`
	FDThresholdPercent int    `json:"fd_threshold_percent"`
	GoroutineThreshold int    `json:"goroutine_threshold"`
	MemoryThresholdMB  int    `json:"memory_threshold_mb"`
	HistorySize        int    `json:"history_size"`
}

// CrashDumpConfigResponse represents crash dump configuration.
type CrashDumpConfigResponse struct {
	Dir          string `json:"dir"`
	MaxFiles     int    `json:"max_files"`
	IncludeStack bool   `json:"include_stack"`
	IncludeEnv   bool   `json:"include_env"`
}

// PreflightConfigResponse represents preflight checks configuration.
type PreflightConfigResponse struct {
	Enabled          bool `json:"enabled"`
	MinFreeFDPercent int  `json:"min_free_fd_percent"`
	MinFreeMemoryMB  int  `json:"min_free_memory_mb"`
}

// IssuesConfigResponse represents issues configuration.
type IssuesConfigResponse struct {
	Enabled        bool                         `json:"enabled"`
	Provider       string                       `json:"provider"`
	AutoGenerate   bool                         `json:"auto_generate"`
	Timeout        string                       `json:"timeout"`
	Mode           string                       `json:"mode"`
	DraftDirectory string                       `json:"draft_directory"`
	Repository     string                       `json:"repository"`
	ParentTemplate string                       `json:"parent_template"`
	Template       IssueTemplateConfigResponse  `json:"template"`
	Labels         []string                     `json:"default_labels"`
	Assignees      []string                     `json:"default_assignees"`
	GitLab         GitLabIssueConfigResponse    `json:"gitlab"`
	Generator      IssueGeneratorConfigResponse `json:"generator"`
}

// IssueTemplateConfigResponse represents issue template configuration.
type IssueTemplateConfigResponse struct {
	Language           string `json:"language"`
	Tone               string `json:"tone"`
	IncludeDiagrams    bool   `json:"include_diagrams"`
	TitleFormat        string `json:"title_format"`
	BodyTemplateFile   string `json:"body_template_file"`
	Convention         string `json:"convention"`
	CustomInstructions string `json:"custom_instructions"`
}

// GitLabIssueConfigResponse represents GitLab-specific issue configuration.
type GitLabIssueConfigResponse struct {
	UseEpics  bool   `json:"use_epics"`
	ProjectID string `json:"project_id"`
}

// IssueGeneratorConfigResponse represents LLM-based issue generation configuration.
type IssueGeneratorConfigResponse struct {
	Enabled           bool   `json:"enabled"`
	Agent             string `json:"agent"`
	Model             string `json:"model"`
	Summarize         bool   `json:"summarize"`
	MaxBodyLength     int    `json:"max_body_length"`
	ReasoningEffort   string `json:"reasoning_effort"`
	Instructions      string `json:"instructions"`
	TitleInstructions string `json:"title_instructions"`
}

// ============================================================================
// UPDATE DTOs (for PATCH /api/v1/config)
// All fields are pointers to support partial updates (nil = unchanged)
// ============================================================================

// FullConfigUpdate represents a complete configuration update request.
type FullConfigUpdate struct {
	Log         *LogConfigUpdate         `json:"log,omitempty"`
	Trace       *TraceConfigUpdate       `json:"trace,omitempty"`
	Workflow    *WorkflowConfigUpdate    `json:"workflow,omitempty"`
	Phases      *PhasesConfigUpdate      `json:"phases,omitempty"`
	Agents      *AgentsConfigUpdate      `json:"agents,omitempty"`
	State       *StateConfigUpdate       `json:"state,omitempty"`
	Git         *GitConfigUpdate         `json:"git,omitempty"`
	GitHub      *GitHubConfigUpdate      `json:"github,omitempty"`
	Chat        *ChatConfigUpdate        `json:"chat,omitempty"`
	Report      *ReportConfigUpdate      `json:"report,omitempty"`
	Diagnostics *DiagnosticsConfigUpdate `json:"diagnostics,omitempty"`
	Issues      *IssuesConfigUpdate      `json:"issues,omitempty"`
}

// LogConfigUpdate represents log configuration update.
type LogConfigUpdate struct {
	Level  *string `json:"level,omitempty"`
	Format *string `json:"format,omitempty"`
}

// TraceConfigUpdate represents trace configuration update.
type TraceConfigUpdate struct {
	Mode            *string   `json:"mode,omitempty"`
	Dir             *string   `json:"dir,omitempty"`
	SchemaVersion   *int      `json:"schema_version,omitempty"`
	Redact          *bool     `json:"redact,omitempty"`
	RedactPatterns  *[]string `json:"redact_patterns,omitempty"`
	RedactAllowlist *[]string `json:"redact_allowlist,omitempty"`
	MaxBytes        *int64    `json:"max_bytes,omitempty"`
	TotalMaxBytes   *int64    `json:"total_max_bytes,omitempty"`
	MaxFiles        *int      `json:"max_files,omitempty"`
	IncludePhases   *[]string `json:"include_phases,omitempty"`
}

// WorkflowConfigUpdate represents workflow configuration update.
type WorkflowConfigUpdate struct {
	Timeout    *string                `json:"timeout,omitempty"`
	MaxRetries *int                   `json:"max_retries,omitempty"`
	DryRun     *bool                  `json:"dry_run,omitempty"`
	Sandbox    *bool                  `json:"sandbox,omitempty"`
	DenyTools  *[]string              `json:"deny_tools,omitempty"`
	Heartbeat  *HeartbeatConfigUpdate `json:"heartbeat,omitempty"`
}

// HeartbeatConfigUpdate represents heartbeat configuration update.
type HeartbeatConfigUpdate struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	Interval       *string `json:"interval,omitempty"`
	StaleThreshold *string `json:"stale_threshold,omitempty"`
	CheckInterval  *string `json:"check_interval,omitempty"`
	AutoResume     *bool   `json:"auto_resume,omitempty"`
	MaxResumes     *int    `json:"max_resumes,omitempty"`
}

// PhasesConfigUpdate represents phases configuration update.
type PhasesConfigUpdate struct {
	Analyze *AnalyzePhaseConfigUpdate `json:"analyze,omitempty"`
	Plan    *PlanPhaseConfigUpdate    `json:"plan,omitempty"`
	Execute *ExecutePhaseConfigUpdate `json:"execute,omitempty"`
}

// AnalyzePhaseConfigUpdate represents analyze phase update.
type AnalyzePhaseConfigUpdate struct {
	Timeout     *string                  `json:"timeout,omitempty"`
	Refiner     *RefinerConfigUpdate     `json:"refiner,omitempty"`
	Moderator   *ModeratorConfigUpdate   `json:"moderator,omitempty"`
	Synthesizer *SynthesizerConfigUpdate `json:"synthesizer,omitempty"`
	SingleAgent *SingleAgentConfigUpdate `json:"single_agent,omitempty"`
}

// RefinerConfigUpdate represents refiner update.
type RefinerConfigUpdate struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Agent   *string `json:"agent,omitempty"`
}

// ModeratorConfigUpdate represents moderator update.
type ModeratorConfigUpdate struct {
	Enabled             *bool    `json:"enabled,omitempty"`
	Agent               *string  `json:"agent,omitempty"`
	Threshold           *float64 `json:"threshold,omitempty"`
	MinRounds           *int     `json:"min_rounds,omitempty"`
	MaxRounds           *int     `json:"max_rounds,omitempty"`
	WarningThreshold    *float64 `json:"warning_threshold,omitempty"`
	StagnationThreshold *float64 `json:"stagnation_threshold,omitempty"`
}

// SynthesizerConfigUpdate represents synthesizer update.
type SynthesizerConfigUpdate struct {
	Agent *string `json:"agent,omitempty"`
}

// SingleAgentConfigUpdate represents single-agent mode update.
type SingleAgentConfigUpdate struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Agent   *string `json:"agent,omitempty"`
	Model   *string `json:"model,omitempty"`
}

// PlanPhaseConfigUpdate represents plan phase update.
type PlanPhaseConfigUpdate struct {
	Timeout     *string                      `json:"timeout,omitempty"`
	Synthesizer *PlanSynthesizerConfigUpdate `json:"synthesizer,omitempty"`
}

// PlanSynthesizerConfigUpdate represents plan synthesizer update.
type PlanSynthesizerConfigUpdate struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Agent   *string `json:"agent,omitempty"`
}

// ExecutePhaseConfigUpdate represents execute phase update.
type ExecutePhaseConfigUpdate struct {
	Timeout *string `json:"timeout,omitempty"`
}

// AgentsConfigUpdate represents agents configuration update.
type AgentsConfigUpdate struct {
	Default  *string                `json:"default,omitempty"`
	Claude   *FullAgentConfigUpdate `json:"claude,omitempty"`
	Gemini   *FullAgentConfigUpdate `json:"gemini,omitempty"`
	Codex    *FullAgentConfigUpdate `json:"codex,omitempty"`
	Copilot  *FullAgentConfigUpdate `json:"copilot,omitempty"`
	OpenCode *FullAgentConfigUpdate `json:"opencode,omitempty"`
}

// FullAgentConfigUpdate represents complete agent update.
type FullAgentConfigUpdate struct {
	Enabled                   *bool              `json:"enabled,omitempty"`
	Path                      *string            `json:"path,omitempty"`
	Model                     *string            `json:"model,omitempty"`
	PhaseModels               *map[string]string `json:"phase_models,omitempty"`
	Phases                    *map[string]bool   `json:"phases,omitempty"`
	ReasoningEffort           *string            `json:"reasoning_effort,omitempty"`
	ReasoningEffortPhases     *map[string]string `json:"reasoning_effort_phases,omitempty"`
	TokenDiscrepancyThreshold *float64           `json:"token_discrepancy_threshold,omitempty"`
}

// StateConfigUpdate represents state configuration update.
type StateConfigUpdate struct {
	Path       *string `json:"path,omitempty"`
	BackupPath *string `json:"backup_path,omitempty"`
	LockTTL    *string `json:"lock_ttl,omitempty"`
}

// GitConfigUpdate represents git configuration update with semantic grouping.
type GitConfigUpdate struct {
	Worktree     *WorktreeConfigUpdate        `json:"worktree,omitempty"`
	Task         *GitTaskConfigUpdate         `json:"task,omitempty"`
	Finalization *GitFinalizationConfigUpdate `json:"finalization,omitempty"`
}

// WorktreeConfigUpdate represents worktree configuration update.
type WorktreeConfigUpdate struct {
	Dir       *string `json:"dir,omitempty"`
	Mode      *string `json:"mode,omitempty"`
	AutoClean *bool   `json:"auto_clean,omitempty"`
}

// GitTaskConfigUpdate represents task configuration update.
type GitTaskConfigUpdate struct {
	AutoCommit *bool `json:"auto_commit,omitempty"`
}

// GitFinalizationConfigUpdate represents finalization configuration update.
type GitFinalizationConfigUpdate struct {
	AutoPush      *bool   `json:"auto_push,omitempty"`
	AutoPR        *bool   `json:"auto_pr,omitempty"`
	AutoMerge     *bool   `json:"auto_merge,omitempty"`
	PRBaseBranch  *string `json:"pr_base_branch,omitempty"`
	MergeStrategy *string `json:"merge_strategy,omitempty"`
}

// GitHubConfigUpdate represents GitHub configuration update.
type GitHubConfigUpdate struct {
	Remote *string `json:"remote,omitempty"`
}

// ChatConfigUpdate represents chat configuration update.
type ChatConfigUpdate struct {
	Timeout          *string `json:"timeout,omitempty"`
	ProgressInterval *string `json:"progress_interval,omitempty"`
	Editor           *string `json:"editor,omitempty"`
}

// ReportConfigUpdate represents report configuration update.
type ReportConfigUpdate struct {
	Enabled    *bool   `json:"enabled,omitempty"`
	BaseDir    *string `json:"base_dir,omitempty"`
	UseUTC     *bool   `json:"use_utc,omitempty"`
	IncludeRaw *bool   `json:"include_raw,omitempty"`
}

// DiagnosticsConfigUpdate represents diagnostics configuration update.
type DiagnosticsConfigUpdate struct {
	Enabled            *bool                           `json:"enabled,omitempty"`
	ResourceMonitoring *ResourceMonitoringConfigUpdate `json:"resource_monitoring,omitempty"`
	CrashDump          *CrashDumpConfigUpdate          `json:"crash_dump,omitempty"`
	PreflightChecks    *PreflightConfigUpdate          `json:"preflight_checks,omitempty"`
}

// ResourceMonitoringConfigUpdate represents resource monitoring update.
type ResourceMonitoringConfigUpdate struct {
	Interval           *string `json:"interval,omitempty"`
	FDThresholdPercent *int    `json:"fd_threshold_percent,omitempty"`
	GoroutineThreshold *int    `json:"goroutine_threshold,omitempty"`
	MemoryThresholdMB  *int    `json:"memory_threshold_mb,omitempty"`
	HistorySize        *int    `json:"history_size,omitempty"`
}

// CrashDumpConfigUpdate represents crash dump update.
type CrashDumpConfigUpdate struct {
	Dir          *string `json:"dir,omitempty"`
	MaxFiles     *int    `json:"max_files,omitempty"`
	IncludeStack *bool   `json:"include_stack,omitempty"`
	IncludeEnv   *bool   `json:"include_env,omitempty"`
}

// PreflightConfigUpdate represents preflight checks update.
type PreflightConfigUpdate struct {
	Enabled          *bool `json:"enabled,omitempty"`
	MinFreeFDPercent *int  `json:"min_free_fd_percent,omitempty"`
	MinFreeMemoryMB  *int  `json:"min_free_memory_mb,omitempty"`
}

// IssuesConfigUpdate represents issues configuration update.
type IssuesConfigUpdate struct {
	Enabled        *bool                       `json:"enabled,omitempty"`
	Provider       *string                     `json:"provider,omitempty"`
	AutoGenerate   *bool                       `json:"auto_generate,omitempty"`
	Timeout        *string                     `json:"timeout,omitempty"`
	Mode           *string                     `json:"mode,omitempty"`
	DraftDirectory *string                     `json:"draft_directory,omitempty"`
	Repository     *string                     `json:"repository,omitempty"`
	ParentTemplate *string                     `json:"parent_template,omitempty"`
	Template       *IssueTemplateConfigUpdate  `json:"template,omitempty"`
	Labels         *[]string                   `json:"default_labels,omitempty"`
	Assignees      *[]string                   `json:"default_assignees,omitempty"`
	GitLab         *GitLabIssueConfigUpdate    `json:"gitlab,omitempty"`
	Generator      *IssueGeneratorConfigUpdate `json:"generator,omitempty"`
}

// IssueTemplateConfigUpdate represents issue template update.
type IssueTemplateConfigUpdate struct {
	Language           *string `json:"language,omitempty"`
	Tone               *string `json:"tone,omitempty"`
	IncludeDiagrams    *bool   `json:"include_diagrams,omitempty"`
	TitleFormat        *string `json:"title_format,omitempty"`
	BodyTemplateFile   *string `json:"body_template_file,omitempty"`
	Convention         *string `json:"convention,omitempty"`
	CustomInstructions *string `json:"custom_instructions,omitempty"`
}

// GitLabIssueConfigUpdate represents GitLab-specific issue update.
type GitLabIssueConfigUpdate struct {
	UseEpics  *bool   `json:"use_epics,omitempty"`
	ProjectID *string `json:"project_id,omitempty"`
}

// IssueGeneratorConfigUpdate represents issue generator update.
type IssueGeneratorConfigUpdate struct {
	Enabled           *bool   `json:"enabled,omitempty"`
	Agent             *string `json:"agent,omitempty"`
	Model             *string `json:"model,omitempty"`
	Summarize         *bool   `json:"summarize,omitempty"`
	MaxBodyLength     *int    `json:"max_body_length,omitempty"`
	ReasoningEffort   *string `json:"reasoning_effort,omitempty"`
	Instructions      *string `json:"instructions,omitempty"`
	TitleInstructions *string `json:"title_instructions,omitempty"`
}

// ============================================================================
// METADATA DTOs (for GET /api/v1/config/schema)
// ============================================================================

// ConfigFieldMeta provides metadata about a configuration field.
type ConfigFieldMeta struct {
	Type        string      `json:"type"`          // string, int, bool, float64, []string, map[string]string, map[string]bool
	Default     interface{} `json:"default"`       // Default value from loader
	Required    bool        `json:"required"`      // Is this field required?
	ValidValues []string    `json:"valid_values"`  // For enums, list of valid values
	Min         *float64    `json:"min,omitempty"` // For numeric types, minimum value
	Max         *float64    `json:"max,omitempty"` // For numeric types, maximum value
	Description string      `json:"description"`   // Human-readable description
	Tooltip     string      `json:"tooltip"`       // Tooltip text for UI
	DependsOn   []string    `json:"depends_on"`    // Fields this depends on
	Category    string      `json:"category"`      // UI grouping category
}
