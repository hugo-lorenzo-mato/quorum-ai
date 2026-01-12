package service

import (
	"fmt"
	"sync"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// DAGBuilder constructs and manages task dependency graphs.
type DAGBuilder struct {
	tasks   map[core.TaskID]*core.Task
	edges   map[core.TaskID][]core.TaskID // task -> dependencies
	reverse map[core.TaskID][]core.TaskID // task -> dependents
	mu      sync.RWMutex
}

// NewDAGBuilder creates a new DAG builder.
func NewDAGBuilder() *DAGBuilder {
	return &DAGBuilder{
		tasks:   make(map[core.TaskID]*core.Task),
		edges:   make(map[core.TaskID][]core.TaskID),
		reverse: make(map[core.TaskID][]core.TaskID),
	}
}

// AddTask adds a task to the DAG.
func (d *DAGBuilder) AddTask(task *core.Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	d.tasks[task.ID] = task
	d.edges[task.ID] = make([]core.TaskID, 0)
	d.reverse[task.ID] = make([]core.TaskID, 0)

	return nil
}

// AddDependency adds a dependency: from depends on to.
func (d *DAGBuilder) AddDependency(from, to core.TaskID) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.tasks[from]; !exists {
		return fmt.Errorf("task %s not found", from)
	}
	if _, exists := d.tasks[to]; !exists {
		return fmt.Errorf("task %s not found", to)
	}

	// Check if dependency already exists
	for _, dep := range d.edges[from] {
		if dep == to {
			return nil // Already exists
		}
	}

	d.edges[from] = append(d.edges[from], to)
	d.reverse[to] = append(d.reverse[to], from)

	return nil
}

// DAGState represents a validated DAG.
type DAGState struct {
	Tasks        map[core.TaskID]*core.Task
	Order        []core.TaskID
	Levels       [][]core.TaskID
	Dependencies map[core.TaskID][]core.TaskID
}

// Build validates the DAG and returns the state.
func (d *DAGBuilder) Build() (*DAGState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Detect cycles
	if err := d.detectCycle(); err != nil {
		return nil, err
	}

	// Topological sort
	order, err := d.topologicalSort()
	if err != nil {
		return nil, err
	}

	// Calculate levels for parallel execution
	levels := d.calculateLevels()

	return &DAGState{
		Tasks:        d.copyTasks(),
		Order:        order,
		Levels:       levels,
		Dependencies: d.copyEdges(),
	}, nil
}

// topologicalSort returns tasks in dependency order using Kahn's algorithm.
func (d *DAGBuilder) topologicalSort() ([]core.TaskID, error) {
	// Calculate in-degrees
	inDegree := make(map[core.TaskID]int)
	for id := range d.tasks {
		inDegree[id] = len(d.edges[id])
	}

	// Find tasks with no dependencies
	queue := make([]core.TaskID, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Process queue
	result := make([]core.TaskID, 0, len(d.tasks))
	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Update dependents
		for _, dependent := range d.reverse[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(d.tasks) {
		return nil, core.ErrValidation("CYCLE_DETECTED", "task dependency graph contains a cycle")
	}

	return result, nil
}

// detectCycle checks for cycles in the graph using DFS.
func (d *DAGBuilder) detectCycle() error {
	visited := make(map[core.TaskID]bool)
	recStack := make(map[core.TaskID]bool)

	var dfs func(id core.TaskID) bool
	dfs = func(id core.TaskID) bool {
		visited[id] = true
		recStack[id] = true

		for _, dep := range d.edges[id] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	for id := range d.tasks {
		if !visited[id] {
			if dfs(id) {
				return core.ErrValidation("CYCLE_DETECTED", "task dependency graph contains a cycle")
			}
		}
	}

	return nil
}

// calculateLevels groups tasks into parallel execution levels.
func (d *DAGBuilder) calculateLevels() [][]core.TaskID {
	if len(d.tasks) == 0 {
		return nil
	}

	levels := make([][]core.TaskID, 0)
	assigned := make(map[core.TaskID]bool)

	for len(assigned) < len(d.tasks) {
		level := make([]core.TaskID, 0)

		for id := range d.tasks {
			if assigned[id] {
				continue
			}

			// Check if all dependencies are assigned
			allDepsAssigned := true
			for _, dep := range d.edges[id] {
				if !assigned[dep] {
					allDepsAssigned = false
					break
				}
			}

			if allDepsAssigned {
				level = append(level, id)
			}
		}

		// Mark level tasks as assigned
		for _, id := range level {
			assigned[id] = true
		}

		levels = append(levels, level)
	}

	return levels
}

// GetReadyTasks returns tasks ready for execution.
func (d *DAGBuilder) GetReadyTasks(completed map[core.TaskID]bool) []*core.Task {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ready := make([]*core.Task, 0)

	for id, task := range d.tasks {
		// Skip already completed
		if completed[id] {
			continue
		}

		// Skip already running
		if task.Status == core.TaskStatusRunning {
			continue
		}

		// Check all dependencies completed
		allDepsComplete := true
		for _, dep := range d.edges[id] {
			if !completed[dep] {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, task)
		}
	}

	return ready
}

// GetTask returns a task by ID.
func (d *DAGBuilder) GetTask(id core.TaskID) (*core.Task, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	task, ok := d.tasks[id]
	return task, ok
}

// GetDependencies returns task dependencies.
func (d *DAGBuilder) GetDependencies(id core.TaskID) []core.TaskID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	deps := d.edges[id]
	if deps == nil {
		return nil
	}
	result := make([]core.TaskID, len(deps))
	copy(result, deps)
	return result
}

// GetDependents returns tasks that depend on the given task.
func (d *DAGBuilder) GetDependents(id core.TaskID) []core.TaskID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	deps := d.reverse[id]
	if deps == nil {
		return nil
	}
	result := make([]core.TaskID, len(deps))
	copy(result, deps)
	return result
}

// TaskCount returns the number of tasks in the DAG.
func (d *DAGBuilder) TaskCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.tasks)
}

func (d *DAGBuilder) copyEdges() map[core.TaskID][]core.TaskID {
	result := make(map[core.TaskID][]core.TaskID)
	for k, v := range d.edges {
		result[k] = append([]core.TaskID{}, v...)
	}
	return result
}

func (d *DAGBuilder) copyTasks() map[core.TaskID]*core.Task {
	result := make(map[core.TaskID]*core.Task)
	for k, v := range d.tasks {
		result[k] = v
	}
	return result
}
