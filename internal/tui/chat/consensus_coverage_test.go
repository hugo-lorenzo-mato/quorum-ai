package chat

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewConsensusPanel
// ---------------------------------------------------------------------------

func TestNewConsensusPanel_DefaultThreshold(t *testing.T) {
	p := NewConsensusPanel(0)
	if p == nil {
		t.Fatal("NewConsensusPanel(0) returned nil")
	}
	if p.threshold != 80.0 {
		t.Errorf("Default threshold should be 80, got %f", p.threshold)
	}
	if p.visible {
		t.Error("Should start not visible")
	}
	if !p.expanded {
		t.Error("Should start expanded")
	}
}

func TestNewConsensusPanel_NegativeThreshold(t *testing.T) {
	p := NewConsensusPanel(-10)
	if p.threshold != 80.0 {
		t.Errorf("Negative threshold should default to 80, got %f", p.threshold)
	}
}

func TestNewConsensusPanel_CustomThreshold(t *testing.T) {
	p := NewConsensusPanel(90)
	if p.threshold != 90 {
		t.Errorf("Threshold should be 90, got %f", p.threshold)
	}
}

// ---------------------------------------------------------------------------
// SetScore / SetPairScore / SetAgentOutput / ClearOutputs
// ---------------------------------------------------------------------------

func TestConsensusPanel_SetScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(75.5)
	if p.score != 75.5 {
		t.Errorf("Score should be 75.5, got %f", p.score)
	}
}

func TestConsensusPanel_SetPairScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetPairScore("claude", "gemini", "", 85.0)
	key := "claude \u2194 gemini"
	if score, ok := p.pairScores[key]; !ok || score != 85.0 {
		t.Errorf("Pair score should be 85.0, got %f (found: %v)", score, ok)
	}
}

func TestConsensusPanel_SetAgentOutput(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetAgentOutput("claude", "some output")
	if p.agentOutputs["claude"] != "some output" {
		t.Error("Agent output should be stored")
	}
}

func TestConsensusPanel_ClearOutputs(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(90)
	p.SetPairScore("a", "b", "", 80)
	p.SetAgentOutput("a", "out")
	p.AddRound(1, 80)
	p.SetAnalysisPath("/tmp/report")

	p.ClearOutputs()

	if p.score != 0 {
		t.Error("Score should be reset to 0")
	}
	if len(p.pairScores) != 0 {
		t.Error("Pair scores should be cleared")
	}
	if len(p.agentOutputs) != 0 {
		t.Error("Agent outputs should be cleared")
	}
	if len(p.history) != 0 {
		t.Error("History should be cleared")
	}
	if p.analysisPath != "" {
		t.Error("Analysis path should be cleared")
	}
}

// ---------------------------------------------------------------------------
// SetHistory / AddRound
// ---------------------------------------------------------------------------

func TestConsensusPanel_SetHistory(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetHistory([]ConsensusRound{
		{Round: 1, Score: 60},
		{Round: 2, Score: 80},
	})
	if len(p.history) != 2 {
		t.Errorf("Expected 2 history rounds, got %d", len(p.history))
	}
}

func TestConsensusPanel_AddRound(t *testing.T) {
	p := NewConsensusPanel(80)
	p.AddRound(1, 70)
	p.AddRound(2, 85)
	if len(p.history) != 2 {
		t.Errorf("Expected 2 rounds, got %d", len(p.history))
	}
	if p.history[1].Score != 85 {
		t.Errorf("Second round score should be 85, got %f", p.history[1].Score)
	}
}

// ---------------------------------------------------------------------------
// SetAnalysisPath
// ---------------------------------------------------------------------------

func TestConsensusPanel_SetAnalysisPath(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetAnalysisPath("/tmp/analysis")
	if p.analysisPath != "/tmp/analysis" {
		t.Errorf("Analysis path should be '/tmp/analysis', got %q", p.analysisPath)
	}
}

// ---------------------------------------------------------------------------
// Toggle / IsVisible
// ---------------------------------------------------------------------------

