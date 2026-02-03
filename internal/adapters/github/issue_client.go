package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	client      *Client
	rateLimiter *GitHubRateLimiter
}

// GitHubRateLimiter implements a simple rate limiter for GitHub API calls.
// GitHub allows 5000 requests/hour for authenticated users, but we use conservative limits.
type GitHubRateLimiter struct {
	maxPerMinute int
	calls        []time.Time
}

// NewGitHubRateLimiter creates a rate limiter with the specified max calls per minute.
func NewGitHubRateLimiter(maxPerMinute int) *GitHubRateLimiter {
	if maxPerMinute <= 0 {
		maxPerMinute = 30 // Conservative default: 30/min = 1800/hour
	}
	return &GitHubRateLimiter{
		maxPerMinute: maxPerMinute,
		calls:        make([]time.Time, 0),
	}
}

// Wait blocks until a request is allowed under the rate limit.
func (r *GitHubRateLimiter) Wait(ctx context.Context) error {
	for {
		// Clean up old calls (older than 1 minute)
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		newCalls := make([]time.Time, 0, len(r.calls))
		for _, t := range r.calls {
			if t.After(cutoff) {
				newCalls = append(newCalls, t)
			}
		}
		r.calls = newCalls

		// Check if we can make a call
		if len(r.calls) < r.maxPerMinute {
			r.calls = append(r.calls, now)
			return nil
		}

		// Calculate wait time
		oldestCall := r.calls[0]
		waitTime := time.Minute - now.Sub(oldestCall) + 100*time.Millisecond // Add small buffer

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again
		}
	}
}

// NewIssueClientAdapter creates a new IssueClient adapter from an existing GitHub Client.
func NewIssueClientAdapter(client *Client) *IssueClientAdapter {
	return &IssueClientAdapter{
		client:      client,
		rateLimiter: NewGitHubRateLimiter(30), // 30 requests/minute default
	}
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
	// Apply rate limiting before making the API call
	if a.rateLimiter != nil {
		if err := a.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

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
		issue = &core.Issue{
			Number:    issueNum,
			Title:     opts.Title,
			Body:      opts.Body,
			State:     "open",
			URL:       strings.TrimSpace(output),
			Labels:    opts.Labels,
			Assignees: opts.Assignees,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	// Link to parent if specified
	if opts.ParentIssue > 0 {
		if err := a.LinkIssues(ctx, opts.ParentIssue, issueNum); err != nil {
			// Log the error but don't fail - issue was created successfully
			// IMPORTANT: Do NOT set ParentIssue if linking failed
			slog.Warn("failed to link issue to parent",
				"child_issue", issueNum,
				"parent_issue", opts.ParentIssue,
				"error", err)
		} else {
			// Only set ParentIssue if linking succeeded
			issue.ParentIssue = opts.ParentIssue
		}
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
		"--json", "id,number,title,body,url,state,labels,assignees,createdAt,updatedAt")
	if err != nil {
		return nil, fmt.Errorf("getting issue #%d: %w", number, err)
	}

	return parseIssueJSON(output)
}

// LinkIssues creates a parent-child relationship between issues.
// For GitHub: Creates a sub-issue relationship via the REST API.
func (a *IssueClientAdapter) LinkIssues(ctx context.Context, parent, child int) error {
	if parent <= 0 || child <= 0 {
		return fmt.Errorf("invalid issue numbers: parent=%d child=%d", parent, child)
	}

	childIssue, err := a.GetIssue(ctx, child)
	if err != nil {
		return fmt.Errorf("getting child issue #%d: %w", child, err)
	}
	if childIssue == nil || childIssue.ID == 0 {
		return fmt.Errorf("child issue ID not found for #%d", child)
	}

	endpoint := fmt.Sprintf("/repos/%s/issues/%d/sub_issues", a.client.Repo(), parent)
	_, err = a.client.run(ctx, "api", "-X", "POST", endpoint, "-f", fmt.Sprintf("sub_issue_id=%d", childIssue.ID))
	if err != nil {
		return fmt.Errorf("creating sub-issue link (parent %d, child %d): %w", parent, child, err)
	}

	return nil
}

// parseIssueJSON parses GitHub issue JSON output to core.Issue.
func parseIssueJSON(output string) (*core.Issue, error) {
	var data struct {
		ID        int64     `json:"id"`
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
		ID:        data.ID,
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
