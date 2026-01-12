package service

import (
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// MetricsCollector collects workflow metrics.
type MetricsCollector struct {
	workflow  WorkflowMetrics
	tasks     map[core.TaskID]*TaskMetrics
	agents    map[string]*AgentMetrics
	consensus []ConsensusMetrics
	mu        sync.RWMutex
}

// WorkflowMetrics holds workflow-level metrics.
type WorkflowMetrics struct {
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	TotalDuration  time.Duration `json:"total_duration"`
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	TotalCostUSD   float64       `json:"total_cost_usd"`
	TasksTotal     int           `json:"tasks_total"`
	TasksCompleted int           `json:"tasks_completed"`
	TasksFailed    int           `json:"tasks_failed"`
	TasksSkipped   int           `json:"tasks_skipped"`
	RetriesTotal   int           `json:"retries_total"`
	V3Invocations  int           `json:"v3_invocations"`
}

// TaskMetrics holds task-level metrics.
type TaskMetrics struct {
	TaskID    core.TaskID   `json:"task_id"`
	Name      string        `json:"name"`
	Phase     core.Phase    `json:"phase"`
	Agent     string        `json:"agent"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	CostUSD   float64       `json:"cost_usd"`
	Retries   int           `json:"retries"`
	Success   bool          `json:"success"`
	ErrorMsg  string        `json:"error,omitempty"`
}

// AgentMetrics holds agent-level metrics.
type AgentMetrics struct {
	Name           string        `json:"name"`
	Invocations    int           `json:"invocations"`
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	TotalCostUSD   float64       `json:"total_cost_usd"`
	TotalDuration  time.Duration `json:"total_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	Errors         int           `json:"errors"`
	AvgTokensIn    int           `json:"avg_tokens_in"`
	AvgTokensOut   int           `json:"avg_tokens_out"`
}

// ConsensusMetrics holds consensus evaluation metrics.
type ConsensusMetrics struct {
	Phase           core.Phase `json:"phase"`
	Score           float64    `json:"score"`
	ClaimsScore     float64    `json:"claims_score"`
	RisksScore      float64    `json:"risks_score"`
	RecsScore       float64    `json:"recs_score"`
	V3Required      bool       `json:"v3_required"`
	HumanRequired   bool       `json:"human_required"`
	DivergenceCount int        `json:"divergence_count"`
	Timestamp       time.Time  `json:"timestamp"`
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		tasks:     make(map[core.TaskID]*TaskMetrics),
		agents:    make(map[string]*AgentMetrics),
		consensus: make([]ConsensusMetrics, 0),
	}
}

// StartWorkflow marks workflow start.
func (m *MetricsCollector) StartWorkflow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflow.StartTime = time.Now()
}

// EndWorkflow marks workflow end.
func (m *MetricsCollector) EndWorkflow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflow.EndTime = time.Now()
	m.workflow.TotalDuration = m.workflow.EndTime.Sub(m.workflow.StartTime)
}

// StartTask starts tracking a task.
func (m *MetricsCollector) StartTask(task *core.Task, agent string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = &TaskMetrics{
		TaskID:    task.ID,
		Name:      task.Name,
		Phase:     task.Phase,
		Agent:     agent,
		StartTime: time.Now(),
	}
	m.workflow.TasksTotal++
}

// EndTask ends tracking a task.
func (m *MetricsCollector) EndTask(taskID core.TaskID, result *core.ExecuteResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tm, ok := m.tasks[taskID]
	if !ok {
		return
	}

	tm.EndTime = time.Now()
	tm.Duration = tm.EndTime.Sub(tm.StartTime)

	if result != nil {
		tm.TokensIn = result.TokensIn
		tm.TokensOut = result.TokensOut
		tm.CostUSD = result.CostUSD

		m.workflow.TotalTokensIn += result.TokensIn
		m.workflow.TotalTokensOut += result.TokensOut
		m.workflow.TotalCostUSD += result.CostUSD

		// Update agent metrics
		m.updateAgentMetrics(tm.Agent, result, tm.Duration, err != nil)
	}

	if err != nil {
		tm.Success = false
		tm.ErrorMsg = err.Error()
		m.workflow.TasksFailed++
	} else {
		tm.Success = true
		m.workflow.TasksCompleted++
	}
}

// RecordRetry records a task retry.
func (m *MetricsCollector) RecordRetry(taskID core.TaskID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tm, ok := m.tasks[taskID]; ok {
		tm.Retries++
	}
	m.workflow.RetriesTotal++
}

// RecordConsensus records consensus evaluation.
func (m *MetricsCollector) RecordConsensus(result ConsensusResult, phase core.Phase) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cm := ConsensusMetrics{
		Phase:           phase,
		Score:           result.Score,
		V3Required:      result.NeedsV3,
		HumanRequired:   result.NeedsHumanReview,
		DivergenceCount: len(result.Divergences),
		Timestamp:       time.Now(),
	}

	if scores, ok := result.CategoryScores["claims"]; ok {
		cm.ClaimsScore = scores
	}
	if scores, ok := result.CategoryScores["risks"]; ok {
		cm.RisksScore = scores
	}
	if scores, ok := result.CategoryScores["recommendations"]; ok {
		cm.RecsScore = scores
	}

	m.consensus = append(m.consensus, cm)

	if result.NeedsV3 {
		m.workflow.V3Invocations++
	}
}

// RecordSkipped records a skipped task.
func (m *MetricsCollector) RecordSkipped(_ core.TaskID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflow.TasksSkipped++
}

// updateAgentMetrics updates agent-level metrics.
func (m *MetricsCollector) updateAgentMetrics(agent string, result *core.ExecuteResult, duration time.Duration, isError bool) {
	am, ok := m.agents[agent]
	if !ok {
		am = &AgentMetrics{Name: agent}
		m.agents[agent] = am
	}

	am.Invocations++
	am.TotalTokensIn += result.TokensIn
	am.TotalTokensOut += result.TokensOut
	am.TotalCostUSD += result.CostUSD
	am.TotalDuration += duration
	am.AvgDuration = am.TotalDuration / time.Duration(am.Invocations)
	am.AvgTokensIn = am.TotalTokensIn / am.Invocations
	am.AvgTokensOut = am.TotalTokensOut / am.Invocations

	if isError {
		am.Errors++
	}
}

// GetWorkflowMetrics returns workflow metrics.
func (m *MetricsCollector) GetWorkflowMetrics() WorkflowMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workflow
}

// GetTaskMetrics returns metrics for a specific task.
func (m *MetricsCollector) GetTaskMetrics(taskID core.TaskID) (*TaskMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tm, ok := m.tasks[taskID]
	if !ok {
		return nil, false
	}
	copy := *tm
	return &copy, true
}

// GetAllTaskMetrics returns metrics for all tasks.
func (m *MetricsCollector) GetAllTaskMetrics() []*TaskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TaskMetrics, 0, len(m.tasks))
	for _, tm := range m.tasks {
		copy := *tm
		result = append(result, &copy)
	}
	return result
}

// GetAgentMetrics returns metrics for all agents.
func (m *MetricsCollector) GetAgentMetrics() map[string]*AgentMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*AgentMetrics)
	for k, v := range m.agents {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetConsensusMetrics returns consensus metrics.
func (m *MetricsCollector) GetConsensusMetrics() []ConsensusMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]ConsensusMetrics{}, m.consensus...)
}

// Reset clears all metrics.
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workflow = WorkflowMetrics{}
	m.tasks = make(map[core.TaskID]*TaskMetrics)
	m.agents = make(map[string]*AgentMetrics)
	m.consensus = make([]ConsensusMetrics, 0)
}
