package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// --- SidebarConfig ---

func TestSidebarConfig_Fields(t *testing.T) {
	t.Parallel()

	cfg := SidebarConfig{
		Width:     30,
		Height:    25,
		ShowStats: true,
		TotalReqs: 42,
	}

	if cfg.Width != 30 {
		t.Errorf("Width = %d, want 30", cfg.Width)
	}
	if cfg.Height != 25 {
		t.Errorf("Height = %d, want 25", cfg.Height)
	}
	if !cfg.ShowStats {
		t.Error("ShowStats = false, want true")
	}
	if cfg.TotalReqs != 42 {
		t.Errorf("TotalReqs = %d, want 42", cfg.TotalReqs)
	}
}

func TestSidebarConfig_ZeroValue(t *testing.T) {
	t.Parallel()

	var cfg SidebarConfig
	if cfg.Width != 0 {
		t.Errorf("zero value Width = %d, want 0", cfg.Width)
	}
	if cfg.Height != 0 {
		t.Errorf("zero value Height = %d, want 0", cfg.Height)
	}
	if cfg.ShowStats {
		t.Error("zero value ShowStats = true, want false")
	}
	if cfg.TotalReqs != 0 {
		t.Errorf("zero value TotalReqs = %d, want 0", cfg.TotalReqs)
	}
}

// --- DefaultSidebarConfig ---

func TestDefaultSidebarConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultSidebarConfig()

	if cfg.Width != 24 {
		t.Errorf("Width = %d, want 24", cfg.Width)
	}
	if cfg.Height != 20 {
		t.Errorf("Height = %d, want 20", cfg.Height)
	}
	if !cfg.ShowStats {
		t.Error("ShowStats = false, want true")
	}
	if cfg.TotalReqs != 0 {
		t.Errorf("TotalReqs = %d, want 0", cfg.TotalReqs)
	}
}

// --- Styles ---

func TestSidebarStyle(t *testing.T) {
	t.Parallel()
	result := sidebarStyle.Render("test")
	if len(result) == 0 {
		t.Error("sidebarStyle.Render returned empty")
	}
}

func TestSidebarTitleStyle(t *testing.T) {
	t.Parallel()
	result := sidebarTitleStyle.Render("AGENTS")
	if !strings.Contains(result, "AGENTS") {
		t.Error("sidebarTitleStyle.Render does not contain input")
	}
}

func TestSidebarDividerStyle(t *testing.T) {
	t.Parallel()
	result := sidebarDividerStyle.Render("───")
	if len(result) == 0 {
		t.Error("sidebarDividerStyle.Render returned empty")
	}
}

func TestStatsLabelStyle(t *testing.T) {
	t.Parallel()
	result := statsLabelStyle.Render("Session:")
	if !strings.Contains(result, "Session:") {
		t.Error("statsLabelStyle.Render does not contain input")
	}
}

// --- RenderSidebar ---

func TestRenderSidebar_NoAgents(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	cfg := DefaultSidebarConfig()
	cfg.ShowStats = false

	result := RenderSidebar([]*Agent{}, sp, cfg)
	if len(result) == 0 {
		t.Fatal("RenderSidebar returned empty with no agents")
	}
	if !strings.Contains(result, "AGENTS") {
		t.Error("sidebar should contain title 'AGENTS'")
	}
}

func TestRenderSidebar_SingleAgent(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed"), Duration: 500 * time.Millisecond},
	}
	cfg := DefaultSidebarConfig()
	cfg.ShowStats = false

	result := RenderSidebar(agents, sp, cfg)
	if !strings.Contains(result, "AGENTS") {
		t.Error("sidebar should contain title 'AGENTS'")
	}
	if !strings.Contains(result, "claude") {
		t.Error("sidebar should contain agent name 'claude'")
	}
}

func TestRenderSidebar_MultipleAgents(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed"), Duration: 100 * time.Millisecond},
		{Name: "gemini", Status: StatusWorking, Color: lipgloss.Color("#3b82f6")},
		{Name: "codex", Status: StatusIdle, Color: lipgloss.Color("#10b981")},
	}
	cfg := DefaultSidebarConfig()
	cfg.ShowStats = false

	result := RenderSidebar(agents, sp, cfg)
	if !strings.Contains(result, "claude") {
		t.Error("sidebar should contain 'claude'")
	}
	if !strings.Contains(result, "gemini") {
		t.Error("sidebar should contain 'gemini'")
	}
	if !strings.Contains(result, "codex") {
		t.Error("sidebar should contain 'codex'")
	}
}

