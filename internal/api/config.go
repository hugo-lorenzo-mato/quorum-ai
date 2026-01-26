package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigResponse represents the configuration response.
type ConfigResponse struct {
	Workflow WorkflowConfigResponse `json:"workflow"`
	Agents   AgentsConfigResponse   `json:"agents"`
	Git      GitConfigResponse      `json:"git"`
	Log      LogConfigResponse      `json:"log"`
}

// WorkflowConfigResponse represents workflow configuration.
type WorkflowConfigResponse struct {
	Timeout   string   `json:"timeout"`
	Sandbox   bool     `json:"sandbox"`
	DenyTools []string `json:"deny_tools"`
}

// AgentsConfigResponse represents agent configuration.
type AgentsConfigResponse struct {
	Default string              `json:"default"`
	Claude  AgentConfigResponse `json:"claude"`
	Gemini  AgentConfigResponse `json:"gemini"`
	Codex   AgentConfigResponse `json:"codex"`
}

// AgentConfigResponse represents individual agent configuration.
type AgentConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model"`
	Path    string `json:"path"`
}

// GitConfigResponse represents git configuration.
type GitConfigResponse struct {
	AutoCommit   bool   `json:"auto_commit"`
	AutoPush     bool   `json:"auto_push"`
	WorktreeMode string `json:"worktree_mode"`
}

// LogConfigResponse represents logging configuration.
type LogConfigResponse struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// ConfigUpdateRequest represents a config update request.
type ConfigUpdateRequest struct {
	Workflow *WorkflowConfigUpdate `json:"workflow,omitempty"`
	Agents   *AgentsConfigUpdate   `json:"agents,omitempty"`
	Git      *GitConfigUpdate      `json:"git,omitempty"`
	Log      *LogConfigUpdate      `json:"log,omitempty"`
}

// WorkflowConfigUpdate represents workflow configuration update.
type WorkflowConfigUpdate struct {
	Timeout   *string   `json:"timeout,omitempty"`
	Sandbox   *bool     `json:"sandbox,omitempty"`
	DenyTools *[]string `json:"deny_tools,omitempty"`
}

// AgentsConfigUpdate represents agents configuration update.
type AgentsConfigUpdate struct {
	Default *string            `json:"default,omitempty"`
	Claude  *AgentConfigUpdate `json:"claude,omitempty"`
	Gemini  *AgentConfigUpdate `json:"gemini,omitempty"`
	Codex   *AgentConfigUpdate `json:"codex,omitempty"`
}

// AgentConfigUpdate represents individual agent configuration update.
type AgentConfigUpdate struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Model   *string `json:"model,omitempty"`
}

// GitConfigUpdate represents git configuration update.
type GitConfigUpdate struct {
	AutoCommit   *bool   `json:"auto_commit,omitempty"`
	AutoPush     *bool   `json:"auto_push,omitempty"`
	WorktreeMode *string `json:"worktree_mode,omitempty"`
}

// LogConfigUpdate represents logging configuration update.
type LogConfigUpdate struct {
	Level  *string `json:"level,omitempty"`
	Format *string `json:"format,omitempty"`
}

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := s.loadConfig()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	response := s.configToResponse(config)
	respondJSON(w, http.StatusOK, response)
}

