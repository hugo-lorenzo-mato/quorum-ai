//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func newIntegrationTestServerWithEnabledAgent(t *testing.T, agent string) *testServer {
	ts := newIntegrationTestServer(t)
	loader := config.NewLoader()
	v := loader.Viper()
	v.Set("agents.default", agent)
	v.Set("agents."+agent+".enabled", true)
	ts.server.configLoader = loader
	return ts
}

// TestIntegration_CreateWorkflow_WithExecutionMode tests creating workflows
// with different execution mode configurations.
func TestIntegration_CreateWorkflow_WithExecutionMode(t *testing.T) {
	t.Parallel()
	ts := newIntegrationTestServerWithEnabledAgent(t, "claude")

	t.Run("create workflow with single-agent mode", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"prompt": "Test single-agent workflow",
			"title":  "Single Agent Test",
			"blueprint": map[string]interface{}{
				"execution_mode":     "single_agent",
				"single_agent_name":  "claude",
				"single_agent_model": "test-model",
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			ts.URL+"/api/v1/workflows",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify blueprint is returned
		config, ok := result["blueprint"].(map[string]interface{})
		if !ok {
			t.Fatal("expected config in response")
		}

		if config["execution_mode"] != "single_agent" {
			t.Errorf("expected execution_mode 'single_agent', got '%v'", config["execution_mode"])
		}
		if config["single_agent_name"] != "claude" {
			t.Errorf("expected single_agent_name 'claude', got '%v'", config["single_agent_name"])
		}
	})

	t.Run("create workflow with multi-agent mode (default)", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"prompt": "Test multi-agent workflow",
			"blueprint": map[string]interface{}{
				"execution_mode": "multi_agent",
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			ts.URL+"/api/v1/workflows",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		config, ok := result["blueprint"].(map[string]interface{})
		if !ok {
			t.Fatal("expected config in response")
		}

		if config["execution_mode"] != "multi_agent" {
			t.Errorf("expected execution_mode 'multi_agent', got '%v'", config["execution_mode"])
		}
	})

	t.Run("create workflow without config defaults to multi-agent", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"prompt": "Test default workflow",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			ts.URL+"/api/v1/workflows",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Config can be nil or have default values
		// The important thing is that the workflow was created
		id := result["id"].(string)
		if id == "" {
			t.Error("expected non-empty workflow ID")
		}
	})
}

// TestIntegration_GetWorkflow_IncludesExecutionMode tests that getting a workflow
// returns the execution mode configuration.
func TestIntegration_GetWorkflow_IncludesExecutionMode(t *testing.T) {
	t.Parallel()
	ts := newIntegrationTestServerWithEnabledAgent(t, "claude")

	// Create a workflow with single-agent mode
	reqBody := map[string]interface{}{
		"prompt": "Test workflow",
		"blueprint": map[string]interface{}{
			"execution_mode":    "single_agent",
			"single_agent_name": "claude",
		},
	}
	body, _ := json.Marshal(reqBody)

	createResp, err := http.Post(
		ts.URL+"/api/v1/workflows",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createResp.StatusCode)
	}

	var createResult map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&createResult); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	id, _ := createResult["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty workflow ID")
	}

	// Get the workflow
	getResp, err := http.Get(ts.URL + "/api/v1/workflows/" + id + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getResp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify config is returned
	config, ok := result["blueprint"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config in response")
	}

	if config["execution_mode"] != "single_agent" {
		t.Errorf("expected execution_mode 'single_agent', got '%v'", config["execution_mode"])
	}
	if config["single_agent_name"] != "claude" {
		t.Errorf("expected single_agent_name 'claude', got '%v'", config["single_agent_name"])
	}
}

