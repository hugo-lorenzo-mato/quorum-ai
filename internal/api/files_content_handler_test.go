package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServerForFiles(root string) *Server {
	return &Server{
		root:   root,
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
}

func TestHandleGetFileContent_PathRequired(t *testing.T) {
	s := newTestServerForFiles(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/files/content", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "path is required" {
		t.Fatalf("error=%q want=%q", body["error"], "path is required")
	}
}

func TestHandleGetFileContent_InvalidPath(t *testing.T) {
	s := newTestServerForFiles(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/files/content?path=../secrets.txt", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != msgInvalidPath {
		t.Fatalf("error=%q want=%q", body["error"], msgInvalidPath)
	}
}

func TestHandleGetFileContent_NotFound(t *testing.T) {
	s := newTestServerForFiles(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/files/content?path=missing.txt", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusNotFound)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "file not found" {
		t.Fatalf("error=%q want=%q", body["error"], "file not found")
	}
}

func TestHandleGetFileContent_DirectoryRejected(t *testing.T) {
	root := t.TempDir()
	s := newTestServerForFiles(root)
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/content?path=dir", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if !strings.Contains(body["error"], "directory") {
		t.Fatalf("error=%q want contains %q", body["error"], "directory")
	}
}

func TestHandleGetFileContent_TooLarge(t *testing.T) {
	root := t.TempDir()
	s := newTestServerForFiles(root)
	p := filepath.Join(root, "big.bin")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Make it slightly larger than the 10MB limit without allocating huge buffers.
	if err := os.Truncate(p, 10*1024*1024+1); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/content?path=big.bin", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "file too large" {
		t.Fatalf("error=%q want=%q", body["error"], "file too large")
	}
}

func TestHandleGetFileContent_OK_Text(t *testing.T) {
	root := t.TempDir()
	s := newTestServerForFiles(root)
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/content?path=hello.txt", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}
	var resp FileContentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Path != "hello.txt" {
		t.Fatalf("Path=%q want=%q", resp.Path, "hello.txt")
	}
	if resp.Binary {
		t.Fatalf("Binary=true want=false")
	}
	if resp.Content != "hello" {
		t.Fatalf("Content=%q want=%q", resp.Content, "hello")
	}
}