// handleUpdateConfig updates configuration values.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load current config
	config, err := s.loadConfig()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Apply updates
	s.applyConfigUpdates(config, &req)

	// Save updated config
	if err := s.saveConfig(config); err != nil {
		s.logger.Error("failed to save config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}

	response := s.configToResponse(config)
	respondJSON(w, http.StatusOK, response)
}

// handleGetAgents returns available agents and their status.
func (s *Server) handleGetAgents(w http.ResponseWriter, _ *http.Request) {
	agents := []map[string]interface{}{
		{
			"name":        "claude",
			"displayName": "Claude",
			"models":      []string{"claude-opus-4-5-20251101", "claude-sonnet-4-5-20250514", "claude-3-5-sonnet-latest"},
			"available":   true,
		},
		{
			"name":        "gemini",
			"displayName": "Gemini",
			"models":      []string{"gemini-2.0-flash-thinking-exp", "gemini-2.0-flash-exp", "gemini-1.5-pro"},
			"available":   true,
		},
		{
			"name":        "codex",
			"displayName": "Codex",
			"models":      []string{"gpt-5.2-codex", "gpt-5.1-codex", "gpt-4.1"},
			"available":   true,
		},
	}

	respondJSON(w, http.StatusOK, agents)
}

// loadConfig loads the configuration from file.
func (s *Server) loadConfig() (map[string]interface{}, error) {
	configPath := s.getConfigPath()

	// #nosec G304 -- config path is within application-controlled directory
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.getDefaultConfig(), nil
		}
		return nil, err
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// saveConfig saves the configuration to file.
func (s *Server) saveConfig(config map[string]interface{}) error {
	configPath := s.getConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0o600)
}

// getConfigPath returns the config file path.
func (s *Server) getConfigPath() string {
	return filepath.Join(".quorum", "config.yaml")
}

// getDefaultConfig returns default configuration.
func (s *Server) getDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"workflow": map[string]interface{}{
			"timeout":    "1h",
			"sandbox":    false,
			"deny_tools": []string{},
		},
		"agents": map[string]interface{}{
			"default": "claude",
			"claude": map[string]interface{}{
				"enabled": true,
				"model":   "claude-opus-4-5-20251101",
				"path":    "claude",
			},
			"gemini": map[string]interface{}{
				"enabled": true,
				"model":   "gemini-2.0-flash-thinking-exp",
				"path":    "gemini",
			},
			"codex": map[string]interface{}{
				"enabled": true,
				"model":   "gpt-5.2-codex",
				"path":    "codex",
			},
		},
		"git": map[string]interface{}{
			"auto_commit":   false,
			"auto_push":     false,
			"worktree_mode": "per-task",
		},
		"log": map[string]interface{}{
			"level":  "info",
			"format": "auto",
		},
	}
}

// configToResponse converts config map to response structure.
func (s *Server) configToResponse(config map[string]interface{}) ConfigResponse {
	response := ConfigResponse{
		Workflow: WorkflowConfigResponse{
			Timeout:   "1h",
			Sandbox:   false,
			DenyTools: []string{},
		},
		Agents: AgentsConfigResponse{
			Default: "claude",
			Claude:  AgentConfigResponse{Enabled: true, Model: "claude-opus-4-5-20251101", Path: "claude"},
			Gemini:  AgentConfigResponse{Enabled: true, Model: "gemini-2.0-flash-thinking-exp", Path: "gemini"},
			Codex:   AgentConfigResponse{Enabled: true, Model: "gpt-5.2-codex", Path: "codex"},
		},
		Git: GitConfigResponse{
			AutoCommit:   false,
			AutoPush:     false,
			WorktreeMode: "per-task",
		},
		Log: LogConfigResponse{
			Level:  "info",
			Format: "auto",
		},
	}

	// Extract values from config map
	if workflow, ok := config["workflow"].(map[string]interface{}); ok {
		if timeout, ok := workflow["timeout"].(string); ok {
			response.Workflow.Timeout = timeout
		}
		if sandbox, ok := workflow["sandbox"].(bool); ok {
			response.Workflow.Sandbox = sandbox
		}
	}

	if agents, ok := config["agents"].(map[string]interface{}); ok {
		if def, ok := agents["default"].(string); ok {
			response.Agents.Default = def
		}
		if claude, ok := agents["claude"].(map[string]interface{}); ok {
			if enabled, ok := claude["enabled"].(bool); ok {
				response.Agents.Claude.Enabled = enabled
			}
			if model, ok := claude["model"].(string); ok {
				response.Agents.Claude.Model = model
			}
		}
		if gemini, ok := agents["gemini"].(map[string]interface{}); ok {
			if enabled, ok := gemini["enabled"].(bool); ok {
				response.Agents.Gemini.Enabled = enabled
			}
			if model, ok := gemini["model"].(string); ok {
				response.Agents.Gemini.Model = model
			}
		}
		if codex, ok := agents["codex"].(map[string]interface{}); ok {
			if enabled, ok := codex["enabled"].(bool); ok {
				response.Agents.Codex.Enabled = enabled
			}
			if model, ok := codex["model"].(string); ok {
				response.Agents.Codex.Model = model
			}
		}
	}

	if git, ok := config["git"].(map[string]interface{}); ok {
		if autoCommit, ok := git["auto_commit"].(bool); ok {
			response.Git.AutoCommit = autoCommit
		}
		if autoPush, ok := git["auto_push"].(bool); ok {
			response.Git.AutoPush = autoPush
		}
		if mode, ok := git["worktree_mode"].(string); ok {
			response.Git.WorktreeMode = mode
		}
	}

	if log, ok := config["log"].(map[string]interface{}); ok {
		if level, ok := log["level"].(string); ok {
			response.Log.Level = level
		}
		if format, ok := log["format"].(string); ok {
			response.Log.Format = format
		}
	}

	return response
}

