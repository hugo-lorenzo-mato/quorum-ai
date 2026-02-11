package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// --- ProgressCardConfig ---

func TestProgressCardConfig_Fields(t *testing.T) {
	t.Parallel()

	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "Test Workflow",
		Percentage:   0.5,
		ShowPipeline: true,
		Agents: []*Agent{
			{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
		},
	}

	if cfg.Width != 80 {
		t.Errorf("Width = %d, want 80", cfg.Width)
	}
	if cfg.Title != "Test Workflow" {
		t.Errorf("Title = %q, want %q", cfg.Title, "Test Workflow")
	}
	if cfg.Percentage != 0.5 {
		t.Errorf("Percentage = %f, want 0.5", cfg.Percentage)
	}
	if !cfg.ShowPipeline {
		t.Error("ShowPipeline = false, want true")
	}
	if len(cfg.Agents) != 1 {
		t.Errorf("len(Agents) = %d, want 1", len(cfg.Agents))
	}
}

func TestProgressCardConfig_ZeroValue(t *testing.T) {
	t.Parallel()

	var cfg ProgressCardConfig
	if cfg.Width != 0 {
		t.Errorf("zero value Width = %d, want 0", cfg.Width)
	}
	if cfg.Percentage != 0 {
		t.Errorf("zero value Percentage = %f, want 0", cfg.Percentage)
	}
	if cfg.ShowPipeline {
		t.Error("zero value ShowPipeline = true, want false")
	}
	if cfg.Agents != nil {
		t.Error("zero value Agents should be nil")
	}
}

// --- Styles ---

func TestProgressCardStyle(t *testing.T) {
	t.Parallel()
	result := progressCardStyle.Render("test")
	if len(result) == 0 {
		t.Error("progressCardStyle.Render returned empty")
	}
}

func TestProgressTitleStyle(t *testing.T) {
	t.Parallel()
	result := progressTitleStyle.Render("title")
	if !strings.Contains(result, "title") {
		t.Error("progressTitleStyle.Render does not contain input")
	}
}

func TestProgressPctStyle(t *testing.T) {
	t.Parallel()
	result := progressPctStyle.Render("50%")
	if !strings.Contains(result, "50%") {
		t.Error("progressPctStyle.Render does not contain input")
	}
}

// --- NewProgressBar ---

func TestNewProgressBar_ReturnsModel(t *testing.T) {
	t.Parallel()

	pb := NewProgressBar()
	// progress.Model is a struct; verify it renders without panic
	view := pb.ViewAs(0.5)
	if len(view) == 0 {
		t.Error("NewProgressBar().ViewAs(0.5) returned empty")
	}
}

func TestNewProgressBar_ZeroPercent(t *testing.T) {
	t.Parallel()

	pb := NewProgressBar()
	view := pb.ViewAs(0.0)
	if len(view) == 0 {
		t.Error("NewProgressBar().ViewAs(0.0) returned empty")
	}
}

func TestNewProgressBar_FullPercent(t *testing.T) {
	t.Parallel()

	pb := NewProgressBar()
	view := pb.ViewAs(1.0)
	if len(view) == 0 {
		t.Error("NewProgressBar().ViewAs(1.0) returned empty")
	}
}

// --- RenderProgressCard ---

func TestRenderProgressCard_Basic(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "Test Workflow",
		Percentage:   0.5,
		ShowPipeline: false,
	}

	result := RenderProgressCard(cfg, pb)
	if len(result) == 0 {
		t.Fatal("RenderProgressCard returned empty")
	}
	if !strings.Contains(result, "Test Workflow") {
		t.Error("progress card should contain title 'Test Workflow'")
	}
	if !strings.Contains(result, "50%") {
		t.Error("progress card should contain '50%'")
	}
}

func TestRenderProgressCard_ZeroPercent(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:      80,
		Title:      "Starting",
		Percentage: 0.0,
	}

	result := RenderProgressCard(cfg, pb)
	if !strings.Contains(result, "0%") {
		t.Error("progress card should contain '0%'")
	}
	if !strings.Contains(result, "Starting") {
		t.Error("progress card should contain title 'Starting'")
	}
}

func TestRenderProgressCard_FullPercent(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:      80,
		Title:      "Done",
		Percentage: 1.0,
	}

	result := RenderProgressCard(cfg, pb)
	if !strings.Contains(result, "100%") {
		t.Error("progress card should contain '100%'")
	}
}

func TestRenderProgressCard_WithPipeline(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed"), Duration: 100 * time.Millisecond},
		{Name: "gemini", Status: StatusWorking, Color: lipgloss.Color("#3b82f6")},
		{Name: "codex", Status: StatusIdle, Color: lipgloss.Color("#10b981")},
	}
	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "Multi-Agent",
		Percentage:   0.33,
		ShowPipeline: true,
		Agents:       agents,
	}

	result := RenderProgressCard(cfg, pb)
	if !strings.Contains(result, "Multi-Agent") {
		t.Error("progress card should contain title 'Multi-Agent'")
	}
	if !strings.Contains(result, "33%") {
		t.Error("progress card should contain '33%'")
	}
	// Pipeline nodes should be rendered (first letters)
	if !strings.Contains(result, "c") {
		t.Error("progress card with pipeline should contain pipeline node 'c'")
	}
}

func TestRenderProgressCard_ShowPipelineNoAgents(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "No Agents",
		Percentage:   0.0,
		ShowPipeline: true,
		Agents:       []*Agent{},
	}

	// Should not panic with ShowPipeline=true and empty agents
	result := RenderProgressCard(cfg, pb)
	if len(result) == 0 {
		t.Error("RenderProgressCard returned empty")
	}
}

func TestRenderProgressCard_ShowPipelineNilAgents(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "Nil Agents",
		Percentage:   0.0,
		ShowPipeline: true,
		Agents:       nil,
	}

	// Should not panic with ShowPipeline=true and nil agents
	result := RenderProgressCard(cfg, pb)
	if len(result) == 0 {
		t.Error("RenderProgressCard returned empty")
	}
}

func TestRenderProgressCard_PipelineDisabled(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
	}
	cfg := ProgressCardConfig{
		Width:        80,
		Title:        "No Pipeline",
		Percentage:   1.0,
		ShowPipeline: false,
		Agents:       agents,
	}

	result := RenderProgressCard(cfg, pb)
	if !strings.Contains(result, "No Pipeline") {
		t.Error("progress card should contain title")
	}
}

func TestRenderProgressCard_PercentageBoundaries(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	percentages := []float64{0.0, 0.01, 0.25, 0.5, 0.75, 0.99, 1.0}

	for _, pct := range percentages {
		cfg := ProgressCardConfig{
			Width:      80,
			Title:      "Boundary",
			Percentage: pct,
		}
		result := RenderProgressCard(cfg, pb)
		if len(result) == 0 {
			t.Errorf("RenderProgressCard returned empty for percentage %f", pct)
		}
	}
}

func TestRenderProgressCard_NarrowWidth(t *testing.T) {
	t.Parallel()

	pb := progress.New(progress.WithoutPercentage())
	cfg := ProgressCardConfig{
		Width:      20,
		Title:      "Narrow",
		Percentage: 0.5,
	}

	// Should not panic with narrow width
	result := RenderProgressCard(cfg, pb)
	if len(result) == 0 {
		t.Error("RenderProgressCard returned empty for narrow width")
	}
}
