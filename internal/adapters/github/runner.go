package github

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	// Run executes a command and returns its output.
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ExecRunner is the production CommandRunner that uses os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes the command using os/exec.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in error for debugging
		if stderr.Len() > 0 {
			return "", &RunError{
				Command: name + " " + strings.Join(args, " "),
				Stderr:  stderr.String(),
				Err:     err,
			}
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunError wraps command execution errors with context.
type RunError struct {
	Command string
	Stderr  string
	Err     error
}

func (e *RunError) Error() string {
	if e.Stderr != "" {
		return e.Command + ": " + e.Stderr + ": " + e.Err.Error()
	}
	return e.Command + ": " + e.Err.Error()
}

func (e *RunError) Unwrap() error {
	return e.Err
}
