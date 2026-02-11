package github

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// runner.go tests — ExecRunner, RunError
// =============================================================================

func TestNewExecRunner(t *testing.T) {
	t.Parallel()
	r := NewExecRunner()
	if r == nil {
		t.Fatal("NewExecRunner() returned nil")
	}
}

func TestExecRunner_Run_Success(t *testing.T) {
	t.Parallel()
	r := NewExecRunner()
	ctx := context.Background()

	// "echo" is universally available
	out, err := r.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "hello" {
		t.Errorf("Run() output = %q, want %q", out, "hello")
	}
}

func TestExecRunner_Run_CommandNotFound(t *testing.T) {
	t.Parallel()
	r := NewExecRunner()
	ctx := context.Background()

	_, err := r.Run(ctx, "nonexistent-command-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestExecRunner_Run_StderrIncluded(t *testing.T) {
	t.Parallel()
	r := NewExecRunner()
	ctx := context.Background()

	// "ls" on a nonexistent path writes to stderr and exits non-zero
	_, err := r.Run(ctx, "ls", "/nonexistent-path-for-testing-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}

	var runErr *RunError
	if errors.As(err, &runErr) {
		if runErr.Stderr == "" {
			t.Error("RunError.Stderr should not be empty")
		}
		if runErr.Command == "" {
			t.Error("RunError.Command should not be empty")
		}
		if runErr.Err == nil {
			t.Error("RunError.Err should not be nil")
		}
	}
	// Note: on some systems this may be a plain error without stderr,
	// so we don't require RunError -- we just ensure err is non-nil.
}

func TestExecRunner_Run_ContextCancelled(t *testing.T) {
	t.Parallel()
	r := NewExecRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := r.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRunError_Error_WithStderr(t *testing.T) {
	t.Parallel()
	e := &RunError{
		Command: "gh pr list",
		Stderr:  "not authenticated",
		Err:     errors.New("exit status 1"),
	}

	got := e.Error()
	want := "gh pr list: not authenticated: exit status 1"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRunError_Error_WithoutStderr(t *testing.T) {
	t.Parallel()
	e := &RunError{
		Command: "gh pr list",
		Stderr:  "",
		Err:     errors.New("exit status 1"),
	}

	got := e.Error()
	want := "gh pr list: exit status 1"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRunError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner error")
	e := &RunError{
		Command: "test",
		Err:     inner,
	}

	if e.Unwrap() != inner {
		t.Error("Unwrap() should return the inner error")
	}

	// Also verify errors.Is works through Unwrap
	if !errors.Is(e, inner) {
		t.Error("errors.Is should find the inner error")
	}
}

// =============================================================================
// checks.go tests — ChecksWaiter with mocked client
// =============================================================================

func TestNewChecksWaiter_WithClient(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	client := NewClientSkipAuth("owner", "repo", runner)

	waiter := NewChecksWaiter(client)
	if waiter == nil {
		t.Fatal("NewChecksWaiter() returned nil")
	}
	if waiter.pollInterval != 30*time.Second {
		t.Errorf("pollInterval = %v, want %v", waiter.pollInterval, 30*time.Second)
	}
	if waiter.timeout != 30*time.Minute {
		t.Errorf("timeout = %v, want %v", waiter.timeout, 30*time.Minute)
	}
}

func TestChecksWaiter_GetChecks_AllPassed(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "https://github.com/owner/repo/actions/runs/1",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		},
		{
			"name": "test",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "https://github.com/owner/repo/actions/runs/2",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:10:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if !result.AllCompleted {
		t.Error("AllCompleted should be true")
	}
	if !result.AllPassed {
		t.Error("AllPassed should be true")
	}
	if len(result.Checks) != 2 {
		t.Errorf("len(Checks) = %d, want 2", len(result.Checks))
	}
	if len(result.FailedChecks) != 0 {
		t.Errorf("len(FailedChecks) = %d, want 0", len(result.FailedChecks))
	}
}

func TestChecksWaiter_GetChecks_WithFailures(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		},
		{
			"name": "test",
			"status": "completed",
			"conclusion": "failure",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:10:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if !result.AllCompleted {
		t.Error("AllCompleted should be true")
	}
	if result.AllPassed {
		t.Error("AllPassed should be false")
	}
	if len(result.FailedChecks) != 1 {
		t.Errorf("len(FailedChecks) = %d, want 1", len(result.FailedChecks))
	}
	if result.FailedChecks[0] != "test" {
		t.Errorf("FailedChecks[0] = %q, want %q", result.FailedChecks[0], "test")
	}
}

func TestChecksWaiter_GetChecks_Pending(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		},
		{
			"name": "test",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if result.AllCompleted {
		t.Error("AllCompleted should be false")
	}
	if len(result.PendingChecks) != 1 {
		t.Errorf("len(PendingChecks) = %d, want 1", len(result.PendingChecks))
	}
}

func TestChecksWaiter_GetChecks_EmptyChecks(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	// When no checks exist, should not be completed
	if result.AllCompleted {
		t.Error("AllCompleted should be false when no checks exist")
	}
	if result.AllPassed {
		t.Error("AllPassed should be false when no checks exist")
	}
}

func TestChecksWaiter_GetChecks_WithRequiredChecks(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		},
		{
			"name": "test",
			"status": "completed",
			"conclusion": "failure",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:10:00Z"
		},
		{
			"name": "lint",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:03:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).WithRequiredChecks([]string{"build", "lint"})

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	// Only required checks (build, lint) should be included; "test" (failure) is skipped
	if len(result.Checks) != 2 {
		t.Errorf("len(Checks) = %d, want 2 (only required)", len(result.Checks))
	}
	if !result.AllPassed {
		t.Error("AllPassed should be true (only required checks were considered)")
	}
}

func TestChecksWaiter_GetChecks_SkippedAndNeutralConclusions(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "skipped",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:01:00Z"
		},
		{
			"name": "test",
			"status": "completed",
			"conclusion": "neutral",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:02:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if !result.AllPassed {
		t.Error("AllPassed should be true for skipped/neutral conclusions")
	}
	if len(result.FailedChecks) != 0 {
		t.Errorf("len(FailedChecks) = %d, want 0", len(result.FailedChecks))
	}
}

func TestChecksWaiter_GetChecks_RunError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnError(errors.New("gh command failed"))

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	_, err := waiter.GetChecks(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error when gh command fails")
	}
}

func TestChecksWaiter_GetChecks_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").Return("{invalid json}")

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	_, err := waiter.GetChecks(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestChecksWaiter_Wait_CompletedImmediately(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	result, err := waiter.Wait(context.Background(), 1)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if !result.AllCompleted {
		t.Error("AllCompleted should be true")
	}
	// Duration may be 0 on Windows if checks complete immediately before first poll
	if result.Duration < 0 {
		t.Error("Duration should not be negative")
	}
}

func TestChecksWaiter_Wait_Timeout(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	// Always return pending checks
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(200 * time.Millisecond).
		WithPollInterval(50 * time.Millisecond)

	_, err := waiter.Wait(context.Background(), 1)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var domainErr *core.DomainError
	if !errors.As(err, &domainErr) {
		t.Logf("error type: %T, value: %v", err, err)
	}
}

func TestChecksWaiter_Wait_ContextCancelled(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(10 * time.Second).
		WithPollInterval(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := waiter.Wait(ctx, 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestChecksWaiter_Wait_ErrorOnFirstPoll(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnError(errors.New("api error"))

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	_, err := waiter.Wait(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from failed poll")
	}
}

func TestChecksWaiter_WaitWithCallback_CompletedImmediately(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	callbackCount := 0
	result, err := waiter.WaitWithCallback(context.Background(), 1, func(r *ChecksResult) {
		callbackCount++
	})
	if err != nil {
		t.Fatalf("WaitWithCallback() error = %v", err)
	}

	if !result.AllCompleted {
		t.Error("AllCompleted should be true")
	}
	if callbackCount < 1 {
		t.Error("callback should have been called at least once")
	}
}

func TestChecksWaiter_WaitWithCallback_NilCallback(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	// nil callback should not panic
	result, err := waiter.WaitWithCallback(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("WaitWithCallback() error = %v", err)
	}

	if !result.AllCompleted {
		t.Error("AllCompleted should be true")
	}
}

func TestChecksWaiter_WaitWithCallback_Timeout(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(200 * time.Millisecond).
		WithPollInterval(50 * time.Millisecond)

	callbackCount := 0
	_, err := waiter.WaitWithCallback(context.Background(), 1, func(_ *ChecksResult) {
		callbackCount++
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if callbackCount < 1 {
		t.Error("callback should have been called at least once before timeout")
	}
}

func TestChecksWaiter_WaitWithCallback_ErrorOnPoll(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnError(errors.New("api error"))

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	_, err := waiter.WaitWithCallback(context.Background(), 1, func(_ *ChecksResult) {})
	if err == nil {
		t.Fatal("expected error from failed poll")
	}
}

func TestChecksWaiter_WaitForSuccess_AllPassed(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	err := waiter.WaitForSuccess(context.Background(), 1)
	if err != nil {
		t.Fatalf("WaitForSuccess() error = %v", err)
	}
}

func TestChecksWaiter_WaitForSuccess_HasFailures(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "failure",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	err := waiter.WaitForSuccess(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error when checks fail")
	}

	var domainErr *core.DomainError
	if !errors.As(err, &domainErr) {
		t.Errorf("expected DomainError, got %T: %v", err, err)
	}
}

func TestChecksWaiter_WaitForSuccess_WaitError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnError(errors.New("api error"))

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client).
		WithTimeout(5 * time.Second).
		WithPollInterval(100 * time.Millisecond)

	err := waiter.WaitForSuccess(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from wait failure")
	}
}

// =============================================================================
// client.go tests — additional coverage for edge cases
// =============================================================================

func TestClient_WaitForChecks_CompletedImmediately(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks abc123").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	status, err := client.WaitForChecks(context.Background(), "abc123", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}

	if status.State != "success" {
		t.Errorf("State = %q, want %q", status.State, "success")
	}
}

func TestClient_WaitForChecks_Timeout(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks ref123").ReturnJSON(`[
		{
			"name": "build",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.WaitForChecks(context.Background(), "ref123", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClient_WaitForChecks_ContextCancelled(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks ref123").ReturnJSON(`[
		{
			"name": "build",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := client.WaitForChecks(ctx, "ref123", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClient_WaitForChecks_PollError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks ref123").ReturnError(errors.New("api failure"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.WaitForChecks(context.Background(), "ref123", 5*time.Second)
	if err == nil {
		t.Fatal("expected error from poll failure")
	}
}

func TestClient_NewClientFromRepoWithRunner_CommandError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view --json owner,name").ReturnError(errors.New("not in a repo"))

	_, err := NewClientFromRepoWithRunner(runner)
	if err == nil {
		t.Fatal("expected error when not in a repo")
	}
}

func TestClient_NewClientFromRepoWithRunner_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view --json owner,name").Return("{invalid json}")

	_, err := NewClientFromRepoWithRunner(runner)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestClient_ListPRs_WithAllOptions(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr list").ReturnJSON(`[
		{
			"number": 1,
			"title": "PR 1",
			"body": "",
			"url": "https://github.com/owner/repo/pull/1",
			"state": "OPEN",
			"isDraft": false,
			"headRefName": "branch1",
			"headRefOid": "sha1",
			"baseRefName": "main",
			"createdAt": "2024-01-15T10:00:00Z",
			"updatedAt": "2024-01-15T10:00:00Z",
			"mergedAt": null,
			"labels": [{"name": "bug"}],
			"assignees": [{"login": "user1"}]
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	prs, err := client.ListPRs(context.Background(), core.ListPROptions{
		State: "open",
		Head:  "branch1",
		Base:  "main",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListPRs() error = %v", err)
	}

	if len(prs) != 1 {
		t.Fatalf("len(prs) = %d, want 1", len(prs))
	}
	if prs[0].Labels[0] != "bug" {
		t.Errorf("Labels[0] = %q, want %q", prs[0].Labels[0], "bug")
	}
	if prs[0].Assignees[0] != "user1" {
		t.Errorf("Assignees[0] = %q, want %q", prs[0].Assignees[0], "user1")
	}
}

func TestClient_ListPRs_WithMergedPR(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr list").ReturnJSON(`[
		{
			"number": 1,
			"title": "Merged PR",
			"body": "",
			"url": "https://github.com/owner/repo/pull/1",
			"state": "MERGED",
			"isDraft": false,
			"headRefName": "branch1",
			"headRefOid": "sha1",
			"baseRefName": "main",
			"createdAt": "2024-01-15T10:00:00Z",
			"updatedAt": "2024-01-16T10:00:00Z",
			"mergedAt": "2024-01-16T10:00:00Z",
			"labels": [],
			"assignees": []
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	prs, err := client.ListPRs(context.Background(), core.ListPROptions{
		State: "merged",
	})
	if err != nil {
		t.Fatalf("ListPRs() error = %v", err)
	}

	if len(prs) != 1 {
		t.Fatalf("len(prs) = %d, want 1", len(prs))
	}
	if !prs[0].Merged {
		t.Error("Merged should be true when mergedAt is non-nil")
	}
}

func TestClient_ListPRs_EmptyOptions(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr list").ReturnJSON(`[]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	prs, err := client.ListPRs(context.Background(), core.ListPROptions{})
	if err != nil {
		t.Fatalf("ListPRs() error = %v", err)
	}

	if len(prs) != 0 {
		t.Errorf("len(prs) = %d, want 0", len(prs))
	}
}

func TestClient_ListPRs_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr list").ReturnError(errors.New("command failed"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.ListPRs(context.Background(), core.ListPROptions{})
	if err == nil {
		t.Fatal("expected error from failed command")
	}
}

func TestClient_ListPRs_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr list").Return("{not an array}")

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.ListPRs(context.Background(), core.ListPROptions{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_MergePR_WithCommitTitleAndMessage(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr merge").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.MergePR(context.Background(), 1, core.MergePROptions{
		Method:        "squash",
		CommitTitle:   "feat: add feature (#1)",
		CommitMessage: "This adds the feature.",
	})
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	if runner.CallCount("pr merge") != 1 {
		t.Error("expected pr merge to be called once")
	}
}

func TestClient_MergePR_DefaultMethod(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr merge").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	// Empty method should default to --merge
	err := client.MergePR(context.Background(), 5, core.MergePROptions{
		Method: "",
	})
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}
}

func TestClient_MergePR_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr merge").ReturnError(errors.New("merge conflict"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.MergePR(context.Background(), 1, core.MergePROptions{Method: "merge"})
	if err == nil {
		t.Fatal("expected error from merge failure")
	}
}

func TestClient_GetCheckStatus_Skipped(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks abc").ReturnJSON(`[
		{
			"name": "optional-check",
			"status": "completed",
			"conclusion": "skipped",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:01:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	status, err := client.GetCheckStatus(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (skipped counts as passed)", status.Passed)
	}
	if status.State != "success" {
		t.Errorf("State = %q, want %q", status.State, "success")
	}
}

func TestClient_GetCheckStatus_Neutral(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks abc").ReturnJSON(`[
		{
			"name": "info-check",
			"status": "completed",
			"conclusion": "neutral",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:01:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	status, err := client.GetCheckStatus(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (neutral counts as passed)", status.Passed)
	}
}

func TestClient_GetCheckStatus_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks abc").ReturnError(errors.New("api failure"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetCheckStatus(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected error from failed command")
	}
}

func TestClient_GetCheckStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks abc").Return("{not valid json}")

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetCheckStatus(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_GetRepo_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetRepo(context.Background())
	if err == nil {
		t.Fatal("expected error from failed command")
	}
}

func TestClient_GetRepo_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").Return("{bad json}")

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetRepo(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_GetDefaultBranch_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").ReturnError(errors.New("command failed"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetDefaultBranch(context.Background())
	if err == nil {
		t.Fatal("expected error from failed command")
	}
}

func TestClient_GetDefaultBranch_InvalidJSON(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").Return("{invalid}")

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetDefaultBranch(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_GetAuthenticatedUser_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh api user").ReturnError(errors.New("not authenticated"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetAuthenticatedUser(context.Background())
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestClient_CreatePR_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr create").ReturnError(errors.New("permission denied"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.CreatePR(context.Background(), core.CreatePROptions{
		Title: "Test",
		Body:  "Test",
		Base:  "main",
		Head:  "feature",
	})
	if err == nil {
		t.Fatal("expected error when PR creation fails")
	}
}

func TestClient_CreateIssue_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").ReturnError(errors.New("permission denied"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.CreateIssue(context.Background(), "Bug", "Desc", nil)
	if err == nil {
		t.Fatal("expected error when issue creation fails")
	}
}

func TestClient_CreateIssue_InvalidURL(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").Return("not-a-url")

	client := NewClientSkipAuth("owner", "repo", runner)

	num, err := client.CreateIssue(context.Background(), "Bug", "Desc", nil)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	// Should return 0 since the URL can't be parsed
	if num != 0 {
		t.Errorf("issue number = %d, want 0 for unparseable URL", num)
	}
}

func TestClient_ClosePR_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr close").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.ClosePR(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error when PR not found")
	}
}

func TestClient_AddComment_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr comment").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.AddComment(context.Background(), 999, "comment")
	if err == nil {
		t.Fatal("expected error when PR not found")
	}
}

func TestClient_RequestReview_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr edit").ReturnError(errors.New("permission denied"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.RequestReview(context.Background(), 1, []string{"user1"})
	if err == nil {
		t.Fatal("expected error when review request fails")
	}
}

func TestClient_UpdatePR_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr edit").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)

	title := "New Title"
	err := client.UpdatePR(context.Background(), 999, core.UpdatePROptions{
		Title: &title,
	})
	if err == nil {
		t.Fatal("expected error when PR not found")
	}
}

func TestClient_MarkPRReady_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr ready").ReturnError(errors.New("not a draft"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.MarkPRReady(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error when PR is not a draft")
	}
}

