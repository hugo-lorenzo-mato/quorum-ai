package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestMultiAgentExecution_ParallelTasks tests concurrent execution of tasks by multiple agents.
func TestMultiAgentExecution_ParallelTasks(t *testing.T) {
	t.Parallel()

	// Create mock orchestrator
	orchestrator := NewMockOrchestrator()

	// Define test scenario with parallel tasks
	tasks := []TaskDefinition{
		{
			ID:           core.TaskID("parallel-task-1"),
			Name:         "File Analysis",
			Agent:        "claude",
			Dependencies: []core.TaskID{}, // No dependencies - can run immediately
			EstimatedDuration: 2 * time.Second,
		},
		{
			ID:           core.TaskID("parallel-task-2"),
			Name:         "Code Review",
			Agent:        "gemini",
			Dependencies: []core.TaskID{}, // No dependencies - can run in parallel
			EstimatedDuration: 3 * time.Second,
		},
		{
			ID:           core.TaskID("parallel-task-3"),
			Name:         "Documentation Check",
			Agent:        "gpt",
			Dependencies: []core.TaskID{}, // No dependencies - can run in parallel
			EstimatedDuration: 1 * time.Second,
		},
		{
			ID:           core.TaskID("sequential-task-4"),
			Name:         "Integration",
			Agent:        "claude",
			Dependencies: []core.TaskID{"parallel-task-1", "parallel-task-2", "parallel-task-3"}, // Depends on all parallel tasks
			EstimatedDuration: 1 * time.Second,
		},
	}

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	results, err := orchestrator.ExecuteWorkflow(ctx, tasks)
	totalDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify all tasks completed
	if len(results) != len(tasks) {
		t.Errorf("Expected %d task results, got %d", len(tasks), len(results))
	}

	// Verify all tasks succeeded
	for _, result := range results {
		if result.Status != core.TaskStatusCompleted {
			t.Errorf("Task %s failed: status %s, error: %s", result.TaskID, result.Status, result.Error)
		}
	}

	// Verify parallel execution efficiency
	// Parallel tasks should run concurrently, so total time should be less than sum of individual durations
	sequentialTime := 2*time.Second + 3*time.Second + 1*time.Second + 1*time.Second // 7 seconds
	parallelTime := 3*time.Second + 1*time.Second // max(2,3,1) + 1 = 4 seconds expected

	if totalDuration > parallelTime+2*time.Second { // Allow 2 second buffer for overhead
		t.Errorf("Execution took too long: %v (expected around %v for parallel execution)", totalDuration, parallelTime)
	}

	if totalDuration > sequentialTime-1*time.Second {
		t.Errorf("Execution did not demonstrate parallelism: %v (sequential would be %v)", totalDuration, sequentialTime)
	}

	t.Logf("Multi-agent parallel execution completed in %v (parallel advantage: %v)", 
		totalDuration, sequentialTime-totalDuration)
}

