package events

import "time"

// Event type constants for metrics events.
const (
	TypeMetricsUpdate = "metrics_update"
	TypeCostAlert     = "cost_alert"
)

// AlertLevel indicates the severity of a cost alert.
type AlertLevel int

const (
	AlertWarning  AlertLevel = iota // 80% of budget
	AlertCritical                   // 95% of budget
	AlertExceeded                   // 100%+ of budget
)

// String returns the string representation of the alert level.
func (a AlertLevel) String() string {
	switch a {
	case AlertWarning:
		return "warning"
	case AlertCritical:
		return "critical"
	case AlertExceeded:
		return "exceeded"
	default:
		return "unknown"
	}
}

// MetricsUpdateEvent is emitted periodically with current metrics.
type MetricsUpdateEvent struct {
	BaseEvent
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	TotalCostUSD   float64       `json:"total_cost_usd"`
	CostLimit      float64       `json:"cost_limit"`
	UsagePercent   float64       `json:"usage_percent"`
	ConsensusScore float64       `json:"consensus_score"`
	Duration       time.Duration `json:"duration"`
}

// NewMetricsUpdateEvent creates a new metrics update event.
func NewMetricsUpdateEvent(workflowID, projectID string, tokensIn, tokensOut int, cost, limit, consensusScore float64, duration time.Duration) MetricsUpdateEvent {
	usagePercent := 0.0
	if limit > 0 {
		usagePercent = (cost / limit) * 100
	}
	return MetricsUpdateEvent{
		BaseEvent:      NewBaseEvent(TypeMetricsUpdate, workflowID, projectID),
		TotalTokensIn:  tokensIn,
		TotalTokensOut: tokensOut,
		TotalCostUSD:   cost,
		CostLimit:      limit,
		UsagePercent:   usagePercent,
		ConsensusScore: consensusScore,
		Duration:       duration,
	}
}

// CostAlertEvent is emitted when cost thresholds are approached/exceeded.
// This is a PRIORITY event - never dropped.
type CostAlertEvent struct {
	BaseEvent
	CurrentCost float64    `json:"current_cost"`
	Limit       float64    `json:"limit"`
	Level       AlertLevel `json:"level"`
}

// NewCostAlertEvent creates a new cost alert event.
func NewCostAlertEvent(workflowID, projectID string, currentCost, limit float64, level AlertLevel) CostAlertEvent {
	return CostAlertEvent{
		BaseEvent:   NewBaseEvent(TypeCostAlert, workflowID, projectID),
		CurrentCost: currentCost,
		Limit:       limit,
		Level:       level,
	}
}
