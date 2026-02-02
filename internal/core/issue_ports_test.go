package core

import (
	"testing"
	"time"
)

func TestIssueSet_IssueNumbers(t *testing.T) {
	tests := []struct {
		name     string
		set      IssueSet
		expected []int
	}{
		{
			name:     "empty set",
			set:      IssueSet{},
			expected: []int{},
		},
		{
			name: "main issue only",
			set: IssueSet{
				MainIssue: &Issue{Number: 1},
			},
			expected: []int{1},
		},
		{
			name: "main with sub-issues",
			set: IssueSet{
				MainIssue: &Issue{Number: 1},
				SubIssues: []*Issue{
					{Number: 2},
					{Number: 3},
				},
			},
			expected: []int{1, 2, 3},
		},
		{
			name: "sub-issues only",
			set: IssueSet{
				SubIssues: []*Issue{
					{Number: 5},
					{Number: 6},
				},
			},
			expected: []int{5, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.set.IssueNumbers()
			if len(got) != len(tt.expected) {
				t.Errorf("IssueNumbers() = %v, want %v", got, tt.expected)
				return
			}
			for i, num := range got {
				if num != tt.expected[i] {
					t.Errorf("IssueNumbers()[%d] = %v, want %v", i, num, tt.expected[i])
				}
			}
		})
	}
}

func TestIssueSet_TotalCount(t *testing.T) {
	tests := []struct {
		name     string
		set      IssueSet
		expected int
	}{
		{
			name:     "empty set",
			set:      IssueSet{},
			expected: 0,
		},
		{
			name: "main issue only",
			set: IssueSet{
				MainIssue: &Issue{Number: 1},
			},
			expected: 1,
		},
		{
			name: "main with sub-issues",
			set: IssueSet{
				MainIssue: &Issue{Number: 1},
				SubIssues: []*Issue{
					{Number: 2},
					{Number: 3},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.set.TotalCount(); got != tt.expected {
				t.Errorf("TotalCount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIssueProvider_IsValid(t *testing.T) {
	tests := []struct {
		provider IssueProvider
		valid    bool
	}{
		{IssueProviderGitHub, true},
		{IssueProviderGitLab, true},
		{IssueProvider("invalid"), false},
		{IssueProvider(""), false},
		{IssueProvider("bitbucket"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if got := tt.provider.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestIssue_Fields(t *testing.T) {
	now := time.Now()
	issue := Issue{
		Number:      42,
		Title:       "Test Issue",
		Body:        "This is a test body",
		State:       "open",
		URL:         "https://github.com/owner/repo/issues/42",
		Labels:      []string{"bug", "priority:high"},
		Assignees:   []string{"user1", "user2"},
		ParentIssue: 10,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if issue.Number != 42 {
		t.Errorf("Number = %v, want 42", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Title = %v, want 'Test Issue'", issue.Title)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("Labels length = %v, want 2", len(issue.Labels))
	}
	if issue.ParentIssue != 10 {
		t.Errorf("ParentIssue = %v, want 10", issue.ParentIssue)
	}
}
