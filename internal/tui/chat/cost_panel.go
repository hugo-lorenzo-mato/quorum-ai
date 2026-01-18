package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AgentCost tracks cost for a single agent
type AgentCost struct {
	Name     string
	Cost     float64
	Tokens   int
	Requests int
}

// CostPanel displays real-time cost tracking
type CostPanel struct {
	agents       map[string]*AgentCost
	agentOrder   []string // To maintain consistent ordering
	budget       float64
	alertAt80    bool // Whether we've shown 80% alert
	alertAt95    bool // Whether we've shown 95% alert
	width        int
	height       int
	visible      bool
}

// NewCostPanel creates a new cost tracking panel
func NewCostPanel(budget float64) *CostPanel {
	if budget <= 0 {
		budget = 1.0 // Default $1 budget
	}
	return &CostPanel{
		agents:     make(map[string]*AgentCost),
		agentOrder: make([]string, 0),
		budget:     budget,
		visible:    false,
	}
}

// SetBudget sets the budget limit
func (p *CostPanel) SetBudget(budget float64) {
	p.budget = budget
}

// AddCost adds cost for an agent
func (p *CostPanel) AddCost(agent string, cost float64, tokens int) {
	if _, exists := p.agents[agent]; !exists {
		p.agents[agent] = &AgentCost{Name: agent}
		p.agentOrder = append(p.agentOrder, agent)
	}
	p.agents[agent].Cost += cost
	p.agents[agent].Tokens += tokens
	p.agents[agent].Requests++
}

// SetAgentCost sets the total cost for an agent
func (p *CostPanel) SetAgentCost(agent string, cost float64, tokens int) {
	if _, exists := p.agents[agent]; !exists {
		p.agents[agent] = &AgentCost{Name: agent}
		p.agentOrder = append(p.agentOrder, agent)
	}
	p.agents[agent].Cost = cost
	p.agents[agent].Tokens = tokens
}

// GetTotalCost returns the total cost across all agents
func (p *CostPanel) GetTotalCost() float64 {
	total := 0.0
	for _, agent := range p.agents {
		total += agent.Cost
	}
	return total
}

// GetTotalTokens returns the total tokens used
func (p *CostPanel) GetTotalTokens() int {
	total := 0
	for _, agent := range p.agents {
		total += agent.Tokens
	}
	return total
}

// GetBudgetUsage returns the percentage of budget used
func (p *CostPanel) GetBudgetUsage() float64 {
	if p.budget <= 0 {
		return 0
	}
	return p.GetTotalCost() / p.budget * 100
}

// IsOverBudget returns true if over budget
func (p *CostPanel) IsOverBudget() bool {
	return p.GetTotalCost() >= p.budget
}

// ShouldAlert80 returns true if we should show 80% alert (only once)
func (p *CostPanel) ShouldAlert80() bool {
	if !p.alertAt80 && p.GetBudgetUsage() >= 80 {
		p.alertAt80 = true
		return true
	}
	return false
}

// ShouldAlert95 returns true if we should show 95% alert (only once)
func (p *CostPanel) ShouldAlert95() bool {
	if !p.alertAt95 && p.GetBudgetUsage() >= 95 {
		p.alertAt95 = true
		return true
	}
	return false
}

// Reset resets all costs
func (p *CostPanel) Reset() {
	p.agents = make(map[string]*AgentCost)
	p.agentOrder = make([]string, 0)
	p.alertAt80 = false
	p.alertAt95 = false
}

// Toggle toggles visibility
func (p *CostPanel) Toggle() {
	p.visible = !p.visible
}

// IsVisible returns visibility
func (p *CostPanel) IsVisible() bool {
	return p.visible
}

// SetSize sets the panel dimensions
func (p *CostPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Render renders the cost panel
func (p *CostPanel) Render() string {
	if !p.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")). // Emerald
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	agentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e5e7eb"))

	costStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")) // Amber

	totalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Bold(true)

	// Budget bar colors based on usage
	usage := p.GetBudgetUsage()
	var barColor lipgloss.Color
	switch {
	case usage >= 95:
		barColor = lipgloss.Color("#ef4444") // Red
	case usage >= 80:
		barColor = lipgloss.Color("#eab308") // Yellow
	default:
		barColor = lipgloss.Color("#22c55e") // Green
	}

	barStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	var sb strings.Builder

	// Header
	header := headerStyle.Render(" Session Cost")
	sb.WriteString(header)
	sb.WriteString("\n")

	// Separator
	sb.WriteString(dimStyle.Render(strings.Repeat("─", p.width-6)))
	sb.WriteString("\n")

	// Agent costs
	for _, agentName := range p.agentOrder {
		agent := p.agents[agentName]
		name := agentStyle.Render(fmt.Sprintf("%-10s", agent.Name+":"))
		cost := costStyle.Render(fmt.Sprintf("$%.4f", agent.Cost))
		sb.WriteString(name)
		sb.WriteString(cost)
		sb.WriteString("\n")
	}

	// Separator before total
	if len(p.agentOrder) > 0 {
		sb.WriteString(dimStyle.Render(strings.Repeat("─", p.width-6)))
		sb.WriteString("\n")
	}

	// Total
	totalLabel := totalStyle.Render("Total:    ")
	totalCost := totalStyle.Render(fmt.Sprintf("$%.4f", p.GetTotalCost()))
	sb.WriteString(totalLabel)
	sb.WriteString(totalCost)
	sb.WriteString("\n")

	// Budget
	budgetLabel := dimStyle.Render("Budget:   ")
	budgetValue := agentStyle.Render(fmt.Sprintf("$%.4f", p.budget))
	sb.WriteString(budgetLabel)
	sb.WriteString(budgetValue)
	sb.WriteString("\n")

	// Progress bar
	barWidth := p.width - 12
	if barWidth < 10 {
		barWidth = 10
	}
	filled := int(usage * float64(barWidth) / 100)
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		emptyBarStyle.Render(strings.Repeat("░", barWidth-filled))

	sb.WriteString(bar)
	sb.WriteString(" ")
	sb.WriteString(barStyle.Render(fmt.Sprintf("%.1f%%", usage)))

	// Warning if near/over budget
	if usage >= 95 {
		sb.WriteString("\n")
		warnStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444")).
			Bold(true)
		sb.WriteString(warnStyle.Render("⚠ NEAR BUDGET LIMIT!"))
	} else if usage >= 80 {
		sb.WriteString("\n")
		warnStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eab308"))
		sb.WriteString(warnStyle.Render("⚠ 80% of budget used"))
	}

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#10b981")). // Green border
		BorderBackground(lipgloss.Color("#1f1f23")).
		Background(lipgloss.Color("#1f1f23")).
		Padding(0, 1).
		Width(p.width - 2)

	return boxStyle.Render(sb.String())
}

// CompactRender renders a one-line summary
func (p *CostPanel) CompactRender() string {
	usage := p.GetBudgetUsage()

	// Color based on usage
	var color lipgloss.Color
	switch {
	case usage >= 95:
		color = lipgloss.Color("#ef4444")
	case usage >= 80:
		color = lipgloss.Color("#eab308")
	default:
		color = lipgloss.Color("#22c55e")
	}

	style := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	return dimStyle.Render("$") + style.Render(fmt.Sprintf("%.2f", p.GetTotalCost())) +
		dimStyle.Render("/") + dimStyle.Render(fmt.Sprintf("%.2f", p.budget))
}
