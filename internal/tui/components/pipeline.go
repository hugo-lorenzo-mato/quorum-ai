package components

import (
	"github.com/charmbracelet/lipgloss"
)

// PipelineNode represents a node in the visual pipeline.
type PipelineNode struct {
	Label  string
	Status AgentStatus
	Color  lipgloss.Color
}

var (
	pipelineConnectorDone = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7c3aed")).
				Render("═══")

	pipelineConnectorPending = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#374151")).
					Render("───")
)

// RenderPipeline renders a visual pipeline of nodes.
// Example: [C]═══[G]═══[X]
func RenderPipeline(nodes []PipelineNode) string {
	if len(nodes) == 0 {
		return ""
	}

	result := ""
	for i, node := range nodes {
		result += renderPipelineNode(node)

		if i < len(nodes)-1 {
			// Add connector based on current node status
			if node.Status == StatusDone {
				result += pipelineConnectorDone
			} else {
				result += pipelineConnectorPending
			}
		}
	}

	return result
}

func renderPipelineNode(node PipelineNode) string {
	var bg lipgloss.Color
	switch node.Status {
	case StatusDone:
		bg = node.Color
	case StatusWorking:
		bg = lipgloss.Color("#374151") // gray, slightly highlighted
	default:
		bg = lipgloss.Color("#1f2937") // dark gray
	}

	nodeStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(lipgloss.Color("#fff")).
		Padding(0, 1).
		Bold(node.Status == StatusWorking)

	return nodeStyle.Render(node.Label)
}

// BuildPipelineFromAgents creates pipeline nodes from agent list.
func BuildPipelineFromAgents(agents []*Agent) []PipelineNode {
	nodes := make([]PipelineNode, len(agents))
	for i, agent := range agents {
		label := string(agent.Name[0]) // First letter as label
		nodes[i] = PipelineNode{
			Label:  label,
			Status: agent.Status,
			Color:  agent.Color,
		}
	}
	return nodes
}
