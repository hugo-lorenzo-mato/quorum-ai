package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- AgentsConfig method tests ---

func TestAgentsConfig_GetAgentConfig(t *testing.T) {
	t.Parallel()

	agents := AgentsConfig{
		Claude:   AgentConfig{Enabled: true, Path: "claude"},
		Gemini:   AgentConfig{Enabled: true, Path: "gemini"},
		Codex:    AgentConfig{Enabled: true, Path: "codex"},
		Copilot:  AgentConfig{Enabled: true, Path: "copilot"},
		OpenCode: AgentConfig{Enabled: true, Path: "opencode"},
	}

	tests := []struct {
		name     string
		wantPath string
		wantNil  bool
	}{
		{"claude", "claude", false},
		{"gemini", "gemini", false},
		{"codex", "codex", false},
		{"copilot", "copilot", false},
		{"opencode", "opencode", false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := agents.GetAgentConfig(tt.name)
			if tt.wantNil {
				if cfg != nil {
					t.Errorf("GetAgentConfig(%q) should return nil", tt.name)
				}
			} else {
				if cfg == nil {
					t.Fatalf("GetAgentConfig(%q) returned nil", tt.name)
				}
				if cfg.Path != tt.wantPath {
					t.Errorf("GetAgentConfig(%q).Path = %q, want %q", tt.name, cfg.Path, tt.wantPath)
				}
			}
		})
	}
}

func TestAgentsConfig_EnabledAgentNames(t *testing.T) {
	t.Parallel()

	agents := AgentsConfig{
		Claude:   AgentConfig{Enabled: true},
		Gemini:   AgentConfig{Enabled: false},
		Codex:    AgentConfig{Enabled: true},
		Copilot:  AgentConfig{Enabled: false},
		OpenCode: AgentConfig{Enabled: true},
	}

	names := agents.EnabledAgentNames()

	// Should contain 3 enabled agents
	if len(names) != 3 {
		t.Errorf("EnabledAgentNames() returned %d names, want 3", len(names))
	}

	// Build a set from the names
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}

	for _, expected := range []string{"claude", "codex", "opencode"} {
		if !nameSet[expected] {
			t.Errorf("EnabledAgentNames() missing %q", expected)
		}
	}
}

func TestAgentsConfig_EnabledAgentNames_AllDisabled(t *testing.T) {
	t.Parallel()

	agents := AgentsConfig{}
	names := agents.EnabledAgentNames()
	if len(names) != 0 {
		t.Errorf("EnabledAgentNames() returned %d names, want 0", len(names))
	}
}

func TestAgentsConfig_ListEnabledForPhase(t *testing.T) {
	t.Parallel()

	agents := AgentsConfig{
		Claude: AgentConfig{
			Enabled: true,
			Phases:  map[string]bool{"analyze": true, "plan": true, "execute": true},
		},
		Gemini: AgentConfig{
			Enabled: true,
			Phases:  map[string]bool{"analyze": true, "execute": true},
		},
		Codex: AgentConfig{
			Enabled: true,
			Phases:  map[string]bool{"analyze": true, "plan": true},
		},
		Copilot: AgentConfig{
			Enabled: false,
			Phases:  map[string]bool{"analyze": true}, // disabled agent should not be included
		},
	}

	analyzeAgents := agents.ListEnabledForPhase("analyze")
	if len(analyzeAgents) != 3 {
		t.Errorf("ListEnabledForPhase('analyze') returned %d, want 3", len(analyzeAgents))
	}

	planAgents := agents.ListEnabledForPhase("plan")
	if len(planAgents) != 2 {
		t.Errorf("ListEnabledForPhase('plan') returned %d, want 2", len(planAgents))
	}

	moderateAgents := agents.ListEnabledForPhase("moderate")
	if len(moderateAgents) != 0 {
		t.Errorf("ListEnabledForPhase('moderate') returned %d, want 0", len(moderateAgents))
	}
}

// --- AgentConfig method tests ---

