package service

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestDAGBuilder_AddTask(t *testing.T) {
	dag := NewDAGBuilder()

	task := core.NewTask("task-1", "Test Task", core.PhaseAnalyze)
	err := dag.AddTask(task)
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}

	if dag.TaskCount() != 1 {
		t.Errorf("TaskCount() = %d, want 1", dag.TaskCount())
	}

	// Add duplicate should fail
	err = dag.AddTask(task)
	if err == nil {
		t.Error("AddTask() should fail for duplicate task")
	}
}

func TestDAGBuilder_AddDependency(t *testing.T) {
	dag := NewDAGBuilder()

	task1 := core.NewTask("task-1", "First Task", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second Task", core.PhasePlan)

	dag.AddTask(task1)
	dag.AddTask(task2)

	// task-2 depends on task-1
	err := dag.AddDependency("task-2", "task-1")
	if err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	deps := dag.GetDependencies("task-2")
	if len(deps) != 1 || deps[0] != "task-1" {
		t.Errorf("GetDependencies() = %v, want [task-1]", deps)
	}

	dependents := dag.GetDependents("task-1")
	if len(dependents) != 1 || dependents[0] != "task-2" {
		t.Errorf("GetDependents() = %v, want [task-2]", dependents)
	}
}

func TestDAGBuilder_AddDependencyNotFound(t *testing.T) {
	dag := NewDAGBuilder()
	task1 := core.NewTask("task-1", "First Task", core.PhaseAnalyze)
	dag.AddTask(task1)

	// Should fail for non-existent "from" task
	err := dag.AddDependency("task-2", "task-1")
	if err == nil {
		t.Error("AddDependency() should fail for non-existent task")
	}

	// Should fail for non-existent "to" task
	err = dag.AddDependency("task-1", "task-3")
	if err == nil {
		t.Error("AddDependency() should fail for non-existent dependency")
	}
}

func TestDAGBuilder_TopologicalSort(t *testing.T) {
	dag := NewDAGBuilder()

	// Create a simple dependency chain: task-1 -> task-2 -> task-3
	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second", core.PhasePlan)
	task3 := core.NewTask("task-3", "Third", core.PhaseExecute)

	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.AddTask(task3)

	dag.AddDependency("task-2", "task-1") // task-2 depends on task-1
	dag.AddDependency("task-3", "task-2") // task-3 depends on task-2

	state, err := dag.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// task-1 must come before task-2, task-2 before task-3
	order := state.Order
	t.Logf("Order: %v", order)

	indexOf := func(id core.TaskID) int {
		for i, tid := range order {
			if tid == id {
				return i
			}
		}
		return -1
	}

	if indexOf("task-1") > indexOf("task-2") {
		t.Error("task-1 should come before task-2")
	}
	if indexOf("task-2") > indexOf("task-3") {
		t.Error("task-2 should come before task-3")
	}
}

func TestDAGBuilder_CycleDetection(t *testing.T) {
	dag := NewDAGBuilder()

	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second", core.PhasePlan)
	task3 := core.NewTask("task-3", "Third", core.PhaseExecute)

	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.AddTask(task3)

	// Create a cycle: task-1 -> task-2 -> task-3 -> task-1
	dag.AddDependency("task-2", "task-1")
	dag.AddDependency("task-3", "task-2")
	dag.AddDependency("task-1", "task-3") // Creates cycle

	_, err := dag.Build()
	if err == nil {
		t.Error("Build() should fail with cycle detected")
	}

	domErr, ok := err.(*core.DomainError)
	if !ok {
		t.Fatalf("error should be DomainError, got %T", err)
	}
	if domErr.Code != "CYCLE_DETECTED" {
		t.Errorf("error code = %s, want CYCLE_DETECTED", domErr.Code)
	}
}

func TestDAGBuilder_GetReadyTasks(t *testing.T) {
	dag := NewDAGBuilder()

	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second", core.PhasePlan)
	task3 := core.NewTask("task-3", "Third", core.PhaseExecute)

	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.AddTask(task3)

	dag.AddDependency("task-2", "task-1")
	dag.AddDependency("task-3", "task-2")

	// Initially only task-1 is ready
	completed := make(map[core.TaskID]bool)
	ready := dag.GetReadyTasks(completed)
	if len(ready) != 1 || ready[0].ID != "task-1" {
		t.Errorf("GetReadyTasks() = %v, want [task-1]", taskIDs(ready))
	}

	// After task-1 completes, task-2 is ready
	completed["task-1"] = true
	ready = dag.GetReadyTasks(completed)
	if len(ready) != 1 || ready[0].ID != "task-2" {
		t.Errorf("GetReadyTasks() = %v, want [task-2]", taskIDs(ready))
	}

	// After task-2 completes, task-3 is ready
	completed["task-2"] = true
	ready = dag.GetReadyTasks(completed)
	if len(ready) != 1 || ready[0].ID != "task-3" {
		t.Errorf("GetReadyTasks() = %v, want [task-3]", taskIDs(ready))
	}

	// After task-3 completes, no tasks ready
	completed["task-3"] = true
	ready = dag.GetReadyTasks(completed)
	if len(ready) != 0 {
		t.Errorf("GetReadyTasks() = %v, want empty", taskIDs(ready))
	}
}

func TestDAGBuilder_Levels(t *testing.T) {
	dag := NewDAGBuilder()

	// Create a diamond dependency pattern:
	//     task-1
	//    /      \
	// task-2   task-3
	//    \      /
	//     task-4

	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second Left", core.PhasePlan)
	task3 := core.NewTask("task-3", "Second Right", core.PhasePlan)
	task4 := core.NewTask("task-4", "Third", core.PhaseExecute)

	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.AddTask(task3)
	dag.AddTask(task4)

	dag.AddDependency("task-2", "task-1")
	dag.AddDependency("task-3", "task-1")
	dag.AddDependency("task-4", "task-2")
	dag.AddDependency("task-4", "task-3")

	state, err := dag.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	levels := state.Levels
	t.Logf("Levels: %v", levels)

	if len(levels) != 3 {
		t.Fatalf("len(Levels) = %d, want 3", len(levels))
	}

	// Level 0: task-1
	if len(levels[0]) != 1 {
		t.Errorf("Level 0 len = %d, want 1", len(levels[0]))
	}

	// Level 1: task-2, task-3 (can run in parallel)
	if len(levels[1]) != 2 {
		t.Errorf("Level 1 len = %d, want 2", len(levels[1]))
	}

	// Level 2: task-4
	if len(levels[2]) != 1 {
		t.Errorf("Level 2 len = %d, want 1", len(levels[2]))
	}
}

func TestDAGBuilder_GetTask(t *testing.T) {
	dag := NewDAGBuilder()

	task := core.NewTask("task-1", "Test Task", core.PhaseAnalyze)
	dag.AddTask(task)

	retrieved, ok := dag.GetTask("task-1")
	if !ok {
		t.Fatal("GetTask() should find task-1")
	}
	if retrieved.Name != "Test Task" {
		t.Errorf("GetTask().Name = %s, want Test Task", retrieved.Name)
	}

	_, ok = dag.GetTask("nonexistent")
	if ok {
		t.Error("GetTask() should not find nonexistent task")
	}
}

func TestDAGBuilder_EmptyDAG(t *testing.T) {
	dag := NewDAGBuilder()

	state, err := dag.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(state.Order) != 0 {
		t.Errorf("Order should be empty, got %v", state.Order)
	}
	if len(state.Tasks) != 0 {
		t.Errorf("Tasks should be empty, got %d", len(state.Tasks))
	}
}

func TestDAGBuilder_ParallelTasks(t *testing.T) {
	dag := NewDAGBuilder()

	// Independent tasks can all run in parallel
	task1 := core.NewTask("task-1", "Task 1", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Task 2", core.PhaseAnalyze)
	task3 := core.NewTask("task-3", "Task 3", core.PhaseAnalyze)

	dag.AddTask(task1)
	dag.AddTask(task2)
	dag.AddTask(task3)

	state, err := dag.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// All tasks should be at level 0
	if len(state.Levels) != 1 {
		t.Errorf("len(Levels) = %d, want 1", len(state.Levels))
	}
	if len(state.Levels[0]) != 3 {
		t.Errorf("Level 0 len = %d, want 3", len(state.Levels[0]))
	}
}

func TestDAGBuilder_DuplicateDependency(t *testing.T) {
	dag := NewDAGBuilder()

	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second", core.PhasePlan)

	dag.AddTask(task1)
	dag.AddTask(task2)

	// Add same dependency twice
	dag.AddDependency("task-2", "task-1")
	dag.AddDependency("task-2", "task-1") // Duplicate

	deps := dag.GetDependencies("task-2")
	if len(deps) != 1 {
		t.Errorf("GetDependencies() = %d, want 1 (no duplicates)", len(deps))
	}
}

func TestDAGBuilder_SkipsRunningTasks(t *testing.T) {
	dag := NewDAGBuilder()

	task1 := core.NewTask("task-1", "First", core.PhaseAnalyze)
	task2 := core.NewTask("task-2", "Second", core.PhaseAnalyze)
	task1.Status = core.TaskStatusRunning

	dag.AddTask(task1)
	dag.AddTask(task2)

	completed := make(map[core.TaskID]bool)
	ready := dag.GetReadyTasks(completed)

	// Only task-2 should be ready (task-1 is running)
	if len(ready) != 1 || ready[0].ID != "task-2" {
		t.Errorf("GetReadyTasks() = %v, want [task-2]", taskIDs(ready))
	}
}

func TestDAGBuilder_ComplexGraph(t *testing.T) {
	dag := NewDAGBuilder()

	// Create a more complex graph:
	//      A
	//     / \
	//    B   C
	//   / \ / \
	//  D   E   F
	//   \ / \ /
	//    G   H

	tasks := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for _, id := range tasks {
		dag.AddTask(core.NewTask(core.TaskID(id), "Task "+id, core.PhaseAnalyze))
	}

	dependencies := [][2]string{
		{"B", "A"},
		{"C", "A"},
		{"D", "B"},
		{"E", "B"},
		{"E", "C"},
		{"F", "C"},
		{"G", "D"},
		{"G", "E"},
		{"H", "E"},
		{"H", "F"},
	}

	for _, dep := range dependencies {
		dag.AddDependency(core.TaskID(dep[0]), core.TaskID(dep[1]))
	}

	state, err := dag.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Logf("Order: %v", state.Order)
	t.Logf("Levels: %v", state.Levels)

	// Verify order respects dependencies
	indexOf := func(id core.TaskID) int {
		for i, tid := range state.Order {
			if tid == id {
				return i
			}
		}
		return -1
	}

	for _, dep := range dependencies {
		from := core.TaskID(dep[0])
		to := core.TaskID(dep[1])
		if indexOf(to) > indexOf(from) {
			t.Errorf("%s should come before %s in topological order", to, from)
		}
	}
}

// Helper function to extract task IDs
func taskIDs(tasks []*core.Task) []core.TaskID {
	ids := make([]core.TaskID, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}
