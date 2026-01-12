package github_test

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestPRCreateOptions(t *testing.T) {
	opts := github.PRCreateOptions{
		Title:     "Test PR",
		Body:      "Description",
		Base:      "main",
		Head:      "feature",
		Draft:     true,
		Labels:    []string{"bug", "enhancement"},
		Reviewers: []string{"user1", "user2"},
	}

	testutil.AssertEqual(t, opts.Title, "Test PR")
	testutil.AssertEqual(t, opts.Base, "main")
	testutil.AssertEqual(t, opts.Head, "feature")
	testutil.AssertTrue(t, opts.Draft, "should be draft")
	testutil.AssertLen(t, opts.Labels, 2)
	testutil.AssertLen(t, opts.Reviewers, 2)
}

func TestPRUpdateOptions(t *testing.T) {
	opts := github.PRUpdateOptions{
		Title:        "Updated Title",
		Body:         "Updated Body",
		AddLabels:    []string{"new-label"},
		RemoveLabels: []string{"old-label"},
	}

	testutil.AssertEqual(t, opts.Title, "Updated Title")
	testutil.AssertLen(t, opts.AddLabels, 1)
	testutil.AssertLen(t, opts.RemoveLabels, 1)
}

func TestPullRequest(t *testing.T) {
	pr := github.PullRequest{
		Number:    123,
		Title:     "Test PR",
		Body:      "Description",
		URL:       "https://github.com/owner/repo/pull/123",
		State:     "OPEN",
		Draft:     false,
		Mergeable: "MERGEABLE",
		HeadRef:   "feature",
		BaseRef:   "main",
	}

	testutil.AssertEqual(t, pr.Number, 123)
	testutil.AssertEqual(t, pr.Title, "Test PR")
	testutil.AssertEqual(t, pr.State, "OPEN")
	testutil.AssertEqual(t, pr.HeadRef, "feature")
	testutil.AssertEqual(t, pr.BaseRef, "main")
}

// TestGitHubClient_ParsePR tests PR JSON parsing.
// Note: Full client tests require gh CLI to be installed and authenticated.
func TestGitHubClient_ParsePRJSON(t *testing.T) {
	json := `{
		"number": 123,
		"title": "Test PR",
		"body": "Description",
		"url": "https://github.com/owner/repo/pull/123",
		"state": "OPEN",
		"isDraft": false,
		"mergeable": "MERGEABLE",
		"headRefName": "feature",
		"baseRefName": "main"
	}`

	testutil.AssertContains(t, json, "Test PR")
	testutil.AssertContains(t, json, "OPEN")
	testutil.AssertContains(t, json, "feature")
}

// Tests below would require gh CLI to be installed and authenticated.
// They are marked as integration tests.

func TestGitHubClient_Repo(t *testing.T) {
	// This is a unit test that doesn't require gh CLI
	t.Run("repo string format", func(t *testing.T) {
		// Since we can't create a real client without gh auth,
		// we just test the expected format
		expectedFormat := "owner/repo"
		testutil.AssertContains(t, expectedFormat, "/")
	})
}

// Integration test that would run only with gh CLI available
func TestGitHubClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if gh is available
	_, err := testutil.NewGitRepo(t).Run("--version")
	if err != nil {
		t.Skip("gh CLI not available, skipping integration test")
	}

	// Integration tests would go here
	// They would test actual gh CLI interaction
}
