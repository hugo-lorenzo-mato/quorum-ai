package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

// Note: DTO types are defined in config_types.go

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadConfig()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Calculate ETag
	etag, err := calculateETag(cfg)
	if err != nil {
		s.logger.Error("failed to calculate ETag", "error", err)
		// Non-fatal, continue without ETag
		etag = ""
	}

	// Get file metadata
	configPath := s.getConfigPath()
	meta, _ := getConfigFileMeta(configPath)

	// Set ETag header
	if etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
	}

	// Check If-None-Match for conditional GET
	if clientETag := r.Header.Get("If-None-Match"); clientETag != "" {
		// Remove quotes if present
		clientETag = strings.Trim(clientETag, `"`)
		if clientETag == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Determine last modified time
	lastModified := ""
	if meta.Exists {
		lastModified = meta.LastModified.Format(time.RFC3339)
	}

	response := ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:         etag,
			LastModified: lastModified,
			Source:       s.determineConfigSource(configPath),
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// determineConfigSource returns whether config comes from file or defaults.
func (s *Server) determineConfigSource(configPath string) string {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "default"
	}
	return "file"
}

// handleUpdateConfig updates configuration values.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req FullConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	configPath := s.getConfigPath()

	// Check for force update flag
	forceUpdate := r.URL.Query().Get("force") == "true"

	// Check If-Match header for ETag validation (unless forcing)
	clientETag := r.Header.Get("If-Match")
	if !forceUpdate && clientETag != "" {
		clientETag = strings.Trim(clientETag, `"`)

		matches, currentETag, err := ETagMatch(clientETag, configPath)
		if err != nil {
			s.logger.Error("failed to check ETag", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to check configuration version")
			return
		}

		if !matches {
			// Conflict detected!
			s.logger.Warn("config update conflict",
				"client_etag", clientETag,
				"current_etag", currentETag)

			// Return 412 with current config and ETag
			cfg, loadErr := s.loadConfig()
			if loadErr != nil {
				respondError(w, http.StatusInternalServerError, "failed to load current configuration")
				return
			}

			w.Header().Set("ETag", fmt.Sprintf("%q", currentETag))
			respondJSON(w, http.StatusPreconditionFailed, map[string]interface{}{
				"error":          "Configuration was modified externally",
				"code":           "CONFLICT",
				"current_etag":   currentETag,
				"current_config": configToFullResponse(cfg),
			})
			return
		}
	}

	// Load current config
	cfg, err := s.loadConfig()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Apply updates
	applyFullConfigUpdates(cfg, &req)

	// Validate merged config before saving
	if !validateConfig(w, cfg, s.logger) {
		return
	}

	// Save with atomic write
	if err := atomicWriteConfig(cfg, configPath); err != nil {
		s.logger.Error("failed to save config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}

	// Calculate new ETag
	newETag, _ := calculateETag(cfg)
	w.Header().Set("ETag", fmt.Sprintf("%q", newETag))

	response := ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:   newETag,
			Source: "file",
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// handleResetConfig resets configuration to defaults.
func (s *Server) handleResetConfig(w http.ResponseWriter, _ *http.Request) {
	configPath := s.getConfigPath()

	// Ensure .quorum directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		s.logger.Error("failed to create config directory", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create configuration directory")
		return
	}

	// Write the default configuration (same as quorum init)
	if err := os.WriteFile(configPath, []byte(config.DefaultConfigYAML), 0o600); err != nil {
		s.logger.Error("failed to write default config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to write default configuration")
		return
	}
	s.logger.Info("config reset to defaults", "path", configPath)

	// Load the newly written config
	cfg, err := config.NewLoader().WithConfigFile(configPath).Load()
	if err != nil {
		s.logger.Error("failed to load default config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load default configuration")
		return
	}

	// Calculate ETag for the new config
	etag, _ := calculateETag(cfg)
	if etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
	}

	response := ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:   etag,
			Source: "file",
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetAgents returns available agents and their status.
// Models are synced with internal/adapters/cli/*.go SupportedModels.
func (s *Server) handleGetAgents(w http.ResponseWriter, _ *http.Request) {
	agents := []map[string]interface{}{
		{
			"name":        "claude",
			"displayName": "Claude",
			// Synced with internal/adapters/cli/claude.go
			"models": []string{
				"claude-opus-4-5-20251101",
				"claude-sonnet-4-5-20250929",
				"claude-haiku-4-5-20251001",
				"claude-sonnet-4-20250514",
				"claude-opus-4-20250514",
				"claude-opus-4-1-20250805",
			},
			"available": true,
		},
		{
			"name":        "gemini",
			"displayName": "Gemini",
			// Synced with internal/adapters/cli/gemini.go
			"models": []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-2.5-flash-lite",
				"gemini-3-pro-preview",
				"gemini-3-flash-preview",
			},
			"available": true,
		},
		{
			"name":        "codex",
			"displayName": "Codex",
			// Synced with internal/adapters/cli/codex.go
			"models": []string{
				"gpt-5.2",
				"gpt-5.2-codex",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1-codex-mini",
				"gpt-5.1",
				"gpt-5",
				"gpt-5-mini",
				"gpt-4.1",
				"o3",
				"o4-mini",
			},
			"hasReasoningEffort": true,
			"available":          true,
		},
		{
			"name":        "copilot",
			"displayName": "Copilot",
			// Synced with internal/adapters/cli/copilot.go
			"models": []string{
				"claude-sonnet-4.5",
				"claude-haiku-4.5",
				"claude-opus-4.5",
				"claude-sonnet-4",
				"gpt-5.2-codex",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.2",
				"gpt-5.1",
				"gpt-5",
				"gpt-5.1-codex-mini",
				"gpt-5-mini",
				"gpt-4.1",
				"gemini-3-pro-preview",
			},
			"available": true,
		},
		{
			"name":        "opencode",
			"displayName": "OpenCode",
			// Synced with internal/adapters/cli/opencode.go
			"models": []string{
				"qwen2.5-coder",
				"deepseek-coder-v2",
				"llama3.1",
				"deepseek-r1",
			},
			"available": true,
		},
	}

	respondJSON(w, http.StatusOK, agents)
}

// loadConfig loads the configuration from file using the config loader.
func (s *Server) loadConfig() (*config.Config, error) {
	configPath := s.getConfigPath()

	// Check if config file exists, if not use defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config.NewLoader().Load()
	}

	// Load using config loader for consistency
	return config.NewLoader().WithConfigFile(configPath).Load()
}

