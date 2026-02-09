package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// noGitChanges represents no filesystem changes detected (nil or empty)
var noGitChanges *GitChangesInfo = nil

// withGitChanges creates a GitChangesInfo indicating files were changed
func withGitChanges(files ...string) *GitChangesInfo {
	return &GitChangesInfo{
		HasChanges:    true,
		ModifiedFiles: files,
	}
}

func TestValidateTaskOutput_GitChangesAsPrimarySignal(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	tests := []struct {
		name       string
		result     *core.ExecuteResult
		task       *core.Task
		gitChanges *GitChangesInfo
		wantValid  bool
		wantWarn   bool
	}{
		{
			name: "git changes detected should pass even with low tokens",
			result: &core.ExecuteResult{
				TokensOut: 100, // Very low
				ToolCalls: []core.ToolCall{},
			},
			task:       &core.Task{Name: "Implement feature"},
			gitChanges: withGitChanges("src/feature.go"),
			wantValid:  true,
			wantWarn:   true, // Informational warning about low tokens
		},
		{
			name: "git changes with multiple files should pass",
			result: &core.ExecuteResult{
				TokensOut: 162, // Low like Copilot
				ToolCalls: []core.ToolCall{},
			},
			task:       &core.Task{Name: "Create Dashboard Component"},
			gitChanges: withGitChanges("src/dashboard.tsx", "src/styles.css"),
			wantValid:  true,
			wantWarn:   true,
		},
		{
			name: "no git changes with implementation task should fail",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:       &core.Task{Name: "Implement API Handler"},
			gitChanges: noGitChanges,
			wantValid:  false,
			wantWarn:   true,
		},
		{
			name: "no git changes with analysis task should pass",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:       &core.Task{Name: "Analyze codebase"},
			gitChanges: noGitChanges,
			wantValid:  true,
			wantWarn:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task, tt.gitChanges)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v (warning: %s)",
					result.Valid, tt.wantValid, result.Warning)
			}
			hasWarning := result.Warning != ""
			if hasWarning != tt.wantWarn {
				t.Errorf("validateTaskOutput() hasWarning = %v, wantWarn = %v, warning = %q",
					hasWarning, tt.wantWarn, result.Warning)
			}
		})
	}
}

func TestValidateTaskOutput_AnalysisTasks(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
	}{
		{
			name: "analyze task with adequate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Analyze existing code"},
			wantValid: true,
		},
		{
			name: "review task with adequate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Review changes"},
			wantValid: true,
		},
		{
			name: "check task with adequate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 180,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Check code quality"},
			wantValid: true,
		},
		{
			name: "analyze task with very low tokens should fail",
			result: &core.ExecuteResult{
				TokensOut: 100,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Analyze patterns"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task, noGitChanges)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v (warning: %s)",
					result.Valid, tt.wantValid, result.Warning)
			}
		})
	}
}

func TestValidateTaskOutput_ImplementationTasksWithoutGitChanges(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	// Implementation tasks without git changes should fail
	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
	}{
		{
			name: "implement task with no git changes should fail",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Implement feature"},
			wantValid: false,
		},
		{
			name: "create task with no git changes should fail",
			result: &core.ExecuteResult{
				TokensOut: 300,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Create component"},
			wantValid: false,
		},
		{
			name: "add task with no git changes should fail",
			result: &core.ExecuteResult{
				TokensOut: 400,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Add new endpoint"},
			wantValid: false,
		},
		{
			name: "build task with no git changes should fail",
			result: &core.ExecuteResult{
				TokensOut: 500,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Build dashboard"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task, noGitChanges)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v (warning: %s)",
					result.Valid, tt.wantValid, result.Warning)
			}
		})
	}
}

func TestValidateTaskOutput_FileOpsDetection(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	toolCallTests := []struct {
		toolName    string
		wantFileOps bool
	}{
		{"write_file", true},
		{"edit_file", true},
		{"create_file", true},
		{"bash", true},
		{"str_replace", true},
		{"read_file", false},
		{"search", false},
		{"grep", false},
	}

	for _, tt := range toolCallTests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := &core.ExecuteResult{
				TokensOut: 500,
				ToolCalls: []core.ToolCall{{Name: tt.toolName}},
			}
			task := &core.Task{Name: "Some task"}

			validation := executor.validateTaskOutput(result, task, noGitChanges)
			if validation.HasFileOps != tt.wantFileOps {
				t.Errorf("validateTaskOutput() HasFileOps = %v for tool %q, want %v",
					validation.HasFileOps, tt.toolName, tt.wantFileOps)
			}
		})
	}
}

