package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestMetricsCollector_Basic tests basic metric collection functionality.
func TestMetricsCollector_Basic(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()

	// Test workflow start/end
	collector.StartWorkflow()
	time.Sleep(10 * time.Millisecond) // Small delay
	
	// Test task start/end
	testTask := &core.Task{
		ID:   core.TaskID("test-task"),
		Name: "test",
	}
	
	collector.StartTask(testTask, "claude")
	time.Sleep(5 * time.Millisecond)
	
	result := &core.ExecuteResult{
		Output: "test output",
	}
	collector.EndTask(core.TaskID("test-task"), result, nil)
	
	collector.EndWorkflow()

	// Get metrics
	metrics := collector.GetWorkflowMetrics()
	
	// Verify basic fields
	if metrics.TasksTotal != 1 {
		t.Errorf("Expected 1 task total, got %d", metrics.TasksTotal)
	}

	if metrics.TasksCompleted != 1 {
		t.Errorf("Expected 1 completed task, got %d", metrics.TasksCompleted)
	}
}

// TestMetricsCollector_MultipleAgents tests metrics with multiple agents.
func TestMetricsCollector_MultipleAgents(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// Record tasks from different agents
	agents := []string{"claude", "gemini", "gpt"}
	for i, agent := range agents {
		task := &core.Task{
			ID:   core.TaskID(fmt.Sprintf("task-%d", i)),
			Name: fmt.Sprintf("task%d", i),
		}
		
		collector.StartTask(task, agent)
		time.Sleep(time.Duration(i+1) * time.Millisecond) // Variable timing
		
		result := &core.ExecuteResult{
			Output: fmt.Sprintf("output for task %d", i),
		}
		collector.EndTask(task.ID, result, nil)
	}

	collector.EndWorkflow()
	metrics := collector.GetWorkflowMetrics()

	expectedTasks := len(agents)
	if metrics.TasksTotal != expectedTasks {
		t.Errorf("Expected %d tasks total, got %d", expectedTasks, metrics.TasksTotal)
	}

	if metrics.TasksCompleted != expectedTasks {
		t.Errorf("Expected %d completed tasks, got %d", expectedTasks, metrics.TasksCompleted)
	}

	if metrics.TasksFailed != 0 {
		t.Errorf("Expected 0 failed tasks, got %d", metrics.TasksFailed)
	}
}

// TestMetricsCollector_TaskFailures tests metrics with task failures.
func TestMetricsCollector_TaskFailures(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// Successful task
	successTask := &core.Task{
		ID:   core.TaskID("success-task"),
		Name: "success",
	}
	collector.StartTask(successTask, "claude")
	
	successResult := &core.ExecuteResult{
		Output: "success",
	}
	collector.EndTask(successTask.ID, successResult, nil)

	// Failed task
	failTask := &core.Task{
		ID:   core.TaskID("fail-task"),
		Name: "fail",
	}
	collector.StartTask(failTask, "claude")
	
	failResult := &core.ExecuteResult{
		Output: "failed",
	}
	collector.EndTask(failTask.ID, failResult, fmt.Errorf("task failed"))

	collector.EndWorkflow()
	metrics := collector.GetWorkflowMetrics()

	if metrics.TasksTotal != 2 {
		t.Errorf("Expected 2 tasks total, got %d", metrics.TasksTotal)
	}

	if metrics.TasksCompleted != 1 {
		t.Errorf("Expected 1 completed task, got %d", metrics.TasksCompleted)
	}

	if metrics.TasksFailed != 1 {
		t.Errorf("Expected 1 failed task, got %d", metrics.TasksFailed)
	}
}

// TestMetricsCollector_Retries tests metrics with task retries.
func TestMetricsCollector_Retries(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()
	collector.StartWorkflow()

	taskID := core.TaskID("retry-task")
	
	// Record several retries
	collector.RecordRetry(taskID)
	collector.RecordRetry(taskID)
	collector.RecordRetry(taskID)

	task := &core.Task{
		ID:   taskID,
		Name: "retry-task",
	}
	collector.StartTask(task, "claude")
	
	result := &core.ExecuteResult{
		Output: "success after retries",
	}
	collector.EndTask(taskID, result, nil)

	collector.EndWorkflow()
	metrics := collector.GetWorkflowMetrics()

	if metrics.RetriesTotal != 3 {
		t.Errorf("Expected 3 retries total, got %d", metrics.RetriesTotal)
	}

	if metrics.TasksCompleted != 1 {
		t.Errorf("Expected 1 completed task, got %d", metrics.TasksCompleted)
	}
}

// TestMetricsCollector_Concurrency tests concurrent metric collection.
func TestMetricsCollector_Concurrency(t *testing.T) {
	t.Parallel()

	collector := NewMetricsCollector()
	collector.StartWorkflow()

	// Simulate concurrent task execution
	numTasks := 10
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(taskNum int) {
			defer func() { done <- true }()
			
			task := &core.Task{
				ID:   core.TaskID(fmt.Sprintf("concurrent-task-%d", taskNum)),
				Name: fmt.Sprintf("task%d", taskNum),
			}
			
			collector.StartTask(task, "claude")
			time.Sleep(time.Duration(taskNum%5+1) * time.Millisecond) // Variable timing
			
			result := &core.ExecuteResult{
				Output: fmt.Sprintf("output %d", taskNum),
			}
			collector.EndTask(task.ID, result, nil)
		}(i)
	}

	// Wait for all tasks to complete
	for i := 0; i < numTasks; i++ {
		<-done
	}

	collector.EndWorkflow()
	metrics := collector.GetWorkflowMetrics()

	if metrics.TasksTotal != numTasks {
		t.Errorf("Expected %d tasks total, got %d", numTasks, metrics.TasksTotal)
	}

	if metrics.TasksCompleted != numTasks {
		t.Errorf("Expected %d completed tasks, got %d", numTasks, metrics.TasksCompleted)
	}

	if metrics.TasksFailed != 0 {
		t.Errorf("Expected 0 failed tasks, got %d", metrics.TasksFailed)
	}
}
