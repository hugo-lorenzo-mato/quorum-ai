package api

import (
	"context"
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
		respondError(w, http.StatusNotFound, msgTaskNotFound)
		return
	}

	response := taskStateToResponse(task)
	respondJSON(w, http.StatusOK, response)
}

// loadMutableTaskState loads a workflow state and validates it can accept task mutations.
// Returns (state, stateManager, ok). If ok is false, an error response was already written.
func (s *Server) loadMutableTaskState(w http.ResponseWriter, r *http.Request) (*core.WorkflowState, core.StateManager, bool) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")
	stateManager := GetStateManagerFromContext(ctx, s.stateManager)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return nil, nil, false
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return nil, nil, false
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return nil, nil, false
	}

	if !canMutateTasks(state) {
		respondError(w, http.StatusConflict, "tasks can only be modified when workflow is awaiting review or completed, after planning phase")
		return nil, nil, false
	}

	return state, stateManager, true
}

// saveMutatedTaskState saves a workflow state after task mutation and handles errors.
// Returns true if save succeeded.
func (s *Server) saveMutatedTaskState(w http.ResponseWriter, ctx context.Context, state *core.WorkflowState, stateManager core.StateManager) bool {
	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		workflowID := string(state.WorkflowID)
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save workflow")
		return false
	}
	return true
}

// handleCreateTask creates a new task in a workflow.
// POST /api/v1/workflows/{workflowID}/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	state, stateManager, ok := s.loadMutableTaskState(w, r)
	if !ok {
		return
	}

	var req CreateTaskRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respondError(w, http.StatusBadRequest, msgInvalidRequestBody)
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

	if !s.saveMutatedTaskState(w, r.Context(), state, stateManager) {
		return
	}

	respondJSON(w, http.StatusCreated, taskStateToResponse(newTask))
}

// handleUpdateTask updates a task in a workflow.
// PATCH /api/v1/workflows/{workflowID}/tasks/{taskID}
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	state, stateManager, ok := s.loadMutableTaskState(w, r)
	if !ok {
		return
	}

	taskID := chi.URLParam(r, "taskID")
	task, ok := state.Tasks[core.TaskID(taskID)]
	if !ok {
		respondError(w, http.StatusNotFound, msgTaskNotFound)
		return
	}

	var req UpdateTaskRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respondError(w, http.StatusBadRequest, msgInvalidRequestBody)
		return
	}

	if status, msg, ok := applyUpdateTaskRequest(state, task, req); !ok {
		respondError(w, status, msg)
		return
	}

	if !s.saveMutatedTaskState(w, r.Context(), state, stateManager) {
		return
	}

	respondJSON(w, http.StatusOK, taskStateToResponse(task))
}

// handleDeleteTask deletes a task from a workflow.
// DELETE /api/v1/workflows/{workflowID}/tasks/{taskID}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	state, stateManager, ok := s.loadMutableTaskState(w, r)
	if !ok {
		return
	}

	taskID := chi.URLParam(r, "taskID")
	tid := core.TaskID(taskID)
	if _, ok := state.Tasks[tid]; !ok {
		respondError(w, http.StatusNotFound, msgTaskNotFound)
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

	if !s.saveMutatedTaskState(w, r.Context(), state, stateManager) {
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleReorderTasks reorders tasks in a workflow.
// PUT /api/v1/workflows/{workflowID}/tasks/reorder
func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	state, stateManager, ok := s.loadMutableTaskState(w, r)
	if !ok {
		return
	}

	var req ReorderTasksRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		respondError(w, http.StatusBadRequest, msgInvalidRequestBody)
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

	if !s.saveMutatedTaskState(w, r.Context(), state, stateManager) {
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
	case core.WorkflowStatusAwaitingReview, core.WorkflowStatusCompleted:
		return true
	default:
		return false
	}
}

// validateTaskDAG checks that the task dependency graph is acyclic.
func validateTaskDAG(state *core.WorkflowState) error {
	adj := buildTaskAdjacency(state)
	return detectTaskCycle(adj)
}

func buildTaskAdjacency(state *core.WorkflowState) map[core.TaskID][]core.TaskID {
	adj := make(map[core.TaskID][]core.TaskID, len(state.Tasks))
	for _, task := range state.Tasks {
		adj[task.ID] = task.Dependencies
	}
	return adj
}

// detectTaskCycle performs DFS-based cycle detection.
func detectTaskCycle(adj map[core.TaskID][]core.TaskID) error {
	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)
	color := make(map[core.TaskID]int)

	for id := range adj {
		if color[id] != white {
			continue
		}
		if err := visitTaskForCycle(id, adj, color, white, gray, black); err != nil {
			return err
		}
	}
	return nil
}

func visitTaskForCycle(id core.TaskID, adj map[core.TaskID][]core.TaskID, color map[core.TaskID]int, white, gray, black int) error {
	color[id] = gray
	for _, dep := range adj[id] {
		depColor := color[dep]
		if depColor == gray {
			return fmt.Errorf("circular dependency detected involving task %s", dep)
		}
		if depColor == white {
			if err := visitTaskForCycle(dep, adj, color, white, gray, black); err != nil {
				return err
			}
		}
	}
	color[id] = black
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

func applyUpdateTaskRequest(state *core.WorkflowState, task *core.TaskState, req UpdateTaskRequest) (int, string, bool) {
	if req.Name != nil {
		if *req.Name == "" {
			return http.StatusBadRequest, "name cannot be empty", false
		}
		task.Name = *req.Name
	}
	if req.CLI != nil {
		if *req.CLI == "" {
			return http.StatusBadRequest, "cli cannot be empty", false
		}
		task.CLI = *req.CLI
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Dependencies != nil {
		deps, status, msg, ok := buildDependenciesForUpdate(state, task.ID, req.Dependencies)
		if !ok {
			return status, msg, false
		}
		oldDeps := task.Dependencies
		task.Dependencies = deps
		if err := validateTaskDAG(state); err != nil {
			task.Dependencies = oldDeps
			return http.StatusBadRequest, err.Error(), false
		}
	}
	return 0, "", true
}

func buildDependenciesForUpdate(state *core.WorkflowState, taskID core.TaskID, deps []string) ([]core.TaskID, int, string, bool) {
	out := make([]core.TaskID, 0, len(deps))
	for _, d := range deps {
		depID := core.TaskID(d)
		if _, ok := state.Tasks[depID]; !ok {
			return nil, http.StatusBadRequest, fmt.Sprintf("dependency task not found: %s", d), false
		}
		if depID == taskID {
			return nil, http.StatusBadRequest, "task cannot depend on itself", false
		}
		out = append(out, depID)
	}
	return out, 0, "", true
}
