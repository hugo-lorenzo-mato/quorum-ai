package chat

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// PhaseStatus represents the status of a workflow phase
type PhaseStatus int

const (
	PhasePending PhaseStatus = iota
	PhaseActive
	PhaseComplete
	PhaseError
	PhaseSkipped
)

// WorkflowPhase represents a single phase in the workflow
type WorkflowPhase struct {
	Name      string
	Status    PhaseStatus
	SubPhase  string // e.g., "V1", "V2", "V3" for Analyze
	StartTime time.Time
	EndTime   time.Time
	Error     string
}

// WorkflowProgress tracks the progress of a workflow
type WorkflowProgress struct {
	phases       []WorkflowPhase
	currentPhase int
	width        int
	compact      bool // Compact single-line mode
	visible      bool
}

// NewWorkflowProgress creates a new workflow progress tracker
func NewWorkflowProgress() *WorkflowProgress {
	return &WorkflowProgress{
		phases: []WorkflowPhase{
			{Name: "Optimize", Status: PhasePending},
			{Name: "Analyze", Status: PhasePending},
			{Name: "Plan", Status: PhasePending},
			{Name: "Execute", Status: PhasePending},
		},
		currentPhase: -1,
		compact:      false,
		visible:      true,
	}
}

// Reset resets the progress to initial state
func (p *WorkflowProgress) Reset() {
	for i := range p.phases {
		p.phases[i].Status = PhasePending
		p.phases[i].SubPhase = ""
		p.phases[i].StartTime = time.Time{}
		p.phases[i].EndTime = time.Time{}
		p.phases[i].Error = ""
	}
	p.currentPhase = -1
}

// StartPhase starts a phase
func (p *WorkflowProgress) StartPhase(phaseName string) {
	for i := range p.phases {
		if strings.EqualFold(p.phases[i].Name, phaseName) {
			p.phases[i].Status = PhaseActive
			p.phases[i].StartTime = time.Now()
			p.currentPhase = i
			return
		}
	}
}

// SetSubPhase sets the sub-phase (e.g., V1, V2, V3)
func (p *WorkflowProgress) SetSubPhase(subPhase string) {
	if p.currentPhase >= 0 && p.currentPhase < len(p.phases) {
		p.phases[p.currentPhase].SubPhase = subPhase
	}
}

// CompletePhase marks a phase as complete
func (p *WorkflowProgress) CompletePhase(phaseName string) {
	for i := range p.phases {
		if strings.EqualFold(p.phases[i].Name, phaseName) {
			p.phases[i].Status = PhaseComplete
			p.phases[i].EndTime = time.Now()
			return
		}
	}
}

// FailPhase marks a phase as failed
func (p *WorkflowProgress) FailPhase(phaseName string, err string) {
	for i := range p.phases {
		if strings.EqualFold(p.phases[i].Name, phaseName) {
			p.phases[i].Status = PhaseError
			p.phases[i].EndTime = time.Now()
			p.phases[i].Error = err
			return
		}
	}
}

// SkipPhase marks a phase as skipped
func (p *WorkflowProgress) SkipPhase(phaseName string) {
	for i := range p.phases {
		if strings.EqualFold(p.phases[i].Name, phaseName) {
			p.phases[i].Status = PhaseSkipped
			return
		}
	}
}

// GetCurrentPhase returns the current phase name
func (p *WorkflowProgress) GetCurrentPhase() string {
	if p.currentPhase >= 0 && p.currentPhase < len(p.phases) {
		return p.phases[p.currentPhase].Name
	}
	return ""
}

// GetElapsedTime returns elapsed time for current phase
func (p *WorkflowProgress) GetElapsedTime() time.Duration {
	if p.currentPhase >= 0 && p.currentPhase < len(p.phases) {
		phase := p.phases[p.currentPhase]
		if phase.Status == PhaseActive && !phase.StartTime.IsZero() {
			return time.Since(phase.StartTime)
		}
	}
	return 0
}

// SetWidth sets the render width
func (p *WorkflowProgress) SetWidth(width int) {
	p.width = width
}

// SetCompact sets compact mode
func (p *WorkflowProgress) SetCompact(compact bool) {
	p.compact = compact
}

// Toggle toggles visibility
func (p *WorkflowProgress) Toggle() {
	p.visible = !p.visible
}

