package github

import (
	"testing"
	"time"
)

func TestClient_parseCorePR(t *testing.T) {
	client := &Client{
		repoOwner: "testowner",
		repoName:  "testrepo",
	}

	tests := []struct {
		name    string
		json    string
		wantNum int
		wantErr bool
	}{
		{
			name: "valid PR with all fields",
			json: `{
				"number": 123,
				"title": "Test PR Title",
				"body": "Test body content",
				"url": "https://github.com/testowner/testrepo/pull/123",
				"state": "OPEN",
				"isDraft": false,
				"mergeable": "MERGEABLE",
				"headRefName": "feature-branch",
				"headRefOid": "abc123def456",
				"baseRefName": "main",
				"createdAt": "2024-01-15T10:00:00Z",
				"updatedAt": "2024-01-16T12:00:00Z",
				"mergedAt": null,
				"labels": [{"name": "bug"}, {"name": "enhancement"}],
				"assignees": [{"login": "user1"}, {"login": "user2"}]
			}`,
			wantNum: 123,
			wantErr: false,
		},
		{
			name: "merged PR",
			json: `{
				"number": 456,
				"title": "Merged PR",
				"body": "",
				"url": "https://github.com/testowner/testrepo/pull/456",
				"state": "MERGED",
				"isDraft": false,
				"mergeable": "",
				"headRefName": "fix-branch",
				"headRefOid": "def789",
				"baseRefName": "main",
				"createdAt": "2024-01-10T10:00:00Z",
				"updatedAt": "2024-01-11T12:00:00Z",
				"mergedAt": "2024-01-11T12:00:00Z",
				"labels": [],
				"assignees": []
			}`,
			wantNum: 456,
			wantErr: false,
		},
		{
			name: "draft PR",
			json: `{
				"number": 789,
				"title": "Draft PR",
				"body": "Work in progress",
				"url": "https://github.com/testowner/testrepo/pull/789",
				"state": "OPEN",
				"isDraft": true,
				"mergeable": "UNKNOWN",
				"headRefName": "wip-branch",
				"headRefOid": "wip123",
				"baseRefName": "develop",
				"createdAt": "2024-01-20T10:00:00Z",
				"updatedAt": "2024-01-20T10:00:00Z",
				"mergedAt": null,
				"labels": [{"name": "wip"}],
				"assignees": []
			}`,
			wantNum: 789,
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantNum: 0,
			wantErr: true,
		},
		{
			name:    "empty json",
			json:    `{}`,
			wantNum: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := client.parseCorePR(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCorePR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && pr.Number != tt.wantNum {
				t.Errorf("parseCorePR() Number = %d, want %d", pr.Number, tt.wantNum)
			}
		})
	}
}

func TestClient_parseCorePR_FieldMapping(t *testing.T) {
	client := &Client{
		repoOwner: "owner",
		repoName:  "repo",
	}

	json := `{
		"number": 42,
		"title": "Feature: Add new API",
		"body": "This PR adds a new API endpoint",
		"url": "https://github.com/owner/repo/pull/42",
		"state": "OPEN",
		"isDraft": true,
		"mergeable": "MERGEABLE",
		"headRefName": "feature/api",
		"headRefOid": "sha123456",
		"baseRefName": "main",
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-16T12:00:00Z",
		"mergedAt": null,
		"labels": [{"name": "feature"}, {"name": "api"}],
		"assignees": [{"login": "developer1"}]
	}`

	pr, err := client.parseCorePR(json)
	if err != nil {
		t.Fatalf("parseCorePR() error = %v", err)
	}

	// Verify all fields
	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Feature: Add new API" {
		t.Errorf("Title = %q, want %q", pr.Title, "Feature: Add new API")
	}
	if pr.Body != "This PR adds a new API endpoint" {
		t.Errorf("Body = %q, want %q", pr.Body, "This PR adds a new API endpoint")
	}
	if pr.State != "open" {
		t.Errorf("State = %q, want %q", pr.State, "open")
	}
	if !pr.Draft {
		t.Error("Draft should be true")
	}
	if pr.Mergeable == nil || !*pr.Mergeable {
		t.Error("Mergeable should be true")
	}
	if pr.Head.Ref != "feature/api" {
		t.Errorf("Head.Ref = %q, want %q", pr.Head.Ref, "feature/api")
	}
	if pr.Head.SHA != "sha123456" {
		t.Errorf("Head.SHA = %q, want %q", pr.Head.SHA, "sha123456")
	}
	if pr.Base.Ref != "main" {
		t.Errorf("Base.Ref = %q, want %q", pr.Base.Ref, "main")
	}
	if len(pr.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(pr.Labels))
	}
	if len(pr.Assignees) != 1 {
		t.Errorf("Assignees count = %d, want 1", len(pr.Assignees))
	}
	if pr.Merged {
		t.Error("Merged should be false")
	}
}

