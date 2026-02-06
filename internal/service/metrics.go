package service

import (
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// MetricsCollector collects workflow metrics.
type MetricsCollector struct {
	workflow WorkflowMetrics
	tasks    map[core.TaskID]*TaskMetrics
	agents   map[string]*AgentMetrics
	arbiter  []ArbiterMetrics
	mu       sync.RWMutex
}

// WorkflowMetrics holds workflow-level metrics.
type WorkflowMetrics struct {
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	TotalDuration  time.Duration `json:"total_duration"`
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	TasksTotal     int           `json:"tasks_total"`
	TasksCompleted int           `json:"tasks_completed"`
	TasksFailed    int           `json:"tasks_failed"`
	TasksSkipped   int           `json:"tasks_skipped"`
	RetriesTotal   int           `json:"retries_total"`
	ArbiterRounds  int           `json:"arbiter_rounds"`
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
	TotalDuration  time.Duration `json:"total_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	Errors         int           `json:"errors"`
	AvgTokensIn    int           `json:"avg_tokens_in"`
	AvgTokensOut   int           `json:"avg_tokens_out"`
}

// ArbiterMetrics holds arbiter evaluation metrics.
type ArbiterMetrics struct {
	Phase           core.Phase `json:"phase"`
	Round           int        `json:"round"`
	Score           float64    `json:"score"`
	DivergenceCount int        `json:"divergence_count"`
	AgreementCount  int        `json:"agreement_count"`
	TokensIn        int        `json:"tokens_in"`
	TokensOut       int        `json:"tokens_out"`
	DurationMS      int64      `json:"duration_ms"`
	Timestamp       time.Time  `json:"timestamp"`
}

// ArbiterMetricsInput is the input for recording arbiter evaluations.
type ArbiterMetricsInput struct {
	Score           float64
	DivergenceCount int
	AgreementCount  int
	TokensIn        int
	TokensOut       int
	DurationMS      int64
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		tasks:   make(map[core.TaskID]*TaskMetrics),
		agents:  make(map[string]*AgentMetrics),
		arbiter: make([]ArbiterMetrics, 0),
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

		m.workflow.TotalTokensIn += result.TokensIn
		m.workflow.TotalTokensOut += result.TokensOut

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

// RecordArbiterEvaluation records an arbiter evaluation.
func (m *MetricsCollector) RecordArbiterEvaluation(input ArbiterMetricsInput, phase core.Phase, round int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	am := ArbiterMetrics{
		Phase:           phase,
		Round:           round,
		Score:           input.Score,
		DivergenceCount: input.DivergenceCount,
		AgreementCount:  input.AgreementCount,
		TokensIn:        input.TokensIn,
		TokensOut:       input.TokensOut,
		DurationMS:      input.DurationMS,
		Timestamp:       time.Now(),
	}

	m.arbiter = append(m.arbiter, am)
	m.workflow.ArbiterRounds++
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
	taskCopy := *tm
	return &taskCopy, true
}

// GetAllTaskMetrics returns metrics for all tasks.
func (m *MetricsCollector) GetAllTaskMetrics() []*TaskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TaskMetrics, 0, len(m.tasks))
	for _, tm := range m.tasks {
		taskCopy := *tm
		result = append(result, &taskCopy)
	}
	return result
}

// GetAgentMetrics returns metrics for all agents.
func (m *MetricsCollector) GetAgentMetrics() map[string]*AgentMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*AgentMetrics)
	for k, v := range m.agents {
		agentCopy := *v
		result[k] = &agentCopy
	}
	return result
}

// GetArbiterMetrics returns arbiter evaluation metrics.
func (m *MetricsCollector) GetArbiterMetrics() []ArbiterMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]ArbiterMetrics{}, m.arbiter...)
}

// Reset clears all metrics.
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workflow = WorkflowMetrics{}
	m.tasks = make(map[core.TaskID]*TaskMetrics)
	m.agents = make(map[string]*AgentMetrics)
	m.arbiter = make([]ArbiterMetrics, 0)
}
