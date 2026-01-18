package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// agentUsage holds aggregated metrics per agent.
type agentUsage struct {
	Name      string
	Model     string
	TokensIn  int
	TokensOut int
	Tasks     int
}

// FallbackOutput provides plain text output for non-interactive mode.
type FallbackOutput struct {
	writer    io.Writer
	useColor  bool
	verbose   bool
	mu        sync.Mutex
	startTime time.Time
	lastPhase core.Phase
}

// NewFallbackOutput creates a new fallback output handler.
func NewFallbackOutput(useColor, verbose bool) *FallbackOutput {
	return &FallbackOutput{
		writer:    os.Stdout,
		useColor:  useColor,
		verbose:   verbose,
		startTime: time.Now(),
	}
}

// WithWriter sets a custom writer.
func (f *FallbackOutput) WithWriter(w io.Writer) *FallbackOutput {
	f.writer = w
	return f
}

// WorkflowStarted logs workflow start.
func (f *FallbackOutput) WorkflowStarted(prompt string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.startTime = time.Now()
	f.printHeader("Workflow Started")

	if f.verbose {
		truncated := prompt
		if len(truncated) > 100 {
			truncated = truncated[:100] + "..."
		}
		f.printf("  Prompt: %s\n", truncated)
	}
}

// PhaseStarted logs phase start.
func (f *FallbackOutput) PhaseStarted(phase core.Phase) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastPhase = phase
	f.printSection(fmt.Sprintf("Phase: %s", strings.ToUpper(string(phase))))
}

// TaskStarted logs task start.
func (f *FallbackOutput) TaskStarted(task *core.Task) {
	f.mu.Lock()
	defer f.mu.Unlock()

	icon := f.statusIcon("running")
	f.printf("%s [RUNNING] %s\n", icon, task.Name)
}

// TaskCompleted logs task completion.
func (f *FallbackOutput) TaskCompleted(task *core.Task, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	icon := f.statusIcon("completed")
	f.printf("%s [DONE] %s (%s)\n", icon, task.Name, duration.Round(time.Millisecond))

	if f.verbose && task.TokensIn > 0 {
		f.printf("    Tokens: %d in / %d out, Cost: $%.4f\n",
			task.TokensIn, task.TokensOut, task.CostUSD)
	}
}

// TaskFailed logs task failure.
func (f *FallbackOutput) TaskFailed(task *core.Task, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	icon := f.statusIcon("failed")
	f.printf("%s [FAILED] %s: %v\n", icon, task.Name, err)
}

// TaskSkipped logs skipped task.
func (f *FallbackOutput) TaskSkipped(task *core.Task, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	icon := f.statusIcon("skipped")
	f.printf("%s [SKIPPED] %s: %s\n", icon, task.Name, reason)
}

// ConsensusResult logs consensus evaluation.
func (f *FallbackOutput) ConsensusResult(score float64, needsV3 bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.printf("  Consensus Score: %.2f%%\n", score*100)
	if needsV3 {
		f.printf("  %s V3 reconciliation required\n", f.warnIcon())
	}
}

// WorkflowCompleted logs workflow completion.
func (f *FallbackOutput) WorkflowCompleted(state *core.WorkflowState) {
	f.mu.Lock()
	defer f.mu.Unlock()

	duration := time.Since(f.startTime)

	f.printSection("Workflow Completed")
	f.printf("  Duration: %s\n", duration.Round(time.Second))
	f.printf("  Tasks: %d completed\n", f.countCompleted(state))

	// Show detailed agent usage breakdown
	agentBreakdown := f.aggregateAgentUsage(state)
	if len(agentBreakdown) > 0 {
		f.printf("\n  Agent Usage:\n")
		f.printf("  %-12s %-30s %12s %12s\n", "Agent", "Model", "Tokens In", "Tokens Out")
		f.printf("  %s %s %s %s\n",
			strings.Repeat("-", 12),
			strings.Repeat("-", 30),
			strings.Repeat("-", 12),
			strings.Repeat("-", 12))

		var totalTokensIn, totalTokensOut int
		for _, au := range agentBreakdown {
			model := au.Model
			if len(model) > 28 {
				model = model[:25] + "..."
			}
			f.printf("  %-12s %-30s %12d %12d\n",
				au.Name, model, au.TokensIn, au.TokensOut)
			totalTokensIn += au.TokensIn
			totalTokensOut += au.TokensOut
		}

		f.printf("  %s %s %s %s\n",
			strings.Repeat("-", 12),
			strings.Repeat("-", 30),
			strings.Repeat("-", 12),
			strings.Repeat("-", 12))
		f.printf("  %-12s %-30s %12d %12d\n",
			"TOTAL", "", totalTokensIn, totalTokensOut)
	}
}

