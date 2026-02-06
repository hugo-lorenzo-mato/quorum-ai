package api

import (
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
