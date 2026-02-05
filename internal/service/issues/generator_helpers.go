package issues

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func (g *Generator) openIssuesLogger(workflowID string) (*slog.Logger, func() error, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("getting working directory: %w", err)
	}

	logDir := filepath.Join(cwd, ".quorum", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("issues-%s.log", workflowID))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("opening issues log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, file.Close, nil
}

func (g *Generator) buildLLMResilienceConfig(cfg config.LLMResilienceConfig, logger *slog.Logger) LLMResilienceConfig {
	if isZeroResilienceConfig(cfg) {
		return DefaultLLMResilienceConfig()
	}

	res := DefaultLLMResilienceConfig()
	res.Enabled = cfg.Enabled

	if cfg.MaxRetries != 0 || cfg.Enabled {
		res.MaxRetries = cfg.MaxRetries
	}
	if cfg.InitialBackoff != "" {
		if d, err := time.ParseDuration(cfg.InitialBackoff); err == nil {
			res.InitialBackoff = d
		} else if logger != nil {
			logger.Warn("invalid initial_backoff, using default", "value", cfg.InitialBackoff, "error", err)
		}
	}
	if cfg.MaxBackoff != "" {
		if d, err := time.ParseDuration(cfg.MaxBackoff); err == nil {
			res.MaxBackoff = d
		} else if logger != nil {
			logger.Warn("invalid max_backoff, using default", "value", cfg.MaxBackoff, "error", err)
		}
	}
	if cfg.BackoffMultiplier != 0 {
		res.BackoffMultiplier = cfg.BackoffMultiplier
	}
	if cfg.FailureThreshold != 0 {
		res.FailureThreshold = cfg.FailureThreshold
	}
	if cfg.ResetTimeout != "" {
		if d, err := time.ParseDuration(cfg.ResetTimeout); err == nil {
			res.ResetTimeout = d
		} else if logger != nil {
			logger.Warn("invalid reset_timeout, using default", "value", cfg.ResetTimeout, "error", err)
		}
	}

	return res
}

func isZeroResilienceConfig(cfg config.LLMResilienceConfig) bool {
	return !cfg.Enabled && cfg.MaxRetries == 0 && cfg.InitialBackoff == "" && cfg.MaxBackoff == "" &&
		cfg.BackoffMultiplier == 0 && cfg.FailureThreshold == 0 && cfg.ResetTimeout == ""
}

func resolveExecuteTimeout(ctxDeadline time.Time, hasDeadline bool, fallback time.Duration) time.Duration {
	if hasDeadline {
		remaining := time.Until(ctxDeadline)
		if remaining > 0 && remaining < fallback {
			return remaining
		}
	}
	return fallback
}
