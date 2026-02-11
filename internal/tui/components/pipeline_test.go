package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// --- PipelineNode struct ---

func TestPipelineNode_Fields(t *testing.T) {
	t.Parallel()

	node := PipelineNode{
		Label:  "C",
		Status: StatusDone,
		Color:  lipgloss.Color("#7c3aed"),
	}

	if node.Label != "C" {
		t.Errorf("Label = %q, want %q", node.Label, "C")
	}
	if node.Status != StatusDone {
		t.Errorf("Status = %d, want StatusDone", node.Status)
	}
	if node.Color != lipgloss.Color("#7c3aed") {
		t.Errorf("Color = %v, want #7c3aed", node.Color)
	}
}

func TestPipelineNode_ZeroValue(t *testing.T) {
	t.Parallel()

	var node PipelineNode
	if node.Label != "" {
		t.Errorf("zero value Label = %q, want empty", node.Label)
	}
	if node.Status != StatusIdle {
		t.Errorf("zero value Status = %d, want StatusIdle", node.Status)
	}
}

// --- Pipeline connector styles ---

func TestPipelineConnectorStyles(t *testing.T) {
	t.Parallel()

	if len(pipelineConnectorDone) == 0 {
		t.Error("pipelineConnectorDone is empty")
	}
	if len(pipelineConnectorPending) == 0 {
		t.Error("pipelineConnectorPending is empty")
	}
}

// --- renderPipelineNode ---

func TestRenderPipelineNode_StatusDone(t *testing.T) {
	t.Parallel()

	node := PipelineNode{
		Label:  "C",
		Status: StatusDone,
		Color:  lipgloss.Color("#7c3aed"),
	}
	result := renderPipelineNode(node)
	if len(result) == 0 {
		t.Fatal("renderPipelineNode returned empty for done node")
	}
	if !strings.Contains(result, "C") {
		t.Error("rendered node should contain label 'C'")
	}
}

func TestRenderPipelineNode_StatusWorking(t *testing.T) {
	t.Parallel()

	node := PipelineNode{
		Label:  "G",
		Status: StatusWorking,
		Color:  lipgloss.Color("#3b82f6"),
	}
	result := renderPipelineNode(node)
	if len(result) == 0 {
		t.Fatal("renderPipelineNode returned empty for working node")
	}
	if !strings.Contains(result, "G") {
		t.Error("rendered node should contain label 'G'")
	}
}

func TestRenderPipelineNode_StatusIdle(t *testing.T) {
	t.Parallel()

	node := PipelineNode{
		Label:  "X",
		Status: StatusIdle,
		Color:  lipgloss.Color("#10b981"),
	}
	result := renderPipelineNode(node)
	if len(result) == 0 {
		t.Fatal("renderPipelineNode returned empty for idle node")
	}
	if !strings.Contains(result, "X") {
		t.Error("rendered node should contain label 'X'")
	}
}

func TestRenderPipelineNode_StatusError(t *testing.T) {
	t.Parallel()

	node := PipelineNode{
		Label:  "E",
		Status: StatusError,
		Color:  lipgloss.Color("#ef4444"),
	}
	result := renderPipelineNode(node)
	if len(result) == 0 {
		t.Fatal("renderPipelineNode returned empty for error node")
	}
	if !strings.Contains(result, "E") {
		t.Error("rendered node should contain label 'E'")
	}
}

// --- RenderPipeline ---

func TestRenderPipeline_EmptyNodes(t *testing.T) {
	t.Parallel()

	result := RenderPipeline(nil)
	if result != "" {
		t.Errorf("RenderPipeline(nil) = %q, want empty", result)
	}

	result = RenderPipeline([]PipelineNode{})
	if result != "" {
		t.Errorf("RenderPipeline([]) = %q, want empty", result)
	}
}

func TestRenderPipeline_SingleNode(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "C", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
	}
	result := RenderPipeline(nodes)
	if len(result) == 0 {
		t.Fatal("RenderPipeline with single node returned empty")
	}
	if !strings.Contains(result, "C") {
		t.Error("single-node pipeline should contain 'C'")
	}
}

func TestRenderPipeline_TwoNodes_DoneConnector(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "C", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
		{Label: "G", Status: StatusWorking, Color: lipgloss.Color("#3b82f6")},
	}
	result := RenderPipeline(nodes)
	if !strings.Contains(result, "C") {
		t.Error("pipeline should contain 'C'")
	}
	if !strings.Contains(result, "G") {
		t.Error("pipeline should contain 'G'")
	}
	// First node is done, so connector should be the done connector (═══)
	if !strings.Contains(result, "═") {
		t.Error("pipeline should contain done connector '═' after done node")
	}
}

func TestRenderPipeline_TwoNodes_PendingConnector(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "C", Status: StatusWorking, Color: lipgloss.Color("#7c3aed")},
		{Label: "G", Status: StatusIdle, Color: lipgloss.Color("#3b82f6")},
	}
	result := RenderPipeline(nodes)
	if !strings.Contains(result, "C") {
		t.Error("pipeline should contain 'C'")
	}
	if !strings.Contains(result, "G") {
		t.Error("pipeline should contain 'G'")
	}
	// First node is working (not done), so connector should be pending (───)
	if !strings.Contains(result, "─") {
		t.Error("pipeline should contain pending connector '─' after non-done node")
	}
}

