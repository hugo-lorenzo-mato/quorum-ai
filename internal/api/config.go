package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// Note: DTO types are defined in config_types.go

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Use read lock to allow concurrent reads
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	configPath, scope, mode, err := s.effectiveConfigPath(ctx)
	if err != nil {
		s.logger.Error("failed to resolve config path", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to resolve configuration path")
		return
	}

	// Preserve relative paths for config editing/viewing in the web UI.
	cfg, err := config.NewLoader().WithConfigFile(configPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Get file metadata and ETag from file (not from marshaled config!)
	// IMPORTANT: ETag must be calculated from file bytes to match PATCH validation
	meta, _ := getConfigFileMeta(configPath)

	// Use file-based ETag for consistency with PATCH validation
	etag := meta.ETag

	// If no file exists (using defaults), generate ETag from default config content
	if etag == "" {
		etag, _ = calculateETag(cfg)
	}

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
			ETag:              etag,
			LastModified:      lastModified,
			Source:            s.determineConfigSource(configPath),
			Scope:             scope,
			ProjectConfigMode: mode,
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

func (s *Server) getProjectConfigMode(ctx context.Context) (string, error) {
	// Legacy mode (no multi-project): treat as custom project config.
	if s.projectRegistry == nil {
		return project.ConfigModeCustom, nil
	}

	projectID := middleware.GetProjectID(ctx)
	if projectID == "" {
		// No project context: treat as custom.
		return project.ConfigModeCustom, nil
	}

	p, err := s.projectRegistry.GetProject(ctx, projectID)
	if err != nil || p == nil {
		// Fail closed to custom (project file) in case of lookup errors.
		return project.ConfigModeCustom, nil
	}

	if p.ConfigMode == project.ConfigModeInheritGlobal || p.ConfigMode == project.ConfigModeCustom {
		return p.ConfigMode, nil
	}

	// Infer if unset: custom if project config exists, otherwise inherit global.
	projectConfigPath := filepath.Join(p.Path, ".quorum", "config.yaml")
	if _, err := os.Stat(projectConfigPath); err == nil {
		return project.ConfigModeCustom, nil
	}
	return project.ConfigModeInheritGlobal, nil
}

// effectiveConfigPath returns the config file path to use for the current request context,
// plus scope/mode metadata for the frontend.
func (s *Server) effectiveConfigPath(ctx context.Context) (path string, scope string, mode string, err error) {
	mode, _ = s.getProjectConfigMode(ctx)
	if mode == project.ConfigModeInheritGlobal {
		globalPath, gpErr := config.EnsureGlobalConfigFile()
		if gpErr != nil {
			return "", "", "", gpErr
		}
		return globalPath, "global", mode, nil
	}
	return s.getProjectConfigPath(ctx), "project", mode, nil
}

// handleUpdateConfig updates configuration values.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Use write lock to prevent concurrent modifications
	s.configMu.Lock()
	defer s.configMu.Unlock()

	var req FullConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mode, _ := s.getProjectConfigMode(ctx)
	if mode == project.ConfigModeInheritGlobal {
		// Project inherits global config - require switching to custom to edit.
		respondJSON(w, http.StatusConflict, map[string]interface{}{
			"error": "project inherits global configuration; switch to custom config to edit project settings",
			"code":  "INHERITS_GLOBAL",
		})
		return
	}

	configPath := s.getProjectConfigPath(ctx)

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
			cfg, loadErr := s.loadConfigForContext(ctx)
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
	cfg, err := s.loadConfigForContext(ctx)
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

	// Calculate new ETag from file (must match how PATCH validation calculates it)
	newETag, _ := calculateETagFromFile(configPath)
	w.Header().Set("ETag", fmt.Sprintf("%q", newETag))

	response := ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:              newETag,
			Source:            "file",
			Scope:             "project",
			ProjectConfigMode: mode,
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// handleResetConfig resets configuration to defaults.
func (s *Server) handleResetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Use write lock to prevent concurrent modifications
	s.configMu.Lock()
	defer s.configMu.Unlock()

	mode, _ := s.getProjectConfigMode(ctx)
	if mode == project.ConfigModeInheritGlobal {
		respondJSON(w, http.StatusConflict, map[string]interface{}{
			"error": "project inherits global configuration; switch to custom config to reset project settings",
			"code":  "INHERITS_GLOBAL",
		})
		return
	}

	configPath := s.getProjectConfigPath(ctx)

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
	cfg, err := config.NewLoader().WithConfigFile(configPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load default config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load default configuration")
		return
	}

	// Calculate ETag from file (must match how PATCH validation calculates it)
	etag, _ := calculateETagFromFile(configPath)
	if etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
	}

	response := ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:              etag,
			Source:            "file",
			Scope:             "project",
			ProjectConfigMode: mode,
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetGlobalConfig returns the global configuration.
func (s *Server) handleGetGlobalConfig(w http.ResponseWriter, r *http.Request) {
	// Use read lock to allow concurrent reads
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	globalPath, err := config.EnsureGlobalConfigFile()
	if err != nil {
		s.logger.Error("failed to ensure global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	cfg, err := config.NewLoader().WithConfigFile(globalPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	meta, _ := getConfigFileMeta(globalPath)
	etag := meta.ETag
	if etag == "" {
		etag, _ = calculateETag(cfg)
	}

	if etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
	}

	// Conditional GET
	if clientETag := r.Header.Get("If-None-Match"); clientETag != "" {
		clientETag = strings.Trim(clientETag, `"`)
		if clientETag == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	lastModified := ""
	if meta.Exists {
		lastModified = meta.LastModified.Format(time.RFC3339)
	}

	respondJSON(w, http.StatusOK, ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:         etag,
			LastModified: lastModified,
			Source:       s.determineConfigSource(globalPath),
			Scope:        "global",
		},
	})
}

// handleUpdateGlobalConfig updates global configuration values.
func (s *Server) handleUpdateGlobalConfig(w http.ResponseWriter, r *http.Request) {
	// Use write lock to prevent concurrent modifications
	s.configMu.Lock()
	defer s.configMu.Unlock()

	var req FullConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	globalPath, err := config.EnsureGlobalConfigFile()
	if err != nil {
		s.logger.Error("failed to ensure global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	// Check for force update flag
	forceUpdate := r.URL.Query().Get("force") == "true"

	// Check If-Match header for ETag validation (unless forcing)
	clientETag := r.Header.Get("If-Match")
	if !forceUpdate && clientETag != "" {
		clientETag = strings.Trim(clientETag, `"`)

		matches, currentETag, err := ETagMatch(clientETag, globalPath)
		if err != nil {
			s.logger.Error("failed to check ETag", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to check configuration version")
			return
		}

		if !matches {
			// Conflict detected!
			cfg, loadErr := config.NewLoader().WithConfigFile(globalPath).WithResolvePaths(false).Load()
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

	// Load current global config
	cfg, err := config.NewLoader().WithConfigFile(globalPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	// Apply updates
	applyFullConfigUpdates(cfg, &req)

	// Validate merged config before saving
	if !validateConfig(w, cfg, s.logger) {
		return
	}

	// Save with atomic write
	if err := atomicWriteConfig(cfg, globalPath); err != nil {
		s.logger.Error("failed to save global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}

	// New ETag from file bytes
	newETag, _ := calculateETagFromFile(globalPath)
	w.Header().Set("ETag", fmt.Sprintf("%q", newETag))

	respondJSON(w, http.StatusOK, ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:   newETag,
			Source: "file",
			Scope:  "global",
		},
	})
}

// handleResetGlobalConfig resets the global configuration to defaults.
func (s *Server) handleResetGlobalConfig(w http.ResponseWriter, _ *http.Request) {
	// Use write lock to prevent concurrent modifications
	s.configMu.Lock()
	defer s.configMu.Unlock()

	globalPath, err := config.GlobalConfigPath()
	if err != nil {
		s.logger.Error("failed to resolve global config path", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to resolve global configuration path")
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o750); err != nil {
		s.logger.Error("failed to create global config directory", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create configuration directory")
		return
	}

	if err := os.WriteFile(globalPath, []byte(config.DefaultConfigYAML), 0o600); err != nil {
		s.logger.Error("failed to write default global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to write default configuration")
		return
	}

	cfg, err := config.NewLoader().WithConfigFile(globalPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load default global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load default configuration")
		return
	}

	etag, _ := calculateETagFromFile(globalPath)
	if etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
	}

	respondJSON(w, http.StatusOK, ConfigResponseWithMeta{
		Config: configToFullResponse(cfg),
		Meta: ConfigMeta{
			ETag:   etag,
			Source: "file",
			Scope:  "global",
		},
	})
}

// handleGetAgents returns available agents and their status.
// Models are synced with internal/adapters/cli/*.go SupportedModels.
// Reasoning efforts synced with internal/core/constants.go.
func (s *Server) handleGetAgents(w http.ResponseWriter, _ *http.Request) {
	// Codex reasoning efforts
	codexReasoningEfforts := core.CodexReasoningEfforts

	agents := []map[string]interface{}{
		{
			"name":        "claude",
			"displayName": "Claude",
			// Synced with internal/adapters/cli/claude.go
			// Aliases (recommended): opus, sonnet, haiku
			// Full names for explicit version control
			"models": []string{
				// Aliases (map to latest version)
				"opus",
				"sonnet",
				"haiku",
				// Claude 4.6 (latest opus)
				"claude-opus-4-6",
				// Claude 4.5 family
				"claude-sonnet-4-5-20250929",
				"claude-haiku-4-5-20251001",
				// Claude 4 family
				"claude-opus-4-20250514",
				"claude-opus-4-1-20250805",
				"claude-sonnet-4-20250514",
			},
			"hasReasoningEffort": true,
			"reasoningEfforts":   []string{"low", "medium", "high", "max"},
			"available":          true,
		},
		{
			"name":        "gemini",
			"displayName": "Gemini",
			// Synced with internal/adapters/cli/gemini.go
			"models": []string{
				// Gemini 2.5 family (stable, recommended)
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-2.5-flash-lite",
				// Gemini 2.0 family (retiring March 2026)
				"gemini-2.0-flash",
				"gemini-2.0-flash-lite",
				// Gemini 3 preview
				"gemini-3-pro-preview",
				"gemini-3-flash-preview",
			},
			"available": true,
		},
		{
			"name":        "codex",
			"displayName": "Codex",
			// Synced with internal/adapters/cli/codex.go
			// Note: o3, o4-mini, gpt-4.1, gpt-5-mini require API key (not ChatGPT account)
			"models": []string{
				// GPT-5.3 family (latest)
				"gpt-5.3-codex",
				// GPT-5.2 family
				"gpt-5.2-codex",
				"gpt-5.2",
				// GPT-5.1 family
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1-codex-mini",
				"gpt-5.1",
				// GPT-5 family
				"gpt-5",
				// Reasoning models (require API key)
				"o3",
				"o4-mini",
				// Legacy (require API key)
				"gpt-5-mini",
				"gpt-4.1",
			},
			"hasReasoningEffort": true,
			"reasoningEfforts":   codexReasoningEfforts,
			"available":          true,
		},
		{
			"name":        "copilot",
			"displayName": "Copilot",
			// Synced with internal/adapters/cli/copilot.go
			// Copilot supports multiple providers via GitHub subscription
			// Note: Copilot CLI has no reasoning effort flag/env var/config
			"models": []string{
				// Anthropic Claude (via Copilot)
				"claude-sonnet-4.5",
				"claude-opus-4.6",
				"claude-haiku-4.5",
				"claude-sonnet-4",
				// OpenAI GPT (via Copilot)
				"gpt-5.2-codex",
				"gpt-5.2",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1",
				"gpt-5",
				"gpt-5.1-codex-mini",
				"gpt-5-mini",
				"gpt-4.1",
				// Google Gemini (via Copilot)
				"gemini-3-pro-preview",
			},
			"available": true,
		},
		{
			"name":        "opencode",
			"displayName": "OpenCode",
			// Synced with internal/adapters/cli/opencode.go
			// Requires local Ollama server with these models installed
			"models": []string{
				// Local Ollama models (from `ollama list`)
				"qwen2.5-coder:32b", // Best local coding model
				"qwen3-coder:30b",   // Latest Qwen coder
				"deepseek-r1:32b",   // Reasoning model
				"codestral:22b",     // Mistral code model
				"gpt-oss:20b",       // Open source GPT
			},
			"available": true,
		},
	}

	respondJSON(w, http.StatusOK, agents)
}

// loadConfigForContext loads the configuration using the project-scoped config loader.
func (s *Server) loadConfigForContext(ctx context.Context) (*config.Config, error) {
	configPath, _, _, err := s.effectiveConfigPath(ctx)
	if err != nil {
		return nil, err
	}
	// If the config file exists on disk, load it directly (project-scoped config).
	if _, statErr := os.Stat(configPath); statErr == nil {
		return config.NewLoader().WithConfigFile(configPath).WithResolvePaths(false).Load()
	}
	// File doesn't exist on disk — fall back to the server/project config loader
	// which may have programmatic defaults or Viper overrides (e.g. test setups).
	if loader := s.getProjectConfigLoader(ctx); loader != nil {
		return loader.Load()
	}
	// Last resort: new loader with the (missing) path, returns built-in defaults.
	return config.NewLoader().WithConfigFile(configPath).WithResolvePaths(false).Load()
}

// getProjectConfigPath returns the config file path for the current project context.
func (s *Server) getProjectConfigPath(ctx context.Context) string {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		return filepath.Join(pc.ProjectRoot(), ".quorum", "config.yaml")
	}
	// Fallback to server root (legacy mode)
	if s.root != "" {
		return filepath.Join(s.root, ".quorum", "config.yaml")
	}
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
			DenyTools:  denyTools,
			Heartbeat: HeartbeatConfigResponse{
				Enabled:        true, // Heartbeat is always active; field kept for API compat
				Interval:       cfg.Workflow.Heartbeat.Interval,
				StaleThreshold: cfg.Workflow.Heartbeat.StaleThreshold,
				CheckInterval:  cfg.Workflow.Heartbeat.CheckInterval,
				AutoResume:     cfg.Workflow.Heartbeat.AutoResume,
				MaxResumes:     cfg.Workflow.Heartbeat.MaxResumes,
			},
		},
		Phases: PhasesConfigResponse{
			Analyze: AnalyzePhaseConfigResponse{
				Timeout: cfg.Phases.Analyze.Timeout,
				Refiner: RefinerConfigResponse{
					Enabled:  cfg.Phases.Analyze.Refiner.Enabled,
					Agent:    cfg.Phases.Analyze.Refiner.Agent,
					Template: cfg.Phases.Analyze.Refiner.Template,
				},
				Moderator: ModeratorConfigResponse{
					Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
					Agent:               cfg.Phases.Analyze.Moderator.Agent,
					Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
					MinSuccessfulAgents: cfg.Phases.Analyze.Moderator.MinSuccessfulAgents,
					MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
					MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
					WarningThreshold:    cfg.Phases.Analyze.Moderator.WarningThreshold,
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
			Path:       cfg.State.Path,
			BackupPath: cfg.State.BackupPath,
			LockTTL:    cfg.State.LockTTL,
		},
		Git: GitConfigResponse{
			Worktree: WorktreeConfigResponse{
				Dir:       cfg.Git.Worktree.Dir,
				Mode:      cfg.Git.Worktree.Mode,
				AutoClean: cfg.Git.Worktree.AutoClean,
			},
			Task: GitTaskConfigResponse{
				AutoCommit: cfg.Git.Task.AutoCommit,
			},
			Finalization: GitFinalizationConfigResponse{
				AutoPush:      cfg.Git.Finalization.AutoPush,
				AutoPR:        cfg.Git.Finalization.AutoPR,
				AutoMerge:     cfg.Git.Finalization.AutoMerge,
				PRBaseBranch:  cfg.Git.Finalization.PRBaseBranch,
				MergeStrategy: cfg.Git.Finalization.MergeStrategy,
			},
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
		Issues: issuesToResponse(&cfg.Issues),
	}
}

// issuesToResponse converts IssuesConfig to IssuesConfigResponse.
func issuesToResponse(cfg *config.IssuesConfig) IssuesConfigResponse {
	labels := cfg.Labels
	if labels == nil {
		labels = []string{}
	}
	assignees := cfg.Assignees
	if assignees == nil {
		assignees = []string{}
	}

	return IssuesConfigResponse{
		Enabled:        cfg.Enabled,
		Provider:       cfg.Provider,
		AutoGenerate:   cfg.AutoGenerate,
		Timeout:        cfg.Timeout,
		Mode:           cfg.Mode,
		DraftDirectory: cfg.DraftDirectory,
		Repository:     cfg.Repository,
		ParentPrompt:   cfg.ParentPrompt,
		Prompt: IssuePromptConfigResponse{
			Language:           cfg.Prompt.Language,
			Tone:               cfg.Prompt.Tone,
			IncludeDiagrams:    cfg.Prompt.IncludeDiagrams,
			TitleFormat:        cfg.Prompt.TitleFormat,
			BodyPromptFile:     cfg.Prompt.BodyPromptFile,
			Convention:         cfg.Prompt.Convention,
			CustomInstructions: cfg.Prompt.CustomInstructions,
		},
		Labels:    labels,
		Assignees: assignees,
		GitLab: GitLabIssueConfigResponse{
			UseEpics:  cfg.GitLab.UseEpics,
			ProjectID: cfg.GitLab.ProjectID,
		},
		Generator: IssueGeneratorConfigResponse{
			Enabled:           cfg.Generator.Enabled,
			Agent:             cfg.Generator.Agent,
			Model:             cfg.Generator.Model,
			Summarize:         cfg.Generator.Summarize,
			MaxBodyLength:     cfg.Generator.MaxBodyLength,
			ReasoningEffort:   cfg.Generator.ReasoningEffort,
			Instructions:      cfg.Generator.Instructions,
			TitleInstructions: cfg.Generator.TitleInstructions,
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
	if req.Issues != nil {
		applyIssuesUpdates(&cfg.Issues, req.Issues)
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
	if update.DenyTools != nil {
		cfg.DenyTools = *update.DenyTools
	}
	if update.Heartbeat != nil {
		applyHeartbeatUpdates(&cfg.Heartbeat, update.Heartbeat)
	}
}

func applyHeartbeatUpdates(cfg *config.HeartbeatConfig, update *HeartbeatConfigUpdate) {
	// Enabled is intentionally ignored — heartbeat is always active.
	if update.Interval != nil {
		cfg.Interval = *update.Interval
	}
	if update.StaleThreshold != nil {
		cfg.StaleThreshold = *update.StaleThreshold
	}
	if update.CheckInterval != nil {
		cfg.CheckInterval = *update.CheckInterval
	}
	if update.AutoResume != nil {
		cfg.AutoResume = *update.AutoResume
	}
	if update.MaxResumes != nil {
		cfg.MaxResumes = *update.MaxResumes
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
		if update.Refiner.Template != nil {
			cfg.Refiner.Template = *update.Refiner.Template
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
		if update.Moderator.MinSuccessfulAgents != nil {
			cfg.Moderator.MinSuccessfulAgents = *update.Moderator.MinSuccessfulAgents
		}
		if update.Moderator.MinRounds != nil {
			cfg.Moderator.MinRounds = *update.Moderator.MinRounds
		}
		if update.Moderator.MaxRounds != nil {
			cfg.Moderator.MaxRounds = *update.Moderator.MaxRounds
		}
		if update.Moderator.WarningThreshold != nil {
			cfg.Moderator.WarningThreshold = *update.Moderator.WarningThreshold
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
	// Worktree updates
	if update.Worktree != nil {
		if update.Worktree.Dir != nil {
			cfg.Worktree.Dir = *update.Worktree.Dir
		}
		if update.Worktree.Mode != nil {
			cfg.Worktree.Mode = *update.Worktree.Mode
		}
		if update.Worktree.AutoClean != nil {
			cfg.Worktree.AutoClean = *update.Worktree.AutoClean
		}
	}
	// Task updates
	if update.Task != nil {
		if update.Task.AutoCommit != nil {
			cfg.Task.AutoCommit = *update.Task.AutoCommit
		}
	}
	// Finalization updates
	if update.Finalization != nil {
		if update.Finalization.AutoPush != nil {
			cfg.Finalization.AutoPush = *update.Finalization.AutoPush
		}
		if update.Finalization.AutoPR != nil {
			cfg.Finalization.AutoPR = *update.Finalization.AutoPR
		}
		if update.Finalization.AutoMerge != nil {
			cfg.Finalization.AutoMerge = *update.Finalization.AutoMerge
		}
		if update.Finalization.PRBaseBranch != nil {
			cfg.Finalization.PRBaseBranch = *update.Finalization.PRBaseBranch
		}
		if update.Finalization.MergeStrategy != nil {
			cfg.Finalization.MergeStrategy = *update.Finalization.MergeStrategy
		}
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

func applyIssuesUpdates(cfg *config.IssuesConfig, update *IssuesConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.Provider != nil {
		cfg.Provider = *update.Provider
	}
	if update.AutoGenerate != nil {
		cfg.AutoGenerate = *update.AutoGenerate
	}
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.Mode != nil {
		cfg.Mode = *update.Mode
	}
	if update.DraftDirectory != nil {
		cfg.DraftDirectory = *update.DraftDirectory
	}
	if update.Repository != nil {
		cfg.Repository = *update.Repository
	}
	if update.ParentPrompt != nil {
		cfg.ParentPrompt = *update.ParentPrompt
	}
	if update.Labels != nil {
		cfg.Labels = *update.Labels
	}
	if update.Assignees != nil {
		cfg.Assignees = *update.Assignees
	}
	if update.Prompt != nil {
		applyIssuePromptUpdates(&cfg.Prompt, update.Prompt)
	}
	if update.GitLab != nil {
		applyGitLabIssueUpdates(&cfg.GitLab, update.GitLab)
	}
	if update.Generator != nil {
		applyIssueGeneratorUpdates(&cfg.Generator, update.Generator)
	}
}

func applyIssuePromptUpdates(cfg *config.IssuePromptConfig, update *IssuePromptConfigUpdate) {
	if update.Language != nil {
		cfg.Language = *update.Language
	}
	if update.Tone != nil {
		cfg.Tone = *update.Tone
	}
	if update.IncludeDiagrams != nil {
		cfg.IncludeDiagrams = *update.IncludeDiagrams
	}
	if update.TitleFormat != nil {
		cfg.TitleFormat = *update.TitleFormat
	}
	if update.BodyPromptFile != nil {
		cfg.BodyPromptFile = *update.BodyPromptFile
	}
	if update.Convention != nil {
		cfg.Convention = *update.Convention
	}
	if update.CustomInstructions != nil {
		cfg.CustomInstructions = *update.CustomInstructions
	}
}

func applyGitLabIssueUpdates(cfg *config.GitLabIssueConfig, update *GitLabIssueConfigUpdate) {
	if update.UseEpics != nil {
		cfg.UseEpics = *update.UseEpics
	}
	if update.ProjectID != nil {
		cfg.ProjectID = *update.ProjectID
	}
}

func applyIssueGeneratorUpdates(cfg *config.IssueGeneratorConfig, update *IssueGeneratorConfigUpdate) {
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.Agent != nil {
		cfg.Agent = *update.Agent
	}
	if update.Model != nil {
		cfg.Model = *update.Model
	}
	if update.Summarize != nil {
		cfg.Summarize = *update.Summarize
	}
	if update.MaxBodyLength != nil {
		cfg.MaxBodyLength = *update.MaxBodyLength
	}
	if update.ReasoningEffort != nil {
		cfg.ReasoningEffort = *update.ReasoningEffort
	}
	if update.Instructions != nil {
		cfg.Instructions = *update.Instructions
	}
	if update.TitleInstructions != nil {
		cfg.TitleInstructions = *update.TitleInstructions
	}
}
