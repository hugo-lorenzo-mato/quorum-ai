package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ConsensusRound represents a single consensus evaluation round
type ConsensusRound struct {
	Round int
	Score float64 // 0-100
}

// ConsensusPanel displays the consensus level between agents
type ConsensusPanel struct {
	score        float64            // Overall consensus score (0-100)
	threshold    float64            // Configured threshold (e.g., 80)
	pairScores   map[string]float64 // Scores between agent pairs
	agentOutputs map[string]string  // Raw outputs for diff
	history      []ConsensusRound   // Previous consensus rounds
	analysisPath string             // Path to analysis reports
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
		history:      make([]ConsensusRound, 0),
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
	p.history = make([]ConsensusRound, 0)
	p.analysisPath = ""
	p.score = 0
}

// SetHistory sets the consensus round history
func (p *ConsensusPanel) SetHistory(history []ConsensusRound) {
	p.history = history
}

// AddRound adds a consensus round to history
func (p *ConsensusPanel) AddRound(round int, score float64) {
	p.history = append(p.history, ConsensusRound{Round: round, Score: score})
}

// SetAnalysisPath sets the path to the analysis reports directory
func (p *ConsensusPanel) SetAnalysisPath(path string) {
	p.analysisPath = path
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
	return len(p.pairScores) > 0 || p.score > 0 || len(p.history) > 0
}

// Render renders the consensus panel
func (p *ConsensusPanel) Render() string {
	if !p.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2).
		Width(p.width - 2)

	// If no data, show empty state
	if !p.HasData() {
		emptyContent := headerStyle.Render("◆ Quorum") + "\n\n" +
			dimStyle.Render("No quorum data yet.\n\n") +
			dimStyle.Render("Run /analyze or /run to see quorum results\n") +
			dimStyle.Render("between multiple agents.") + "\n\n" +
			dimStyle.Render("Press Ctrl+Q or Esc to close")
		return boxStyle.Render(emptyContent)
	}

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
	sb.WriteString(headerStyle.Render("◆ Quorum"))
	sb.WriteString("\n\n")

	// Progress bar
	barWidth := p.width - 30
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
	sb.WriteString("\n\n")

	// Pair scores (if expanded and available)
	if p.expanded && len(p.pairScores) > 0 {
		sb.WriteString(dimStyle.Render("Agent pairs:"))
		sb.WriteString("\n")
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
			sb.WriteString(fmt.Sprintf("  %s: %s\n", pair, pairStyle.Render(fmt.Sprintf("%.0f%%", score))))
		}
		sb.WriteString("\n")
	}

	// Consensus history (previous rounds)
	if len(p.history) > 0 {
		sb.WriteString(dimStyle.Render("Round history:"))
		sb.WriteString("\n")
		for _, round := range p.history {
			var roundColor lipgloss.Color
			switch {
			case round.Score >= p.threshold:
				roundColor = lipgloss.Color("#22c55e")
			case round.Score >= 60:
				roundColor = lipgloss.Color("#eab308")
			default:
				roundColor = lipgloss.Color("#ef4444")
			}
			roundStyle := lipgloss.NewStyle().Foreground(roundColor)
			icon := "○"
			if round.Score >= p.threshold {
				icon = "●"
			}
			sb.WriteString(fmt.Sprintf("  %s V%d: %s\n", icon, round.Round, roundStyle.Render(fmt.Sprintf("%.0f%%", round.Score))))
		}
		sb.WriteString("\n")
	}

	// Analysis path
	if p.analysisPath != "" {
		sb.WriteString(dimStyle.Render("Analysis: "))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa"))
		sb.WriteString(pathStyle.Render(p.analysisPath))
		sb.WriteString("\n\n")
	}

	sb.WriteString(dimStyle.Render("Press Ctrl+Q or Esc to close"))

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
