package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Compile-time interface conformance check.
var _ core.GitHubClient = (*Client)(nil)

// Client wraps GitHub CLI operations.
type Client struct {
	repoOwner string
	repoName  string
	timeout   time.Duration
}

// NewClient creates a new GitHub client.
func NewClient(owner, repo string) (*Client, error) {
	client := &Client{
		repoOwner: owner,
		repoName:  repo,
		timeout:   60 * time.Second,
	}

	// Verify gh is installed and authenticated
	if err := client.verifyAuth(); err != nil {
		return nil, err
	}

	return client, nil
}

// NewClientFromRepo creates a client detecting repo from git remote.
func NewClientFromRepo() (*Client, error) {
	output, err := exec.Command("gh", "repo", "view", "--json", "owner,name").Output()
	if err != nil {
		return nil, fmt.Errorf("detecting repo: %w", err)
	}

	var repo struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(output, &repo); err != nil {
		return nil, fmt.Errorf("parsing repo info: %w", err)
	}

	return NewClient(repo.Owner.Login, repo.Name)
}

// verifyAuth checks if gh is authenticated.
func (c *Client) verifyAuth() error {
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return core.ErrValidation("GH_NOT_AUTHENTICATED",
			"gh CLI is not authenticated, run 'gh auth login'")
	}
	return nil
}

// run executes a gh command.
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", core.ErrTimeout("gh command timed out")
		}
		return "", fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// PullRequest represents a GitHub PR.