// applyConfigUpdates applies update request to config.
func (s *Server) applyConfigUpdates(config map[string]interface{}, req *ConfigUpdateRequest) {
	if req.Workflow != nil {
		workflow, ok := config["workflow"].(map[string]interface{})
		if !ok {
			workflow = make(map[string]interface{})
			config["workflow"] = workflow
		}
		if req.Workflow.Timeout != nil {
			workflow["timeout"] = *req.Workflow.Timeout
		}
		if req.Workflow.Sandbox != nil {
			workflow["sandbox"] = *req.Workflow.Sandbox
		}
		if req.Workflow.DenyTools != nil {
			workflow["deny_tools"] = *req.Workflow.DenyTools
		}
	}

	if req.Agents != nil {
		agents, ok := config["agents"].(map[string]interface{})
		if !ok {
			agents = make(map[string]interface{})
			config["agents"] = agents
		}
		if req.Agents.Default != nil {
			agents["default"] = *req.Agents.Default
		}
		s.applyAgentUpdate(agents, "claude", req.Agents.Claude)
		s.applyAgentUpdate(agents, "gemini", req.Agents.Gemini)
		s.applyAgentUpdate(agents, "codex", req.Agents.Codex)
	}

	if req.Git != nil {
		git, ok := config["git"].(map[string]interface{})
		if !ok {
			git = make(map[string]interface{})
			config["git"] = git
		}
		if req.Git.AutoCommit != nil {
			git["auto_commit"] = *req.Git.AutoCommit
		}
		if req.Git.AutoPush != nil {
			git["auto_push"] = *req.Git.AutoPush
		}
		if req.Git.WorktreeMode != nil {
			git["worktree_mode"] = *req.Git.WorktreeMode
		}
	}

	if req.Log != nil {
		log, ok := config["log"].(map[string]interface{})
		if !ok {
			log = make(map[string]interface{})
			config["log"] = log
		}
		if req.Log.Level != nil {
			log["level"] = *req.Log.Level
		}
		if req.Log.Format != nil {
			log["format"] = *req.Log.Format
		}
	}
}

// applyAgentUpdate applies agent-specific update.
func (s *Server) applyAgentUpdate(agents map[string]interface{}, name string, update *AgentConfigUpdate) {
	if update == nil {
		return
	}

	agent, ok := agents[name].(map[string]interface{})
	if !ok {
		agent = make(map[string]interface{})
		agents[name] = agent
	}

	if update.Enabled != nil {
		agent["enabled"] = *update.Enabled
	}
	if update.Model != nil {
		agent["model"] = *update.Model
	}
}
