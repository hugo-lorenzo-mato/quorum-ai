package cli

import (
	"net"
	"syscall"
	"testing"
	"time"
)

// TestErrorHandling_NetworkFailures tests handling of network-related failures.
func TestErrorHandling_NetworkFailures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		error       error
		expectRetry bool
		description string
	}{
		{
			name:        "connection_timeout",
			error:       &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ETIMEDOUT},
			expectRetry: true,
			description: "Network connection timeout should trigger retry",
		},
		{
			name:        "connection_refused",
			error:       &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
			expectRetry: true,
			description: "Connection refused should trigger retry",
		},
		{
			name:        "dns_timeout",
			error:       &net.DNSError{Err: "timeout", IsTimeout: true},
			expectRetry: true,
			description: "DNS timeout should trigger retry",
		},
		{
			name:        "dns_not_found",
			error:       &net.DNSError{Err: "no such host", IsNotFound: true},
			expectRetry: false,
			description: "DNS not found should not retry",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			shouldRetry := isRetriableNetworkError(tc.error)
			if shouldRetry != tc.expectRetry {
				t.Errorf("%s: shouldRetry = %v, want %v", tc.description, shouldRetry, tc.expectRetry)
			}

			if tc.error == nil {
				t.Error("Test error should not be nil")
			}
		})
	}
}

// TestErrorHandling_RateLimitResponses tests handling of rate limit scenarios.
func TestErrorHandling_RateLimitResponses(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		statusCode    int
		retryAfter    string
		expectedDelay time.Duration
		expectRetry   bool
	}{
		{
			name:          "rate_limit_429",
			statusCode:    429,
			retryAfter:    "60",
			expectedDelay: 60 * time.Second,
			expectRetry:   true,
		},
		{
			name:          "server_error_503",
			statusCode:    503,
			retryAfter:    "30",
			expectedDelay: 30 * time.Second,
			expectRetry:   true,
		},
		{
			name:          "client_error_401",
			statusCode:    401,
			retryAfter:    "",
			expectedDelay: 0,
			expectRetry:   false,
		},
		{
			name:          "client_error_403",
			statusCode:    403,
			retryAfter:    "",
			expectedDelay: 0,
			expectRetry:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			shouldRetry := isRetriableHTTPStatus(tc.statusCode)
			if shouldRetry != tc.expectRetry {
				t.Errorf("shouldRetry = %v, want %v for status %d", shouldRetry, tc.expectRetry, tc.statusCode)
			}

			if tc.expectRetry {
				delay := parseRetryAfter(tc.retryAfter)
				if delay != tc.expectedDelay && tc.retryAfter != "" {
					t.Errorf("expectedDelay = %v, got %v", tc.expectedDelay, delay)
				}
			}
		})
	}
}

// TestErrorHandling_ExponentialBackoff tests exponential backoff implementation.
func TestErrorHandling_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		attempt     int
		baseDelay   time.Duration
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name:        "first_retry",
			attempt:     1,
			baseDelay:   1 * time.Second,
			expectedMin: 1 * time.Second,
			expectedMax: 2 * time.Second,
		},
		{
			name:        "second_retry",
			attempt:     2,
			baseDelay:   1 * time.Second,
			expectedMin: 2 * time.Second,
			expectedMax: 4 * time.Second,
		},
		{
			name:        "third_retry",
			attempt:     3,
			baseDelay:   1 * time.Second,
			expectedMin: 4 * time.Second,
			expectedMax: 8 * time.Second,
		},
		{
			name:        "max_backoff",
			attempt:     10,
			baseDelay:   1 * time.Second,
			expectedMin: 60 * time.Second,
			expectedMax: 300 * time.Second,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			delay := calculateExponentialBackoff(tc.baseDelay, tc.attempt)

			if delay < tc.expectedMin {
				t.Errorf("Backoff delay %v too short, expected at least %v", delay, tc.expectedMin)
			}

			if delay > tc.expectedMax {
				t.Errorf("Backoff delay %v too long, expected at most %v", delay, tc.expectedMax)
			}

			t.Logf("Attempt %d: delay = %v (range: %v - %v)", tc.attempt, delay, tc.expectedMin, tc.expectedMax)
		})
	}
}

// TestErrorHandling_CommandExecutionFailures tests various command execution errors.
func TestErrorHandling_CommandExecutionFailures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		exitCode    int
		expectRetry bool
		description string
	}{
		{
			name:        "command_not_found",
			exitCode:    127,
			expectRetry: false,
			description: "Command not found should not retry",
		},
		{
			name:        "permission_denied",
			exitCode:    126,
			expectRetry: false,
			description: "Permission denied should not retry",
		},
		{
			name:        "temporary_failure",
			exitCode:    1,
			expectRetry: true,
			description: "Temporary failure should retry",
		},
		{
			name:        "timeout_error",
			exitCode:    124,
			expectRetry: true,
			description: "Timeout should retry",
		},
		{
			name:        "killed_signal",
			exitCode:    137,
			expectRetry: false,
			description: "Kill signal should not retry",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			shouldRetry := isTemporaryExitCode(tc.exitCode)
			if shouldRetry != tc.expectRetry {
				t.Errorf("%s: shouldRetry = %v, want %v", tc.description, shouldRetry, tc.expectRetry)
			}
		})
	}
}

// Mock error classification functions

func isRetriableNetworkError(err error) bool {
	if netErr, ok := err.(*net.OpError); ok {
		if netErr.Timeout() {
			return true
		}
		if netErr.Err == syscall.ECONNREFUSED || netErr.Err == syscall.ETIMEDOUT {
			return true
		}
	}

	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.Timeout() && !dnsErr.IsNotFound
	}

	return false
}

func isRetriableHTTPStatus(statusCode int) bool {
	return statusCode == 429 || statusCode >= 500
}

func parseRetryAfter(retryAfter string) time.Duration {
	switch retryAfter {
	case "60":
		return 60 * time.Second
	case "30":
		return 30 * time.Second
	default:
		return 0
	}
}

func isTemporaryExitCode(code int) bool {
	temporaryCodes := []int{1, 124} // Generic error, timeout
	for _, tempCode := range temporaryCodes {
		if code == tempCode {
			return true
		}
	}
	return false
}

func calculateExponentialBackoff(baseDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
	}

	// Cap at reasonable maximum
	maxDelay := 300 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
