package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// mockStateManager implements core.StateManager for testing.
type mockStateManager struct {
	mu           sync.Mutex
	workflows    map[core.WorkflowID]*core.WorkflowState
	activeID     core.WorkflowID
	saveErr      error
	loadErr      error
	listErr      error
	lockAcquired bool
}

func newMockStateManager() *mockStateManager {
	return &mockStateManager{
		workflows: make(map[core.WorkflowID]*core.WorkflowState),
	}
}

func (m *mockStateManager) Save(_ context.Context, state *core.WorkflowState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.workflows[state.WorkflowID] = state
	m.activeID = state.WorkflowID
	return nil
}

func (m *mockStateManager) Load(_ context.Context) (*core.WorkflowState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.activeID == "" {
		return nil, nil
	}
	return m.workflows[m.activeID], nil
}

func (m *mockStateManager) LoadByID(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.workflows[id], nil
}

func (m *mockStateManager) ListWorkflows(_ context.Context) ([]core.WorkflowSummary, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	summaries := make([]core.WorkflowSummary, 0, len(m.workflows))
	for id, wf := range m.workflows {
		summaries = append(summaries, core.WorkflowSummary{
			WorkflowID:   id,
			Status:       wf.Status,
			CurrentPhase: wf.CurrentPhase,
			Prompt:       wf.Prompt,
			CreatedAt:    wf.CreatedAt,
			UpdatedAt:    wf.UpdatedAt,
			IsActive:     id == m.activeID,
		})
	}
	return summaries, nil
}

func (m *mockStateManager) GetActiveWorkflowID(_ context.Context) (core.WorkflowID, error) {
	return m.activeID, nil
}

func (m *mockStateManager) SetActiveWorkflowID(_ context.Context, id core.WorkflowID) error {
	m.activeID = id
	return nil
}

func (m *mockStateManager) AcquireLock(_ context.Context) error {
	m.lockAcquired = true
	return nil
}

func (m *mockStateManager) ReleaseLock(_ context.Context) error {
	m.lockAcquired = false
	return nil
}

func (m *mockStateManager) Exists() bool {
	return len(m.workflows) > 0
}

func (m *mockStateManager) Backup(_ context.Context) error {
	return nil
}

func (m *mockStateManager) Restore(_ context.Context) (*core.WorkflowState, error) {
	return nil, nil
}

func (m *mockStateManager) DeactivateWorkflow(_ context.Context) error {
	m.activeID = ""
	return nil
}

func (m *mockStateManager) ArchiveWorkflows(_ context.Context) (int, error) {
	count := 0
	for id, wf := range m.workflows {
		if wf.Status == core.WorkflowStatusCompleted || wf.Status == core.WorkflowStatusFailed {
			if id != m.activeID {
				delete(m.workflows, id)
				count++
			}
		}
	}
	return count, nil
}

func (m *mockStateManager) PurgeAllWorkflows(_ context.Context) (int, error) {
	count := len(m.workflows)
	m.workflows = make(map[core.WorkflowID]*core.WorkflowState)
	m.activeID = ""
	return count, nil
}

func (m *mockStateManager) DeleteWorkflow(_ context.Context, id core.WorkflowID) error {
	if _, exists := m.workflows[id]; !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}
	delete(m.workflows, id)
	if m.activeID == id {
		m.activeID = ""
	}
	return nil
}

func (m *mockStateManager) UpdateHeartbeat(_ context.Context, id core.WorkflowID) error {
	if wf, exists := m.workflows[id]; exists {
		now := time.Now().UTC()
		wf.HeartbeatAt = &now
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) FindZombieWorkflows(_ context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	var zombies []*core.WorkflowState
	cutoff := time.Now().UTC().Add(-staleThreshold)
	for _, wf := range m.workflows {
		if wf.Status == core.WorkflowStatusRunning {
			if wf.HeartbeatAt == nil || wf.HeartbeatAt.Before(cutoff) {
				zombies = append(zombies, wf)
			}
		}
	}
	return zombies, nil
}

func (m *mockStateManager) AcquireWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.lockAcquired = true
	return nil
}

func (m *mockStateManager) ReleaseWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.lockAcquired = false
	return nil
}

func (m *mockStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	var running []core.WorkflowID
	for id, wf := range m.workflows {
		if wf.Status == core.WorkflowStatusRunning {
			running = append(running, id)
		}
	}
	return running, nil
}

func (m *mockStateManager) IsWorkflowRunning(_ context.Context, id core.WorkflowID) (bool, error) {
	if wf, exists := m.workflows[id]; exists {
		return wf.Status == core.WorkflowStatusRunning, nil
	}
	return false, nil
}

func (m *mockStateManager) UpdateWorkflowHeartbeat(_ context.Context, id core.WorkflowID) error {
	if wf, exists := m.workflows[id]; exists {
		now := time.Now().UTC()
		wf.HeartbeatAt = &now
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) FindWorkflowsByPrompt(_ context.Context, _ string) ([]core.DuplicateWorkflowInfo, error) {
	return nil, nil
}

func (m *mockStateManager) ExecuteAtomically(_ context.Context, fn func(core.AtomicStateContext) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomicCtx := &mockAtomicStateContext{m: m}
	return fn(atomicCtx)
}

type mockAtomicStateContext struct {
	m *mockStateManager
}

func (a *mockAtomicStateContext) LoadByID(id core.WorkflowID) (*core.WorkflowState, error) {
	if wf, exists := a.m.workflows[id]; exists {
		return wf, nil
	}
	return nil, nil
}

func (a *mockAtomicStateContext) Save(state *core.WorkflowState) error {
	a.m.workflows[state.WorkflowID] = state
	return nil
}

func (a *mockAtomicStateContext) SetWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicStateContext) ClearWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicStateContext) IsWorkflowRunning(id core.WorkflowID) (bool, error) {
	if wf, exists := a.m.workflows[id]; exists {
		return wf.Status == core.WorkflowStatusRunning, nil
	}
	return false, nil
}