func TestAgentConfig_IsEnabledForPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   AgentConfig
		phase string
		want  bool
	}{
		{
			"enabled and phase present",
			AgentConfig{Enabled: true, Phases: map[string]bool{"analyze": true}},
			"analyze", true,
		},
		{
			"enabled but phase false",
			AgentConfig{Enabled: true, Phases: map[string]bool{"analyze": false}},
			"analyze", false,
		},
		{
			"enabled but phase not in map",
			AgentConfig{Enabled: true, Phases: map[string]bool{"plan": true}},
			"analyze", false,
		},
		{
			"disabled agent",
			AgentConfig{Enabled: false, Phases: map[string]bool{"analyze": true}},
			"analyze", false,
		},
		{
			"nil phases map",
			AgentConfig{Enabled: true},
			"analyze", false,
		},
		{
			"empty phases map",
			AgentConfig{Enabled: true, Phases: map[string]bool{}},
			"analyze", false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.IsEnabledForPhase(tt.phase)
			if got != tt.want {
				t.Errorf("IsEnabledForPhase(%q) = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestAgentConfig_GetModelForPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   AgentConfig
		phase string
		want  string
	}{
		{
			"phase model override",
			AgentConfig{Model: "default-model", PhaseModels: map[string]string{"analyze": "special-model"}},
			"analyze", "special-model",
		},
		{
			"no phase model, uses default",
			AgentConfig{Model: "default-model", PhaseModels: map[string]string{}},
			"analyze", "default-model",
		},
		{
			"nil phase models, uses default",
			AgentConfig{Model: "default-model"},
			"analyze", "default-model",
		},
		{
			"empty phase model value, uses default",
			AgentConfig{Model: "default-model", PhaseModels: map[string]string{"analyze": ""}},
			"analyze", "default-model",
		},
		{
			"phase model for different phase",
			AgentConfig{Model: "default-model", PhaseModels: map[string]string{"plan": "plan-model"}},
			"analyze", "default-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.GetModelForPhase(tt.phase)
			if got != tt.want {
				t.Errorf("GetModelForPhase(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestAgentConfig_GetReasoningEffortForPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   AgentConfig
		phase string
		want  string
	}{
		{
			"phase-specific override",
			AgentConfig{
				ReasoningEffort:       "medium",
				ReasoningEffortPhases: map[string]string{"analyze": "xhigh"},
			},
			"analyze", "xhigh",
		},
		{
			"no phase override, uses default",
			AgentConfig{
				ReasoningEffort:       "high",
				ReasoningEffortPhases: map[string]string{},
			},
			"analyze", "high",
		},
		{
			"nil phases map, uses default",
			AgentConfig{ReasoningEffort: "low"},
			"plan", "low",
		},
		{
			"empty phase value, uses default",
			AgentConfig{
				ReasoningEffort:       "medium",
				ReasoningEffortPhases: map[string]string{"analyze": ""},
			},
			"analyze", "medium",
		},
		{
			"no default, no phase",
			AgentConfig{},
			"analyze", "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.GetReasoningEffortForPhase(tt.phase)
			if got != tt.want {
				t.Errorf("GetReasoningEffortForPhase(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

// --- Config.ExtractAgentPhases tests ---

func TestConfig_ExtractAgentPhases(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Claude: AgentConfig{
				Enabled: true,
				Phases: map[string]bool{
					"analyze": true,
					"plan":    true,
					"execute": true,
				},
			},
			Gemini: AgentConfig{
				Enabled: true,
				Phases: map[string]bool{
					"analyze": true,
					"execute": false, // explicitly false
				},
			},
			Codex: AgentConfig{
				Enabled: false, // disabled - should not appear
				Phases:  map[string]bool{"analyze": true},
			},
		},
	}

	phases := cfg.ExtractAgentPhases()

	claudePhases := phases["claude"]
	if len(claudePhases) != 3 {
		t.Errorf("claude phases = %d, want 3", len(claudePhases))
	}

	geminiPhases := phases["gemini"]
	if len(geminiPhases) != 1 {
		t.Errorf("gemini phases = %d, want 1", len(geminiPhases))
	}

	if _, ok := phases["codex"]; ok {
		t.Error("codex should not be in phases (disabled)")
	}
}

// --- IssuesConfig.Validate tests ---

func TestIssuesConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     IssuesConfig
		wantErr bool
		errMsg  string
	}{
		{
			"valid github config",
			IssuesConfig{Enabled: true, Provider: "github"},
			false, "",
		},
		{
			"valid gitlab config",
			IssuesConfig{Enabled: true, Provider: "gitlab", GitLab: GitLabIssueConfig{ProjectID: "123"}},
			false, "",
		},
		{
			"disabled config always valid",
			IssuesConfig{Enabled: false, Provider: "invalid"},
			false, "",
		},
		{
			"invalid provider",
			IssuesConfig{Enabled: true, Provider: "jira"},
			true, "issues.provider",
		},
		{
			"invalid tone",
			IssuesConfig{Enabled: true, Prompt: IssuePromptConfig{Tone: "angry"}},
			true, "issues.prompt.tone",
		},
		{
			"invalid language",
			IssuesConfig{Enabled: true, Prompt: IssuePromptConfig{Language: "klingon"}},
			true, "issues.prompt.language",
		},
		{
			"gitlab missing project_id",
			IssuesConfig{Enabled: true, Provider: "gitlab"},
			true, "issues.gitlab.project_id",
		},
		{
			"valid language alias",
			IssuesConfig{Enabled: true, Prompt: IssuePromptConfig{Language: "en"}},
			false, "",
		},
		{
			"empty provider enabled",
			IssuesConfig{Enabled: true, Provider: ""},
			false, "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Validate() error = nil, want error")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			}
		})
	}
}

// --- Loader additional tests ---

func TestLoader_WithProjectDir(t *testing.T) {
	t.Parallel()

	loader := NewLoader().WithProjectDir("/custom/project")
	_, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loader.ProjectDir() != "/custom/project" {
		t.Errorf("ProjectDir() = %q, want %q", loader.ProjectDir(), "/custom/project")
	}
}

func TestLoader_WithResolvePaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configContent := `
state:
  path: relative/state.db
  backup_path: relative/state.db.bak
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// With resolve disabled
	loader := NewLoader().WithConfigFile(configPath).WithResolvePaths(false)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.State.Path != "relative/state.db" {
		t.Errorf("State.Path = %q, want %q (relative preserved)", cfg.State.Path, "relative/state.db")
	}
}

func TestLoader_WithResolvePaths_Enabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configContent := `
state:
  path: relative/state.db
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath).WithResolvePaths(true)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !filepath.IsAbs(cfg.State.Path) {
		t.Errorf("State.Path = %q, want absolute path", cfg.State.Path)
	}
}

func TestLoader_GetSetIsSet(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	_, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Get a default value
	val := loader.Get("log.level")
	if val == nil {
		t.Error("Get('log.level') returned nil")
	}

	// Set a value
	loader.Set("custom.key", "custom-value")
	if !loader.IsSet("custom.key") {
		t.Error("IsSet('custom.key') = false after Set()")
	}
	if loader.Get("custom.key") != "custom-value" {
		t.Errorf("Get('custom.key') = %v, want 'custom-value'", loader.Get("custom.key"))
	}
}

func TestLoader_AllSettings(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	_, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	settings := loader.AllSettings()
	if len(settings) == 0 {
		t.Error("AllSettings() returned empty map")
	}

	// Should contain top-level keys
	expectedKeys := []string{"log", "trace", "workflow", "agents", "state", "git"}
	for _, key := range expectedKeys {
		if _, ok := settings[key]; !ok {
			t.Errorf("AllSettings() missing key %q", key)
		}
	}
}

func TestLoader_NewLoaderWithViper(t *testing.T) {
	t.Parallel()

	loader := NewLoaderWithViper(nil)
	if loader == nil {
		t.Fatal("NewLoaderWithViper(nil) returned nil")
	}
}

func TestLoader_Viper(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	v := loader.Viper()
	if v == nil {
		t.Error("Viper() returned nil")
	}
}

func TestLoader_ConfigFileNoFileSet(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	_, _ = loader.Load()

	// Without explicit config file, ConfigFile() returns viper's config file used
	configFile := loader.ConfigFile()
	// It may or may not have found one, but it should not panic
	_ = configFile
}

func TestLoader_NonExistentExplicitConfig(t *testing.T) {
	t.Parallel()

	// An explicit config file that doesn't exist should gracefully fall back to defaults
	loader := NewLoader().WithConfigFile("/nonexistent/path/config.yaml")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (should fall back to defaults)", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q (default)", cfg.Log.Level, "info")
	}
}

