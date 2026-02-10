package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TaskResponse is the API response for a task.
type TaskResponse struct {
	ID           string     `json:"id"`
	Phase        string     `json:"phase"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Status       string     `json:"status"`
	CLI          string     `json:"cli"`
	Model        string     `json:"model"`
	Dependencies []string   `json:"dependencies"`
	TokensIn     int        `json:"tokens_in"`
	TokensOut    int        `json:"tokens_out"`
	Retries      int        `json:"retries"`
	Error        string     `json:"error,omitempty"`
	WorktreePath string     `json:"worktree_path,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Output       string     `json:"output,omitempty"`
	OutputFile   string     `json:"output_file,omitempty"`
}

// UpdateTaskRequest is the request body for updating a task.
type UpdateTaskRequest struct {
	Name         *string  `json:"name,omitempty"`
	CLI          *string  `json:"cli,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// CreateTaskRequest is the request body for creating a new task.
type CreateTaskRequest struct {
	Name         string   `json:"name"`
	CLI          string   `json:"cli"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// ReorderTasksRequest is the request body for reordering tasks.
type ReorderTasksRequest struct {
	TaskOrder []string `json:"task_order"`
}

// handleListTasks returns all tasks for a workflow.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Return tasks in order
	response := make([]TaskResponse, 0, len(state.TaskOrder))
	for _, taskID := range state.TaskOrder {
		task, ok := state.Tasks[taskID]
		if !ok {
			continue
		}
		response = append(response, taskStateToResponse(task))
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetTask returns a specific task from a workflow.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	taskID := chi.URLParam(r, "taskID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	if taskID == "" {
		respondError(w, http.StatusBadRequest, "task ID is required")
		return
	}

	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	task, ok := state.Tasks[core.TaskID(taskID)]
	if !ok {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	response := taskStateToResponse(task)
	respondJSON(w, http.StatusOK, response)
}

// handleCreateTask creates a new task in a workflow.
// POST /api/v1/workflows/{workflowID}/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	stateManager := GetStateManagerFromContext(ctx, s.stateManager)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Only allow task mutation in awaiting_review or completed states, with plan generated
	if !canMutateTasks(state) {
		respondError(w, http.StatusConflict, "tasks can only be modified when workflow is awaiting review or completed, after planning phase")
		return
	}

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.CLI == "" {
		respondError(w, http.StatusBadRequest, "cli is required")
		return
	}

	// Generate task ID
	taskID := core.TaskID(generateTaskID())

	// Build dependencies
	deps := make([]core.TaskID, 0, len(req.Dependencies))
	for _, d := range req.Dependencies {
		depID := core.TaskID(d)
		if _, ok := state.Tasks[depID]; !ok {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("dependency task not found: %s", d))
			return
		}
		deps = append(deps, depID)
	}

	// Create the task state
	newTask := &core.TaskState{
		ID:           taskID,
		Phase:        core.PhaseExecute,
		Name:         req.Name,
		Description:  req.Description,
		Status:       core.TaskStatusPending,
		CLI:          req.CLI,
		Dependencies: deps,
	}

	// Add to state
	if state.Tasks == nil {
		state.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	state.Tasks[taskID] = newTask
	state.TaskOrder = append(state.TaskOrder, taskID)

	// Validate DAG
	if err := validateTaskDAG(state); err != nil {
		// Rollback
		delete(state.Tasks, taskID)
		state.TaskOrder = state.TaskOrder[:len(state.TaskOrder)-1]
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save workflow")
		return
	}

	respondJSON(w, http.StatusCreated, taskStateToResponse(newTask))
}

// handleUpdateTask updates a task in a workflow.
// PATCH /api/v1/workflows/{workflowID}/tasks/{taskID}
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	taskID := chi.URLParam(r, "taskID")
	stateManager := GetStateManagerFromContext(ctx, s.stateManager)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	if !canMutateTasks(state) {
		respondError(w, http.StatusConflict, "tasks can only be modified when workflow is awaiting review or completed, after planning phase")
		return
	}

	task, ok := state.Tasks[core.TaskID(taskID)]
	if !ok {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply updates
	if req.Name != nil {
		if *req.Name == "" {
			respondError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		task.Name = *req.Name
	}
	if req.CLI != nil {
		if *req.CLI == "" {
			respondError(w, http.StatusBadRequest, "cli cannot be empty")
			return
		}
		task.CLI = *req.CLI
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Dependencies != nil {
		deps := make([]core.TaskID, 0, len(req.Dependencies))
		for _, d := range req.Dependencies {
			depID := core.TaskID(d)
			if _, ok := state.Tasks[depID]; !ok {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("dependency task not found: %s", d))
				return
			}
			if depID == task.ID {
				respondError(w, http.StatusBadRequest, "task cannot depend on itself")
				return
			}
			deps = append(deps, depID)
		}
		oldDeps := task.Dependencies
		task.Dependencies = deps

		// Validate DAG
		if err := validateTaskDAG(state); err != nil {
			task.Dependencies = oldDeps // Rollback
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save workflow")
		return
	}

	respondJSON(w, http.StatusOK, taskStateToResponse(task))
}

// handleDeleteTask deletes a task from a workflow.
// DELETE /api/v1/workflows/{workflowID}/tasks/{taskID}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	taskID := chi.URLParam(r, "taskID")
	stateManager := GetStateManagerFromContext(ctx, s.stateManager)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	if !canMutateTasks(state) {
		respondError(w, http.StatusConflict, "tasks can only be modified when workflow is awaiting review or completed, after planning phase")
		return
	}

	tid := core.TaskID(taskID)
	if _, ok := state.Tasks[tid]; !ok {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	// Check if other tasks depend on this task
	for otherID, otherTask := range state.Tasks {
		if otherID == tid {
			continue
		}
		for _, dep := range otherTask.Dependencies {
			if dep == tid {
				respondError(w, http.StatusConflict,
					fmt.Sprintf("cannot delete task: task %s depends on it", otherID))
				return
			}
		}
	}

	// Remove from Tasks map
	delete(state.Tasks, tid)

	// Remove from TaskOrder
	newOrder := make([]core.TaskID, 0, len(state.TaskOrder)-1)
	for _, id := range state.TaskOrder {
		if id != tid {
			newOrder = append(newOrder, id)
		}
	}
	state.TaskOrder = newOrder

	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save workflow")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleReorderTasks reorders tasks in a workflow.
// PUT /api/v1/workflows/{workflowID}/tasks/reorder
func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	stateManager := GetStateManagerFromContext(ctx, s.stateManager)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	if !canMutateTasks(state) {
		respondError(w, http.StatusConflict, "tasks can only be modified when workflow is awaiting review or completed, after planning phase")
		return
	}

	var req ReorderTasksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate that the new order contains exactly the same task IDs
	if len(req.TaskOrder) != len(state.TaskOrder) {
		respondError(w, http.StatusBadRequest, "task_order must contain all tasks")
		return
	}

	newOrderSet := make(map[core.TaskID]bool, len(req.TaskOrder))
	newOrder := make([]core.TaskID, 0, len(req.TaskOrder))
	for _, id := range req.TaskOrder {
		tid := core.TaskID(id)
		if _, ok := state.Tasks[tid]; !ok {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown task: %s", id))
			return
		}
		if newOrderSet[tid] {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("duplicate task in order: %s", id))
			return
		}
		newOrderSet[tid] = true
		newOrder = append(newOrder, tid)
	}

	state.TaskOrder = newOrder
	state.UpdatedAt = time.Now()

	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save workflow")
		return
	}

	// Return tasks in new order
	response := make([]TaskResponse, 0, len(state.TaskOrder))
	for _, taskID := range state.TaskOrder {
		task, ok := state.Tasks[taskID]
		if !ok {
			continue
		}
		response = append(response, taskStateToResponse(task))
	}

	respondJSON(w, http.StatusOK, response)
}

// canMutateTasks checks if a workflow is in a state where tasks can be modified.
// Tasks can only be mutated after planning (current_phase >= execute) and when
// the workflow is paused for review or completed.
func canMutateTasks(state *core.WorkflowState) bool {
	if state.CurrentPhase != core.PhaseExecute && state.CurrentPhase != core.PhaseDone {
		return false
	}
	switch state.Status {
	case core.WorkflowStatusAwaitingReview, core.WorkflowStatusCompleted, core.WorkflowStatusPaused:
		return true
	default:
		return false
	}
}

// validateTaskDAG checks that the task dependency graph is acyclic.
func validateTaskDAG(state *core.WorkflowState) error {
	// Build adjacency list
	adj := make(map[core.TaskID][]core.TaskID)
	for _, task := range state.Tasks {
		adj[task.ID] = task.Dependencies
	}

	// DFS-based cycle detection
	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)
	color := make(map[core.TaskID]int)

	var visit func(id core.TaskID) error
	visit = func(id core.TaskID) error {
		color[id] = gray
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				return fmt.Errorf("circular dependency detected involving task %s", dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for id := range state.Tasks {
		if color[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateTaskID generates a random task ID.
func generateTaskID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("task_%d", time.Now().UnixNano())
	}
	return "task_" + hex.EncodeToString(b)
}

// taskStateToResponse converts a TaskState to a TaskResponse.
func taskStateToResponse(task *core.TaskState) TaskResponse {
	deps := make([]string, 0, len(task.Dependencies))
	for _, d := range task.Dependencies {
		deps = append(deps, string(d))
	}

	return TaskResponse{
		ID:           string(task.ID),
		Phase:        string(task.Phase),
		Name:         task.Name,
		Description:  task.Description,
		Status:       string(task.Status),
		CLI:          task.CLI,
		Model:        task.Model,
		Dependencies: deps,
		TokensIn:     task.TokensIn,
		TokensOut:    task.TokensOut,
		Retries:      task.Retries,
		Error:        task.Error,
		WorktreePath: task.WorktreePath,
		StartedAt:    task.StartedAt,
		CompletedAt:  task.CompletedAt,
		Output:       task.Output,
		OutputFile:   task.OutputFile,
	}
}
