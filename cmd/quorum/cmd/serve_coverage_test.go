package cmd

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// --- buildHeartbeatConfig ---

func TestBuildHeartbeatConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{}
	result := buildHeartbeatConfig(cfg)

	defaults := workflow.DefaultHeartbeatConfig()
	if result.Interval != defaults.Interval {
		t.Errorf("expected default interval %v, got %v", defaults.Interval, result.Interval)
	}
	if result.StaleThreshold != defaults.StaleThreshold {
		t.Errorf("expected default stale threshold %v, got %v", defaults.StaleThreshold, result.StaleThreshold)
	}
	if result.CheckInterval != defaults.CheckInterval {
		t.Errorf("expected default check interval %v, got %v", defaults.CheckInterval, result.CheckInterval)
	}
	if result.AutoResume != false {
		t.Error("expected auto_resume=false by default")
	}
}

func TestBuildHeartbeatConfig_AllFieldsSet(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		Interval:       "10s",
		StaleThreshold: "1m",
		CheckInterval:  "30s",
		AutoResume:     true,
		MaxResumes:     5,
	}

	result := buildHeartbeatConfig(cfg)

	if result.Interval != 10*time.Second {
		t.Errorf("expected interval 10s, got %v", result.Interval)
	}
	if result.StaleThreshold != 1*time.Minute {
		t.Errorf("expected stale threshold 1m, got %v", result.StaleThreshold)
	}
	if result.CheckInterval != 30*time.Second {
		t.Errorf("expected check interval 30s, got %v", result.CheckInterval)
	}
	if !result.AutoResume {
		t.Error("expected auto_resume=true")
	}
	if result.MaxResumes != 5 {
		t.Errorf("expected max_resumes=5, got %d", result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_InvalidInterval(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		Interval: "not-a-duration",
	}

	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	// Invalid interval should fall back to default
	if result.Interval != defaults.Interval {
		t.Errorf("expected default interval for invalid value, got %v", result.Interval)
	}
}

func TestBuildHeartbeatConfig_InvalidStaleThreshold(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		StaleThreshold: "invalid",
	}

	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	if result.StaleThreshold != defaults.StaleThreshold {
		t.Errorf("expected default stale threshold for invalid value, got %v", result.StaleThreshold)
	}
}

func TestBuildHeartbeatConfig_InvalidCheckInterval(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		CheckInterval: "invalid",
	}

	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	if result.CheckInterval != defaults.CheckInterval {
		t.Errorf("expected default check interval for invalid value, got %v", result.CheckInterval)
	}
}

func TestBuildHeartbeatConfig_PartialFields(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		Interval:   "15s",
		AutoResume: true,
		MaxResumes: 3,
	}

	result := buildHeartbeatConfig(cfg)
	if result.Interval != 15*time.Second {
		t.Errorf("expected interval 15s, got %v", result.Interval)
	}
	defaults := workflow.DefaultHeartbeatConfig()
	// Other fields should be default
	if result.StaleThreshold != defaults.StaleThreshold {
		t.Errorf("expected default stale threshold, got %v", result.StaleThreshold)
	}
	if !result.AutoResume {
		t.Error("expected auto_resume=true")
	}
	if result.MaxResumes != 3 {
		t.Errorf("expected max_resumes=3, got %d", result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_ZeroMaxResumes(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		MaxResumes: 0,
	}

	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	// Zero max resumes should use default
	if result.MaxResumes != defaults.MaxResumes {
		t.Errorf("expected default max_resumes, got %d", result.MaxResumes)
	}
}

// --- parseLogLevel ---

func TestParseLogLevel_Debug(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel("debug"); lvl.String() != "DEBUG" {
		t.Errorf("expected DEBUG, got %s", lvl.String())
	}
}

func TestParseLogLevel_Warn(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel("warn"); lvl.String() != "WARN" {
		t.Errorf("expected WARN, got %s", lvl.String())
	}
}

func TestParseLogLevel_Error(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel("error"); lvl.String() != "ERROR" {
		t.Errorf("expected ERROR, got %s", lvl.String())
	}
}

func TestParseLogLevel_Info(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel("info"); lvl.String() != "INFO" {
		t.Errorf("expected INFO, got %s", lvl.String())
	}
}

func TestParseLogLevel_Default(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel("unknown"); lvl.String() != "INFO" {
		t.Errorf("expected INFO for unknown level, got %s", lvl.String())
	}
}

func TestParseLogLevel_Empty(t *testing.T) {
	t.Parallel()
	if lvl := parseLogLevel(""); lvl.String() != "INFO" {
		t.Errorf("expected INFO for empty level, got %s", lvl.String())
	}
}