// --- resolvePathRelativeTo tests ---

func TestResolvePathRelativeTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		baseDir string
		want    string
	}{
		{"absolute unchanged", "/abs/path", "/base", "/abs/path"},
		{"relative resolved", "relative/path", "/base", "/base/relative/path"},
		{"dot prefix", "./relative", "/base", "/base/relative"},
		{"just filename", "file.db", "/base", "/base/file.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePathRelativeTo(tt.path, tt.baseDir)
			if got != tt.want {
				t.Errorf("resolvePathRelativeTo(%q, %q) = %q, want %q", tt.path, tt.baseDir, got, tt.want)
			}
		})
	}
}

// --- Validator additional coverage ---

func TestValidator_WorktreeAutoCleanWithoutAutoCommit(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Git.Worktree.AutoClean = true
	cfg.Git.Task.AutoCommit = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for auto_clean without auto_commit")
	}

	if !strings.Contains(err.Error(), "auto_clean") {
		t.Errorf("error = %v, should mention auto_clean", err)
	}
}

func TestValidator_GitFinalizationDependencies(t *testing.T) {
	t.Parallel()

	t.Run("auto_pr without auto_push", func(t *testing.T) {
		t.Parallel()
		cfg := validConfig()
		cfg.Git.Finalization.AutoPR = true
		cfg.Git.Finalization.AutoPush = false

		v := NewValidator()
		err := v.Validate(cfg)
		if err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "auto_pr") {
			t.Errorf("error = %v, should mention auto_pr", err)
		}
	})

	t.Run("auto_merge without auto_pr", func(t *testing.T) {
		t.Parallel()
		cfg := validConfig()
		cfg.Git.Finalization.AutoMerge = true
		cfg.Git.Finalization.AutoPR = false

		v := NewValidator()
		err := v.Validate(cfg)
		if err == nil {
			t.Fatal("Validate() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "auto_merge") {
			t.Errorf("error = %v, should mention auto_merge", err)
		}
	})
}

func TestValidator_InvalidWorktreeMode(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Git.Worktree.Mode = "turbo"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid worktree mode")
	}
	if !strings.Contains(err.Error(), "git.worktree.mode") {
		t.Errorf("error = %v, should mention git.worktree.mode", err)
	}
}

func TestValidator_ValidWorktreeModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"always", "parallel", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig()
			cfg.Git.Worktree.Mode = mode

			v := NewValidator()
			err := v.Validate(cfg)
			if err != nil {
				t.Errorf("Validate() error = %v for valid mode %q", err, mode)
			}
		})
	}
}

func TestValidator_InvalidMergeStrategy(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Git.Finalization.MergeStrategy = "yolo"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid merge strategy")
	}
	if !strings.Contains(err.Error(), "git.finalization.merge_strategy") {
		t.Errorf("error = %v, should mention merge_strategy", err)
	}
}

