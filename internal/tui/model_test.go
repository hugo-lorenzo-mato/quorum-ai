package tui

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestModel_PhaseUpdateMsg(t *testing.T) {
	model := New()

	updated, _ := model.Update(PhaseUpdateMsg{Phase: core.PhaseAnalyze})
	m := updated.(Model)

	if m.currentPhase != core.PhaseAnalyze {
		t.Errorf("expected phase analyze, got %v", m.currentPhase)
	}
}

func TestRenderHeader_ShowsCurrentPhase(t *testing.T) {
	model := Model{
		currentPhase: core.PhaseExecute,
		workflow:     &core.WorkflowState{Status: core.WorkflowStatusRunning},
		width:        80,
	}

	header := model.renderHeader()

	if !strings.Contains(header, "execute") {
		t.Errorf("header should show execute phase, got: %s", header)
	}
	if !strings.Contains(header, "running") {
		t.Errorf("header should show running status, got: %s", header)
	}
}

func TestRenderHeader_NoWorkflow(t *testing.T) {
	model := Model{
		currentPhase: core.PhaseOptimize,
		workflow:     nil,
		width:        80,
	}

	header := model.renderHeader()

	if !strings.Contains(header, "optimize") {
		t.Errorf("header should show optimize phase, got: %s", header)
	}
}
