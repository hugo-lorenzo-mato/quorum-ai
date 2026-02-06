package events

import "time"

// Event type constants for metrics events.
const (
	TypeMetricsUpdate = "metrics_update"
)

// MetricsUpdateEvent is emitted periodically with current metrics.
type MetricsUpdateEvent struct {
	BaseEvent
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	ConsensusScore float64       `json:"consensus_score"`
	Duration       time.Duration `json:"duration"`
}

// NewMetricsUpdateEvent creates a new metrics update event.
func NewMetricsUpdateEvent(workflowID, projectID string, tokensIn, tokensOut int, consensusScore float64, duration time.Duration) MetricsUpdateEvent {
	return MetricsUpdateEvent{
		BaseEvent:      NewBaseEvent(TypeMetricsUpdate, workflowID, projectID),
		TotalTokensIn:  tokensIn,
		TotalTokensOut: tokensOut,
		ConsensusScore: consensusScore,
		Duration:       duration,
	}
}