func TestValidator_ValidMergeStrategies(t *testing.T) {
	t.Parallel()

	for _, strategy := range []string{"merge", "squash", "rebase"} {
		t.Run(strategy, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig()
			cfg.Git.Finalization.MergeStrategy = strategy

			v := NewValidator()
			err := v.Validate(cfg)
			if err != nil {
				t.Errorf("Validate() error = %v for valid strategy %q", err, strategy)
			}
		})
	}
}

func TestValidator_TraceInvalidMode(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.Mode = "verbose"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.mode") {
		t.Errorf("error = %v, should mention trace.mode", err)
	}
}

func TestValidator_TraceInvalidSchemaVersion(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.SchemaVersion = 0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.schema_version") {
		t.Errorf("error = %v, should mention trace.schema_version", err)
	}
}

func TestValidator_TraceInvalidMaxBytes(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.MaxBytes = 0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.max_bytes") {
		t.Errorf("error = %v, should mention trace.max_bytes", err)
	}
}

func TestValidator_TraceTotalMaxBytesLessThanMaxBytes(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.MaxBytes = 1000
	cfg.Trace.TotalMaxBytes = 500

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.total_max_bytes") {
		t.Errorf("error = %v, should mention trace.total_max_bytes", err)
	}
}

func TestValidator_TraceInvalidMaxFiles(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.MaxFiles = 0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.max_files") {
		t.Errorf("error = %v, should mention trace.max_files", err)
	}
}

func TestValidator_TraceInvalidPhase(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.IncludePhases = []string{"analyze", "unknown-phase"}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.include_phases") {
		t.Errorf("error = %v, should mention trace.include_phases", err)
	}
}

func TestValidator_TraceEmptyDir(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Trace.Dir = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "trace.dir") {
		t.Errorf("error = %v, should mention trace.dir", err)
	}
}

func TestValidator_ReasoningEffortDefault(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Claude.ReasoningEffort = "ultra" // invalid

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "reasoning_effort") {
		t.Errorf("error = %v, should mention reasoning_effort", err)
	}
}

func TestValidator_ReasoningEffortPhases(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Claude.ReasoningEffortPhases = map[string]string{
		"analyze": "invalid-effort",
	}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "reasoning_effort") {
		t.Errorf("error = %v, should mention reasoning_effort", err)
	}
}

func TestValidator_ReasoningEffortPhasesInvalidKey(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Claude.ReasoningEffortPhases = map[string]string{
		"unknown_phase": "high",
	}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "reasoning_effort_phases") {
		t.Errorf("error = %v, should mention reasoning_effort_phases", err)
	}
}

func TestValidator_PhaseModelEmptyValue(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Claude.PhaseModels = map[string]string{
		"analyze": "  ", // whitespace only
	}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty model value")
	}
	if !strings.Contains(err.Error(), "phase_models") {
		t.Errorf("error = %v, should mention phase_models", err)
	}
}

func TestValidator_AgentPhasesAllFalse(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Claude.Phases = map[string]bool{
		"analyze": false,
		"plan":    false,
	}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for all-false phases")
	}
	if !strings.Contains(err.Error(), "phases") {
		t.Errorf("error = %v, should mention phases", err)
	}
}

func TestValidator_ModeratorWarningThresholdInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.WarningThreshold = -0.1
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 1
	cfg.Phases.Analyze.Moderator.MinRounds = 1
	cfg.Phases.Analyze.Moderator.MaxRounds = 3

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "warning_threshold") {
		t.Errorf("error = %v, should mention warning_threshold", err)
	}
}

func TestValidator_ModeratorStagnationThresholdInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.StagnationThreshold = 1.5
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 1
	cfg.Phases.Analyze.Moderator.MinRounds = 1
	cfg.Phases.Analyze.Moderator.MaxRounds = 3

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "stagnation_threshold") {
		t.Errorf("error = %v, should mention stagnation_threshold", err)
	}
}

func TestValidator_ModeratorMinSuccessfulAgents(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 0
	cfg.Phases.Analyze.Moderator.MinRounds = 1
	cfg.Phases.Analyze.Moderator.MaxRounds = 3

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "min_successful_agents") {
		t.Errorf("error = %v, should mention min_successful_agents", err)
	}
}

func TestValidator_ModeratorMinRoundsInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 1
	cfg.Phases.Analyze.Moderator.MinRounds = 0
	cfg.Phases.Analyze.Moderator.MaxRounds = 3

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "min_rounds") {
		t.Errorf("error = %v, should mention min_rounds", err)
	}
}

func TestValidator_ModeratorMaxRoundsLessThanMin(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 1
	cfg.Phases.Analyze.Moderator.MinRounds = 5
	cfg.Phases.Analyze.Moderator.MaxRounds = 2

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "max_rounds") {
		t.Errorf("error = %v, should mention max_rounds", err)
	}
}

func TestValidator_RefinerDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Refiner.Enabled = true
	cfg.Phases.Analyze.Refiner.Agent = "gemini"
	cfg.Agents.Gemini.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled refiner agent")
	}
	if !strings.Contains(err.Error(), "refiner.agent") {
		t.Errorf("error = %v, should mention refiner.agent", err)
	}
}

func TestValidator_RefinerUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Refiner.Enabled = true
	cfg.Phases.Analyze.Refiner.Agent = "unknown"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown refiner agent")
	}
	if !strings.Contains(err.Error(), "refiner.agent") {
		t.Errorf("error = %v, should mention refiner.agent", err)
	}
}

