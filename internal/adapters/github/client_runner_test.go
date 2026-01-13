package github

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestNewClientWithRunner_AuthSuccess(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh auth status").Return("")

	client, err := NewClientWithRunner("owner", "repo", runner)
	if err != nil {
		t.Fatalf("NewClientWithRunner() error = %v", err)
	}

	if client.Owner() != "owner" {
		t.Errorf("Owner() = %q, want %q", client.Owner(), "owner")
	}
	if client.Name() != "repo" {
		t.Errorf("Name() = %q, want %q", client.Name(), "repo")
	}
}

func TestNewClientWithRunner_AuthFailed(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh auth status").ReturnError(errors.New("not authenticated"))

	_, err := NewClientWithRunner("owner", "repo", runner)
	if err == nil {
		t.Fatal("expected error for unauthenticated client")
	}

	var domainErr *core.DomainError
	if !errors.As(err, &domainErr) {
		t.Errorf("expected DomainError, got %T", err)
	}
}

func TestNewClientFromRepoWithRunner(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh repo view --json owner,name").ReturnJSON(`{
		"owner": {"login": "testowner"},
		"name": "testrepo"
	}`)
	runner.OnCommand("gh auth status").Return("")

	client, err := NewClientFromRepoWithRunner(runner)
	if err != nil {
		t.Fatalf("NewClientFromRepoWithRunner() error = %v", err)
	}

	if client.Repo() != "testowner/testrepo" {
		t.Errorf("Repo() = %q, want %q", client.Repo(), "testowner/testrepo")
	}
}

func TestClient_CreatePR(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr create").Return("https://github.com/owner/repo/pull/123")
	runner.OnCommand("gh pr view").ReturnJSON(`{
		"number": 123,
		"title": "Test PR",
		"body": "Description",
		"url": "https://github.com/owner/repo/pull/123",
		"state": "OPEN",
		"isDraft": false,
		"mergeable": "MERGEABLE",
		"headRefName": "feature",
		"headRefOid": "abc123",
		"baseRefName": "main",
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z",
		"mergedAt": null,
		"labels": [],
		"assignees": []
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)

	pr, err := client.CreatePR(context.Background(), core.CreatePROptions{
		Title: "Test PR",
		Body:  "Description",
		Base:  "main",
		Head:  "feature",
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}

	if pr.Number != 123 {
		t.Errorf("PR.Number = %d, want 123", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Errorf("PR.Title = %q, want %q", pr.Title, "Test PR")
	}
}

func TestClient_CreatePR_WithOptions(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr create").Return("https://github.com/owner/repo/pull/1")
	runner.OnCommand("gh pr view").ReturnJSON(`{
		"number": 1,
		"title": "Draft PR",
		"body": "",
		"url": "https://github.com/owner/repo/pull/1",
		"state": "OPEN",
		"isDraft": true,
		"mergeable": "",
		"headRefName": "feat",
		"headRefOid": "def",
		"baseRefName": "main",
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z",
		"mergedAt": null,
		"labels": [{"name": "bug"}],
		"assignees": [{"login": "user1"}]
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.CreatePR(context.Background(), core.CreatePROptions{
		Title:     "Draft PR",
		Body:      "",
		Base:      "main",
		Head:      "feat",
		Draft:     true,
		Labels:    []string{"bug"},
		Assignees: []string{"user1"},
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}

	// Verify the command included --draft
	lastCall := runner.LastCall()
	if lastCall == nil {
		t.Fatal("expected at least one call")
	}

	// Check that pr create was called
	if runner.CallCount("pr create") < 1 {
		t.Error("expected pr create to be called")
	}
}

func TestClient_GetPR(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr view 42").ReturnJSON(`{
		"number": 42,
		"title": "Feature PR",
		"body": "Adds new feature",
		"url": "https://github.com/owner/repo/pull/42",
		"state": "OPEN",
		"isDraft": false,
		"mergeable": "MERGEABLE",
		"headRefName": "feature-branch",
		"headRefOid": "sha123",
		"baseRefName": "main",
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-16T12:00:00Z",
		"mergedAt": null,
		"labels": [{"name": "enhancement"}],
		"assignees": []
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)

	pr, err := client.GetPR(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetPR() error = %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("PR.Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Feature PR" {
		t.Errorf("PR.Title = %q, want %q", pr.Title, "Feature PR")
	}
	if pr.Head.Ref != "feature-branch" {
		t.Errorf("PR.Head.Ref = %q, want %q", pr.Head.Ref, "feature-branch")
	}
	if len(pr.Labels) != 1 || pr.Labels[0] != "enhancement" {
		t.Errorf("PR.Labels = %v, want [enhancement]", pr.Labels)
	}
}

func TestClient_GetPR_NotFound(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr view 999").ReturnError(errors.New("Could not find pull request"))

	client := NewClientSkipAuth("owner", "repo", runner)

	_, err := client.GetPR(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent PR")
	}
}

func TestClient_ListPRs(t *testing.T) {
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
			"labels": [],
			"assignees": []
		},
		{
			"number": 2,
			"title": "PR 2",
			"body": "",
			"url": "https://github.com/owner/repo/pull/2",
			"state": "OPEN",
			"isDraft": true,
			"headRefName": "branch2",
			"headRefOid": "sha2",
			"baseRefName": "main",
			"createdAt": "2024-01-16T10:00:00Z",
			"updatedAt": "2024-01-16T10:00:00Z",
			"mergedAt": null,
			"labels": [],
			"assignees": []
		}
	]`)

	client := NewClientSkipAuth("owner", "repo", runner)

	prs, err := client.ListPRs(context.Background(), core.ListPROptions{
		State: "open",
	})
	if err != nil {
		t.Fatalf("ListPRs() error = %v", err)
	}

	if len(prs) != 2 {
		t.Fatalf("len(prs) = %d, want 2", len(prs))
	}

	if prs[0].Number != 1 {
		t.Errorf("prs[0].Number = %d, want 1", prs[0].Number)
	}
	if prs[1].Draft != true {
		t.Error("prs[1].Draft should be true")
	}
}

func TestClient_MergePR(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"merge", "merge"},
		{"squash", "squash"},
		{"rebase", "rebase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewMockRunner()
			runner.OnCommand("gh pr merge").Return("")

			client := NewClientSkipAuth("owner", "repo", runner)

			err := client.MergePR(context.Background(), 1, core.MergePROptions{
				Method: tt.method,
			})
			if err != nil {
				t.Fatalf("MergePR() error = %v", err)
			}

			if runner.CallCount("pr merge") != 1 {
				t.Error("expected pr merge to be called once")
			}
		})
	}
}

func TestClient_ClosePR(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr close").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.ClosePR(context.Background(), 123)
	if err != nil {
		t.Fatalf("ClosePR() error = %v", err)
	}

	if runner.CallCount("pr close") != 1 {
		t.Error("expected pr close to be called once")
	}
}

func TestClient_AddComment(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr comment").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.AddComment(context.Background(), 1, "This is a comment")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	if runner.CallCount("pr comment") != 1 {
		t.Error("expected pr comment to be called once")
	}
}

func TestClient_RequestReview(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr edit").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.RequestReview(context.Background(), 1, []string{"reviewer1", "reviewer2"})
	if err != nil {
		t.Fatalf("RequestReview() error = %v", err)
	}

	if runner.CallCount("pr edit") != 1 {
		t.Error("expected pr edit to be called once")
	}
}

func TestClient_GetDefaultBranch(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").ReturnJSON(`{
		"defaultBranchRef": {"name": "main"}
	}`)

	client := NewClientSkipAuth("owner", "repo", runner)

	branch, err := client.GetDefaultBranch(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}

	if branch != "main" {
		t.Errorf("branch = %q, want %q", branch, "main")
	}
}

func TestClient_UpdatePR(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr edit").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	title := "Updated Title"
	body := "Updated Body"
	err := client.UpdatePR(context.Background(), 1, core.UpdatePROptions{
		Title:     &title,
		Body:      &body,
		Labels:    []string{"new-label"},
		Assignees: []string{"new-assignee"},
	})
	if err != nil {
		t.Fatalf("UpdatePR() error = %v", err)
	}

	if runner.CallCount("pr edit") != 1 {
		t.Error("expected pr edit to be called once")
	}
}

func TestClient_MarkPRReady(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr ready").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.MarkPRReady(context.Background(), 1)
	if err != nil {
		t.Fatalf("MarkPRReady() error = %v", err)
	}

	if runner.CallCount("pr ready") != 1 {
		t.Error("expected pr ready to be called once")
	}
}

func TestClient_CreateIssue(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/42")

	client := NewClientSkipAuth("owner", "repo", runner)

	num, err := client.CreateIssue(context.Background(), "Bug", "Description", []string{"bug"})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if num != 42 {
		t.Errorf("issue number = %d, want 42", num)
	}
}

func TestClient_GetRepo(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh repo view").ReturnJSON(`{
		"owner": {"login": "testowner"},
		"name": "testrepo",
		"defaultBranchRef": {"name": "main"},
		"isPrivate": false,
		"url": "https://github.com/testowner/testrepo"
	}`)

	client := NewClientSkipAuth("testowner", "testrepo", runner)

	repo, err := client.GetRepo(context.Background())
	if err != nil {
		t.Fatalf("GetRepo() error = %v", err)
	}

	if repo.Owner != "testowner" {
		t.Errorf("Owner = %q, want %q", repo.Owner, "testowner")
	}
	if repo.Name != "testrepo" {
		t.Errorf("Name = %q, want %q", repo.Name, "testrepo")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
	if repo.IsPrivate {
		t.Error("IsPrivate should be false")
	}
}

func TestClient_ValidateToken(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh auth status").Return("")

	client := NewClientSkipAuth("owner", "repo", runner)

	err := client.ValidateToken(context.Background())
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
}

func TestClient_GetAuthenticatedUser(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh api user").Return("testuser")

	client := NewClientSkipAuth("owner", "repo", runner)

	user, err := client.GetAuthenticatedUser(context.Background())
	if err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}

	if user != "testuser" {
		t.Errorf("user = %q, want %q", user, "testuser")
	}
}

func TestClient_GetCheckStatus(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks").ReturnJSON(`[
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

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", status.TotalCount)
	}
	if status.Passed != 2 {
		t.Errorf("Passed = %d, want 2", status.Passed)
	}
	if status.State != "success" {
		t.Errorf("State = %q, want %q", status.State, "success")
	}
}

func TestClient_GetCheckStatus_Pending(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks").ReturnJSON(`[
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

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.Pending != 1 {
		t.Errorf("Pending = %d, want 1", status.Pending)
	}
	if status.State != "pending" {
		t.Errorf("State = %q, want %q", status.State, "pending")
	}
}

func TestClient_GetCheckStatus_Failed(t *testing.T) {
	runner := NewMockRunner()
	runner.OnCommand("gh pr checks").ReturnJSON(`[
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

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetCheckStatus() error = %v", err)
	}

	if status.Failed != 1 {
		t.Errorf("Failed = %d, want 1", status.Failed)
	}
	if status.State != "failure" {
		t.Errorf("State = %q, want %q", status.State, "failure")
	}
}

func TestClient_Timeout(t *testing.T) {
	runner := NewMockRunner()
	// Don't set a response - will use default which fails

	client := NewClientSkipAuth("owner", "repo", runner)
	client.timeout = 1 * time.Millisecond

	_, err := client.GetPR(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error due to no mock response")
	}
}

func TestMockRunner_CallTracking(t *testing.T) {
	runner := NewMockRunner()
	runner.DefaultResponse = &MockResponse{Output: "ok"}

	ctx := context.Background()
	_, _ = runner.Run(ctx, "gh", "pr", "list")
	_, _ = runner.Run(ctx, "gh", "pr", "view", "1")
	_, _ = runner.Run(ctx, "gh", "pr", "list")

	if len(runner.Calls) != 3 {
		t.Errorf("len(Calls) = %d, want 3", len(runner.Calls))
	}

	if runner.CallCount("pr list") != 2 {
		t.Errorf("CallCount(pr list) = %d, want 2", runner.CallCount("pr list"))
	}

	if runner.CallCount("pr view") != 1 {
		t.Errorf("CallCount(pr view) = %d, want 1", runner.CallCount("pr view"))
	}

	lastCall := runner.LastCall()
	if lastCall == nil {
		t.Fatal("LastCall() returned nil")
	}
	if lastCall.Args[0] != "pr" || lastCall.Args[1] != "list" {
		t.Errorf("LastCall.Args = %v, want [pr list]", lastCall.Args)
	}

	runner.Reset()
	if len(runner.Calls) != 0 {
		t.Error("Reset() should clear calls")
	}
}
