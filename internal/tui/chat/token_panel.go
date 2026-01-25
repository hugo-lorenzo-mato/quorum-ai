package chat

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// TokenEntry represents a single token usage line.
type TokenEntry struct {
	Scope     string // chat | workflow
	CLI       string
	Model     string
	Phase     string
	TokensIn  int
	TokensOut int
}

// TokenPanel displays token usage details.
type TokenPanel struct {
	mu       sync.Mutex
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	entries []TokenEntry
}

// NewTokenPanel creates a new token panel.
func NewTokenPanel() *TokenPanel {
	return &TokenPanel{
		entries: make([]TokenEntry, 0),
	}
}

// SetSize updates the panel dimensions.
func (p *TokenPanel) SetSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.width = width
	p.height = height

	// Viewport height: total - header(2) - borders(2)
	viewportHeight := height - 4
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	if !p.ready {
		p.viewport = viewport.New(width-4, viewportHeight)
		p.ready = true
	} else {
		p.viewport.Width = width - 4
		p.viewport.Height = viewportHeight
	}
	p.updateContent()
}

// SetEntries updates token entries.
func (p *TokenPanel) SetEntries(entries []TokenEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = entries
	p.updateContent()
}

// Update handles viewport updates.
func (p *TokenPanel) Update(msg interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready {
		p.viewport, _ = p.viewport.Update(msg)
	}
}

// ScrollUp scrolls the viewport up.
func (p *TokenPanel) ScrollUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollUp(1)
	}
}

// ScrollDown scrolls the viewport down.
func (p *TokenPanel) ScrollDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollDown(1)
	}
}

// PageUp scrolls up by half a page.
func (p *TokenPanel) PageUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.HalfPageUp()
	}
}

// PageDown scrolls down by half a page.
func (p *TokenPanel) PageDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.HalfPageDown()
	}
}

// GotoTop scrolls to the top.
func (p *TokenPanel) GotoTop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoTop()
	}
}

// GotoBottom scrolls to the bottom.
func (p *TokenPanel) GotoBottom() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoBottom()
	}
}

// Width returns panel width.
func (p *TokenPanel) Width() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width
}

// Height returns panel height.
func (p *TokenPanel) Height() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.height
}

// updateContent refreshes the viewport content.
func (p *TokenPanel) updateContent() {
	if !p.ready {
		return
	}

	content := p.renderContent()
	p.viewport.SetContent(content)
}

// renderContent builds the token content.
func (p *TokenPanel) renderContent() string {
	// Styles
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f0fdf4")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))

	innerWidth := p.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	var sb strings.Builder

	if len(p.entries) == 0 {
		sb.WriteString(dimStyle.Render("  No token data yet"))
		return sb.String()
	}

	// Group by scope
	byScope := map[string][]TokenEntry{}
	totalInAll := 0
	totalOutAll := 0
	for _, e := range p.entries {
		byScope[e.Scope] = append(byScope[e.Scope], e)
		totalInAll += e.TokensIn
		totalOutAll += e.TokensOut
	}
	order := []string{"chat", "workflow"}

	for _, scope := range order {
		entries := byScope[scope]
		if len(entries) == 0 {
			continue
		}

		sort.Slice(entries, func(i, j int) bool {
			totalI := entries[i].TokensIn + entries[i].TokensOut
			totalJ := entries[j].TokensIn + entries[j].TokensOut
			if totalI == totalJ {
				return entries[i].CLI < entries[j].CLI
			}
			return totalI > totalJ
		})

		scopeLabel := strings.ToUpper(scope[:1]) + scope[1:]
		sb.WriteString(sectionStyle.Render("" + scopeLabel))
		sb.WriteString("\n")

		// Header line
		header := fmt.Sprintf("  %-6s %-10s %-7s %s",
			labelStyle.Render("CLI"),
			labelStyle.Render("Model"),
			labelStyle.Render("Phase"),
			labelStyle.Render("↑in ↓out"),
		)
		sb.WriteString(truncateToWidth(header, innerWidth))
		sb.WriteString("\n")

		totalIn := 0
		totalOut := 0
		for _, e := range entries {
			totalIn += e.TokensIn
			totalOut += e.TokensOut
			cli := padOrTrim(e.CLI, 6)
			model := padOrTrim(e.Model, 10)
			phase := padOrTrim(e.Phase, 7)
			tokenStr := fmt.Sprintf("↑%s ↓%s", formatTokenCount(e.TokensIn), formatTokenCount(e.TokensOut))
			if lipgloss.Width(tokenStr) > 11 {
				tokenStr = fmt.Sprintf("↑%s↓%s", formatTokenCount(e.TokensIn), formatTokenCount(e.TokensOut))
			}
			line := fmt.Sprintf("  %s %s %s %s",
				labelStyle.Render(cli),
				labelStyle.Render(model),
				labelStyle.Render(phase),
				valueStyle.Render(tokenStr),
			)
			sb.WriteString(truncateToWidth(line, innerWidth))
			sb.WriteString("\n")
		}

		// Scope total
		totalLine := fmt.Sprintf("  %s %s %s %s",
			labelStyle.Render(padOrTrim("TOTAL", 6)),
			labelStyle.Render(padOrTrim("-", 10)),
			labelStyle.Render(padOrTrim("-", 7)),
			valueStyle.Render(fmt.Sprintf("↑%s ↓%s", formatTokenCount(totalIn), formatTokenCount(totalOut))),
		)
		sb.WriteString(truncateToWidth(totalLine, innerWidth))
		sb.WriteString("\n")

		sb.WriteString("\n")
	}

	// Grand total
	grandLine := fmt.Sprintf("  %s %s",
		labelStyle.Render(padOrTrim("ALL", 6)),
		valueStyle.Render(fmt.Sprintf("↑%s ↓%s", formatTokenCount(totalInAll), formatTokenCount(totalOutAll))),
	)
	sb.WriteString(truncateToWidth(grandLine, innerWidth))
	sb.WriteString("\n")

	return strings.TrimRight(sb.String(), "\n")
}

func padOrTrim(s string, width int) string {
	trimmed := truncateToWidth(s, width)
	pad := width - lipgloss.Width(trimmed)
	if pad > 0 {
		trimmed += strings.Repeat(" ", pad)
	}
	return trimmed
}

// Render renders the token panel.
func (p *TokenPanel) Render() string {
	return p.RenderWithFocus(false)
}

// RenderWithFocus renders the token panel with focus indicator.
func (p *TokenPanel) RenderWithFocus(focused bool) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.ready {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee")).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#06b6d4"))

	header := headerStyle.Render("Tokens")

	scrollInfo := ""
	if !p.viewport.AtBottom() {
		scrollPct := int(p.viewport.ScrollPercent() * 100)
		scrollInfo = scrollStyle.Render(fmt.Sprintf(" ↕%d%%", scrollPct))
	}

	help := helpStyle.Render("^T")

	headerWidth := p.width - 4
	gap := headerWidth - lipgloss.Width(header) - lipgloss.Width(scrollInfo) - lipgloss.Width(help)
	if gap < 1 {
		gap = 1
	}
	headerLine := header + scrollInfo + strings.Repeat(" ", gap) + help

	content := p.viewport.View()

	var sb strings.Builder
	sb.WriteString(headerLine)
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("─", p.width-4)))
	sb.WriteString("\n")
	sb.WriteString(content)

	borderColor := lipgloss.Color("#374151")
	if focused {
		borderColor = lipgloss.Color("#22d3ee")
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(p.width - 2).
		Height(p.height - 2)

	return boxStyle.Render(sb.String())
}