func TestValidateTaskOutput_UnknownTaskType(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
		wantWarn  bool
	}{
		{
			name: "unknown task with low tokens should fail",
			result: &core.ExecuteResult{
				TokensOut: 100,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Random task"},
			wantValid: false,
			wantWarn:  true,
		},
		{
			name: "unknown task with adequate tokens should pass with warning",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Do something"},
			wantValid: true,
			wantWarn:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task, noGitChanges)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v (warning: %s)",
					result.Valid, tt.wantValid, result.Warning)
			}
			hasWarning := result.Warning != ""
			if hasWarning != tt.wantWarn {
				t.Errorf("validateTaskOutput() hasWarning = %v, wantWarn = %v",
					hasWarning, tt.wantWarn)
			}
		})
	}
}

// TestValidateTaskOutput_CopilotScenario tests the exact scenario that was failing with Copilot
func TestValidateTaskOutput_CopilotScenario(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	// Copilot typically reports low tokens (estimated) but actually writes files
	t.Run("Copilot with git changes should pass", func(t *testing.T) {
		result := &core.ExecuteResult{
			TokensOut: 162,               // Actual value from the error
			ToolCalls: []core.ToolCall{}, // Copilot doesn't report tool calls
		}
		task := &core.Task{Name: "Implement feature"}
		gitChanges := withGitChanges("src/feature.go", "src/feature_test.go")

		validation := executor.validateTaskOutput(result, task, gitChanges)
		if !validation.Valid {
			t.Errorf("Copilot scenario with git changes should pass, got Valid=%v, warning=%s",
				validation.Valid, validation.Warning)
		}
		if !validation.GitChanges {
			t.Error("GitChanges flag should be set to true")
		}
	})

	t.Run("Copilot without git changes should fail", func(t *testing.T) {
		result := &core.ExecuteResult{
			TokensOut: 162,
			ToolCalls: []core.ToolCall{},
		}
		task := &core.Task{Name: "Implement feature"}

		validation := executor.validateTaskOutput(result, task, noGitChanges)
		if validation.Valid {
			t.Errorf("Copilot scenario without git changes should fail, got Valid=%v",
				validation.Valid)
		}
	})
}

// TestValidateTaskOutput_IssueScenario tests the original issue scenarios
// These should now fail when no git changes are detected
func TestValidateTaskOutput_IssueScenario(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	// These are the actual values from the original issue
	issueScenarios := []struct {
		taskName  string
		tokensOut int
	}{
		{"Implement Frontend Zustand Stores and SSE Client", 131},
		{"Implement File Browser Handlers", 172},
		{"Implement Config Handlers", 154},
		{"Create Frontend Dashboard and Workflow Pages", 130},
		{"Create Frontend Chat Page with Streaming", 109},
		{"Create Frontend Files and Settings Pages", 128},
	}

	for _, scenario := range issueScenarios {
		t.Run(scenario.taskName+" without git changes", func(t *testing.T) {
			result := &core.ExecuteResult{
				TokensOut: scenario.tokensOut,
				ToolCalls: []core.ToolCall{},
			}
			task := &core.Task{Name: scenario.taskName}

			// Without git changes, these should fail (implementation tasks)
			validation := executor.validateTaskOutput(result, task, noGitChanges)
			if validation.Valid {
				t.Errorf("Issue scenario %q with %d tokens should fail without git changes",
					scenario.taskName, scenario.tokensOut)
			}
		})

		t.Run(scenario.taskName+" with git changes", func(t *testing.T) {
			result := &core.ExecuteResult{
				TokensOut: scenario.tokensOut,
				ToolCalls: []core.ToolCall{},
			}
			task := &core.Task{Name: scenario.taskName}

			// WITH git changes, these should pass
			validation := executor.validateTaskOutput(result, task, withGitChanges("file.go"))
			if !validation.Valid {
				t.Errorf("Issue scenario %q with %d tokens should pass WITH git changes. Warning: %s",
					scenario.taskName, scenario.tokensOut, validation.Warning)
			}
		})
	}
}

func TestGitChangesInfo(t *testing.T) {
	t.Parallel()
	t.Run("empty struct has no changes", func(t *testing.T) {
		info := &GitChangesInfo{}
		if info.HasChanges {
			t.Error("Empty GitChangesInfo should have HasChanges=false")
		}
	})

	t.Run("withGitChanges helper sets HasChanges", func(t *testing.T) {
		info := withGitChanges("file1.go", "file2.go")
		if !info.HasChanges {
			t.Error("withGitChanges should set HasChanges=true")
		}
		if len(info.ModifiedFiles) != 2 {
			t.Errorf("Expected 2 modified files, got %d", len(info.ModifiedFiles))
		}
	})
}
