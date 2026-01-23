package service

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"text/template"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

//go:embed prompts/*.md.tmpl
var promptsFS embed.FS

// PromptRenderer renders prompts from templates.
type PromptRenderer struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// NewPromptRenderer creates a new prompt renderer.
func NewPromptRenderer() (*PromptRenderer, error) {
	r := &PromptRenderer{
		templates: make(map[string]*template.Template),
	}

	if err := r.loadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return r, nil
}

// loadTemplates loads all templates from the embedded filesystem.
func (r *PromptRenderer) loadTemplates() error {
	return fs.WalkDir(promptsFS, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md.tmpl") {
			return nil
		}

		content, err := promptsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		name := strings.TrimPrefix(path, "prompts/")
		name = strings.TrimSuffix(name, ".md.tmpl")

		tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}

		r.templates[name] = tmpl
		return nil
	})
}

// templateFuncs returns custom template functions.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join":      strings.Join,
		"indent":    indent,
		"trimSpace": strings.TrimSpace,
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
	}
}

func indent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

// RefinePromptParams contains parameters for prompt refinement template.
type RefinePromptParams struct {
	OriginalPrompt string
}

// RenderRefinePrompt renders the prompt refinement template.
func (r *PromptRenderer) RenderRefinePrompt(params RefinePromptParams) (string, error) {
	return r.render("refine-prompt", params)
}

// AnalyzeV1Params contains parameters for analyze-v1 template.
type AnalyzeV1Params struct {
	Prompt         string
	ProjectPath    string
	Context        string
	Constraints    []string
	OutputFilePath string // Optional: if set, LLM should write output to this file
}

// RenderAnalyzeV1 renders the initial analysis prompt.
func (r *PromptRenderer) RenderAnalyzeV1(params AnalyzeV1Params) (string, error) {
	return r.render("analyze-v1", params)
}

// AnalysisOutput represents the output from an analysis agent for templates.
type AnalysisOutput struct {
	AgentName       string
	RawOutput       string
	Claims          []string
	Risks           []string
	Recommendations []string
}

// SynthesizeAnalysisParams contains parameters for analysis synthesis template.
type SynthesizeAnalysisParams struct {
	Prompt         string
	Analyses       []AnalysisOutput
	OutputFilePath string // Optional: if set, LLM should write output to this file
}

// RenderSynthesizeAnalysis renders the analysis synthesis prompt.
func (r *PromptRenderer) RenderSynthesizeAnalysis(params SynthesizeAnalysisParams) (string, error) {
	return r.render("synthesize-analysis", params)
}

// PlanParams contains parameters for plan generation template.
type PlanParams struct {
	Prompt               string
	ConsolidatedAnalysis string
	Constraints          []string
	MaxTasks             int
}

// RenderPlanGenerate renders the plan generation prompt.
func (r *PromptRenderer) RenderPlanGenerate(params PlanParams) (string, error) {
	return r.render("plan-generate", params)
}

// PlanProposal represents a plan proposal from an agent.
type PlanProposal struct {
	AgentName string
	Model     string
	Content   string
}

// SynthesizePlansParams contains parameters for plan synthesis template.
type SynthesizePlansParams struct {
	Prompt   string
	Analysis string
	Plans    []PlanProposal
	MaxTasks int
}

// RenderSynthesizePlans renders the multi-agent plan synthesis prompt.
func (r *PromptRenderer) RenderSynthesizePlans(params SynthesizePlansParams) (string, error) {
	return r.render("consolidate-plans", params)
}

// TaskExecuteParams contains parameters for task execution template.
type TaskExecuteParams struct {
	Task        *core.Task
	Context     string
	WorkDir     string
	Constraints []string
}

// RenderTaskExecute renders the task execution prompt.
func (r *PromptRenderer) RenderTaskExecute(params TaskExecuteParams) (string, error) {
	return r.render("task-execute", params)
}

// Render renders a template by name with the given data.
func (r *PromptRenderer) Render(name string, data interface{}) (string, error) {
	return r.render(name, data)
}

// render executes a template with the given data.
func (r *PromptRenderer) render(name string, data interface{}) (string, error) {
	r.mu.RLock()
	tmpl, ok := r.templates[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", name, err)
	}

	return buf.String(), nil
}

// ListTemplates returns available template names.
func (r *PromptRenderer) ListTemplates() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}

// HasTemplate checks if a template exists.
func (r *PromptRenderer) HasTemplate(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.templates[name]
	return ok
}

// ModeratorAnalysisSummary represents an analysis for moderator evaluation.
type ModeratorAnalysisSummary struct {
	AgentName string
	Output    string
}

// ModeratorEvaluateParams contains parameters for moderator evaluation template.
type ModeratorEvaluateParams struct {
	Prompt         string
	Round          int
	NextRound      int // Round + 1, for recommendations
	Analyses       []ModeratorAnalysisSummary
	BelowThreshold bool
	OutputFilePath string // Path where LLM should write moderator report
}

// RenderModeratorEvaluate renders the moderator semantic evaluation prompt.
func (r *PromptRenderer) RenderModeratorEvaluate(params ModeratorEvaluateParams) (string, error) {
	return r.render("moderator-evaluate", params)
}

// VnDivergenceInfo contains divergence information for V(n) refinement.
type VnDivergenceInfo struct {
	Category       string
	YourPosition   string
	OtherPositions string
	Guidance       string
}

// VnRefineParams contains parameters for vn-refine template.
type VnRefineParams struct {
	Prompt               string
	Context              string
	Round                int
	PreviousRound        int
	PreviousAnalysis     string
	HasArbiterEvaluation bool    // True if arbiter has evaluated (V3+), false for V2
	ConsensusScore       float64 // Only meaningful if HasArbiterEvaluation is true
	Threshold            float64
	Agreements           []string
	Divergences          []VnDivergenceInfo
	MissingPerspectives  []string
	Constraints          []string
	OutputFilePath       string // Optional: if set, LLM should write output to this file
}

// RenderVnRefine renders the V(n) refinement prompt.
func (r *PromptRenderer) RenderVnRefine(params VnRefineParams) (string, error) {
	return r.render("vn-refine", params)
}
