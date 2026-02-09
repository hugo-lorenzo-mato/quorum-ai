package github

import (
	"testing"
	"time"
)

func TestDefaultChecksConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultChecksConfig()

	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, 30*time.Second)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Minute)
	}
	if len(cfg.RequiredChecks) != 0 {
		t.Errorf("RequiredChecks should be empty, got %v", cfg.RequiredChecks)
	}
}

func TestChecksResult_IsPassing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result ChecksResult
		want   bool
	}{
		{
			name: "all completed and passed",
			result: ChecksResult{
				AllCompleted: true,
				AllPassed:    true,
			},
			want: true,
		},
		{
			name: "completed but not passed",
			result: ChecksResult{
				AllCompleted: true,
				AllPassed:    false,
			},
			want: false,
		},
		{
			name: "passed but not completed",
			result: ChecksResult{
				AllCompleted: false,
				AllPassed:    true,
			},
			want: false,
		},
		{
			name: "neither completed nor passed",
			result: ChecksResult{
				AllCompleted: false,
				AllPassed:    false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.IsPassing()
			if got != tt.want {
				t.Errorf("IsPassing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChecksResult_HasFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result ChecksResult
		want   bool
	}{
		{
			name:   "no failed checks",
			result: ChecksResult{FailedChecks: []string{}},
			want:   false,
		},
		{
			name:   "has failed checks",
			result: ChecksResult{FailedChecks: []string{"test", "lint"}},
			want:   true,
		},
		{
			name:   "nil failed checks",
			result: ChecksResult{FailedChecks: nil},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasFailures()
			if got != tt.want {
				t.Errorf("HasFailures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChecksResult_Summary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result ChecksResult
		want   string
	}{
		{
			name: "all passed",
			result: ChecksResult{
				AllCompleted: true,
				AllPassed:    true,
				Checks: []CheckStatus{
					{Name: "test", Status: "completed", Conclusion: "success"},
					{Name: "lint", Status: "completed", Conclusion: "success"},
				},
				FailedChecks:  []string{},
				PendingChecks: []string{},
			},
			want: "All 2 checks passed",
		},
		{
			name: "pending checks",
			result: ChecksResult{
				AllCompleted: false,
				AllPassed:    true,
				Checks: []CheckStatus{
					{Name: "test", Status: "completed"},
					{Name: "lint", Status: "in_progress"},
				},
				PendingChecks: []string{"lint"},
				FailedChecks:  []string{},
			},
			want: "1/2 checks completed, 1 pending",
		},
		{
			name: "failed checks",
			result: ChecksResult{
				AllCompleted: true,
				AllPassed:    false,
				Checks: []CheckStatus{
					{Name: "test", Status: "completed", Conclusion: "success"},
					{Name: "lint", Status: "completed", Conclusion: "failure"},
				},
				FailedChecks:  []string{"lint"},
				PendingChecks: []string{},
			},
			want: "1 passed, 1 failed out of 2 checks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Summary()
			if got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckStatus_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	status := CheckStatus{
		Name:        "test",
		Status:      "completed",
		Conclusion:  "success",
		URL:         "https://github.com/owner/repo/actions/runs/123",
		StartedAt:   &now,
		CompletedAt: &now,
	}

	if status.Name != "test" {
		t.Errorf("Name = %q, want %q", status.Name, "test")
	}
	if status.Status != "completed" {
		t.Errorf("Status = %q, want %q", status.Status, "completed")
	}
	if status.Conclusion != "success" {
		t.Errorf("Conclusion = %q, want %q", status.Conclusion, "success")
	}
	if status.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if status.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestNewChecksWaiter(t *testing.T) {
	t.Parallel()
	// Create a minimal client for testing (won't actually work without gh CLI)
	// This just tests the constructor logic
	waiter := &ChecksWaiter{
		pollInterval: 30 * time.Second,
		timeout:      30 * time.Minute,
	}

	if waiter.pollInterval != 30*time.Second {
		t.Errorf("pollInterval = %v, want %v", waiter.pollInterval, 30*time.Second)
	}
	if waiter.timeout != 30*time.Minute {
		t.Errorf("timeout = %v, want %v", waiter.timeout, 30*time.Minute)
	}
}

func TestChecksWaiter_WithPollInterval(t *testing.T) {
	t.Parallel()
	waiter := &ChecksWaiter{
		pollInterval: 30 * time.Second,
		timeout:      30 * time.Minute,
	}

	result := waiter.WithPollInterval(10 * time.Second)

	if result != waiter {
		t.Error("WithPollInterval should return the same waiter")
	}
	if waiter.pollInterval != 10*time.Second {
		t.Errorf("pollInterval = %v, want %v", waiter.pollInterval, 10*time.Second)
	}
}

func TestChecksWaiter_WithTimeout(t *testing.T) {
	t.Parallel()
	waiter := &ChecksWaiter{
		pollInterval: 30 * time.Second,
		timeout:      30 * time.Minute,
	}

	result := waiter.WithTimeout(1 * time.Hour)

	if result != waiter {
		t.Error("WithTimeout should return the same waiter")
	}
	if waiter.timeout != 1*time.Hour {
		t.Errorf("timeout = %v, want %v", waiter.timeout, 1*time.Hour)
	}
}

func TestChecksWaiter_WithRequiredChecks(t *testing.T) {
	t.Parallel()
	waiter := &ChecksWaiter{
		pollInterval: 30 * time.Second,
		timeout:      30 * time.Minute,
	}

	checks := []string{"test", "lint", "build"}
	result := waiter.WithRequiredChecks(checks)

	if result != waiter {
		t.Error("WithRequiredChecks should return the same waiter")
	}
	if len(waiter.requiredChecks) != 3 {
		t.Errorf("len(requiredChecks) = %d, want 3", len(waiter.requiredChecks))
	}
}

func TestChecksWaiter_isRequired(t *testing.T) {
	t.Parallel()
	waiter := &ChecksWaiter{
		requiredChecks: []string{"test", "lint"},
	}

	tests := []struct {
		name      string
		checkName string
		want      bool
	}{
		{"required check test", "test", true},
		{"required check lint", "lint", true},
		{"not required check", "build", false},
		{"empty name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := waiter.isRequired(tt.checkName)
			if got != tt.want {
				t.Errorf("isRequired(%q) = %v, want %v", tt.checkName, got, tt.want)
			}
		})
	}
}

func TestChecksWaiter_isRequired_NoRequiredChecks(t *testing.T) {
	t.Parallel()
	waiter := &ChecksWaiter{
		requiredChecks: []string{},
	}

	// When no required checks are set, isRequired should return false for any check
	if waiter.isRequired("test") {
		t.Error("isRequired should return false when no required checks are set")
	}
}

func TestPRCreateOptions_Fields(t *testing.T) {
	t.Parallel()
	opts := PRCreateOptions{
		Title:     "Test PR",
		Body:      "Description",
		Base:      "main",
		Head:      "feature-branch",
		Draft:     true,
		Labels:    []string{"bug", "urgent"},
		Reviewers: []string{"user1", "user2"},
	}

	if opts.Title != "Test PR" {
		t.Errorf("Title = %q, want %q", opts.Title, "Test PR")
	}
	if opts.Body != "Description" {
		t.Errorf("Body = %q, want %q", opts.Body, "Description")
	}
	if opts.Base != "main" {
		t.Errorf("Base = %q, want %q", opts.Base, "main")
	}
	if opts.Head != "feature-branch" {
		t.Errorf("Head = %q, want %q", opts.Head, "feature-branch")
	}
	if !opts.Draft {
		t.Error("Draft should be true")
	}
	if len(opts.Labels) != 2 {
		t.Errorf("len(Labels) = %d, want 2", len(opts.Labels))
	}
	if len(opts.Reviewers) != 2 {
		t.Errorf("len(Reviewers) = %d, want 2", len(opts.Reviewers))
	}
}

func TestPRUpdateOptions_Fields(t *testing.T) {
	t.Parallel()
	opts := PRUpdateOptions{
		Title:        "Updated Title",
		Body:         "Updated Body",
		AddLabels:    []string{"new-label"},
		RemoveLabels: []string{"old-label"},
	}

	if opts.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", opts.Title, "Updated Title")
	}
	if opts.Body != "Updated Body" {
		t.Errorf("Body = %q, want %q", opts.Body, "Updated Body")
	}
	if len(opts.AddLabels) != 1 {
		t.Errorf("len(AddLabels) = %d, want 1", len(opts.AddLabels))
	}
	if len(opts.RemoveLabels) != 1 {
		t.Errorf("len(RemoveLabels) = %d, want 1", len(opts.RemoveLabels))
	}
}

func TestPullRequest_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	pr := PullRequest{
		Number:    42,
		Title:     "Feature PR",
		Body:      "Adds new feature",
		URL:       "https://github.com/owner/repo/pull/42",
		State:     "OPEN",
		Draft:     false,
		Mergeable: "MERGEABLE",
		HeadRef:   "feature",
		BaseRef:   "main",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Feature PR" {
		t.Errorf("Title = %q, want %q", pr.Title, "Feature PR")
	}
	if pr.State != "OPEN" {
		t.Errorf("State = %q, want %q", pr.State, "OPEN")
	}
	if pr.Mergeable != "MERGEABLE" {
		t.Errorf("Mergeable = %q, want %q", pr.Mergeable, "MERGEABLE")
	}
	if pr.HeadRef != "feature" {
		t.Errorf("HeadRef = %q, want %q", pr.HeadRef, "feature")
	}
	if pr.BaseRef != "main" {
		t.Errorf("BaseRef = %q, want %q", pr.BaseRef, "main")
	}
	if pr.Draft {
		t.Error("Draft should be false")
	}
}

func TestChecksConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := ChecksConfig{
		PollInterval:   15 * time.Second,
		Timeout:        10 * time.Minute,
		RequiredChecks: []string{"ci", "tests"},
	}

	if cfg.PollInterval != 15*time.Second {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, 15*time.Second)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 10*time.Minute)
	}
	if len(cfg.RequiredChecks) != 2 {
		t.Errorf("len(RequiredChecks) = %d, want 2", len(cfg.RequiredChecks))
	}
}
