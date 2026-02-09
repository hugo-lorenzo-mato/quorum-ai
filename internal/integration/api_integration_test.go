package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAPI_WorkflowLifecycle tests complete workflow lifecycle through API.
func TestAPI_WorkflowLifecycle(t *testing.T) {
	t.Parallel()

	// Create test API server
	apiHandler := NewMockAPIHandler()
	server := httptest.NewServer(apiHandler)
	defer server.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	// Test 1: Create new workflow
	createReq := WorkflowCreateRequest{
		Prompt:    "Analyze the codebase and suggest improvements",
		GitBranch: "main",
		Agents:    []string{"claude", "gemini"},
	}

	createBody, _ := json.Marshal(createReq)
	resp, err := client.Post(server.URL+"/api/v1/workflows", "application/json", 
		strings.NewReader(string(createBody)))
	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var createResp WorkflowCreateResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	if err != nil {
		t.Fatalf("Failed to decode create response: %v", err)
	}

	workflowID := createResp.WorkflowID
	if workflowID == "" {
		t.Fatal("Workflow ID should not be empty")
	}

	// Test 2: Get workflow status
	statusResp, err := client.Get(server.URL + "/api/v1/workflows/" + workflowID)
	if err != nil {
		t.Fatalf("Failed to get workflow status: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 for workflow status, got %d", statusResp.StatusCode)
	}

	var status WorkflowStatusResponse
	err = json.NewDecoder(statusResp.Body).Decode(&status)
	if err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}

	if status.ID != workflowID {
		t.Errorf("Expected workflow ID %s, got %s", workflowID, status.ID)
	}

	if status.Status != "running" && status.Status != "pending" {
		t.Errorf("Expected workflow to be running or pending, got %s", status.Status)
	}

	// Test 3: List all workflows
	listResp, err := client.Get(server.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("Failed to list workflows: %v", err)
	}
	defer listResp.Body.Close()

	var list WorkflowListResponse
	err = json.NewDecoder(listResp.Body).Decode(&list)
	if err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}

	if len(list.Workflows) == 0 {
		t.Error("Expected at least one workflow in list")
	}

	foundWorkflow := false
	for _, w := range list.Workflows {
		if w.ID == workflowID {
			foundWorkflow = true
			break
		}
	}

	if !foundWorkflow {
		t.Errorf("Created workflow %s not found in list", workflowID)
	}

	// Test 4: Cancel workflow
	cancelReq, err := http.NewRequest("DELETE", server.URL+"/api/v1/workflows/"+workflowID, nil)
	if err != nil {
		t.Fatalf("Failed to create cancel request: %v", err)
	}

	cancelResp, err := client.Do(cancelReq)
	if err != nil {
		t.Fatalf("Failed to cancel workflow: %v", err)
	}
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 for cancel, got %d", cancelResp.StatusCode)
	}

	// Test 5: Verify workflow is cancelled
	finalStatusResp, err := client.Get(server.URL + "/api/v1/workflows/" + workflowID)
	if err != nil {
		t.Fatalf("Failed to get final status: %v", err)
	}
	defer finalStatusResp.Body.Close()

	var finalStatus WorkflowStatusResponse
	err = json.NewDecoder(finalStatusResp.Body).Decode(&finalStatus)
	if err != nil {
		t.Fatalf("Failed to decode final status: %v", err)
	}

	if finalStatus.Status != "cancelled" && finalStatus.Status != "failed" {
		t.Errorf("Expected workflow to be cancelled or failed, got %s", finalStatus.Status)
	}

	t.Logf("API workflow lifecycle test completed for workflow %s", workflowID)
}