// IsVisible returns visibility
func (p *WorkflowProgress) IsVisible() bool {
	return p.visible
}

// IsActive returns true if any phase is active
func (p *WorkflowProgress) IsActive() bool {
	for _, phase := range p.phases {
		if phase.Status == PhaseActive {
			return true
		}
	}
	return false
}

// Render renders the workflow progress
func (p *WorkflowProgress) Render() string {
	if !p.visible {
		return ""
	}

	if p.compact {
		return p.renderCompact()
	}
	return p.renderFull()
}

// renderFull renders the full two-line progress
func (p *WorkflowProgress) renderFull() string {
	// Styles
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	activeNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308")).Bold(true)
	completeNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	errorNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	connectorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	activeConnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))

	var line1, line2 strings.Builder

	for i, phase := range p.phases {
		// Phase name
		var phaseName string
		var statusIcon string
		var subPhaseStr string

		switch phase.Status {
		case PhasePending:
			phaseName = nameStyle.Render(phase.Name)
			statusIcon = dimStyle.Render("○")
		case PhaseActive:
			phaseName = activeNameStyle.Render(phase.Name)
			statusIcon = activeNameStyle.Render("◐")
			if phase.SubPhase != "" {
				subPhaseStr = dimStyle.Render(" " + phase.SubPhase)
			}
		case PhaseComplete:
			phaseName = completeNameStyle.Render(phase.Name)
			statusIcon = completeNameStyle.Render("✓")
		case PhaseError:
			phaseName = errorNameStyle.Render(phase.Name)
			statusIcon = errorNameStyle.Render("✗")
		case PhaseSkipped:
			phaseName = dimStyle.Render(phase.Name)
			statusIcon = dimStyle.Render("⊘")
		}

		// Line 1: phase names with connectors
		line1.WriteString(phaseName)

		// Line 2: status icons
		// Calculate padding to align icon under name
		nameWidth := lipgloss.Width(phaseName)
		iconPadding := (nameWidth - 1) / 2
		line2.WriteString(strings.Repeat(" ", iconPadding))
		line2.WriteString(statusIcon)
		line2.WriteString(subPhaseStr)
		remainingPad := nameWidth - iconPadding - 1 - lipgloss.Width(subPhaseStr)
		if remainingPad > 0 {
			line2.WriteString(strings.Repeat(" ", remainingPad))
		}

		// Connector between phases
		if i < len(p.phases)-1 {
			connector := " ━━━━━━━━ "
			if phase.Status == PhaseComplete {
				line1.WriteString(activeConnStyle.Render(connector))
			} else {
				line1.WriteString(connectorStyle.Render(connector))
			}
			line2.WriteString(strings.Repeat(" ", len(connector)))
		}
	}

	return line1.String() + "\n" + line2.String()
}

// renderCompact renders a single-line compact version
func (p *WorkflowProgress) renderCompact() string {
	// Styles
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#eab308")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1).
		Bold(true)
	completeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e")).
		Padding(0, 1)
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444")).
		Padding(0, 1)
	pendingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Padding(0, 1)

	var parts []string

	for _, phase := range p.phases {
		var icon string
		var style lipgloss.Style
		name := phase.Name[:3] // Abbreviate to 3 chars

		switch phase.Status {
		case PhasePending:
			icon = "○"
			style = pendingStyle
		case PhaseActive:
			icon = "◐"
			style = activeStyle
			if phase.SubPhase != "" {
				name = phase.Name[:3] + ":" + phase.SubPhase
			}
		case PhaseComplete:
			icon = "✓"
			style = completeStyle
		case PhaseError:
			icon = "✗"
			style = errorStyle
		case PhaseSkipped:
			icon = "⊘"
			style = pendingStyle
		}

		parts = append(parts, style.Render(icon+" "+name))
	}

	return strings.Join(parts, dimStyle.Render(" → "))
}

// GetProgress returns the overall progress percentage
func (p *WorkflowProgress) GetProgress() float64 {
	if len(p.phases) == 0 {
		return 0
	}

	completed := 0
	for _, phase := range p.phases {
		if phase.Status == PhaseComplete || phase.Status == PhaseSkipped {
			completed++
		}
	}

	return float64(completed) / float64(len(p.phases)) * 100
}
