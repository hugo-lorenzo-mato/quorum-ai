package api

import (
	"net/http"
)

// ConfigSchema represents the configuration schema for UI generation.
type ConfigSchema struct {
	Sections []SchemaSection `json:"sections"`
}

// SchemaSection represents a configuration section.
type SchemaSection struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Tab         string        `json:"tab"`
	Fields      []SchemaField `json:"fields"`
}

// SchemaField represents a single configuration field.
type SchemaField struct {
	Path        string           `json:"path"`
	Type        string           `json:"type"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Tooltip     string           `json:"tooltip"`
	Default     interface{}      `json:"default"`
	Required    bool             `json:"required"`
	ValidValues []string         `json:"valid_values,omitempty"`
	Min         *float64         `json:"min,omitempty"`
	Max         *float64         `json:"max,omitempty"`
	DependsOn   *FieldDependency `json:"depends_on,omitempty"`
	DangerLevel string           `json:"danger_level,omitempty"`
	Category    string           `json:"category"`
}

// FieldDependency defines when a field is enabled.
type FieldDependency struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

// handleGetConfigSchema returns the configuration schema for UI generation.
func (s *Server) handleGetConfigSchema(w http.ResponseWriter, _ *http.Request) {
	schema := buildConfigSchema()
	respondJSON(w, http.StatusOK, schema)
}

func buildConfigSchema() ConfigSchema {
	return ConfigSchema{
		Sections: []SchemaSection{
			buildLogSection(),
			buildChatSection(),
			buildReportSection(),
			buildWorkflowSection(),
			buildStateSection(),
			buildAgentsSection(),
			buildPhasesAnalyzeSection(),
			buildPhasesPlanSection(),
			buildPhasesExecuteSection(),
			buildGitSection(),
			buildGitHubSection(),
			buildTraceSection(),
			buildDiagnosticsSection(),
		},
	}
}

func buildLogSection() SchemaSection {
	return SchemaSection{
		ID:          "log",
		Title:       "Logging",
		Description: "Configure logging behavior",
		Tab:         "general",
		Fields: []SchemaField{
			{
				Path:        "log.level",
				Type:        "string",
				Title:       "Log Level",
				Description: "Controls logging verbosity",
				Tooltip:     "Use 'debug' for troubleshooting, 'info' for normal operation, 'warn' for warnings only, or 'error' for errors only.",
				Default:     "info",
				Required:    true,
				ValidValues: []string{"debug", "info", "warn", "error"},
				Category:    "basic",
			},
			{
				Path:        "log.format",
				Type:        "string",
				Title:       "Log Format",
				Description: "Output format for logs",
				Tooltip:     "'auto' detects terminal capability, 'text' for human-readable, 'json' for machine-parseable logs.",
				Default:     "auto",
				Required:    true,
				ValidValues: []string{"auto", "text", "json"},
				Category:    "basic",
			},
		},
	}
}

func buildChatSection() SchemaSection {
	return SchemaSection{
		ID:          "chat",
		Title:       "Chat Settings",
		Description: "Configure chat behavior in TUI",
		Tab:         "general",
		Fields: []SchemaField{
			{
				Path:        "chat.timeout",
				Type:        "duration",
				Title:       "Chat Timeout",
				Description: "Timeout for chat responses",
				Tooltip:     "Example: '3m' for 3 minutes, '5m' for 5 minutes.",
				Default:     "3m",
				Category:    "basic",
			},
			{
				Path:        "chat.progress_interval",
				Type:        "duration",
				Title:       "Progress Interval",
				Description: "Interval for progress logs",
				Tooltip:     "Example: '15s' for 15 seconds.",
				Default:     "15s",
				Category:    "advanced",
			},
			{
				Path:        "chat.editor",
				Type:        "string",
				Title:       "Editor Command",
				Description: "Editor for file edits",
				Tooltip:     "Example: 'code', 'nvim', 'vim'.",
				Default:     "vim",
				Category:    "basic",
			},
		},
	}
}

func buildReportSection() SchemaSection {
	return SchemaSection{
		ID:          "report",
		Title:       "Report Generation",
		Description: "Configure markdown report generation",
		Tab:         "general",
		Fields: []SchemaField{
			{
				Path:        "report.enabled",
				Type:        "bool",
				Title:       "Enable Reports",
				Description: "Enable markdown report generation for workflow runs",
				Tooltip:     "When enabled, generates a markdown report after each workflow run.",
				Default:     true,
				Category:    "basic",
			},
			{
				Path:        "report.base_dir",
				Type:        "string",
				Title:       "Report Directory",
				Description: "Base directory for workflow reports",
				Tooltip:     "Default: '.quorum/runs'. Reports are saved as timestamped markdown files.",
				Default:     ".quorum/runs",
				Category:    "advanced",
			},
			{
				Path:        "report.use_utc",
				Type:        "bool",
				Title:       "Use UTC Timestamps",
				Description: "Use UTC timestamps instead of local time",
				Tooltip:     "When enabled, all timestamps in reports use UTC timezone.",
				Default:     true,
				Category:    "advanced",
			},
			{
				Path:        "report.include_raw",
				Type:        "bool",
				Title:       "Include Raw Output",
				Description: "Include raw LLM outputs in reports",
				Tooltip:     "When enabled, includes the raw LLM responses. Increases report size.",
				Default:     true,
				Category:    "advanced",
			},
		},
	}
}

func buildWorkflowSection() SchemaSection {
	min0 := float64(0)
	max10 := float64(10)

	return SchemaSection{
		ID:          "workflow",
		Title:       "Workflow Execution",
		Description: "Configure how workflows are executed",
		Tab:         "workflow",
		Fields: []SchemaField{
			{
				Path:        "workflow.timeout",
				Type:        "duration",
				Title:       "Timeout",
				Description: "Maximum total execution time for a workflow",
				Tooltip:     "Format: '12h', '30m', '1h30m'. Default: 12 hours.",
				Default:     "12h",
				Required:    true,
				Category:    "basic",
			},
			{
				Path:        "workflow.max_retries",
				Type:        "int",
				Title:       "Max Retries",
				Description: "Number of retry attempts for failed tasks",
				Tooltip:     "Set to 0 to disable retries. Maximum: 10.",
				Default:     3,
				Required:    true,
				Min:         &min0,
				Max:         &max10,
				Category:    "basic",
			},
			{
				Path:        "workflow.sandbox",
				Type:        "bool",
				Title:       "Sandbox Mode",
				Description: "Restrict dangerous operations",
				Tooltip:     "When enabled, restricts operations that could modify the system outside the project directory. Recommended for safety.",
				Default:     true,
				DangerLevel: "danger",
				Category:    "basic",
			},
			{
				Path:        "workflow.dry_run",
				Type:        "bool",
				Title:       "Dry Run",
				Description: "Simulate without executing",
				Tooltip:     "Simulates workflow execution without actually running agents. Useful for testing configurations.",
				Default:     false,
				Category:    "basic",
			},
			{
				Path:        "workflow.deny_tools",
				Type:        "[]string",
				Title:       "Denied Tools",
				Description: "List of tools to block during execution",
				Tooltip:     "Example: ['rm', 'curl']. These tools will be blocked from agent use.",
				Default:     []string{},
				Category:    "advanced",
			},
		},
	}
}

func buildStateSection() SchemaSection {
	return SchemaSection{
		ID:          "state",
		Title:       "State Persistence",
		Description: "Configure workflow state storage",
		Tab:         "workflow",
		Fields: []SchemaField{
			{
				Path:        "state.backend",
				Type:        "string",
				Title:       "Backend",
				Description: "State storage backend",
				Tooltip:     "'sqlite': recommended for large workflows and concurrent access. 'json': human-readable, good for debugging.",
				Default:     "sqlite",
				ValidValues: []string{"sqlite", "json"},
				Category:    "advanced",
			},
			{
				Path:        "state.path",
				Type:        "string",
				Title:       "State Path",
				Description: "Path to state database",
				Tooltip:     "Extension adjusted automatically based on backend.",
				Default:     ".quorum/state/state.db",
				Required:    true,
				Category:    "advanced",
			},
			{
				Path:        "state.lock_ttl",
				Type:        "duration",
				Title:       "Lock TTL",
				Description: "Lock time-to-live before considered stale",
				Tooltip:     "Default: 1 hour. Locks older than this are considered stale.",
				Default:     "1h",
				Category:    "advanced",
			},
		},
	}
}

func buildAgentsSection() SchemaSection {
	return SchemaSection{
		ID:          "agents",
		Title:       "Agent Configuration",
		Description: "Configure AI agents and their settings",
		Tab:         "agents",
		Fields: []SchemaField{
			{
				Path:        "agents.default",
				Type:        "string",
				Title:       "Default Agent",
				Description: "Default agent for single-agent operations",
				Tooltip:     "Must be an enabled agent. Used when no specific agent is specified.",
				Default:     "",
				ValidValues: []string{"claude", "gemini", "codex", "copilot", "opencode"},
				Category:    "basic",
			},
		},
	}
}

func buildPhasesAnalyzeSection() SchemaSection {
	min0 := float64(0)
	max1 := float64(1)
	min1 := float64(1)
	max10 := float64(10)

	return SchemaSection{
		ID:          "phases.analyze",
		Title:       "Analyze Phase",
		Description: "Configure the analysis phase",
		Tab:         "phases",
		Fields: []SchemaField{
			{
				Path:        "phases.analyze.timeout",
				Type:        "duration",
				Title:       "Timeout",
				Description: "Maximum duration for analysis phase",
				Tooltip:     "Default: 2 hours. Format: '2h', '90m'.",
				Default:     "2h",
				Category:    "basic",
			},
			// Refiner
			{
				Path:        "phases.analyze.refiner.enabled",
				Type:        "bool",
				Title:       "Enable Refiner",
				Description: "Enhance prompts before analysis",
				Tooltip:     "Enhances user prompts for better LLM effectiveness. Original prompt is preserved.",
				Default:     false,
				Category:    "advanced",
			},
			{
				Path:        "phases.analyze.refiner.agent",
				Type:        "string",
				Title:       "Refiner Agent",
				Description: "Agent to use for refinement",
				Tooltip:     "Must be an enabled agent.",
				Default:     "",
				ValidValues: []string{"claude", "gemini", "codex", "copilot", "opencode"},
				DependsOn:   &FieldDependency{Field: "phases.analyze.refiner.enabled", Value: true},
				Category:    "advanced",
			},
			// Moderator
			{
				Path:        "phases.analyze.moderator.enabled",
				Type:        "bool",
				Title:       "Enable Moderator",
				Description: "Enable multi-agent consensus evaluation",
				Tooltip:     "Enables consensus evaluation between agents. Cannot be used with single-agent mode.",
				Default:     false,
				Category:    "basic",
			},
			{
				Path:        "phases.analyze.moderator.agent",
				Type:        "string",
				Title:       "Moderator Agent",
				Description: "Agent to evaluate consensus",
				Tooltip:     "Must be an enabled agent.",
				Default:     "",
				ValidValues: []string{"claude", "gemini", "codex", "copilot", "opencode"},
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "basic",
			},
			{
				Path:        "phases.analyze.moderator.threshold",
				Type:        "float",
				Title:       "Consensus Threshold",
				Description: "Score required to proceed (0.0-1.0)",
				Tooltip:     "Higher values require more agreement. Default: 0.80.",
				Default:     0.80,
				Min:         &min0,
				Max:         &max1,
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "advanced",
			},
			{
				Path:        "phases.analyze.moderator.min_rounds",
				Type:        "int",
				Title:       "Minimum Rounds",
				Description: "Minimum refinement rounds before accepting",
				Tooltip:     "Default: 2. At least this many rounds before consensus can be accepted.",
				Default:     2,
				Min:         &min1,
				Max:         &max10,
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "advanced",
			},
			{
				Path:        "phases.analyze.moderator.max_rounds",
				Type:        "int",
				Title:       "Maximum Rounds",
				Description: "Maximum refinement rounds",
				Tooltip:     "Default: 5. Stops after this many rounds even without consensus.",
				Default:     5,
				Min:         &min1,
				Max:         &max10,
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "advanced",
			},
			{
				Path:        "phases.analyze.moderator.abort_threshold",
				Type:        "float",
				Title:       "Abort Threshold",
				Description: "Score below which workflow is aborted",
				Tooltip:     "Triggers human review if score drops below this. Default: 0.30.",
				Default:     0.30,
				Min:         &min0,
				Max:         &max1,
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "advanced",
			},
			{
				Path:        "phases.analyze.moderator.stagnation_threshold",
				Type:        "float",
				Title:       "Stagnation Threshold",
				Description: "Minimum improvement per round",
				Tooltip:     "Triggers early exit if improvement is below this. Default: 0.02.",
				Default:     0.02,
				Min:         &min0,
				Max:         &max1,
				DependsOn:   &FieldDependency{Field: "phases.analyze.moderator.enabled", Value: true},
				Category:    "advanced",
			},
			// Synthesizer
			{
				Path:        "phases.analyze.synthesizer.agent",
				Type:        "string",
				Title:       "Synthesizer Agent",
				Description: "Agent to combine analyses",
				Tooltip:     "Combines all agent analyses into unified report. Optional.",
				Default:     "",
				ValidValues: []string{"", "claude", "gemini", "codex", "copilot", "opencode"},
				Category:    "advanced",
			},
			// Single Agent
			{
				Path:        "phases.analyze.single_agent.enabled",
				Type:        "bool",
				Title:       "Single Agent Mode",
				Description: "Bypass multi-agent consensus",
				Tooltip:     "Uses single agent for analysis. Cannot be used with moderator enabled.",
				Default:     false,
				Category:    "basic",
			},
			{
				Path:        "phases.analyze.single_agent.agent",
				Type:        "string",
				Title:       "Single Agent",
				Description: "Agent for single-agent analysis",
				Tooltip:     "Must be an enabled agent.",
				Default:     "",
				ValidValues: []string{"claude", "gemini", "codex", "copilot", "opencode"},
				DependsOn:   &FieldDependency{Field: "phases.analyze.single_agent.enabled", Value: true},
				Category:    "basic",
			},
			{
				Path:        "phases.analyze.single_agent.model",
				Type:        "string",
				Title:       "Model Override",
				Description: "Override agent's default model",
				Tooltip:     "Optional. Uses agent's default model if empty.",
				Default:     "",
				DependsOn:   &FieldDependency{Field: "phases.analyze.single_agent.enabled", Value: true},
				Category:    "advanced",
			},
		},
	}
}

func buildPhasesPlanSection() SchemaSection {
	return SchemaSection{
		ID:          "phases.plan",
		Title:       "Plan Phase",
		Description: "Configure the planning phase",
		Tab:         "phases",
		Fields: []SchemaField{
			{
				Path:        "phases.plan.timeout",
				Type:        "duration",
				Title:       "Timeout",
				Description: "Maximum duration for planning phase",
				Tooltip:     "Default: 1 hour. Format: '1h', '30m'.",
				Default:     "1h",
				Category:    "basic",
			},
			{
				Path:        "phases.plan.synthesizer.enabled",
				Type:        "bool",
				Title:       "Enable Multi-Agent Planning",
				Description: "Multiple agents propose plans, then synthesize",
				Tooltip:     "When enabled, multiple agents create plans in parallel, then a synthesizer combines them.",
				Default:     false,
				Category:    "advanced",
			},
			{
				Path:        "phases.plan.synthesizer.agent",
				Type:        "string",
				Title:       "Plan Synthesizer Agent",
				Description: "Agent to synthesize plans",
				Tooltip:     "Must be an enabled agent.",
				Default:     "",
				ValidValues: []string{"", "claude", "gemini", "codex", "copilot", "opencode"},
				DependsOn:   &FieldDependency{Field: "phases.plan.synthesizer.enabled", Value: true},
				Category:    "advanced",
			},
		},
	}
}

func buildPhasesExecuteSection() SchemaSection {
	return SchemaSection{
		ID:          "phases.execute",
		Title:       "Execute Phase",
		Description: "Configure the execution phase",
		Tab:         "phases",
		Fields: []SchemaField{
			{
				Path:        "phases.execute.timeout",
				Type:        "duration",
				Title:       "Timeout",
				Description: "Maximum duration for execution phase",
				Tooltip:     "Default: 2 hours. Format: '2h', '90m'.",
				Default:     "2h",
				Category:    "basic",
			},
		},
	}
}

func buildGitSection() SchemaSection {
	return SchemaSection{
		ID:          "git",
		Title:       "Git Integration",
		Description: "Configure git operations and automation",
		Tab:         "git",
		Fields: []SchemaField{
			{
				Path:        "git.worktree_dir",
				Type:        "string",
				Title:       "Worktree Directory",
				Description: "Directory for git worktrees",
				Tooltip:     "Default: '.worktrees'. Created under project root.",
				Default:     ".worktrees",
				Required:    true,
				Category:    "basic",
			},
			{
				Path:        "git.worktree_mode",
				Type:        "string",
				Title:       "Worktree Mode",
				Description: "When to create worktrees",
				Tooltip:     "'always': every task gets worktree, 'parallel': only concurrent tasks, 'disabled': no isolation.",
				Default:     "always",
				ValidValues: []string{"always", "parallel", "disabled"},
				Category:    "basic",
			},
			{
				Path:        "git.auto_clean",
				Type:        "bool",
				Title:       "Auto Clean",
				Description: "Remove worktrees after task completion",
				Tooltip:     "Recommended to save disk space.",
				Default:     true,
				Category:    "basic",
			},
			{
				Path:        "git.auto_commit",
				Type:        "bool",
				Title:       "Auto Commit",
				Description: "Automatically commit changes after task",
				Tooltip:     "Creates a commit when task completes successfully.",
				Default:     false,
				Category:    "basic",
			},
			{
				Path:        "git.auto_push",
				Type:        "bool",
				Title:       "Auto Push",
				Description: "Push branch to remote after commit",
				Tooltip:     "Requires auto_commit. Pushes task branch to remote.",
				Default:     false,
				DependsOn:   &FieldDependency{Field: "git.auto_commit", Value: true},
				Category:    "basic",
			},
			{
				Path:        "git.auto_pr",
				Type:        "bool",
				Title:       "Auto PR",
				Description: "Create pull request for task branch",
				Tooltip:     "Requires auto_push. Creates PR automatically.",
				Default:     false,
				DependsOn:   &FieldDependency{Field: "git.auto_push", Value: true},
				Category:    "basic",
			},
			{
				Path:        "git.auto_merge",
				Type:        "bool",
				Title:       "Auto Merge",
				Description: "Merge PR immediately after creation",
				Tooltip:     "Requires auto_pr. Use with caution! Merges without human review.",
				Default:     false,
				DangerLevel: "danger",
				DependsOn:   &FieldDependency{Field: "git.auto_pr", Value: true},
				Category:    "basic",
			},
			{
				Path:        "git.pr_base_branch",
				Type:        "string",
				Title:       "PR Base Branch",
				Description: "Target branch for pull requests",
				Tooltip:     "Leave empty to use current branch as base.",
				Default:     "",
				Category:    "advanced",
			},
			{
				Path:        "git.merge_strategy",
				Type:        "string",
				Title:       "Merge Strategy",
				Description: "How to merge pull requests",
				Tooltip:     "'merge' preserves history, 'squash' creates single commit, 'rebase' creates linear history.",
				Default:     "squash",
				ValidValues: []string{"merge", "squash", "rebase"},
				Category:    "advanced",
			},
		},
	}
}

func buildGitHubSection() SchemaSection {
	return SchemaSection{
		ID:          "github",
		Title:       "GitHub Settings",
		Description: "Configure GitHub integration",
		Tab:         "git",
		Fields: []SchemaField{
			{
				Path:        "github.remote",
				Type:        "string",
				Title:       "Remote Name",
				Description: "Git remote for GitHub operations",
				Tooltip:     "Default: 'origin'. The git remote to use for push/PR operations.",
				Default:     "origin",
				Required:    true,
				Category:    "basic",
			},
		},
	}
}

func buildTraceSection() SchemaSection {
	return SchemaSection{
		ID:          "trace",
		Title:       "Trace Settings",
		Description: "Configure execution tracing",
		Tab:         "advanced",
		Fields: []SchemaField{
			{
				Path:        "trace.mode",
				Type:        "string",
				Title:       "Trace Mode",
				Description: "Level of tracing detail",
				Tooltip:     "'off' disables tracing, 'summary' for high-level, 'full' for detailed output.",
				Default:     "off",
				ValidValues: []string{"off", "summary", "full"},
				Category:    "basic",
			},
			{
				Path:        "trace.dir",
				Type:        "string",
				Title:       "Trace Directory",
				Description: "Directory for trace files",
				Tooltip:     "Example: '.quorum/traces'.",
				Default:     ".quorum/traces",
				Required:    true,
				Category:    "advanced",
			},
			{
				Path:        "trace.redact",
				Type:        "bool",
				Title:       "Redact Sensitive Data",
				Description: "Redact sensitive data in traces",
				Tooltip:     "Recommended for security. Redacts API keys, tokens, etc.",
				Default:     true,
				DangerLevel: "warning",
				Category:    "basic",
			},
			{
				Path:        "trace.redact_patterns",
				Type:        "[]string",
				Title:       "Redact Patterns",
				Description: "Regex patterns to redact",
				Tooltip:     "Example: 'AKIA[0-9A-Z]{16}' for AWS keys.",
				Default:     []string{},
				Category:    "advanced",
			},
			{
				Path:        "trace.max_bytes",
				Type:        "int",
				Title:       "Max Bytes Per File",
				Description: "Maximum bytes per trace file",
				Tooltip:     "Example: 262144 (256KB).",
				Default:     262144,
				Category:    "advanced",
			},
			{
				Path:        "trace.total_max_bytes",
				Type:        "int",
				Title:       "Total Max Bytes",
				Description: "Total trace size cap per workflow",
				Tooltip:     "Example: 10485760 (10MB).",
				Default:     10485760,
				Category:    "advanced",
			},
			{
				Path:        "trace.max_files",
				Type:        "int",
				Title:       "Max Files",
				Description: "Maximum trace files to keep",
				Tooltip:     "Older files are deleted when limit is reached.",
				Default:     500,
				Category:    "advanced",
			},
			{
				Path:        "trace.include_phases",
				Type:        "[]string",
				Title:       "Include Phases",
				Description: "Which phases to trace",
				Tooltip:     "Valid: refine, analyze, plan, execute.",
				Default:     []string{"analyze", "plan", "execute"},
				ValidValues: []string{"refine", "analyze", "plan", "execute"},
				Category:    "advanced",
			},
		},
	}
}

func buildDiagnosticsSection() SchemaSection {
	return SchemaSection{
		ID:          "diagnostics",
		Title:       "Diagnostics",
		Description: "Configure system diagnostics and monitoring",
		Tab:         "advanced",
		Fields: []SchemaField{
			{
				Path:        "diagnostics.enabled",
				Type:        "bool",
				Title:       "Enable Diagnostics",
				Description: "Enable system diagnostics for process resilience",
				Tooltip:     "Recommended for long-running sessions.",
				Default:     true,
				Category:    "basic",
			},
			// Resource Monitoring
			{
				Path:        "diagnostics.resource_monitoring.interval",
				Type:        "duration",
				Title:       "Monitoring Interval",
				Description: "How often to take resource snapshots",
				Tooltip:     "Default: 30 seconds.",
				Default:     "30s",
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.resource_monitoring.fd_threshold_percent",
				Type:        "int",
				Title:       "FD Threshold (%)",
				Description: "File descriptor usage warning threshold",
				Tooltip:     "Triggers warning when FD usage exceeds this percentage (0-100).",
				Default:     80,
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.resource_monitoring.goroutine_threshold",
				Type:        "int",
				Title:       "Goroutine Threshold",
				Description: "Goroutine count warning threshold",
				Tooltip:     "Triggers warning when goroutine count exceeds this.",
				Default:     10000,
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.resource_monitoring.memory_threshold_mb",
				Type:        "int",
				Title:       "Memory Threshold (MB)",
				Description: "Heap memory warning threshold",
				Tooltip:     "Triggers warning when heap memory exceeds this (in MB).",
				Default:     4096,
				Category:    "advanced",
			},
			// Crash Dump
			{
				Path:        "diagnostics.crash_dump.dir",
				Type:        "string",
				Title:       "Crash Dump Directory",
				Description: "Directory for crash dump files",
				Tooltip:     "Default: '.quorum/crashdumps'.",
				Default:     ".quorum/crashdumps",
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.crash_dump.max_files",
				Type:        "int",
				Title:       "Max Crash Dumps",
				Description: "Maximum crash dumps to retain",
				Tooltip:     "Older dumps are deleted when limit is reached.",
				Default:     10,
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.crash_dump.include_stack",
				Type:        "bool",
				Title:       "Include Stack Traces",
				Description: "Include goroutine stacks in crash dumps",
				Tooltip:     "Helpful for debugging but increases dump size.",
				Default:     true,
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.crash_dump.include_env",
				Type:        "bool",
				Title:       "Include Environment",
				Description: "Include environment variables in crash dumps",
				Tooltip:     "May expose sensitive data like API keys!",
				Default:     false,
				DangerLevel: "warning",
				Category:    "advanced",
			},
			// Preflight
			{
				Path:        "diagnostics.preflight_checks.enabled",
				Type:        "bool",
				Title:       "Enable Preflight Checks",
				Description: "Run health checks before execution",
				Tooltip:     "Aborts if resources are insufficient.",
				Default:     true,
				Category:    "basic",
			},
			{
				Path:        "diagnostics.preflight_checks.min_free_fd_percent",
				Type:        "int",
				Title:       "Min Free FD (%)",
				Description: "Minimum free file descriptors required",
				Tooltip:     "Aborts if free FD percentage is below this.",
				Default:     20,
				Category:    "advanced",
			},
			{
				Path:        "diagnostics.preflight_checks.min_free_memory_mb",
				Type:        "int",
				Title:       "Min Free Memory (MB)",
				Description: "Minimum free memory required",
				Tooltip:     "Aborts if estimated free memory is below this.",
				Default:     256,
				Category:    "advanced",
			},
		},
	}
}