func TestValidator_ModeratorDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "gemini"
	cfg.Phases.Analyze.Moderator.Threshold = 0.80
	cfg.Phases.Analyze.Moderator.MinSuccessfulAgents = 1
	cfg.Phases.Analyze.Moderator.MinRounds = 1
	cfg.Phases.Analyze.Moderator.MaxRounds = 3
	cfg.Agents.Gemini.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled moderator agent")
	}
	if !strings.Contains(err.Error(), "moderator.agent") {
		t.Errorf("error = %v, should mention moderator.agent", err)
	}
}

func TestValidator_ModeratorUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "unknown"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown moderator agent")
	}
	if !strings.Contains(err.Error(), "moderator.agent") {
		t.Errorf("error = %v, should mention moderator.agent", err)
	}
}

func TestValidator_SynthesizerDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Synthesizer.Agent = "gemini"
	cfg.Agents.Gemini.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled synthesizer agent")
	}
	if !strings.Contains(err.Error(), "synthesizer") {
		t.Errorf("error = %v, should mention synthesizer", err)
	}
}

func TestValidator_SynthesizerUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Synthesizer.Agent = "unknown"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown synthesizer agent")
	}
	if !strings.Contains(err.Error(), "synthesizer") {
		t.Errorf("error = %v, should mention synthesizer", err)
	}
}

func TestValidator_PlanSynthesizerEnabled(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Plan.Synthesizer.Enabled = true
	cfg.Phases.Plan.Synthesizer.Agent = "claude"

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil for valid plan synthesizer", err)
	}
}

func TestValidator_PlanSynthesizerDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Plan.Synthesizer.Enabled = true
	cfg.Phases.Plan.Synthesizer.Agent = "gemini"
	cfg.Agents.Gemini.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled plan synthesizer agent")
	}
}

func TestValidator_PhaseTimeout_EmptyIsOK(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Timeout = ""
	cfg.Phases.Plan.Timeout = ""
	cfg.Phases.Execute.Timeout = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil (empty timeouts should be ok)", err)
	}
}

func TestValidator_PhaseTimeout_Invalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Phases.Analyze.Timeout = "not-a-duration"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid phase timeout")
	}
	if !strings.Contains(err.Error(), "phases.analyze.timeout") {
		t.Errorf("error = %v, should mention phases.analyze.timeout", err)
	}
}

func TestValidator_DefaultAgentDisabledValidation(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents.Default = "gemini"
	cfg.Agents.Gemini.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled default agent")
	}
	if !strings.Contains(err.Error(), "agents.default") {
		t.Errorf("error = %v, should mention agents.default", err)
	}
}

func TestValidator_Errors(t *testing.T) {
	t.Parallel()

	v := NewValidator()
	if v.Errors().HasErrors() {
		t.Error("new validator should have no errors")
	}

	cfg := validConfig()
	cfg.Log.Level = "invalid"
	v.Validate(cfg)

	if !v.Errors().HasErrors() {
		t.Error("validator should have errors after validation failure")
	}
}

// --- Loader path resolution tests ---

func TestLoader_PathResolution_QuorumDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create .quorum directory and config
	quorumDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(quorumDir, 0o750); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	configContent := `
state:
  path: .quorum/state/state.db
trace:
  dir: .quorum/traces
`
	configPath := filepath.Join(quorumDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// State path should be resolved relative to project dir (parent of .quorum)
	expectedStatePath := filepath.Join(tmpDir, ".quorum/state/state.db")
	if cfg.State.Path != expectedStatePath {
		t.Errorf("State.Path = %q, want %q", cfg.State.Path, expectedStatePath)
	}

	expectedTraceDir := filepath.Join(tmpDir, ".quorum/traces")
	if cfg.Trace.Dir != expectedTraceDir {
		t.Errorf("Trace.Dir = %q, want %q", cfg.Trace.Dir, expectedTraceDir)
	}
}

// --- loadNormalizedConfigMap tests ---

func TestLoadNormalizedConfigMap_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	result, err := loadNormalizedConfigMap(path)
	if err != nil {
		t.Fatalf("loadNormalizedConfigMap() error = %v", err)
	}
	if result != nil {
		t.Error("loadNormalizedConfigMap() should return nil for empty file")
	}
}