// TestIntegration_ListWorkflows_IncludesExecutionMode tests that listing workflows
// returns execution mode configuration for each workflow.
func TestIntegration_ListWorkflows_IncludesExecutionMode(t *testing.T) {
	t.Parallel()
	ts := newIntegrationTestServerWithEnabledAgent(t, "claude")

	// Create workflows with different execution modes
	configs := []map[string]interface{}{
		{
			"execution_mode":    "single_agent",
			"single_agent_name": "claude",
		},
		{
			"execution_mode": "multi_agent",
		},
	}

	for i, cfg := range configs {
		reqBody := map[string]interface{}{
			"prompt":    "Test workflow " + string(rune('A'+i)),
			"blueprint": cfg,
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			ts.URL+"/api/v1/workflows",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
	}

	// List all workflows
	listResp, err := http.Get(ts.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listResp.StatusCode)
	}

	var workflows []map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&workflows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(workflows) < 2 {
		t.Fatalf("expected at least 2 workflows, got %d", len(workflows))
	}

	// Verify each workflow has config
	for _, wf := range workflows {
		config, ok := wf["blueprint"].(map[string]interface{})
		if !ok {
			continue // Some workflows may not have config
		}

		mode := config["execution_mode"]
		if mode != "single_agent" && mode != "multi_agent" && mode != nil && mode != "" {
			t.Errorf("unexpected execution_mode: %v", mode)
		}
	}
}

// TestIntegration_WorkflowConfigPersistence tests that workflow config is persisted
// correctly across create and get operations.
func TestIntegration_WorkflowConfigPersistence(t *testing.T) {
	t.Parallel()
	ts := newIntegrationTestServerWithEnabledAgent(t, "claude")

	t.Run("full config is persisted", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"prompt": "Test config persistence",
			"blueprint": map[string]interface{}{
				"execution_mode":      "single_agent",
				"single_agent_name":   "claude",
				"single_agent_model":  "test-model-v1",
				"consensus_threshold": 0.85,
				"dry_run":             true,
			},
		}
		body, _ := json.Marshal(reqBody)

		createResp, err := http.Post(
			ts.URL+"/api/v1/workflows",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer createResp.Body.Close()

		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, createResp.StatusCode)
		}

		var createResult map[string]interface{}
		if err := json.NewDecoder(createResp.Body).Decode(&createResult); err != nil {
			t.Fatalf("failed to decode create response: %v", err)
		}
		id, _ := createResult["id"].(string)
		if id == "" {
			t.Fatal("expected non-empty workflow ID")
		}

		// Get the workflow back
		getResp, err := http.Get(ts.URL + "/api/v1/workflows/" + id + "/")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer getResp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(getResp.Body).Decode(&result)

		config := result["blueprint"].(map[string]interface{})

		// Verify all fields
		if config["execution_mode"] != "single_agent" {
			t.Errorf("execution_mode mismatch: got %v", config["execution_mode"])
		}
		if config["single_agent_name"] != "claude" {
			t.Errorf("single_agent_name mismatch: got %v", config["single_agent_name"])
		}
		if config["single_agent_model"] != "test-model-v1" {
			t.Errorf("single_agent_model mismatch: got %v", config["single_agent_model"])
		}
		if config["dry_run"] != true {
			t.Errorf("dry_run mismatch: got %v", config["dry_run"])
		}
	})
}

// TestIntegration_WorkflowState_ConfigMapping tests that WorkflowState correctly
// maps config between API and core types.
func TestIntegration_WorkflowState_ConfigMapping(t *testing.T) {
	t.Parallel()
	// Test that core.WorkflowState config fields are properly set
	sm := newThreadSafeMockStateManager()

	// Directly set a workflow state with config
	wfState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-config-map-test",
			Prompt:     "Test prompt",
			Blueprint: &core.Blueprint{
				ExecutionMode: "single_agent",
				SingleAgent:   core.BlueprintSingleAgent{Agent: "claude"},
			},
			CreatedAt: time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			UpdatedAt:    time.Now(),
		},
	}

	if err := sm.Save(context.Background(), wfState); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load it back
	loaded, err := sm.LoadByID(context.Background(), "wf-config-map-test")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Blueprint == nil {
		t.Fatal("expected blueprint to be loaded")
	}

	if loaded.Blueprint.ExecutionMode != "single_agent" {
		t.Errorf("ExecutionMode mismatch: got %s", loaded.Blueprint.ExecutionMode)
	}
	if loaded.Blueprint.SingleAgent.Agent != "claude" {
		t.Errorf("SingleAgent.Agent mismatch: got %s", loaded.Blueprint.SingleAgent.Agent)
	}
}
