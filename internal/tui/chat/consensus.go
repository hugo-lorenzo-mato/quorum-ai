package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ConsensusPanel displays the consensus level between agents
type ConsensusPanel struct {
	score        float64            // Overall consensus score (0-100)
	threshold    float64            // Configured threshold (e.g., 80)
	pairScores   map[string]float64 // Scores between agent pairs
	agentOutputs map[string]string  // Raw outputs for diff
	width        int
	height       int
	visible      bool
	expanded     bool // Show pair details
}

// NewConsensusPanel creates a new consensus panel
func NewConsensusPanel(threshold float64) *ConsensusPanel {
	if threshold <= 0 {
		threshold = 80.0
	}
	return &ConsensusPanel{
		threshold:    threshold,
		pairScores:   make(map[string]float64),
		agentOutputs: make(map[string]string),
		visible:      false,
		expanded:     true,
	}
}

// SetScore updates the overall consensus score
func (p *ConsensusPanel) SetScore(score float64) {
	p.score = score
}

// SetPairScore sets the consensus between two agents
func (p *ConsensusPanel) SetPairScore(agent1, agent2, _ string, val float64) {
	key := fmt.Sprintf("%s ↔ %s", agent1, agent2)
	p.pairScores[key] = val
}

// SetAgentOutput stores an agent's output for diff comparison
func (p *ConsensusPanel) SetAgentOutput(agent, output string) {
	p.agentOutputs[agent] = output
}

// ClearOutputs resets all agent outputs
func (p *ConsensusPanel) ClearOutputs() {
	p.agentOutputs = make(map[string]string)
	p.pairScores = make(map[string]float64)
	p.score = 0
}

// Toggle toggles panel visibility
func (p *ConsensusPanel) Toggle() {
	p.visible = !p.visible
}

// IsVisible returns whether the panel is visible
func (p *ConsensusPanel) IsVisible() bool {
	return p.visible
}

// SetSize sets the panel dimensions
func (p *ConsensusPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// HasData returns true if there's consensus data to display
func (p *ConsensusPanel) HasData() bool {
	return len(p.pairScores) > 0 || p.score > 0
}

// Render renders the consensus panel
func (p *ConsensusPanel) Render() string {
	if !p.visible || !p.HasData() {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	// Determine bar color based on score
	var barColor lipgloss.Color
	switch {
	case p.score >= p.threshold:
		barColor = lipgloss.Color("#22c55e") // Green
	case p.score >= 60:
		barColor = lipgloss.Color("#eab308") // Yellow
	default:
		barColor = lipgloss.Color("#ef4444") // Red
	}

	barStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	// Build content
	var sb strings.Builder

	// Header
	header := headerStyle.Render(" Consensus")
	sb.WriteString(header)
	sb.WriteString("\n")

	// Progress bar
	barWidth := p.width - 25
	if barWidth < 10 {
		barWidth = 10
	}
	filled := int(float64(barWidth) * p.score / 100)
	if filled > barWidth {
		filled = barWidth
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		emptyBarStyle.Render(strings.Repeat("░", barWidth-filled))

	scoreStr := fmt.Sprintf(" %.0f%%", p.score)
	thresholdStr := dimStyle.Render(fmt.Sprintf(" (threshold: %.0f%%)", p.threshold))

	sb.WriteString(bar)
	sb.WriteString(barStyle.Render(scoreStr))
	sb.WriteString(thresholdStr)
	sb.WriteString("\n")

	// Pair scores (if expanded and available)
	if p.expanded && len(p.pairScores) > 0 {
		var pairs []string
		for pair, score := range p.pairScores {
			var pairColor lipgloss.Color
			switch {
			case score >= p.threshold:
				pairColor = lipgloss.Color("#22c55e")
			case score >= 60:
				pairColor = lipgloss.Color("#eab308")
			default:
				pairColor = lipgloss.Color("#ef4444")
			}
			pairStyle := lipgloss.NewStyle().Foreground(pairColor)
			pairs = append(pairs, fmt.Sprintf("%s: %s", pair, pairStyle.Render(fmt.Sprintf("%.0f%%", score))))
		}

		pairsLine := dimStyle.Render(strings.Join(pairs, "  "))
		sb.WriteString(pairsLine)
	}

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		Width(p.width - 2)

	return boxStyle.Render(sb.String())
}

// CompactRender renders a single-line version for the header
func (p *ConsensusPanel) CompactRender() string {
	if !p.HasData() {
		return ""
	}

	// Determine color based on score
	var color lipgloss.Color
	switch {
	case p.score >= p.threshold:
		color = lipgloss.Color("#22c55e")
	case p.score >= 60:
		color = lipgloss.Color("#eab308")
	default:
		color = lipgloss.Color("#ef4444")
	}

	style := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	// Mini bar (5 chars)
	barWidth := 5
	filled := int(float64(barWidth) * p.score / 100)
	bar := style.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", barWidth-filled))

	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%.0f%%", p.score)))
}