// TestMultiAgentExecution_DependencyResolution tests complex dependency graphs.
func TestMultiAgentExecution_DependencyResolution(t *testing.T) {
	t.Parallel()

	orchestrator := NewMockOrchestrator()

	// Create complex dependency graph:
	// A -> B -> D
	// A -> C -> D
	// B -> E
	// C -> E
	// D,E -> F
	tasks := []TaskDefinition{
		{ID: core.TaskID("A"), Name: "Initialize", Agent: "claude", Dependencies: []core.TaskID{}},
		{ID: core.TaskID("B"), Name: "Branch B", Agent: "gemini", Dependencies: []core.TaskID{"A"}},
		{ID: core.TaskID("C"), Name: "Branch C", Agent: "gpt", Dependencies: []core.TaskID{"A"}},
		{ID: core.TaskID("D"), Name: "Merge BD", Agent: "claude", Dependencies: []core.TaskID{"B", "C"}},
		{ID: core.TaskID("E"), Name: "Parallel E", Agent: "gemini", Dependencies: []core.TaskID{"B", "C"}},
		{ID: core.TaskID("F"), Name: "Final", Agent: "gpt", Dependencies: []core.TaskID{"D", "E"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := orchestrator.ExecuteWorkflow(ctx, tasks)
	if err != nil {
		t.Fatalf("Complex workflow execution failed: %v", err)
	}

	// Verify execution order respects dependencies
	executionOrder := make(map[core.TaskID]int)
	for i, result := range results {
		executionOrder[result.TaskID] = i
	}

	// Check dependency constraints
	dependencyTests := []struct {
		task        core.TaskID
		mustBeAfter []core.TaskID
	}{
		{"B", []core.TaskID{"A"}},
		{"C", []core.TaskID{"A"}},
		{"D", []core.TaskID{"B", "C"}},
		{"E", []core.TaskID{"B", "C"}},
		{"F", []core.TaskID{"D", "E"}},
	}

	for _, test := range dependencyTests {
		taskOrder, taskExists := executionOrder[test.task]
		if !taskExists {
			t.Errorf("Task %s was not executed", test.task)
			continue
		}

		for _, dependency := range test.mustBeAfter {
			depOrder, depExists := executionOrder[dependency]
			if !depExists {
				t.Errorf("Dependency task %s was not executed", dependency)
				continue
			}

			if taskOrder <= depOrder {
				t.Errorf("Task %s (order %d) should execute after dependency %s (order %d)", 
					test.task, taskOrder, dependency, depOrder)
			}
		}
	}

	// Verify all tasks completed successfully
	for _, result := range results {
		if result.Status != core.TaskStatusCompleted {
			t.Errorf("Task %s failed in complex workflow: %s", result.TaskID, result.Error)
		}
	}

	t.Logf("Complex dependency resolution completed successfully with %d tasks", len(results))
}

// TestMultiAgentExecution_ErrorPropagation tests how errors propagate through the workflow.
func TestMultiAgentExecution_ErrorPropagation(t *testing.T) {
	t.Parallel()

	orchestrator := NewMockOrchestrator()

	// Create workflow where one task fails and affects dependent tasks
	tasks := []TaskDefinition{
		{ID: core.TaskID("success-1"), Name: "Success Task", Agent: "claude", Dependencies: []core.TaskID{}},
		{ID: core.TaskID("failure"), Name: "Failing Task", Agent: "failing-agent", Dependencies: []core.TaskID{}}, // Will fail
		{ID: core.TaskID("dependent-1"), Name: "Depends on Success", Agent: "gemini", Dependencies: []core.TaskID{"success-1"}},
		{ID: core.TaskID("dependent-2"), Name: "Depends on Failure", Agent: "gpt", Dependencies: []core.TaskID{"failure"}},
		{ID: core.TaskID("dependent-both"), Name: "Depends on Both", Agent: "claude", Dependencies: []core.TaskID{"success-1", "failure"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := orchestrator.ExecuteWorkflow(ctx, tasks)

	// Should get partial results even with failures
	if err == nil {
		t.Error("Expected workflow to return error when tasks fail")
	}

	// Analyze results
	successfulTasks := 0
	failedTasks := 0

	for _, result := range results {
		switch result.Status {
		case core.TaskStatusCompleted:
			successfulTasks++
		case core.TaskStatusFailed:
			failedTasks++
		case core.TaskStatusPending:
			// Tasks that couldn't run due to failed dependencies
		}
	}

	// Expected outcome:
	// success-1: should complete
	// failure: should fail
	// dependent-1: should complete (dependency succeeded)
	// dependent-2: should fail/skip (dependency failed)
	// dependent-both: should fail/skip (one dependency failed)

	expectedSuccessful := 2 // success-1, dependent-1
	expectedFailed := 1     // failure

	if successfulTasks != expectedSuccessful {
		t.Errorf("Expected %d successful tasks, got %d", expectedSuccessful, successfulTasks)
	}

	if failedTasks < expectedFailed {
		t.Errorf("Expected at least %d failed tasks, got %d", expectedFailed, failedTasks)
	}

	// Verify specific task outcomes
	resultMap := make(map[core.TaskID]*TaskResult)
	for _, result := range results {
		resultMap[result.TaskID] = result
	}

	if result := resultMap[core.TaskID("success-1")]; result.Status != core.TaskStatusCompleted {
		t.Errorf("Independent successful task should complete, got status: %s", result.Status)
	}

	if result := resultMap[core.TaskID("failure")]; result.Status == core.TaskStatusCompleted {
		t.Errorf("Failing task should not complete, got status: %s", result.Status)
	}

	if result := resultMap[core.TaskID("dependent-1")]; result.Status != core.TaskStatusCompleted {
		t.Errorf("Task dependent on success should complete, got status: %s", result.Status)
	}

	t.Logf("Error propagation test: %d successful, %d failed", 
		successfulTasks, failedTasks)
}

// TestMultiAgentExecution_AgentResourceLimits tests resource constraints and agent limits.
func TestMultiAgentExecution_AgentResourceLimits(t *testing.T) {
	t.Parallel()

	// Create orchestrator with resource limits
	orchestrator := NewMockOrchestrator()
	orchestrator.SetAgentLimit("claude", 2)   // Max 2 concurrent tasks
	orchestrator.SetAgentLimit("gemini", 1)   // Max 1 concurrent task
	orchestrator.SetAgentLimit("gpt", 3)      // Max 3 concurrent tasks

	// Create many tasks for limited agents
	tasks := make([]TaskDefinition, 10)
	for i := 0; i < 10; i++ {
		agent := []string{"claude", "gemini", "gpt"}[i%3]
		tasks[i] = TaskDefinition{
			ID:                core.TaskID(fmt.Sprintf("task-%d", i)),
			Name:              fmt.Sprintf("Task %d", i),
			Agent:             agent,
			Dependencies:      []core.TaskID{},
			EstimatedDuration: 1 * time.Second,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	startTime := time.Now()
	results, err := orchestrator.ExecuteWorkflow(ctx, tasks)
	totalDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Resource-limited workflow failed: %v", err)
	}

	// Verify all tasks completed
	if len(results) != len(tasks) {
		t.Errorf("Expected %d results, got %d", len(tasks), len(results))
	}

	// All tasks should succeed
	for _, result := range results {
		if result.Status != core.TaskStatusCompleted {
			t.Errorf("Task %s failed: %s", result.TaskID, result.Error)
		}
	}

	// With limits, execution should take longer than unlimited parallelism
	minimumDuration := 3 * time.Second // Limited by gemini agent (1 concurrent * 4 tasks * 1s each)
	if totalDuration < minimumDuration {
		t.Errorf("Execution too fast: %v (expected at least %v due to resource limits)", 
			totalDuration, minimumDuration)
	}

	// But shouldn't be fully sequential
	maxDuration := 10 * time.Second // Full sequential execution
	if totalDuration > maxDuration {
		t.Errorf("Execution too slow: %v (should benefit from partial parallelism)", totalDuration)
	}

	t.Logf("Resource-limited execution: %v with agent limits (claude:2, gemini:1, gpt:3)", totalDuration)
}

// Mock implementations for testing

type TaskDefinition struct {
	ID                core.TaskID
	Name              string
	Agent             string
	Dependencies      []core.TaskID
	EstimatedDuration time.Duration
}

type TaskResult struct {
	TaskID      core.TaskID
	Status      core.TaskStatus
	Error       string
	StartTime   time.Time
	EndTime     time.Time
	AgentUsed   string
}

type MockOrchestrator struct {
	agentLimits map[string]int
	mu          sync.Mutex
}

func NewMockOrchestrator() *MockOrchestrator {
	return &MockOrchestrator{
		agentLimits: make(map[string]int),
	}
}

func (o *MockOrchestrator) SetAgentLimit(agent string, limit int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.agentLimits[agent] = limit
}

func (o *MockOrchestrator) ExecuteWorkflow(ctx context.Context, tasks []TaskDefinition) ([]*TaskResult, error) {
	// Build dependency graph
	taskMap := make(map[core.TaskID]*TaskDefinition)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}

	// Track completion status
	results := make([]*TaskResult, 0, len(tasks))
	completed := make(map[core.TaskID]bool)
	failed := make(map[core.TaskID]bool)
	
	// Agent semaphores for resource limits
	agentSems := make(map[string]chan struct{})
	for agent, limit := range o.agentLimits {
		if limit > 0 {
			agentSems[agent] = make(chan struct{}, limit)
		}
	}

	// Execute tasks respecting dependencies and resource limits
	var wg sync.WaitGroup
	resultChan := make(chan *TaskResult, len(tasks))
	
	// Track running tasks to avoid duplicates
	running := make(map[core.TaskID]bool)
	var runMutex sync.Mutex

	var scheduleTask func(taskID core.TaskID)
	scheduleTask = func(taskID core.TaskID) {
		task := taskMap[taskID]
		
		runMutex.Lock()
		if running[taskID] || completed[taskID] || failed[taskID] {
			runMutex.Unlock()
			return
		}
		
		// Check if all dependencies are satisfied
		for _, dep := range task.Dependencies {
			if !completed[dep] {
				runMutex.Unlock()
				return
			}
		}
		
		running[taskID] = true
		runMutex.Unlock()

		wg.Add(1)
		go func(t *TaskDefinition) {
			defer wg.Done()
			
			// Acquire agent resource if limited
			if sem, exists := agentSems[t.Agent]; exists {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					resultChan <- &TaskResult{
						TaskID: t.ID,
						Status: core.TaskStatusFailed,
						Error:  "context cancelled",
					}
					return
				}
			}

			result := &TaskResult{
				TaskID:    t.ID,
				StartTime: time.Now(),
				AgentUsed: t.Agent,
			}

			// Simulate task execution
			if t.Agent == "failing-agent" {
				result.Status = core.TaskStatusFailed
				result.Error = "simulated agent failure"
			} else {
				// Simulate work duration
				select {
				case <-time.After(t.EstimatedDuration):
					result.Status = core.TaskStatusCompleted
				case <-ctx.Done():
					result.Status = core.TaskStatusFailed
					result.Error = "context cancelled"
				}
			}

			result.EndTime = time.Now()
			resultChan <- result
		}(task)
	}

	// Start scheduling loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Try to schedule any ready tasks
				for taskID := range taskMap {
					scheduleTask(taskID)
				}
				time.Sleep(10 * time.Millisecond) // Small delay to avoid busy waiting
			}
		}
	}()

	// Collect results
	var resultErr error
	for len(results) < len(tasks) {
		select {
		case result := <-resultChan:
			results = append(results, result)
			
			runMutex.Lock()
			if result.Status == core.TaskStatusCompleted {
				completed[result.TaskID] = true
			} else {
				failed[result.TaskID] = true
				if resultErr == nil {
					resultErr = fmt.Errorf("task %s failed: %s", result.TaskID, result.Error)
				}
			}
			running[result.TaskID] = false
			runMutex.Unlock()
			
		case <-ctx.Done():
			resultErr = fmt.Errorf("workflow execution timed out")
			break
		}
	}

	wg.Wait()
	close(resultChan)

	return results, resultErr
}