type PullRequest struct {
	Number    int
	Title     string
	Body      string
	URL       string
	State     string
	Draft     bool
	Mergeable string
	HeadRef   string
	BaseRef   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreatePR creates a new pull request (implements core.GitHubClient).
func (c *Client) CreatePR(ctx context.Context, opts core.CreatePROptions) (*core.PullRequest, error) {
	args := []string{"pr", "create",
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--title", opts.Title,
		"--body", opts.Body,
		"--base", opts.Base,
		"--head", opts.Head,
	}

	if opts.Draft {
		args = append(args, "--draft")
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	for _, assignee := range opts.Assignees {
		args = append(args, "--assignee", assignee)
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Output is the PR URL
	return c.getPRByURL(ctx, output)
}

// PRCreateOptions holds options for PR creation (local type, deprecated).
type PRCreateOptions struct {
	Title     string
	Body      string
	Base      string
	Head      string
	Draft     bool
	Labels    []string
	Reviewers []string
}

// GetPR retrieves a PR by number (implements core.GitHubClient).
func (c *Client) GetPR(ctx context.Context, number int) (*core.PullRequest, error) {
	output, err := c.run(ctx, "pr", "view", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "number,title,body,url,state,isDraft,mergeable,headRefName,baseRefName,headRefOid,createdAt,updatedAt,mergedAt,labels,assignees")
	if err != nil {
		return nil, err
	}

	return c.parseCorePR(output)
}

// getPRByURL retrieves a PR by URL (internal use).
func (c *Client) getPRByURL(ctx context.Context, url string) (*core.PullRequest, error) {
	output, err := c.run(ctx, "pr", "view", url,
		"--json", "number,title,body,url,state,isDraft,mergeable,headRefName,baseRefName,headRefOid,createdAt,updatedAt,mergedAt,labels,assignees")
	if err != nil {
		return nil, err
	}

	return c.parseCorePR(output)
}

// parseCorePR parses PR JSON output to core.PullRequest.
func (c *Client) parseCorePR(output string) (*core.PullRequest, error) {
	var data struct {
		Number      int       `json:"number"`
		Title       string    `json:"title"`
		Body        string    `json:"body"`
		URL         string    `json:"url"`
		State       string    `json:"state"`
		IsDraft     bool      `json:"isDraft"`
		Mergeable   string    `json:"mergeable"`
		HeadRefName string    `json:"headRefName"`
		HeadRefOid  string    `json:"headRefOid"`
		BaseRefName string    `json:"baseRefName"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		MergedAt    *time.Time `json:"mergedAt"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parsing PR: %w", err)
	}

	var mergeable *bool
	if data.Mergeable != "" {
		m := data.Mergeable == "MERGEABLE"
		mergeable = &m
	}

	labels := make([]string, len(data.Labels))
	for i, l := range data.Labels {
		labels[i] = l.Name
	}

	assignees := make([]string, len(data.Assignees))
	for i, a := range data.Assignees {
		assignees[i] = a.Login
	}

	return &core.PullRequest{
		Number:    data.Number,
		Title:     data.Title,
		Body:      data.Body,
		State:     strings.ToLower(data.State),
		Head:      core.PRBranch{Ref: data.HeadRefName, SHA: data.HeadRefOid, Repo: c.Repo()},
		Base:      core.PRBranch{Ref: data.BaseRefName, Repo: c.Repo()},
		HTMLURL:   data.URL,
		Draft:     data.IsDraft,
		Merged:    data.MergedAt != nil,
		Mergeable: mergeable,
		Labels:    labels,
		Assignees: assignees,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		MergedAt:  data.MergedAt,
	}, nil
}

// parsePR parses PR JSON output (local type, deprecated).
func (c *Client) parsePR(output string) (*PullRequest, error) {
	var data struct {
		Number      int       `json:"number"`
		Title       string    `json:"title"`
		Body        string    `json:"body"`
		URL         string    `json:"url"`
		State       string    `json:"state"`
		IsDraft     bool      `json:"isDraft"`
		Mergeable   string    `json:"mergeable"`
		HeadRefName string    `json:"headRefName"`
		BaseRefName string    `json:"baseRefName"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parsing PR: %w", err)
	}

	return &PullRequest{
		Number:    data.Number,
		Title:     data.Title,
		Body:      data.Body,
		URL:       data.URL,
		State:     data.State,
		Draft:     data.IsDraft,
		Mergeable: data.Mergeable,
		HeadRef:   data.HeadRefName,
		BaseRef:   data.BaseRefName,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// ListPRs lists PRs with options (implements core.GitHubClient).
func (c *Client) ListPRs(ctx context.Context, opts core.ListPROptions) ([]*core.PullRequest, error) {
	args := []string{"pr", "list",
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "number,title,body,url,state,isDraft,headRefName,baseRefName,headRefOid,createdAt,updatedAt,mergedAt,labels,assignees",
	}

	if opts.State != "" {
		args = append(args, "--state", opts.State)
	}
	if opts.Head != "" {
		args = append(args, "--head", opts.Head)
	}
	if opts.Base != "" {
		args = append(args, "--base", opts.Base)
	}
	if opts.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", opts.Limit))
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var prs []struct {
		Number      int        `json:"number"`
		Title       string     `json:"title"`
		Body        string     `json:"body"`
		URL         string     `json:"url"`
		State       string     `json:"state"`
		IsDraft     bool       `json:"isDraft"`
		HeadRefName string     `json:"headRefName"`
		HeadRefOid  string     `json:"headRefOid"`
		BaseRefName string     `json:"baseRefName"`
		CreatedAt   time.Time  `json:"createdAt"`
		UpdatedAt   time.Time  `json:"updatedAt"`
		MergedAt    *time.Time `json:"mergedAt"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, err
	}

	result := make([]*core.PullRequest, len(prs))
	for i, pr := range prs {
		labels := make([]string, len(pr.Labels))
		for j, l := range pr.Labels {
			labels[j] = l.Name
		}
		assignees := make([]string, len(pr.Assignees))
		for j, a := range pr.Assignees {
			assignees[j] = a.Login
		}

		result[i] = &core.PullRequest{
			Number:    pr.Number,
			Title:     pr.Title,
			Body:      pr.Body,
			State:     strings.ToLower(pr.State),
			Head:      core.PRBranch{Ref: pr.HeadRefName, SHA: pr.HeadRefOid, Repo: c.Repo()},
			Base:      core.PRBranch{Ref: pr.BaseRefName, Repo: c.Repo()},
			HTMLURL:   pr.URL,
			Draft:     pr.IsDraft,
			Merged:    pr.MergedAt != nil,
			Labels:    labels,
			Assignees: assignees,
			CreatedAt: pr.CreatedAt,
			UpdatedAt: pr.UpdatedAt,
			MergedAt:  pr.MergedAt,
		}
	}

	return result, nil
}

// MergePR merges a pull request (implements core.GitHubClient).
func (c *Client) MergePR(ctx context.Context, number int, opts core.MergePROptions) error {
	args := []string{"pr", "merge", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
	}

	switch opts.Method {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}

	if opts.CommitTitle != "" {
		args = append(args, "--subject", opts.CommitTitle)
	}
	if opts.CommitMessage != "" {
		args = append(args, "--body", opts.CommitMessage)
	}

	_, err := c.run(ctx, args...)
	return err
}

// ClosePR closes a pull request without merging.
func (c *Client) ClosePR(ctx context.Context, number int) error {
	_, err := c.run(ctx, "pr", "close", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName))
	return err
}

// AddComment adds a comment to a PR.
func (c *Client) AddComment(ctx context.Context, number int, body string) error {
	_, err := c.run(ctx, "pr", "comment", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--body", body)
	return err
}

// RequestReview requests review from users.
func (c *Client) RequestReview(ctx context.Context, number int, reviewers []string) error {
	args := []string{"pr", "edit", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
	}

	for _, reviewer := range reviewers {
		args = append(args, "--add-reviewer", reviewer)
	}

	_, err := c.run(ctx, args...)
	return err
}

// GetDefaultBranch returns the default branch name.
func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	output, err := c.run(ctx, "repo", "view",
		fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "defaultBranchRef")
	if err != nil {
		return "", err
	}

	var data struct {
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return "", err
	}

	return data.DefaultBranchRef.Name, nil
}

// Repo returns owner/name.
func (c *Client) Repo() string {
	return fmt.Sprintf("%s/%s", c.repoOwner, c.repoName)
}

// Owner returns the repository owner.
func (c *Client) Owner() string {
	return c.repoOwner
}

// Name returns the repository name.
func (c *Client) Name() string {
	return c.repoName
}

// WithTimeout sets the command timeout.
func (c *Client) WithTimeout(d time.Duration) *Client {
	c.timeout = d
	return c
}

// UpdatePR updates a pull request (implements core.GitHubClient).
func (c *Client) UpdatePR(ctx context.Context, number int, opts core.UpdatePROptions) error {
	args := []string{"pr", "edit", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
	}

	if opts.Title != nil {
		args = append(args, "--title", *opts.Title)
	}
	if opts.Body != nil {
		args = append(args, "--body", *opts.Body)
	}
	for _, label := range opts.Labels {
		args = append(args, "--add-label", label)
	}
	for _, assignee := range opts.Assignees {
		args = append(args, "--add-assignee", assignee)
	}

	_, err := c.run(ctx, args...)
	return err
}

// PRUpdateOptions holds options for PR updates (local type, deprecated).
type PRUpdateOptions struct {
	Title        string
	Body         string
	AddLabels    []string
	RemoveLabels []string
}

// MarkPRReady marks a draft PR as ready for review.
func (c *Client) MarkPRReady(ctx context.Context, number int) error {
	_, err := c.run(ctx, "pr", "ready", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName))
	return err
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
	args := []string{"issue", "create",
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--title", title,
		"--body", body,
	}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return 0, err
	}

	// Parse issue number from URL
	// Format: https://github.com/owner/repo/issues/123
	parts := strings.Split(output, "/")
	if len(parts) > 0 {
		var num int
		_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &num)
		return num, nil
	}

	return 0, nil
}

