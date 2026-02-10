package chat

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

type mockWorkflowRunner struct {
	workflows []core.WorkflowSummary
	state     *core.WorkflowState
}

func (m *mockWorkflowRunner) Run(_ context.Context, _ string) error                        { return nil }
func (m *mockWorkflowRunner) Analyze(_ context.Context, _ string) error                     { return nil }
func (m *mockWorkflowRunner) Plan(_ context.Context) error                                  { return nil }
func (m *mockWorkflowRunner) Replan(_ context.Context, _ string) error                      { return nil }
func (m *mockWorkflowRunner) UsePlan(_ context.Context) error                               { return nil }
func (m *mockWorkflowRunner) Resume(_ context.Context) error                                { return nil }
func (m *mockWorkflowRunner) GetState(_ context.Context) (*core.WorkflowState, error)       { return m.state, nil }
func (m *mockWorkflowRunner) SaveState(_ context.Context, _ *core.WorkflowState) error      { return nil }
func (m *mockWorkflowRunner) ListWorkflows(_ context.Context) ([]core.WorkflowSummary, error) { return m.workflows, nil }
func (m *mockWorkflowRunner) LoadWorkflow(_ context.Context, workflowID string) (*core.WorkflowState, error) {
	if m.state != nil && string(m.state.WorkflowID) == workflowID {
		return m.state, nil
	}
	return nil, errors.New("not found")
}
func (m *mockWorkflowRunner) DeactivateWorkflow(_ context.Context) error { return nil }
func (m *mockWorkflowRunner) ArchiveWorkflows(_ context.Context) (int, error) {
	return 1, nil
}
func (m *mockWorkflowRunner) PurgeAllWorkflows(_ context.Context) (int, error) { return 2, nil }
func (m *mockWorkflowRunner) DeleteWorkflow(_ context.Context, workflowID string) error {
	if m.state != nil && string(m.state.WorkflowID) == workflowID {
		m.state = nil
	}
	return nil
}

func TestModel_UpdateAndView_Smoke(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	// Layout init
	modelAny, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = modelAny.(Model)

	// Exercise common message types without requiring a workflow runner or agents.
	modelAny, _ = m.Update(StatsTickMsg{})
	m = modelAny.(Model)

	modelAny, _ = m.Update(AgentResponseMsg{
		Agent:     "Claude",
		Content:   "hello",
		TokensIn:  10,
		TokensOut: 20,
	})
	m = modelAny.(Model)

	modelAny, _ = m.Update(ShellOutputMsg{
		Command:  "echo hi",
		Output:   "hi\n",
		ExitCode: 0,
	})
	m = modelAny.(Model)

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{Prompt: "p"},
		WorkflowRun:        core.WorkflowRun{Status: core.WorkflowStatusRunning},
	}
	modelAny, _ = m.Update(WorkflowUpdateMsg{State: state})
	m = modelAny.(Model)

	modelAny, _ = m.Update(WorkflowStartedMsg{Prompt: "do something"})
	m = modelAny.(Model)

	modelAny, _ = m.Update(WorkflowCompletedMsg{State: state})
	m = modelAny.(Model)

	v := m.View()
	if strings.TrimSpace(v) == "" {
		t.Fatalf("View() returned empty output")
	}
}

func TestModel_BuildConversationMessages_Truncates(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	long := strings.Repeat("x", 2500)
	m.history.Add(NewUserMessage("hi"))
	m.history.Add(NewAgentMessage("Claude", long))

	msgs := m.buildConversationMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if got := msgs[1].Content; len(got) <= 2000 {
		t.Fatalf("expected agent message to be truncated, got len=%d", len(got))
	}
}

func TestModel_ExecuteShellCommand_Smoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command execution via sh is not available on windows in this test")
	}

	m := NewModel(nil, nil, "claude", "default")
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	modelAny, cmd := m.executeShellCommand("printf 'ok'")
	_ = modelAny.(Model)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	msg := cmd()
	out, ok := msg.(ShellOutputMsg)
	if !ok {
		t.Fatalf("expected ShellOutputMsg, got %T", msg)
	}
	if out.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (err=%q)", out.ExitCode, out.Error)
	}
	if strings.TrimSpace(out.Output) != "ok" {
		t.Fatalf("unexpected output: %q", out.Output)
	}
}

