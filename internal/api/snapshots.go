package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/snapshot"
)

// SnapshotExportRequest defines request payload for snapshot export.
type SnapshotExportRequest struct {
	OutputPath       string   `json:"output_path"`
	ProjectIDs       []string `json:"project_ids,omitempty"`
	IncludeWorktrees bool     `json:"include_worktrees,omitempty"`
	QuorumVersion    string   `json:"quorum_version,omitempty"`
}

// SnapshotImportRequest defines request payload for snapshot import.
type SnapshotImportRequest struct {
	InputPath          string            `json:"input_path"`
	Mode               string            `json:"mode,omitempty"`
	DryRun             bool              `json:"dry_run,omitempty"`
	ConflictPolicy     string            `json:"conflict_policy,omitempty"`
	PathMap            map[string]string `json:"path_map,omitempty"`
	PreserveProjectIDs bool              `json:"preserve_project_ids,omitempty"`
	IncludeWorktrees   bool              `json:"include_worktrees,omitempty"`
}

// SnapshotValidateRequest defines request payload for snapshot validation.
type SnapshotValidateRequest struct {
	InputPath string `json:"input_path"`
}

func (s *Server) handleSnapshotExport(w http.ResponseWriter, r *http.Request) {
	var req SnapshotExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		respondError(w, http.StatusBadRequest, "output_path is required")
		return
	}

	result, err := snapshot.Export(&snapshot.ExportOptions{
		OutputPath:       req.OutputPath,
		ProjectIDs:       req.ProjectIDs,
		IncludeWorktrees: req.IncludeWorktrees,
		QuorumVersion:    req.QuorumVersion,
	})
	if err != nil {
		respondError(w, statusForSnapshotError(err), err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleSnapshotImport(w http.ResponseWriter, r *http.Request) {
	var req SnapshotImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.InputPath) == "" {
		respondError(w, http.StatusBadRequest, "input_path is required")
		return
	}

	report, err := snapshot.Import(&snapshot.ImportOptions{
		InputPath:          req.InputPath,
		Mode:               snapshot.ImportMode(req.Mode),
		DryRun:             req.DryRun,
		ConflictPolicy:     snapshot.ConflictPolicy(req.ConflictPolicy),
		PathMap:            req.PathMap,
		PreserveProjectIDs: req.PreserveProjectIDs,
		IncludeWorktrees:   req.IncludeWorktrees,
	})
	if err != nil {
		respondError(w, statusForSnapshotError(err), err.Error())
		return
	}

	respondJSON(w, http.StatusOK, report)
}

func (s *Server) handleSnapshotValidate(w http.ResponseWriter, r *http.Request) {
	var req SnapshotValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.InputPath) == "" {
		respondError(w, http.StatusBadRequest, "input_path is required")
		return
	}

	manifest, err := snapshot.ValidateSnapshot(req.InputPath)
	if err != nil {
		respondError(w, statusForSnapshotError(err), err.Error())
		return
	}
	respondJSON(w, http.StatusOK, manifest)
}

func statusForSnapshotError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "required"), strings.Contains(msg, "missing"):
		return http.StatusBadRequest
	case strings.Contains(msg, "conflict"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