// TestAPI_ErrorHandling tests API error responses.
func TestAPI_ErrorHandling(t *testing.T) {
	t.Parallel()

	apiHandler := NewMockAPIHandler()
	server := httptest.NewServer(apiHandler)
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name           string
		method         string
		url            string
		body           string
		contentType    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid_json",
			method:         "POST",
			url:            "/api/v1/workflows",
			body:           "{ invalid json }",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON",
		},
		{
			name:           "missing_prompt",
			method:         "POST",
			url:            "/api/v1/workflows",
			body:           `{"git_branch": "main"}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "prompt is required",
		},
		{
			name:           "workflow_not_found",
			method:         "GET",
			url:            "/api/v1/workflows/nonexistent-id",
			expectedStatus: http.StatusNotFound,
			expectedError:  "workflow not found",
		},
		{
			name:           "unsupported_method",
			method:         "PATCH",
			url:            "/api/v1/workflows",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method not allowed",
		},
		{
			name:           "wrong_content_type",
			method:         "POST",
			url:            "/api/v1/workflows",
			body:           `{"prompt": "test"}`,
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "unsupported media type",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}

			req, err := http.NewRequest(tc.method, server.URL+tc.url, body)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			var errorResp ErrorResponse
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			if err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if !strings.Contains(strings.ToLower(errorResp.Message), strings.ToLower(tc.expectedError)) {
				t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, errorResp.Message)
			}

			t.Logf("Error test %s: status=%d, message=%s", tc.name, resp.StatusCode, errorResp.Message)
		})
	}
}

// TestAPI_Authentication tests API authentication and authorization.
func TestAPI_Authentication(t *testing.T) {
	t.Parallel()

	apiHandler := NewMockAPIHandler()
	apiHandler.EnableAuth(true)
	server := httptest.NewServer(apiHandler)
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// Test 1: Request without authentication
	resp, err := client.Get(server.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("Failed to make unauthenticated request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}

	// Test 2: Request with invalid token
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/workflows", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request with invalid token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d", resp.StatusCode)
	}

	// Test 3: Request with valid token
	req, _ = http.NewRequest("GET", server.URL+"/api/v1/workflows", nil)
	req.Header.Set("Authorization", "Bearer valid-test-token")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make authenticated request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for valid token, got %d", resp.StatusCode)
	}

	// Test 4: API key authentication
	req, _ = http.NewRequest("GET", server.URL+"/api/v1/workflows", nil)
	req.Header.Set("X-API-Key", "valid-api-key")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make API key request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for valid API key, got %d", resp.StatusCode)
	}

	t.Logf("Authentication tests completed")
}

// TestAPI_RateLimiting tests API rate limiting functionality.
func TestAPI_RateLimiting(t *testing.T) {
	t.Parallel()

	apiHandler := NewMockAPIHandler()
	apiHandler.EnableRateLimit(true, 5, time.Minute) // 5 requests per minute
	server := httptest.NewServer(apiHandler)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Make requests up to the limit
	for i := 0; i < 5; i++ {
		resp, err := client.Get(server.URL + "/api/v1/workflows")
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// Next request should be rate limited
	resp, err := client.Get(server.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("Rate limit test request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 for rate limited request, got %d", resp.StatusCode)
	}

	// Check rate limit headers
	if resp.Header.Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}

	if resp.Header.Get("X-RateLimit-Remaining") == "" {
		t.Error("Expected X-RateLimit-Remaining header")
	}

	t.Logf("Rate limiting test completed - limit enforced after 5 requests")
}

// TestAPI_ConcurrentRequests tests API behavior under concurrent load.
func TestAPI_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	apiHandler := NewMockAPIHandler()
	server := httptest.NewServer(apiHandler)
	defer server.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	// Create multiple workflows concurrently
	const numConcurrent = 10
	results := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(id int) {
			createReq := WorkflowCreateRequest{
				Prompt:    fmt.Sprintf("Concurrent test workflow %d", id),
				GitBranch: "main",
			}

			body, _ := json.Marshal(createReq)
			resp, err := client.Post(server.URL+"/api/v1/workflows", "application/json", 
				strings.NewReader(string(body)))
			
			if err != nil {
				results <- fmt.Errorf("request %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				results <- fmt.Errorf("request %d: expected 201, got %d", id, resp.StatusCode)
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numConcurrent; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		} else {
			successCount++
		}
	}

	if successCount != numConcurrent {
		t.Errorf("Expected %d successful requests, got %d", numConcurrent, successCount)
	}

	// Verify all workflows were created
	listResp, err := client.Get(server.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("Failed to list workflows: %v", err)
	}
	defer listResp.Body.Close()

	var list WorkflowListResponse
	err = json.NewDecoder(listResp.Body).Decode(&list)
	if err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}

	if len(list.Workflows) < numConcurrent {
		t.Errorf("Expected at least %d workflows, got %d", numConcurrent, len(list.Workflows))
	}

	t.Logf("Concurrent requests test: %d/%d successful", successCount, numConcurrent)
}

// Mock API implementation

type MockAPIHandler struct {
	workflows   map[string]*WorkflowInfo
	authEnabled bool
	rateLimit   *RateLimiter
}

type WorkflowInfo struct {
	ID       string    `json:"id"`
	Status   string    `json:"status"`
	Prompt   string    `json:"prompt"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

type RateLimiter struct {
	enabled    bool
	limit      int
	window     time.Duration
	requests   map[string][]time.Time
}

type WorkflowCreateRequest struct {
	Prompt    string   `json:"prompt"`
	GitBranch string   `json:"git_branch"`
	Agents    []string `json:"agents"`
}

type WorkflowCreateResponse struct {
	WorkflowID string `json:"workflow_id"`
}

type WorkflowStatusResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Prompt  string `json:"prompt"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type WorkflowListResponse struct {
	Workflows []WorkflowInfo `json:"workflows"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewMockAPIHandler() *MockAPIHandler {
	return &MockAPIHandler{
		workflows: make(map[string]*WorkflowInfo),
	}
}

func (h *MockAPIHandler) EnableAuth(enabled bool) {
	h.authEnabled = enabled
}

func (h *MockAPIHandler) EnableRateLimit(enabled bool, limit int, window time.Duration) {
	h.rateLimit = &RateLimiter{
		enabled:  enabled,
		limit:    limit,
		window:   window,
		requests: make(map[string][]time.Time),
	}
}

func (h *MockAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authentication check
	if h.authEnabled && !h.isAuthenticated(r) {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Rate limiting check
	if h.rateLimit != nil && h.rateLimit.enabled && !h.checkRateLimit(w, r) {
		return
	}

	// Route requests
	switch {
	case r.Method == "POST" && r.URL.Path == "/api/v1/workflows":
		h.createWorkflow(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/v1/workflows":
		h.listWorkflows(w, r)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/workflows/"):
		h.getWorkflow(w, r)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/workflows/"):
		h.cancelWorkflow(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *MockAPIHandler) isAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	apiKey := r.Header.Get("X-API-Key")
	
	return auth == "Bearer valid-test-token" || apiKey == "valid-api-key"
}

func (h *MockAPIHandler) checkRateLimit(w http.ResponseWriter, r *http.Request) bool {
	clientIP := r.RemoteAddr
	now := time.Now()
	
	// Clean old requests
	if requests, exists := h.rateLimit.requests[clientIP]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if now.Sub(reqTime) < h.rateLimit.window {
				validRequests = append(validRequests, reqTime)
			}
		}
		h.rateLimit.requests[clientIP] = validRequests
	}
	
	// Check limit
	currentRequests := len(h.rateLimit.requests[clientIP])
	if currentRequests >= h.rateLimit.limit {
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", h.rateLimit.limit))
		w.Header().Set("X-RateLimit-Remaining", "0")
		h.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return false
	}
	
	// Record request
	h.rateLimit.requests[clientIP] = append(h.rateLimit.requests[clientIP], now)
	
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", h.rateLimit.limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", h.rateLimit.limit-currentRequests-1))
	
	return true
}