func TestLoadNormalizedConfigMap_EmptyYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(path, []byte("# only comments\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	result, err := loadNormalizedConfigMap(path)
	if err != nil {
		t.Fatalf("loadNormalizedConfigMap() error = %v", err)
	}
	if result != nil {
		t.Error("loadNormalizedConfigMap() should return nil for empty YAML")
	}
}

func TestLoadNormalizedConfigMap_ValidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(path, []byte("log:\n  level: info\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	result, err := loadNormalizedConfigMap(path)
	if err != nil {
		t.Fatalf("loadNormalizedConfigMap() error = %v", err)
	}
	if result == nil {
		t.Fatal("loadNormalizedConfigMap() returned nil")
	}
	if _, ok := result["log"]; !ok {
		t.Error("result should contain 'log' key")
	}
}

func TestLoadNormalizedConfigMap_InvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(path, []byte("invalid: [yaml"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	_, err := loadNormalizedConfigMap(path)
	if err == nil {
		t.Error("loadNormalizedConfigMap() should return error for invalid YAML")
	}
}

func TestLoadNormalizedConfigMap_NonExistent(t *testing.T) {
	t.Parallel()

	_, err := loadNormalizedConfigMap("/nonexistent/file.yaml")
	if err == nil {
		t.Error("loadNormalizedConfigMap() should return error for nonexistent file")
	}
}

// --- Legacy key normalization tests ---

func TestNormalizeLegacyConfigMap_NilInput(t *testing.T) {
	t.Parallel()

	result := normalizeLegacyConfigMap(nil)
	if result != nil {
		t.Error("normalizeLegacyConfigMap(nil) should return nil")
	}
}

func TestNormalizeLegacyConfigMap_EmptyMap(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{}
	result := normalizeLegacyConfigMap(data)
	if result == nil {
		t.Error("normalizeLegacyConfigMap should return non-nil for empty map")
	}
}

func TestApplyLegacyPathMappings_WorktreeDir(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{
			"worktree_dir": "/tmp/wt",
		},
	}
	applyLegacyPathMappings(data)

	git := data["git"].(map[string]interface{})
	worktree := git["worktree"].(map[string]interface{})
	if worktree["dir"] != "/tmp/wt" {
		t.Errorf("worktree.dir = %v, want /tmp/wt", worktree["dir"])
	}
	if _, ok := git["worktree_dir"]; ok {
		t.Error("legacy worktree_dir key should be deleted")
	}
}

func TestApplyLegacyPathMappings_WorktreeMode(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{
			"worktree_mode": "always",
		},
	}
	applyLegacyPathMappings(data)

	git := data["git"].(map[string]interface{})
	worktree := git["worktree"].(map[string]interface{})
	if worktree["mode"] != "always" {
		t.Errorf("worktree.mode = %v, want always", worktree["mode"])
	}
	if _, ok := git["worktree_mode"]; ok {
		t.Error("legacy worktree_mode key should be deleted")
	}
}

func TestApplyLegacyPathMappings_AutoClean(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{
			"auto_clean": true,
		},
	}
	applyLegacyPathMappings(data)

	git := data["git"].(map[string]interface{})
	worktree := git["worktree"].(map[string]interface{})
	if worktree["auto_clean"] != true {
		t.Errorf("worktree.auto_clean = %v, want true", worktree["auto_clean"])
	}
	if _, ok := git["auto_clean"]; ok {
		t.Error("legacy auto_clean key should be deleted")
	}
}

func TestApplyLegacyPathMappings_AutoCommit(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{
			"auto_commit": true,
		},
	}
	applyLegacyPathMappings(data)

	git := data["git"].(map[string]interface{})
	task := git["task"].(map[string]interface{})
	if task["auto_commit"] != true {
		t.Errorf("task.auto_commit = %v, want true", task["auto_commit"])
	}
	if _, ok := git["auto_commit"]; ok {
		t.Error("legacy auto_commit key should be deleted")
	}
}

func TestApplyLegacyPathMappings_IssueLanguageNormalization(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"issues": map[string]interface{}{
			"prompt": map[string]interface{}{
				"language": "en",
			},
		},
	}
	applyLegacyPathMappings(data)

	issues := data["issues"].(map[string]interface{})
	prompt := issues["prompt"].(map[string]interface{})
	if prompt["language"] != "english" {
		t.Errorf("language = %v, want english", prompt["language"])
	}
}

func TestApplyLegacyPathMappings_NoGit(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"log": map[string]interface{}{"level": "info"},
	}
	// Should not panic without git key
	applyLegacyPathMappings(data)
}

func TestApplyLegacyPathMappings_NoIssues(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{},
	}
	// Should not panic without issues key
	applyLegacyPathMappings(data)
}

func TestApplyLegacyPathMappings_ExistingWorktreeNotOverwritten(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"git": map[string]interface{}{
			"worktree_dir": "/legacy",
			"worktree": map[string]interface{}{
				"dir": "/new",
			},
		},
	}
	applyLegacyPathMappings(data)

	git := data["git"].(map[string]interface{})
	worktree := git["worktree"].(map[string]interface{})
	if worktree["dir"] != "/new" {
		t.Errorf("worktree.dir = %v, want /new (should not be overwritten)", worktree["dir"])
	}
}

func TestNormalizeMapForStruct_NilInput(t *testing.T) {
	t.Parallel()
	result := normalizeMapForStruct(nil, nil)
	if result != nil {
		t.Error("normalizeMapForStruct(nil, nil) should return nil")
	}
}

func TestNormalizeValueForType_SliceOfStructs(t *testing.T) {
	t.Parallel()
	// The IssuesConfig has nested struct types.
	// We test normalizeMapForStruct with a map that has underscore-free keys
	data := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "debug",
		},
	}
	result := normalizeLegacyConfigMap(data)
	if result == nil {
		t.Error("normalizeLegacyConfigMap should not return nil for valid input")
	}
}