func TestModel_HandleSubmit_CoversCommandPaths(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	modelAny, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	m = modelAny.(Model)

	inputs := []string{
		"/help",
		"/status",
		"/clear",
		"/model",
		"/model gpt-test",
		"/agent",
		"/agent codex",
		"/theme",
		"/theme light",
		"/theme dark",
		"/unknown this is not a real command",
		"regular message",
	}
	for _, in := range inputs {
		m.textarea.SetValue(in)
		modelAny, cmd := m.handleSubmit()
		m = modelAny.(Model)
		if cmd != nil {
			msg := cmd()
			modelAny, _ = m.Update(msg)
			m = modelAny.(Model)
		}
	}

	if runtime.GOOS != "windows" {
		m.textarea.SetValue("!printf 'ok'")
		modelAny, cmd := m.handleSubmit()
		m = modelAny.(Model)
		if cmd != nil {
			msg := cmd()
			modelAny, _ = m.Update(msg)
			m = modelAny.(Model)
		}
	}

	_ = m.View()
}

func TestModel_RenderHelpers_Smoke(t *testing.T) {
	m := NewModel(nil, nil, "claude", "default")
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	modelAny, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 55})
	m = modelAny.(Model)

	// Seed history with content that triggers markdown rendering paths.
	m.history.Add(NewUserMessage("hello"))
	m.history.Add(NewAgentMessage("Claude", "Here is `inline` and:\n\n```go\nfmt.Println(\"hi\")\n```"))
	m.updateViewport()

	// Suggestions overlay
	m.showSuggestions = true
	m.suggestions = []string{"/help", "/status", "/clear"}
	m.suggestionIndex = 1
	m.suggestionType = "command"

	// Panels and overlays toggles
	m.showExplorer = true
	m.showLogs = true
	m.showStats = true
	m.showTokens = true
	m.workflowRunning = true
	m.streaming = true

	if out := m.renderHistory(); strings.TrimSpace(out) == "" {
		t.Fatalf("renderHistory() empty")
	}
	if out := m.renderHeader(m.width); strings.TrimSpace(out) == "" {
		t.Fatalf("renderHeader() empty")
	}
	if out := m.renderMainContent(m.width); strings.TrimSpace(out) == "" {
		t.Fatalf("renderMainContent() empty")
	}
	if out := m.renderInput(m.width); strings.TrimSpace(out) == "" {
		t.Fatalf("renderInput() empty")
	}
	if out := m.renderInlineSuggestions(m.width); strings.TrimSpace(out) == "" {
		t.Fatalf("renderInlineSuggestions() empty")
	}
	if out := m.renderFooter(m.width); strings.TrimSpace(out) == "" {
		t.Fatalf("renderFooter() empty")
	}
	if out := m.renderFullScreenModal("modal", m.width, m.height); strings.TrimSpace(out) == "" {
		t.Fatalf("renderFullScreenModal() empty")
	}
}

func TestModel_HandleCommand_WithRunnerPaths(t *testing.T) {
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: core.WorkflowID("wf-1"),
			Prompt:     "test prompt",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			CurrentPhase: core.PhaseAnalyze,
			Metrics:      &core.StateMetrics{ConsensusScore: 0.9},
		},
	}
	r := &mockWorkflowRunner{
		workflows: []core.WorkflowSummary{
			{
				WorkflowID:   "wf-1",
				Status:       core.WorkflowStatusCompleted,
				CurrentPhase: core.PhaseAnalyze,
				Prompt:       "test prompt",
				IsActive:     true,
			},
		},
		state: state,
	}

	m := NewModel(nil, nil, "claude", "default").WithChatConfig(0, 0)
	m.runner = r
	t.Cleanup(func() {
		if m.explorerPanel != nil {
			m.explorerPanel.Close()
		}
	})

	modelAny, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 55})
	m = modelAny.(Model)

	// Commands that traverse runner-backed paths.
	inputs := []string{
		"/workflows",
		"/load",       // list workflows
		"/load wf-1",  // load workflow state
		"/new",        // deactivate
		"/new -a",     // archive
		"/new -p",     // purge
		"/delete wf-1",
	}
	for _, in := range inputs {
		m.textarea.SetValue(in)
		modelAny, cmd := m.handleSubmit()
		m = modelAny.(Model)
		if cmd != nil {
			msg := cmd()
			modelAny, _ = m.Update(msg)
			m = modelAny.(Model)
		}
	}

	_ = m.View()
}
