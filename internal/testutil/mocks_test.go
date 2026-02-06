package testutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestMockAgent_Name(t *testing.T) {
	mock := testutil.NewMockAgent("test-agent")
	testutil.AssertEqual(t, mock.Name(), "test-agent")
}

func TestMockAgent_Capabilities(t *testing.T) {
	mock := testutil.NewMockAgent("test")
	caps := mock.Capabilities()

	testutil.AssertTrue(t, caps.SupportsJSON, "should support JSON")
	if caps.MaxContextTokens <= 0 {
		t.Error("MaxContextTokens should be positive")
	}
}

func TestMockAgent_Execute(t *testing.T) {
	mock := testutil.NewMockAgent("test")

	result, err := mock.Execute(context.Background(), core.ExecuteOptions{
		Prompt: "test prompt",
	})

	testutil.AssertNoError(t, err)
	testutil.AssertContains(t, result.Output, "Mock response")
	testutil.AssertEqual(t, mock.CallCount("Execute"), 1)
}

func TestMockAgent_WithResponse(t *testing.T) {
	mock := testutil.NewMockAgent("test").WithResponse("custom response")

	result, err := mock.Execute(context.Background(), core.ExecuteOptions{
		Prompt: "test",
	})

	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, result.Output, "custom response")
}

func TestMockAgent_WithError(t *testing.T) {
	expectedErr := errors.New("test error")
	mock := testutil.NewMockAgent("test").WithError(expectedErr)

	_, err := mock.Execute(context.Background(), core.ExecuteOptions{
		Prompt: "test",
	})

	testutil.AssertError(t, err)
	if !errors.Is(err, expectedErr) {
		t.Errorf("got error %v, want %v", err, expectedErr)
	}
}

func TestMockAgent_WithExecuteFunc(t *testing.T) {
	calls := 0
	mock := testutil.NewMockAgent("test").WithExecuteFunc(
		func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			calls++
			return &core.ExecuteResult{Output: "custom"}, nil
		},
	)

	mock.Execute(context.Background(), core.ExecuteOptions{Prompt: "test"})
	mock.Execute(context.Background(), core.ExecuteOptions{Prompt: "test2"})

	testutil.AssertEqual(t, calls, 2)
}

func TestMockAgent_Ping(t *testing.T) {
	mock := testutil.NewMockAgent("test")
	err := mock.Ping(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, mock.CallCount("Ping"), 1)
}

func TestMockAgent_WithPingFunc(t *testing.T) {
	expectedErr := errors.New("ping failed")
	mock := testutil.NewMockAgent("test").WithPingFunc(func(ctx context.Context) error {
		return expectedErr
	})

	err := mock.Ping(context.Background())
	testutil.AssertError(t, err)
}

func TestMockAgent_Reset(t *testing.T) {
	mock := testutil.NewMockAgent("test")
	mock.Execute(context.Background(), core.ExecuteOptions{Prompt: "test"})
	mock.Ping(context.Background())

	testutil.AssertEqual(t, len(mock.Calls()), 2)

	mock.Reset()
	testutil.AssertEqual(t, len(mock.Calls()), 0)
}

func TestMockAgent_WithCapabilities(t *testing.T) {
	mock := testutil.NewMockAgent("test").WithCapabilities(core.Capabilities{
		SupportsJSON:     false,
		MaxContextTokens: 50000,
	})

	caps := mock.Capabilities()
	testutil.AssertFalse(t, caps.SupportsJSON, "should not support JSON")
	testutil.AssertEqual(t, caps.MaxContextTokens, 50000)
}

func TestMockStateManager_Save_Load(t *testing.T) {
	sm := testutil.NewMockStateManager()

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "test-id"},
		WorkflowRun:        core.WorkflowRun{Status: core.WorkflowStatusRunning},
	}

	err := sm.Save(context.Background(), state)
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, sm.Exists(), "state should exist")

	loaded, err := sm.Load(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, loaded.WorkflowID, core.WorkflowID("test-id"))
}

func TestMockStateManager_Lock(t *testing.T) {
	sm := testutil.NewMockStateManager()

	err := sm.AcquireLock(context.Background())
	testutil.AssertNoError(t, err)

	// Second lock should fail
	err = sm.AcquireLock(context.Background())
	testutil.AssertError(t, err)

	// Release and retry
	err = sm.ReleaseLock(context.Background())
	testutil.AssertNoError(t, err)

	err = sm.AcquireLock(context.Background())
	testutil.AssertNoError(t, err)
}

func TestMockStateManager_WithSaveError(t *testing.T) {
	expectedErr := errors.New("save failed")
	sm := testutil.NewMockStateManager().WithSaveError(expectedErr)

	err := sm.Save(context.Background(), &core.WorkflowState{})
	testutil.AssertError(t, err)
}

func TestMockRegistry_Add_Get(t *testing.T) {
	registry := testutil.NewMockRegistry()

	agent := testutil.NewMockAgent("test")
	registry.Add("test", agent)

	got, err := registry.Get("test")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, got.Name(), "test")
}

func TestMockRegistry_Get_NotFound(t *testing.T) {
	registry := testutil.NewMockRegistry()

	_, err := registry.Get("nonexistent")
	testutil.AssertError(t, err)
}

func TestMockRegistry_List(t *testing.T) {
	registry := testutil.NewMockRegistry()
	registry.Add("agent1", testutil.NewMockAgent("agent1"))
	registry.Add("agent2", testutil.NewMockAgent("agent2"))

	names := registry.List()
	testutil.AssertLen(t, names, 2)
}

func TestMockRegistry_Available(t *testing.T) {
	registry := testutil.NewMockRegistry()

	goodAgent := testutil.NewMockAgent("good")
	badAgent := testutil.NewMockAgent("bad").WithPingFunc(func(ctx context.Context) error {
		return errors.New("ping failed")
	})

	registry.Add("good", goodAgent)
	registry.Add("bad", badAgent)

	available := registry.Available(context.Background())
	testutil.AssertLen(t, available, 1)
	testutil.AssertEqual(t, available[0], "good")
}