func TestConsensusPanel_Toggle(t *testing.T) {
	p := NewConsensusPanel(80)
	if p.IsVisible() {
		t.Error("Should start not visible")
	}
	p.Toggle()
	if !p.IsVisible() {
		t.Error("Should be visible after toggle")
	}
	p.Toggle()
	if p.IsVisible() {
		t.Error("Should not be visible after second toggle")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestConsensusPanel_SetSize(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetSize(100, 50)
	if p.width != 100 || p.height != 50 {
		t.Errorf("Size should be 100x50, got %dx%d", p.width, p.height)
	}
}

// ---------------------------------------------------------------------------
// HasData
// ---------------------------------------------------------------------------

func TestConsensusPanel_HasData_Empty(t *testing.T) {
	p := NewConsensusPanel(80)
	if p.HasData() {
		t.Error("New panel should have no data")
	}
}

func TestConsensusPanel_HasData_WithScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(50)
	if !p.HasData() {
		t.Error("Panel with score should have data")
	}
}

func TestConsensusPanel_HasData_WithPairScores(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetPairScore("a", "b", "", 90)
	if !p.HasData() {
		t.Error("Panel with pair scores should have data")
	}
}

func TestConsensusPanel_HasData_WithHistory(t *testing.T) {
	p := NewConsensusPanel(80)
	p.AddRound(1, 70)
	if !p.HasData() {
		t.Error("Panel with history should have data")
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestConsensusPanel_Render_NotVisible(t *testing.T) {
	p := NewConsensusPanel(80)
	result := p.Render()
	if result != "" {
		t.Error("Not visible panel should render empty string")
	}
}

func TestConsensusPanel_Render_NoData(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle() // make visible
	p.SetSize(80, 40)

	result := p.Render()
	if !strings.Contains(result, "No quorum data yet") {
		t.Error("No data should show empty state message")
	}
	if !strings.Contains(result, "Ctrl+Q") {
		t.Error("Should show close hint")
	}
}

func TestConsensusPanel_Render_HighScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(90)
	p.SetPairScore("claude", "gemini", "", 92)
	p.AddRound(1, 85)
	p.SetAnalysisPath("/tmp/analysis")

	result := p.Render()
	if !strings.Contains(result, "Quorum") {
		t.Error("Should contain 'Quorum' header")
	}
	if !strings.Contains(result, "90%") {
		t.Error("Should show score 90%")
	}
	if !strings.Contains(result, "claude") {
		t.Error("Should show agent pair name 'claude'")
	}
	if !strings.Contains(result, "gemini") {
		t.Error("Should show agent pair name 'gemini'")
	}
	if !strings.Contains(result, "V1") {
		t.Error("Should show round history V1")
	}
	if !strings.Contains(result, "/tmp/analysis") {
		t.Error("Should show analysis path")
	}
}

func TestConsensusPanel_Render_LowScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(40) // below 60, should be red

	result := p.Render()
	if !strings.Contains(result, "40%") {
		t.Error("Should show score 40%")
	}
}

func TestConsensusPanel_Render_MediumScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(70) // between 60 and threshold, should be yellow

	result := p.Render()
	if !strings.Contains(result, "70%") {
		t.Error("Should show score 70%")
	}
}

func TestConsensusPanel_Render_PairScoreLow(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(50)
	p.SetPairScore("a", "b", "", 40) // low pair score

	result := p.Render()
	if !strings.Contains(result, "40%") {
		t.Error("Should show low pair score")
	}
}

func TestConsensusPanel_Render_PairScoreMedium(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(70)
	p.SetPairScore("a", "b", "", 65) // medium pair score

	result := p.Render()
	if !strings.Contains(result, "65%") {
		t.Error("Should show medium pair score")
	}
}

func TestConsensusPanel_Render_HistoryLowScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(50)
	p.AddRound(1, 40) // below threshold

	result := p.Render()
	if !strings.Contains(result, "V1") {
		t.Error("Should show round V1")
	}
}

func TestConsensusPanel_Render_HistoryMediumScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(70)
	p.AddRound(1, 65) // between 60 and threshold

	result := p.Render()
	if !strings.Contains(result, "V1") {
		t.Error("Should show round V1")
	}
}

func TestConsensusPanel_Render_NarrowWidth(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(30, 40) // very narrow, barWidth clamps to 10
	p.SetScore(85)

	result := p.Render()
	if result == "" {
		t.Error("Narrow panel should still render")
	}
}

func TestConsensusPanel_Render_NotExpanded(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(85)
	p.SetPairScore("a", "b", "", 90)
	p.expanded = false

	result := p.Render()
	if strings.Contains(result, "Agent pairs:") {
		t.Error("Not expanded should not show pair details")
	}
}

func TestConsensusPanel_Render_NoAnalysisPath(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(85)

	result := p.Render()
	if strings.Contains(result, "Analysis:") {
		t.Error("No analysis path should not show analysis section")
	}
}

func TestConsensusPanel_Render_ScoreAboveBar(t *testing.T) {
	p := NewConsensusPanel(80)
	p.Toggle()
	p.SetSize(80, 40)
	p.SetScore(120) // above 100, filled should cap at barWidth

	result := p.Render()
	if result == "" {
		t.Error("Score above 100 should still render")
	}
}

// ---------------------------------------------------------------------------
// CompactRender
// ---------------------------------------------------------------------------

func TestConsensusPanel_CompactRender_NoData(t *testing.T) {
	p := NewConsensusPanel(80)
	result := p.CompactRender()
	if result != "" {
		t.Error("No data should return empty string")
	}
}

func TestConsensusPanel_CompactRender_HighScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(90)

	result := p.CompactRender()
	if !strings.Contains(result, "90%") {
		t.Error("Should contain '90%'")
	}
}

func TestConsensusPanel_CompactRender_MediumScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(65) // between 60 and threshold

	result := p.CompactRender()
	if !strings.Contains(result, "65%") {
		t.Error("Should contain '65%'")
	}
}

func TestConsensusPanel_CompactRender_LowScore(t *testing.T) {
	p := NewConsensusPanel(80)
	p.SetScore(30) // below 60

	result := p.CompactRender()
	if !strings.Contains(result, "30%") {
		t.Error("Should contain '30%'")
	}
}