func TestRenderSidebar_WithStats(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	agents := []*Agent{
		{Name: "claude", Status: StatusIdle, Color: lipgloss.Color("#7c3aed")},
	}
	cfg := SidebarConfig{
		Width:     30,
		Height:    20,
		ShowStats: true,
		TotalReqs: 7,
	}

	result := RenderSidebar(agents, sp, cfg)
	if !strings.Contains(result, "Session:") {
		t.Error("sidebar with stats should contain 'Session:'")
	}
	if !strings.Contains(result, "7 req") {
		t.Error("sidebar with stats should contain '7 req'")
	}
}

func TestRenderSidebar_StatsDisabled(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	agents := []*Agent{
		{Name: "claude", Status: StatusIdle, Color: lipgloss.Color("#7c3aed")},
	}
	cfg := SidebarConfig{
		Width:     30,
		Height:    20,
		ShowStats: false,
		TotalReqs: 10,
	}

	result := RenderSidebar(agents, sp, cfg)
	if strings.Contains(result, "Session:") {
		t.Error("sidebar without stats should not contain 'Session:'")
	}
}

func TestRenderSidebar_ZeroReqs(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	cfg := SidebarConfig{
		Width:     30,
		Height:    20,
		ShowStats: true,
		TotalReqs: 0,
	}

	result := RenderSidebar([]*Agent{}, sp, cfg)
	if !strings.Contains(result, "0 req") {
		t.Error("sidebar with zero requests should show '0 req'")
	}
}

func TestRenderSidebar_CustomDimensions(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	cfg := SidebarConfig{
		Width:  50,
		Height: 40,
	}

	result := RenderSidebar([]*Agent{}, sp, cfg)
	if len(result) == 0 {
		t.Error("RenderSidebar with custom dimensions returned empty")
	}
}

func TestRenderSidebar_SmallDimensions(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	cfg := SidebarConfig{
		Width:     10,
		Height:    5,
		ShowStats: true,
		TotalReqs: 1,
	}

	// Should not panic with small dimensions
	result := RenderSidebar([]*Agent{}, sp, cfg)
	if len(result) == 0 {
		t.Error("RenderSidebar with small dimensions returned empty")
	}
}

func TestRenderSidebar_AllAgentStatuses(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	agents := []*Agent{
		{Name: "idle-agent", Status: StatusIdle, Color: lipgloss.Color("#6b7280")},
		{Name: "working-agent", Status: StatusWorking, Color: lipgloss.Color("#3b82f6")},
		{Name: "done-agent", Status: StatusDone, Color: lipgloss.Color("#10b981"), Duration: 200 * time.Millisecond},
		{Name: "error-agent", Status: StatusError, Color: lipgloss.Color("#ef4444"), Error: "timeout"},
	}
	cfg := SidebarConfig{
		Width:     40,
		Height:    30,
		ShowStats: true,
		TotalReqs: 4,
	}

	result := RenderSidebar(agents, sp, cfg)
	if !strings.Contains(result, "idle-agent") {
		t.Error("sidebar should contain 'idle-agent'")
	}
	if !strings.Contains(result, "working-agent") {
		t.Error("sidebar should contain 'working-agent'")
	}
	if !strings.Contains(result, "done-agent") {
		t.Error("sidebar should contain 'done-agent'")
	}
	if !strings.Contains(result, "error-agent") {
		t.Error("sidebar should contain 'error-agent'")
	}
}

func TestRenderSidebar_StatsDividerPresent(t *testing.T) {
	t.Parallel()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	cfg := SidebarConfig{
		Width:     30,
		Height:    20,
		ShowStats: true,
		TotalReqs: 3,
	}

	result := RenderSidebar([]*Agent{}, sp, cfg)
	// Divider uses "─" characters
	if !strings.Contains(result, "─") {
		t.Error("sidebar with stats should contain divider character '─'")
	}
}
