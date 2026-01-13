package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockDAGBuilder is a test DAG implementation for planner tests.
type mockDAGBuilder struct {
	tasks map[core.TaskID]*core.Task
	deps  map[core.TaskID][]core.TaskID
}

func (m *mockDAGBuilder) AddTask(task *core.Task) error {
	if m.tasks == nil {
		m.tasks = make(map[core.TaskID]*core.Task)
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockDAGBuilder) AddDependency(from, to core.TaskID) error {
	if m.deps == nil {
		m.deps = make(map[core.TaskID][]core.TaskID)
	}
	m.deps[from] = append(m.deps[from], to)
	return nil
}

func (m *mockDAGBuilder) Build() (interface{}, error) {
	return m, nil
}

func (m *mockDAGBuilder) GetReadyTasks(completed map[core.TaskID]bool) []*core.Task {
	var ready []*core.Task
	for id, task := range m.tasks {
		if completed[id] {
			continue
		}
		deps := m.deps[id]
		allDepsComplete := true
		for _, dep := range deps {
			if !completed[dep] {
				allDepsComplete = false
				break
			}
		}
		if allDepsComplete {
			ready = append(ready, task)
		}
	}
	return ready
}

// mockStateSaver is a test state saver for planner tests.
type mockStateSaver struct {
	state *core.WorkflowState
	err   error
}

func (m *mockStateSaver) Save(_ context.Context, state *core.WorkflowState) error {
	if m.err != nil {
		return m.err
	}
	m.state = state
	return nil
}

func TestNewPlanner(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}

	planner := NewPlanner(dag, saver)

	if planner == nil {
		t.Fatal("NewPlanner() returned nil")
	}
	if planner.dag != dag {
		t.Error("dag not set correctly")
	}
	if planner.stateSaver != saver {
		t.Error("stateSaver not set correctly")
	}
}

func TestParsePlanItems_ValidJSONArray(t *testing.T) {
	input := `[
		{"id": "task-1", "name": "Task 1", "description": "First task", "cli": "claude", "dependencies": []},
		{"id": "task-2", "name": "Task 2", "description": "Second task", "cli": "gemini", "dependencies": ["task-1"]}
	]`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("parsePlanItems() returned %d items, want 2", len(items))
	}

	if items[0].ID != "task-1" {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, "task-1")
	}
	if items[0].Name != "Task 1" {
		t.Errorf("items[0].Name = %q, want %q", items[0].Name, "Task 1")
	}
	if items[1].CLI != "gemini" {
		t.Errorf("items[1].CLI = %q, want %q", items[1].CLI, "gemini")
	}
	if len(items[1].Dependencies) != 1 || items[1].Dependencies[0] != "task-1" {
		t.Errorf("items[1].Dependencies = %v, want [task-1]", items[1].Dependencies)
	}
}

