package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestValidateTaskOutput_SuspiciouslyLowTokens(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
		wantWarn  bool
	}{
		{
			name: "very low tokens should fail",
			result: &core.ExecuteResult{
				TokensOut: 100, // Below SuspiciouslyLowTokenThreshold
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Implement feature"},
			wantValid: false,
			wantWarn:  true,
		},
		{
			name: "adequate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 500,
				ToolCalls: []core.ToolCall{{Name: "write_file"}},
			},
			task:      &core.Task{Name: "Implement feature"},
			wantValid: true,
			wantWarn:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v", result.Valid, tt.wantValid)
			}
			hasWarning := result.Warning != ""
			if hasWarning != tt.wantWarn {
				t.Errorf("validateTaskOutput() warning = %q, wantWarn = %v", result.Warning, tt.wantWarn)
			}
		})
	}
}

func TestValidateTaskOutput_ImplementationTaskNoToolCalls(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
		wantWarn  bool
	}{
		{
			name: "implement task with no tool calls and low tokens should fail",
			result: &core.ExecuteResult{
				TokensOut: 200, // Below MinExpectedTokensForImplementation
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Implement Frontend Zustand Stores"},
			wantValid: false,
			wantWarn:  true,
		},
		{
			name: "implement task with no tool calls but high tokens should warn",
			result: &core.ExecuteResult{
				TokensOut: 500,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Implement Config Handlers"},
			wantValid: true,
			wantWarn:  true, // Warning but not failure
		},
		{
			name: "implement task with tool calls should pass",
			result: &core.ExecuteResult{
				TokensOut: 500,
				ToolCalls: []core.ToolCall{{Name: "write_file"}},
			},
			task:      &core.Task{Name: "Implement API endpoint"},
			wantValid: true,
			wantWarn:  false,
		},
		{
			name: "create task with no tool calls and low tokens should fail",
			result: &core.ExecuteResult{
				TokensOut: 150,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Create Frontend Dashboard"},
			wantValid: false,
			wantWarn:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task)
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

func TestValidateTaskOutput_SubstantialCodeTasks(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
		wantWarn  bool
	}{
		{
			name: "page task with low tokens and no file ops should fail",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Create Dashboard Page"},
			wantValid: false,
			wantWarn:  true,
		},
		{
			name: "component task with low tokens but has file ops should warn",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{{Name: "write_file"}},
			},
			task:      &core.Task{Name: "Create Header Component"},
			wantValid: true,
			wantWarn:  true,
		},
		{
			name: "handler task with adequate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 400,
				ToolCalls: []core.ToolCall{{Name: "edit_file"}},
			},
			task:      &core.Task{Name: "Implement API Handler"},
			wantValid: true,
			wantWarn:  false,
		},
		{
			name: "store task with low tokens and no tool calls should fail",
			result: &core.ExecuteResult{
				TokensOut: 130, // Codex example from issue
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Implement Frontend Zustand Stores and SSE Client"},
			wantValid: false,
			wantWarn:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task)
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

func TestValidateTaskOutput_FileOpsDetection(t *testing.T) {
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

			validation := executor.validateTaskOutput(result, task)
			if validation.HasFileOps != tt.wantFileOps {
				t.Errorf("validateTaskOutput() HasFileOps = %v for tool %q, want %v",
					validation.HasFileOps, tt.toolName, tt.wantFileOps)
			}
		})
	}
}

func TestValidateTaskOutput_NonImplementationTasks(t *testing.T) {
	executor := &Executor{}

	// Non-implementation tasks should have more lenient validation
	tests := []struct {
		name      string
		result    *core.ExecuteResult
		task      *core.Task
		wantValid bool
	}{
		{
			name: "analyze task with moderate tokens should pass",
			result: &core.ExecuteResult{
				TokensOut: 200,
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Analyze existing code"},
			wantValid: true,
		},
		{
			name: "review task with low tokens should still fail below threshold",
			result: &core.ExecuteResult{
				TokensOut: 100, // Below absolute minimum
				ToolCalls: []core.ToolCall{},
			},
			task:      &core.Task{Name: "Review changes"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.validateTaskOutput(tt.result, tt.task)
			if result.Valid != tt.wantValid {
				t.Errorf("validateTaskOutput().Valid = %v, want %v (warning: %s)",
					result.Valid, tt.wantValid, result.Warning)
			}
		})
	}
}

// TestValidateTaskOutput_IssueScenario tests the exact scenario from issue #171
func TestValidateTaskOutput_IssueScenario(t *testing.T) {
	executor := &Executor{}

	// These are the actual values from the issue
	issueScenarios := []struct {
		taskName  string
		tokensOut int
		wantValid bool
	}{
		{"Implement Frontend Zustand Stores and SSE Client", 131, false},
		{"Implement File Browser Handlers", 172, false},
		{"Implement Config Handlers", 154, false},
		{"Create Frontend Dashboard and Workflow Pages", 130, false},
		{"Create Frontend Chat Page with Streaming", 109, false},
		{"Create Frontend Files and Settings Pages", 128, false},
	}

	for _, scenario := range issueScenarios {
		t.Run(scenario.taskName, func(t *testing.T) {
			result := &core.ExecuteResult{
				TokensOut: scenario.tokensOut,
				ToolCalls: []core.ToolCall{}, // No tool calls as reported in issue
			}
			task := &core.Task{Name: scenario.taskName}

			validation := executor.validateTaskOutput(result, task)
			if validation.Valid != scenario.wantValid {
				t.Errorf("Issue scenario %q with %d tokens: got Valid=%v, want %v. Warning: %s",
					scenario.taskName, scenario.tokensOut, validation.Valid, scenario.wantValid, validation.Warning)
			}
		})
	}
}
