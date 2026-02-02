package github

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Compile-time interface conformance check.
var _ core.IssueClient = (*IssueClientAdapter)(nil)

// IssueClientAdapter wraps the GitHub Client to implement core.IssueClient.
type IssueClientAdapter struct {
	client *Client
}

// NewIssueClientAdapter creates a new IssueClient adapter from an existing GitHub Client.
func NewIssueClientAdapter(client *Client) *IssueClientAdapter {
	return &IssueClientAdapter{client: client}
}

// NewIssueClient creates a new IssueClient from owner/repo.
func NewIssueClient(owner, repo string) (*IssueClientAdapter, error) {
	client, err := NewClient(owner, repo)
	if err != nil {
		return nil, err
	}
	return NewIssueClientAdapter(client), nil
}

// NewIssueClientFromRepo creates a new IssueClient detecting repo from git remote.
func NewIssueClientFromRepo() (*IssueClientAdapter, error) {
	client, err := NewClientFromRepo()
	if err != nil {
		return nil, err
	}
	return NewIssueClientAdapter(client), nil
}

// CreateIssue creates a new issue and returns the created issue.
func (a *IssueClientAdapter) CreateIssue(ctx context.Context, opts core.CreateIssueOptions) (*core.Issue, error) {
	args := []string{"issue", "create",
		"--repo", a.client.Repo(),
		"--title", opts.Title,
		"--body", opts.Body,
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	for _, assignee := range opts.Assignees {
		args = append(args, "--assignee", assignee)
	}

	if opts.Milestone != "" {
		args = append(args, "--milestone", opts.Milestone)
	}

	if opts.Project != "" {
		args = append(args, "--project", opts.Project)
	}

	output, err := a.client.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("creating issue: %w", err)
	}

	// Parse issue number from URL (output is the issue URL)
	// Format: https://github.com/owner/repo/issues/123
	issueNum, err := parseIssueNumberFromURL(strings.TrimSpace(output))
	if err != nil {
		return nil, fmt.Errorf("parsing issue URL: %w", err)
	}

	// Fetch full issue details
	issue, err := a.GetIssue(ctx, issueNum)
	if err != nil {
		// Return partial issue if fetch fails
		return &core.Issue{
			Number:    issueNum,
			Title:     opts.Title,
			Body:      opts.Body,
			State:     "open",
			URL:       strings.TrimSpace(output),
			Labels:    opts.Labels,
			Assignees: opts.Assignees,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	// Link to parent if specified
	if opts.ParentIssue > 0 {
		if err := a.LinkIssues(ctx, opts.ParentIssue, issueNum); err != nil {
			// Log but don't fail - issue was created successfully
			// The caller can handle linking separately if needed
		}
		issue.ParentIssue = opts.ParentIssue
	}

	return issue, nil
}

// UpdateIssue updates an existing issue's title and body.
func (a *IssueClientAdapter) UpdateIssue(ctx context.Context, number int, title, body string) error {
	args := []string{"issue", "edit", strconv.Itoa(number),
		"--repo", a.client.Repo(),
	}

	if title != "" {
		args = append(args, "--title", title)
	}

	if body != "" {
		args = append(args, "--body", body)
	}

	_, err := a.client.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("updating issue #%d: %w", number, err)
	}

	return nil
}

// CloseIssue closes an issue by number.
func (a *IssueClientAdapter) CloseIssue(ctx context.Context, number int) error {
	_, err := a.client.run(ctx, "issue", "close", strconv.Itoa(number),
		"--repo", a.client.Repo())
	if err != nil {
		return fmt.Errorf("closing issue #%d: %w", number, err)
	}

	return nil
}

// AddIssueComment adds a comment to an existing issue.
func (a *IssueClientAdapter) AddIssueComment(ctx context.Context, number int, comment string) error {
	_, err := a.client.run(ctx, "issue", "comment", strconv.Itoa(number),
		"--repo", a.client.Repo(),
		"--body", comment)
	if err != nil {
		return fmt.Errorf("adding comment to issue #%d: %w", number, err)
	}

	return nil
}

// GetIssue retrieves an issue by number.
func (a *IssueClientAdapter) GetIssue(ctx context.Context, number int) (*core.Issue, error) {
	output, err := a.client.run(ctx, "issue", "view", strconv.Itoa(number),
		"--repo", a.client.Repo(),
		"--json", "number,title,body,url,state,labels,assignees,createdAt,updatedAt")
	if err != nil {
		return nil, fmt.Errorf("getting issue #%d: %w", number, err)
	}

	return parseIssueJSON(output)
}

// LinkIssues creates a parent-child relationship between issues.
// For GitHub: Updates parent body with a task list containing child reference.
func (a *IssueClientAdapter) LinkIssues(ctx context.Context, parent, child int) error {
	// Get parent issue
	parentIssue, err := a.GetIssue(ctx, parent)
	if err != nil {
		return fmt.Errorf("getting parent issue #%d: %w", parent, err)
	}

	// Create task list item for child
	childLink := fmt.Sprintf("- [ ] #%d", child)

	// Check if parent body already has a task list section
	var newBody string
	if strings.Contains(parentIssue.Body, "## Sub-Issues") ||
		strings.Contains(parentIssue.Body, "## Tasks") ||
		strings.Contains(parentIssue.Body, "## Related Issues") {
		// Append to existing section
		newBody = parentIssue.Body + "\n" + childLink
	} else {
		// Create new section
		if parentIssue.Body != "" {
			newBody = parentIssue.Body + "\n\n## Sub-Issues\n\n" + childLink
		} else {
			newBody = "## Sub-Issues\n\n" + childLink
		}
	}

	// Update parent issue body
	return a.UpdateIssue(ctx, parent, "", newBody)
}

// parseIssueJSON parses GitHub issue JSON output to core.Issue.
func parseIssueJSON(output string) (*core.Issue, error) {
	var data struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		URL       string    `json:"url"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parsing issue JSON: %w", err)
	}

	labels := make([]string, len(data.Labels))
	for i, l := range data.Labels {
		labels[i] = l.Name
	}

	assignees := make([]string, len(data.Assignees))
	for i, a := range data.Assignees {
		assignees[i] = a.Login
	}

	return &core.Issue{
		Number:    data.Number,
		Title:     data.Title,
		Body:      data.Body,
		State:     strings.ToLower(data.State),
		URL:       data.URL,
		Labels:    labels,
		Assignees: assignees,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// parseIssueNumberFromURL extracts issue number from GitHub issue URL.
func parseIssueNumberFromURL(url string) (int, error) {
	// Match pattern: https://github.com/owner/repo/issues/123
	re := regexp.MustCompile(`/issues/(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid issue URL format: %s", url)
	}

	return strconv.Atoi(matches[1])
}
