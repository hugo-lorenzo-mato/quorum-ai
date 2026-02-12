package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/snapshot"
	"gopkg.in/yaml.v3"
)

func TestSnapshotEndpoints_ExportValidateImportDryRun(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectPath := filepath.Join(t.TempDir(), "project-a")
	if err := os.MkdirAll(filepath.Join(projectPath, ".quorum", "state"), 0o750); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, ".quorum", "state", "state.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, ".quorum", "config.yaml"), []byte("log:\n  level: info\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	registryPath := filepath.Join(homeDir, ".quorum-registry", "projects.yaml")
	globalPath := filepath.Join(homeDir, ".quorum-registry", "global-config.yaml")
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}

	now := time.Now().UTC()
	cfg := &project.RegistryConfig{
		Version:        1,
		DefaultProject: "proj-1",
		Projects: []*project.Project{{
			ID:           "proj-1",
			Path:         projectPath,
			Name:         "Project A",
			Status:       project.StatusHealthy,
			CreatedAt:    now,
			LastAccessed: now,
			ConfigMode:   project.ConfigModeCustom,
		}},
	}
	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml marshal: %v", err)
	}
	if err := os.WriteFile(registryPath, cfgData, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if err := os.WriteFile(globalPath, []byte("global-config: true\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	srv := NewServer(newMockStateManager(), events.New(10))
	h := srv.Handler()

	snapshotPath := filepath.Join(t.TempDir(), "api-snapshot.tar.gz")
	exportReqBody := SnapshotExportRequest{
		OutputPath:       snapshotPath,
		IncludeWorktrees: false,
	}
	body, _ := json.Marshal(exportReqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/export", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export status = %d body=%s", w.Code, w.Body.String())
	}

	var exportResp snapshot.ExportResult
	if err := json.NewDecoder(w.Body).Decode(&exportResp); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if exportResp.Manifest == nil || exportResp.Manifest.ProjectCount != 1 {
		t.Fatalf("unexpected export manifest: %+v", exportResp.Manifest)
	}

	validateReqBody := SnapshotValidateRequest{InputPath: snapshotPath}
	body, _ = json.Marshal(validateReqBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/import/validate", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", w.Code, w.Body.String())
	}

	var manifest snapshot.Manifest
	if err := json.NewDecoder(w.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	if manifest.ProjectCount != 1 {
		t.Fatalf("manifest.ProjectCount = %d, want 1", manifest.ProjectCount)
	}

	importReqBody := SnapshotImportRequest{
		InputPath:          snapshotPath,
		Mode:               string(snapshot.ImportModeMerge),
		DryRun:             true,
		ConflictPolicy:     string(snapshot.ConflictSkip),
		IncludeWorktrees:   false,
		PreserveProjectIDs: true,
	}
	body, _ = json.Marshal(importReqBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/import", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import status = %d body=%s", w.Code, w.Body.String())
	}

	var importResp snapshot.ImportReport
	if err := json.NewDecoder(w.Body).Decode(&importResp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if !importResp.DryRun {
		t.Fatalf("expected dry_run=true in response")
	}
}

func TestSnapshotEndpoint_ExportBadRequest(t *testing.T) {
	srv := NewServer(newMockStateManager(), events.New(10))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/export", bytes.NewReader([]byte(`{"output_path":""}`)))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestSnapshotEndpoint_ValidateBadRequest(t *testing.T) {
	srv := NewServer(newMockStateManager(), events.New(10))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/validate", bytes.NewReader([]byte(`{"input_path":""}`)))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}
