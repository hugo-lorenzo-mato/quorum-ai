package service

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"gopkg.in/yaml.v3"
)

func parseConsensusSchemaExample(t *testing.T, content string) map[string]any {
	t.Helper()

	// Look for an actual code block (newline before ```yaml), not prose text
	const marker = "\n```yaml\n"
	start := strings.Index(content, marker)
	if start == -1 {
		t.Fatalf("yaml example block not found")
	}
	rest := content[start+len(marker):]

	end := strings.Index(rest, "\n```")
	if end == -1 {
		t.Fatalf("yaml example block not closed")
	}

	block := strings.TrimSpace(rest[:end])
	// Strip YAML document separators if present (---)
	block = strings.TrimPrefix(block, "---\n")
	block = strings.TrimSuffix(block, "\n---")
	block = strings.TrimSpace(block)

	var data map[string]any
	if err := yaml.Unmarshal([]byte(block), &data); err != nil {
		t.Fatalf("yaml example parse failed: %v", err)
	}
	return data
}

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
		"task-detail-generate",
		"moderator-evaluate",
		"vn-refine",
		"synthesize-analysis",
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
	if !strings.Contains(result, "Investigate the code") {
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

func TestPromptRenderer_RenderTaskDetailGenerate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := TaskDetailGenerateParams{
		TaskID:               "task-1",
		TaskName:             "Create web server",
		Dependencies:         []string{"task-0"},
		OutputPath:           "/output/tasks/task-1-create-web-server.md",
		ConsolidatedAnalysis: "This is the consolidated analysis with architecture details",
	}

	result, err := renderer.RenderTaskDetailGenerate(params)
	if err != nil {
		t.Fatalf("RenderTaskDetailGenerate() error = %v", err)
	}

	// Check content includes expected elements
	if !strings.Contains(result, "task-1") {
		t.Error("result should contain task ID")
	}
	if !strings.Contains(result, "Create web server") {
		t.Error("result should contain task name")
	}
	if !strings.Contains(result, "task-0") {
		t.Error("result should contain dependencies")
	}
	if !strings.Contains(result, "/output/tasks/task-1-create-web-server.md") {
		t.Error("result should contain output path")
	}
	if !strings.Contains(result, "consolidated analysis") {
		t.Error("result should contain consolidated analysis")
	}
	if !strings.Contains(result, "SELF-CONTAINED") {
		t.Error("result should emphasize self-contained requirement")
	}
	if !strings.Contains(result, "Exhaustive") && !strings.Contains(result, "exhaustive") {
		t.Error("result should mention exhaustiveness")
	}
}

func TestPromptRenderer_RenderTaskDetailGenerate_NoDependencies(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := TaskDetailGenerateParams{
		TaskID:               "task-1",
		TaskName:             "Initial setup",
		Dependencies:         nil, // No dependencies
		OutputPath:           "/output/tasks/task-1-initial-setup.md",
		ConsolidatedAnalysis: "Analysis content",
	}

	result, err := renderer.RenderTaskDetailGenerate(params)
	if err != nil {
		t.Fatalf("RenderTaskDetailGenerate() error = %v", err)
	}

	if !strings.Contains(result, "None") {
		t.Error("result should show 'None' for empty dependencies")
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

func TestPromptRenderer_RenderModeratorEvaluate(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := ModeratorEvaluateParams{
		Prompt:    "Add a new feature",
		Round:     2,
		NextRound: 3,
		Analyses: []ModeratorAnalysisSummary{
			{AgentName: "claude", FilePath: "/path/to/v2/claude-analysis.md"},
			{AgentName: "gemini", FilePath: "/path/to/v2/gemini-analysis.md"},
		},
		BelowThreshold: true,
	}

	result, err := renderer.RenderModeratorEvaluate(params)
	if err != nil {
		t.Fatalf("RenderArbiterEvaluate() error = %v", err)
	}

	if !strings.Contains(result, "Round 2") {
		t.Error("result should contain round number")
	}
	if !strings.Contains(result, "claude") {
		t.Error("result should contain agent name")
	}

	fm := parseConsensusSchemaExample(t, result)
	requiredKeys := []string{
		"consensus_score",
		"high_impact_divergences",
		"medium_impact_divergences",
		"low_impact_divergences",
		"agreements_count",
	}
	for _, key := range requiredKeys {
		if _, ok := fm[key]; !ok {
			t.Errorf("frontmatter missing required key: %s", key)
		}
	}
}

func TestPromptRenderer_RenderVnRefine(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	params := VnRefineParams{
		Prompt:               "Add a new feature",
		Context:              "Project context",
		Round:                3,
		PreviousRound:        2,
		PreviousAnalysis:     "Previous analysis content",
		HasArbiterEvaluation: true, // V3 has arbiter evaluation
		ConsensusScore:       75.0,
		Threshold:            90.0,
		Agreements:           []string{"Agreement 1", "Agreement 2"},
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

func TestPromptRenderer_RenderVnRefine_V2_NoArbiter(t *testing.T) {
	renderer, err := NewPromptRenderer()
	if err != nil {
		t.Fatalf("NewPromptRenderer() error = %v", err)
	}

	// V2 is the first refinement - no arbiter evaluation yet
	params := VnRefineParams{
		Prompt:               "Add a new feature",
		Context:              "Project context",
		Round:                2,
		PreviousRound:        1,
		PreviousAnalysis:     "Previous V1 analysis content",
		HasArbiterEvaluation: false, // V2 has no arbiter evaluation yet
		ConsensusScore:       0,
		Threshold:            90.0,
		Agreements:           nil,
		Divergences:          nil,
		MissingPerspectives:  nil,
	}

	result, err := renderer.RenderVnRefine(params)
	if err != nil {
		t.Fatalf("RenderVnRefine() error = %v", err)
	}

	if !strings.Contains(result, "Round 2") {
		t.Error("result should contain round number")
	}
	if !strings.Contains(result, "Previous V1 analysis content") {
		t.Error("result should contain previous analysis")
	}
	// V2 should show ultracritical review section, not arbiter evaluation
	if !strings.Contains(result, "Ultra-Critical Review") {
		t.Error("V2 result should contain ultracritical review section")
	}
	// V2 should NOT show arbiter evaluation
	if strings.Contains(result, "Arbiter Evaluation") {
		t.Error("V2 result should NOT contain arbiter evaluation section")
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
