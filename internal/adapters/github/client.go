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

// CreatePR creates a new pull request.
func (c *Client) CreatePR(ctx context.Context, opts PRCreateOptions) (*PullRequest, error) {
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

	for _, reviewer := range opts.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Output is the PR URL
	return c.GetPRByURL(ctx, output)
}

// PRCreateOptions holds options for PR creation.
type PRCreateOptions struct {
	Title     string
	Body      string
	Base      string
	Head      string
	Draft     bool
	Labels    []string
	Reviewers []string
}

// GetPR retrieves a PR by number.
func (c *Client) GetPR(ctx context.Context, number int) (*PullRequest, error) {
	output, err := c.run(ctx, "pr", "view", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--json", "number,title,body,url,state,isDraft,mergeable,headRefName,baseRefName,createdAt,updatedAt")
	if err != nil {
		return nil, err
	}

	return c.parsePR(output)
}

// GetPRByURL retrieves a PR by URL.
func (c *Client) GetPRByURL(ctx context.Context, url string) (*PullRequest, error) {
	output, err := c.run(ctx, "pr", "view", url,
		"--json", "number,title,body,url,state,isDraft,mergeable,headRefName,baseRefName,createdAt,updatedAt")
	if err != nil {
		return nil, err
	}

	return c.parsePR(output)
}

// parsePR parses PR JSON output.
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

// ListPRs lists open PRs.
func (c *Client) ListPRs(ctx context.Context, state string) ([]PullRequest, error) {
	args := []string{"pr", "list",
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
		"--state", state,
		"--json", "number,title,url,state,isDraft,headRefName,baseRefName",
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var prs []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		State       string `json:"state"`
		IsDraft     bool   `json:"isDraft"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
	}

	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, err
	}

	result := make([]PullRequest, len(prs))
	for i, pr := range prs {
		result[i] = PullRequest{
			Number:  pr.Number,
			Title:   pr.Title,
			URL:     pr.URL,
			State:   pr.State,
			Draft:   pr.IsDraft,
			HeadRef: pr.HeadRefName,
			BaseRef: pr.BaseRefName,
		}
	}

	return result, nil
}

// MergePR merges a pull request.
func (c *Client) MergePR(ctx context.Context, number int, method string) error {
	args := []string{"pr", "merge", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
	}

	switch method {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
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

// UpdatePR updates a pull request.
func (c *Client) UpdatePR(ctx context.Context, number int, opts PRUpdateOptions) error {
	args := []string{"pr", "edit", fmt.Sprintf("%d", number),
		"--repo", fmt.Sprintf("%s/%s", c.repoOwner, c.repoName),
	}

	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	}
	for _, label := range opts.AddLabels {
		args = append(args, "--add-label", label)
	}
	for _, label := range opts.RemoveLabels {
		args = append(args, "--remove-label", label)
	}

	_, err := c.run(ctx, args...)
	return err
}

// PRUpdateOptions holds options for PR updates.
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
		fmt.Sscanf(parts[len(parts)-1], "%d", &num)
		return num, nil
	}

	return 0, nil
}
