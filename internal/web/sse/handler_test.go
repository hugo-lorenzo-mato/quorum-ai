package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestNewHandler(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.ClientCount())
	}
}

func TestHandler_ServeHTTP_ConnectsClient(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	h.SetHeartbeatFrequency(100 * time.Millisecond)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Verify headers
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}

	// Read first event (connection event)
	reader := bufio.NewReader(resp.Body)
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if !strings.HasPrefix(eventLine, "event: connected") {
		t.Errorf("expected connected event, got %s", eventLine)
	}
}

func TestHandler_StreamsEvents(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	h.SetHeartbeatFrequency(10 * time.Second) // Long heartbeat to avoid interference

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Connect client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Skip connection event
	for i := 0; i < 2; i++ { // event: + data: lines
		_, _ = reader.ReadString('\n')
	}
	_, _ = reader.ReadString('\n') // empty line

	// Give the handler time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Publish an event
	bus.Publish(events.NewWorkflowStartedEvent("wf-123", "", "test prompt"))

	// Read the workflow_started event
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if !strings.HasPrefix(eventLine, "event: workflow_started") {
		t.Errorf("expected workflow_started event, got %s", eventLine)
	}

	dataLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}
	if !strings.HasPrefix(dataLine, "data: ") {
		t.Errorf("expected data line, got %s", dataLine)
	}

	// Parse the JSON data
	jsonStr := strings.TrimPrefix(dataLine, "data: ")
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &eventData); err != nil {
		t.Fatalf("failed to parse event data: %v", err)
	}

	if eventData["workflow_id"] != "wf-123" {
		t.Errorf("expected workflow_id wf-123, got %v", eventData["workflow_id"])
	}
	if eventData["prompt"] != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %v", eventData["prompt"])
	}
}

func TestHandler_FiltersWorkflow(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	h.SetHeartbeatFrequency(10 * time.Second)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with workflow filter
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"?workflow=wf-123", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Connect client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Skip connection event
	for i := 0; i < 3; i++ {
		_, _ = reader.ReadString('\n')
	}

	// Give the handler time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Publish events for different workflows
	bus.Publish(events.NewWorkflowStartedEvent("wf-456", "", "other workflow"))
	bus.Publish(events.NewWorkflowStartedEvent("wf-123", "", "filtered workflow"))

	// We should only receive the wf-123 event
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}

	dataLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}

	jsonStr := strings.TrimPrefix(dataLine, "data: ")
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &eventData); err != nil {
		t.Fatalf("failed to parse event data: %v", err)
	}

	if eventData["workflow_id"] != "wf-123" {
		t.Errorf("expected filtered workflow wf-123, got %v (event: %s)", eventData["workflow_id"], eventLine)
	}
}

func TestHandler_FiltersProject(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	h.SetHeartbeatFrequency(10 * time.Second)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with project filter
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"?project=proj-1", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Connect client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Skip connection event
	for i := 0; i < 3; i++ {
		_, _ = reader.ReadString('\n')
	}

	// Give the handler time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Publish events for different projects
	bus.Publish(events.NewWorkflowStartedEvent("wf-1", "proj-2", "other project"))
	bus.Publish(events.NewWorkflowStartedEvent("wf-2", "proj-1", "filtered project"))

	// We should only receive the proj-1 event
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}

	dataLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}

	jsonStr := strings.TrimPrefix(dataLine, "data: ")
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &eventData); err != nil {
		t.Fatalf("failed to parse event data: %v", err)
	}

	if eventData["project_id"] != "proj-1" {
		t.Errorf("expected filtered project proj-1, got %v (event: %s)", eventData["project_id"], eventLine)
	}
	if eventData["workflow_id"] != "wf-2" {
		t.Errorf("expected workflow wf-2, got %v", eventData["workflow_id"])
	}
}

func TestHandler_ClientCount(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Initial count should be 0
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.ClientCount())
	}

	// Connect a client
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Give handler time to register client
	time.Sleep(100 * time.Millisecond)

	if h.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", h.ClientCount())
	}

	// Disconnect client
	cancel()
	resp.Body.Close()

	// Give handler time to clean up
	time.Sleep(100 * time.Millisecond)

	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", h.ClientCount())
	}
}

func TestHandler_Shutdown(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Connect some clients
	clients := make([]*http.Response, 3)
	// Ensure all response bodies are closed on function exit
	defer func() {
		for _, resp := range clients {
			if resp != nil {
				resp.Body.Close()
			}
		}
	}()

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
		resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed in deferred cleanup above
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		clients[i] = resp
	}

	// Wait for connections
	time.Sleep(100 * time.Millisecond)

	if h.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", h.ClientCount())
	}

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}

	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", h.ClientCount())
	}
}

func TestHandler_Heartbeat(t *testing.T) {
	bus := events.New(100)
	defer bus.Close()

	h := NewHandler(bus)
	h.SetHeartbeatFrequency(100 * time.Millisecond)

	// Create test server
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Skip initial event
	for i := 0; i < 3; i++ {
		_, _ = reader.ReadString('\n')
	}

	// Wait for heartbeat
	time.Sleep(150 * time.Millisecond)

	// Read heartbeat comment
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read heartbeat: %v", err)
	}

	if !strings.HasPrefix(line, ": heartbeat") {
		t.Errorf("expected heartbeat comment, got %s", line)
	}
}
