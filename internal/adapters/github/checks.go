package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ChecksWaiter waits for GitHub CI checks to complete.
type ChecksWaiter struct {
	client         *Client
	pollInterval   time.Duration
	timeout        time.Duration
	requiredChecks []string
}

// NewChecksWaiter creates a new checks waiter.
func NewChecksWaiter(client *Client) *ChecksWaiter {
	return &ChecksWaiter{
		client:       client,
		pollInterval: 30 * time.Second,
		timeout:      30 * time.Minute,
	}
}

// WithPollInterval sets the poll interval.
func (w *ChecksWaiter) WithPollInterval(d time.Duration) *ChecksWaiter {
	w.pollInterval = d
	return w
}

// WithTimeout sets the timeout.
func (w *ChecksWaiter) WithTimeout(d time.Duration) *ChecksWaiter {
	w.timeout = d
	return w
}

// WithRequiredChecks sets specific checks to wait for.
func (w *ChecksWaiter) WithRequiredChecks(checks []string) *ChecksWaiter {
	w.requiredChecks = checks
	return w
}

// CheckStatus represents the status of a CI check.
type CheckStatus struct {
	Name        string
	Status      string // queued, in_progress, completed
	Conclusion  string // success, failure, neutral, cancelled, skipped, timed_out, action_required
	URL         string
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// ChecksResult represents aggregated check results.
type ChecksResult struct {
	AllPassed     bool
	AllCompleted  bool
	Checks        []CheckStatus
	FailedChecks  []string
	PendingChecks []string
	Duration      time.Duration
}

// Wait waits for all checks to complete on a PR.
func (w *ChecksWaiter) Wait(ctx context.Context, prNumber int) (*ChecksResult, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	startTime := time.Now()

	for {
		result, err := w.getChecksStatus(ctx, prNumber)
		if err != nil {
			return nil, err
		}

		if result.AllCompleted {
			result.Duration = time.Since(startTime)
			return result, nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, core.ErrTimeout(
					fmt.Sprintf("checks did not complete within %v", w.timeout))
			}
			return nil, ctx.Err()
		case <-time.After(w.pollInterval):
			// Continue polling
		}
	}
}

// WaitWithCallback waits for checks and calls callback on each poll.
func (w *ChecksWaiter) WaitWithCallback(
	ctx context.Context,
	prNumber int,
	callback func(*ChecksResult),
) (*ChecksResult, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	startTime := time.Now()

	for {
		result, err := w.getChecksStatus(ctx, prNumber)
		if err != nil {
			return nil, err
		}

		if callback != nil {
			callback(result)
		}

		if result.AllCompleted {
			result.Duration = time.Since(startTime)
			return result, nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, core.ErrTimeout(
					fmt.Sprintf("checks did not complete within %v", w.timeout))
			}
			return nil, ctx.Err()
		case <-time.After(w.pollInterval):
			// Continue polling
		}
	}
}

// getChecksStatus retrieves current check status.
func (w *ChecksWaiter) getChecksStatus(ctx context.Context, prNumber int) (*ChecksResult, error) {
	output, err := w.client.run(ctx, "pr", "checks", fmt.Sprintf("%d", prNumber),
		"--repo", w.client.Repo(),
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

	result := &ChecksResult{
		AllPassed:     true,
		AllCompleted:  true,
		Checks:        make([]CheckStatus, 0, len(rawChecks)),
		FailedChecks:  make([]string, 0),
		PendingChecks: make([]string, 0),
	}

	for _, rc := range rawChecks {
		// Skip if required checks specified and this isn't one
		if len(w.requiredChecks) > 0 && !w.isRequired(rc.Name) {
			continue
		}

		check := CheckStatus{
			Name:        rc.Name,
			Status:      rc.Status,
			Conclusion:  rc.Conclusion,
			URL:         rc.DetailsURL,
			StartedAt:   rc.StartedAt,
			CompletedAt: rc.CompletedAt,
		}

		result.Checks = append(result.Checks, check)

		if rc.Status != "completed" {
			result.AllCompleted = false
			result.PendingChecks = append(result.PendingChecks, rc.Name)
		}

		if rc.Conclusion != "" && rc.Conclusion != "success" && rc.Conclusion != "skipped" && rc.Conclusion != "neutral" {
			result.AllPassed = false
			result.FailedChecks = append(result.FailedChecks, rc.Name)
		}
	}

	// If no checks yet, not completed
	if len(result.Checks) == 0 {
		result.AllCompleted = false
		result.AllPassed = false
	}

	return result, nil
}

// isRequired checks if a check name is in the required list.
func (w *ChecksWaiter) isRequired(name string) bool {
	for _, required := range w.requiredChecks {
		if required == name {
			return true
		}
	}
	return false
}

// GetChecks retrieves check status without waiting.
func (w *ChecksWaiter) GetChecks(ctx context.Context, prNumber int) (*ChecksResult, error) {
	return w.getChecksStatus(ctx, prNumber)
}

// WaitForSuccess waits for checks and returns error if any fail.
func (w *ChecksWaiter) WaitForSuccess(ctx context.Context, prNumber int) error {
	result, err := w.Wait(ctx, prNumber)
	if err != nil {
		return err
	}

	if !result.AllPassed {
		return core.ErrExecution("CHECKS_FAILED",
			fmt.Sprintf("CI checks failed: %v", result.FailedChecks))
	}

	return nil
}

// ChecksConfig holds checks waiter configuration.
type ChecksConfig struct {
	PollInterval   time.Duration
	Timeout        time.Duration
	RequiredChecks []string
}

// DefaultChecksConfig returns default configuration.
func DefaultChecksConfig() ChecksConfig {
	return ChecksConfig{
		PollInterval: 30 * time.Second,
		Timeout:      30 * time.Minute,
	}
}

// IsPassing returns true if all checks passed.
func (r *ChecksResult) IsPassing() bool {
	return r.AllCompleted && r.AllPassed
}

// HasFailures returns true if any check failed.
func (r *ChecksResult) HasFailures() bool {
	return len(r.FailedChecks) > 0
}

// Summary returns a human-readable summary.
func (r *ChecksResult) Summary() string {
	total := len(r.Checks)
	pending := len(r.PendingChecks)
	failed := len(r.FailedChecks)
	passed := total - pending - failed

	if r.AllCompleted && r.AllPassed {
		return fmt.Sprintf("All %d checks passed", total)
	}

	if !r.AllCompleted {
		return fmt.Sprintf("%d/%d checks completed, %d pending", total-pending, total, pending)
	}

	return fmt.Sprintf("%d passed, %d failed out of %d checks", passed, failed, total)
}
