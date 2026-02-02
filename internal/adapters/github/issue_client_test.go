package github

import (
	"context"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestParseIssueNumberFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr bool
	}{
		{
			name:    "valid GitHub issue URL",
			url:     "https://github.com/owner/repo/issues/123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "valid URL with trailing newline",
			url:     "https://github.com/owner/repo/issues/456\n",
			want:    456,
			wantErr: false,
		},
		{
			name:    "URL with large issue number",
			url:     "https://github.com/owner/repo/issues/99999",
			want:    99999,
			wantErr: false,
		},
		{
			name:    "invalid URL - no issue number",
			url:     "https://github.com/owner/repo/issues/",
			wantErr: true,
		},
		{
			name:    "invalid URL - PR instead of issue",
			url:     "https://github.com/owner/repo/pull/123",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueNumberFromURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIssueNumberFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseIssueNumberFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIssueJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    *core.Issue
		wantErr bool
	}{
		{
			name: "valid issue JSON",
			json: `{
				"number": 42,
				"title": "Test Issue",
				"body": "This is the issue body",
				"url": "https://github.com/owner/repo/issues/42",
				"state": "OPEN",
				"labels": [{"name": "bug"}, {"name": "priority:high"}],
				"assignees": [{"login": "user1"}],
				"createdAt": "2024-01-15T10:00:00Z",
				"updatedAt": "2024-01-15T12:00:00Z"
			}`,
			want: &core.Issue{
				Number:    42,
				Title:     "Test Issue",
				Body:      "This is the issue body",
				URL:       "https://github.com/owner/repo/issues/42",
				State:     "open",
				Labels:    []string{"bug", "priority:high"},
				Assignees: []string{"user1"},
			},
			wantErr: false,
		},
		{
			name: "issue with empty labels and assignees",
			json: `{
				"number": 1,
				"title": "Simple Issue",
				"body": "",
				"url": "https://github.com/owner/repo/issues/1",
				"state": "CLOSED",
				"labels": [],
				"assignees": [],
				"createdAt": "2024-01-01T00:00:00Z",
				"updatedAt": "2024-01-01T00:00:00Z"
			}`,
			want: &core.Issue{
				Number:    1,
				Title:     "Simple Issue",
				Body:      "",
				URL:       "https://github.com/owner/repo/issues/1",
				State:     "closed",
				Labels:    []string{},
				Assignees: []string{},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueJSON(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIssueJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Number != tt.want.Number {
				t.Errorf("Number = %v, want %v", got.Number, tt.want.Number)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %v, want %v", got.Title, tt.want.Title)
			}
			if got.Body != tt.want.Body {
				t.Errorf("Body = %v, want %v", got.Body, tt.want.Body)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %v, want %v", got.State, tt.want.State)
			}
			if len(got.Labels) != len(tt.want.Labels) {
				t.Errorf("Labels length = %v, want %v", len(got.Labels), len(tt.want.Labels))
			}
			if len(got.Assignees) != len(tt.want.Assignees) {
				t.Errorf("Assignees length = %v, want %v", len(got.Assignees), len(tt.want.Assignees))
			}
		})
	}
}

func TestIssueClientAdapter_CreateIssue(t *testing.T) {
	mockRunner := NewMockRunner()

	// Setup mock responses
	mockRunner.OnCommand("gh issue create").Return("https://github.com/owner/repo/issues/123\n")
	mockRunner.OnCommand("gh issue view 123").Return(`{
		"number": 123,
		"title": "Test Issue",
		"body": "Test body",
		"url": "https://github.com/owner/repo/issues/123",
		"state": "OPEN",
		"labels": [{"name": "quorum-generated"}],
		"assignees": [],
		"createdAt": "2024-01-15T10:00:00Z",
		"updatedAt": "2024-01-15T10:00:00Z"
	}`)

	client := NewClientSkipAuth("owner", "repo", mockRunner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.CreateIssue(context.Background(), core.CreateIssueOptions{
		Title:  "Test Issue",
		Body:   "Test body",
		Labels: []string{"quorum-generated"},
	})

	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.Number != 123 {
		t.Errorf("Number = %v, want 123", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Title = %v, want 'Test Issue'", issue.Title)
	}
}

func TestIssueClientAdapter_CloseIssue(t *testing.T) {
	mockRunner := NewMockRunner()
	mockRunner.OnCommand("gh issue close 42").Return("")

	client := NewClientSkipAuth("owner", "repo", mockRunner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.CloseIssue(context.Background(), 42)
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}

	// Verify the command was called
	if mockRunner.CallCount("issue close 42") == 0 {
		t.Error("Expected issue close command to be called")
	}
}

func TestIssueClientAdapter_AddIssueComment(t *testing.T) {
	mockRunner := NewMockRunner()
	mockRunner.OnCommand("gh issue comment 42").Return("")

	client := NewClientSkipAuth("owner", "repo", mockRunner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.AddIssueComment(context.Background(), 42, "Test comment")
	if err != nil {
		t.Fatalf("AddIssueComment() error = %v", err)
	}

	// Verify the command was called
	if mockRunner.CallCount("issue comment 42") == 0 {
		t.Error("Expected issue comment command to be called")
	}
}

func TestIssueClientAdapter_GetIssue(t *testing.T) {
	mockRunner := NewMockRunner()
	mockRunner.OnCommand("gh issue view 42").Return(`{
		"number": 42,
		"title": "Existing Issue",
		"body": "Issue body content",
		"url": "https://github.com/owner/repo/issues/42",
		"state": "OPEN",
		"labels": [{"name": "bug"}],
		"assignees": [{"login": "developer"}],
		"createdAt": "2024-01-10T08:00:00Z",
		"updatedAt": "2024-01-12T15:30:00Z"
	}`)

	client := NewClientSkipAuth("owner", "repo", mockRunner)
	adapter := NewIssueClientAdapter(client)

	issue, err := adapter.GetIssue(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if issue.Number != 42 {
		t.Errorf("Number = %v, want 42", issue.Number)
	}
	if issue.Title != "Existing Issue" {
		t.Errorf("Title = %v, want 'Existing Issue'", issue.Title)
	}
	if issue.State != "open" {
		t.Errorf("State = %v, want 'open'", issue.State)
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != "bug" {
		t.Errorf("Labels = %v, want ['bug']", issue.Labels)
	}
}

func TestIssueClientAdapter_UpdateIssue(t *testing.T) {
	mockRunner := NewMockRunner()
	mockRunner.OnCommand("gh issue edit 42").Return("")

	client := NewClientSkipAuth("owner", "repo", mockRunner)
	adapter := NewIssueClientAdapter(client)

	err := adapter.UpdateIssue(context.Background(), 42, "Updated Title", "Updated Body")
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}

	// Verify the command was called
	if mockRunner.CallCount("issue edit 42") == 0 {
		t.Error("Expected issue edit command to be called")
	}
}