// aggregateAgentUsage groups task metrics by agent/CLI.
func (f *FallbackOutput) aggregateAgentUsage(state *core.WorkflowState) []agentUsage {
	if len(state.Tasks) == 0 {
		return nil
	}

	// Aggregate by agent+model combination
	usageMap := make(map[string]*agentUsage)
	for _, task := range state.Tasks {
		if task.Status != core.TaskStatusCompleted {
			continue
		}
		agent := task.CLI
		if agent == "" {
			agent = "default"
		}
		model := task.Model
		if model == "" {
			model = "(default)"
		}

		key := agent + "|" + model
		au, ok := usageMap[key]
		if !ok {
			au = &agentUsage{
				Name:  agent,
				Model: model,
			}
			usageMap[key] = au
		}
		au.TokensIn += task.TokensIn
		au.TokensOut += task.TokensOut
		au.Tasks++
	}

	// Convert to sorted slice
	result := make([]agentUsage, 0, len(usageMap))
	for _, au := range usageMap {
		result = append(result, *au)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Model < result[j].Model
	})

	return result
}

// WorkflowFailed logs workflow failure.
func (f *FallbackOutput) WorkflowFailed(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.printError(fmt.Sprintf("Workflow Failed: %v", err))
}

// Log outputs a log message.
func (f *FallbackOutput) Log(level, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.verbose && level == "debug" {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	levelStr := f.formatLevel(level)
	f.printf("[%s] %s %s\n", timestamp, levelStr, message)
}

// Progress outputs progress information.
func (f *FallbackOutput) Progress(current, total int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	percentage := float64(current) / float64(total) * 100
	bar := f.progressBar(percentage, 20)
	f.printf("\r%s %.0f%% %s", bar, percentage, message)

	if current == total {
		f.printf("\n")
	}
}

// Helper methods

func (f *FallbackOutput) printf(format string, args ...interface{}) {
	fmt.Fprintf(f.writer, format, args...)
}

func (f *FallbackOutput) printHeader(text string) {
	line := strings.Repeat("=", 60)
	if f.useColor {
		f.printf("\n%s\n%s %s\n%s\n",
			f.colorize(line, "cyan"),
			f.colorize(">>>", "cyan"),
			text,
			f.colorize(line, "cyan"))
	} else {
		f.printf("\n%s\n>>> %s\n%s\n", line, text, line)
	}
}

func (f *FallbackOutput) printSection(text string) {
	if f.useColor {
		f.printf("\n%s %s\n", f.colorize("---", "blue"), text)
	} else {
		f.printf("\n--- %s\n", text)
	}
}

func (f *FallbackOutput) printError(text string) {
	if f.useColor {
		f.printf("\n%s %s\n", f.colorize("!!!", "red"), f.colorize(text, "red"))
	} else {
		f.printf("\n!!! %s\n", text)
	}
}

func (f *FallbackOutput) statusIcon(status string) string {
	icons := map[string]string{
		"pending":   "○",
		"running":   "●",
		"completed": "✓",
		"failed":    "✗",
		"skipped":   "⊘",
	}

	icon := icons[status]
	if !f.useColor {
		return icon
	}

	colors := map[string]string{
		"pending":   "gray",
		"running":   "cyan",
		"completed": "green",
		"failed":    "red",
		"skipped":   "gray",
	}

	return f.colorize(icon, colors[status])
}

func (f *FallbackOutput) warnIcon() string {
	icon := "⚠"
	if f.useColor {
		return f.colorize(icon, "yellow")
	}
	return icon
}

func (f *FallbackOutput) formatLevel(level string) string {
	upper := strings.ToUpper(level)
	if !f.useColor {
		return fmt.Sprintf("[%5s]", upper)
	}

	colors := map[string]string{
		"debug": "gray",
		"info":  "blue",
		"warn":  "yellow",
		"error": "red",
	}

	return f.colorize(fmt.Sprintf("[%5s]", upper), colors[level])
}

func (f *FallbackOutput) progressBar(percentage float64, width int) string {
	filled := int(float64(width) * percentage / 100)
	empty := width - filled

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"

	if f.useColor {
		return f.colorize(bar, "green")
	}
	return bar
}

func (f *FallbackOutput) colorize(text, color string) string {
	codes := map[string]string{
		"red":    "\033[31m",
		"green":  "\033[32m",
		"yellow": "\033[33m",
		"blue":   "\033[34m",
		"cyan":   "\033[36m",
		"gray":   "\033[90m",
		"reset":  "\033[0m",
	}

	code, ok := codes[color]
	if !ok {
		return text
	}

	return code + text + codes["reset"]
}

func (f *FallbackOutput) countCompleted(state *core.WorkflowState) int {
	count := 0
	for _, task := range state.Tasks {
		if task.Status == core.TaskStatusCompleted {
			count++
		}
	}
	return count
}

// JSONOutput provides structured JSON output.
type JSONOutput struct {
	writer io.Writer
	enc    *json.Encoder
}

// NewJSONOutput creates a new JSON output handler.
func NewJSONOutput() *JSONOutput {
	j := &JSONOutput{
		writer: os.Stdout,
	}
	j.enc = json.NewEncoder(j.writer)
	return j
}

// WithWriter sets a custom writer.
func (j *JSONOutput) WithWriter(w io.Writer) *JSONOutput {
	j.writer = w
	j.enc = json.NewEncoder(j.writer)
	return j
}

// JSONEvent represents a JSON event.
type JSONEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func (j *JSONOutput) emit(eventType string, data interface{}) {
	_ = j.enc.Encode(JSONEvent{ // Errors on stdout encoding are non-recoverable
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// WorkflowStarted emits a workflow started event.
func (j *JSONOutput) WorkflowStarted(prompt string) {
	j.emit("workflow_started", map[string]string{"prompt": prompt})
}

// PhaseStarted emits a phase started event.
func (j *JSONOutput) PhaseStarted(phase core.Phase) {
	j.emit("phase_started", map[string]string{"phase": string(phase)})
}

// TaskCompleted emits a task completed event.
func (j *JSONOutput) TaskCompleted(task *core.Task, duration time.Duration) {
	j.emit("task_completed", map[string]interface{}{
		"task_id":    task.ID,
		"name":       task.Name,
		"duration":   duration.Milliseconds(),
		"tokens_in":  task.TokensIn,
		"tokens_out": task.TokensOut,
		"cost_usd":   task.CostUSD,
	})
}

// TaskFailed emits a task failed event.
func (j *JSONOutput) TaskFailed(task *core.Task, err error) {
	j.emit("task_failed", map[string]interface{}{
		"task_id": task.ID,
		"name":    task.Name,
		"error":   err.Error(),
	})
}

// WorkflowCompleted emits a workflow completed event.
func (j *JSONOutput) WorkflowCompleted(state *core.WorkflowState) {
	totalTokensIn := 0
	totalTokensOut := 0
	if state.Metrics != nil {
		totalTokensIn = state.Metrics.TotalTokensIn
		totalTokensOut = state.Metrics.TotalTokensOut
	}

	// Aggregate agent usage
	agentUsage := j.aggregateAgentUsage(state)

	j.emit("workflow_completed", map[string]interface{}{
		"total_tokens_in":  totalTokensIn,
		"total_tokens_out": totalTokensOut,
		"completed_tasks":  len(state.Tasks),
		"agent_usage":      agentUsage,
	})
}

// aggregateAgentUsage groups task metrics by agent/CLI for JSON output.
func (j *JSONOutput) aggregateAgentUsage(state *core.WorkflowState) []map[string]interface{} {
	if len(state.Tasks) == 0 {
		return nil
	}

	// Aggregate by agent+model combination
	type usage struct {
		Agent     string
		Model     string
		TokensIn  int
		TokensOut int
		Tasks     int
	}
	usageMap := make(map[string]*usage)

	for _, task := range state.Tasks {
		if task.Status != core.TaskStatusCompleted {
			continue
		}
		agent := task.CLI
		if agent == "" {
			agent = "default"
		}
		model := task.Model
		if model == "" {
			model = "(default)"
		}

		key := agent + "|" + model
		u, ok := usageMap[key]
		if !ok {
			u = &usage{Agent: agent, Model: model}
			usageMap[key] = u
		}
		u.TokensIn += task.TokensIn
		u.TokensOut += task.TokensOut
		u.Tasks++
	}

	// Convert to slice of maps for JSON
	result := make([]map[string]interface{}, 0, len(usageMap))
	for _, u := range usageMap {
		result = append(result, map[string]interface{}{
			"agent":      u.Agent,
			"model":      u.Model,
			"tokens_in":  u.TokensIn,
			"tokens_out": u.TokensOut,
			"tasks":      u.Tasks,
		})
	}

	return result
}

// WorkflowFailed emits a workflow failed event.
func (j *JSONOutput) WorkflowFailed(err error) {
	j.emit("workflow_failed", map[string]string{"error": err.Error()})
}

// Log emits a log event.
func (j *JSONOutput) Log(level, message string) {
	j.emit("log", map[string]string{
		"level":   level,
		"message": message,
	})
}
