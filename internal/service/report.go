package service

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// ReportGenerator generates workflow reports.
type ReportGenerator struct {
	metrics *MetricsCollector
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator(metrics *MetricsCollector) *ReportGenerator {
	return &ReportGenerator{metrics: metrics}
}

// GenerateTextReport generates a text report.
func (r *ReportGenerator) GenerateTextReport(w io.Writer) error {
	wm := r.metrics.GetWorkflowMetrics()

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, strings.Repeat("=", 60))
	fmt.Fprintln(w, "WORKFLOW REPORT")
	fmt.Fprintln(w, strings.Repeat("=", 60))
	fmt.Fprintln(w, "")

	// Summary
	fmt.Fprintln(w, "SUMMARY")
	fmt.Fprintln(w, strings.Repeat("-", 40))
	fmt.Fprintf(w, "  Duration:        %s\n", wm.TotalDuration.Round(time.Second))
	fmt.Fprintf(w, "  Tasks Total:     %d\n", wm.TasksTotal)
	fmt.Fprintf(w, "  Tasks Completed: %d\n", wm.TasksCompleted)
	fmt.Fprintf(w, "  Tasks Failed:    %d\n", wm.TasksFailed)
	fmt.Fprintf(w, "  Tasks Skipped:   %d\n", wm.TasksSkipped)
	fmt.Fprintf(w, "  Retries:         %d\n", wm.RetriesTotal)
	fmt.Fprintf(w, "  V3 Invocations:  %d\n", wm.V3Invocations)
	fmt.Fprintln(w, "")

	// Token usage
	fmt.Fprintln(w, "TOKEN USAGE")
	fmt.Fprintln(w, strings.Repeat("-", 40))
	fmt.Fprintf(w, "  Input Tokens:    %d\n", wm.TotalTokensIn)
	fmt.Fprintf(w, "  Output Tokens:   %d\n", wm.TotalTokensOut)
	fmt.Fprintf(w, "  Total Tokens:    %d\n", wm.TotalTokensIn+wm.TotalTokensOut)
	fmt.Fprintln(w, "")

	// Cost
	fmt.Fprintln(w, "COST")
	fmt.Fprintln(w, strings.Repeat("-", 40))
	fmt.Fprintf(w, "  Total Cost:      $%.4f\n", wm.TotalCostUSD)
	fmt.Fprintln(w, "")

	// Agent breakdown
	r.writeAgentTable(w)

	// Task breakdown
	r.writeTaskTable(w)

	// Consensus summary
	r.writeConsensusSummary(w)

	fmt.Fprintln(w, strings.Repeat("=", 60))

	return nil
}

// writeAgentTable writes the agent metrics table.
func (r *ReportGenerator) writeAgentTable(w io.Writer) {
	agents := r.metrics.GetAgentMetrics()
	if len(agents) == 0 {
		return
	}

	fmt.Fprintln(w, "AGENT METRICS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Agent\tCalls\tTokens\tCost\tAvg Time")
	fmt.Fprintln(tw, "  -----\t-----\t------\t----\t--------")

	for _, am := range agents {
		fmt.Fprintf(tw, "  %s\t%d\t%d\t$%.4f\t%s\n",
			am.Name,
			am.Invocations,
			am.TotalTokensIn+am.TotalTokensOut,
			am.TotalCostUSD,
			am.AvgDuration.Round(time.Millisecond),
		)
	}
	tw.Flush()
	fmt.Fprintln(w, "")
}

// writeTaskTable writes the task metrics table.
func (r *ReportGenerator) writeTaskTable(w io.Writer) {
	tasks := r.metrics.GetAllTaskMetrics()
	if len(tasks) == 0 {
		return
	}

	fmt.Fprintln(w, "TASK METRICS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Task\tPhase\tStatus\tDuration\tCost")
	fmt.Fprintln(tw, "  ----\t-----\t------\t--------\t----")

	for _, tm := range tasks {
		status := "✓"
		if !tm.Success {
			status = "✗"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t$%.4f\n",
			truncate(tm.Name, 20),
			tm.Phase,
			status,
			tm.Duration.Round(time.Millisecond),
			tm.CostUSD,
		)
	}
	tw.Flush()
	fmt.Fprintln(w, "")
}

// writeConsensusSummary writes consensus summary.
func (r *ReportGenerator) writeConsensusSummary(w io.Writer) {
	consensus := r.metrics.GetConsensusMetrics()
	if len(consensus) == 0 {
		return
	}

	fmt.Fprintln(w, "CONSENSUS EVALUATIONS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	for _, cm := range consensus {
		fmt.Fprintf(w, "  Phase: %s\n", cm.Phase)
		fmt.Fprintf(w, "    Score:        %.2f%%\n", cm.Score*100)
		fmt.Fprintf(w, "    Claims:       %.2f%%\n", cm.ClaimsScore*100)
		fmt.Fprintf(w, "    Risks:        %.2f%%\n", cm.RisksScore*100)
		fmt.Fprintf(w, "    Recs:         %.2f%%\n", cm.RecsScore*100)
		fmt.Fprintf(w, "    V3 Required:  %v\n", cm.V3Required)
		fmt.Fprintf(w, "    Divergences:  %d\n", cm.DivergenceCount)
		fmt.Fprintln(w, "")
	}
}

// GenerateJSONReport generates a JSON report.
func (r *ReportGenerator) GenerateJSONReport(w io.Writer) error {
	report := Report{
		GeneratedAt: time.Now(),
		Workflow:    r.metrics.GetWorkflowMetrics(),
		Agents:      r.metrics.GetAgentMetrics(),
		Tasks:       r.metrics.GetAllTaskMetrics(),
		Consensus:   r.metrics.GetConsensusMetrics(),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// GenerateSummary generates a brief summary string.
func (r *ReportGenerator) GenerateSummary() string {
	wm := r.metrics.GetWorkflowMetrics()

	return fmt.Sprintf(
		"Duration: %s | Tasks: %d/%d | Cost: $%.4f | V3: %d",
		wm.TotalDuration.Round(time.Second),
		wm.TasksCompleted,
		wm.TasksTotal,
		wm.TotalCostUSD,
		wm.V3Invocations,
	)
}

// Report represents a generated report.
type Report struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Workflow    WorkflowMetrics          `json:"workflow"`
	Agents      map[string]*AgentMetrics `json:"agents"`
	Tasks       []*TaskMetrics           `json:"tasks"`
	Consensus   []ConsensusMetrics       `json:"consensus"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