func TestNormalizeIssueLanguage_Aliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"en", "english"},
		{"es", "spanish"},
		{"fr", "french"},
		{"de", "german"},
		{"pt", "portuguese"},
		{"pt-br", "portuguese"},
		{"pt_br", "portuguese"},
		{"zh", "chinese"},
		{"zh-cn", "chinese"},
		{"zh_cn", "chinese"},
		{"zh-tw", "chinese"},
		{"zh_tw", "chinese"},
		{"ja", "japanese"},
		{"jp", "japanese"},
		{"english", "english"}, // already normalized
		{"", ""},               // empty returns as-is
		{"ENGLISH", "english"}, // case insensitive
		{"EN", "english"},      // uppercase alias
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeIssueLanguage(tt.input)
			if got != tt.want {
				t.Errorf("normalizeIssueLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCanonicalTagName_MapstructureTag(t *testing.T) {
	t.Parallel()
	// Config struct fields use mapstructure tags
	// We test indirectly through normalizeLegacyConfigMap
	data := map[string]interface{}{
		"maxretries": 3,
	}
	result := normalizeLegacyConfigMap(data)
	// max_retries should not appear as a top-level key (it's nested under workflow)
	// but normalizeMapForStruct should handle the data without panicking
	if result == nil {
		t.Error("normalizeMapForStruct should handle unknown keys without nil")
	}
}

// --- Validate() function in loader.go (comprehensive) ---

func TestValidate_EmptyDefault(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty agents.default")
	}
	if !strings.Contains(err.Error(), "agents.default is required") {
		t.Errorf("error = %v, want 'agents.default is required'", err)
	}
}

func TestValidate_UnknownDefaultAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{Default: "unknown"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown default agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error = %v, want 'unknown agent'", err)
	}
}

func TestValidate_DisabledDefaultAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: false},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled default agent")
	}
	if !strings.Contains(err.Error(), "disabled agent") {
		t.Errorf("error = %v, want 'disabled agent'", err)
	}
}

func TestValidate_SynthesizerAgentRequired(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: ""},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing synthesizer agent")
	}
	if !strings.Contains(err.Error(), "synthesizer.agent is required") {
		t.Errorf("error = %v, want 'synthesizer.agent is required'", err)
	}
}

func TestValidate_SynthesizerUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "unknown"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown synthesizer agent")
	}
	if !strings.Contains(err.Error(), "synthesizer.agent references unknown") {
		t.Errorf("error = %v, want 'synthesizer.agent references unknown'", err)
	}
}

func TestValidate_SynthesizerDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: false},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "gemini"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled synthesizer agent")
	}
	if !strings.Contains(err.Error(), "synthesizer.agent references disabled") {
		t.Errorf("error = %v, want 'synthesizer.agent references disabled'", err)
	}
}

func TestValidate_SynthesizerNoModel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: true}, // no model
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "gemini"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for synthesizer agent without model")
	}
	if !strings.Contains(err.Error(), "no model configured") {
		t.Errorf("error = %v, want 'no model configured'", err)
	}
}

func TestValidate_RefinerEmptyAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner:     RefinerConfig{Enabled: true, Agent: ""},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty refiner agent")
	}
	if !strings.Contains(err.Error(), "refiner.agent is required") {
		t.Errorf("error = %v, want 'refiner.agent is required'", err)
	}
}

func TestValidate_RefinerUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner:     RefinerConfig{Enabled: true, Agent: "unknown"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown refiner agent")
	}
	if !strings.Contains(err.Error(), "refiner.agent references unknown") {
		t.Errorf("error = %v, want 'refiner.agent references unknown'", err)
	}
}

func TestValidate_RefinerDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: false},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner:     RefinerConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled refiner agent")
	}
	if !strings.Contains(err.Error(), "refiner.agent references disabled") {
		t.Errorf("error = %v, want 'refiner.agent references disabled'", err)
	}
}

func TestValidate_RefinerNoModel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: true}, // no model
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner:     RefinerConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for refiner agent without model")
	}
	if !strings.Contains(err.Error(), "no model configured for optimize phase") {
		t.Errorf("error = %v, want 'no model configured for optimize phase'", err)
	}
}

func TestValidate_ModeratorEmptyAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Moderator:   ModeratorConfig{Enabled: true, Agent: ""},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty moderator agent")
	}
	if !strings.Contains(err.Error(), "moderator.agent is required") {
		t.Errorf("error = %v, want 'moderator.agent is required'", err)
	}
}

func TestValidate_ModeratorUnknownAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Moderator:   ModeratorConfig{Enabled: true, Agent: "unknown"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown moderator agent")
	}
	if !strings.Contains(err.Error(), "moderator.agent references unknown") {
		t.Errorf("error = %v, want 'moderator.agent references unknown'", err)
	}
}

func TestValidate_ModeratorDisabledAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: false},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Moderator:   ModeratorConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled moderator agent")
	}
	if !strings.Contains(err.Error(), "moderator.agent references disabled") {
		t.Errorf("error = %v, want 'moderator.agent references disabled'", err)
	}
}

func TestValidate_ModeratorNoModel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: true}, // no model
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Moderator:   ModeratorConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for moderator agent without model")
	}
	if !strings.Contains(err.Error(), "no model configured for analyze phase") {
		t.Errorf("error = %v, want 'no model configured for analyze phase'", err)
	}
}

