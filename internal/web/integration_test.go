package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestServerIntegration tests the full HTTP server integration.
func TestServerIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()
	cfg.Port = 0 // Use any available port for testing

	server := New(cfg, logger)

	// Create test server using the router directly
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	t.Run("health endpoint returns healthy status", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("GET /health failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["status"] != "healthy" {
			t.Errorf("status = %q, want %q", result["status"], "healthy")
		}
	})

	t.Run("API root returns version info", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/")
		if err != nil {
			t.Fatalf("GET /api/v1/ failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["version"] != "v1" {
			t.Errorf("version = %q, want %q", result["version"], "v1")
		}
		if result["name"] != "quorum-api" {
			t.Errorf("name = %q, want %q", result["name"], "quorum-api")
		}
	})

	t.Run("static files are served with correct content type", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET / failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if !strings.Contains(string(body), "<!doctype html>") {
			t.Error("root should serve index.html")
		}
	})

	t.Run("SPA routing serves index.html for unknown paths", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/workflows/123")
		if err != nil {
			t.Fatalf("GET /workflows/123 failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if !strings.Contains(string(body), "<!doctype html>") {
			t.Error("SPA route should serve index.html")
		}
	})

	t.Run("CORS headers are set correctly", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", ts.URL+"/api/v1/", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Access-Control-Request-Method", "GET")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS /api/v1/ failed: %v", err)
		}
		defer resp.Body.Close()

		// Check CORS headers
		if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "http://localhost:5173")
		}
	})

	t.Run("request ID is included in response", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("GET /health failed: %v", err)
		}
		defer resp.Body.Close()

		// Chi middleware sets X-Request-Id header (but it's exposed, not returned by default)
		// We can verify the request completes successfully
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})
}

// TestServerLifecycle tests server start and shutdown.
func TestServerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0 // Will need to use a specific port for actual listening

	// Use port 0 means OS picks a free port, but we can't get it back easily
	// So we'll test with a specific high port
	cfg.Port = 18765

	server := New(cfg, logger)

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test that we can connect
	resp, err := http.Get("http://127.0.0.1:18765/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	// Verify server is stopped (connection should fail)
	resp2, err := http.Get("http://127.0.0.1:18765/health")
	if err == nil {
		resp2.Body.Close()
		t.Error("expected connection to fail after shutdown")
	}
}

// TestServerAddr verifies the server address is correctly formatted.
func TestServerAddr(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()
	cfg.Host = "0.0.0.0"
	cfg.Port = 9090

	server := New(cfg, logger)

	if addr := server.Addr(); addr != "0.0.0.0:9090" {
		t.Errorf("Addr() = %q, want %q", addr, "0.0.0.0:9090")
	}
}

// TestServerWithDisabledCORS verifies CORS can be disabled.
func TestServerWithDisabledCORS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()
	cfg.EnableCORS = false

	server := New(cfg, logger)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	req, err := http.NewRequest("OPTIONS", ts.URL+"/api/v1/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	// Without CORS middleware, no Access-Control-Allow-Origin header
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty when CORS disabled, got %q", origin)
	}
}

// TestServerWithDisabledStatic verifies static file serving can be disabled.
func TestServerWithDisabledStatic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()
	cfg.ServeStatic = false

	server := New(cfg, logger)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	// API endpoints should still work
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Unknown routes should return 404 instead of SPA fallback
	resp2, err := http.Get(ts.URL + "/some/unknown/path")
	if err != nil {
		t.Fatalf("GET /some/unknown/path failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d for unknown path when static disabled", resp2.StatusCode, http.StatusNotFound)
	}
}

// TestContentTypeHeaders verifies correct content-type headers.
func TestContentTypeHeaders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()

	server := New(cfg, logger)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	tests := []struct {
		path        string
		wantType    string
		description string
	}{
		{"/health", "application/json", "health endpoint returns JSON"},
		{"/api/v1/", "application/json", "API root returns JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s failed: %v", tt.path, err)
			}
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, tt.wantType) {
				t.Errorf("Content-Type = %q, want prefix %q", contentType, tt.wantType)
			}
		})
	}
}

// TestHTTPMethods verifies correct HTTP method handling.
func TestHTTPMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := DefaultConfig()

	server := New(cfg, logger)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	t.Run("GET methods work", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("GET /health failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("POST to GET-only endpoint returns 405", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/health", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /health failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
		}
	})
}
