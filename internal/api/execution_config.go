package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// EffectiveExecutionConfig is the config snapshot used to build a runner for a workflow attempt.
// It is resolved deterministically from the active ProjectContext (global vs project config).
type EffectiveExecutionConfig struct {
	Config        *config.Config
	RawYAML       []byte
	ConfigPath    string
	ConfigScope   string // "global" | "project"
	ConfigMode    string // "inherit_global" | "custom"
	FileETag      string
	EffectiveETag string

	ProjectID   string
	ProjectRoot string
}

func resolveConfigMode(projectRoot, explicitMode string) (string, error) {
	mode := strings.TrimSpace(explicitMode)
	if mode == project.ConfigModeInheritGlobal || mode == project.ConfigModeCustom {
		return mode, nil
	}

	// Infer: custom if project config exists, otherwise inherit_global.
	projectConfigPath := filepath.Join(projectRoot, ".quorum", "config.yaml")
	if _, err := os.Stat(projectConfigPath); err == nil {
		return project.ConfigModeCustom, nil
	} else if os.IsNotExist(err) {
		return project.ConfigModeInheritGlobal, nil
	} else {
		return "", fmt.Errorf("checking project config file: %w", err)
	}
}

// ResolveEffectiveExecutionConfig resolves the config file to use for execution based on ProjectContext.
// It requires a ProjectContext to be present in ctx. This ensures every execution is tied to an
// explicit project, even when triggered from background systems (Kanban, heartbeat).
func ResolveEffectiveExecutionConfig(ctx context.Context) (*EffectiveExecutionConfig, error) {
	pc := middleware.GetProjectContext(ctx)
	if pc == nil {
		return nil, core.ErrValidation("PROJECT_CONTEXT_REQUIRED",
			"project context is required for execution; pass ?project=<id> or set a default project")
	}

	projectRoot := strings.TrimSpace(pc.ProjectRoot())
	if projectRoot == "" {
		return nil, core.ErrValidation("PROJECT_ROOT_REQUIRED", "project root is required for execution")
	}

	// Try to read explicit config_mode from the concrete project context if available.
	explicitMode := ""
	if concrete, ok := pc.(*project.ProjectContext); ok && concrete != nil {
		explicitMode = concrete.ConfigMode
	}

	mode, err := resolveConfigMode(projectRoot, explicitMode)
	if err != nil {
		return nil, err
	}

	scope := "project"
	configPath := filepath.Join(projectRoot, ".quorum", "config.yaml")
	if mode == project.ConfigModeInheritGlobal {
		globalPath, err := config.EnsureGlobalConfigFile()
		if err != nil {
			return nil, err
		}
		scope = "global"
		configPath = globalPath
	}

	// Enforce that the effective config file exists.
	if _, statErr := os.Stat(configPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, core.ErrValidation("CONFIG_FILE_MISSING",
				fmt.Sprintf("effective config file does not exist: %s (mode=%s)", configPath, mode))
		}
		return nil, fmt.Errorf("checking effective config file: %w", statErr)
	}

	raw, err := os.ReadFile(configPath) // #nosec G304 -- path from trusted config source
	if err != nil {
		return nil, fmt.Errorf("reading effective config file: %w", err)
	}

	fileETag, err := calculateETagFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("calculating config ETag: %w", err)
	}

	// IMPORTANT: Resolve relative paths relative to the project root. This makes global config
	// inheritance behave correctly (config file lives outside the project).
	loader := config.NewLoader().
		WithConfigFile(configPath).
		WithProjectDir(projectRoot)

	cfg, err := loader.Load()
	if err != nil {
		return nil, core.ErrValidation("CONFIG_LOAD_FAILED",
			fmt.Sprintf("failed to load effective config %s: %v", configPath, err)).WithCause(err)
	}
	if cfg == nil {
		return nil, core.ErrValidation("CONFIG_LOAD_FAILED",
			fmt.Sprintf("failed to load effective config %s: empty config", configPath))
	}

	// Fail fast on invalid config.
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, core.ErrValidation("INVALID_CONFIG",
			fmt.Sprintf("invalid effective config %s: %v", configPath, err)).
			WithCause(err).
			WithDetail("config_path", configPath).
			WithDetail("config_mode", mode).
			WithDetail("config_scope", scope)
	}

	effectiveETag, err := calculateETag(cfg)
	if err != nil {
		return nil, fmt.Errorf("calculating effective config ETag: %w", err)
	}

	return &EffectiveExecutionConfig{
		Config:        cfg,
		RawYAML:       raw,
		ConfigPath:    configPath,
		ConfigScope:   scope,
		ConfigMode:    mode,
		FileETag:      fileETag,
		EffectiveETag: effectiveETag,
		ProjectID:     pc.ProjectID(),
		ProjectRoot:   projectRoot,
	}, nil
}
