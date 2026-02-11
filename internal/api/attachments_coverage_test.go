package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ---------------------------------------------------------------------------
// Helper: create a Server with an attachment store pointing to tmpDir
// ---------------------------------------------------------------------------

func setupAttachmentsTestServer(t *testing.T) (http.Handler, *mockStateManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))
	return srv.Handler(), sm, tmpDir
}

// ---------------------------------------------------------------------------
// handleListWorkflowAttachments
// ---------------------------------------------------------------------------

func TestHandleListWorkflowAttachments_Success(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-att-1",
			Prompt:     "test",
			CreatedAt:  time.Now(),
			Attachments: []core.Attachment{
				{ID: "att-1", Name: "file.txt", Size: 100, ContentType: "text/plain"},
			},
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-att-1"] = wf

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-att-1/attachments/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var atts []core.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &atts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(atts) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(atts))
	}
}

func TestHandleListWorkflowAttachments_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	handler, _, _ := setupAttachmentsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/attachments/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleListWorkflowAttachments_LoadError(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)
	sm.loadErr = fmt.Errorf("db error")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/attachments/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// handleUploadWorkflowAttachments
// ---------------------------------------------------------------------------

func TestHandleUploadWorkflowAttachments_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	handler, _, _ := setupAttachmentsTestServer(t)

	body, contentType := createMultipartBody(t, "files", "test.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadWorkflowAttachments_RunningWorkflow(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-running",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusRunning, // Running → cannot upload
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-running"] = wf

	body, contentType := createMultipartBody(t, "files", "test.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-running/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadWorkflowAttachments_NoFiles(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-upload",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-upload"] = wf

	// Multipart with wrong field name (not "files").
	body, contentType := createMultipartBody(t, "other", "test.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-upload/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadWorkflowAttachments_Success(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-upload-ok",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-upload-ok"] = wf

	body, contentType := createMultipartBody(t, "files", "test.txt", []byte("hello world"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-upload-ok/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var atts []core.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &atts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(atts) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(atts))
	}
	if atts[0].Name != "test.txt" {
		t.Errorf("expected 'test.txt', got %q", atts[0].Name)
	}
}

func TestHandleUploadWorkflowAttachments_InvalidMultipart(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-bad-upload",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-bad-upload"] = wf

	// Send non-multipart body.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-bad-upload/attachments/", bytes.NewReader([]byte("plain text")))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleDownloadWorkflowAttachment
// ---------------------------------------------------------------------------

func TestHandleDownloadWorkflowAttachment_StoreNotAvailable(t *testing.T) {
	t.Parallel()
	// Build a server without an attachment store by setting root to empty dir.
	// Actually the server always creates one, but a nil store check is done via
	// getProjectAttachmentStore which falls back to server.attachments.
	// We need to test the 404 path for missing attachments.
	handler, sm, tmpDir := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-dl",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-dl"] = wf
	_ = tmpDir

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-dl/attachments/nonexistent-att/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDownloadWorkflowAttachment_Success(t *testing.T) {
	t.Parallel()
	handler, sm, tmpDir := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-dl-ok",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-dl-ok"] = wf

	// Upload a file first.
	body, contentType := createMultipartBody(t, "files", "download-me.txt", []byte("content here"))
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-dl-ok/attachments/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}
	var atts []core.Attachment
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &atts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	attID := atts[0].ID
	_ = tmpDir

	// Download.
	dlReq := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-dl-ok/attachments/"+attID+"/download", nil)
	dlRec := httptest.NewRecorder()
	handler.ServeHTTP(dlRec, dlReq)

	if dlRec.Code != http.StatusOK {
		t.Fatalf("download: expected 200, got %d: %s", dlRec.Code, dlRec.Body.String())
	}

	dlBody := dlRec.Body.String()
	if dlBody != "content here" {
		t.Errorf("expected 'content here', got %q", dlBody)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteWorkflowAttachment
// ---------------------------------------------------------------------------

func TestHandleDeleteWorkflowAttachment_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	handler, _, _ := setupAttachmentsTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/nonexistent/attachments/att-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDeleteWorkflowAttachment_RunningWorkflow(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-del-running",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusRunning,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-del-running"] = wf

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-del-running/attachments/att-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestHandleDeleteWorkflowAttachment_NotFound(t *testing.T) {
	t.Parallel()
	handler, sm, tmpDir := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-del-nf",
			Prompt:     "test",
			CreatedAt:  time.Now(),
			Attachments: []core.Attachment{
				{ID: "att-1", Name: "file.txt", Path: filepath.Join(tmpDir, "nonexistent")},
			},
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-del-nf"] = wf

	// Delete an attachment that has no physical file (should still succeed via store.Delete which
	// just calls os.RemoveAll). The response depends on the store implementation.
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-del-nf/attachments/att-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// RemoveAll on a nonexistent path succeeds, so we should get 204.
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteWorkflowAttachment_Success(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-del-ok",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-del-ok"] = wf

	// Upload first.
	body, contentType := createMultipartBody(t, "files", "deleteme.txt", []byte("delete me"))
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-del-ok/attachments/", body)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}
	var atts []core.Attachment
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &atts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	attID := atts[0].ID

	// Delete.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-del-ok/attachments/"+attID, nil)
	delRec := httptest.NewRecorder()
	handler.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Verify attachment was removed from workflow state.
	updatedWf := sm.workflows["wf-del-ok"]
	for _, a := range updatedWf.Attachments {
		if a.ID == attID {
			t.Error("attachment should have been removed from state")
		}
	}
}

func TestHandleDeleteWorkflowAttachment_LoadError(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)
	sm.loadErr = fmt.Errorf("db error")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-1/attachments/att-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleUploadWorkflowAttachments_LoadError(t *testing.T) {
	t.Parallel()
	handler, sm, _ := setupAttachmentsTestServer(t)
	sm.loadErr = fmt.Errorf("db error")

	body, contentType := createMultipartBody(t, "files", "test.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Multipart helper
// ---------------------------------------------------------------------------

func createMultipartBody(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// ---------------------------------------------------------------------------
// handleUploadWorkflowAttachments_SaveError (state manager save fails)
// ---------------------------------------------------------------------------

func TestHandleUploadWorkflowAttachments_SaveError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))
	handler := srv.Handler()

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-save-err",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
		},
	}
	sm.workflows["wf-save-err"] = wf

	// Upload file, but set save error after workflow loads
	body, contentType := createMultipartBody(t, "files", "test.txt", []byte("hello"))
	// We can't easily set the error between load and save, but we can set it now.
	// The upload handler loads the workflow first (which succeeds), then saves.
	// Since mockStateManager.Save checks saveErr, we set it now and the load
	// uses LoadByID which checks loadErr (different field).
	sm.saveErr = fmt.Errorf("save failed")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-save-err/attachments/", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleDownloadWorkflowAttachment — missing IDs
// ---------------------------------------------------------------------------

func TestHandleDownloadWorkflowAttachment_MissingResolveError(t *testing.T) {
	t.Parallel()
	handler, _, tmpDir := setupAttachmentsTestServer(t)

	// Create a meta.json that references a non-existent file to trigger os.IsNotExist in Resolve.
	attDir := filepath.Join(tmpDir, ".quorum", "attachments", "workflows", "wf-x", "att-bad")
	if err := os.MkdirAll(attDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Don't create meta.json, so Resolve will return os.ErrNotExist.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-x/attachments/att-bad/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
