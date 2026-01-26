//go:build integration

package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestIntegration_CreateAndGetWorkflow(t *testing.T) {
	ts := newIntegrationTestServer(t)

	// Create a workflow
	id, err := ts.createWorkflow("Test integration workflow")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	if id == "" {
		t.Fatal("workflow ID is empty")
	}

	// Get the workflow
	wf, err := ts.getWorkflow(id)
	if err != nil {
		t.Fatalf("failed to get workflow: %v", err)
	}

	if wf["id"] != id {
		t.Errorf("workflow ID mismatch: got %v, want %v", wf["id"], id)
	}

	if wf["status"] != "pending" {
		t.Errorf("unexpected status: %v", wf["status"])
	}
}

func TestIntegration_RunWorkflow_MissingConfig(t *testing.T) {
	// This test verifies behavior when config loader is not set
	ts := newIntegrationTestServer(t)

	// Create a workflow
	id, err := ts.createWorkflow("Test workflow")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Try to run it (should fail due to missing config)
	resp, err := ts.runWorkflow(id)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should get 503 Service Unavailable due to missing config
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestIntegration_SSE_Connection(t *testing.T) {
	ts := newIntegrationTestServer(t)

	// Connect SSE client
	sseClient := newSSEClient(ts.URL)
	sseClient.connect()
	defer sseClient.close()

	// Wait for connection
	if err := sseClient.waitConnected(2 * time.Second); err != nil {
		t.Fatalf("SSE connection failed: %v", err)
	}

	// Wait for connected event
	if !sseClient.waitForEvent("connected", 1*time.Second) {
		t.Error("did not receive 'connected' event")
	}
}

func TestIntegration_SSE_ReceivesWorkflowEvents(t *testing.T) {
	ts := newIntegrationTestServer(t)

	// Connect SSE client first
	sseClient := newSSEClient(ts.URL)
	sseClient.connect()
	defer sseClient.close()

	if err := sseClient.waitConnected(2 * time.Second); err != nil {
		t.Fatalf("SSE connection failed: %v", err)
	}

	// Wait for connection to be ready
	time.Sleep(100 * time.Millisecond)

	// Publish an event directly to the event bus
	ts.eventBus.Publish(events.NewWorkflowStartedEvent("wf-test", "test prompt"))

	// Wait for the event to arrive via SSE
	if !sseClient.waitForEvent("workflow_started", 2*time.Second) {
		t.Error("did not receive 'workflow_started' event via SSE")
	}

	// Verify event data
	receivedEvents := sseClient.getEvents()
	var found bool
	for _, e := range receivedEvents {
		if e.Type == "workflow_started" {
			if e.Data["workflow_id"] != "wf-test" {
				t.Errorf("unexpected workflow_id: %v", e.Data["workflow_id"])
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("workflow_started event not found in collected events")
	}
}

func TestIntegration_ConcurrentWorkflowCreation(t *testing.T) {
	ts := newIntegrationTestServer(t)

	// Create multiple workflows concurrently
	// Note: Workflow IDs are timestamp-based (seconds granularity), so
	// concurrent creations within the same second may get the same ID.
	// This test verifies concurrent requests don't panic/error, and
	// that created workflows can be retrieved.
	const numWorkflows = 5
	ids := make(chan string, numWorkflows)
	errs := make(chan error, numWorkflows)

	for i := 0; i < numWorkflows; i++ {
		go func(n int) {
			id, err := ts.createWorkflow(fmt.Sprintf("Workflow %d", n))
			if err != nil {
				errs <- err
			} else {
				ids <- id
			}
		}(i)
	}

	// Collect results
	var createdIDs []string
	timeout := time.After(10 * time.Second)

	for i := 0; i < numWorkflows; i++ {
		select {
		case id := <-ids:
			createdIDs = append(createdIDs, id)
		case err := <-errs:
			t.Errorf("concurrent creation failed: %v", err)
		case <-timeout:
			t.Fatal("timeout waiting for concurrent workflow creation")
		}
	}

	// Verify all workflows have valid IDs (non-empty)
	// Note: Due to timestamp-based ID generation, some IDs may be duplicates
	// when created within the same second. This is a known limitation.
	for _, id := range createdIDs {
		if id == "" {
			t.Error("received empty workflow ID")
		}
		// Verify each workflow can be retrieved
		wf, err := ts.getWorkflow(id)
		if err != nil {
			t.Errorf("failed to get workflow %s: %v", id, err)
		}
		if wf["id"] != id {
			t.Errorf("workflow ID mismatch: got %v, want %v", wf["id"], id)
		}
	}
}

func TestIntegration_RunWorkflow_NotFound(t *testing.T) {
	ts := newIntegrationTestServer(t)

	resp, err := ts.runWorkflow("nonexistent-workflow")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_RunWorkflow_DoubleRun(t *testing.T) {
	ts := newIntegrationTestServer(t)

	// Create workflow
	id, err := ts.createWorkflow("Test workflow")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Mark as running manually (simulating first run in progress)
	if !markRunning(id) {
		t.Fatal("failed to mark workflow as running")
	}
	defer markFinished(id)

	// Try to run again
	resp, err := ts.runWorkflow(id)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should get 409 Conflict
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}
