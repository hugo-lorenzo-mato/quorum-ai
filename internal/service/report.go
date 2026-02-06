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
	fmt.Fprintf(w, "  Arbiter Rounds:  %d\n", wm.ArbiterRounds)
	fmt.Fprintln(w, "")

	// Token usage
	fmt.Fprintln(w, "TOKEN USAGE")
	fmt.Fprintln(w, strings.Repeat("-", 40))
	fmt.Fprintf(w, "  Input Tokens:    %d\n", wm.TotalTokensIn)
	fmt.Fprintf(w, "  Output Tokens:   %d\n", wm.TotalTokensOut)
	fmt.Fprintf(w, "  Total Tokens:    %d\n", wm.TotalTokensIn+wm.TotalTokensOut)
	fmt.Fprintln(w, "")

	// Agent breakdown
	if err := r.writeAgentTable(w); err != nil {
		return err
	}

	// Task breakdown
	if err := r.writeTaskTable(w); err != nil {
		return err
	}

	// Consensus summary
	r.writeConsensusSummary(w)

	fmt.Fprintln(w, strings.Repeat("=", 60))

	return nil
}

// writeAgentTable writes the agent metrics table.
func (r *ReportGenerator) writeAgentTable(w io.Writer) error {
	agents := r.metrics.GetAgentMetrics()
	if len(agents) == 0 {
		return nil
	}

	fmt.Fprintln(w, "AGENT METRICS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Agent\tCalls\tTokens\tAvg Time")
	fmt.Fprintln(tw, "  -----\t-----\t------\t--------")

	for _, am := range agents {
		fmt.Fprintf(tw, "  %s\t%d\t%d\t%s\n",
			am.Name,
			am.Invocations,
			am.TotalTokensIn+am.TotalTokensOut,
			am.AvgDuration.Round(time.Millisecond),
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w, "")
	return nil
}

// writeTaskTable writes the task metrics table.
func (r *ReportGenerator) writeTaskTable(w io.Writer) error {
	tasks := r.metrics.GetAllTaskMetrics()
	if len(tasks) == 0 {
		return nil
	}

	fmt.Fprintln(w, "TASK METRICS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Task\tPhase\tStatus\tDuration")
	fmt.Fprintln(tw, "  ----\t-----\t------\t--------")

	for _, tm := range tasks {
		status := "✓"
		if !tm.Success {
			status = "✗"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
			truncate(tm.Name, 20),
			tm.Phase,
			status,
			tm.Duration.Round(time.Millisecond),
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w, "")
	return nil
}

// writeConsensusSummary writes arbiter evaluation summary.
func (r *ReportGenerator) writeConsensusSummary(w io.Writer) {
	arbiter := r.metrics.GetArbiterMetrics()
	if len(arbiter) == 0 {
		return
	}

	fmt.Fprintln(w, "ARBITER EVALUATIONS")
	fmt.Fprintln(w, strings.Repeat("-", 40))

	for _, am := range arbiter {
		fmt.Fprintf(w, "  Phase: %s (Round %d)\n", am.Phase, am.Round)
		fmt.Fprintf(w, "    Score:        %.2f%%\n", am.Score*100)
		fmt.Fprintf(w, "    Agreements:   %d\n", am.AgreementCount)
		fmt.Fprintf(w, "    Divergences:  %d\n", am.DivergenceCount)
		fmt.Fprintf(w, "    Tokens:       %d in / %d out\n", am.TokensIn, am.TokensOut)
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
		Arbiter:     r.metrics.GetArbiterMetrics(),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// GenerateSummary generates a brief summary string.
func (r *ReportGenerator) GenerateSummary() string {
	wm := r.metrics.GetWorkflowMetrics()

	return fmt.Sprintf(
		"Duration: %s | Tasks: %d/%d | Arbiter: %d rounds",
		wm.TotalDuration.Round(time.Second),
		wm.TasksCompleted,
		wm.TasksTotal,
		wm.ArbiterRounds,
	)
}

// Report represents a generated report.
type Report struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Workflow    WorkflowMetrics          `json:"workflow"`
	Agents      map[string]*AgentMetrics `json:"agents"`
	Tasks       []*TaskMetrics           `json:"tasks"`
	Arbiter     []ArbiterMetrics         `json:"arbiter"`
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
