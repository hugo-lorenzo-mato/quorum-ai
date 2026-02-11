package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

type recordingNotifier struct {
	lastKind    string
	lastAgent   string
	lastMessage string
	lastData    map[string]interface{}
}

func (n *recordingNotifier) PhaseStarted(_ core.Phase)                   {}
func (n *recordingNotifier) TaskStarted(_ *core.Task)                    {}
func (n *recordingNotifier) TaskCompleted(_ *core.Task, _ time.Duration) {}
func (n *recordingNotifier) TaskFailed(_ *core.Task, _ error)            {}
func (n *recordingNotifier) TaskSkipped(_ *core.Task, _ string)          {}
func (n *recordingNotifier) WorkflowStateUpdated(_ *core.WorkflowState)  {}
func (n *recordingNotifier) Log(_, _, _ string)                          {}
func (n *recordingNotifier) AgentEvent(kind, agent, message string, data map[string]interface{}) {
	n.lastKind = kind
	n.lastAgent = agent
	n.lastMessage = message
	n.lastData = data
}

type fakeStreamingAgent struct {
	handler core.AgentEventHandler
}

func (a *fakeStreamingAgent) Name() string { return "fake" }
func (a *fakeStreamingAgent) Capabilities() core.Capabilities {
	return core.Capabilities{SupportsStreaming: true}
}
func (a *fakeStreamingAgent) Ping(_ context.Context) error { return nil }
func (a *fakeStreamingAgent) Execute(_ context.Context, _ core.ExecuteOptions) (*core.ExecuteResult, error) {
	return &core.ExecuteResult{}, nil
}
func (a *fakeStreamingAgent) SetEventHandler(handler core.AgentEventHandler) { a.handler = handler }

func TestDefaultAgentName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	if got := defaultAgentName(cfg); got != "claude" {
		t.Fatalf("defaultAgentName(empty) = %q, want %q", got, "claude")
	}

	cfg.Agents.Default = "codex"
	if got := defaultAgentName(cfg); got != "codex" {
		t.Fatalf("defaultAgentName(set) = %q, want %q", got, "codex")
	}
}

func TestParseRunnerTimeouts(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Workflow.Timeout = "10s"
	cfg.Phases.Analyze.Timeout = "11s"
	cfg.Phases.Plan.Timeout = "12s"
	cfg.Phases.Execute.Timeout = "13s"

	timeout, analyze, plan, exec, err := parseRunnerTimeouts(cfg)
	if err != nil {
		t.Fatalf("parseRunnerTimeouts unexpected error: %v", err)
	}
	if timeout != 10*time.Second || analyze != 11*time.Second || plan != 12*time.Second || exec != 13*time.Second {
		t.Fatalf("parseRunnerTimeouts got %v %v %v %v, want 10s 11s 12s 13s", timeout, analyze, plan, exec)
	}

	cfg.Phases.Analyze.Timeout = "not-a-duration"
	if _, _, _, _, err := parseRunnerTimeouts(cfg); err == nil {
		t.Fatalf("parseRunnerTimeouts expected error for invalid duration")
	}
}

func TestCreateStateManager(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.State.Path = filepath.Join(tmpDir, "state") // exercise factory's .db normalization
	cfg.State.LockTTL = "1s"
	cfg.State.BackupPath = filepath.Join(tmpDir, "backup.db")

	logger := logging.NewNop()
	sm, err := createStateManager(cfg, logger)
	if err != nil {
		t.Fatalf("createStateManager unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = state.CloseStateManager(sm) })

	if _, err := os.Stat(filepath.Join(tmpDir, "state.db")); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}

	// Invalid lock ttl should not fail runner creation (warn + default).
	cfg.State.Path = filepath.Join(tmpDir, "state2.db")
	cfg.State.LockTTL = "nope"
	if sm2, err := createStateManager(cfg, logger); err != nil || sm2 == nil {
		t.Fatalf("createStateManager with invalid lock ttl got (%v, %v), want (non-nil, nil)", sm2, err)
	} else {
		t.Cleanup(func() { _ = state.CloseStateManager(sm2) })
	}
}

func TestCreateWorktreeManager_NoGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	cfg := &config.Config{}
	logger := logging.NewNop()
	mgr, err := createWorktreeManager(cfg, logger)
	if err != nil {
		t.Fatalf("createWorktreeManager unexpected error: %v", err)
	}
	if mgr != nil {
		t.Fatalf("expected nil worktree manager outside git repo")
	}
}

func TestConnectRegistryToOutputNotifier(t *testing.T) {
	t.Parallel()

	reg := cli.NewRegistry()
	agent := &fakeStreamingAgent{}
	if err := reg.Register("fake", agent); err != nil {
		t.Fatalf("Register: %v", err)
	}

	notifier := &recordingNotifier{}
	connectRegistryToOutputNotifier(reg, notifier)
	if agent.handler == nil {
		t.Fatalf("expected streaming handler to be set on agent")
	}

	ev := core.NewAgentEvent(core.AgentEventToolUse, "fake", "hello").WithData(map[string]any{"x": 1})
	agent.handler(ev)

	if notifier.lastKind != string(core.AgentEventToolUse) || notifier.lastAgent != "fake" || notifier.lastMessage != "hello" {
		t.Fatalf("notifier got (%q,%q,%q), want (%q,%q,%q)", notifier.lastKind, notifier.lastAgent, notifier.lastMessage, string(core.AgentEventToolUse), "fake", "hello")
	}
	if notifier.lastData == nil || notifier.lastData["x"] != 1 {
		t.Fatalf("notifier data got %#v, want x=1", notifier.lastData)
	}

	// Nil output notifier should be a no-op.
	reg2 := cli.NewRegistry()
	connectRegistryToOutputNotifier(reg2, nil)
}
