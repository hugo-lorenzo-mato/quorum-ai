package service

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestPromptRenderer_Load(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	templates := renderer.ListTemplates()
	if len(templates) == 0 {
		t.Error("expected at least one template to be loaded")
	}

	t.Logf("Loaded templates: %v", templates)

	// Check expected templates exist
	expectedTemplates := []string{
		"analyze-v1",
		"plan-generate",
		"task-execute",
		"arbiter-evaluate",
		"vn-refine",
		"consolidate-analysis",
	}

	for _, expected := range expectedTemplates {
		if !renderer.HasTemplate(expected) {
			t.Errorf("expected template %q not found", expected)
		}
	}
}

func TestPromptRenderer_RenderAnalyzeV1(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := AnalyzeV1Params{
		Prompt:      "Add a new feature",
		ProjectPath: "/path/to/project",
		Context:     "This is a Go project with tests",
		Constraints: []string{"No breaking changes", "Must have tests"},
	}

	result, err := renderer.RenderAnalyzeV1(params)
	if err != nil {
		t.Fatalf("RenderAnalyzeV1() error = %v", err)
	}

	// Check content includes expected elements
	if !strings.Contains(result, "Add a new feature") {
		t.Error("result should contain the prompt")
	}
	if !strings.Contains(result, "This is a Go project") {
		t.Error("result should contain the context")
	}
	if !strings.Contains(result, "No breaking changes") {
		t.Error("result should contain constraints")
	}
	if !strings.Contains(result, "Investigar el código") {
		t.Error("result should contain analysis instructions")
	}
}

func TestPromptRenderer_RenderPlanGenerate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := PlanParams{
		Prompt:               "Implement user authentication",
		ConsolidatedAnalysis: "Analysis suggests using JWT",
		Constraints:          []string{"Use existing auth library"},
		MaxTasks:             5,
	}

	result, err := renderer.RenderPlanGenerate(params)
	if err != nil {
		t.Fatalf("RenderPlanGenerate() error = %v", err)
	}

	if !strings.Contains(result, "user authentication") {
		t.Error("result should contain the prompt")
	}
	if !strings.Contains(result, "5") {
		t.Error("result should contain max tasks")
	}
	if !strings.Contains(result, "Task Plan") {
		t.Error("result should contain 'Task Plan' header")
	}
}

func TestPromptRenderer_RenderTaskExecute(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	task := core.NewTask("task-1", "Implement login", core.PhaseExecute)
	task.Description = "Create login endpoint"

	params := TaskExecuteParams{
		Task:        task,
		Context:     "User auth context",
		WorkDir:     "/path/to/project",
		Constraints: []string{"Follow existing patterns"},
	}

	result, err := renderer.RenderTaskExecute(params)
	if err != nil {
		t.Fatalf("RenderTaskExecute() error = %v", err)
	}

	if !strings.Contains(result, "task-1") {
		t.Error("result should contain task ID")
	}
	if !strings.Contains(result, "Implement login") {
		t.Error("result should contain task name")
	}
	if !strings.Contains(result, "/path/to/project") {
		t.Error("result should contain work directory")
	}
}

func TestPromptRenderer_MissingTemplate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	_, err = renderer.Render("nonexistent-template", nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestIndent(t *testing.T) {
	tests := []struct {
		name   string
		spaces int
		input  string
		want   string
	}{
		{
			name:   "single line",
			spaces: 2,
			input:  "hello",
			want:   "  hello",
		},
		{
			name:   "multiple lines",
			spaces: 4,
			input:  "line1\nline2\nline3",
			want:   "    line1\n    line2\n    line3",
		},
		{
			name:   "empty lines preserved",
			spaces: 2,
			input:  "line1\n\nline3",
			want:   "  line1\n\n  line3",
		},
		{
			name:   "zero spaces",
			spaces: 0,
			input:  "hello",
			want:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indent(tt.spaces, tt.input)
			if got != tt.want {
				t.Errorf("indent(%d, %q) = %q, want %q", tt.spaces, tt.input, got, tt.want)
			}
		})
	}
}

func TestPromptRenderer_HasTemplate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	if !renderer.HasTemplate("analyze-v1") {
		t.Error("HasTemplate should return true for existing template")
	}

	if renderer.HasTemplate("nonexistent") {
		t.Error("HasTemplate should return false for non-existing template")
	}
}

func TestPromptRenderer_RenderArbiterEvaluate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := ArbiterEvaluateParams{
		Prompt: "Add a new feature",
		Round:  2,
		Analyses: []ArbiterAnalysisSummary{
			{AgentName: "claude", Output: "Analysis from Claude"},
			{AgentName: "gemini", Output: "Analysis from Gemini"},
		},
		BelowThreshold: true,
	}

	result, err := renderer.RenderArbiterEvaluate(params)
	if err != nil {
		t.Fatalf("RenderArbiterEvaluate() error = %v", err)
	}

	if !strings.Contains(result, "Round 2") {
		t.Error("result should contain round number")
	}
	if !strings.Contains(result, "claude") {
		t.Error("result should contain agent name")
	}
	if !strings.Contains(result, "CONSENSUS_SCORE") {
		t.Error("result should contain consensus score instruction")
	}
}

func TestPromptRenderer_RenderVnRefine(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := VnRefineParams{
		Prompt:           "Add a new feature",
		Context:          "Project context",
		Round:            3,
		PreviousRound:    2,
		PreviousAnalysis: "Previous analysis content",
		ConsensusScore:   75.0,
		Threshold:        90.0,
		Agreements:       []string{"Agreement 1", "Agreement 2"},
		Divergences: []VnDivergenceInfo{
			{
				Category:       "claims",
				YourPosition:   "Position A",
				OtherPositions: "Position B",
				Guidance:       "Refine based on evidence",
			},
		},
		MissingPerspectives: []string{"Missing perspective 1"},
	}

	result, err := renderer.RenderVnRefine(params)
	if err != nil {
		t.Fatalf("RenderVnRefine() error = %v", err)
	}

	if !strings.Contains(result, "Round 3") {
		t.Error("result should contain round number")
	}
	if !strings.Contains(result, "Agreement 1") {
		t.Error("result should contain agreements")
	}
	if !strings.Contains(result, "75") {
		t.Error("result should contain consensus score")
	}
	if !strings.Contains(result, "Previous analysis content") {
		t.Error("result should contain previous analysis")
	}
}

func TestPromptRenderer_NoConstraints(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := AnalyzeV1Params{
		Prompt:      "Simple request",
		Context:     "Context here",
		Constraints: nil, // No constraints
	}

	result, err := renderer.RenderAnalyzeV1(params)
	if err != nil {
		t.Fatalf("RenderAnalyzeV1() error = %v", err)
	}

	// Should still render without error
	if !strings.Contains(result, "Simple request") {
		t.Error("result should contain the prompt")
	}
}
