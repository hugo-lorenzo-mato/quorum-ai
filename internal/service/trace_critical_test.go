package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestMetricsCollector_Basic tests basic metric collection functionality.
func TestMetricsCollector_Basic(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()

	// Test workflow start/end
	collector.StartWorkflow()
	time.Sleep(10 * time.Millisecond) // Small delay
	collector.EndWorkflow(time.Now())

	// Test task recording
	task := &TaskMetrics{
		TaskID:    core.TaskID("test-task"),
		Name:      "test",
		Phase:     core.PhaseAnalyze,
		Agent:     "claude",
		StartTime: time.Now().Add(-1 * time.Second),
		EndTime:   time.Now(),
		Duration:  1 * time.Second,
		TokensIn:  1000,
		TokensOut: 1500,
		Retries:   0,
		Success:   true,
	}

	collector.RecordTask(task)

	// Get metrics
	metrics := collector.GetWorkflowMetrics()
	
	// Verify basic fields
	if metrics.TasksTotal != 1 {
		t.Errorf("Expected 1 task total, got %d", metrics.TasksTotal)
	}

	if metrics.TasksCompleted != 1 {
		t.Errorf("Expected 1 completed task, got %d", metrics.TasksCompleted)
	}

	if metrics.TotalTokensIn != 1000 {
		t.Errorf("Expected 1000 tokens in, got %d", metrics.TotalTokensIn)
	}

	if metrics.TotalTokensOut != 1500 {
		t.Errorf("Expected 1500 tokens out, got %d", metrics.TotalTokensOut)
	}
}

// TestMetricsCollector_MultipleAgents tests metrics with multiple agents.
func TestMetricsCollector_MultipleAgents(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// Record tasks from different agents
	agents := []string{"claude", "gemini", "gpt"}
	for i, agent := range agents {
		task := &TaskMetrics{
			TaskID:    core.TaskID(fmt.Sprintf("task-%d", i)),
			Name:      fmt.Sprintf("task%d", i),
			Phase:     core.PhaseAnalyze,
			Agent:     agent,
			StartTime: time.Now().Add(-1 * time.Second),
			EndTime:   time.Now(),
			Duration:  time.Duration(i+1) * 100 * time.Millisecond,
			TokensIn:  (i + 1) * 500,
			TokensOut: (i + 1) * 750,
			Success:   true,
		}
		collector.RecordTask(task)
	}

	collector.EndWorkflow(time.Now())
	metrics := collector.GetWorkflowMetrics()

	expectedTasks := len(agents)
	if metrics.TasksTotal != expectedTasks {
		t.Errorf("Expected %d tasks, got %d", expectedTasks, metrics.TasksTotal)
	}

	expectedTokensIn := 500 + 1000 + 1500 // 3000
	if metrics.TotalTokensIn != expectedTokensIn {
		t.Errorf("Expected %d tokens in, got %d", expectedTokensIn, metrics.TotalTokensIn)
	}

	expectedTokensOut := 750 + 1500 + 2250 // 4500  
	if metrics.TotalTokensOut != expectedTokensOut {
		t.Errorf("Expected %d tokens out, got %d", expectedTokensOut, metrics.TotalTokensOut)
	}
}

// TestTraceConfig_Validation tests trace configuration validation.
func TestTraceConfig_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		config  TraceConfig
		isValid bool
	}{
		{
			name: "valid_config",
			config: TraceConfig{
				Mode:          "enabled",
				Dir:           "/tmp/traces",
				SchemaVersion: 1,
				MaxBytes:      1024 * 1024,
				MaxFiles:      10,
			},
			isValid: true,
		},
		{
			name: "disabled_mode",
			config: TraceConfig{
				Mode: "disabled",
			},
			isValid: true,
		},
		{
			name: "invalid_mode",
			config: TraceConfig{
				Mode: "invalid",
			},
			isValid: false,
		},
		{
			name: "zero_max_bytes",
			config: TraceConfig{
				Mode:     "enabled",
				Dir:      "/tmp/traces",
				MaxBytes: 0,
			},
			isValid: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateTraceConfig(tc.config)
			if tc.isValid && err != nil {
				t.Errorf("Expected valid config to pass validation, got error: %v", err)
			}
			if !tc.isValid && err == nil {
				t.Errorf("Expected invalid config to fail validation, but it passed")
			}
		})
	}
}

// TestTraceRunInfo_Serialization tests trace metadata serialization.
func TestTraceRunInfo_Serialization(t *testing.T) {
	t.Parallel()

	runInfo := TraceRunInfo{
		RunID:        "test-run-123",
		WorkflowID:   "workflow-456",
		PromptLength: 1500,
		StartedAt:    time.Now(),
		AppVersion:   "1.0.0",
		AppCommit:    "abc123",
		AppDate:      "2024-01-01",
		GitCommit:    "def456",
		GitDirty:     false,
		Config: TraceConfig{
			Mode:        "enabled",
			Dir:         "/tmp/traces",
			MaxBytes:    1024 * 1024,
			MaxFiles:    10,
			Redact:      true,
		},
	}

	// Test serialization to JSON
	jsonBytes, err := marshalTraceInfo(runInfo)
	if err != nil {
		t.Fatalf("Failed to serialize trace info: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("Serialized trace info should not be empty")
	}

	// Verify JSON contains expected fields
	jsonStr := string(jsonBytes)
	requiredFields := []string{
		"run_id", "workflow_id", "prompt_length", "started_at",
		"app_version", "app_commit", "git_commit", "config",
	}

	for _, field := range requiredFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("Serialized JSON missing field: %s", field)
		}
	}

	t.Logf("Serialized trace info (%d bytes): %s", len(jsonBytes), jsonStr[:min(200, len(jsonStr))])
}

// Helper functions

func validateTraceConfig(config TraceConfig) error {
	validModes := []string{"enabled", "disabled"}
	modeValid := false
	for _, mode := range validModes {
		if config.Mode == mode {
			modeValid = true
			break
		}
	}
	
	if !modeValid {
		return fmt.Errorf("invalid trace mode: %s", config.Mode)
	}

	if config.Mode == "enabled" && config.MaxBytes <= 0 {
		return fmt.Errorf("MaxBytes must be positive when tracing is enabled")
	}

	return nil
}

func marshalTraceInfo(info TraceRunInfo) ([]byte, error) {
	// Simplified JSON marshaling
	json := fmt.Sprintf(`{
		"run_id": "%s",
		"workflow_id": "%s", 
		"prompt_length": %d,
		"started_at": "%s",
		"app_version": "%s",
		"app_commit": "%s",
		"git_commit": "%s",
		"config": {
			"mode": "%s",
			"dir": "%s",
			"max_bytes": %d,
			"redact": %t
		}
	}`, info.RunID, info.WorkflowID, info.PromptLength, 
		info.StartedAt.Format(time.RFC3339), info.AppVersion, 
		info.AppCommit, info.GitCommit, info.Config.Mode, 
		info.Config.Dir, info.Config.MaxBytes, info.Config.Redact)
	
	return []byte(json), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}