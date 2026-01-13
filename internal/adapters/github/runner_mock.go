package github

import (
	"context"
	"errors"
	"strings"
)

// MockRunner is a test double for CommandRunner.
type MockRunner struct {
	// Responses maps command patterns to responses.
	// The key is matched against the joined args.
	Responses map[string]MockResponse

	// Calls records all calls made to the runner.
	Calls []MockCall

	// DefaultResponse is used when no matching response is found.
	DefaultResponse *MockResponse
}

// MockResponse represents a mocked command response.
type MockResponse struct {
	Output string
	Err    error
}

// MockCall records a single call to the runner.
type MockCall struct {
	Name string
	Args []string
}

// NewMockRunner creates a new MockRunner.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses: make(map[string]MockResponse),
		Calls:     make([]MockCall, 0),
	}
}

// Run implements CommandRunner.
func (m *MockRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args})

	// Try to find a matching response
	fullCmd := name + " " + strings.Join(args, " ")

	// Check for exact match first
	if resp, ok := m.Responses[fullCmd]; ok {
		return resp.Output, resp.Err
	}

	// Check for partial matches (prefix)
	for pattern, resp := range m.Responses {
		if strings.HasPrefix(fullCmd, pattern) {
			return resp.Output, resp.Err
		}
	}

	// Check for pattern matches (contains)
	for pattern, resp := range m.Responses {
		if strings.Contains(fullCmd, pattern) {
			return resp.Output, resp.Err
		}
	}

	// Use default response if available
	if m.DefaultResponse != nil {
		return m.DefaultResponse.Output, m.DefaultResponse.Err
	}

	return "", errors.New("no mock response configured for: " + fullCmd)
}

// OnCommand sets a response for a specific command pattern.
func (m *MockRunner) OnCommand(pattern string) *MockResponseBuilder {
	return &MockResponseBuilder{
		runner:  m,
		pattern: pattern,
	}
}

// MockResponseBuilder helps build mock responses fluently.
type MockResponseBuilder struct {
	runner  *MockRunner
	pattern string
}

// Return sets the output for this command.
func (b *MockResponseBuilder) Return(output string) *MockRunner {
	b.runner.Responses[b.pattern] = MockResponse{Output: output}
	return b.runner
}

// ReturnError sets an error for this command.
func (b *MockResponseBuilder) ReturnError(err error) *MockRunner {
	b.runner.Responses[b.pattern] = MockResponse{Err: err}
	return b.runner
}

// ReturnJSON marshals the value and returns it as JSON.
func (b *MockResponseBuilder) ReturnJSON(output string) *MockRunner {
	b.runner.Responses[b.pattern] = MockResponse{Output: output}
	return b.runner
}

// CallCount returns the number of times a command pattern was called.
func (m *MockRunner) CallCount(pattern string) int {
	count := 0
	for _, call := range m.Calls {
		fullCmd := call.Name + " " + strings.Join(call.Args, " ")
		if strings.Contains(fullCmd, pattern) {
			count++
		}
	}
	return count
}

// LastCall returns the last call made, or nil if no calls.
func (m *MockRunner) LastCall() *MockCall {
	if len(m.Calls) == 0 {
		return nil
	}
	return &m.Calls[len(m.Calls)-1]
}

// Reset clears all recorded calls.
func (m *MockRunner) Reset() {
	m.Calls = m.Calls[:0]
}