func TestHealthEndpoint(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp["status"])
	}
}

func TestListWorkflowsEmpty(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp []WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(resp))
	}
}

func TestCreateWorkflow(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	reqBody := CreateWorkflowRequest{
		Prompt: "Test workflow prompt",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Prompt != reqBody.Prompt {
		t.Errorf("expected prompt '%s', got '%s'", reqBody.Prompt, resp.Prompt)
	}

	if resp.Status != string(core.WorkflowStatusPending) {
		t.Errorf("expected status '%s', got '%s'", core.WorkflowStatusPending, resp.Status)
	}

	if !resp.IsActive {
		t.Error("expected workflow to be active")
	}
}

func TestCreateWorkflowMissingPrompt(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	reqBody := CreateWorkflowRequest{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetWorkflow(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create a workflow first
	wfID := core.WorkflowID("wf-test-123")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhasePlan,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	sm.workflows[wfID] = state
	sm.activeID = wfID

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != string(wfID) {
		t.Errorf("expected ID '%s', got '%s'", wfID, resp.ID)
	}

	if resp.Status != string(core.WorkflowStatusRunning) {
		t.Errorf("expected status '%s', got '%s'", core.WorkflowStatusRunning, resp.Status)
	}
}

func TestGetWorkflowNotFound(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestListTasks(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create a workflow with tasks
	wfID := core.WorkflowID("wf-test-tasks")
	taskID := core.TaskID("task-1")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			taskID: {
				ID:     taskID,
				Phase:  core.PhaseExecute,
				Name:   "Test Task",
				Status: core.TaskStatusCompleted,
				CLI:    "claude",
				Model:  "opus",
			},
		},
		TaskOrder: []core.TaskID{taskID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	sm.workflows[wfID] = state

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/tasks/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp []TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 1 {
		t.Errorf("expected 1 task, got %d", len(resp))
	}

	if resp[0].ID != string(taskID) {
		t.Errorf("expected task ID '%s', got '%s'", taskID, resp[0].ID)
	}
}

func TestGetTask(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create a workflow with a task
	wfID := core.WorkflowID("wf-test-get-task")
	taskID := core.TaskID("task-get-1")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			taskID: {
				ID:       taskID,
				Phase:    core.PhaseExecute,
				Name:     "Get Task Test",
				Status:   core.TaskStatusRunning,
				CLI:      "gemini",
				Model:    "pro",
				TokensIn: 100,
			},
		},
		TaskOrder: []core.TaskID{taskID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	sm.workflows[wfID] = state

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/tasks/"+string(taskID), nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != string(taskID) {
		t.Errorf("expected task ID '%s', got '%s'", taskID, resp.ID)
	}

	if resp.TokensIn != 100 {
		t.Errorf("expected tokens_in 100, got %d", resp.TokensIn)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create a workflow without the requested task
	wfID := core.WorkflowID("wf-test-no-task")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	sm.workflows[wfID] = state

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/tasks/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestActivateWorkflow(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create two workflows
	wfID1 := core.WorkflowID("wf-1")
	wfID2 := core.WorkflowID("wf-2")

	sm.workflows[wfID1] = &core.WorkflowState{
		WorkflowID: wfID1,
		Status:     core.WorkflowStatusCompleted,
		Prompt:     "First workflow",
		Tasks:      make(map[core.TaskID]*core.TaskState),
		TaskOrder:  []core.TaskID{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	sm.workflows[wfID2] = &core.WorkflowState{
		WorkflowID: wfID2,
		Status:     core.WorkflowStatusPending,
		Prompt:     "Second workflow",
		Tasks:      make(map[core.TaskID]*core.TaskState),
		TaskOrder:  []core.TaskID{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	sm.activeID = wfID1

	// Activate wf-2
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+string(wfID2)+"/activate", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.IsActive {
		t.Error("expected workflow to be active")
	}

	if sm.activeID != wfID2 {
		t.Errorf("expected active ID '%s', got '%s'", wfID2, sm.activeID)
	}
}

func TestUpdateWorkflow(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create a workflow
	wfID := core.WorkflowID("wf-update")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusPending,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test prompt",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	sm.workflows[wfID] = state

	// Update status
	updateReq := map[string]string{
		"status": string(core.WorkflowStatusRunning),
		"phase":  string(core.PhasePlan),
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+string(wfID)+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != string(core.WorkflowStatusRunning) {
		t.Errorf("expected status '%s', got '%s'", core.WorkflowStatusRunning, resp.Status)
	}

	if resp.CurrentPhase != string(core.PhasePlan) {
		t.Errorf("expected phase '%s', got '%s'", core.PhasePlan, resp.CurrentPhase)
	}
}

func TestGetActiveWorkflow(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	// Create and activate a workflow
	wfID := core.WorkflowID("wf-active-test")
	state := &core.WorkflowState{
		WorkflowID:   wfID,
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "Active workflow",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	sm.workflows[wfID] = state
	sm.activeID = wfID

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/active", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != string(wfID) {
		t.Errorf("expected ID '%s', got '%s'", wfID, resp.ID)
	}

	if !resp.IsActive {
		t.Error("expected workflow to be active")
	}
}

func TestGetActiveWorkflowNone(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/active", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