func TestRenderPipeline_ThreeNodes_MixedStatuses(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "C", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
		{Label: "G", Status: StatusDone, Color: lipgloss.Color("#3b82f6")},
		{Label: "X", Status: StatusIdle, Color: lipgloss.Color("#10b981")},
	}
	result := RenderPipeline(nodes)
	if !strings.Contains(result, "C") {
		t.Error("pipeline should contain 'C'")
	}
	if !strings.Contains(result, "G") {
		t.Error("pipeline should contain 'G'")
	}
	if !strings.Contains(result, "X") {
		t.Error("pipeline should contain 'X'")
	}
}

func TestRenderPipeline_AllDone(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "A", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
		{Label: "B", Status: StatusDone, Color: lipgloss.Color("#3b82f6")},
		{Label: "C", Status: StatusDone, Color: lipgloss.Color("#10b981")},
	}
	result := RenderPipeline(nodes)
	// All connectors should be done (═══)
	if !strings.Contains(result, "═") {
		t.Error("all-done pipeline should use done connectors")
	}
}

func TestRenderPipeline_AllIdle(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "A", Status: StatusIdle, Color: lipgloss.Color("#7c3aed")},
		{Label: "B", Status: StatusIdle, Color: lipgloss.Color("#3b82f6")},
	}
	result := RenderPipeline(nodes)
	// All connectors should be pending (───)
	if !strings.Contains(result, "─") {
		t.Error("all-idle pipeline should use pending connectors")
	}
	// Should not have done connectors
	if strings.Contains(result, "═") {
		t.Error("all-idle pipeline should not have done connectors")
	}
}

func TestRenderPipeline_ErrorNode_UsesPendingConnector(t *testing.T) {
	t.Parallel()

	nodes := []PipelineNode{
		{Label: "E", Status: StatusError, Color: lipgloss.Color("#ef4444")},
		{Label: "B", Status: StatusIdle, Color: lipgloss.Color("#3b82f6")},
	}
	result := RenderPipeline(nodes)
	// Error node is not done, so connector should be pending
	if strings.Contains(result, "═") {
		t.Error("error node should use pending connector, not done connector")
	}
}

// --- BuildPipelineFromAgents ---

func TestBuildPipelineFromAgents_Empty(t *testing.T) {
	t.Parallel()

	agents := []*Agent{}
	nodes := BuildPipelineFromAgents(agents)
	if len(nodes) != 0 {
		t.Errorf("BuildPipelineFromAgents([]) len = %d, want 0", len(nodes))
	}
}

func TestBuildPipelineFromAgents_SingleAgent(t *testing.T) {
	t.Parallel()

	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
	}
	nodes := BuildPipelineFromAgents(agents)
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].Label != "c" {
		t.Errorf("Label = %q, want %q", nodes[0].Label, "c")
	}
	if nodes[0].Status != StatusDone {
		t.Errorf("Status = %d, want StatusDone", nodes[0].Status)
	}
	if nodes[0].Color != lipgloss.Color("#7c3aed") {
		t.Errorf("Color = %v, want #7c3aed", nodes[0].Color)
	}
}

func TestBuildPipelineFromAgents_MultipleAgents(t *testing.T) {
	t.Parallel()

	agents := []*Agent{
		{Name: "claude", Status: StatusDone, Color: lipgloss.Color("#7c3aed")},
		{Name: "gemini", Status: StatusWorking, Color: lipgloss.Color("#3b82f6")},
		{Name: "codex", Status: StatusIdle, Color: lipgloss.Color("#10b981")},
	}
	nodes := BuildPipelineFromAgents(agents)
	if len(nodes) != 3 {
		t.Fatalf("len(nodes) = %d, want 3", len(nodes))
	}

	expected := []struct {
		label  string
		status AgentStatus
	}{
		{"c", StatusDone},
		{"g", StatusWorking},
		{"c", StatusIdle},
	}

	for i, want := range expected {
		if nodes[i].Label != want.label {
			t.Errorf("nodes[%d].Label = %q, want %q", i, nodes[i].Label, want.label)
		}
		if nodes[i].Status != want.status {
			t.Errorf("nodes[%d].Status = %d, want %d", i, nodes[i].Status, want.status)
		}
	}
}

func TestBuildPipelineFromAgents_PreservesColor(t *testing.T) {
	t.Parallel()

	color := lipgloss.Color("#abcdef")
	agents := []*Agent{
		{Name: "test", Status: StatusIdle, Color: color},
	}
	nodes := BuildPipelineFromAgents(agents)
	if nodes[0].Color != color {
		t.Errorf("Color = %v, want %v", nodes[0].Color, color)
	}
}

func TestBuildPipelineFromAgents_FirstLetterLabel(t *testing.T) {
	t.Parallel()

	agents := []*Agent{
		{Name: "Alpha", Status: StatusIdle, Color: lipgloss.Color("#000000")},
		{Name: "Beta", Status: StatusIdle, Color: lipgloss.Color("#000000")},
		{Name: "zeta", Status: StatusIdle, Color: lipgloss.Color("#000000")},
	}
	nodes := BuildPipelineFromAgents(agents)

	if nodes[0].Label != "A" {
		t.Errorf("nodes[0].Label = %q, want %q", nodes[0].Label, "A")
	}
	if nodes[1].Label != "B" {
		t.Errorf("nodes[1].Label = %q, want %q", nodes[1].Label, "B")
	}
	if nodes[2].Label != "z" {
		t.Errorf("nodes[2].Label = %q, want %q", nodes[2].Label, "z")
	}
}
