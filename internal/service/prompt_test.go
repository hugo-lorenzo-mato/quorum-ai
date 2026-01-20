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
		"analyze-v2-critique",
		"analyze-v3-reconcile",
		"consensus-check",
		"plan-generate",
		"task-execute",
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
	if !strings.Contains(result, "Claims") {
		t.Error("result should contain 'Claims' instruction")
	}
}

func TestPromptRenderer_RenderAnalyzeV2(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := AnalyzeV2Params{
		Prompt: "Add a new feature",
		AllV1Analyses: []V1AnalysisSummary{
			{AgentName: "claude", Output: "Previous analysis content from Claude"},
			{AgentName: "gemini", Output: "Previous analysis content from Gemini"},
		},
		Constraints: []string{"Must be backward compatible"},
	}

	result, err := renderer.RenderAnalyzeV2(params)
	if err != nil {
		t.Fatalf("RenderAnalyzeV2() error = %v", err)
	}

	if !strings.Contains(result, "Previous analysis content from Claude") {
		t.Error("result should contain V1 analysis from Claude")
	}
	if !strings.Contains(result, "claude") {
		t.Error("result should contain agent name")
	}
	if !strings.Contains(result, "Critical Review") {
		t.Error("result should contain 'Critical Review' header")
	}
}

func TestPromptRenderer_RenderAnalyzeV3(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := AnalyzeV3Params{
		Prompt:     "Add a new feature",
		V1Analysis: "V1 analysis content",
		V2Analysis: "V2 analysis content",
		Divergences: []Divergence{
			{
				Category:     "claims",
				Agent1:       "claude",
				Agent1Items:  []string{"Claim A"},
				Agent2:       "gemini",
				Agent2Items:  []string{"Claim B"},
				JaccardScore: 0.3,
			},
		},
	}

	result, err := renderer.RenderAnalyzeV3(params)
	if err != nil {
		t.Fatalf("RenderAnalyzeV3() error = %v", err)
	}

	if !strings.Contains(result, "Reconciliation") {
		t.Error("result should contain 'Reconciliation' header")
	}
	if !strings.Contains(result, "claims") {
		t.Error("result should contain divergence category")
	}
	if !strings.Contains(result, "0.30") {
		t.Error("result should contain formatted Jaccard score")
	}
}

func TestPromptRenderer_RenderConsensusCheck(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := ConsensusParams{
		Analyses: []AnalysisOutput{
			{
				AgentName:       "claude",
				Claims:          []string{"Claim 1", "Claim 2"},
				Risks:           []string{"Risk 1"},
				Recommendations: []string{"Rec 1"},
			},
		},
		Result: ConsensusResult{
			Score: 0.85,
			CategoryScores: map[string]float64{
				"claims": 0.9,
				"risks":  0.8,
			},
		},
	}

	result, err := renderer.RenderConsensusCheck(params)
	if err != nil {
		t.Fatalf("RenderConsensusCheck() error = %v", err)
	}

	if !strings.Contains(result, "claude") {
		t.Error("result should contain agent name")
	}
	if !strings.Contains(result, "0.85") {
		t.Error("result should contain overall score")
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