// getConfigPath returns the config file path.
func (s *Server) getConfigPath() string {
	return filepath.Join(".quorum", "config.yaml")
}

// configToFullResponse converts *config.Config to FullConfigResponse.
func configToFullResponse(cfg *config.Config) FullConfigResponse {
	// Ensure slices are never nil (use empty slices instead)
	denyTools := cfg.Workflow.DenyTools
	if denyTools == nil {
		denyTools = []string{}
	}
	redactPatterns := cfg.Trace.RedactPatterns
	if redactPatterns == nil {
		redactPatterns = []string{}
	}
	redactAllowlist := cfg.Trace.RedactAllowlist
	if redactAllowlist == nil {
		redactAllowlist = []string{}
	}
	includePhases := cfg.Trace.IncludePhases
	if includePhases == nil {
		includePhases = []string{}
	}

	return FullConfigResponse{
		Log: LogConfigResponse{
			Level:  cfg.Log.Level,
			Format: cfg.Log.Format,
		},
		Trace: TraceConfigResponse{
			Mode:            cfg.Trace.Mode,
			Dir:             cfg.Trace.Dir,
			SchemaVersion:   cfg.Trace.SchemaVersion,
			Redact:          cfg.Trace.Redact,
			RedactPatterns:  redactPatterns,
			RedactAllowlist: redactAllowlist,
			MaxBytes:        cfg.Trace.MaxBytes,
			TotalMaxBytes:   cfg.Trace.TotalMaxBytes,
			MaxFiles:        cfg.Trace.MaxFiles,
			IncludePhases:   includePhases,
		},
		Workflow: WorkflowConfigResponse{
			Timeout:    cfg.Workflow.Timeout,
			MaxRetries: cfg.Workflow.MaxRetries,
			DryRun:     cfg.Workflow.DryRun,
			Sandbox:    cfg.Workflow.Sandbox,
			DenyTools:  denyTools,
		},
		Phases: PhasesConfigResponse{
			Analyze: AnalyzePhaseConfigResponse{
				Timeout: cfg.Phases.Analyze.Timeout,
				Refiner: RefinerConfigResponse{
					Enabled: cfg.Phases.Analyze.Refiner.Enabled,
					Agent:   cfg.Phases.Analyze.Refiner.Agent,
				},
				Moderator: ModeratorConfigResponse{
					Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
					Agent:               cfg.Phases.Analyze.Moderator.Agent,
					Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
					MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
					MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
					AbortThreshold:      cfg.Phases.Analyze.Moderator.AbortThreshold,
					StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
				},
				Synthesizer: SynthesizerConfigResponse{
					Agent: cfg.Phases.Analyze.Synthesizer.Agent,
				},
				SingleAgent: SingleAgentConfigResponse{
					Enabled: cfg.Phases.Analyze.SingleAgent.Enabled,
					Agent:   cfg.Phases.Analyze.SingleAgent.Agent,
					Model:   cfg.Phases.Analyze.SingleAgent.Model,
				},
			},
			Plan: PlanPhaseConfigResponse{
				Timeout: cfg.Phases.Plan.Timeout,
				Synthesizer: PlanSynthesizerConfigResponse{
					Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
					Agent:   cfg.Phases.Plan.Synthesizer.Agent,
				},
			},
			Execute: ExecutePhaseConfigResponse{
				Timeout: cfg.Phases.Execute.Timeout,
			},
		},
		Agents: AgentsConfigResponse{
			Default:  cfg.Agents.Default,
			Claude:   agentConfigToResponse(&cfg.Agents.Claude),
			Gemini:   agentConfigToResponse(&cfg.Agents.Gemini),
			Codex:    agentConfigToResponse(&cfg.Agents.Codex),
			Copilot:  agentConfigToResponse(&cfg.Agents.Copilot),
			OpenCode: agentConfigToResponse(&cfg.Agents.OpenCode),
		},
		State: StateConfigResponse{
			Backend:    cfg.State.Backend,
			Path:       cfg.State.Path,
			BackupPath: cfg.State.BackupPath,
			LockTTL:    cfg.State.LockTTL,
		},
		Git: GitConfigResponse{
			WorktreeDir:   cfg.Git.WorktreeDir,
			AutoClean:     cfg.Git.AutoClean,
			WorktreeMode:  cfg.Git.WorktreeMode,
			AutoCommit:    cfg.Git.AutoCommit,
			AutoPush:      cfg.Git.AutoPush,
			AutoPR:        cfg.Git.AutoPR,
			AutoMerge:     cfg.Git.AutoMerge,
			PRBaseBranch:  cfg.Git.PRBaseBranch,
			MergeStrategy: cfg.Git.MergeStrategy,
		},
		GitHub: GitHubConfigResponse{
			Remote: cfg.GitHub.Remote,
		},
		Chat: ChatConfigResponse{
			Timeout:          cfg.Chat.Timeout,
			ProgressInterval: cfg.Chat.ProgressInterval,
			Editor:           cfg.Chat.Editor,
		},
		Report: ReportConfigResponse{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
		Diagnostics: DiagnosticsConfigResponse{
			Enabled: cfg.Diagnostics.Enabled,
			ResourceMonitoring: ResourceMonitoringConfigResponse{
				Interval:           cfg.Diagnostics.ResourceMonitoring.Interval,
				FDThresholdPercent: cfg.Diagnostics.ResourceMonitoring.FDThresholdPercent,
				GoroutineThreshold: cfg.Diagnostics.ResourceMonitoring.GoroutineThreshold,
				MemoryThresholdMB:  cfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB,
				HistorySize:        cfg.Diagnostics.ResourceMonitoring.HistorySize,
			},
			CrashDump: CrashDumpConfigResponse{
				Dir:          cfg.Diagnostics.CrashDump.Dir,
				MaxFiles:     cfg.Diagnostics.CrashDump.MaxFiles,
				IncludeStack: cfg.Diagnostics.CrashDump.IncludeStack,
				IncludeEnv:   cfg.Diagnostics.CrashDump.IncludeEnv,
			},
			PreflightChecks: PreflightConfigResponse{
				Enabled:          cfg.Diagnostics.PreflightChecks.Enabled,
				MinFreeFDPercent: cfg.Diagnostics.PreflightChecks.MinFreeFDPercent,
				MinFreeMemoryMB:  cfg.Diagnostics.PreflightChecks.MinFreeMemoryMB,
			},
		},
	}
}

// agentConfigToResponse converts *config.AgentConfig to FullAgentConfigResponse.
func agentConfigToResponse(cfg *config.AgentConfig) FullAgentConfigResponse {
	phaseModels := cfg.PhaseModels
	if phaseModels == nil {
		phaseModels = make(map[string]string)
	}
	phases := cfg.Phases
	if phases == nil {
		phases = make(map[string]bool)
	}
	reasoningEffortPhases := cfg.ReasoningEffortPhases
	if reasoningEffortPhases == nil {
		reasoningEffortPhases = make(map[string]string)
	}

	return FullAgentConfigResponse{
		Enabled:                   cfg.Enabled,
		Path:                      cfg.Path,
		Model:                     cfg.Model,
		PhaseModels:               phaseModels,
		Phases:                    phases,
		ReasoningEffort:           cfg.ReasoningEffort,
		ReasoningEffortPhases:     reasoningEffortPhases,
		TokenDiscrepancyThreshold: cfg.TokenDiscrepancyThreshold,
	}
}

// applyFullConfigUpdates applies partial updates to config.
func applyFullConfigUpdates(cfg *config.Config, req *FullConfigUpdate) {
	if req.Log != nil {
		applyLogUpdates(&cfg.Log, req.Log)
	}
	if req.Trace != nil {
		applyTraceUpdates(&cfg.Trace, req.Trace)
	}
	if req.Workflow != nil {
		applyWorkflowUpdates(&cfg.Workflow, req.Workflow)
	}
	if req.Phases != nil {
		applyPhasesUpdates(&cfg.Phases, req.Phases)
	}
	if req.Agents != nil {
		applyAgentsUpdates(&cfg.Agents, req.Agents)
	}
	if req.State != nil {
		applyStateUpdates(&cfg.State, req.State)
	}
	if req.Git != nil {
		applyGitUpdates(&cfg.Git, req.Git)
	}
	if req.GitHub != nil {
		applyGitHubUpdates(&cfg.GitHub, req.GitHub)
	}
	if req.Chat != nil {
		applyChatUpdates(&cfg.Chat, req.Chat)
	}
	if req.Report != nil {
		applyReportUpdates(&cfg.Report, req.Report)
	}
	if req.Diagnostics != nil {
		applyDiagnosticsUpdates(&cfg.Diagnostics, req.Diagnostics)
	}
}

func applyLogUpdates(cfg *config.LogConfig, update *LogConfigUpdate) {
	if update.Level != nil {
		cfg.Level = *update.Level
	}
	if update.Format != nil {
		cfg.Format = *update.Format
	}
}

func applyTraceUpdates(cfg *config.TraceConfig, update *TraceConfigUpdate) {
	if update.Mode != nil {
		cfg.Mode = *update.Mode
	}
	if update.Dir != nil {
		cfg.Dir = *update.Dir
	}
	if update.SchemaVersion != nil {
		cfg.SchemaVersion = *update.SchemaVersion
	}
	if update.Redact != nil {
		cfg.Redact = *update.Redact
	}
	if update.RedactPatterns != nil {
		cfg.RedactPatterns = *update.RedactPatterns
	}
	if update.RedactAllowlist != nil {
		cfg.RedactAllowlist = *update.RedactAllowlist
	}
	if update.MaxBytes != nil {
		cfg.MaxBytes = *update.MaxBytes
	}
	if update.TotalMaxBytes != nil {
		cfg.TotalMaxBytes = *update.TotalMaxBytes
	}
	if update.MaxFiles != nil {
		cfg.MaxFiles = *update.MaxFiles
	}
	if update.IncludePhases != nil {
		cfg.IncludePhases = *update.IncludePhases
	}
}

func applyWorkflowUpdates(cfg *config.WorkflowConfig, update *WorkflowConfigUpdate) {
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.MaxRetries != nil {
		cfg.MaxRetries = *update.MaxRetries
	}
	if update.DryRun != nil {
		cfg.DryRun = *update.DryRun
	}
	if update.Sandbox != nil {
		cfg.Sandbox = *update.Sandbox
	}
	if update.DenyTools != nil {
		cfg.DenyTools = *update.DenyTools
	}
}

func applyPhasesUpdates(cfg *config.PhasesConfig, update *PhasesConfigUpdate) {
	if update.Analyze != nil {
		applyAnalyzePhaseUpdates(&cfg.Analyze, update.Analyze)
	}
	if update.Plan != nil {
		applyPlanPhaseUpdates(&cfg.Plan, update.Plan)
	}
	if update.Execute != nil {
		applyExecutePhaseUpdates(&cfg.Execute, update.Execute)
	}
}

func applyAnalyzePhaseUpdates(cfg *config.AnalyzePhaseConfig, update *AnalyzePhaseConfigUpdate) {
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.Refiner != nil {
		if update.Refiner.Enabled != nil {
			cfg.Refiner.Enabled = *update.Refiner.Enabled
		}
		if update.Refiner.Agent != nil {
			cfg.Refiner.Agent = *update.Refiner.Agent
		}
	}
	if update.Moderator != nil {
		if update.Moderator.Enabled != nil {
			cfg.Moderator.Enabled = *update.Moderator.Enabled
		}
		if update.Moderator.Agent != nil {
			cfg.Moderator.Agent = *update.Moderator.Agent
		}
		if update.Moderator.Threshold != nil {
			cfg.Moderator.Threshold = *update.Moderator.Threshold
		}
		if update.Moderator.MinRounds != nil {
			cfg.Moderator.MinRounds = *update.Moderator.MinRounds
		}
		if update.Moderator.MaxRounds != nil {
			cfg.Moderator.MaxRounds = *update.Moderator.MaxRounds
		}
		if update.Moderator.AbortThreshold != nil {
			cfg.Moderator.AbortThreshold = *update.Moderator.AbortThreshold
		}
		if update.Moderator.StagnationThreshold != nil {
			cfg.Moderator.StagnationThreshold = *update.Moderator.StagnationThreshold
		}
	}
	if update.Synthesizer != nil {
		if update.Synthesizer.Agent != nil {
			cfg.Synthesizer.Agent = *update.Synthesizer.Agent
		}
	}
	if update.SingleAgent != nil {
		if update.SingleAgent.Enabled != nil {
			cfg.SingleAgent.Enabled = *update.SingleAgent.Enabled
		}
		if update.SingleAgent.Agent != nil {
			cfg.SingleAgent.Agent = *update.SingleAgent.Agent
		}
		if update.SingleAgent.Model != nil {
			cfg.SingleAgent.Model = *update.SingleAgent.Model
		}
	}
}

func applyPlanPhaseUpdates(cfg *config.PlanPhaseConfig, update *PlanPhaseConfigUpdate) {
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.Synthesizer != nil {
		if update.Synthesizer.Enabled != nil {
			cfg.Synthesizer.Enabled = *update.Synthesizer.Enabled
		}
		if update.Synthesizer.Agent != nil {
			cfg.Synthesizer.Agent = *update.Synthesizer.Agent
		}
	}
}

func applyExecutePhaseUpdates(cfg *config.ExecutePhaseConfig, update *ExecutePhaseConfigUpdate) {
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
}

func applyAgentsUpdates(cfg *config.AgentsConfig, update *AgentsConfigUpdate) {
	if update.Default != nil {
		cfg.Default = *update.Default
	}
	if update.Claude != nil {
		applyAgentUpdates(&cfg.Claude, update.Claude)
	}
	if update.Gemini != nil {
		applyAgentUpdates(&cfg.Gemini, update.Gemini)
	}
	if update.Codex != nil {
		applyAgentUpdates(&cfg.Codex, update.Codex)
	}
	if update.Copilot != nil {
		applyAgentUpdates(&cfg.Copilot, update.Copilot)
	}
	if update.OpenCode != nil {
		applyAgentUpdates(&cfg.OpenCode, update.OpenCode)
	}
}

func applyAgentUpdates(cfg *config.AgentConfig, update *FullAgentConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.Path != nil {
		cfg.Path = *update.Path
	}
	if update.Model != nil {
		cfg.Model = *update.Model
	}
	if update.PhaseModels != nil {
		cfg.PhaseModels = *update.PhaseModels
	}
	if update.Phases != nil {
		cfg.Phases = *update.Phases
	}
	if update.ReasoningEffort != nil {
		cfg.ReasoningEffort = *update.ReasoningEffort
	}
	if update.ReasoningEffortPhases != nil {
		cfg.ReasoningEffortPhases = *update.ReasoningEffortPhases
	}
	if update.TokenDiscrepancyThreshold != nil {
		cfg.TokenDiscrepancyThreshold = *update.TokenDiscrepancyThreshold
	}
}

func applyStateUpdates(cfg *config.StateConfig, update *StateConfigUpdate) {
	if update.Backend != nil {
		cfg.Backend = *update.Backend
	}
	if update.Path != nil {
		cfg.Path = *update.Path
	}
	if update.BackupPath != nil {
		cfg.BackupPath = *update.BackupPath
	}
	if update.LockTTL != nil {
		cfg.LockTTL = *update.LockTTL
	}
}

func applyGitUpdates(cfg *config.GitConfig, update *GitConfigUpdate) {
	if update.WorktreeDir != nil {
		cfg.WorktreeDir = *update.WorktreeDir
	}
	if update.AutoClean != nil {
		cfg.AutoClean = *update.AutoClean
	}
	if update.WorktreeMode != nil {
		cfg.WorktreeMode = *update.WorktreeMode
	}
	if update.AutoCommit != nil {
		cfg.AutoCommit = *update.AutoCommit
	}
	if update.AutoPush != nil {
		cfg.AutoPush = *update.AutoPush
	}
	if update.AutoPR != nil {
		cfg.AutoPR = *update.AutoPR
	}
	if update.AutoMerge != nil {
		cfg.AutoMerge = *update.AutoMerge
	}
	if update.PRBaseBranch != nil {
		cfg.PRBaseBranch = *update.PRBaseBranch
	}
	if update.MergeStrategy != nil {
		cfg.MergeStrategy = *update.MergeStrategy
	}
}

func applyGitHubUpdates(cfg *config.GitHubConfig, update *GitHubConfigUpdate) {
	if update.Remote != nil {
		cfg.Remote = *update.Remote
	}
}

func applyChatUpdates(cfg *config.ChatConfig, update *ChatConfigUpdate) {
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.ProgressInterval != nil {
		cfg.ProgressInterval = *update.ProgressInterval
	}
	if update.Editor != nil {
		cfg.Editor = *update.Editor
	}
}

func applyReportUpdates(cfg *config.ReportConfig, update *ReportConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.BaseDir != nil {
		cfg.BaseDir = *update.BaseDir
	}
	if update.UseUTC != nil {
		cfg.UseUTC = *update.UseUTC
	}
	if update.IncludeRaw != nil {
		cfg.IncludeRaw = *update.IncludeRaw
	}
}

func applyDiagnosticsUpdates(cfg *config.DiagnosticsConfig, update *DiagnosticsConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.ResourceMonitoring != nil {
		applyResourceMonitoringUpdates(&cfg.ResourceMonitoring, update.ResourceMonitoring)
	}
	if update.CrashDump != nil {
		applyCrashDumpUpdates(&cfg.CrashDump, update.CrashDump)
	}
	if update.PreflightChecks != nil {
		applyPreflightUpdates(&cfg.PreflightChecks, update.PreflightChecks)
	}
}

func applyResourceMonitoringUpdates(cfg *config.ResourceMonitoringConfig, update *ResourceMonitoringConfigUpdate) {
	if update.Interval != nil {
		cfg.Interval = *update.Interval
	}
	if update.FDThresholdPercent != nil {
		cfg.FDThresholdPercent = *update.FDThresholdPercent
	}
	if update.GoroutineThreshold != nil {
		cfg.GoroutineThreshold = *update.GoroutineThreshold
	}
	if update.MemoryThresholdMB != nil {
		cfg.MemoryThresholdMB = *update.MemoryThresholdMB
	}
	if update.HistorySize != nil {
		cfg.HistorySize = *update.HistorySize
	}
}

func applyCrashDumpUpdates(cfg *config.CrashDumpConfig, update *CrashDumpConfigUpdate) {
	if update.Dir != nil {
		cfg.Dir = *update.Dir
	}
	if update.MaxFiles != nil {
		cfg.MaxFiles = *update.MaxFiles
	}
	if update.IncludeStack != nil {
		cfg.IncludeStack = *update.IncludeStack
	}
	if update.IncludeEnv != nil {
		cfg.IncludeEnv = *update.IncludeEnv
	}
}

func applyPreflightUpdates(cfg *config.PreflightConfig, update *PreflightConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.MinFreeFDPercent != nil {
		cfg.MinFreeFDPercent = *update.MinFreeFDPercent
	}
	if update.MinFreeMemoryMB != nil {
		cfg.MinFreeMemoryMB = *update.MinFreeMemoryMB
	}
}