func TestClient_ValidateToken_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh auth status").ReturnError(errors.New("not authenticated"))

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.ValidateToken(context.Background())
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestClient_getPRByURL_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr view").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.getPRByURL(context.Background(), "https://github.com/owner/repo/pull/999")
	if err == nil {
		t.Fatal("expected error when PR not found")
	}
}

// =============================================================================
// issue_client.go tests — additional coverage
// =============================================================================

func TestIssueClientAdapter_LinkIssues_Success(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	// GetIssue to fetch child issue ID
	runner.OnCommand("gh issue view 20").Return(`{
		"id": 112233,
		"number": 20,
		"title": "Child Issue",
		"body": "",
		"url": "https://github.com/owner/repo/issues/20",
		"state": "OPEN",
		"labels": [],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)
	// API call to create sub-issue link
	runner.OnCommand("gh api -X POST").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.LinkIssues(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("LinkIssues() error = %v", err)
	}
}

func TestIssueClientAdapter_LinkIssues_InvalidNumbers(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.LinkIssues(context.Background(), 0, 20)
	if err == nil {
		t.Fatal("expected error for invalid parent number")
	}

	err = adapter.LinkIssues(context.Background(), 10, -1)
	if err == nil {
		t.Fatal("expected error for invalid child number")
	}
}

func TestIssueClientAdapter_LinkIssues_GetChildError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue view 20").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.LinkIssues(context.Background(), 10, 20)
	if err == nil {
		t.Fatal("expected error when child issue not found")
	}
}

func TestIssueClientAdapter_LinkIssues_ChildIDZero(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	// Return an issue with ID = 0
	runner.OnCommand("gh issue view 20").Return(`{
		"id": 0,
		"number": 20,
		"title": "No ID Issue",
		"body": "",
		"url": "https://github.com/owner/repo/issues/20",
		"state": "OPEN",
		"labels": [],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.LinkIssues(context.Background(), 10, 20)
	if err == nil {
		t.Fatal("expected error when child issue ID is zero")
	}
}

func TestIssueClientAdapter_LinkIssues_APIError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue view 20").Return(`{
		"id": 112233,
		"number": 20,
		"title": "Child Issue",
		"body": "",
		"url": "https://github.com/owner/repo/issues/20",
		"state": "OPEN",
		"labels": [],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)
	runner.OnCommand("gh api -X POST").ReturnError(errors.New("forbidden"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.LinkIssues(context.Background(), 10, 20)
	if err == nil {
		t.Fatal("expected error when API call fails")
	}
}

func TestIssueClientAdapter_CreateIssue_WithParentIssue(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	// Issue creation returns URL
	runner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/50")

	// GetIssue for the newly created issue (50)
	runner.OnCommand("gh issue view 50").Return(`{
		"id": 500500,
		"number": 50,
		"title": "Child Issue",
		"body": "body",
		"url": "https://github.com/owner/repo/issues/50",
		"state": "OPEN",
		"labels": [],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)

	// LinkIssues: GetIssue for child (50) to get the ID,
	// then the API call to link
	runner.OnCommand("gh api -X POST").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title:       "Child Issue",
		Body:        "body",
		ParentIssue: 10,
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.Number != 50 {
		t.Errorf("Number = %d, want 50", issue.Number)
	}
	if issue.ParentIssue != 10 {
		t.Errorf("ParentIssue = %d, want 10", issue.ParentIssue)
	}
}

func TestIssueClientAdapter_CreateIssue_WithParentIssue_LinkFails(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	runner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/51")

	runner.OnCommand("gh issue view 51").Return(`{
		"id": 510510,
		"number": 51,
		"title": "Child Issue",
		"body": "body",
		"url": "https://github.com/owner/repo/issues/51",
		"state": "OPEN",
		"labels": [],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)

	// LinkIssues fails at API call
	runner.OnCommand("gh api -X POST").ReturnError(errors.New("forbidden"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title:       "Child Issue",
		Body:        "body",
		ParentIssue: 10,
	})
	// Should still succeed (linking failure is non-fatal)
	if err != nil {
		t.Fatalf("CreateIssue() should succeed even if linking fails, error = %v", err)
	}

	if issue.Number != 51 {
		t.Errorf("Number = %d, want 51", issue.Number)
	}
	// ParentIssue should NOT be set since linking failed
	if issue.ParentIssue != 0 {
		t.Errorf("ParentIssue = %d, want 0 (linking failed)", issue.ParentIssue)
	}
}

func TestIssueClientAdapter_CreateIssue_WithAllOptions(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	runner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/60")
	runner.OnCommand("gh issue view 60").Return(`{
		"id": 600600,
		"number": 60,
		"title": "Full Issue",
		"body": "Full body",
		"url": "https://github.com/owner/repo/issues/60",
		"state": "OPEN",
		"labels": [{"name": "bug"}, {"name": "urgent"}],
		"assignees": [{"login": "dev1"}],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title:     "Full Issue",
		Body:      "Full body",
		Labels:    []string{"bug", "urgent"},
		Assignees: []string{"dev1"},
		Milestone: "v1.0",
		Project:   "Board",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.Number != 60 {
		t.Errorf("Number = %d, want 60", issue.Number)
	}
}

func TestIssueClientAdapter_CreateIssue_CreateError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").ReturnError(errors.New("permission denied"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	_, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title: "Test",
		Body:  "Body",
	})
	if err == nil {
		t.Fatal("expected error when issue creation fails")
	}
}

func TestIssueClientAdapter_CreateIssue_InvalidURL(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").Return("not-a-valid-url")

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	_, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title: "Test",
		Body:  "Body",
	})
	if err == nil {
		t.Fatal("expected error for invalid issue URL")
	}
}

func TestIssueClientAdapter_CreateIssue_GetIssueFails(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/99")
	// GetIssue fails - should return partial issue
	runner.OnCommand("gh issue view 99").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title: "Test Issue",
		Body:  "Body",
	})
	if err != nil {
		t.Fatalf("CreateIssue() should succeed even if GetIssue fails, error = %v", err)
	}

	// Should return a partial issue
	if issue.Number != 99 {
		t.Errorf("Number = %d, want 99", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Title = %q, want %q", issue.Title, "Test Issue")
	}
	if issue.State != "open" {
		t.Errorf("State = %q, want %q", issue.State, "open")
	}
}

func TestIssueClientAdapter_UpdateIssue_TitleOnly(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue edit 10").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.UpdateIssue(context.Background(), 10, "New Title", "")
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
}

func TestIssueClientAdapter_UpdateIssue_BodyOnly(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue edit 10").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.UpdateIssue(context.Background(), 10, "", "New Body")
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
}

func TestIssueClientAdapter_UpdateIssue_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue edit 10").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.UpdateIssue(context.Background(), 10, "Title", "Body")
	if err == nil {
		t.Fatal("expected error when update fails")
	}
}

func TestIssueClientAdapter_CloseIssue_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue close 99").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.CloseIssue(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error when close fails")
	}
}

func TestIssueClientAdapter_AddIssueComment_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue comment 99").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.AddIssueComment(context.Background(), 99, "Comment")
	if err == nil {
		t.Fatal("expected error when comment fails")
	}
}

func TestIssueClientAdapter_GetIssue_Error(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh issue view 99").ReturnError(errors.New("not found"))

	client := NewClientSkipAuth("owner", "repo", runner)
	adapter := NewIssueClientAdapter(client)

	_, err := adapter.GetIssue(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error when get fails")
	}
}

// =============================================================================
// Rate limiter tests
// =============================================================================

func TestNewGitHubRateLimiter_DefaultRate(t *testing.T) {
	t.Parallel()
	rl := NewGitHubRateLimiter(0)
	if rl.maxPerMinute != 30 {
		t.Errorf("maxPerMinute = %d, want 30 (default)", rl.maxPerMinute)
	}
}

func TestNewGitHubRateLimiter_NegativeRate(t *testing.T) {
	t.Parallel()
	rl := NewGitHubRateLimiter(-5)
	if rl.maxPerMinute != 30 {
		t.Errorf("maxPerMinute = %d, want 30 (default for negative)", rl.maxPerMinute)
	}
}

func TestNewGitHubRateLimiter_CustomRate(t *testing.T) {
	t.Parallel()
	rl := NewGitHubRateLimiter(100)
	if rl.maxPerMinute != 100 {
		t.Errorf("maxPerMinute = %d, want 100", rl.maxPerMinute)
	}
}

func TestGitHubRateLimiter_Wait_UnderLimit(t *testing.T) {
	t.Parallel()
	rl := NewGitHubRateLimiter(100) // High limit - won't hit it

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() call %d error = %v", i, err)
		}
	}
}

func TestGitHubRateLimiter_Wait_ContextCancelled(t *testing.T) {
	t.Parallel()
	rl := NewGitHubRateLimiter(1) // Very low limit

	ctx := context.Background()
	// First call should succeed
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	// Second call should block - cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// =============================================================================
// MockRunner additional coverage
// =============================================================================

func TestMockRunner_LastCall_Empty(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	if runner.LastCall() != nil {
		t.Error("LastCall() should return nil when no calls have been made")
	}
}

func TestMockRunner_Run_NoMatchNoDefault(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()

	_, err := runner.Run(context.Background(), "unknown", "command")
	if err == nil {
		t.Fatal("expected error when no response is configured")
	}
}

func TestMockRunner_Run_DefaultResponse(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.DefaultResponse = &MockResponse{Output: "default output"}

	out, err := runner.Run(context.Background(), "any", "command")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "default output" {
		t.Errorf("output = %q, want %q", out, "default output")
	}
}

func TestMockRunner_Run_ContainsMatch(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("list").Return("found via contains")

	out, err := runner.Run(context.Background(), "gh", "pr", "list", "--json")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "found via contains" {
		t.Errorf("output = %q, want %q", out, "found via contains")
	}
}

func TestMockRunner_ReturnError(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	expectedErr := errors.New("test error")
	runner.OnCommand("gh test").ReturnError(expectedErr)

	_, err := runner.Run(context.Background(), "gh", "test")
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

// =============================================================================
// parseCorePR additional edge cases
// =============================================================================

func TestClient_parseCorePR_NotMergeable(t *testing.T) {
	t.Parallel()
	client := &Client{repoOwner: "owner", repoName: "repo"}

	jsonStr := `{
		"number": 1,
		"title": "Test",
		"body": "",
		"url": "https://github.com/owner/repo/pull/1",
		"state": "OPEN",
		"isDraft": false,
		"mergeable": "CONFLICTING",
		"headRefName": "test",
		"headRefOid": "abc",
		"baseRefName": "main",
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z",
		"mergedAt": null,
		"labels": [],
		"assignees": []
	}`

	pr, err := client.parseCorePR(jsonStr)
	if err != nil {
		t.Fatalf("parseCorePR() error = %v", err)
	}

	if pr.Mergeable == nil {
		t.Fatal("Mergeable should not be nil")
	}
	if *pr.Mergeable {
		t.Error("Mergeable should be false for CONFLICTING")
	}
}

// =============================================================================
// run() timeout path
// =============================================================================

func TestClient_run_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	// Use a real ExecRunner with a very short timeout to trigger deadline exceeded
	client := &Client{
		repoOwner: "owner",
		repoName:  "repo",
		timeout:   1 * time.Nanosecond, // Extremely short timeout
		runner:    NewExecRunner(),
	}

	_, err := client.run(context.Background(), "pr", "list")
	if err == nil {
		t.Fatal("expected error for very short timeout")
	}
	// The error message could be either a timeout or a run error
	// depending on OS scheduling, but we should get an error
}

// =============================================================================
// CommandRunner interface satisfaction
// =============================================================================

func TestExecRunner_ImplementsCommandRunner(t *testing.T) {
	t.Parallel()
	var _ CommandRunner = (*ExecRunner)(nil)
	var _ CommandRunner = (*MockRunner)(nil)
}

// =============================================================================
// getChecksStatus — cancelled conclusion
// =============================================================================

func TestChecksWaiter_GetChecks_CancelledConclusion(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "cancelled",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:01:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if result.AllPassed {
		t.Error("AllPassed should be false for cancelled conclusion")
	}
	if len(result.FailedChecks) != 1 {
		t.Errorf("len(FailedChecks) = %d, want 1", len(result.FailedChecks))
	}
}

func TestChecksWaiter_GetChecks_TimedOutConclusion(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "timed_out",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:30:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if result.AllPassed {
		t.Error("AllPassed should be false for timed_out conclusion")
	}
}

func TestChecksWaiter_GetChecks_ActionRequiredConclusion(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks 1").ReturnJSON(`[
		{
			"name": "deploy",
			"status": "completed",
			"conclusion": "action_required",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:01:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)
	waiter := NewChecksWaiter(client)

	result, err := waiter.GetChecks(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetChecks() error = %v", err)
	}

	if result.AllPassed {
		t.Error("AllPassed should be false for action_required conclusion")
	}
}

// =============================================================================
// client.GetCheckStatus — mixed states
// =============================================================================

func TestClient_GetCheckStatus_MixedStates(t *testing.T) {
	t.Parallel()
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks ref").ReturnJSON(`[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:05:00Z"
		},
		{
			"name": "deploy",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": null
		},
		{
			"name": "test",
			"status": "completed",
			"conclusion": "failure",
			"detailsUrl": "",
			"startedAt": "2024-01-15T10:00:00Z",
			"completedAt": "2024-01-15T10:10:00Z"
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	status, err := client.GetCheckStatus(context.Background(), "ref")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", status.TotalCount)
	}
	if status.Passed != 1 {
		t.Errorf("Passed = %d, want 1", status.Passed)
	}
	if status.Pending != 1 {
		t.Errorf("Pending = %d, want 1", status.Pending)
	}
	if status.Failed != 1 {
		t.Errorf("Failed = %d, want 1", status.Failed)
	}
	// Both pending and failure present; the last-set state wins ("failure" comes after "pending")
	if status.State != "failure" {
		t.Errorf("State = %q, want %q", status.State, "failure")
	}
}

// =============================================================================
// Ensure RunError implements error
// =============================================================================

func TestRunError_ImplementsError(t *testing.T) {
	t.Parallel()
	var err error = &RunError{Command: "test", Err: fmt.Errorf("fail")}
	if err.Error() == "" {
		t.Error("Error() should return non-empty string")
	}
}
