package events

// Event type constants for consensus events.
const (
	TypeConsensusEvaluated = "consensus_evaluated"
)

// DivergenceDetail provides structured information about a divergence.
type DivergenceDetail struct {
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Agent1       string   `json:"agent1"`
	Agent1Items  []string `json:"agent1_items"`
	Agent2       string   `json:"agent2"`
	Agent2Items  []string `json:"agent2_items"`
	JaccardScore float64  `json:"jaccard_score"`
	Severity     string   `json:"severity"` // low, medium, high
}

// ConsensusEvaluatedEvent is emitted after consensus evaluation.
// If NeedsHumanReview is true, this is a PRIORITY event.
type ConsensusEvaluatedEvent struct {
	BaseEvent
	Score            float64            `json:"score"`
	NeedsV2          bool               `json:"needs_v2"`
	NeedsV3          bool               `json:"needs_v3"`
	NeedsHumanReview bool               `json:"needs_human_review"`
	CategoryScores   map[string]float64 `json:"category_scores"`
	Divergences      []DivergenceDetail `json:"divergences"`
}

// NewConsensusEvaluatedEvent creates a new consensus evaluated event.
func NewConsensusEvaluatedEvent(workflowID string, score float64, needsV2, needsV3, needsHuman bool, categoryScores map[string]float64, divergences []DivergenceDetail) ConsensusEvaluatedEvent {
	return ConsensusEvaluatedEvent{
		BaseEvent:        NewBaseEvent(TypeConsensusEvaluated, workflowID),
		Score:            score,
		NeedsV2:          needsV2,
		NeedsV3:          needsV3,
		NeedsHumanReview: needsHuman,
		CategoryScores:   categoryScores,
		Divergences:      divergences,
	}
}
