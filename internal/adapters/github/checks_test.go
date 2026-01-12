package github_test

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestCheckStatus(t *testing.T) {
	check := github.CheckStatus{
		Name:       "build",
		Status:     "completed",
		Conclusion: "success",
		URL:        "https://github.com/...",
	}

	testutil.AssertEqual(t, check.Name, "build")
	testutil.AssertEqual(t, check.Status, "completed")
	testutil.AssertEqual(t, check.Conclusion, "success")
}

func TestChecksResult_AllPassed(t *testing.T) {
	result := &github.ChecksResult{
		AllPassed:    true,
		AllCompleted: true,
		Checks: []github.CheckStatus{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "success"},
		},
		FailedChecks:  []string{},
		PendingChecks: []string{},
	}

	testutil.AssertTrue(t, result.IsPassing(), "should be passing")
	testutil.AssertFalse(t, result.HasFailures(), "should not have failures")
	testutil.AssertContains(t, result.Summary(), "passed")
}

func TestChecksResult_HasFailures(t *testing.T) {
	result := &github.ChecksResult{
		AllPassed:    false,
		AllCompleted: true,
		Checks: []github.CheckStatus{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "failure"},
		},
		FailedChecks:  []string{"test"},
		PendingChecks: []string{},
	}

	testutil.AssertFalse(t, result.IsPassing(), "should not be passing")
	testutil.AssertTrue(t, result.HasFailures(), "should have failures")
	testutil.AssertContains(t, result.Summary(), "failed")
}

func TestChecksResult_Pending(t *testing.T) {
	result := &github.ChecksResult{
		AllPassed:    true,
		AllCompleted: false,
		Checks: []github.CheckStatus{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "in_progress", Conclusion: ""},
		},
		FailedChecks:  []string{},
		PendingChecks: []string{"test"},
	}

	testutil.AssertFalse(t, result.IsPassing(), "should not be passing (not completed)")
	testutil.AssertFalse(t, result.HasFailures(), "should not have failures")
	testutil.AssertContains(t, result.Summary(), "pending")
}

func TestDefaultChecksConfig(t *testing.T) {
	cfg := github.DefaultChecksConfig()

	if cfg.PollInterval <= 0 {
		t.Error("PollInterval should be positive")
	}
	if cfg.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
}

func TestChecksWaiter_ParseChecksJSON(t *testing.T) {
	// Test checks JSON parsing structure
	json := `[
		{
			"name": "build",
			"status": "completed",
			"conclusion": "success",
			"detailsUrl": "https://github.com/..."
		},
		{
			"name": "test",
			"status": "in_progress",
			"conclusion": "",
			"detailsUrl": "https://github.com/..."
		}
	]`

	testutil.AssertContains(t, json, "build")
	testutil.AssertContains(t, json, "completed")
	testutil.AssertContains(t, json, "in_progress")
}

func TestChecksResult_Summary_AllPassed(t *testing.T) {
	result := &github.ChecksResult{
		AllPassed:    true,
		AllCompleted: true,
		Checks: []github.CheckStatus{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "success"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
		},
		FailedChecks:  []string{},
		PendingChecks: []string{},
	}

	summary := result.Summary()
	testutil.AssertContains(t, summary, "3")
	testutil.AssertContains(t, summary, "passed")
}

func TestChecksResult_Summary_Mixed(t *testing.T) {
	result := &github.ChecksResult{
		AllPassed:    false,
		AllCompleted: true,
		Checks: []github.CheckStatus{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "failure"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
		},
		FailedChecks:  []string{"test"},
		PendingChecks: []string{},
	}

	summary := result.Summary()
	testutil.AssertContains(t, summary, "2") // passed
	testutil.AssertContains(t, summary, "1") // failed
	testutil.AssertContains(t, summary, "3") // total
}
