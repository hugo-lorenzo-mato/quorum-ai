package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// handleListWorkflowAttachments lists attachments associated with a workflow.
func (s *Server) handleListWorkflowAttachments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
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

	respondJSON(w, http.StatusOK, state.Attachments)
}

// handleUploadWorkflowAttachments uploads one or more files as workflow attachments.
func (s *Server) handleUploadWorkflowAttachments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)
	store := s.getProjectAttachmentStore(ctx)
	if store == nil {
		respondError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
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
	if state.Status == core.WorkflowStatusRunning {
		respondError(w, http.StatusConflict, "cannot modify attachments while workflow is running")
		return
	}

	// Limit total upload size (best-effort). Per-file limits are enforced by the store.
	r.Body = http.MaxBytesReader(w, r.Body, int64(attachments.MaxAttachmentSizeBytes)*10)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	form := r.MultipartForm
	if form == nil || len(form.File) == 0 {
		respondError(w, http.StatusBadRequest, "no files provided")
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		respondError(w, http.StatusBadRequest, "no files provided")
		return
	}

	saved := make([]core.Attachment, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			respondError(w, http.StatusBadRequest, "failed to open uploaded file")
			return
		}
		att, err := store.Save(attachments.OwnerWorkflow, workflowID, f, fh.Filename)
		_ = f.Close()
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		saved = append(saved, att)
	}

	// Persist in workflow state (canonical for prompting).
	state.Attachments = append(state.Attachments, saved...)
	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow with attachments", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to persist attachments")
		return
	}

	respondJSON(w, http.StatusCreated, saved)
}

// handleDownloadWorkflowAttachment downloads a workflow attachment.
func (s *Server) handleDownloadWorkflowAttachment(w http.ResponseWriter, r *http.Request) {
	store := s.getProjectAttachmentStore(r.Context())
	if store == nil {
		respondError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")
	attachmentID := chi.URLParam(r, "attachmentID")
	if workflowID == "" || attachmentID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID and attachment ID are required")
		return
	}

	meta, absPath, err := store.Resolve(attachments.OwnerWorkflow, workflowID, attachmentID)
	if err != nil {
		if os.IsNotExist(err) {
			respondError(w, http.StatusNotFound, "attachment not found")
			return
		}
		s.logger.Error("failed to resolve attachment", "workflow_id", workflowID, "attachment_id", attachmentID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to resolve attachment")
		return
	}

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", meta.Name))
	http.ServeFile(w, r, absPath)
}

// handleDeleteWorkflowAttachment deletes a workflow attachment.
func (s *Server) handleDeleteWorkflowAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)
	store := s.getProjectAttachmentStore(ctx)
	if store == nil {
		respondError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")
	attachmentID := chi.URLParam(r, "attachmentID")
	if workflowID == "" || attachmentID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID and attachment ID are required")
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
	if state.Status == core.WorkflowStatusRunning {
		respondError(w, http.StatusConflict, "cannot modify attachments while workflow is running")
		return
	}

	if err := store.Delete(attachments.OwnerWorkflow, workflowID, attachmentID); err != nil {
		if os.IsNotExist(err) {
			respondError(w, http.StatusNotFound, "attachment not found")
			return
		}
		s.logger.Error("failed to delete attachment", "workflow_id", workflowID, "attachment_id", attachmentID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}

	// Remove from state
	filtered := make([]core.Attachment, 0, len(state.Attachments))
	for _, a := range state.Attachments {
		if a.ID != attachmentID {
			filtered = append(filtered, a)
		}
	}
	state.Attachments = filtered
	state.UpdatedAt = time.Now()
	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow after deleting attachment", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to persist attachment deletion")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