func (h *MockAPIHandler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		h.writeError(w, http.StatusUnsupportedMediaType, "unsupported media type")
		return
	}

	var req WorkflowCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Prompt == "" {
		h.writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	workflowID := fmt.Sprintf("wf_%d", time.Now().UnixNano())
	workflow := &WorkflowInfo{
		ID:      workflowID,
		Status:  "running",
		Prompt:  req.Prompt,
		Created: time.Now(),
		Updated: time.Now(),
	}

	h.workflows[workflowID] = workflow

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(WorkflowCreateResponse{WorkflowID: workflowID})
}

func (h *MockAPIHandler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := make([]WorkflowInfo, 0, len(h.workflows))
	for _, workflow := range h.workflows {
		workflows = append(workflows, *workflow)
	}

	json.NewEncoder(w).Encode(WorkflowListResponse{Workflows: workflows})
}

func (h *MockAPIHandler) getWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
	
	workflow, exists := h.workflows[workflowID]
	if !exists {
		h.writeError(w, http.StatusNotFound, "workflow not found")
		return
	}

	response := WorkflowStatusResponse{
		ID:      workflow.ID,
		Status:  workflow.Status,
		Prompt:  workflow.Prompt,
		Created: workflow.Created.Format(time.RFC3339),
		Updated: workflow.Updated.Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (h *MockAPIHandler) cancelWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
	
	workflow, exists := h.workflows[workflowID]
	if !exists {
		h.writeError(w, http.StatusNotFound, "workflow not found")
		return
	}

	workflow.Status = "cancelled"
	workflow.Updated = time.Now()

	json.NewEncoder(w).Encode(map[string]string{"message": "workflow cancelled"})
}

func (h *MockAPIHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Message: message, Code: status})
}