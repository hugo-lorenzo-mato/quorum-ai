package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// promptPhaseReview prompts the user to review a completed phase.
// Returns (action, feedback) where action is "continue", "rerun", or "abort".
func promptPhaseReview(scanner *bufio.Scanner, phaseName string) (string, string) {
	fmt.Printf("\n  [Enter] Continue to next phase\n")
	fmt.Printf("  [f]     Add feedback to %s\n", phaseName)
	fmt.Printf("  [r]     Re-run %s\n", phaseName)
	fmt.Printf("  [q]     Abort\n")
	fmt.Print("  > ")

	if !scanner.Scan() {
		return "continue", ""
	}

	input := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(input) {
	case "", "c":
		return "continue", ""
	case "f":
		fmt.Printf("  Enter feedback: ")
		if !scanner.Scan() {
			return "continue", ""
		}
		feedback := strings.TrimSpace(scanner.Text())
		if feedback != "" {
			fmt.Println("  Feedback saved. Continuing...")
		}
		return "continue", feedback
	case "r":
		return "rerun", ""
	case "q":
		return "abort", ""
	default:
		return "continue", ""
	}
}

// promptPlanReview prompts the user to review the task plan.
// Returns (action, feedback) where action is "continue", "edit", "replan", or "abort".
func promptPlanReview(scanner *bufio.Scanner) (string, string) {
	fmt.Println("\n  [Enter] Execute plan")
	fmt.Println("  [e]     Edit tasks")
	fmt.Println("  [r]     Regenerate plan")
	fmt.Println("  [q]     Abort")
	fmt.Print("  > ")

	if !scanner.Scan() {
		return "continue", ""
	}

	input := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(input) {
	case "", "c":
		return "continue", ""
	case "e":
		return "edit", ""
	case "r":
		fmt.Print("  Feedback for replanning (optional, Enter to skip): ")
		if !scanner.Scan() {
			return "replan", ""
		}
		return "replan", strings.TrimSpace(scanner.Text())
	case "q":
		return "abort", ""
	default:
		return "continue", ""
	}
}

// editTasksInteractive allows the user to edit tasks interactively.
func editTasksInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	for {
		fmt.Printf("\n  Edit task number (1-%d), [a]dd, [d]elete, or [Enter] done: ", len(state.TaskOrder))
		if !scanner.Scan() {
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			return
		}

		switch strings.ToLower(input) {
		case "a":
			addTaskInteractive(scanner, state)
		case "d":
			deleteTaskInteractive(scanner, state)
		default:
			// Try to parse as task number
			num, err := strconv.Atoi(input)
			if err != nil || num < 1 || num > len(state.TaskOrder) {
				fmt.Println("  Invalid input.")
				continue
			}
			editSingleTask(scanner, state, num-1)
		}
	}
}

// editSingleTask edits a single task by index.
func editSingleTask(scanner *bufio.Scanner, state *core.WorkflowState, idx int) {
	taskID := state.TaskOrder[idx]
	task, ok := state.Tasks[taskID]
	if !ok {
		fmt.Println("  Task not found.")
		return
	}

	fmt.Printf("  Current: [%s] %s\n", task.CLI, task.Name)

	fmt.Print("  New name (Enter to keep): ")
	if scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			task.Name = name
		}
	}

	fmt.Print("  New description (Enter to keep): ")
	if scanner.Scan() {
		desc := strings.TrimSpace(scanner.Text())
		if desc != "" {
			task.Description = desc
		}
	}

	fmt.Print("  New agent (Enter to keep): ")
	if scanner.Scan() {
		agent := strings.TrimSpace(scanner.Text())
		if agent != "" {
			task.CLI = agent
		}
	}

	fmt.Println("  Task updated.")
}

// addTaskInteractive adds a new task interactively.
func addTaskInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	fmt.Print("  Task name: ")
	if !scanner.Scan() {
		return
	}
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		fmt.Println("  Name required. Cancelled.")
		return
	}

	fmt.Print("  Agent (e.g., claude, gemini, codex): ")
	if !scanner.Scan() {
		return
	}
	agent := strings.TrimSpace(scanner.Text())
	if agent == "" {
		fmt.Println("  Agent required. Cancelled.")
		return
	}

	fmt.Print("  Description (optional): ")
	var desc string
	if scanner.Scan() {
		desc = strings.TrimSpace(scanner.Text())
	}

	taskID := core.TaskID(fmt.Sprintf("task_interactive_%d", time.Now().UnixNano()))
	newTask := &core.TaskState{
		ID:          taskID,
		Phase:       core.PhaseExecute,
		Name:        name,
		Description: desc,
		Status:      core.TaskStatusPending,
		CLI:         agent,
	}

	if state.Tasks == nil {
		state.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	state.Tasks[taskID] = newTask
	state.TaskOrder = append(state.TaskOrder, taskID)
	fmt.Println("  Task added.")
}

// deleteTaskInteractive deletes a task interactively.
func deleteTaskInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	fmt.Printf("  Delete task number (1-%d): ", len(state.TaskOrder))
	if !scanner.Scan() {
		return
	}

	num, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || num < 1 || num > len(state.TaskOrder) {
		fmt.Println("  Invalid task number.")
		return
	}

	idx := num - 1
	taskID := state.TaskOrder[idx]

	// Check dependencies
	for _, otherTask := range state.Tasks {
		for _, dep := range otherTask.Dependencies {
			if dep == taskID {
				fmt.Printf("  Cannot delete: task %s depends on it.\n", otherTask.Name)
				return
			}
		}
	}

	delete(state.Tasks, taskID)
	state.TaskOrder = append(state.TaskOrder[:idx], state.TaskOrder[idx+1:]...)
	fmt.Println("  Task deleted.")
}

// displayTaskPlan shows the current task plan.
func displayTaskPlan(state *core.WorkflowState) {
	fmt.Printf("\n  === Task Plan (%d tasks) ===\n", len(state.TaskOrder))

	// Precompute 1-based positions to avoid nested scans (reduces cognitive complexity).
	indexByID := make(map[core.TaskID]int, len(state.TaskOrder))
	for i, tid := range state.TaskOrder {
		indexByID[tid] = i + 1
	}

	for i, taskID := range state.TaskOrder {
		task, ok := state.Tasks[taskID]
		if !ok {
			continue
		}
		deps := ""
		if len(task.Dependencies) > 0 {
			depNums := dependencyIndices(task.Dependencies, indexByID)
			deps = fmt.Sprintf(" (depends: %s)", strings.Join(depNums, ", "))
		}
		fmt.Printf("  %d. [%s] %s%s\n", i+1, task.CLI, task.Name, deps)
	}
}

func dependencyIndices(deps []core.TaskID, indexByID map[core.TaskID]int) []string {
	out := make([]string, 0, len(deps))
	for _, dep := range deps {
		if n, ok := indexByID[dep]; ok {
			out = append(out, strconv.Itoa(n))
		}
	}
	return out
}

// displayTruncated displays text, truncated to maxLines.
func displayTruncated(text string, maxLines int) {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		return
	}
	for _, line := range lines[:maxLines] {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("  ... (%d more lines)\n", len(lines)-maxLines)
}