func TestClient_parseCorePR_MergeableStates(t *testing.T) {
	client := &Client{
		repoOwner: "owner",
		repoName:  "repo",
	}

	tests := []struct {
		name          string
		mergeable     string
		wantMergeable *bool
	}{
		{"mergeable", "MERGEABLE", boolPtr(true)},
		{"conflicting", "CONFLICTING", boolPtr(false)},
		{"unknown", "UNKNOWN", boolPtr(false)},
		{"empty", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json := `{
				"number": 1,
				"title": "Test",
				"body": "",
				"url": "https://github.com/owner/repo/pull/1",
				"state": "OPEN",
				"isDraft": false,
				"mergeable": "` + tt.mergeable + `",
				"headRefName": "test",
				"headRefOid": "abc",
				"baseRefName": "main",
				"createdAt": "2024-01-15T10:00:00Z",
				"updatedAt": "2024-01-15T10:00:00Z",
				"mergedAt": null,
				"labels": [],
				"assignees": []
			}`

			pr, err := client.parseCorePR(json)
			if err != nil {
				t.Fatalf("parseCorePR() error = %v", err)
			}

			if tt.wantMergeable == nil {
				if pr.Mergeable != nil {
					t.Errorf("Mergeable = %v, want nil", *pr.Mergeable)
				}
			} else {
				if pr.Mergeable == nil {
					t.Error("Mergeable is nil, want non-nil")
				} else if *pr.Mergeable != *tt.wantMergeable {
					t.Errorf("Mergeable = %v, want %v", *pr.Mergeable, *tt.wantMergeable)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestClient_Repo(t *testing.T) {
	client := &Client{
		repoOwner: "myowner",
		repoName:  "myrepo",
	}

	if got := client.Repo(); got != "myowner/myrepo" {
		t.Errorf("Repo() = %q, want %q", got, "myowner/myrepo")
	}
}

func TestClient_Owner(t *testing.T) {
	client := &Client{
		repoOwner: "testowner",
		repoName:  "testrepo",
	}

	if got := client.Owner(); got != "testowner" {
		t.Errorf("Owner() = %q, want %q", got, "testowner")
	}
}

func TestClient_Name(t *testing.T) {
	client := &Client{
		repoOwner: "testowner",
		repoName:  "testrepo",
	}

	if got := client.Name(); got != "testrepo" {
		t.Errorf("Name() = %q, want %q", got, "testrepo")
	}
}

func TestClient_WithTimeout(t *testing.T) {
	client := &Client{
		repoOwner: "owner",
		repoName:  "repo",
		timeout:   60 * time.Second,
	}

	newTimeout := 120 * time.Second
	result := client.WithTimeout(newTimeout)

	if result != client {
		t.Error("WithTimeout() should return the same client instance")
	}
	if client.timeout != newTimeout {
		t.Errorf("timeout = %v, want %v", client.timeout, newTimeout)
	}
}

func TestPullRequestStruct(t *testing.T) {
	now := time.Now()
	pr := PullRequest{
		Number:    100,
		Title:     "Test PR",
		Body:      "Description",
		URL:       "https://github.com/owner/repo/pull/100",
		State:     "OPEN",
		Draft:     true,
		Mergeable: "MERGEABLE",
		HeadRef:   "feature",
		BaseRef:   "main",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if pr.Number != 100 {
		t.Errorf("Number = %d, want 100", pr.Number)
	}
	if pr.State != "OPEN" {
		t.Errorf("State = %q, want OPEN", pr.State)
	}
	if !pr.Draft {
		t.Error("Draft should be true")
	}
}
