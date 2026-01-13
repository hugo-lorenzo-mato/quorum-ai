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

// OptimizePromptParams contains parameters for optimize-prompt template.
type OptimizePromptParams struct {
	OriginalPrompt string
}

// RenderOptimizePrompt renders the prompt optimization template.
func (r *PromptRenderer) RenderOptimizePrompt(params OptimizePromptParams) (string, error) {
	return r.render("optimize-prompt", params)
}

// AnalyzeV1Params contains parameters for analyze-v1 template.
type AnalyzeV1Params struct {
	Prompt      string
	ProjectPath string
	Context     string
	Constraints []string
}

// RenderAnalyzeV1 renders the initial analysis prompt.
func (r *PromptRenderer) RenderAnalyzeV1(params AnalyzeV1Params) (string, error) {
	return r.render("analyze-v1", params)
}

// AnalyzeV2Params contains parameters for analyze-v2 template.
type AnalyzeV2Params struct {
	Prompt      string
	V1Analysis  string
	AgentName   string
	Constraints []string
}

// RenderAnalyzeV2 renders the critique analysis prompt.
func (r *PromptRenderer) RenderAnalyzeV2(params AnalyzeV2Params) (string, error) {
	return r.render("analyze-v2-critique", params)
}

// AnalyzeV3Params contains parameters for analyze-v3 template.
type AnalyzeV3Params struct {
	Prompt      string
	V1Analysis  string
	V2Analysis  string
	Divergences []Divergence
}

// RenderAnalyzeV3 renders the reconciliation prompt.
func (r *PromptRenderer) RenderAnalyzeV3(params AnalyzeV3Params) (string, error) {
	return r.render("analyze-v3-reconcile", params)
}

// ConsensusParams contains parameters for consensus check template.
type ConsensusParams struct {
	Analyses []AnalysisOutput
	Result   ConsensusResult
}

// RenderConsensusCheck renders the consensus evaluation prompt.
func (r *PromptRenderer) RenderConsensusCheck(params ConsensusParams) (string, error) {
	return r.render("consensus-check", params)
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
