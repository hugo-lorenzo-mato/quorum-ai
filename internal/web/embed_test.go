package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDistFS(t *testing.T) {
	distFS, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v", err)
	}

	// Verify index.html exists
	f, err := distFS.Open("index.html")
	if err != nil {
		t.Fatalf("Open(index.html) error = %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if stat.Size() == 0 {
		t.Error("index.html should not be empty")
	}
}

func TestStaticHandler(t *testing.T) {
	handler, err := StaticHandler()
	if err != nil {
		t.Fatalf("StaticHandler() error = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantInBody string
	}{
		{
			name:       "root serves index.html",
			path:       "/",
			wantStatus: http.StatusOK,
			wantInBody: "<!doctype html>",
		},
		{
			name:       "index.html direct",
			path:       "/index.html",
			wantStatus: http.StatusOK,
			wantInBody: "<!doctype html>",
		},
		{
			name:       "SPA fallback for unknown path",
			path:       "/some/spa/route",
			wantStatus: http.StatusOK,
			wantInBody: "<!doctype html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantInBody != "" && !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("body does not contain %q", tt.wantInBody)
			}
		})
	}
}

func TestDistFSReadDir(t *testing.T) {
	distFS, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v", err)
	}

	entries, err := fs.ReadDir(distFS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(entries) == 0 {
		t.Error("dist directory should not be empty")
	}

	// Should contain index.html
	hasIndex := false
	for _, e := range entries {
		if e.Name() == "index.html" {
			hasIndex = true
			break
		}
	}
	if !hasIndex {
		t.Error("dist directory should contain index.html")
	}
}
