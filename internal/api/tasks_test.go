package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestHandleGetTask_IncludesOutputFile(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowID:   "wf-1",
		Status:       core.WorkflowStatusCompleted,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "test",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:          "task-1",
				Phase:       core.PhaseExecute,
				Name:        "Task 1",
				Status:      core.TaskStatusCompleted,
				CLI:         "claude",
				Model:       "test-model",
				Output:      "short output",
				OutputFile:  ".quorum/outputs/task-1.txt",
				StartedAt:   ptrTime(time.Now().Add(-1 * time.Minute)),
				CompletedAt: ptrTime(time.Now()),
			},
		},
		TaskOrder: []core.TaskID{"task-1"},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now(),
	}

	eb := events.New(100)
	srv := NewServer(sm, eb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/tasks/task-1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.OutputFile != ".quorum/outputs/task-1.txt" {
		t.Errorf("expected output_file %q, got %q", ".quorum/outputs/task-1.txt", resp.OutputFile)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