func TestParsePlanItems_WrappedTasks(t *testing.T) {
	input := `{"tasks": [{"id": "task-1", "name": "Task 1", "description": "First task"}]}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}

	if items[0].ID != "task-1" {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, "task-1")
	}
}

func TestParsePlanItems_EmptyOutput(t *testing.T) {
	_, err := parsePlanItems("")
	if err == nil {
		t.Error("parsePlanItems() should return error for empty output")
	}
}

func TestParsePlanItems_WhitespaceOnly(t *testing.T) {
	_, err := parsePlanItems("   \n\t  ")
	if err == nil {
		t.Error("parsePlanItems() should return error for whitespace only")
	}
}

func TestParsePlanItems_InvalidJSON(t *testing.T) {
	_, err := parsePlanItems("not json at all")
	if err == nil {
		t.Error("parsePlanItems() should return error for invalid JSON")
	}
}

func TestParsePlanItems_EmbeddedJSON(t *testing.T) {
	input := `Here is the plan:
[{"id": "task-1", "name": "Task 1", "description": "Do something"}]
That's all.`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_GeminiFormat(t *testing.T) {
	input := `{
		"candidates": [{
			"content": {
				"parts": [{
					"text": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"
				}]
			}
		}]
	}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_ResultWrapper(t *testing.T) {
	input := `{"result": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_ContentWrapper(t *testing.T) {
	input := `{"content": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_MissingTasksField(t *testing.T) {
	input := `{"other_field": "value"}`

	_, err := parsePlanItems(input)
	if err == nil {
		t.Error("parsePlanItems() should return error when tasks field is missing")
	}
}

func TestExtractJSON_Array(t *testing.T) {
	input := `Some text before [{"key": "value"}] some text after`
	result := extractJSON(input)

	if result != `[{"key": "value"}]` {
		t.Errorf("extractJSON() = %q, want %q", result, `[{"key": "value"}]`)
	}
}

func TestExtractJSON_Object(t *testing.T) {
	input := `Prefix {"key": "value"} suffix`
	result := extractJSON(input)

	if result != `{"key": "value"}` {
		t.Errorf("extractJSON() = %q, want %q", result, `{"key": "value"}`)
	}
}

func TestExtractJSON_Nested(t *testing.T) {
	input := `Start {"outer": {"inner": "value"}} end`
	result := extractJSON(input)

	if result != `{"outer": {"inner": "value"}}` {
		t.Errorf("extractJSON() = %q, want %q", result, `{"outer": {"inner": "value"}}`)
	}
}

func TestExtractJSON_WithStrings(t *testing.T) {
	input := `Text {"key": "value with { braces }"} more`
	result := extractJSON(input)

	if result != `{"key": "value with { braces }"}` {
		t.Errorf("extractJSON() = %q, want %q", result, `{"key": "value with { braces }"}`)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := `No JSON here at all`
	result := extractJSON(input)

	if result != "" {
		t.Errorf("extractJSON() = %q, want empty string", result)
	}
}

func TestExtractJSON_EscapedQuotes(t *testing.T) {
	input := `{"key": "value with \"escaped\" quotes"}`
	result := extractJSON(input)

	if result != `{"key": "value with \"escaped\" quotes"}` {
		t.Errorf("extractJSON() = %q", result)
	}
}

func TestRawToText_DirectString(t *testing.T) {
	raw := json.RawMessage(`"direct string value"`)
	result := rawToText(raw)

	if result != "direct string value" {
		t.Errorf("rawToText() = %q, want %q", result, "direct string value")
	}
}

func TestRawToText_PartsArray(t *testing.T) {
	raw := json.RawMessage(`[{"text": "part1"}, {"text": "part2"}]`)
	result := rawToText(raw)

	if result != "part1\npart2" {
		t.Errorf("rawToText() = %q, want %q", result, "part1\npart2")
	}
}

func TestRawToText_ObjectWithText(t *testing.T) {
	raw := json.RawMessage(`{"text": "text value"}`)
	result := rawToText(raw)

	if result != "text value" {
		t.Errorf("rawToText() = %q, want %q", result, "text value")
	}
}

func TestRawToText_ObjectWithContent(t *testing.T) {
	raw := json.RawMessage(`{"content": "content value"}`)
	result := rawToText(raw)

	if result != "content value" {
		t.Errorf("rawToText() = %q, want %q", result, "content value")
	}
}

func TestRawToText_ObjectWithParts(t *testing.T) {
	raw := json.RawMessage(`{"parts": [{"text": "p1"}, {"text": "p2"}]}`)
	result := rawToText(raw)

	if result != "p1\np2" {
		t.Errorf("rawToText() = %q, want %q", result, "p1\np2")
	}
}

func TestRawToText_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`[]`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty string", result)
	}
}

func TestRawToText_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty string", result)
	}
}

func TestIsShellLikeAgent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"bash", true},
		{"sh", true},
		{"zsh", true},
		{"fish", true},
		{"powershell", true},
		{"pwsh", true},
		{"terminal", true},
		{"shell", true},
		{"command", true},
		{"cli", true},
		{"default", true},
		{"auto", true},
		{"BASH", true},
		{"  bash  ", true},
		{"claude", false},
		{"gemini", false},
		{"copilot", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isShellLikeAgent(tt.input)
			if got != tt.want {
				t.Errorf("isShellLikeAgent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTaskAgent(t *testing.T) {
	tests := []struct {
		name         string
		candidate    string
		agents       []string
		defaultAgent string
		want         string
	}{
		{
			name:         "empty returns default",
			candidate:    "",
			agents:       []string{"claude", "gemini"},
			defaultAgent: "claude",
			want:         "claude",
		},
		{
			name:         "shell name returns default",
			candidate:    "bash",
			agents:       []string{"claude", "gemini"},
			defaultAgent: "claude",
			want:         "claude",
		},
		{
			name:         "known agent case insensitive",
			candidate:    "GEMINI",
			agents:       []string{"claude", "gemini"},
			defaultAgent: "claude",
			want:         "gemini",
		},
		{
			name:         "unknown agent returns default",
			candidate:    "unknown-agent",
			agents:       []string{"claude", "gemini"},
			defaultAgent: "claude",
			want:         "claude",
		},
		{
			name:         "known agent exact match",
			candidate:    "gemini",
			agents:       []string{"claude", "gemini"},
			defaultAgent: "claude",
			want:         "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockAgentRegistry{}
			for _, name := range tt.agents {
				registry.Register(name, &mockAgent{})
			}

			wctx := &Context{
				Agents: registry,
				Config: &Config{DefaultAgent: tt.defaultAgent},
				Logger: logging.NewNop(),
			}

			got := resolveTaskAgent(wctx, tt.candidate)
			if got != tt.want {
				t.Errorf("resolveTaskAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlanner_parsePlan(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})
	registry.Register("gemini", &mockAgent{})

	wctx := &Context{
		Agents: registry,
		Config: &Config{DefaultAgent: "claude"},
		Logger: logging.NewNop(),
	}

	output := `[
		{"id": "task-1", "name": "Task 1", "description": "First", "cli": "claude", "dependencies": []},
		{"id": "task-2", "name": "Task 2", "description": "Second", "agent": "gemini", "dependencies": ["task-1"]}
	]`

	tasks, err := planner.parsePlan(wctx, output)
	if err != nil {
		t.Fatalf("parsePlan() error = %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("parsePlan() returned %d tasks, want 2", len(tasks))
	}

	if tasks[0].ID != "task-1" {
		t.Errorf("tasks[0].ID = %q, want %q", tasks[0].ID, "task-1")
	}
	if tasks[0].CLI != "claude" {
		t.Errorf("tasks[0].CLI = %q, want %q", tasks[0].CLI, "claude")
	}
	if tasks[0].Phase != core.PhaseExecute {
		t.Errorf("tasks[0].Phase = %v, want %v", tasks[0].Phase, core.PhaseExecute)
	}
	if tasks[0].Status != core.TaskStatusPending {
		t.Errorf("tasks[0].Status = %v, want %v", tasks[0].Status, core.TaskStatusPending)
	}

	if tasks[1].CLI != "gemini" {
		t.Errorf("tasks[1].CLI = %q, want %q", tasks[1].CLI, "gemini")
	}
	if len(tasks[1].Dependencies) != 1 || tasks[1].Dependencies[0] != "task-1" {
		t.Errorf("tasks[1].Dependencies = %v, want [task-1]", tasks[1].Dependencies)
	}
}

func TestPlanner_parsePlan_EmptyOutput(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	wctx := &Context{
		Agents: &mockAgentRegistry{},
		Config: &Config{DefaultAgent: "claude"},
		Logger: logging.NewNop(),
	}

	_, err := planner.parsePlan(wctx, "")
	if err == nil {
		t.Error("parsePlan() should return error for empty output")
	}
}

func TestPlanner_parsePlan_NoTasks(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	wctx := &Context{
		Agents: &mockAgentRegistry{},
		Config: &Config{DefaultAgent: "claude"},
		Logger: logging.NewNop(),
	}

	_, err := planner.parsePlan(wctx, "[]")
	if err == nil {
		t.Error("parsePlan() should return error for empty tasks array")
	}
}

func TestPlanner_parsePlan_UsesAgentField(t *testing.T) {
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	planner := NewPlanner(dag, saver)

	registry := &mockAgentRegistry{}
	registry.Register("claude", &mockAgent{})
	registry.Register("gemini", &mockAgent{})

	wctx := &Context{
		Agents: registry,
		Config: &Config{DefaultAgent: "claude"},
		Logger: logging.NewNop(),
	}

	output := `[{"id": "task-1", "name": "Task 1", "description": "First", "agent": "gemini", "dependencies": []}]`

	tasks, err := planner.parsePlan(wctx, output)
	if err != nil {
		t.Fatalf("parsePlan() error = %v", err)
	}

	if tasks[0].CLI != "gemini" {
		t.Errorf("tasks[0].CLI = %q, want %q (from agent field)", tasks[0].CLI, "gemini")
	}
}

func TestParsePlanItems_TextWrapper(t *testing.T) {
	input := `{"text": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestParsePlanItems_OutputWrapper(t *testing.T) {
	input := `{"output": "[{\"id\": \"task-1\", \"name\": \"Task 1\", \"description\": \"Test\"}]"}`

	items, err := parsePlanItems(input)
	if err != nil {
		t.Fatalf("parsePlanItems() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("parsePlanItems() returned %d items, want 1", len(items))
	}
}

func TestExtractJSON_UnbalancedBraces(t *testing.T) {
	input := `Text {"key": "value" extra text without closing`
	result := extractJSON(input)

	// Should return empty since JSON is not properly closed
	if result != "" {
		t.Errorf("extractJSON() = %q, want empty for unbalanced", result)
	}
}

func TestRawToText_EmptyPartsArray(t *testing.T) {
	raw := json.RawMessage(`{"parts": []}`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty string", result)
	}
}

func TestRawToText_WhitespaceOnlyText(t *testing.T) {
	raw := json.RawMessage(`{"text": "   "}`)
	result := rawToText(raw)

	if result != "" {
		t.Errorf("rawToText() = %q, want empty string after trim", result)
	}
}

func TestTaskPlanItem_Fields(t *testing.T) {
	item := TaskPlanItem{
		ID:           "task-1",
		Name:         "Test Task",
		Description:  "A test task",
		CLI:          "claude",
		Agent:        "claude",
		Dependencies: []string{"task-0"},
	}

	if item.ID != "task-1" {
		t.Errorf("ID = %q, want %q", item.ID, "task-1")
	}
	if item.Name != "Test Task" {
		t.Errorf("Name = %q, want %q", item.Name, "Test Task")
	}
	if item.CLI != "claude" {
		t.Errorf("CLI = %q, want %q", item.CLI, "claude")
	}
	if len(item.Dependencies) != 1 {
		t.Errorf("len(Dependencies) = %d, want 1", len(item.Dependencies))
	}
}
