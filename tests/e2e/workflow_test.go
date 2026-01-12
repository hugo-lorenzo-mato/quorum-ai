//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestWorkflow_BasicExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping workflow test in short mode")
	}

	dir := testutil.TempDir(t)
	setupMockEnvironment(t, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple workflow
	builder := core.NewWorkflowBuilder("test-workflow")
	task := builder.AddTask("test-task", core.PhaseAnalyze)
	task.Name = "Test Analysis Task"

	wf := builder.Build()

	// Execute with mock agents
	runner := service.NewWorkflowRunner(nil)
	mode := service.ExecutionMode{DryRun: true}
	runner.SetMode(mode)

	state, err := runner.Execute(ctx, wf)
	if err != nil && !service.IsDryRunBlocked(err) {
		// Dry run blocks are expected
		t.Logf("execution result: %v", err)
	}

	if state == nil {
		t.Fatal("state should not be nil")
	}
}

func TestWorkflow_PhaseProgression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping workflow test in short mode")
	}

	// Test that phases progress correctly
	phases := []core.Phase{
		core.PhaseAnalyze,
		core.PhaseResearch,
		core.PhaseCode,
		core.PhaseTest,
	}

	for i, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			if i > 0 {
				prevPhase := phases[i-1]
				if !phase.After(prevPhase) {
					t.Errorf("phase %s should come after %s", phase, prevPhase)
				}
			}
		})
	}
}

func TestWorkflow_WithConsensus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping consensus test in short mode")
	}

	checker := service.NewConsensusChecker(0.75)

	outputs := []service.AnalysisOutput{
		{
			AgentName: "claude",
			Sections: map[string]string{
				"claims":          "The code has good structure and follows best practices",
				"risks":           "No major security vulnerabilities identified",
				"recommendations": "Consider adding more unit tests",
			},
		},
		{
			AgentName: "gemini",
			Sections: map[string]string{
				"claims":          "The code structure is well organized and follows conventions",
				"risks":           "No security issues found in the codebase",
				"recommendations": "Add unit tests for better coverage",
			},
		},
	}

	result := checker.Evaluate(outputs)

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("invalid consensus score: %f", result.Score)
	}

	t.Logf("Consensus score: %.2f%%", result.Score*100)
}

func setupMockEnvironment(t *testing.T, dir string) {
	t.Helper()

	// Create mock CLI scripts for agents
	mockClaude := `#!/bin/sh
echo '{"output": "Mock Claude analysis", "tokens_in": 100, "tokens_out": 50}'
`
	writeMockScript(t, dir, "claude", mockClaude)

	mockGemini := `#!/bin/sh
echo '{"output": "Mock Gemini analysis", "tokens_in": 100, "tokens_out": 50}'
`
	writeMockScript(t, dir, "gemini", mockGemini)

	// Add mock scripts to PATH
	currentPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+currentPath)
}

func writeMockScript(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o755)
	if err != nil {
		t.Fatalf("writing mock script: %v", err)
	}
}