func TestValidate_SingleAgentAndModeratorMutualExclusion(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Moderator:   ModeratorConfig{Enabled: true, Agent: "claude"},
				SingleAgent: SingleAgentConfig{Enabled: true, Agent: "claude"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for mutual exclusion")
	}
	if !strings.Contains(err.Error(), "cannot both be true") {
		t.Errorf("error = %v, want 'cannot both be true'", err)
	}
}

func TestValidate_SingleAgentEmptyAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				SingleAgent: SingleAgentConfig{Enabled: true, Agent: ""},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty single_agent")
	}
	if !strings.Contains(err.Error(), "single_agent.agent is required") {
		t.Errorf("error = %v, want 'single_agent.agent is required'", err)
	}
}

func TestValidate_SingleAgentUnknown(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				SingleAgent: SingleAgentConfig{Enabled: true, Agent: "unknown"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown single_agent agent")
	}
	if !strings.Contains(err.Error(), "single_agent.agent references unknown") {
		t.Errorf("error = %v, want 'single_agent.agent references unknown'", err)
	}
}

func TestValidate_SingleAgentDisabled(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: false},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				SingleAgent: SingleAgentConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled single_agent agent")
	}
	if !strings.Contains(err.Error(), "single_agent.agent references disabled") {
		t.Errorf("error = %v, want 'single_agent.agent references disabled'", err)
	}
}

func TestValidate_SingleAgentNoModel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: true}, // no model
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				SingleAgent: SingleAgentConfig{Enabled: true, Agent: "gemini"},
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for single_agent agent without model")
	}
	if !strings.Contains(err.Error(), "no model configured for analyze phase") {
		t.Errorf("error = %v, want 'no model configured for analyze phase'", err)
	}
}

func TestValidate_PlanSynthesizerEmptyAgent(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
			Plan: PlanPhaseConfig{
				Synthesizer: PlanSynthesizerConfig{Enabled: true, Agent: ""},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty plan synthesizer agent")
	}
	if !strings.Contains(err.Error(), "plan.synthesizer.agent is required") {
		t.Errorf("error = %v, want 'plan.synthesizer.agent is required'", err)
	}
}

func TestValidate_PlanSynthesizerUnknown(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
			Plan: PlanPhaseConfig{
				Synthesizer: PlanSynthesizerConfig{Enabled: true, Agent: "unknown"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for unknown plan synthesizer agent")
	}
	if !strings.Contains(err.Error(), "plan.synthesizer.agent references unknown") {
		t.Errorf("error = %v, want 'plan.synthesizer.agent references unknown'", err)
	}
}

func TestValidate_PlanSynthesizerDisabled(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: false},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
			Plan: PlanPhaseConfig{
				Synthesizer: PlanSynthesizerConfig{Enabled: true, Agent: "gemini"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for disabled plan synthesizer agent")
	}
	if !strings.Contains(err.Error(), "plan.synthesizer.agent references disabled") {
		t.Errorf("error = %v, want 'plan.synthesizer.agent references disabled'", err)
	}
}

func TestValidate_PlanSynthesizerNoModel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test"},
			Gemini:  AgentConfig{Enabled: true}, // no model
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
			Plan: PlanPhaseConfig{
				Synthesizer: PlanSynthesizerConfig{Enabled: true, Agent: "gemini"},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for plan synthesizer agent without model")
	}
	if !strings.Contains(err.Error(), "no model configured for plan phase") {
		t.Errorf("error = %v, want 'no model configured for plan phase'", err)
	}
}

func TestValidate_ValidFullConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test-model"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
	}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil for valid config", err)
	}
}

func TestValidate_IssuesValidationPropagated(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude:  AgentConfig{Enabled: true, Model: "test-model"},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Synthesizer: SynthesizerConfig{Agent: "claude"},
			},
		},
		Issues: IssuesConfig{
			Enabled:  true,
			Provider: "jira", // invalid
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues provider")
	}
	if !strings.Contains(err.Error(), "issues.provider") {
		t.Errorf("error = %v, want 'issues.provider'", err)
	}
}

// --- AtomicWrite additional coverage ---

func TestAtomicWrite_NewFileInExistingDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "newfile.yaml")

	data := []byte("test data content")
	if err := AtomicWrite(path, data); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(read) != "test data content" {
		t.Errorf("content = %q, want %q", string(read), "test data content")
	}

	// New file should get default 0600 permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestAtomicWrite_CreatesNestedDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "deep", "file.yaml")

	data := []byte("nested file content")
	if err := AtomicWrite(path, data); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(read) != "nested file content" {
		t.Errorf("content = %q, want %q", string(read), "nested file content")
	}
}

func TestAtomicWrite_ExistingFilePreservesPermissions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "existing.yaml")

	// Create with 0644 permissions
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Atomic write should preserve 0644
	if err := AtomicWrite(path, []byte("new")); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("permissions = %o, want 0644", info.Mode().Perm())
	}

	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(read) != "new" {
		t.Errorf("content = %q, want %q", string(read), "new")
	}
}

func TestAtomicWrite_EmptyContent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yaml")

	if err := AtomicWrite(path, []byte("")); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("file size = %d, want 0", info.Size())
	}
}