// =============================================================================
// core.GitHubClient interface methods
// =============================================================================

// GetRepo returns repository information (implements core.GitHubClient).
func (c *Client) GetRepo(ctx context.Context) (*core.RepoInfo, error) {
	output, err := c.run(ctx, "repo", "view",
		fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "owner,name,defaultBranchRef,isPrivate,url")
	if err != nil {
		return nil, err
	}

	var data struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name             string `json:"name"`
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
		IsPrivate bool   `json:"isPrivate"`
		URL       string `json:"url"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("parsing repo info: %w", err)
	}

	return &core.RepoInfo{
		Owner:         data.Owner.Login,
		Name:          data.Name,
		FullName:      fmt.Sprintf("%s/%s", data.Owner.Login, data.Name),
		DefaultBranch: data.DefaultBranchRef.Name,
		IsPrivate:     data.IsPrivate,
		HTMLURL:       data.URL,
	}, nil
}

// ValidateToken validates the GitHub token (implements core.GitHubClient).
func (c *Client) ValidateToken(ctx context.Context) error {
	return c.verifyAuth()
}

// GetAuthenticatedUser returns the authenticated user's login (implements core.GitHubClient).
func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	output, err := c.run(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GetCheckStatus returns the status of checks for a ref (implements core.GitHubClient).
func (c *Client) GetCheckStatus(ctx context.Context, ref string) (*core.CheckStatus, error) {
	output, err := c.run(ctx, "pr", "checks", ref,
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "name,status,conclusion,detailsUrl,startedAt,completedAt")
	if err != nil {
		return nil, fmt.Errorf("getting checks: %w", err)
	}

	var rawChecks []struct {
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		DetailsURL  string     `json:"detailsUrl"`
		StartedAt   *time.Time `json:"startedAt"`
		CompletedAt *time.Time `json:"completedAt"`
	}

	if err := json.Unmarshal([]byte(output), &rawChecks); err != nil {
		return nil, fmt.Errorf("parsing checks: %w", err)
	}

	status := &core.CheckStatus{
		State:      "success",
		TotalCount: len(rawChecks),
		Checks:     make([]core.Check, 0, len(rawChecks)),
		UpdatedAt:  time.Now(),
	}

	for _, rc := range rawChecks {
		check := core.Check{
			Name:        rc.Name,
			Status:      rc.Status,
			Conclusion:  rc.Conclusion,
			HTMLURL:     rc.DetailsURL,
			StartedAt:   rc.StartedAt,
			CompletedAt: rc.CompletedAt,
		}
		status.Checks = append(status.Checks, check)

		if rc.Status != "completed" {
			status.Pending++
			status.State = "pending"
		} else if rc.Conclusion == "success" || rc.Conclusion == "skipped" || rc.Conclusion == "neutral" {
			status.Passed++
		} else {
			status.Failed++
			status.State = "failure"
		}
	}

	return status, nil
}

// WaitForChecks waits for all checks to complete (implements core.GitHubClient).
func (c *Client) WaitForChecks(ctx context.Context, ref string, timeout time.Duration) (*core.CheckStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pollInterval := 30 * time.Second

	for {
		status, err := c.GetCheckStatus(ctx, ref)
		if err != nil {
			return nil, err
		}

		if !status.IsPending() {
			return status, nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, core.ErrTimeout(fmt.Sprintf("checks did not complete within %v", timeout))
			}
			return nil, ctx.Err()
		case <-time.After(pollInterval):
			// Continue polling
		}
	}
}
