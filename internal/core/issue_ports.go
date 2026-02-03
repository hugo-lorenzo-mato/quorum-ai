package core

import (
	"context"
	"time"
)

// =============================================================================
// IssueClient Port (Issue Tracking Integration)
// =============================================================================

// IssueProvider identifies the issue tracking platform.
type IssueProvider string

const (
	// IssueProviderGitHub uses GitHub Issues via the gh CLI.
	IssueProviderGitHub IssueProvider = "github"

	// IssueProviderGitLab uses GitLab Issues via the glab CLI.
	IssueProviderGitLab IssueProvider = "gitlab"
)

// IsValid returns true if the provider is a known value.
func (p IssueProvider) IsValid() bool {
	switch p {
	case IssueProviderGitHub, IssueProviderGitLab:
		return true
	default:
		return false
	}
}

// Issue represents an issue on a tracking platform (GitHub, GitLab).
type Issue struct {
	// ID is the unique database ID for the issue (platform-specific, e.g., GitHub issue ID).
	ID int64

	// Number is the unique issue number within the repository.
	Number int

	// Title is the issue title/summary.
	Title string

	// Body contains the issue description in markdown format.
	Body string

	// State is the issue state: "open" or "closed".
	State string

	// URL is the web URL to view the issue.
	URL string

	// Labels are the labels/tags applied to the issue.
	Labels []string

	// Assignees are the usernames assigned to the issue.
	Assignees []string

	// ParentIssue is the parent issue number for sub-issues (0 = none).
	ParentIssue int

	// CreatedAt is when the issue was created.
	CreatedAt time.Time

	// UpdatedAt is when the issue was last updated.
	UpdatedAt time.Time
}

// CreateIssueOptions configures issue creation.
type CreateIssueOptions struct {
	// Title is the issue title (required).
	Title string

	// Body is the issue body in markdown format (required).
	Body string

	// Labels are labels to apply to the issue.
	Labels []string

	// Assignees are usernames to assign to the issue.
	Assignees []string

	// ParentIssue is the parent issue number to link to (0 = none).
	// For GitHub: creates a task list reference in the parent.
	// For GitLab: creates a related issue link or adds to epic.
	ParentIssue int

	// Milestone is the milestone name or ID (platform-specific).
	Milestone string

	// Project is the project name or ID for GitHub Projects or GitLab boards.
	Project string
}

// IssueSet represents a hierarchical set of issues generated from a workflow.
// It contains a main/parent issue and linked sub-issues for each task.
type IssueSet struct {
	// MainIssue is the parent issue created from the consolidated analysis.
	MainIssue *Issue

	// SubIssues are the child issues created from individual tasks.
	SubIssues []*Issue

	// WorkflowID is the associated Quorum workflow ID.
	WorkflowID string

	// GeneratedAt is when the issue set was created.
	GeneratedAt time.Time
}

// IssueNumbers returns all issue numbers in the set.
func (s *IssueSet) IssueNumbers() []int {
	nums := make([]int, 0, len(s.SubIssues)+1)
	if s.MainIssue != nil {
		nums = append(nums, s.MainIssue.Number)
	}
	for _, sub := range s.SubIssues {
		nums = append(nums, sub.Number)
	}
	return nums
}

// TotalCount returns the total number of issues in the set.
func (s *IssueSet) TotalCount() int {
	count := len(s.SubIssues)
	if s.MainIssue != nil {
		count++
	}
	return count
}

// IssueClient defines the contract for issue tracking system operations.
// Implementations exist for GitHub (via gh CLI) and GitLab (via glab CLI).
type IssueClient interface {
	// CreateIssue creates a new issue and returns the created issue.
	CreateIssue(ctx context.Context, opts CreateIssueOptions) (*Issue, error)

	// UpdateIssue updates an existing issue's title and body.
	UpdateIssue(ctx context.Context, number int, title, body string) error

	// CloseIssue closes an issue by number.
	CloseIssue(ctx context.Context, number int) error

	// AddIssueComment adds a comment to an existing issue.
	AddIssueComment(ctx context.Context, number int, comment string) error

	// GetIssue retrieves an issue by number.
	GetIssue(ctx context.Context, number int) (*Issue, error)

	// LinkIssues creates a parent-child relationship between issues.
	// Implementation is platform-specific:
	// - GitHub: Updates parent body with task list containing child reference
	// - GitLab: Creates "related to" link or adds child to parent's epic
	LinkIssues(ctx context.Context, parent, child int) error
}
